package auth_test

import (
	"context"
	"errors"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/auth"
	"generic-arr-project/internal/library/libtest"
	"testing"
	"time"
)

func TestLoginSessionsAndRateLimit(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, "http://unused", libtest.BasicSetup(base), nil)
	svc := env.Svc
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	env.SetNow(func() time.Time { return now })

	token, err := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.1")
	if err != nil || token == "" {
		t.Fatalf("login = %q, %v", token, err)
	}
	if ok, _ := svc.Auth.ValidateSession(ctx, token); !ok {
		t.Fatal("fresh session invalid")
	}
	if ok, _ := svc.Auth.ValidateSession(ctx, token+"x"); ok {
		t.Error("forged session accepted")
	}

	// Sliding expiry: used on day 29, the session survives past day 31.
	now = now.Add(29 * 24 * time.Hour)
	if ok, _ := svc.Auth.ValidateSession(ctx, token); !ok {
		t.Fatal("session expired early")
	}
	now = now.Add(2 * 24 * time.Hour)
	if ok, _ := svc.Auth.ValidateSession(ctx, token); !ok {
		t.Fatal("sliding renewal did not extend the session")
	}
	// Unused for 31 days, it expires and is purged.
	now = now.Add(31 * 24 * time.Hour)
	if ok, _ := svc.Auth.ValidateSession(ctx, token); ok {
		t.Error("expired session accepted")
	}
	if err := svc.Auth.PurgeExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}

	// Five failures from one address lock it out; others are unaffected.
	for i := 0; i < auth.LoginMaxFailures; i++ {
		if _, err := svc.Auth.Login(ctx, "wrong", "10.0.0.2"); !errors.Is(err, apperr.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	var rl *apperr.RateLimitedError
	if _, err := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.2"); !errors.As(err, &rl) || rl.RetryAfter <= 0 {
		t.Fatalf("6th attempt = %v, want rate limit", err)
	}
	if _, err := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.3"); err != nil {
		t.Errorf("other client blocked: %v", err)
	}
	now = now.Add(auth.LoginWindow)
	if _, err := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.2"); err != nil {
		t.Errorf("still blocked after the window: %v", err)
	}

	// Changing the password keeps the caller's session and ends the others.
	a, _ := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.4")
	b, _ := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.4")
	if err := svc.Auth.ChangePassword(ctx, "wrong", "another long one", a); err == nil {
		t.Error("password changed without the current password")
	}
	if err := svc.Auth.ChangePassword(ctx, libtest.TestPassword, "short", a); err == nil {
		t.Error("short password accepted")
	}
	if err := svc.Auth.ChangePassword(ctx, libtest.TestPassword, "another long one", a); err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.Auth.ValidateSession(ctx, a); !ok {
		t.Error("caller's session ended by password change")
	}
	if ok, _ := svc.Auth.ValidateSession(ctx, b); ok {
		t.Error("other session survived password change")
	}
	if _, err := svc.Auth.Login(ctx, libtest.TestPassword, "10.0.0.5"); !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Error("old password still works")
	}
	if err := svc.Auth.Logout(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.Auth.ValidateSession(ctx, a); ok {
		t.Error("session valid after logout")
	}
}
