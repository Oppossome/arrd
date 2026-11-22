package httpapi

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTVListsAndRSS(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, path := range []string{"/api/v1/series", "/api/v1/queue"} {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("X-Api-Key", srv.apiKey)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(raw)) != "[]" {
			t.Errorf("GET %s = %d %s, want 200 []", path, resp.StatusCode, raw)
		}
	}
	code, body := do(t, srv, "POST", "/api/v1/rss/sync", "", true)
	if code != http.StatusOK || body["releases_seen"] != float64(0) {
		t.Errorf("rss sync = %d %v", code, body)
	}
}

func TestThroughputWithoutDownloads(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := do(t, srv, "GET", "/api/v1/queue/throughput", "", true)
	if code != http.StatusOK || body["active"] != float64(0) || body["download_rate"] != float64(0) {
		t.Errorf("throughput = %d %v", code, body)
	}
	if h, ok := body["history"].([]any); !ok || len(h) != 0 {
		t.Errorf("history = %#v, want []", body["history"])
	}
}
