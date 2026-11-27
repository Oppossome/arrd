package httpapi

import (
	"net/http"
	"testing"
)

// The spec checks a release target's shape; the service checks what each
// kind needs.
func TestReleasesRejectsBadTargets(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, path := range []string{
		"/api/v1/releases?tmdb_id=900001",                                 // no kind
		"/api/v1/releases?kind=trailer&tmdb_id=900001",                    // unknown kind
		"/api/v1/releases?kind=movie",                                     // no TMDB id
		"/api/v1/releases?kind=movie&tmdb_id=0",                           // not a TMDB id
		"/api/v1/releases?kind=season&tmdb_id=800001",                     // no season
		"/api/v1/releases?kind=episode&tmdb_id=800001&season=1",           // no episode
		"/api/v1/releases?kind=episode&tmdb_id=800001&season=1&episode=0", // no episode 0
		"/api/v1/releases?kind=movie&tmdb_id=900001&season=1",             // a movie has no seasons
	} {
		if code, body := do(t, srv, "GET", path, "", true); code != http.StatusBadRequest {
			t.Errorf("%s = %d %v, want 400", path, code, body)
		}
	}
	for _, body := range []string{
		`{"target":{"kind":"season","tmdb_id":800001},"release":{"title":"x","indexer_id":1}}`,
		`{"release":{"title":"x","indexer_id":1}}`,
	} {
		if code, out := do(t, srv, "POST", "/api/v1/grabs", body, true); code != http.StatusBadRequest {
			t.Errorf("grab %s = %d %v, want 400", body, code, out)
		}
	}
}
