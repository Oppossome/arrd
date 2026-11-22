package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPA(t *testing.T) {
	ui := fstest.MapFS{
		"index.html":                  {Data: []byte("<html>app</html>")},
		"_app/immutable/entry/app.js": {Data: []byte("console.log(1)")},
		"_app/version.json":           {Data: []byte(`{"version":"1"}`)},
		"favicon.png":                 {Data: []byte("PNG")},
	}
	srv := newUnconfiguredServerUI(t, ui)
	get := func(path string) (*http.Response, string) {
		resp, err := srv.Client().Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}

	for _, p := range []string{"/", "/movies/5", "/setup?token=x"} {
		resp, body := get(p)
		if resp.StatusCode != http.StatusOK || body != "<html>app</html>" || resp.Header.Get("Cache-Control") != "no-cache" {
			t.Errorf("GET %s = %d %q cache %q", p, resp.StatusCode, body, resp.Header.Get("Cache-Control"))
		}
		if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'self'") {
			t.Errorf("GET %s missing CSP", p)
		}
	}
	if resp, body := get("/_app/immutable/entry/app.js"); body != "console.log(1)" || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("immutable asset: %q %q", body, resp.Header.Get("Cache-Control"))
	}
	if resp, _ := get("/_app/version.json"); resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("version.json cache = %q", resp.Header.Get("Cache-Control"))
	}
	if resp, _ := get("/_app/immutable/missing.js"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing asset = %d", resp.StatusCode)
	}
	// API routes are not swallowed by the fallback.
	if resp, body := get("/api/v1/setup/status"); resp.StatusCode != http.StatusOK || !strings.Contains(body, "setup_complete") {
		t.Errorf("api through SPA mux = %d %q", resp.StatusCode, body)
	}
	if resp, _ := get("/api/v1/nonexistent"); resp.StatusCode == http.StatusOK {
		t.Error("unknown API path served the SPA")
	}

	// Without a build, a helpful page.
	bare := newUnconfiguredServer(t)
	resp, err := bare.Client().Get(bare.URL + "/movies")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(b), "go generate ./web") {
		t.Errorf("unbuilt UI = %d %q", resp.StatusCode, b)
	}
}

func TestCSPAllowsOnlyTheInlineBootstrap(t *testing.T) {
	index := []byte("<html><script src=\"/a.js\"></script><div><script>\n\tboot();\n</script></div></html>")
	csp := contentSecurityPolicy(index)
	sum := sha256.Sum256([]byte("\n\tboot();\n"))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if !strings.Contains(csp, "script-src 'self' "+want+";") {
		t.Errorf("csp = %s, want script hash %s", csp, want)
	}
	if strings.Count(csp, "sha256-") != 1 {
		t.Errorf("external script hashed too: %s", csp)
	}
}
