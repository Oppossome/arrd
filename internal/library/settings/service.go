// Package settings holds download, quality, general and network settings and the
// library root folders.
package settings

import (
	"errors"
	"log/slog"
	"sync"

	"generic-arr-project/internal/library/tmdb"
	"generic-arr-project/internal/store"
)

// Service reads and saves settings.
type Service struct {
	store *store.Store
	tmdb  *tmdb.Provider
	log   *slog.Logger

	netMu   sync.Mutex // guards network and serializes network updates
	network NetworkApplier
}

// Deps are the Service's collaborators.
type Deps struct {
	Store *store.Store
	TMDB  *tmdb.Provider // validates a new TMDB key before it is saved
	Log   *slog.Logger
}

// New creates a Service.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.TMDB == nil || d.Log == nil {
		return nil, errors.New("settings: Store, TMDB and Log are required")
	}
	return &Service{store: d.Store, tmdb: d.TMDB, log: d.Log}, nil
}
