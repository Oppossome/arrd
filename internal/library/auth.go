package library

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"

	"generic-arr-project/internal/store/db"
)

const (
	minPasswordLength = 8
	sessionTTL        = 30 * 24 * time.Hour
	// sessionTouchEvery limits session writes: a request extends the session
	// at most this often.
	sessionTouchEvery = time.Minute

	loginMaxFailures = 5
	loginWindow      = 15 * time.Minute

	timeLayout = "2006-01-02T15:04:05Z"
)

// argon2id parameters (OWASP's minimum recommendation is m=19MiB, t=2).
const (
	argonTime    = 2
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// ErrInvalidCredentials is returned for a wrong password or session.
var ErrInvalidCredentials = errors.New("invalid credentials")

// RateLimitedError says how long a client must wait before retrying.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("too many failed logins; try again in %s", e.RetryAfter.Round(time.Second))
}

func validatePassword(field, pw string) error {
	if len([]rune(pw)) < minPasswordLength {
		return invalid(field, "must be at least %d characters", minPasswordLength)
	}
	if len(pw) > 1024 {
		return invalid(field, "is too long")
	}
	return nil
}

// hashPassword returns an argon2id PHC string.
func hashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// checkPassword verifies pw against a PHC string from hashPassword.
func checkPassword(pw, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported password hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	var mem uint32
	var t uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &threads); err != nil {
		return false, fmt.Errorf("parse argon2 parameters: %w", err)
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(pw), salt, t, mem, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func newSessionToken() (token, idHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// loginLimiter counts failed logins per client key (IP address).
type loginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func (l *loginLimiter) check(key string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.prune(key, now)
	if len(recent) >= loginMaxFailures {
		return &RateLimitedError{RetryAfter: recent[0].Add(loginWindow).Sub(now)}
	}
	return nil
}

func (l *loginLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures == nil {
		l.failures = map[string][]time.Time{}
	}
	l.failures[key] = append(l.prune(key, now), now)
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// prune drops failures outside the window; the caller holds the lock.
func (l *loginLimiter) prune(key string, now time.Time) []time.Time {
	var kept []time.Time
	for _, t := range l.failures[key] {
		if now.Sub(t) < loginWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, key)
	} else {
		l.failures[key] = kept
	}
	return kept
}

// Login checks the admin password and returns a new session token.
// clientKey (normally the client IP) is used for rate limiting.
func (s *Service) Login(ctx context.Context, password, clientKey string) (string, error) {
	now := s.now()
	if err := s.logins.check(clientKey, now); err != nil {
		return "", err
	}
	app, err := s.store.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("setup is not complete")
	}
	if err != nil {
		return "", fmt.Errorf("load settings: %w", err)
	}
	ok, err := checkPassword(password, app.PasswordHash)
	if err != nil {
		return "", err
	}
	if !ok {
		s.logins.fail(clientKey, now)
		s.log.Warn("failed login", "client", clientKey)
		return "", ErrInvalidCredentials
	}
	s.logins.reset(clientKey)
	return s.createSession(ctx, s.store.Queries)
}

func (s *Service) createSession(ctx context.Context, q *db.Queries) (string, error) {
	token, idHash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	if err := q.CreateSession(ctx, db.CreateSessionParams{
		IDHash: idHash, Now: now.Format(timeLayout), ExpiresAt: now.Add(sessionTTL).Format(timeLayout),
	}); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

// ValidateSession reports whether token is a live session, extending it
// (sliding expiry) at most once a minute.
func (s *Service) ValidateSession(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	now := s.now().UTC()
	idHash := hashToken(token)
	sess, err := s.store.GetValidSession(ctx, db.GetValidSessionParams{IDHash: idHash, Now: now.Format(timeLayout)})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load session: %w", err)
	}
	if last, err := time.Parse(timeLayout, sess.LastSeenAt); err != nil || now.Sub(last) >= sessionTouchEvery {
		if err := s.store.TouchSession(ctx, db.TouchSessionParams{
			IDHash: idHash, Now: now.Format(timeLayout), ExpiresAt: now.Add(sessionTTL).Format(timeLayout),
		}); err != nil {
			s.log.Warn("extend session", "err", err)
		}
	}
	return true, nil
}

// Logout ends a session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(token))
}

// ChangePassword replaces the admin password after checking the current
// one, and ends every other session.
func (s *Service) ChangePassword(ctx context.Context, current, next, keepSession string) error {
	app, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	ok, err := checkPassword(current, app.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return invalid("current_password", "is wrong")
	}
	if err := validatePassword("new_password", next); err != nil {
		return err
	}
	hash, err := hashPassword(next)
	if err != nil {
		return err
	}
	return s.store.WithTx(ctx, func(q *db.Queries) error {
		if err := q.SetPasswordHash(ctx, hash); err != nil {
			return err
		}
		keep := ""
		if keepSession != "" {
			keep = hashToken(keepSession)
		}
		return q.DeleteOtherSessions(ctx, keep)
	})
}

// PurgeExpiredSessions deletes expired sessions.
func (s *Service) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.store.DeleteExpiredSessions(ctx, s.now().UTC().Format(timeLayout))
	return err
}
