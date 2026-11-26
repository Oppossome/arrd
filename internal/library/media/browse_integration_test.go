package media_test

import (
	"context"
	"errors"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/testutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// fakeBrowseTMDB serves genres, /discover and recommendations in front of
// libtest.FakeTMDB (which knows The Glass Orchard, so it can be added). It records
// the query of the last /discover request.
func fakeBrowseTMDB(t *testing.T, lastQuery *url.Values) *httptest.Server {
	t.Helper()
	next := libtest.FakeTMDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/genre/movie/list":
			testutil.Write(t, w, `{"genres":[{"id":101,"name":"Quiet Drama"}]}`)
		case "/genre/tv/list":
			testutil.Write(t, w, `{"genres":[{"id":201,"name":"Harbor Mysteries"}]}`)
		case "/discover/movie":
			*lastQuery = r.URL.Query()
			testutil.Write(t, w, `{"page":3,"total_pages":7,"results":[{"id":900001,"title":"The Glass Orchard"},{"id":900002,"title":"Paper Lanterns"}]}`)
		case "/discover/tv":
			*lastQuery = r.URL.Query()
			testutil.Write(t, w, `{"page":1,"total_pages":1,"results":[{"id":800002,"name":"Lantern Coast"}]}`)
		case "/movie/900001/recommendations":
			testutil.Write(t, w, `{"page":1,"results":[{"id":900001,"title":"The Glass Orchard"},{"id":900010,"title":"Copper Tide"}]}`)
		case "/tv/800002/recommendations":
			testutil.Write(t, w, `{"page":1,"results":[{"id":800003,"name":"Tidewater"}]}`)
		default:
			http.Redirect(w, r, next.URL+r.URL.String(), http.StatusTemporaryRedirect)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBrowseMovies(t *testing.T) {
	ctx := context.Background()
	var query url.Values
	tmdb := fakeBrowseTMDB(t, &query)
	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc
	env.SetNow(func() time.Time { return time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC) })
	movie, err := svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}

	if g, err := svc.Media.MovieGenres(ctx); err != nil || len(g) != 1 || g[0].Name != "Quiet Drama" {
		t.Errorf("genres = %+v, %v", g, err)
	}

	page, err := svc.Media.BrowseMovies(ctx, media.BrowseQuery{TitleFilter: media.TitleFilter{GenreIDs: []int64{101, 102}, OriginalLanguage: "qq"}, Sort: metadata.SortNewest, Page: 3})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 3 || page.TotalPages != 7 || len(page.Items) != 2 {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].LibraryID == nil || *page.Items[0].LibraryID != movie.ID || page.Items[1].LibraryID != nil {
		t.Errorf("library ids = %v, %v", page.Items[0].LibraryID, page.Items[1].LibraryID)
	}
	if query.Get("with_genres") != "101|102" || query.Get("with_original_language") != "qq" || query.Get("page") != "3" || query.Get("primary_release_date.lte") != "2026-11-23" {
		t.Errorf("discover query = %v", query)
	}

	like, err := svc.Media.MoviesLike(ctx, 900001)
	if err != nil || len(like) != 2 || like[0].LibraryID == nil || like[1].TMDBID != 900010 || like[1].LibraryID != nil {
		t.Errorf("like = %+v, %v", like, err)
	}
	if _, err := svc.Media.MoviesLike(ctx, 123); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("like unknown = %v", err)
	}
}

func TestBrowseSeries(t *testing.T) {
	ctx := context.Background()
	var query url.Values
	tmdb := fakeBrowseTMDB(t, &query)
	base := t.TempDir()
	svc := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil).Svc

	if g, err := svc.Media.SeriesGenres(ctx); err != nil || len(g) != 1 || g[0].ID != 201 {
		t.Errorf("genres = %+v, %v", g, err)
	}
	page, err := svc.Media.BrowseSeries(ctx, media.BrowseQuery{Sort: metadata.SortTopRated, Page: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].Title != "Lantern Coast" || page.TotalPages != 1 {
		t.Fatalf("page = %+v, %v", page, err)
	}
	if query.Has("with_genres") || query.Has("with_original_language") || query.Get("sort_by") != "vote_average.desc" {
		t.Errorf("discover query = %v", query)
	}
	like, err := svc.Media.SeriesLike(ctx, 800002)
	if err != nil || len(like) != 1 || like[0].Title != "Tidewater" {
		t.Errorf("like = %+v, %v", like, err)
	}
	if _, err := svc.Media.SeriesLike(ctx, 123); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("like unknown = %v", err)
	}
}
