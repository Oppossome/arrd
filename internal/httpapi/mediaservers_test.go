package httpapi

import (
	"fmt"
	"net/http"
	"testing"

	"generic-arr-project/internal/mediaserver/mediaservertest"
)

func TestMediaServerRoutes(t *testing.T) {
	srv, _ := newTestServer(t)
	fake := mediaservertest.Jellyfin(t)

	code, _ := do(t, srv, "GET", "/api/v1/mediaservers/definitions", "", true)
	if code != http.StatusOK {
		t.Fatalf("definitions = %d", code)
	}

	body := fmt.Sprintf(`{"definition":"jellyfin","settings":{"url":%q,"api_key":%q}}`, fake.URL, mediaservertest.Token)
	code, created := do(t, srv, "POST", "/api/v1/mediaservers", body, true)
	if code != http.StatusCreated || created["name"] != "Jellyfin" {
		t.Fatalf("create = %d %v", code, created)
	}
	if settings, _ := created["settings"].(map[string]any); settings["api_key"] != nil {
		t.Errorf("create returned the secret: %v", created)
	}
	path := fmt.Sprintf("/api/v1/mediaservers/%v", created["id"])

	if code, out := do(t, srv, "POST", path+"/test", "", true); code != http.StatusOK || out["last_error"] != nil {
		t.Errorf("test = %d %v", code, out)
	}
	fake.Fail(http.StatusUnauthorized)
	if code, out := do(t, srv, "POST", path+"/test", "", true); code != http.StatusBadGateway || out["error"] == nil {
		t.Errorf("failed test = %d %v", code, out)
	}
	if _, out := do(t, srv, "GET", path, "", true); out["last_error"] == nil {
		t.Errorf("failed test not recorded: %v", out)
	}

	if code, out := do(t, srv, "PUT", path, `{"name":"Den","enabled":false}`, true); code != http.StatusOK || out["name"] != "Den" || out["enabled"] != false {
		t.Errorf("update = %d %v", code, out)
	}
	if code, out := do(t, srv, "POST", "/api/v1/mediaservers", `{"definition":"plex","settings":{"url":"not a url","token":"x"}}`, true); code != http.StatusBadRequest || out["field"] != "settings.url" {
		t.Errorf("invalid create = %d %v", code, out)
	}
	if code, _ := do(t, srv, "DELETE", path, "", true); code != http.StatusNoContent {
		t.Errorf("delete = %d", code)
	}
	if code, _ := do(t, srv, "POST", path+"/test", "", true); code != http.StatusNotFound {
		t.Errorf("test deleted = %d", code)
	}
}
