package media_test

import (
	"context"
	"fmt"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withTMDBRoutes serves fixed JSON bodies by path in front of another fake TMDB.
func withTMDBRoutes(t *testing.T, next *httptest.Server, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := routes[r.URL.Path]; ok {
			testutil.Write(t, w, body)
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
	tmdb := withTMDBRoutes(t, libtest.FakeTMDB(t), map[string]string{
		"/movie/900003": `{"id":900003,"title":"Salt Roads","original_title":"Salt Roads","release_date":"2031-05-01"}`,
		// Salt Roads (added last) recommends a movie in the library and two new ones.
		"/movie/900003/recommendations": `{"page":1,"results":[{"id":900001,"title":"The Glass Orchard"},
			{"id":900010,"title":"Copper Tide","release_date":"2030-01-01"},{"id":900011,"title":"Moth Season"}]}`,
		// The Glass Orchard repeats one of those.
		"/movie/900001/recommendations": `{"page":1,"results":[{"id":900011,"title":"Moth Season"},{"id":900012,"title":"Quiet Engines"}]}`,
	})
	base := t.TempDir()
	svc := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil).Svc

	if rows, err := svc.Media.MovieRecommendations(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("empty library: rows = %+v, %v", rows, err)
	}

	for _, id := range []int64{900001, 900003} {
		if _, err := svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: id}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := svc.Media.MovieRecommendations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if got := rows[0]; got.Because != (media.RecommendationSource{TMDBID: 900003, Title: "Salt Roads"}) || fmt.Sprint(movieIDs(got.Items)) != "[900010 900011]" || got.Items[0].Year != 2030 {
		t.Errorf("first row = %+v", got)
	}
	if got := rows[1]; got.Because != (media.RecommendationSource{TMDBID: 900001, Title: "The Glass Orchard"}) || fmt.Sprint(movieIDs(got.Items)) != "[900012]" {
		t.Errorf("second row = %+v", got)
	}
}

func TestSeriesRecommendations(t *testing.T) {
	ctx := context.Background()
	tmdb := withTMDBRoutes(t, libtest.FakeTVTMDB(t), map[string]string{
		"/tv/800001/recommendations": `{"page":1,"results":[{"id":800001,"name":"Harbor Watch"},{"id":800002,"name":"Lantern Coast","first_air_date":"2028-02-02"}]}`,
	})
	base := t.TempDir()
	svc := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil).Svc
	if _, err := svc.Media.AddSeries(ctx, media.AddSeriesInput{TMDBID: 800001, Monitor: media.MonitorNone}); err != nil {
		t.Fatal(err)
	}

	rows, err := svc.Media.SeriesRecommendations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Because.Title != "Harbor Watch" || len(rows[0].Items) != 1 ||
		rows[0].Items[0].TMDBID != 800002 || rows[0].Items[0].Title != "Lantern Coast" || rows[0].Items[0].Year != 2028 {
		t.Errorf("rows = %+v", rows)
	}
}
