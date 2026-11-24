package settings

import (
	"context"
	"fmt"
	"strings"
)

// GeneralSettings are app-wide settings. Secrets are write-only; only a
// masked hint of each is returned.
type GeneralSettings struct {
	TMDBAPIKeySet  bool   `json:"tmdb_api_key_set"`
	TMDBAPIKeyHint string `json:"tmdb_api_key_hint,omitempty"`
	APIKeyHint     string `json:"api_key_hint"`
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
	return GeneralSettings{
		TMDBAPIKeySet:  app.TMDBAPIKey != "",
		TMDBAPIKeyHint: maskSecret(app.TMDBAPIKey),
		APIKeyHint:     maskSecret(app.APIKey),
	}, nil
}

// secretHintMinLen is the shortest secret whose ends are shown; shorter ones
// would give away too much of themselves.
const secretHintMinLen = 16

// maskSecret keeps a secret's first and last two characters and masks the
// rest with a fixed run, so the hint does not reveal the length.
func maskSecret(secret string) string {
	const mask = "****************"
	if secret == "" {
		return ""
	}
	if len(secret) < secretHintMinLen {
		return mask
	}
	return secret[:2] + mask + secret[len(secret)-2:]
}

// UpdateGeneralSettings validates a new TMDB key with TMDB and saves it.
func (s *Service) UpdateGeneralSettings(ctx context.Context, in GeneralSettingsInput) (GeneralSettings, error) {
	if key := strings.TrimSpace(in.TMDBAPIKey); key != "" {
		if err := s.tmdb.ValidateTMDBKey(ctx, key); err != nil {
			return GeneralSettings{}, err
		}
		if err := s.store.SetTMDBAPIKey(ctx, key); err != nil {
			return GeneralSettings{}, fmt.Errorf("save tmdb key: %w", err)
		}
	}
	return s.GeneralSettings(ctx)
}
