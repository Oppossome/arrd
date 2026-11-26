package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func TestErrorMapping(t *testing.T) {
	srv, base := newTestServer(t)
	tests := []struct {
		method, path, body string
		want               int
		field              string
	}{
		{"GET", "/api/v1/movies/999", "", http.StatusNotFound, ""},
		{"GET", "/api/v1/movies/abc", "", http.StatusBadRequest, ""},
		{"POST", "/api/v1/movies", `{"tmdb_id": 0}`, http.StatusBadRequest, "tmdb_id"},
		{"POST", "/api/v1/movies", `{"bogus": 1}`, http.StatusBadRequest, ""},
		{"POST", "/api/v1/indexers", `{"name":"a","definition":"examplesite","settings":{"base_url":"http://elsewhere.invalid/"}}`, http.StatusBadRequest, "settings.base_url"},
		{"POST", "/api/v1/indexers", `{"name":"a","definition":"nowhere","settings":{}}`, http.StatusBadRequest, "definition"},
		{"POST", "/api/v1/rootfolders", fmt.Sprintf(`{"media_type":"movie","path":%q}`, filepath.Join(base, "Movies")), http.StatusConflict, ""},
		{"DELETE", "/api/v1/indexers/5", "", http.StatusNotFound, ""},
		{"PUT", "/api/v1/settings/quality", `{"min_resolution":1080,"max_resolution":720}`, http.StatusBadRequest, "min_resolution"},
		{"GET", "/api/v1/series/999", "", http.StatusNotFound, ""},
		{"GET", "/api/v1/series/999/episodes", "", http.StatusNotFound, ""},
		{"PUT", "/api/v1/series/1/seasons/x", `{"monitored":true}`, http.StatusBadRequest, ""},
		{"PUT", "/api/v1/series/1/seasons/1", `{"monitored":true}`, http.StatusNotFound, ""},
		{"PUT", "/api/v1/episodes/5", `{}`, http.StatusBadRequest, ""},
		{"PUT", "/api/v1/episodes/5", `{"monitored":false}`, http.StatusNotFound, ""},
		{"POST", "/api/v1/series", `{"tmdb_id":1,"monitor":"some"}`, http.StatusBadRequest, "monitor"},
		{"POST", "/api/v1/series/3/grab", `{"title":"x","indexer_id":1,"download_url":"http://x"}`, http.StatusNotFound, ""},
		{"GET", "/api/v1/series/lookup", "", http.StatusBadRequest, "q"},
		{"POST", "/api/v1/queue/999/pause", "", http.StatusNotFound, ""},
		{"POST", "/api/v1/queue/999/resume", "", http.StatusNotFound, ""},
		{"GET", "/api/v1/queue/999/peers", "", http.StatusNotFound, ""},
		// Checked against openapi.yaml before reaching a handler.
		{"PUT", "/api/v1/episodes/5", `{"monitored":"yes"}`, http.StatusBadRequest, "monitored"},
		{"GET", "/api/v1/queue?movie_id=0", "", http.StatusBadRequest, "movie_id"},
		{"GET", "/api/v1/rootfolders?media_type=music", "", http.StatusBadRequest, "media_type"},
		{"DELETE", "/api/v1/queue/0", "", http.StatusBadRequest, "id"},
		{"POST", "/api/v1/queue/0/pause", "", http.StatusBadRequest, "id"},
		{"POST", "/api/v1/indexers", `{"name":"a"}`, http.StatusBadRequest, "definition"},
		{"POST", "/api/v1/indexers", `{"name":"a","definition":"examplesite","settings":{"base_url":7}}`, http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		code, body := do(t, srv, tt.method, tt.path, tt.body, true)
		if code != tt.want {
			t.Errorf("%s %s = %d %v, want %d", tt.method, tt.path, code, body, tt.want)
			continue
		}
		if tt.field != "" && body["field"] != tt.field {
			t.Errorf("%s %s field = %v, want %s", tt.method, tt.path, body["field"], tt.field)
		}
	}
}
