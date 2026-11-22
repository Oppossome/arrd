package library

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("s3cret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := checkPassword("s3cret-pass", hash); err != nil || !ok {
		t.Errorf("correct password rejected: %v", err)
	}
	if ok, _ := checkPassword("s3cret-pasS", hash); ok {
		t.Error("wrong password accepted")
	}
	if other, _ := hashPassword("s3cret-pass"); other == hash {
		t.Error("salt is not random")
	}
	if _, err := checkPassword("x", "$bcrypt$nope"); err == nil {
		t.Error("foreign hash format accepted")
	}
}

func TestLoginSessionsAndRateLimit(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := newEnv(t, base, "http://unused", basicSetup(base), nil)
	svc := env.svc
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	token, err := svc.Login(ctx, testPassword, "10.0.0.1")
	if err != nil || token == "" {
		t.Fatalf("login = %q, %v", token, err)
	}
	if ok, _ := svc.ValidateSession(ctx, token); !ok {
		t.Fatal("fresh session invalid")
	}
	if ok, _ := svc.ValidateSession(ctx, token+"x"); ok {
		t.Error("forged session accepted")
	}

	// Sliding expiry: used on day 29, the session survives past day 31.
	now = now.Add(29 * 24 * time.Hour)
	if ok, _ := svc.ValidateSession(ctx, token); !ok {
		t.Fatal("session expired early")
	}
	now = now.Add(2 * 24 * time.Hour)
	if ok, _ := svc.ValidateSession(ctx, token); !ok {
		t.Fatal("sliding renewal did not extend the session")
	}
	// Unused for 31 days, it expires and is purged.
	now = now.Add(31 * 24 * time.Hour)
	if ok, _ := svc.ValidateSession(ctx, token); ok {
		t.Error("expired session accepted")
	}
	if err := svc.PurgeExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}

	// Five failures from one address lock it out; others are unaffected.
	for i := 0; i < loginMaxFailures; i++ {
		if _, err := svc.Login(ctx, "wrong", "10.0.0.2"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	var rl *RateLimitedError
	if _, err := svc.Login(ctx, testPassword, "10.0.0.2"); !errors.As(err, &rl) || rl.RetryAfter <= 0 {
		t.Fatalf("6th attempt = %v, want rate limit", err)
	}
	if _, err := svc.Login(ctx, testPassword, "10.0.0.3"); err != nil {
		t.Errorf("other client blocked: %v", err)
	}
	now = now.Add(loginWindow)
	if _, err := svc.Login(ctx, testPassword, "10.0.0.2"); err != nil {
		t.Errorf("still blocked after the window: %v", err)
	}

	// Changing the password keeps the caller's session and ends the others.
	a, _ := svc.Login(ctx, testPassword, "10.0.0.4")
	b, _ := svc.Login(ctx, testPassword, "10.0.0.4")
	if err := svc.ChangePassword(ctx, "wrong", "another long one", a); err == nil {
		t.Error("password changed without the current password")
	}
	if err := svc.ChangePassword(ctx, testPassword, "short", a); err == nil {
		t.Error("short password accepted")
	}
	if err := svc.ChangePassword(ctx, testPassword, "another long one", a); err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.ValidateSession(ctx, a); !ok {
		t.Error("caller's session ended by password change")
	}
	if ok, _ := svc.ValidateSession(ctx, b); ok {
		t.Error("other session survived password change")
	}
	if _, err := svc.Login(ctx, testPassword, "10.0.0.5"); !errors.Is(err, ErrInvalidCredentials) {
		t.Error("old password still works")
	}
	if err := svc.Logout(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.ValidateSession(ctx, a); ok {
		t.Error("session valid after logout")
	}
}

func TestSetupRequiresPasswordAndStartsSession(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := newEnv(t, base, "http://unused", nil, nil)
	in := *basicSetup(base)
	in.AdminPassword = "short"
	if _, err := env.svc.CompleteSetup(ctx, in); err == nil {
		t.Fatal("short admin password accepted")
	}
	in.AdminPassword = testPassword
	res, err := env.svc.CompleteSetup(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := env.svc.ValidateSession(ctx, res.SessionToken); !ok {
		t.Error("setup did not start a session")
	}
}

func TestPosterProxy(t *testing.T) {
	ctx := context.Background()
	var hits atomic.Int32
	tmdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/w342/poster1.jpg":
			time.Sleep(50 * time.Millisecond) // let concurrent requests pile up
			w.Header().Set("Content-Type", "image/jpeg")
			fmt.Fprint(w, "JPEGDATA")
		case "/w342/html.jpg":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html>")
		default:
			http.NotFound(w, r)
		}
	}))
	defer tmdb.Close()

	base := t.TempDir()
	env := newEnv(t, base, "http://unused", basicSetup(base), nil)
	env.svc.posterBaseURL = tmdb.URL

	var wg sync.WaitGroup
	paths := make([]string, 5)
	errs := make([]error, 5)
	for i := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths[i], errs[i] = env.svc.Poster(ctx, "w342", "/poster1.jpg")
		}()
	}
	wg.Wait()
	for i := range paths {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
	}
	if b, err := os.ReadFile(paths[0]); err != nil || string(b) != "JPEGDATA" {
		t.Fatalf("cached poster = %q, %v", b, err)
	}
	if _, err := env.svc.Poster(ctx, "w342", "poster1.jpg"); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("TMDB hit %d times, want 1 (dedup + cache)", n)
	}

	for _, c := range []struct{ size, file string }{
		{"original", "poster1.jpg"},
		{"w342", "../../arrd.db"},
		{"w342", "a/b.jpg"},
		{"w342", "poster1.gif"},
	} {
		var ve *ValidationError
		if _, err := env.svc.Poster(ctx, c.size, c.file); !errors.As(err, &ve) {
			t.Errorf("Poster(%q, %q) = %v, want validation error", c.size, c.file, err)
		}
	}
	if _, err := env.svc.Poster(ctx, "w342", "html.jpg"); err == nil {
		t.Error("non-image response cached")
	}
	if _, err := env.svc.Poster(ctx, "w342", "missing.jpg"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing poster = %v, want not found", err)
	}
}
