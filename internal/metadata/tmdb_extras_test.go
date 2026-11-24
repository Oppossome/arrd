package metadata

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestPickTrailer(t *testing.T) {
	tests := []struct {
		name   string
		videos []tmdbVideo
		want   string // key; "" for none
	}{
		{"none", nil, ""},
		{"only non-youtube", []tmdbVideo{{Key: "v1", Site: "Vimeo", Type: "Trailer", Official: true}}, ""},
		{"only featurette", []tmdbVideo{{Key: "f1", Site: "YouTube", Type: "Featurette"}}, ""},
		{"teaser fallback", []tmdbVideo{{Key: "t1", Site: "YouTube", Type: "Teaser"}}, "t1"},
		{"trailer over teaser", []tmdbVideo{
			{Key: "t1", Site: "YouTube", Type: "Teaser", Official: true},
			{Key: "tr1", Site: "YouTube", Type: "Trailer"},
		}, "tr1"},
		{"official trailer first", []tmdbVideo{
			{Key: "tr1", Site: "YouTube", Type: "Trailer"},
			{Key: "tr2", Site: "YouTube", Type: "Trailer", Official: true},
			{Key: "tr3", Site: "YouTube", Type: "Trailer", Official: true},
		}, "tr2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickTrailer(tt.videos)
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("got %+v, want none", got)
			case tt.want != "" && (got == nil || got.Key != tt.want):
				t.Errorf("got %+v, want key %q", got, tt.want)
			}
		})
	}
}

func TestTopCast(t *testing.T) {
	var cast []tmdbCast
	for i := 20; i > 0; i-- { // reverse billing order
		cast = append(cast, tmdbCast{Name: fmt.Sprintf("Actor %d", i), Order: i})
	}
	cast[19].Character = "Keeper"
	got := topCast(cast)
	if len(got) != maxCast || got[0].Name != "Actor 1" || got[0].Character != "Keeper" || got[maxCast-1].Name != "Actor 12" {
		t.Errorf("topCast = %+v", got)
	}
}

func TestGetMovieExtras(t *testing.T) {
	c := newTMDB(t, "k", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("append_to_response"); got != "videos,credits,release_dates" {
			t.Errorf("append_to_response = %q", got)
		}
		_, _ = w.Write([]byte(`{"id":900001,"title":"The Glass Orchard","release_date":"1999-03-31",
			"tagline":"Nothing grows in glass.","runtime":118,"vote_average":7.4,"vote_count":321,
			"genres":[{"id":101,"name":"Drama"}],
			"videos":{"results":[{"key":"abcXYZ01","name":"Official Trailer","site":"YouTube","type":"Trailer","official":true}]},
			"credits":{"cast":[{"name":"Rowan Vale","character":"Ivy","profile_path":"/p1.jpg","order":0}]},
			"release_dates":{"results":[
				{"iso_3166_1":"QQ","release_dates":[{"certification":"12"}]},
				{"iso_3166_1":"US","release_dates":[{"certification":""},{"certification":"PG-13"}]}]}}`))
	})
	m, err := c.GetMovie(context.Background(), 900001)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tagline != "Nothing grows in glass." || m.Runtime != 118 || m.VoteAverage != 7.4 || m.VoteCount != 321 {
		t.Errorf("extras = %+v", m.Extras)
	}
	if m.Certification != "PG-13" || len(m.Genres) != 1 || m.Genres[0].Name != "Drama" {
		t.Errorf("certification/genres = %q %+v", m.Certification, m.Genres)
	}
	if m.Trailer == nil || m.Trailer.Key != "abcXYZ01" || len(m.Cast) != 1 || m.Cast[0].Character != "Ivy" {
		t.Errorf("trailer/cast = %+v %+v", m.Trailer, m.Cast)
	}
}

func TestGetSeriesExtrasAndSeasonDetails(t *testing.T) {
	c := newTMDB(t, "k", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tv/800001":
			_, _ = w.Write([]byte(`{"id":800001,"name":"Harbor Watch","first_air_date":"2019-09-01",
				"episode_run_time":[44,50],"vote_average":8.1,"networks":[{"name":"Channel Q"}],
				"seasons":[{"season_number":1,"episode_count":2,"name":"Season 1","poster_path":"/s1.jpg","air_date":"2019-09-01"}],
				"aggregate_credits":{"cast":[{"name":"Mara Quill","roles":[{"character":"Captain Fen"}],"order":0}]},
				"content_ratings":{"results":[{"iso_3166_1":"US","rating":"TV-14"}]},
				"videos":{"results":[{"key":"tease001","site":"YouTube","type":"Teaser"}]}}`))
		case "/tv/800001/season/1":
			_, _ = w.Write([]byte(`{"season_number":1,"name":"Season 1","overview":"The first tide.","poster_path":"/s1.jpg","air_date":"2019-09-01",
				"episodes":[{"season_number":1,"episode_number":1,"name":"Pilot","air_date":"2019-09-01","overview":"It begins.","runtime":47,"still_path":"/e1.jpg","vote_average":7.9}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	s, err := c.GetSeries(ctx, 800001)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runtime != 44 || s.VoteAverage != 8.1 || s.Certification != "TV-14" || fmt.Sprint(s.Networks) != "[Channel Q]" {
		t.Errorf("series extras = %+v networks %v", s.Extras, s.Networks)
	}
	if len(s.Cast) != 1 || s.Cast[0].Character != "Captain Fen" || s.Trailer == nil || s.Trailer.Key != "tease001" {
		t.Errorf("cast/trailer = %+v %+v", s.Cast, s.Trailer)
	}
	if s.Seasons[0].Name != "Season 1" || s.Seasons[0].PosterPath != "/s1.jpg" || s.Seasons[0].AirDate != "2019-09-01" {
		t.Errorf("season summary = %+v", s.Seasons[0])
	}
	season, err := c.GetSeasonDetails(ctx, 800001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if season.Overview != "The first tide." || len(season.Episodes) != 1 {
		t.Fatalf("season = %+v", season)
	}
	if e := season.Episodes[0]; e.Runtime != 47 || e.StillPath != "/e1.jpg" || e.VoteAverage != 7.9 || e.Overview != "It begins." {
		t.Errorf("episode = %+v", e)
	}
}
