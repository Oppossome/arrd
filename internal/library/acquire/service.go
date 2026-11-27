// Package acquire finds, grabs, downloads and imports releases: indexer
// searches, RSS sync, the download queue and importing into the library.
package acquire

import (
	"context"
	"errors"
	"sync"
	"time"

	"generic-arr-project/internal/geoip"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/store"

	"github.com/rs/zerolog"
)

// Service grabs and imports releases.
type Service struct {
	store    *store.Store
	engine   *engine.Manager
	settings *settings.Service
	indexers *indexers.Service
	media    *media.Service
	geo      *geoip.DB
	notifier LibraryNotifier
	log      *zerolog.Logger

	importMu    sync.Mutex
	importRetry map[int64]time.Time // download id -> earliest next import attempt
}

// Deps are the Service's collaborators.
type Deps struct {
	Store    *store.Store
	Engine   *engine.Manager
	Settings *settings.Service
	Indexers *indexers.Service
	Media    *media.Service
	Geo      *geoip.DB       // locates peers
	Notifier LibraryNotifier // told after each import
	Log      *zerolog.Logger
}

// LibraryNotifier is told when imports change the library. It must not
// block: it is called while downloads are being processed.
type LibraryNotifier interface {
	LibraryChanged()
}

// New creates a Service and registers it to restore downloads whenever
// the engine starts.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Engine == nil || d.Settings == nil || d.Indexers == nil || d.Media == nil || d.Geo == nil || d.Notifier == nil || d.Log == nil {
		return nil, errors.New("acquire: Store, Engine, Settings, Indexers, Media, Geo, Notifier and Log are required")
	}
	s := &Service{
		store: d.Store, engine: d.Engine, settings: d.Settings, indexers: d.Indexers, media: d.Media,
		geo: d.Geo, notifier: d.Notifier, log: d.Log, importRetry: map[int64]time.Time{},
	}
	d.Engine.OnEngineStart(s.RestoreDownloads)
	return s, nil
}

// SearchNewMovie runs an automatic search for a movie; it satisfies
// media.WantedSearcher.
func (s *Service) SearchNewMovie(ctx context.Context, movieID int64) error {
	movie, err := s.store.GetMovie(ctx, movieID)
	if err != nil {
		return apperr.NotFound(err, "movie", movieID)
	}
	_, err = s.autoSearch(ctx, movieWant{s: s, movie: movie}, nil)
	return err
}

// SearchNewSeries searches for a series' wanted episodes; it satisfies
// media.WantedSearcher.
func (s *Service) SearchNewSeries(ctx context.Context, seriesID int64) error {
	return s.SearchWantedEpisodes(ctx, &seriesID)
}
