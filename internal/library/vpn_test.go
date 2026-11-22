package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"generic-arr-project/internal/vpn/pia"
	"generic-arr-project/internal/vpn/pia/piatest"
	"generic-arr-project/internal/vpn/provider"
	"generic-arr-project/internal/vpn/provider/wireguard"
	"generic-arr-project/internal/vpn/vpntest"
)

// lockedBuffer collects log output from any goroutine.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newVPNEnv is a not-yet-onboarded env whose providers are the static
// WireGuard one and PIA pointed at a fake, logging into the returned buffer.
func newVPNEnv(t *testing.T) (*testEnv, *piatest.Fake, *lockedBuffer) {
	t.Helper()
	fake := piatest.New(t)
	env := newEnv(t, t.TempDir(), "http://unused", nil, nil)
	reg, err := provider.NewRegistry(wireguard.Definition(), pia.Definition(fake.Client()))
	if err != nil {
		t.Fatal(err)
	}
	env.svc.vpn = reg
	logs := &lockedBuffer{}
	env.svc.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return env, fake, logs
}

func piaInput() VPNInput {
	return VPNInput{Provider: pia.ProviderID, Values: provider.Values{
		"username": piatest.Username, "password": piatest.Password, "region": piatest.Region,
	}}
}

func vpnSetup(base string, in VPNInput) SetupInput {
	s := *basicSetup(base)
	s.VPN = &in
	return s
}

// advance makes the service's clock run d ahead of real time.
func advance(env *testEnv, d time.Duration) {
	env.svc.now = func() time.Time { return time.Now().Add(d) }
}

func TestVPNProvidersAndOptions(t *testing.T) {
	ctx := context.Background()
	env, _, _ := newVPNEnv(t)

	var ids []string
	for _, d := range env.svc.VPNProviders() {
		ids = append(ids, d.ID)
	}
	if !slices.Equal(ids, []string{wireguard.ID, pia.ProviderID}) {
		t.Fatalf("providers = %v", ids)
	}
	raw, err := json.Marshal(env.svc.VPNProviders())
	if err != nil {
		t.Fatalf("definitions must encode to JSON: %v", err)
	}
	if !strings.Contains(string(raw), `"secret":true`) {
		t.Errorf("secret flags missing from %s", raw)
	}

	opts, err := env.svc.VPNOptions(ctx, pia.ProviderID, "region")
	if err != nil {
		t.Fatal(err)
	}
	var pf []string
	for _, o := range opts {
		if slices.Contains(o.Tags, pia.TagPortForward) {
			pf = append(pf, o.Value)
		}
	}
	if len(opts) != 2 || !slices.Equal(pf, []string{piatest.Region}) {
		t.Errorf("region options = %+v", opts)
	}
	for _, c := range [][2]string{{"nowhere-vpn", "region"}, {pia.ProviderID, "username"}, {wireguard.ID, "config"}} {
		if _, err := env.svc.VPNOptions(ctx, c[0], c[1]); !errors.Is(err, ErrNotFound) {
			t.Errorf("options %v: err = %v, want not found", c, err)
		}
	}
}

func TestVPNSetupValidation(t *testing.T) {
	ctx := context.Background()
	env, _, _ := newVPNEnv(t)
	base := t.TempDir()

	cases := map[string]struct {
		values func(provider.Values)
		field  string
	}{
		"wrong password": {func(v provider.Values) { v["password"] = "nope" }, "vpn.password"},
		"no password":    {func(v provider.Values) { delete(v, "password") }, "vpn.password"},
		"no username":    {func(v provider.Values) { v["username"] = " " }, "vpn.username"},
		"unknown region": {func(v provider.Values) { v["region"] = "xx-nowhere" }, "vpn.region"},
		"pf unsupported": {func(v provider.Values) { v["region"], v["port_forward"] = piatest.RegionNoPF, "true" }, "vpn.port_forward"},
		"bad toggle":     {func(v provider.Values) { v["port_forward"] = "yes" }, "vpn.port_forward"},
		"unknown field":  {func(v provider.Values) { v["config"] = "x" }, "vpn.config"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := piaInput()
			tc.values(in.Values)
			_, err := env.svc.CompleteSetup(ctx, vpnSetup(base, in))
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Fatalf("err = %v, want validation error on %s", err, tc.field)
			}
		})
	}
	if env.svc.SetupComplete() {
		t.Fatal("setup completed with invalid VPN input")
	}
}

