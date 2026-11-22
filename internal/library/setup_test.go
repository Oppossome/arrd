package library

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/vpn/vpntest"
)

// A syntactically valid config; the test factory never dials it.
const testWG = `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.64.1.2/32
DNS = 10.64.0.1

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = 198.51.100.7:51820
`

func TestSetupValidationAndOnce(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := newEnv(t, base, "http://unused", nil, nil)
	svc := env.svc

	// Before setup: no engine, nothing processed, grabs refused.
	if svc.SetupComplete() || svc.RuntimeError() == nil {
		t.Fatal("service claims to be set up")
	}
	if err := svc.ProcessDownloads(ctx); err != nil {
		t.Errorf("ProcessDownloads before setup = %v, want no-op", err)
	}
	if svc.CheckAPIKey("") || svc.CheckAPIKey("anything") {
		t.Error("API key accepted before setup")
	}

	mutate := func(f func(*SetupInput)) SetupInput {
		in := *basicSetup(base)
		f(&in)
		return in
	}
	cases := []struct {
		field string
		in    SetupInput
	}{
		{"tmdb_api_key", mutate(func(in *SetupInput) { in.TMDBAPIKey = " " })},
		{"vpn.config", mutate(func(in *SetupInput) { v := wgInput("[Interface]\nPrivateKey = nope*\n"); in.VPN = &v })},
		{"vpn.provider", mutate(func(in *SetupInput) { in.VPN = &VPNInput{Provider: "nowhere-vpn"} })},
		{"movie_root", mutate(func(in *SetupInput) { in.MovieRoot, in.TVRoot = "", "" })},
		{"downloads.incomplete_dir", mutate(func(in *SetupInput) { in.Downloads.IncompleteDir = "relative/dir" })},
		{"indexers[1].definition", mutate(func(in *SetupInput) {
			in.Indexers = []IndexerInput{siteIndexer("a", "http://site-a.invalid/"), siteIndexer("b", "http://site-b.invalid/")}
		})},
		{"indexers[1].name", mutate(func(in *SetupInput) {
			in.Indexers = []IndexerInput{siteIndexer("a", "http://site-a.invalid/"), onSite(siteIndexer("a", "http://site-b.invalid/"), otherSiteID)}
		})},
	}
	for _, c := range cases {
		_, err := svc.CompleteSetup(ctx, c.in)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != c.field {
			t.Errorf("want validation error on %s, got %v", c.field, err)
		}
	}
	if svc.SetupComplete() {
		t.Fatal("a failed setup marked setup complete")
	}

	in := mutate(func(in *SetupInput) {
		in.Indexers = []IndexerInput{siteIndexerKey("idx", "http://site-a.invalid/", "s")}
	})
	res, err := svc.CompleteSetup(ctx, in)
	if err != nil || res.APIKey == "" || res.RuntimeError != "" {
		t.Fatalf("setup = %+v, %v", res, err)
	}
	if !svc.SetupComplete() || svc.RuntimeError() != nil || svc.currentEngine() == nil {
		t.Fatal("engine not running after setup")
	}
	if !svc.CheckAPIKey(res.APIKey) || svc.CheckAPIKey(res.APIKey+"x") {
		t.Error("API key check wrong")
	}
	if ixs, _ := svc.Indexers(ctx); len(ixs) != 1 || !slices.Equal(ixs[0].SecretsSet, []string{"apikey"}) {
		t.Errorf("indexers after setup = %+v", ixs)
	}
	roots, _ := svc.RootFolders(ctx, MediaTV)
	if len(roots) != 1 || !roots[0].IsDefault {
		t.Errorf("tv roots = %+v", roots)
	}
	var ce *ConflictError
	if _, err := svc.CompleteSetup(ctx, in); !errors.As(err, &ce) {
		t.Errorf("second setup = %v, want conflict", err)
	}

	// Rotating the key invalidates the old one.
	newKey, err := svc.RegenerateAPIKey(ctx)
	if err != nil || svc.CheckAPIKey(res.APIKey) || !svc.CheckAPIKey(newKey) {
		t.Errorf("regenerate: %v", err)
	}

	// A restarted service picks the saved state up.
	svc.StopRuntime()
	env2 := newEnv(t, base, "http://unused", nil, env.store)
	if !env2.svc.SetupComplete() || !env2.svc.CheckAPIKey(newKey) || env2.svc.currentEngine() == nil {
		t.Error("restart lost setup state")
	}
}

