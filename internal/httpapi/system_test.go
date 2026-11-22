package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDirsEndpoint(t *testing.T) {
	srv := newUnconfiguredServer(t)
	root := filepath.Join(srv.base, "browse")
	for _, d := range []string{"b", "a", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tok := map[string]string{"X-Setup-Token": testSetupToken}

	code, body := doWith(t, srv, "GET", "/api/v1/fs/dirs?path="+root, "", tok)
	dirs, _ := body["dirs"].([]any)
	if code != http.StatusOK || len(dirs) != 2 || dirs[0] != "a" || dirs[1] != "b" || body["parent"] != srv.base {
		t.Fatalf("dirs = %d %v", code, body)
	}
	if code, _ := doWith(t, srv, "GET", "/api/v1/fs/dirs?path="+root, "", nil); code != http.StatusUnauthorized {
		t.Errorf("without token = %d", code)
	}
	for _, p := range []string{"relative/path", root + "/missing", root + "/file.txt"} {
		if code, _ := doWith(t, srv, "GET", "/api/v1/fs/dirs?path="+p, "", tok); code != http.StatusBadRequest {
			t.Errorf("path %q = %d, want 400", p, code)
		}
	}

	// After setup the token no longer works; the API key does.
	code, body = doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), tok)
	if code != http.StatusCreated {
		t.Fatalf("setup = %d %v", code, body)
	}
	srv.apiKey = body["api_key"].(string)
	if code, _ := doWith(t, srv, "GET", "/api/v1/fs/dirs?path="+root, "", tok); code != http.StatusUnauthorized {
		t.Errorf("setup token after setup = %d", code)
	}
	if code, _ := do(t, srv, "GET", "/api/v1/fs/dirs?path="+root, "", true); code != http.StatusOK {
		t.Errorf("api key = %d", code)
	}
}

func TestPosterRouteRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)
	if code, _ := do(t, srv, "GET", "/api/v1/posters/w342/abc.jpg", "", false); code != http.StatusUnauthorized {
		t.Errorf("anonymous poster = %d", code)
	}
	if code, _ := do(t, srv, "GET", "/api/v1/posters/huge/abc.jpg", "", true); code != http.StatusBadRequest {
		t.Errorf("bad size = %d", code)
	}
}
