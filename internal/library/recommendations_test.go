package library

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"generic-arr-project/internal/metadata"
)

// withTMDBRoutes serves fixed JSON bodies by path in front of another fake TMDB.
func withTMDBRoutes(t *testing.T, next *httptest.Server, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := routes[r.URL.Path]; ok {
			fmt.Fprint(w, body)
			return
		}
		http.Redirect(w, r, next.URL+r.URL.String(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func movieIDs(ms []metadata.Movie) []int64 {
	out := make([]int64, len(ms))
	for i, m := range ms {
		out[i] = m.TMDBID
	}
	return out
}

func TestMovieRecommendations(t *testing.T) {
	ctx := context.Background()
	tmdb := withTMDBRoutes(t, fakeTMDB(t), map[string]string{
		"/movie/900003": `{"id":900003,"title":"Salt Roads","original_title":"Salt Roads","release_date":"2031-05-01"}`,
		// Salt Roads (added last) recommends a movie in the library and two new ones.
		"/movie/900003/recommendations": `{"page":1,"results":[{"id":900001,"title":"The Glass Orchard"},
			{"id":900010,"title":"Copper Tide","release_date":"2030-01-01"},{"id":900011,"title":"Moth Season"}]}`,
		// The Glass Orchard repeats one of those.
		"/movie/900001/recommendations": `{"page":1,"results":[{"id":900011,"title":"Moth Season"},{"id":900012,"title":"Quiet Engines"}]}`,
	})
	base := t.TempDir()
	svc := newEnv(t, base, tmdb.URL, basicSetup(base), nil).svc

	if rows, err := svc.MovieRecommendations(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("empty library: rows = %+v, %v", rows, err)
	}

	for _, id := range []int64{900001, 900003} {
		if _, err := svc.AddMovie(ctx, AddMovieInput{TMDBID: id}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := svc.MovieRecommendations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if got := rows[0]; got.Because != (RecommendationSource{900003, "Salt Roads"}) || fmt.Sprint(movieIDs(got.Items)) != "[900010 900011]" || got.Items[0].Year != 2030 {
		t.Errorf("first row = %+v", got)
	}
	if got := rows[1]; got.Because != (RecommendationSource{900001, "The Glass Orchard"}) || fmt.Sprint(movieIDs(got.Items)) != "[900012]" {
		t.Errorf("second row = %+v", got)
	}
}

func TestSeriesRecommendations(t *testing.T) {
	ctx := context.Background()
	tmdb := withTMDBRoutes(t, fakeTVTMDB(t), map[string]string{
		"/tv/800001/recommendations": `{"page":1,"results":[{"id":800001,"name":"Harbor Watch"},{"id":800002,"name":"Lantern Coast","first_air_date":"2028-02-02"}]}`,
	})
	base := t.TempDir()
	svc := newEnv(t, base, tmdb.URL, basicSetup(base), nil).svc
	if _, err := svc.AddSeries(ctx, AddSeriesInput{TMDBID: 800001, Monitor: MonitorNone}); err != nil {
		t.Fatal(err)
	}

	rows, err := svc.SeriesRecommendations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Because.Title != "Harbor Watch" || len(rows[0].Items) != 1 ||
		rows[0].Items[0].TMDBID != 800002 || rows[0].Items[0].Title != "Lantern Coast" || rows[0].Items[0].Year != 2028 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestRecommendSkipsUnknownAndFailsOnErrors(t *testing.T) {
	ctx := context.Background()
	sources := []RecommendationSource{{1, "Gone"}, {2, "Kept"}, {3, "Nothing new"}}
	idOf := func(id int64) int64 { return id }
	fetch := func(_ context.Context, id int64) ([]int64, error) {
		switch id {
		case 1:
			return nil, metadata.ErrNotFound
		case 2:
			return []int64{10, 11}, nil
		default:
			return []int64{11}, nil
		}
	}
	rows, err := recommend(ctx, sources, map[int64]bool{}, fetch, idOf)
	if err != nil || len(rows) != 1 || rows[0].Because.Title != "Kept" || fmt.Sprint(rows[0].Items) != "[10 11]" {
		t.Errorf("rows = %+v, %v", rows, err)
	}

	boom := errors.New("boom")
	failing := func(_ context.Context, id int64) ([]int64, error) {
		if id == 2 {
			return nil, boom
		}
		return nil, nil
	}
	if _, err := recommend(ctx, sources, nil, failing, idOf); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestNewestSeeds(t *testing.T) {
	seeds := []seed{
		{RecommendationSource{1, "Old"}, "2026-01-01T00:00:00Z", 1},
		{RecommendationSource{2, "Same second, lower id"}, "2026-03-01T00:00:00Z", 2},
		{RecommendationSource{3, "Same second, higher id"}, "2026-03-01T00:00:00Z", 3},
	}
	got := newestSeeds(seeds)
	if len(got) != 3 || got[0].TMDBID != 3 || got[1].TMDBID != 2 || got[2].TMDBID != 1 {
		t.Errorf("order = %+v", got)
	}

	many := make([]seed, recommendationSeeds+3)
	for i := range many {
		many[i] = seed{RecommendationSource{TMDBID: int64(i)}, "2026-01-01T00:00:00Z", int64(i)}
	}
	if got := newestSeeds(many); len(got) != recommendationSeeds {
		t.Errorf("len = %d, want %d", len(got), recommendationSeeds)
	}
}
