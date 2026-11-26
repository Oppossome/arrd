package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"generic-arr-project/internal/testutil"
)

func TestCuratedLists(t *testing.T) {
	ctx := context.Background()
	var paths []string
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "/tv/") {
			testutil.Write(t, w, `{"page":1,"total_pages":3,"results":[{"id":800001,"name":"Harbor Watch","first_air_date":"2019-09-01"}]}`)
			return
		}
		testutil.Write(t, w, `{"page":1,"total_pages":3,"results":[{"id":900001,"title":"The Glass Orchard","release_date":"1999-03-31"}]}`)
	})

	for _, l := range []MovieList{MovieTrending, MoviePopular, MovieNowPlaying, MovieUpcoming, MovieTopRated} {
		ms, err := c.MovieList(ctx, l)
		if err != nil || len(ms) != 1 || ms[0].TMDBID != 900001 || ms[0].Year != 1999 {
			t.Errorf("%s: %+v, %v", l, ms, err)
		}
	}
	for _, l := range []SeriesList{SeriesTrending, SeriesPopular, SeriesAiringToday, SeriesOnTheAir, SeriesTopRated} {
		ss, err := c.SeriesList(ctx, l)
		if err != nil || len(ss) != 1 || ss[0].Title != "Harbor Watch" {
			t.Errorf("%s: %+v, %v", l, ss, err)
		}
	}
	want := "[/trending/movie/week /movie/popular /movie/now_playing /movie/upcoming /movie/top_rated " +
		"/trending/tv/week /tv/popular /tv/airing_today /tv/on_the_air /tv/top_rated]"
	if fmt.Sprint(paths) != want {
		t.Errorf("paths = %v", paths)
	}

	if _, err := c.MovieList(ctx, "unknown"); err == nil {
		t.Error("unknown movie list succeeded")
	}
	if _, err := c.SeriesList(ctx, "now_playing"); err == nil {
		t.Error("movie-only list for series succeeded")
	}
}

func TestGenres(t *testing.T) {
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/genre/movie/list":
			testutil.Write(t, w, `{"genres":[{"id":101,"name":"Quiet Drama"},{"id":102,"name":"Loud Comedy"}]}`)
		case "/genre/tv/list":
			testutil.Write(t, w, `{"genres":[{"id":201,"name":"Harbor Mysteries"}]}`)
		default:
			http.NotFound(w, r)
		}
	})
	if g, err := c.MovieGenres(context.Background()); err != nil || fmt.Sprint(g) != "[{101 Quiet Drama} {102 Loud Comedy}]" {
		t.Errorf("movie genres = %v, %v", g, err)
	}
	if g, err := c.SeriesGenres(context.Background()); err != nil || fmt.Sprint(g) != "[{201 Harbor Mysteries}]" {
		t.Errorf("series genres = %v, %v", g, err)
	}
}

func TestLanguages(t *testing.T) {
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configuration/languages" {
			http.NotFound(w, r)
			return
		}
		testutil.Write(t, w, `[{"iso_639_1":"zz","english_name":"Zedish","name":""},{"iso_639_1":"xx","english_name":"No Language","name":""},`+
			`{"iso_639_1":"qq","english_name":"Quenish","name":"Q"},{"iso_639_1":"","english_name":"Blank"}]`)
	})
	got, err := c.Languages(context.Background())
	if err != nil || fmt.Sprint(got) != "[{qq Quenish} {zz Zedish}]" {
		t.Errorf("languages = %v, %v", got, err)
	}
}

func TestDiscover(t *testing.T) {
	ctx := context.Background()
	var last url.Values
	var lastPath string
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		lastPath, last = r.URL.Path, r.URL.Query()
		if r.URL.Path == "/discover/tv" {
			testutil.Write(t, w, `{"page":2,"total_pages":9,"results":[{"id":800001,"name":"Harbor Watch"}]}`)
			return
		}
		testutil.Write(t, w, `{"page":2,"total_pages":38000,"results":[{"id":900001,"title":"The Glass Orchard"},{"id":900002,"title":"Paper Lanterns"}]}`)
	})
	today := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)

	p, err := c.DiscoverMovies(ctx, DiscoverQuery{GenreIDs: []int64{101}, Sort: SortPopular, Page: 2})
	if err != nil || len(p.Results) != 2 || p.Page != 2 || p.TotalPages != MaxDiscoverPage {
		t.Fatalf("page = %+v, %v", p, err)
	}
	if lastPath != "/discover/movie" || last.Get("with_genres") != "101" || last.Get("sort_by") != "popularity.desc" || last.Get("page") != "2" {
		t.Errorf("popular params = %s %v", lastPath, last)
	}

	if _, err := c.DiscoverMovies(ctx, DiscoverQuery{Sort: SortTopRated, Page: 1}); err != nil {
		t.Fatal(err)
	}
	if last.Has("with_genres") || last.Has("with_original_language") || last.Get("sort_by") != "vote_average.desc" || last.Get("vote_count.gte") != "200" {
		t.Errorf("top rated params = %v", last)
	}

	if _, err := c.DiscoverMovies(ctx, DiscoverQuery{Sort: SortNewest, Page: 1, Today: today}); err != nil {
		t.Fatal(err)
	}
	if last.Get("sort_by") != "primary_release_date.desc" || last.Get("primary_release_date.lte") != "2026-11-23" {
		t.Errorf("newest movie params = %v", last)
	}

	s, err := c.DiscoverSeries(ctx, DiscoverQuery{GenreIDs: []int64{201, 202}, OriginalLanguage: "xx", Sort: SortNewest, Page: 2, Today: today})
	if err != nil || len(s.Results) != 1 || s.Results[0].Title != "Harbor Watch" || s.TotalPages != 9 {
		t.Fatalf("series page = %+v, %v", s, err)
	}
	if lastPath != "/discover/tv" || last.Get("sort_by") != "first_air_date.desc" || last.Get("first_air_date.lte") != "2026-11-23" {
		t.Errorf("newest series params = %s %v", lastPath, last)
	}
	if last.Get("with_genres") != "201|202" || last.Get("with_original_language") != "xx" {
		t.Errorf("series filter params = %v", last)
	}

	for _, bad := range []DiscoverQuery{
		{Sort: SortPopular, Page: 0},
		{Sort: SortPopular, Page: MaxDiscoverPage + 1},
		{Sort: "random", Page: 1},
		{Sort: SortNewest, Page: 1}, // no today
	} {
		if _, err := c.DiscoverMovies(ctx, bad); err == nil {
			t.Errorf("%+v succeeded", bad)
		}
	}
}
