package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSetupFlow(t *testing.T) {
	srv := newUnconfiguredServer(t)
	tok := map[string]string{"X-Setup-Token": testSetupToken}

	code, body := doWith(t, srv, "GET", "/api/v1/setup/status", "", nil)
	if code != http.StatusOK || body["setup_complete"] != false || body["suggested"] == nil {
		t.Fatalf("status = %d %v", code, body)
	}
	if code, _ := doWith(t, srv, "GET", "/api/v1/movies", "", map[string]string{"X-Api-Key": "x"}); code != http.StatusServiceUnavailable {
		t.Errorf("api before setup = %d, want 503", code)
	}
	if code, _ := doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), nil); code != http.StatusUnauthorized {
		t.Errorf("setup without token = %d", code)
	}
	if code, _ := doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), map[string]string{"X-Setup-Token": "wrong"}); code != http.StatusUnauthorized {
		t.Errorf("setup with wrong token = %d", code)
	}

	code, body = doWith(t, srv, "POST", "/api/v1/setup/validate/paths",
		fmt.Sprintf(`{"downloads":{"incomplete_dir":"rel","complete_dir":%q},"create_missing":true}`, filepath.Join(srv.base, "c")), tok)
	errs, _ := body["errors"].(map[string]any)
	if code != http.StatusOK || body["valid"] != false || errs["downloads.incomplete_dir"] == nil || errs["movie_root"] == nil {
		t.Errorf("validate paths = %d %v", code, body)
	}
	if code, body := doWith(t, srv, "POST", "/api/v1/setup/validate/vpn", `{"provider":"wireguard","values":{"config":"[Interface]\n"}}`, tok); code != http.StatusBadRequest || body["field"] != "config" {
		t.Errorf("validate vpn = %d %v", code, body)
	}
	if code, body := doWith(t, srv, "POST", "/api/v1/setup", `{"admin_password":"correct horse battery","tmdb_api_key":""}`, tok); code != http.StatusBadRequest || body["field"] != "tmdb_api_key" {
		t.Errorf("bad setup = %d %v", code, body)
	}

	code, body = doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), tok)
	if code != http.StatusCreated || body["api_key"] == "" {
		t.Fatalf("setup = %d %v", code, body)
	}
	key := body["api_key"].(string)
	if code, _ := doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), tok); code != http.StatusConflict {
		t.Errorf("second setup = %d, want 409", code)
	}
	if code, _ := doWith(t, srv, "POST", "/api/v1/setup/validate/paths", `{}`, tok); code != http.StatusConflict {
		t.Errorf("validate after setup = %d, want 409", code)
	}
	if _, body := doWith(t, srv, "GET", "/api/v1/setup/status", "", nil); body["setup_complete"] != true {
		t.Errorf("status after setup = %v", body)
	}
	if code, _ := doWith(t, srv, "GET", "/api/v1/movies", "", map[string]string{"X-Api-Key": key}); code != http.StatusOK {
		t.Errorf("api with new key = %d", code)
	}
	if code, body := doWith(t, srv, "GET", "/api/v1/status", "", map[string]string{"X-Api-Key": key}); code != http.StatusOK || body["downloads_available"] != true {
		t.Errorf("runtime status = %d %v", code, body)
	}

	// Regenerating the key retires the old one.
	srv.apiKey = key
	code, body = do(t, srv, "POST", "/api/v1/settings/api-key/regenerate", "", true)
	if code != http.StatusOK || body["api_key"] == key {
		t.Fatalf("regenerate = %d %v", code, body)
	}
	if code, _ := doWith(t, srv, "GET", "/api/v1/movies", "", map[string]string{"X-Api-Key": key}); code != http.StatusUnauthorized {
		t.Errorf("old key after regenerate = %d", code)
	}
	if code, body := doWith(t, srv, "GET", "/api/v1/settings/vpn", "", map[string]string{"X-Api-Key": body["api_key"].(string)}); code != http.StatusOK || body["configured"] != false {
		t.Errorf("vpn settings = %d %v", code, body)
	}
}
