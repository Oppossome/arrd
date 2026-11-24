// Package engine owns the running torrent engine and VPN tunnel, and picks
// the network indexer and download traffic uses.
package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/torrent"
	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/provider"

	"github.com/rs/zerolog"
)

// Settings are the saved settings the runtime is built from.
type Settings struct {
	VPN provider.Provider // nil = no VPN configured
}

// Runtime is the running torrent engine and, if configured, VPN tunnel.
type Runtime struct {
	Engine *torrent.Engine
	// VPN is the live tunnel, nil when no VPN is configured. A configured
	// VPN whose tunnel fails means no runtime at all: the factory returns
	// an error rather than falling back to direct.
	VPN   *provider.Connection
	Close func()
}

// tunnel returns the live tunnel or nil.
func (rt *Runtime) tunnel() *vpn.Tunnel {
	if rt == nil || rt.VPN == nil {
		return nil
	}
	return rt.VPN.Tunnel
}

// Factory builds a Runtime. It must never fall back to the direct network
// when a VPN is configured.
type Factory func(ctx context.Context, s Settings) (*Runtime, error)

// StartHook runs with a newly started engine while the runtime is locked.
type StartHook func(ctx context.Context, eng *torrent.Engine) error

// Manager starts, swaps and stops the runtime.
type Manager struct {
	store      *store.Store
	newRuntime Factory
	direct     netx.Network
	vpn        *provider.Registry
	log        *zerolog.Logger
	onStart    []StartHook

	// mu guards the runtime. Work that uses the engine holds a read lock
	// for its duration; Apply takes the write lock, so an engine is never
	// closed underneath a grab or an import.
	mu  sync.RWMutex
	rt  *Runtime
	err error // why there is no runtime, if there is none
}

// Deps are the Manager's collaborators.
type Deps struct {
	Store      *store.Store
	NewRuntime Factory
	Direct     netx.Network
	VPN        *provider.Registry
	Log        *zerolog.Logger
}

// New creates a Manager with no runtime.
func New(d Deps) (*Manager, error) {
	if d.Store == nil || d.NewRuntime == nil || d.Direct == nil || d.VPN == nil || d.Log == nil {
		return nil, errors.New("engine: Store, NewRuntime, Direct, VPN and Log are required")
	}
	return &Manager{
		store: d.Store, newRuntime: d.NewRuntime, direct: d.Direct, vpn: d.VPN, log: d.Log,
		err: errors.New("setup is not complete"),
	}, nil
}

// OnEngineStart registers fn to run each time an engine starts. Register
// hooks before the first Apply.
func (m *Manager) OnEngineStart(fn StartHook) { m.onStart = append(m.onStart, fn) }

// Providers is the VPN provider registry.
func (m *Manager) Providers() *provider.Registry { return m.vpn }

// Direct is the host network.
func (m *Manager) Direct() netx.Network { return m.direct }

// errDownloadsUnavailable wraps why there is no engine.
func errDownloadsUnavailable(cause error) error {
	return fmt.Errorf("downloads unavailable: %w", cause)
}

// WithEngine runs fn with the current engine while holding the runtime
// read lock, or returns why downloads are unavailable.
func (m *Manager) WithEngine(fn func(*torrent.Engine) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rt == nil {
		return errDownloadsUnavailable(m.err)
	}
	return fn(m.rt.Engine)
}

// Inspect runs fn with the runtime (nil if there is none) and why there is
// none, holding the read lock so the runtime is not swapped meanwhile.
func (m *Manager) Inspect(fn func(rt *Runtime, rtErr error)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fn(m.rt, m.err)
}

// CurrentEngine returns the engine or nil; for read-only views.
func (m *Manager) CurrentEngine() *torrent.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rt == nil {
		return nil
	}
	return m.rt.Engine
}

// RuntimeError returns why downloads are unavailable, or nil.
func (m *Manager) RuntimeError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rt == nil {
		return m.err
	}
	return nil
}

func (m *Manager) settings(ctx context.Context) (Settings, error) {
	p, err := LoadVPNProvider(ctx, m.store, m.vpn)
	if err != nil {
		return Settings{}, err
	}
	return Settings{VPN: p}, nil
}

// Apply (re)starts the engine and tunnel from the saved settings and runs
// the start hooks (which resume every download). The old engine is closed
// first: its piece database and data files allow a single opener.
func (m *Manager) Apply(ctx context.Context) error {
	settings, err := m.settings(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rt != nil {
		m.rt.Close()
		m.rt = nil
	}
	rt, err := m.newRuntime(ctx, settings)
	if err != nil {
		m.err = err
		m.log.Error().Bool("vpn", settings.VPN != nil).Err(err).Msg("runtime not started")
		return errDownloadsUnavailable(err)
	}
	m.rt, m.err = rt, nil
	m.log.Info().Bool("vpn", rt.VPN != nil).Msg("runtime started")
	for _, hook := range m.onStart {
		if err := hook(ctx, rt.Engine); err != nil {
			m.log.Error().Err(err).Msg("engine start hook")
		}
	}
	return nil
}

// Retry re-applies the runtime if a previous attempt failed (e.g. the VPN
// was unreachable at startup).
func (m *Manager) Retry(ctx context.Context) {
	if m.RuntimeError() == nil {
		return
	}
	if err := m.Apply(ctx); err == nil {
		m.log.Info().Msg("downloads available again")
	}
}

// Stop closes the engine and tunnel. Call on shutdown.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rt != nil {
		m.rt.Close()
		m.rt = nil
	}
	m.err = errors.New("shutting down")
}

// TrafficNetwork is the network indexer and download traffic uses: the VPN
// whenever one is configured, never falling back to direct while its
// tunnel is down, and direct when there is none.
func (m *Manager) TrafficNetwork(ctx context.Context) (netx.Network, error) {
	hasVPN, err := m.VPNConfigured(ctx)
	if err != nil {
		return nil, err
	}
	if !hasVPN {
		return m.direct, nil
	}
	return m.VPNNetwork()
}

// VPNNetwork returns the live tunnel, or why there is none.
func (m *Manager) VPNNetwork() (netx.Network, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if tun := m.rt.tunnel(); tun != nil {
		return tun, nil
	}
	if m.err != nil {
		return nil, fmt.Errorf("vpn is down: %w", m.err)
	}
	return nil, errors.New("no VPN is configured")
}

// VPNConfigured reports whether a VPN is saved.
func (m *Manager) VPNConfigured(ctx context.Context) (bool, error) {
	_, err := m.store.GetVPNSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load vpn settings: %w", err)
	}
	return true, nil
}
