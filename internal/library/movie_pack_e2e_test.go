package library_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
)

func TestMoviePackEndToEnd(t *testing.T) {
	ctx := context.Background()
	magnet := libtest.SeedMoviePack(t)
	idx := libtest.FakeMoviePackIndexer(t, magnet)
	tmdb := libtest.FakeMoviePackTMDB(t)

	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc
	env.SetNow(func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) })
	env.UseSites(t, idx.URL+"/")
	ix, err := svc.Indexers.CreateIndexer(ctx, libtest.SiteIndexer("packs", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Indexers.TestIndexer(ctx, ix.ID); err != nil {
		t.Fatal(err)
	}

	// The second movie is already in the library, monitored; the first is
	// not: the pack is found and grabbed from the discover page.
	ascent, err := svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: 900012})
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.Acquire.Search(ctx, acquire.CollectionTarget(libtest.SkyfarerTMDBID), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Query != libtest.SkyfarerQuery || len(res.Releases) != 3 {
		t.Fatalf("collection search = %+v", res)
	}
	pack := res.Releases[0]
	if pack.Title != libtest.MoviePackName || !pack.Accepted || !pack.MoviePack {
		t.Fatalf("best release = %+v; ranked %+v", pack, res.Releases)
	}
	for _, r := range res.Releases[1:] {
		if r.Accepted {
			t.Errorf("%s accepted, want rejected", r.Title)
		}
	}

	// A pack cannot be grabbed as the single movie it would be imported as.
	var invalid *apperr.ValidationError
	if _, err := svc.Acquire.Grab(ctx, acquire.MovieTarget(ascent.TMDBID), pack); !errors.As(err, &invalid) || invalid.Field != "title" {
		t.Errorf("single-movie grab of a pack: err = %v, want a title validation error", err)
	}

	dl, err := svc.Acquire.Grab(ctx, acquire.CollectionTarget(libtest.SkyfarerTMDBID), pack)
	if err != nil {
		t.Fatal(err)
	}

	// Every released movie is in the library now; the unreleased one is not.
	// Added movies are unmonitored, the one already there is unchanged.
	movies, err := svc.Media.Movies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]media.Movie{}
	for _, m := range movies {
		byTitle[m.Title] = m
	}
	if len(movies) != libtest.SkyfarerMovies {
		t.Fatalf("library = %+v, want the %d released movies", movies, libtest.SkyfarerMovies)
	}
	if byTitle["Skyfarer"].Monitored || byTitle["Skyfarer: Homeward"].Monitored || !byTitle["Skyfarer: Ascent"].Monitored {
		t.Errorf("monitored flags = %+v", byTitle)
	}
	for title, m := range byTitle {
		if m.Status != media.MovieDownloading && m.Monitored {
			t.Errorf("%s status while the pack downloads = %s", title, m.Status)
		}
	}
	// A monitored movie in the pack is not searched for on its own.
	if wanted, _ := env.Store.ListWantedMovies(ctx); len(wanted) != 0 {
		t.Errorf("wanted while the pack downloads = %+v", wanted)
	}

	queue, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
	if len(queue) != 1 || queue[0].ID != dl.ID || queue[0].Title != "Skyfarer" {
		t.Fatalf("queue = %+v", queue)
	}
	wantMovies := []string{"Skyfarer (2011)", "Skyfarer: Ascent (2013)", "Skyfarer: Homeward (2015)"}
	if !slices.Equal(queue[0].Movies, wantMovies) {
		t.Errorf("pack movies = %v, want %v", queue[0].Movies, wantMovies)
	}
	homewardID := byTitle["Skyfarer: Homeward"].ID
	if mine, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{MovieID: &homewardID}); len(mine) != 1 {
		t.Errorf("queue for a movie in the pack = %d rows, want 1", len(mine))
	}

	// The import files the two movies the pack holds, skips the sample and
	// bonus feature, and reports the third movie as missing.
	movie := func(id int64) media.Movie {
		m, err := svc.Media.Movie(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	waitFor(t, svc, "pack import", func() bool {
		q, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
		return len(q) == 1 && q[0].Status == media.DownloadImported
	})
	moviesDir := filepath.Join(base, "Movies")
	for id, file := range map[int64]struct{ src, dst string }{
		byTitle["Skyfarer"].ID: {libtest.MoviePackFirstFile, "Skyfarer (2011)/Skyfarer (2011) [Bluray-1080p].mkv"},
		ascent.ID:              {libtest.MoviePackSecondFile, "Skyfarer- Ascent (2013)/Skyfarer- Ascent (2013) [Bluray-1080p].mkv"},
	} {
		m := movie(id)
		want := filepath.Join(moviesDir, filepath.FromSlash(file.dst))
		if m.FilePath == nil || *m.FilePath != want {
			t.Errorf("%s file = %v, want %s", m.Title, m.FilePath, want)
			continue
		}
		a, errA := os.Stat(want)
		b, errB := os.Stat(filepath.Join(base, "complete", libtest.MoviePackName, file.src))
		if errA != nil || errB != nil || !os.SameFile(a, b) {
			t.Errorf("%s is not a hardlink of the seeding copy (%v, %v)", m.Title, errA, errB)
		}
	}
	if m := movie(homewardID); m.FilePath != nil {
		t.Errorf("Homeward got a file %s, but the pack does not hold it", *m.FilePath)
	}
	queue, _ = svc.Acquire.Queue(ctx, acquire.QueueFilter{})
	if queue[0].Error == nil || !strings.Contains(*queue[0].Error, "Skyfarer: Homeward (2015)") {
		t.Errorf("missing-movie report = %v", queue[0].Error)
	}
}
