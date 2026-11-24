// Package tmdb builds TMDB clients from the saved API key.
package tmdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
)

const timeout = 20 * time.Second

// Provider hands out TMDB clients on the direct network.
type Provider struct {
	store   *store.Store
	baseURL string // "" = TMDB's API
	direct  netx.Network
}

// New returns a Provider. baseURL may be empty for TMDB's API.
func New(st *store.Store, direct netx.Network, baseURL string) (*Provider, error) {
	if st == nil || direct == nil {
		return nil, errors.New("tmdb: store and direct network are required")
	}
	return &Provider{store: st, baseURL: baseURL, direct: direct}, nil
}

// Client returns a TMDB client using the saved API key.
func (p *Provider) Client(ctx context.Context) (*metadata.TMDB, error) {
	app, err := p.store.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("setup is not complete")
	}
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	return p.withKey(app.TMDBAPIKey), nil
}

// ValidateTMDBKey checks a TMDB API key with one request.
func (p *Provider) ValidateTMDBKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return apperr.Invalid("tmdb_api_key", "is required")
	}
	if err := p.withKey(key).Validate(ctx); err != nil {
		return apperr.Invalid("tmdb_api_key", "TMDB rejected the key: %v", err)
	}
	return nil
}

func (p *Provider) withKey(key string) *metadata.TMDB {
	return &metadata.TMDB{APIKey: key, BaseURL: p.baseURL, HTTP: netx.HTTPClient(p.direct, timeout)}
}
