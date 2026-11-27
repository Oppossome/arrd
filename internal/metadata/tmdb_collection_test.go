package metadata

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestGetCollection(t *testing.T) {
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/collection/700001":
			_, _ = w.Write([]byte(`{"id":700001,"name":"Skyfarer Collection","parts":[
				{"id":900013,"title":"Skyfarer: Homeward","release_date":""},
				{"id":900012,"title":"Skyfarer: Ascent","release_date":"2013-06-01"},
				{"id":900011,"title":"Skyfarer","original_title":"Skyfarer","release_date":"2011-05-20"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	got, err := c.GetCollection(ctx, 700001)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 700001 || got.Name != "Skyfarer Collection" || len(got.Parts) != 3 {
		t.Fatalf("got %+v", got)
	}
	var ids []int64
	for _, p := range got.Parts {
		ids = append(ids, p.TMDBID)
	}
	if ids[0] != 900011 || ids[1] != 900012 || ids[2] != 900013 {
		t.Errorf("parts order = %v, want oldest first, unreleased last", ids)
	}
	if _, err := c.GetCollection(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing collection err = %v", err)
	}
}

func TestGetMovieCollection(t *testing.T) {
	c := newTMDB(t, "v3key", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/movie/900011":
			_, _ = w.Write([]byte(`{"id":900011,"title":"Skyfarer","release_date":"2011-05-20","belongs_to_collection":{"id":700001,"name":"Skyfarer Collection"}}`))
		case "/movie/900001":
			_, _ = w.Write([]byte(`{"id":900001,"title":"The Glass Orchard","release_date":"1999-03-31","belongs_to_collection":null}`))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	m, err := c.GetMovie(ctx, 900011)
	if err != nil {
		t.Fatal(err)
	}
	if m.Collection == nil || m.Collection.ID != 700001 || m.Collection.Name != "Skyfarer Collection" {
		t.Errorf("collection = %+v", m.Collection)
	}
	m, err = c.GetMovie(ctx, 900001)
	if err != nil {
		t.Fatal(err)
	}
	if m.Collection != nil {
		t.Errorf("standalone movie collection = %+v, want nil", m.Collection)
	}
}
