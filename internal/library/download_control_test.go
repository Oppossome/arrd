package library

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"testing"
	"time"

	"generic-arr-project/internal/torrent"
)

// fakeTMDBWithArt serves movie 900001 with poster and backdrop images.
func fakeTMDBWithArt(t *testing.T, backdrop *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/900001" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"id":900001,"imdb_id":"tt9999901","title":"The Glass Orchard","original_title":"The Glass Orchard",`+
			`"release_date":"1999-03-31","poster_path":"/orchardposter.jpg","backdrop_path":%q}`, *backdrop)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPauseResumeDownload(t *testing.T) {
	ctx := context.Background()
	magnet := seedMovie(t)
	idx := fakeIndexer(t, magnet)
	backdrop := "/orchardbackdrop.jpg"
	tmdb := fakeTMDBWithArt(t, &backdrop)

	base := t.TempDir()
	env := newEnv(t, base, tmdb.URL, basicSetup(base), nil)
	svc := env.svc
	env.useSites(t, idx.URL+"/")
	good, err := svc.CreateIndexer(ctx, siteIndexer("good", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestIndexer(ctx, good.ID); err != nil {
		t.Fatalf("test indexer: %v", err)
	}
	movie, err := svc.AddMovie(ctx, AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	if movie.BackdropPath == nil || *movie.BackdropPath != backdrop {
		t.Errorf("movie backdrop = %v, want %s", movie.BackdropPath, backdrop)
	}
	auto, err := svc.AutoSearchMovie(ctx, movie.ID)
	if err != nil || auto.Grabbed == nil {
		t.Fatalf("auto search = %+v, %v", auto, err)
	}
	id := auto.Grabbed.ID
	if err := svc.PauseDownload(ctx, id); err != nil {
		t.Fatal(err)
	}

	queue, err := svc.Queue(ctx, QueueFilter{})
	if err != nil {
		t.Fatal(err)
	}
	d := queue[0]
	if !d.Paused || d.Progress == nil || !d.Progress.Paused {
		t.Fatalf("queue after pause = %+v", d)
	}
	if d.PosterPath == nil || *d.PosterPath != "/orchardposter.jpg" || d.BackdropPath == nil || *d.BackdropPath != backdrop {
		t.Errorf("queue images = %v, %v", d.PosterPath, d.BackdropPath)
	}
	if tp, err := svc.Throughput(ctx); err != nil || tp.Active != 1 {
		t.Errorf("throughput = %+v, %v; want 1 active", tp, err)
	}

	// The pause survives a restart.
	svc.StopRuntime()
	env = newEnv(t, base, tmdb.URL, nil, env.store)
	svc = env.svc
	dl, ok := svc.currentEngine().Get(d.InfoHash)
	if !ok || !dl.Paused() {
		t.Fatalf("restored download = %v, paused %v", ok, ok && dl.Paused())
	}

	if err := svc.ResumeDownload(ctx, id); err != nil {
		t.Fatal(err)
	}
	if dl.Paused() {
		t.Fatal("engine download still paused after resume")
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := svc.ProcessDownloads(ctx); err != nil {
			t.Fatal(err)
		}
		if m, _ := svc.Movie(ctx, movie.ID); m.FilePath != nil {
			break
		}
		if time.Now().After(deadline) {
			q, _ := svc.Queue(ctx, QueueFilter{})
			t.Fatalf("not imported after resume; queue = %+v", q)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Finished downloads cannot be paused.
	var conflictErr *ConflictError
	if err := svc.PauseDownload(ctx, id); !errors.As(err, &conflictErr) {
		t.Errorf("pause imported download: err = %v, want conflict", err)
	}
	if tp, err := svc.Throughput(ctx); err != nil || tp.Active != 0 {
		t.Errorf("throughput after import = %+v, %v; want 0 active", tp, err)
	}
	if err := svc.PauseDownload(ctx, id+100); !errors.Is(err, ErrNotFound) {
		t.Errorf("pause unknown download: err = %v, want not found", err)
	}
	if _, err := svc.DownloadPeers(ctx, id+100); !errors.Is(err, ErrNotFound) {
		t.Errorf("peers of unknown download: err = %v, want not found", err)
	}
	if _, err := svc.DownloadPeers(ctx, id); err != nil {
		t.Errorf("peers of imported download: %v", err)
	}
}

func TestBackfillBackdrops(t *testing.T) {
	ctx := context.Background()
	backdrop := ""
	tmdb := fakeTMDBWithArt(t, &backdrop)
	base := t.TempDir()
	env := newEnv(t, base, tmdb.URL, basicSetup(base), nil)

	movie, err := env.svc.AddMovie(ctx, AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	if movie.BackdropPath != nil {
		t.Fatalf("backdrop = %q before TMDB had one", *movie.BackdropPath)
	}
	// TMDB still has none: nothing changes and nothing fails.
	if err := env.svc.BackfillBackdrops(ctx); err != nil {
		t.Fatal(err)
	}

	backdrop = "/orchardbackdrop.jpg"
	if err := env.svc.BackfillBackdrops(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := env.svc.Movie(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BackdropPath == nil || *got.BackdropPath != backdrop {
		t.Errorf("backdrop after backfill = %v, want %s", got.BackdropPath, backdrop)
	}
	if rest, _ := env.store.ListMoviesMissingBackdrop(ctx); len(rest) != 0 {
		t.Errorf("movies still missing a backdrop: %d", len(rest))
	}
}

func TestCountPeerCountries(t *testing.T) {
	peer := func(ip, country string) Peer {
		return Peer{Peer: torrent.Peer{Addr: netip.MustParseAddr(ip)}, Country: country}
	}
	got := countPeerCountries([]Peer{
		peer("198.51.100.1", "DE"),
		peer("198.51.100.2", "NZ"),
		peer("198.51.100.3", "DE"),
		peer("10.0.0.1", ""),
		peer("198.51.100.4", "AT"),
	})
	want := []CountryCount{{"DE", 2}, {"", 1}, {"AT", 1}, {"NZ", 1}}
	if !slices.Equal(got, want) {
		t.Errorf("countPeerCountries = %v, want %v", got, want)
	}
	if got := countPeerCountries(nil); len(got) != 0 {
		t.Errorf("no peers = %v", got)
	}
}
