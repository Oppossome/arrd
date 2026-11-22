package library

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/torrent"
	"generic-arr-project/internal/vpn/provider"
	"generic-arr-project/internal/vpn/provider/wireguard"
)

// testFactory builds runtimes on the host network (IPv4 only). A
// configured VPN is really connected (tests point providers at in-process
// fakes), unless vpnErr is set, which simulates a tunnel that fails to
// come up. With a VPN configured, the engine uses the tunnel and a failed
// tunnel means no runtime, as in production.
type testFactory struct {
	stateDir string
	vpnErr   error
	built    int
}

func (f *testFactory) build(ctx context.Context, rs RuntimeSettings) (*Runtime, error) {
	rt := &Runtime{}
	var n netx.Network = v4Only{}
	if rs.VPN != nil {
		err := f.vpnErr
		var conn *provider.Connection
		if err == nil {
			conn, err = rs.VPN.Connect(ctx, provider.ConnectOptions{PortForward: true}, quietLog())
		}
		if err != nil {
			return nil, err
		}
		rt.VPN, n = conn, conn.Tunnel
	}
	eng, err := torrent.New(torrent.Config{StateDir: f.stateDir, Network: n, Seed: true}, quietLog())
	if err != nil {
		if rt.VPN != nil {
			rt.VPN.Close()
		}
		return nil, err
	}
	f.built++
	rt.Engine = eng
	rt.Close = func() {
		eng.Close()
		if rt.VPN != nil {
			rt.VPN.Close()
		}
	}
	return rt, nil
}

// wgInput selects the static WireGuard provider with config.
func wgInput(config string) VPNInput {
	return VPNInput{Provider: wireguard.ID, Values: provider.Values{"config": config}}
}

type testEnv struct {
	svc     *Service
	store   *store.Store
	factory *testFactory
	apiKey  string
	dataDir string
}

// newEnv opens (or reuses) a store under base/data and starts a Service.
// With setup non-nil, onboarding is completed with it.
func newEnv(t *testing.T, base, tmdbURL string, setup *SetupInput, st *store.Store) *testEnv {
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
		t.Cleanup(func() { st.Close() })
	}
	writeTestDefinition(t, dataDir, defaultSiteLinks...)
	factory := &testFactory{stateDir: filepath.Join(dataDir, "torrent-state")}
	svc, err := New(Deps{
		Store: st, NewRuntime: factory.build, DataDir: dataDir, Log: quietLog(),
		DefinitionsDir: filepath.Join(dataDir, "definitions"),
		Direct:         v4Only{}, TMDBBaseURL: tmdbURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.StopRuntime)
	env := &testEnv{svc: svc, store: st, factory: factory, dataDir: dataDir}
	if setup != nil {
		res, err := svc.CompleteSetup(ctx, *setup)
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		if res.RuntimeError != "" {
			t.Fatalf("setup runtime: %s", res.RuntimeError)
		}
		env.apiKey = res.APIKey
	}
	return env
}

// basicSetup is a direct-network setup with dirs under base.
func basicSetup(base string) *SetupInput {
	return &SetupInput{
		AdminPassword: testPassword,
		TMDBAPIKey:    "k",
		Downloads: DownloadSettings{
			IncompleteDir: filepath.Join(base, "incomplete"),
			CompleteDir:   filepath.Join(base, "complete"),
		},
		MovieRoot:     filepath.Join(base, "Movies"),
		TVRoot:        filepath.Join(base, "TV"),
		CreateMissing: true,
	}
}

const testPassword = "correct horse battery"

func ptr[T any](v T) *T { return &v }

// v4Only is the host network without IPv6, so tests don't depend on it.
type v4Only struct{ netx.Direct }

func (v4Only) HasIPv6() bool { return false }

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