func TestVPNRenewingProviderLifecycle(t *testing.T) {
	ctx := context.Background()
	env, fake, logs := newVPNEnv(t)
	svc := env.svc

	res, err := svc.CompleteSetup(ctx, vpnSetup(t.TempDir(), piaInput()))
	if err != nil || res.RuntimeError != "" {
		t.Fatalf("setup = %+v, %v", res, err)
	}
	if svc.currentEngine() == nil || len(fake.Keys()) != 1 {
		t.Fatalf("engine %v, keys %d", svc.currentEngine(), len(fake.Keys()))
	}

	vs, err := svc.VPNSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if vs.Provider != pia.ProviderID || !vs.TunnelUp || vs.Server != piatest.ServerName ||
		!slices.Equal(vs.SecretsSet, []string{"password"}) || vs.Values["username"] != piatest.Username ||
		vs.Values["password"] != "" || len(vs.Addresses) != 1 {
		t.Fatalf("vpn settings = %+v", vs)
	}
	raw, _ := json.Marshal(vs)
	if strings.Contains(string(raw), piatest.Password) {
		t.Fatalf("VPN settings JSON leaks the password: %s", raw)
	}

	// Healthy: the check leaves the tunnel alone.
	if err := svc.CheckVPN(ctx); err != nil || len(fake.Keys()) != 1 {
		t.Fatalf("healthy check: err %v, keys %d", err, len(fake.Keys()))
	}

	// The provider forgets us: no handshakes. The check reconnects, which
	// registers a new key.
	fake.DropPeers()
	advance(env, 2*vpnStaleAfter)
	if err := svc.CheckVPN(ctx); err != nil {
		t.Fatal(err)
	}
	keys := fake.Keys()
	if len(keys) != 2 || keys[0] == keys[1] {
		t.Fatalf("after stale handshake: %d keys, want 2 distinct", len(keys))
	}
	if svc.currentEngine() == nil {
		t.Fatal("no engine after reconnecting")
	}
	if !strings.Contains(logs.String(), "reconnecting vpn") {
		t.Error("reconnect was not logged")
	}

	// The provider's API is down when the tunnel dies: fail closed.
	fake.SetDown(true)
	fake.DropPeers()
	advance(env, 4*vpnStaleAfter)
	if err := svc.CheckVPN(ctx); err == nil {
		t.Fatal("reconnecting succeeded with the provider down")
	}
	if svc.currentEngine() != nil || svc.RuntimeError() == nil {
		t.Fatal("engine still running without its VPN")
	}
	if _, err := svc.trafficNetwork(ctx); err == nil {
		t.Fatal("traffic network available while the VPN is down")
	}

	// The provider is back: the regular runtime retry restores downloads.
	fake.SetDown(false)
	svc.retryRuntime(ctx)
	if svc.currentEngine() == nil || len(fake.Keys()) != 3 {
		t.Fatalf("after recovery: engine %v, keys %d", svc.currentEngine(), len(fake.Keys()))
	}

	// Saving without the secret keeps the saved one for the same provider…
	in := piaInput()
	delete(in.Values, "password")
	if _, err := svc.SetVPN(ctx, in); err != nil {
		t.Fatalf("SetVPN keeping password: %v", err)
	}
	// …but never carries it over to another provider.
	var ve *ValidationError
	if _, err := svc.SetVPN(ctx, VPNInput{Provider: wireguard.ID}); !errors.As(err, &ve) || ve.Field != "config" {
		t.Fatalf("switch provider without its secret: err = %v", err)
	}

	// Removing the VPN is allowed while it is in use; traffic then runs direct.
	if err := svc.RemoveVPN(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.trafficNetwork(ctx); err != nil || n != svc.direct {
		t.Errorf("traffic network after removal = %v, %v; want direct", n, err)
	}
	if svc.currentEngine() == nil {
		t.Error("no engine after removing the VPN")
	}
	if vs, _ := svc.VPNSettings(ctx); vs.Configured {
		t.Errorf("still configured after removal: %+v", vs)
	}
	// With PIA gone, an empty password is no longer filled in.
	if _, err := svc.SetVPN(ctx, in); !errors.As(err, &ve) || ve.Field != "password" {
		t.Fatalf("password kept after removal: err = %v", err)
	}

	if l := logs.String(); strings.Contains(l, piatest.Password) || !strings.Contains(l, pia.ProviderID) {
		t.Errorf("logs leak the password or never name the provider:\n%s", l)
	}
}

// A static config is not reconnected for a stale handshake: connecting
// again with the same keys cannot help.
func TestStaticVPNNotReprovisioned(t *testing.T) {
	ctx := context.Background()
	env, _, _ := newVPNEnv(t)
	svc := env.svc
	server, clientConf := vpntest.Server(t)
	if _, err := svc.CompleteSetup(ctx, vpnSetup(t.TempDir(), wgInput(clientConf))); err != nil {
		t.Fatal(err)
	}
	vs, _ := svc.VPNSettings(ctx)
	if !vs.TunnelUp || vs.Provider != wireguard.ID || !slices.Equal(vs.SecretsSet, []string{"config"}) || len(vs.Values) != 0 {
		t.Fatalf("vpn settings = %+v", vs)
	}
	built := env.factory.built
	server.Close()
	advance(env, 2*vpnStaleAfter)
	if err := svc.CheckVPN(ctx); err != nil || env.factory.built != built {
		t.Fatalf("static tunnel restarted: err %v, built %d -> %d", err, built, env.factory.built)
	}
}

// A test connection that cannot be made is a user-facing result (400),
// not a server error.
func TestTestVPNConnectFailureIsValidation(t *testing.T) {
	env, _, _ := newVPNEnv(t)
	unreachable := strings.Replace(testWG, "198.51.100.7:51820", "127.0.0.1:9", 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := env.svc.TestVPN(ctx, ptr(wgInput(unreachable)))
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "" || !strings.Contains(ve.Message, "could not connect") {
		t.Fatalf("err = %v, want a field-less validation error", err)
	}
}
