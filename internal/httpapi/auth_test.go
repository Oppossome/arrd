package httpapi

import (
	"net/http"
	"testing"

	"github.com/rs/zerolog"
)

func TestAuth(t *testing.T) {
	srv, _ := newTestServer(t)
	if code, _ := do(t, srv, "GET", "/api/v1/health", "", false); code != http.StatusOK {
		t.Errorf("health without key = %d", code)
	}
	if code, _ := do(t, srv, "GET", "/api/v1/movies", "", false); code != http.StatusUnauthorized {
		t.Errorf("movies without key = %d", code)
	}
	if code, _ := do(t, srv, "GET", "/api/v1/movies?apikey="+srv.apiKey, "", false); code != http.StatusOK {
		t.Errorf("movies with query key = %d", code)
	}
	if _, err := New(nil, "", new(zerolog.Nop()), nil); err == nil {
		t.Error("empty API key accepted")
	}
}

func TestBrowserLoginFlow(t *testing.T) {
	srv := newUnconfiguredServer(t)
	b := newBrowser(t, srv)

	// Finishing setup signs the browser in.
	resp := b.do("POST", "/api/v1/setup", setupBody(srv.base), map[string]string{"X-Setup-Token": testSetupToken})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup = %d", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure {
		t.Fatalf("session cookie = %+v (Secure only over HTTPS)", cookie)
	}
	if r := b.do("GET", "/api/v1/movies", "", nil); r.StatusCode != http.StatusOK {
		t.Errorf("movies with cookie = %d", r.StatusCode)
	}

	// CSRF: unsafe cookie requests need a same-origin Origin or Referer.
	body := `{"min_resolution":720,"max_resolution":1080,"max_size_mb":0}`
	if r := b.do("PUT", "/api/v1/settings/quality", body, map[string]string{"Origin": "https://evil.example"}); r.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin PUT = %d, want 403", r.StatusCode)
	}
	if r := b.do("PUT", "/api/v1/settings/quality", body, map[string]string{"Origin": ""}); r.StatusCode != http.StatusForbidden {
		t.Errorf("PUT without Origin = %d, want 403", r.StatusCode)
	}
	if r := b.do("PUT", "/api/v1/settings/quality", body, map[string]string{"Origin": "", "Referer": srv.URL + "/settings"}); r.StatusCode != http.StatusOK {
		t.Errorf("PUT with same-origin Referer = %d", r.StatusCode)
	}
	if r := b.do("PUT", "/api/v1/settings/quality", body, nil); r.StatusCode != http.StatusOK {
		t.Errorf("same-origin PUT = %d", r.StatusCode)
	}

	// Logout, then the cookie no longer works; logging in restores access.
	if r := b.do("POST", "/api/v1/auth/logout", "", nil); r.StatusCode != http.StatusNoContent {
		t.Fatalf("logout = %d", r.StatusCode)
	}
	if r := b.do("GET", "/api/v1/movies", "", nil); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("after logout = %d", r.StatusCode)
	}
	if r := b.do("POST", "/api/v1/auth/login", `{"password":"nope"}`, nil); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password = %d", r.StatusCode)
	}
	if r := b.do("POST", "/api/v1/auth/login", `{"password":"correct horse battery"}`, map[string]string{"X-Forwarded-Proto": "https"}); r.StatusCode != http.StatusNoContent {
		t.Fatalf("login = %d", r.StatusCode)
	} else if c := r.Cookies(); len(c) != 1 || !c[0].Secure {
		t.Errorf("cookie behind an HTTPS proxy should be Secure: %+v", c)
	}
	_, sess := doWith(t, srv, "GET", "/api/v1/auth/session", "", nil)
	if sess["authenticated"] != false || sess["setup_complete"] != true {
		t.Errorf("anonymous session = %v", sess)
	}

	// Password change via the API.
	if r := b.do("PUT", "/api/v1/settings/password", `{"current_password":"x","new_password":"another long one"}`, nil); r.StatusCode != http.StatusBadRequest {
		t.Errorf("wrong current password = %d", r.StatusCode)
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv, _ := newTestServer(t)
	b := newBrowser(t, srv)
	var last *http.Response
	for i := 0; i < 6; i++ {
		last = b.do("POST", "/api/v1/auth/login", `{"password":"wrong"}`, nil)
	}
	if last.StatusCode != http.StatusTooManyRequests || last.Header.Get("Retry-After") == "" {
		t.Errorf("6th failure = %d, Retry-After %q", last.StatusCode, last.Header.Get("Retry-After"))
	}
}

func TestAPIKeyNeedsNoOrigin(t *testing.T) {
	srv, _ := newTestServer(t)
	// do() sends no Origin header at all.
	code, _ := do(t, srv, "PUT", "/api/v1/settings/quality", `{"min_resolution":720,"max_resolution":1080,"max_size_mb":0}`, true)
	if code != http.StatusOK {
		t.Errorf("API-key PUT without Origin = %d", code)
	}
}
