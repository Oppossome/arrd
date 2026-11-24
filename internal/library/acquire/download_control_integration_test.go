package acquire_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
)

func TestPauseResumeDownload(t *testing.T) {
	ctx := context.Background()
	magnet := libtest.SeedMovie(t)
	idx := libtest.FakeIndexer(t, magnet)
	backdrop := "/orchardbackdrop.jpg"
	tmdb := libtest.FakeTMDBWithArt(t, &backdrop)

	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc
	env.UseSites(t, idx.URL+"/")
	good, err := svc.Indexers.CreateIndexer(ctx, libtest.SiteIndexer("good", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Indexers.TestIndexer(ctx, good.ID); err != nil {
		t.Fatalf("test indexer: %v", err)
	}
	movie, err := svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	if movie.BackdropPath == nil || *movie.BackdropPath != backdrop {
		t.Errorf("movie backdrop = %v, want %s", movie.BackdropPath, backdrop)
	}
	auto, err := svc.Acquire.AutoSearchMovie(ctx, movie.ID)
	if err != nil || auto.Grabbed == nil {
		t.Fatalf("auto search = %+v, %v", auto, err)
	}
	id := auto.Grabbed.ID
	if err := svc.Acquire.PauseDownload(ctx, id); err != nil {
		t.Fatal(err)
	}

	queue, err := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
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
	if tp, err := svc.Acquire.Throughput(ctx); err != nil || tp.Active != 1 {
		t.Errorf("throughput = %+v, %v; want 1 active", tp, err)
	}

	// The pause survives a restart.
	svc.Stop()
	env = libtest.NewEnv(t, base, tmdb.URL, nil, env.Store)
	svc = env.Svc
	dl, ok := svc.Engine.CurrentEngine().Get(d.InfoHash)
	if !ok || !dl.Paused() {
		t.Fatalf("restored download = %v, paused %v", ok, ok && dl.Paused())
	}

	if err := svc.Acquire.ResumeDownload(ctx, id); err != nil {
		t.Fatal(err)
	}
	if dl.Paused() {
		t.Fatal("engine download still paused after resume")
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := svc.Acquire.ProcessDownloads(ctx); err != nil {
			t.Fatal(err)
		}
		if m, _ := svc.Media.Movie(ctx, movie.ID); m.FilePath != nil {
			break
		}
		if time.Now().After(deadline) {
			q, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
			t.Fatalf("not imported after resume; queue = %+v", q)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Finished downloads cannot be paused.
	var conflictErr *apperr.ConflictError
	if err := svc.Acquire.PauseDownload(ctx, id); !errors.As(err, &conflictErr) {
		t.Errorf("pause imported download: err = %v, want conflict", err)
	}
	if tp, err := svc.Acquire.Throughput(ctx); err != nil || tp.Active != 0 {
		t.Errorf("throughput after import = %+v, %v; want 0 active", tp, err)
	}
	if err := svc.Acquire.PauseDownload(ctx, id+100); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("pause unknown download: err = %v, want not found", err)
	}
	if _, err := svc.Acquire.DownloadPeers(ctx, id+100); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("peers of unknown download: err = %v, want not found", err)
	}
	if _, err := svc.Acquire.DownloadPeers(ctx, id); err != nil {
		t.Errorf("peers of imported download: %v", err)
	}
}
