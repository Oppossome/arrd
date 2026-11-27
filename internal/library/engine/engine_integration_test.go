package engine_test

import (
	"context"
	"errors"
	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/store/db"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// With a VPN configured, downloads and indexers never fall back to direct
// while the tunnel is down; removing the VPN brings both back on direct.
func TestTrafficFailsClosedWhileVPNDown(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, "http://unused", libtest.BasicSetup(base), nil)
	svc := env.Svc

	var hits atomic.Int32
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(site.Close)
	env.UseSites(t, site.URL+"/")
	ix, err := svc.Indexers.CreateIndexer(ctx, libtest.SiteIndexer("site", site.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}

	env.Factory.VPNErr = errors.New("handshake timed out")
	if _, err := svc.VPN.SetVPN(ctx, libtest.WGInput(libtest.TestWG)); err != nil {
		t.Fatal(err)
	}
	if svc.Engine.CurrentEngine() != nil {
		t.Fatal("engine running although its VPN is down")
	}
	if vs, _ := svc.VPN.VPNSettings(ctx); !vs.Configured || vs.TunnelUp || !strings.Contains(vs.TunnelError, "handshake") {
		t.Errorf("vpn settings = %+v", vs)
	}
	root, err := svc.Settings.DefaultRootFolder(ctx, settings.MediaMovie)
	if err != nil {
		t.Fatal(err)
	}
	movie, err := env.Store.CreateMovie(ctx, db.CreateMovieParams{TMDBID: 1, Title: "Fictional Film", Year: 2001, RootFolderID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Acquire.Grab(ctx, acquire.MovieTarget(movie.TMDBID), acquire.ReleaseResult{Title: "x", IndexerID: ix.ID, MagnetURI: "magnet:?xt=urn:btih:0000000000000000000000000000000000000000"}); err == nil {
		t.Error("grab succeeded without an engine")
	}
	if _, err := svc.Indexers.TestIndexer(ctx, ix.ID); err == nil || !strings.Contains(err.Error(), "vpn is down") {
		t.Errorf("indexer test while the VPN is down = %v, want vpn is down", err)
	}
	if _, failures, err := svc.Indexers.Search(ctx, func(indexers.Indexer) (indexer.Query, bool) {
		return indexer.Query{Type: indexer.SearchGeneric, Q: "anything"}, true
	}); err != nil || len(failures) != 1 || !strings.Contains(failures[0].Error, "vpn is down") {
		t.Errorf("search while the VPN is down: failures %+v, err %v", failures, err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the site was reached %d times while the VPN was down", n)
	}

	// The scheduler's retry keeps failing closed while the tunnel is down…
	svc.Engine.Retry(ctx)
	if svc.Engine.CurrentEngine() != nil {
		t.Fatal("retry started an engine without the VPN")
	}
	// …and removing the VPN brings everything back on direct.
	if err := svc.VPN.RemoveVPN(ctx); err != nil {
		t.Fatal(err)
	}
	if svc.Engine.CurrentEngine() == nil {
		t.Fatal("no engine after removing the VPN")
	}
	if n, err := svc.Engine.TrafficNetwork(ctx); err != nil || n != svc.Engine.Direct() {
		t.Errorf("traffic network = %v, %v; want direct", n, err)
	}
}