// With a VPN configured, downloads and indexers never fall back to direct
// while the tunnel is down; removing the VPN brings both back on direct.
func TestTrafficFailsClosedWhileVPNDown(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := newEnv(t, base, "http://unused", basicSetup(base), nil)
	svc := env.svc

	var hits atomic.Int32
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(site.Close)
	env.useSites(t, site.URL+"/")
	ix, err := svc.CreateIndexer(ctx, siteIndexer("site", site.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}

	env.factory.vpnErr = errors.New("handshake timed out")
	if _, err := svc.SetVPN(ctx, wgInput(testWG)); err != nil {
		t.Fatal(err)
	}
	if svc.currentEngine() != nil {
		t.Fatal("engine running although its VPN is down")
	}
	if vs, _ := svc.VPNSettings(ctx); !vs.Configured || vs.TunnelUp || !strings.Contains(vs.TunnelError, "handshake") {
		t.Errorf("vpn settings = %+v", vs)
	}
	if _, err := svc.grab(ctx, grabTarget{}, ReleaseResult{Title: "x", IndexerID: ix.ID, MagnetURI: "magnet:?xt=urn:btih:0000000000000000000000000000000000000000"}); err == nil {
		t.Error("grab succeeded without an engine")
	}
	if _, err := svc.TestIndexer(ctx, ix.ID); err == nil || !strings.Contains(err.Error(), "vpn is down") {
		t.Errorf("indexer test while the VPN is down = %v, want vpn is down", err)
	}
	if _, failures, err := svc.searchIndexers(ctx, func(Indexer) (indexer.Query, bool) {
		return indexer.Query{Type: indexer.SearchGeneric, Q: "anything"}, true
	}); err != nil || len(failures) != 1 || !strings.Contains(failures[0].Error, "vpn is down") {
		t.Errorf("search while the VPN is down: failures %+v, err %v", failures, err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the site was reached %d times while the VPN was down", n)
	}

	// The scheduler's retry keeps failing closed while the tunnel is down…
	svc.retryRuntime(ctx)
	if svc.currentEngine() != nil {
		t.Fatal("retry started an engine without the VPN")
	}
	// …and removing the VPN brings everything back on direct.
	if err := svc.RemoveVPN(ctx); err != nil {
		t.Fatal(err)
	}
	if svc.currentEngine() == nil {
		t.Fatal("no engine after removing the VPN")
	}
	if n, err := svc.trafficNetwork(ctx); err != nil || n != svc.direct {
		t.Errorf("traffic network = %v, %v; want direct", n, err)
	}
}

func TestTestVPNThroughRealTunnel(t *testing.T) {
	ctx := context.Background()
	server, clientConf := vpntest.Server(t)

	// An IP echo service reachable only inside the tunnel.
	l, err := server.Listen("tcp4", ":80")
	if err != nil {
		t.Fatal(err)
	}
	echo := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		fmt.Fprint(w, host)
	})}
	go func() { _ = echo.Serve(l) }()
	t.Cleanup(func() { echo.Close() })

	base := t.TempDir()
	env := newEnv(t, base, "http://unused", nil, nil)
	env.svc.ipEchoURL = "http://" + vpntest.AddrB.String() + "/"

	res, err := env.svc.TestVPN(ctx, ptr(wgInput(clientConf)))
	if err != nil {
		t.Fatal(err)
	}
	if res.PublicIP != vpntest.AddrA.String() || res.HasDNS {
		t.Errorf("TestVPN = %+v, want public IP %s (the tunnel address)", res, vpntest.AddrA)
	}
	if _, err := env.svc.TestVPN(ctx, ptr(wgInput("[Interface]\n"))); err == nil {
		t.Error("invalid config accepted")
	}
}

func TestGeneralSettingsValidateTMDBKey(t *testing.T) {
	ctx := context.Background()
	tmdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/configuration" && r.URL.Query().Get("api_key") == "good" {
			fmt.Fprint(w, `{}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"status_message":"Invalid API key"}`)
	}))
	defer tmdb.Close()
	base := t.TempDir()
	env := newEnv(t, base, tmdb.URL, basicSetup(base), nil)

	if _, err := env.svc.UpdateGeneralSettings(ctx, GeneralSettingsInput{TMDBAPIKey: "bad"}); err == nil {
		t.Error("rejected TMDB key saved")
	}
	if gs, err := env.svc.UpdateGeneralSettings(ctx, GeneralSettingsInput{TMDBAPIKey: "good"}); err != nil || !gs.TMDBAPIKeySet {
		t.Errorf("update = %+v, %v", gs, err)
	}
	app, _ := env.store.GetAppSettings(ctx)
	if app.TMDBAPIKey != "good" {
		t.Errorf("stored key = %q", app.TMDBAPIKey)
	}
}
