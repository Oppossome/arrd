// Package media is the movie and TV library and TMDB browsing: adding,
// listing and refreshing titles, discovery, recommendations and artwork.
package media

import (
	"context"
	"errors"
	"time"

	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/library/tmdb"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"

	"github.com/rs/zerolog"
)

// WantedSearcher searches for what a newly added title wants.
type WantedSearcher interface {
	// SearchNewMovie searches for a movie and grabs the best release.
	SearchNewMovie(ctx context.Context, movieID int64) error
	// SearchNewSeries searches for a series' wanted episodes.
	SearchNewSeries(ctx context.Context, seriesID int64) error
}

// Service manages the library.
type Service struct {
	store         *store.Store
	settings      *settings.Service
	tmdb          *tmdb.Provider
	searcher      WantedSearcher
	direct        netx.Network
	posterBaseURL string
	dataDir       string
	log           *zerolog.Logger
	now           func() time.Time
	posters       posterFetcher
}

// Deps are the Service's collaborators.
type Deps struct {
	Store    *store.Store
	Settings *settings.Service
	TMDB     *tmdb.Provider
	// Searcher runs the search requested when a title is added.
	Searcher WantedSearcher
	Direct   netx.Network
	// DataDir holds the poster cache.
	DataDir       string
	PosterBaseURL string // default DefaultPosterBaseURL
	Log           *zerolog.Logger
	Now           func() time.Time // default time.Now
}

// New creates a Service.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Settings == nil || d.TMDB == nil || d.Searcher == nil || d.Direct == nil || d.DataDir == "" || d.Log == nil {
		return nil, errors.New("media: Store, Settings, TMDB, Searcher, Direct, DataDir and Log are required")
	}
	if d.PosterBaseURL == "" {
		d.PosterBaseURL = DefaultPosterBaseURL
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{
		store: d.Store, settings: d.Settings, tmdb: d.TMDB, searcher: d.Searcher, direct: d.Direct,
		posterBaseURL: d.PosterBaseURL, dataDir: d.DataDir, log: d.Log, now: d.Now,
	}, nil
}
