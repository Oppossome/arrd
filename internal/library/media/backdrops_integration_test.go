package media_test

import (
	"context"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"testing"
)

// fakeTMDBWithArt serves movie 900001 with poster and backdrop images.
func TestBackfillBackdrops(t *testing.T) {
	ctx := context.Background()
	backdrop := ""
	tmdb := libtest.FakeTMDBWithArt(t, &backdrop)
	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)

	movie, err := env.Svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	if movie.BackdropPath != nil {
		t.Fatalf("backdrop = %q before TMDB had one", *movie.BackdropPath)
	}
	// TMDB still has none: nothing changes and nothing fails.
	if err := env.Svc.Media.BackfillBackdrops(ctx); err != nil {
		t.Fatal(err)
	}

	backdrop = "/orchardbackdrop.jpg"
	if err := env.Svc.Media.BackfillBackdrops(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := env.Svc.Media.Movie(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BackdropPath == nil || *got.BackdropPath != backdrop {
		t.Errorf("backdrop after backfill = %v, want %s", got.BackdropPath, backdrop)
	}
	if rest, _ := env.Store.ListMoviesMissingBackdrop(ctx); len(rest) != 0 {
		t.Errorf("movies still missing a backdrop: %d", len(rest))
	}
}
