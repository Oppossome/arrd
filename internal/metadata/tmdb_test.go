package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTMDB(t *testing.T, key string, h http.HandlerFunc) *TMDB {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &TMDB{APIKey: key, BaseURL: srv.URL, HTTP: srv.Client()}
}

func TestSearchMovies(t *testing.T) {
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/search/movie" || q.Get("query") != "orchard" || q.Get("year") != "1999" || q.Get("api_key") != "v3key" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":900001,"title":"The Glass Orchard","original_title":"The Glass Orchard","release_date":"1999-03-31","genre_ids":[101,102],"original_language":"qq"},{"id":1,"title":"Untitled","release_date":""}]}`))
	})
	got, err := c.SearchMovies(context.Background(), "orchard", 1999)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TMDBID != 900001 || got[0].Year != 1999 || got[1].Year != 0 {
		t.Errorf("got %+v", got)
	}
	if fmt.Sprint(got[0].GenreIDs) != "[101 102]" || got[0].OriginalLanguage != "qq" {
		t.Errorf("genres/language = %v %q", got[0].GenreIDs, got[0].OriginalLanguage)
	}
}

func TestGetMovieBearerAndErrors(t *testing.T) {
	c := newTMDB(t, "eyJtoken", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer eyJtoken" || r.URL.Query().Has("api_key") {
			t.Errorf("v4 token not sent as bearer: %s %v", r.URL, r.Header)
		}
		switch r.URL.Path {
		case "/movie/900001":
			_, _ = w.Write([]byte(`{"id":900001,"imdb_id":"tt9999901","title":"The Glass Orchard","release_date":"1999-03-31"}`))
		case "/movie/2":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status_message":"Invalid API key"}`))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	m, err := c.GetMovie(ctx, 900001)
	if err != nil || m.IMDBID != "tt9999901" {
		t.Fatalf("GetMovie = %+v, %v", m, err)
	}
	if _, err := c.GetMovie(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing movie err = %v", err)
	}
	if _, err := c.GetMovie(ctx, 2); err == nil {
		t.Error("401 not reported")
	}
	if _, err := (&TMDB{}).GetMovie(ctx, 1); err == nil {
		t.Error("missing API key not reported")
	}
}

func TestSeries(t *testing.T) {
	c := newTMDB(t, "k", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/tv":
			if r.URL.Query().Get("first_air_date_year") != "2019" {
				t.Errorf("year not sent: %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":800001,"name":"Harbor Watch","first_air_date":"2019-09-01"}]}`))
		case "/tv/800001":
			if r.URL.Query().Get("append_to_response") != "external_ids" {
				t.Errorf("external ids not requested: %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"id":800001,"name":"Harbor Watch","original_name":"Harbor Watch","first_air_date":"2019-09-01","status":"Returning Series",
				"seasons":[{"season_number":0,"episode_count":1},{"season_number":1,"episode_count":2}],
				"external_ids":{"tvdb_id":7700001,"imdb_id":"tt9999902"}}`))
		case "/tv/800001/season/1":
			_, _ = w.Write([]byte(`{"episodes":[{"season_number":1,"episode_number":1,"name":"Pilot","air_date":"2019-09-01"},{"season_number":1,"episode_number":2,"name":"Tide","air_date":""}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	found, err := c.SearchSeries(ctx, "harbor", 2019)
	if err != nil || len(found) != 1 || found[0].Year != 2019 {
		t.Fatalf("SearchSeries = %+v, %v", found, err)
	}
	s, err := c.GetSeries(ctx, 800001)
	if err != nil || s.TVDBID != 7700001 || s.IMDBID != "tt9999902" || len(s.Seasons) != 2 || s.Seasons[1].EpisodeCount != 2 {
		t.Fatalf("GetSeries = %+v, %v", s, err)
	}
	eps, err := c.GetSeason(ctx, 800001, 1)
	if err != nil || len(eps) != 2 || eps[0].Title != "Pilot" || eps[1].AirDate != "" {
		t.Fatalf("GetSeason = %+v, %v", eps, err)
	}
}
