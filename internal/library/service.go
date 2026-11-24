// Package library wires arrd's application services together. The logic
// lives in the subpackages:
//
//   - auth: admin password, sessions, the API key and setup state
//   - settings: download, quality and general settings, root folders
//   - engine: the torrent engine and VPN tunnel, and the traffic network
//   - vpnconfig: saving, testing and monitoring the VPN
//   - indexers: configured indexers, site definitions and searching them
//   - media: the movie and TV library, TMDB browsing and artwork
//   - acquire: searching, grabbing, downloading and importing releases
//   - setup: onboarding
//   - apperr: the error kinds the services return
package library

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"generic-arr-project/internal/geoip"
	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/auth"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/library/setup"
	"generic-arr-project/internal/library/tmdb"
	"generic-arr-project/internal/library/vpnconfig"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/vpn/provider"

	"github.com/rs/zerolog"
)

// geoIPTimeout bounds one geolocation database download (~8 MiB).
const geoIPTimeout = 5 * time.Minute

// Services are the application services used by the HTTP API, plus what
// the scheduler needs.
type Services struct {
	Auth     *auth.Service
	Settings *settings.Service
	Engine   *engine.Manager
	VPN      *vpnconfig.Service
	Indexers *indexers.Service
	TMDB     *tmdb.Provider
	Media    *media.Service
	Acquire  *acquire.Service
	Setup    *setup.Service

	geo *geoip.DB
	log *zerolog.Logger
}

// Deps are the services' shared collaborators.
type Deps struct {
	Store *store.Store
	// NewRuntime builds the torrent engine (and VPN tunnel) from settings.
	NewRuntime engine.Factory
	// DataDir is used to suggest default paths during onboarding and holds
	// the poster and geolocation caches.
	DataDir string
	// DefinitionsDir holds the Cardigann site definitions (*.yml) indexers
	// are built from; empty or missing means none.
	DefinitionsDir string
	Log            *zerolog.Logger

	// Optional overrides, mainly for tests.
	Direct        netx.Network       // default netx.Direct{}
	TMDBBaseURL   string             // default TMDB API
	IPEchoURL     string             // default vpnconfig.DefaultIPEchoURL
	PosterBaseURL string             // default media.DefaultPosterBaseURL
	GeoIPBaseURL  string             // default geoip.DefaultBaseURL
	VPNProviders  *provider.Registry // default engine.DefaultVPNProviders(Direct)
	Now           func() time.Time   // default time.Now
}

// New builds the services. Call Start before use.
func New(d Deps) (*Services, error) {
	if d.Store == nil || d.NewRuntime == nil || d.Log == nil || d.DataDir == "" {
		return nil, errors.New("library: Store, NewRuntime, DataDir and Log are required")
	}
	if d.Direct == nil {
		d.Direct = netx.Direct{}
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.VPNProviders == nil {
		reg, err := engine.DefaultVPNProviders(d.Direct)
		if err != nil {
			return nil, err
		}
		d.VPNProviders = reg
	}
	logger := d.Log.With().Str("component", "library").Logger()
	log := &logger
	s := &Services{
		geo: geoip.New(filepath.Join(d.DataDir, "cache", "geoip"), d.GeoIPBaseURL, netx.HTTPClient(d.Direct, geoIPTimeout), d.Log),
		log: log,
	}
	now := d.Now

	var err error
	if s.TMDB, err = tmdb.New(d.Store, d.Direct, d.TMDBBaseURL); err != nil {
		return nil, err
	}
	if s.Auth, err = auth.New(auth.Deps{Store: d.Store, Log: log, Now: now}); err != nil {
		return nil, err
	}
	if s.Settings, err = settings.New(settings.Deps{Store: d.Store, TMDB: s.TMDB, Log: log}); err != nil {
		return nil, err
	}
	if s.Engine, err = engine.New(engine.Deps{Store: d.Store, NewRuntime: d.NewRuntime, Direct: d.Direct, VPN: d.VPNProviders, Log: log}); err != nil {
		return nil, err
	}
	if s.VPN, err = vpnconfig.New(vpnconfig.Deps{Store: d.Store, Engine: s.Engine, IPEchoURL: d.IPEchoURL, Log: log, Now: now}); err != nil {
		return nil, err
	}
	if s.Indexers, err = indexers.New(indexers.Deps{Store: d.Store, Engine: s.Engine, DefinitionsDir: d.DefinitionsDir, Log: log}); err != nil {
		return nil, err
	}
	if s.Media, err = media.New(media.Deps{
		Store: d.Store, Settings: s.Settings, TMDB: s.TMDB, Searcher: wantedSearcher{s},
		Direct: d.Direct, DataDir: d.DataDir, PosterBaseURL: d.PosterBaseURL, Log: log, Now: now,
	}); err != nil {
		return nil, err
	}
	if s.Acquire, err = acquire.New(acquire.Deps{
		Store: d.Store, Engine: s.Engine, Settings: s.Settings, Indexers: s.Indexers, Media: s.Media, Geo: s.geo, Log: log,
	}); err != nil {
		return nil, err
	}
	if s.Setup, err = setup.New(setup.Deps{
		Store: d.Store, Auth: s.Auth, Settings: s.Settings, VPN: s.VPN, Indexers: s.Indexers, Engine: s.Engine,
		DataDir: d.DataDir, Log: log,
	}); err != nil {
		return nil, err
	}
	return s, nil
}

// wantedSearcher hands media's search-after-add to acquire, which is built
// after media.
type wantedSearcher struct{ s *Services }

func (w wantedSearcher) SearchNewMovie(ctx context.Context, id int64) error {
	return w.s.Acquire.SearchNewMovie(ctx, id)
}

func (w wantedSearcher) SearchNewSeries(ctx context.Context, id int64) error {
	return w.s.Acquire.SearchNewSeries(ctx, id)
}

// Start loads the saved settings and, if onboarding is complete, starts the
// torrent engine. A runtime failure (e.g. the VPN is unreachable) is
// logged and retried by the scheduler rather than failing startup.
func (s *Services) Start(ctx context.Context) error {
	if err := s.geo.Open(); err != nil {
		s.log.Warn().Err(err).Msg("peer countries unavailable until the ip geolocation database is downloaded again")
	}
	done, err := s.Auth.Load(ctx)
	if err != nil || !done {
		return err // !done: onboarding pending
	}
	if err := s.Engine.Apply(ctx); err != nil {
		s.log.Error().Err(err).Msg("downloads unavailable")
	}
	return nil
}

// Stop closes the engine and tunnel. Call on shutdown.
func (s *Services) Stop() { s.Engine.Stop() }

// retryRuntime restarts a failed engine once onboarding is complete.
func (s *Services) retryRuntime(ctx context.Context) {
	if s.Auth.SetupComplete() {
		s.Engine.Retry(ctx)
	}
}
