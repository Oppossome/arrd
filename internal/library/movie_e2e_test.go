package library_test

import (
	"context"
	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMovieEndToEnd(t *testing.T) {
	ctx := context.Background()
	magnet := libtest.SeedMovie(t)
	idx := libtest.FakeIndexer(t, magnet)
	tmdb := libtest.FakeTMDB(t)

	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc
	jellyfin := libtest.AddJellyfin(t, svc)

	env.UseSites(t, idx.URL+"/", "http://127.0.0.1:1/")
	good, err := svc.Indexers.CreateIndexer(ctx, libtest.SiteIndexer("good", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Indexers.TestIndexer(ctx, good.ID); err != nil {
		t.Fatalf("test indexer: %v", err)
	}
	broken, err := svc.Indexers.CreateIndexer(ctx, libtest.OnSite(libtest.SiteIndexer("broken", "http://127.0.0.1:1/"), libtest.OtherSiteID))
	if err != nil {
		t.Fatal(err)
	}

	movie, err := svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	if movie.Status != media.MovieMissing || movie.IMDBID == nil || *movie.IMDBID != "tt9999901" {
		t.Fatalf("added movie = %+v", movie)
	}

	// Manual search shows the ranking and the broken indexer's failure.
	res, err := svc.Acquire.SearchMovie(ctx, movie.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Releases) != 2 || !res.Releases[0].Accepted || res.Releases[0].Title != libtest.ReleaseName || res.Releases[1].Accepted {
		t.Fatalf("releases = %+v", res.Releases)
	}
	if len(res.Failures) != 1 || res.Failures[0].IndexerID != broken.ID {
		t.Fatalf("failures = %+v", res.Failures)
	}
	if ix, _ := svc.Indexers.Indexer(ctx, broken.ID); ix.LastError == nil {
		t.Error("broken indexer's error was not recorded")
	}

	auto, err := svc.Acquire.AutoSearchMovie(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if auto.Grabbed == nil || auto.Grabbed.Quality != "Bluray-1080p" {
		t.Fatalf("auto search = %+v", auto)
	}
	if m, _ := svc.Media.Movie(ctx, movie.ID); m.Status != media.MovieDownloading {
		t.Errorf("status while downloading = %s", m.Status)
	}
	if _, err := svc.Acquire.GrabMovieRelease(ctx, movie.ID, res.Releases[0]); err == nil {
		t.Error("grabbing the same torrent twice succeeded")
	}
	// A settings change mid-download restarts the engine; the download resumes.
	if err := svc.Engine.Apply(ctx); err != nil {
		t.Fatal(err)
	}

	// Nothing imported yet: media servers are not told.
	libtest.WantRescans(t, svc, jellyfin, 0)

	// Drive the scheduler's work until the movie is imported.
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := svc.Acquire.ProcessDownloads(ctx); err != nil {
			t.Fatal(err)
		}
		m, err := svc.Media.Movie(ctx, movie.ID)
		if err != nil {
			t.Fatal(err)
		}
		if m.FilePath != nil {
			movie = m
			break
		}
		if time.Now().After(deadline) {
			q, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
			t.Fatalf("not imported in time; queue = %+v", q)
		}
		time.Sleep(100 * time.Millisecond)
	}

	wantFile := filepath.Join(base, "Movies", "The Glass Orchard (1999)", "The Glass Orchard (1999) [Bluray-1080p].mkv")
	if movie.Status != media.MovieDownloaded || *movie.FilePath != wantFile {
		t.Fatalf("imported movie = %+v", movie)
	}
	libtest.WantRescans(t, svc, jellyfin, 1)
	libInfo, err := os.Stat(wantFile)
	if err != nil {
		t.Fatal(err)
	}
	seedInfo, err := os.Stat(filepath.Join(base, "complete", libtest.ReleaseName, "movie.mkv"))
	if err != nil {
		t.Fatalf("download not moved to complete dir: %v", err)
	}
	if !os.SameFile(libInfo, seedInfo) {
		t.Error("library file is not a hardlink of the seeding copy")
	}
	if _, err := os.Stat(filepath.Join(base, "incomplete", libtest.ReleaseName)); !os.IsNotExist(err) {
		t.Errorf("incomplete dir still has the download: %v", err)
	}

	queue, err := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].Status != media.DownloadImported || queue[0].Progress == nil || !queue[0].Progress.Complete {
		t.Fatalf("queue = %+v", queue)
	}
	if queue[0].Title != "The Glass Orchard" || queue[0].Year != 1999 {
		t.Errorf("queue title = %q (%d)", queue[0].Title, queue[0].Year)
	}
	if mine, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{MovieID: &movie.ID}); len(mine) != 1 {
		t.Errorf("movie filter = %d rows", len(mine))
	}
	other := movie.ID + 100
	if none, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{MovieID: &other}); len(none) != 0 {
		t.Errorf("filter for another movie = %d rows", len(none))
	}
	row, _ := env.Store.GetDownload(ctx, queue[0].ID)
	if row.Metainfo == nil {
		t.Error("metainfo was not saved for the magnet download")
	}
	if wanted, _ := env.Store.ListWantedMovies(ctx); len(wanted) != 0 {
		t.Errorf("movie still wanted after import: %+v", wanted)
	}

	// Restart: a fresh engine restores the torrent from complete_dir and
	// keeps seeding without re-downloading.
	env.Svc.Stop()
	env2 := libtest.NewEnv(t, base, tmdb.URL, nil, env.Store)
	dl, ok := env2.Svc.Engine.CurrentEngine().Get(queue[0].InfoHash)
	if !ok || dl.Dir() != filepath.Join(base, "complete") {
		t.Fatalf("restored download = %v, %v", dl, ok)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dl.WaitComplete(waitCtx); err != nil {
		t.Fatalf("restored torrent not complete: %v", err)
	}
}
