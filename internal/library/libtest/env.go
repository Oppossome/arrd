// Package libtest builds a complete, wired set of library services for
// tests, with fake TMDB servers, fake indexer sites and local seeders. It is
// only imported by tests.
package libtest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"generic-arr-project/internal/library"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/library/setup"
	"generic-arr-project/internal/library/vpnconfig"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/torrent"
	"generic-arr-project/internal/vpn/provider"
	"generic-arr-project/internal/vpn/provider/wireguard"
)

// TestPassword is the admin password BasicSetup uses.
const TestPassword = "correct horse battery"

// Factory builds runtimes on the host network (IPv4 only). A configured VPN
// is really connected (tests point providers at in-process fakes), unless
// VPNErr is set, which simulates a tunnel that fails to come up. With a VPN
// configured, the engine uses the tunnel and a failed tunnel means no
// runtime, as in production.
type Factory struct {
	stateDir string
	VPNErr   error
	Built    int // runtimes built so far
}

func (f *Factory) build(ctx context.Context, rs engine.Settings) (*engine.Runtime, error) {
	rt := &engine.Runtime{}
	var n netx.Network = V4Only{}
	if rs.VPN != nil {
		err := f.VPNErr
		var conn *provider.Connection
		if err == nil {
			conn, err = rs.VPN.Connect(ctx, provider.ConnectOptions{PortForward: true}, QuietLog())
		}
		if err != nil {
			return nil, err
		}
		rt.VPN, n = conn, conn.Tunnel
	}
	eng, err := torrent.New(torrent.Config{StateDir: f.stateDir, Network: n, Seed: true}, QuietLog())
	if err != nil {
		if rt.VPN != nil {
			rt.VPN.Close()
		}
		return nil, err
	}
	f.Built++
	rt.Engine = eng
	rt.Close = func() {
		eng.Close()
		if rt.VPN != nil {
			rt.VPN.Close()
		}
	}
	return rt, nil
}

// Env is a running set of services and what they were built with.
type Env struct {
	Svc     *library.Services
	Store   *store.Store
	Factory *Factory
	APIKey  string // set when NewEnv completed setup
	DataDir string

	mu  sync.Mutex
	now func() time.Time
}

// SetNow replaces the clock every service reads.
func (e *Env) SetNow(now func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = now
}

func (e *Env) clock() time.Time {
	e.mu.Lock()
	now := e.now
	e.mu.Unlock()
	return now()
}

// NewEnv opens (or reuses) a store under base/data and starts the services,
// with opts applied to their Deps. With in non-nil, onboarding is completed
// with it.
func NewEnv(t *testing.T, base, tmdbURL string, in *setup.SetupInput, st *store.Store, opts ...func(*library.Deps)) *Env {
	t.Helper()
	ctx := context.Background()
	dataDir := filepath.Join(base, "data")
	if st == nil {
		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			t.Fatal(err)
		}
		var err error
		st, err = store.Open(ctx, filepath.Join(dataDir, "arrd.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := st.Close(); err != nil {
				t.Errorf("close store: %v", err)
			}
		})
	}
	WriteTestDefinition(t, dataDir, DefaultSiteLinks...)
	env := &Env{
		Store: st, DataDir: dataDir, now: time.Now,
		Factory: &Factory{stateDir: filepath.Join(dataDir, "torrent-state")},
	}
	d := library.Deps{
		Store: st, NewRuntime: env.Factory.build, DataDir: dataDir, Log: QuietLog(),
		DefinitionsDir: filepath.Join(dataDir, "definitions"),
		Direct:         V4Only{}, TMDBBaseURL: tmdbURL, Now: env.clock,
	}
	for _, opt := range opts {
		opt(&d)
	}
	svc, err := library.New(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Stop)
	env.Svc = svc
	if in != nil {
		res, err := svc.Setup.CompleteSetup(ctx, *in)
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		if res.RuntimeError != "" {
			t.Fatalf("setup runtime: %s", res.RuntimeError)
		}
		env.APIKey = res.APIKey
	}
	return env
}

// BasicSetup is a direct-network setup with dirs under base.
func BasicSetup(base string) *setup.SetupInput {
	return &setup.SetupInput{
		AdminPassword: TestPassword,
		TMDBAPIKey:    "k",
		Downloads: settings.DownloadSettings{
			IncompleteDir: filepath.Join(base, "incomplete"),
			CompleteDir:   filepath.Join(base, "complete"),
		},
		MovieRoot:     filepath.Join(base, "Movies"),
		TVRoot:        filepath.Join(base, "TV"),
		CreateMissing: true,
	}
}

// WGInput selects the static WireGuard provider with config.
func WGInput(config string) vpnconfig.VPNInput {
	return vpnconfig.VPNInput{Provider: wireguard.ID, Values: provider.Values{"config": config}}
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// V4Only is the host network without IPv6, so tests don't depend on it.
type V4Only struct{ netx.Direct }

// HasIPv6 reports false.
func (V4Only) HasIPv6() bool { return false }

// QuietLog discards everything.
func QuietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
