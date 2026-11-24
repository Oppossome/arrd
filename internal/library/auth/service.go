// Package auth handles the admin password, login sessions, the API key and
// whether onboarding has finished.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"generic-arr-project/internal/store"

	"github.com/rs/zerolog"
)

// Service checks credentials.
type Service struct {
	store  *store.Store
	log    *zerolog.Logger
	now    func() time.Time
	logins loginLimiter

	setupDone atomic.Bool
	apiKey    atomic.Pointer[string]
}

// Deps are the Service's collaborators.
type Deps struct {
	Store *store.Store
	Log   *zerolog.Logger
	Now   func() time.Time // default time.Now
}

// New creates a Service. Call Load before use.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Log == nil {
		return nil, errors.New("auth: Store and Log are required")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{store: d.Store, log: d.Log, now: d.Now}, nil
}

// Load reads the saved API key. It reports false while onboarding is
// pending.
func (s *Service) Load(ctx context.Context) (bool, error) {
	app, err := s.store.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load settings: %w", err)
	}
	s.SetupCompleted(app.APIKey)
	return true, nil
}

// SetupComplete reports whether onboarding has finished.
func (s *Service) SetupComplete() bool { return s.setupDone.Load() }

// SetupCompleted records that onboarding finished with apiKey.
func (s *Service) SetupCompleted(apiKey string) {
	s.apiKey.Store(&apiKey)
	s.setupDone.Store(true)
}

// CheckAPIKey reports whether key is the current API key.
func (s *Service) CheckAPIKey(key string) bool {
	want := s.apiKey.Load()
	if want == nil || key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(*want)) == 1
}

// RegenerateAPIKey replaces the API key; the old one stops working at once.
func (s *Service) RegenerateAPIKey(ctx context.Context) (string, error) {
	key, err := NewAPIKey()
	if err != nil {
		return "", err
	}
	if err := s.store.SetAPIKey(ctx, key); err != nil {
		return "", fmt.Errorf("save api key: %w", err)
	}
	s.apiKey.Store(&key)
	s.log.Info().Msg("api key regenerated")
	return key, nil
}

// NewAPIKey generates a random API key.
func NewAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return hex.EncodeToString(b), nil
}
