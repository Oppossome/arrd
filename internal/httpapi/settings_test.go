package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	srv, base := newTestServer(t)
	code, body := do(t, srv, "GET", "/api/v1/settings/downloads", "", true)
	if code != http.StatusOK || body["network"] != nil || body["import_mode"] != "hardlink" {
		t.Fatalf("get = %d %v", code, body)
	}
	dir := filepath.Join(base, "data", "dl", "complete")
	put := `{"incomplete_dir":"` + filepath.Join(base, "data", "dl", "incomplete") + `","complete_dir":"` + dir + `","import_mode":"copy"}`
	code, body = do(t, srv, "PUT", "/api/v1/settings/downloads", put, true)
	if code != http.StatusOK || body["settings"].(map[string]any)["import_mode"] != "copy" {
		t.Fatalf("put = %d %v", code, body)
	}

	code, body = do(t, srv, "POST", "/api/v1/indexers", `{"name":"p","definition":"examplesite","settings":{"base_url":"http://site.invalid/","apikey":"s"}}`, true)
	if code != http.StatusCreated || fmt.Sprint(body["secrets_set"]) != "[apikey]" || body["settings"].(map[string]any)["apikey"] != nil {
		t.Fatalf("create indexer = %d %v", code, body)
	}
}
