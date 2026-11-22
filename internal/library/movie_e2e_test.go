package library

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"

	"generic-arr-project/internal/torrent"
)

const releaseName = "The.Glass.Orchard.1999.1080p.BluRay.x264-GRP"

// seedMovie creates a 60 MiB sparse "movie" plus a sample and an .nfo,
// starts a seeder for it, and returns its magnet link pointing at the seeder.
func seedMovie(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, releaseName)
	files := map[string]int64{"movie.mkv": 60 << 20, "Sample/sample.mkv": 1 << 20, "release.nfo": 512}
	for name, size := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	info := metainfo.Info{PieceLength: 1 << 20}
	if err := info.BuildFromFilePath(root); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: infoBytes}

	seeder, err := torrent.New(torrent.Config{StateDir: t.TempDir(), Network: v4Only{}, Seed: true}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(seeder.Close)
	dl, err := seeder.AddMetaInfo(mi, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := dl.WaitComplete(ctx); err != nil {
		t.Fatalf("seeder: %v", err)
	}
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&x.pe=%s",
		dl.InfoHash(), url.QueryEscape(releaseName), url.QueryEscape("127.0.0.1:"+strconv.Itoa(seeder.ListenPort())))
}

// fakeIndexer serves the test site with a movie search returning one good
// release (whose download link redirects to magnet) and one CAM release.
func fakeIndexer(t *testing.T, magnet string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = fakeSite(t, func(q url.Values) []siteRow {
		if q.Get("t") != "movie" {
			return nil // the test's query
		}
		if q.Get("imdbid") != "9999901" {
			t.Errorf("site searched without imdb id: %s", q.Encode())
		}
		return []siteRow{
			{Title: "The.Glass.Orchard.1999.HDCAM-BAD", Link: srv.URL + "/dl/cam", Size: 1000, Seeders: 900},
			{Title: releaseName, Link: srv.URL + "/dl/good", Size: 62914560, Seeders: 12},
		}
	}, map[string]http.HandlerFunc{"/dl/good": redirectTo(magnet)})
	return srv
}

func fakeTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/900001" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"id":900001,"imdb_id":"tt9999901","title":"The Glass Orchard","original_title":"The Glass Orchard","release_date":"1999-03-31"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMovieEndToEnd(t *testing.T) {
	ctx := context.Background()
	magnet := seedMovie(t)
	idx := fakeIndexer(t, magnet)
	tmdb := fakeTMDB(t)

	base := t.TempDir()
	env := newEnv(t, base, tmdb.URL, basicSetup(base), nil)
	svc := env.svc

	env.useSites(t, idx.URL+"/", "http://127.0.0.1:1/")
	good, err := svc.CreateIndexer(ctx, siteIndexer("good", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestIndexer(ctx, good.ID); err != nil {
		t.Fatalf("test indexer: %v", err)
	}
	broken, err := svc.CreateIndexer(ctx, onSite(siteIndexer("broken", "http://127.0.0.1:1/"), otherSiteID))
	if err != nil {
		t.Fatal(err)
	}

	movie, err := svc.AddMovie(ctx, AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	if movie.Status != MovieMissing || movie.IMDBID == nil || *movie.IMDBID != "tt9999901" {
		t.Fatalf("added movie = %+v", movie)
	}

	// Manual search shows the ranking and the broken indexer's failure.
	res, err := svc.SearchMovie(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Releases) != 2 || !res.Releases[0].Accepted || res.Releases[0].Title != releaseName || res.Releases[1].Accepted {
		t.Fatalf("releases = %+v", res.Releases)
	}
	if len(res.Failures) != 1 || res.Failures[0].IndexerID != broken.ID {
		t.Fatalf("failures = %+v", res.Failures)
	}
	if ix, _ := svc.Indexer(ctx, broken.ID); ix.LastError == nil {
		t.Error("broken indexer's error was not recorded")
	}

	auto, err := svc.AutoSearchMovie(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if auto.Grabbed == nil || auto.Grabbed.Quality != "Bluray-1080p" {
		t.Fatalf("auto search = %+v", auto)
	}
	if m, _ := svc.Movie(ctx, movie.ID); m.Status != MovieDownloading {
		t.Errorf("status while downloading = %s", m.Status)
	}
	if _, err := svc.GrabMovieRelease(ctx, movie.ID, res.Releases[0]); err == nil {
		t.Error("grabbing the same torrent twice succeeded")
	}
	// A settings change mid-download restarts the engine; the download resumes.
	if err := svc.ApplyRuntime(ctx); err != nil {
		t.Fatal(err)
	}

	// Drive the scheduler's work until the movie is imported.
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := svc.ProcessDownloads(ctx); err != nil {
			t.Fatal(err)
		}
		m, err := svc.Movie(ctx, movie.ID)
		if err != nil {
			t.Fatal(err)
		}
		if m.FilePath != nil {
			movie = m
			break
		}
		if time.Now().After(deadline) {
			q, _ := svc.Queue(ctx, QueueFilter{})
			t.Fatalf("not imported in time; queue = %+v", q)
		}
		time.Sleep(100 * time.Millisecond)
	}

	wantFile := filepath.Join(base, "Movies", "The Glass Orchard (1999)", "The Glass Orchard (1999) [Bluray-1080p].mkv")
	if movie.Status != MovieDownloaded || *movie.FilePath != wantFile {
		t.Fatalf("imported movie = %+v", movie)
	}
	libInfo, err := os.Stat(wantFile)
	if err != nil {
		t.Fatal(err)
	}
	seedInfo, err := os.Stat(filepath.Join(base, "complete", releaseName, "movie.mkv"))
	if err != nil {
		t.Fatalf("download not moved to complete dir: %v", err)
	}
	if !os.SameFile(libInfo, seedInfo) {
		t.Error("library file is not a hardlink of the seeding copy")
	}
	if _, err := os.Stat(filepath.Join(base, "incomplete", releaseName)); !os.IsNotExist(err) {
		t.Errorf("incomplete dir still has the download: %v", err)
	}

	queue, err := svc.Queue(ctx, QueueFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].Status != statusImported || queue[0].Progress == nil || !queue[0].Progress.Complete {
		t.Fatalf("queue = %+v", queue)
	}
	if queue[0].Title != "The Glass Orchard" || queue[0].Year != 1999 {
		t.Errorf("queue title = %q (%d)", queue[0].Title, queue[0].Year)
	}
	if mine, _ := svc.Queue(ctx, QueueFilter{MovieID: &movie.ID}); len(mine) != 1 {
		t.Errorf("movie filter = %d rows", len(mine))
	}
	other := movie.ID + 100
	if none, _ := svc.Queue(ctx, QueueFilter{MovieID: &other}); len(none) != 0 {
		t.Errorf("filter for another movie = %d rows", len(none))
	}
	row, _ := env.store.GetDownload(ctx, queue[0].ID)
	if row.Metainfo == nil {
		t.Error("metainfo was not saved for the magnet download")
	}
	if wanted, _ := env.store.ListWantedMovies(ctx); len(wanted) != 0 {
		t.Errorf("movie still wanted after import: %+v", wanted)
	}

	// Restart: a fresh engine restores the torrent from complete_dir and
	// keeps seeding without re-downloading.
	env.svc.StopRuntime()
	env2 := newEnv(t, base, tmdb.URL, nil, env.store)
	dl, ok := env2.svc.currentEngine().Get(queue[0].InfoHash)
	if !ok || dl.Dir() != filepath.Join(base, "complete") {
		t.Fatalf("restored download = %v, %v", dl, ok)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dl.WaitComplete(waitCtx); err != nil {
		t.Fatalf("restored torrent not complete: %v", err)
	}
}
