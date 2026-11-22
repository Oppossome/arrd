package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"generic-arr-project/internal/torrent"
	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/provider"
)

// RuntimeSettings are the saved settings the runtime is built from.
type RuntimeSettings struct {
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

// RuntimeFactory builds a Runtime. It must never fall back to the direct
// network when a VPN is configured.
type RuntimeFactory func(ctx context.Context, s RuntimeSettings) (*Runtime, error)

// errDownloadsUnavailable wraps why there is no engine.
func errDownloadsUnavailable(cause error) error {
	return fmt.Errorf("downloads unavailable: %w", cause)
}

// withEngine runs fn with the current engine while holding the runtime read
// lock, or returns why downloads are unavailable.
func (s *Service) withEngine(fn func(*torrent.Engine) error) error {
	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	if s.rt == nil {
		return errDownloadsUnavailable(s.rtErr)
	}
	return fn(s.rt.Engine)
}

// currentEngine returns the engine or nil; for read-only views.
func (s *Service) currentEngine() *torrent.Engine {
	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	if s.rt == nil {
		return nil
	}
	return s.rt.Engine
}

// RuntimeError returns why downloads are unavailable, or nil.
func (s *Service) RuntimeError() error {
	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	if s.rt == nil {
		return s.rtErr
	}
	return nil
}

func (s *Service) runtimeSettings(ctx context.Context) (RuntimeSettings, error) {
	p, err := LoadVPNProvider(ctx, s.store, s.vpn)
	if err != nil {
		return RuntimeSettings{}, err
	}
	return RuntimeSettings{VPN: p}, nil
}

// ApplyRuntime (re)starts the engine and tunnel from the saved settings and
// resumes every download. The old engine is closed first: its piece
// database and data files allow a single opener.
func (s *Service) ApplyRuntime(ctx context.Context) error {
	settings, err := s.runtimeSettings(ctx)
	if err != nil {
		return err
	}
	s.rtMu.Lock()
	defer s.rtMu.Unlock()
	if s.rt != nil {
		s.rt.Close()
		s.rt = nil
	}
	rt, err := s.newRuntime(ctx, settings)
	if err != nil {
		s.rtErr = err
		s.log.Error("runtime not started", "vpn", settings.VPN != nil, "err", err)
		return errDownloadsUnavailable(err)
	}
	s.rt, s.rtErr = rt, nil
	s.log.Info("runtime started", "vpn", rt.VPN != nil)
	if err := s.restoreDownloads(ctx, rt.Engine); err != nil {
		s.log.Error("restore downloads", "err", err)
	}
	return nil
}

// retryRuntime re-applies the runtime if a previous attempt failed (e.g.
// the VPN was unreachable at startup).
func (s *Service) retryRuntime(ctx context.Context) {
	if !s.SetupComplete() || s.RuntimeError() == nil {
		return
	}
	if err := s.ApplyRuntime(ctx); err == nil {
		s.log.Info("downloads available again")
	}
}

// StopRuntime closes the engine and tunnel. Call on shutdown.
func (s *Service) StopRuntime() {
	s.rtMu.Lock()
	defer s.rtMu.Unlock()
	if s.rt != nil {
		s.rt.Close()
		s.rt = nil
	}
	s.rtErr = errors.New("shutting down")
}

// appSettingsExist reports whether onboarding has written app_settings.
func (s *Service) appSettingsExist(ctx context.Context) (bool, error) {
	_, err := s.store.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
