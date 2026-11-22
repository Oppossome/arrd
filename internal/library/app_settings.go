package library

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
)

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
	key, err := newAPIKey()
	if err != nil {
		return "", err
	}
	if err := s.store.SetAPIKey(ctx, key); err != nil {
		return "", fmt.Errorf("save api key: %w", err)
	}
	s.apiKey.Store(&key)
	s.log.Info("api key regenerated")
	return key, nil
}

// GeneralSettings are app-wide settings. Secrets are write-only.
type GeneralSettings struct {
	TMDBAPIKeySet bool `json:"tmdb_api_key_set"`
}

// GeneralSettingsInput updates general settings; an empty key is kept.
type GeneralSettingsInput struct {
	TMDBAPIKey string `json:"tmdb_api_key"`
}

// GeneralSettings returns the general settings.
func (s *Service) GeneralSettings(ctx context.Context) (GeneralSettings, error) {
	app, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return GeneralSettings{}, fmt.Errorf("load settings: %w", err)
	}
	return GeneralSettings{TMDBAPIKeySet: app.TMDBAPIKey != ""}, nil
}

// UpdateGeneralSettings validates a new TMDB key with TMDB and saves it.
func (s *Service) UpdateGeneralSettings(ctx context.Context, in GeneralSettingsInput) (GeneralSettings, error) {
	if key := strings.TrimSpace(in.TMDBAPIKey); key != "" {
		if err := s.ValidateTMDBKey(ctx, key); err != nil {
			return GeneralSettings{}, err
		}
		if err := s.store.SetTMDBAPIKey(ctx, key); err != nil {
			return GeneralSettings{}, fmt.Errorf("save tmdb key: %w", err)
		}
	}
	return s.GeneralSettings(ctx)
}
