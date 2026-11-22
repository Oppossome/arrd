package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"generic-arr-project/internal/geoip"
	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/vpn/provider"
)

const (
	tmdbTimeout = 20 * time.Second
	// geoIPTimeout bounds one geolocation database download (~8 MiB).
	geoIPTimeout = 5 * time.Minute
	// DefaultIPEchoURL answers with the caller's public IP as plain text.
	DefaultIPEchoURL = "https://api.ipify.org"
)

// Service is the application core used by the HTTP API and scheduler.
type Service struct {
	store         *store.Store
	direct        netx.Network
	newRuntime    RuntimeFactory
	tmdbBaseURL   string
	ipEchoURL     string
	posterBaseURL string
	vpn           *provider.Registry
	geo           *geoip.DB
	// definitions is replaced when indexer definitions are reloaded.
	definitions atomic.Pointer[indexer.Catalog]
	// definitionsMu serializes changes to the definitions directory.
	definitionsMu  sync.Mutex
	dataDir        string
	definitionsDir string
	log            *slog.Logger

	// rtMu guards the runtime. Work that uses the engine holds a read lock
	// for its duration; ApplyRuntime takes the write lock, so an engine is
	// never closed underneath a grab or an import.
	rtMu  sync.RWMutex
	rt    *Runtime
	rtErr error // why there is no runtime, if there is none

	setupDone atomic.Bool
	apiKey    atomic.Pointer[string]

	backoffMu sync.Mutex
	backoff   map[int64]indexerBackoff // indexer id -> failure state

	searchersMu sync.Mutex
	searchers   map[int64]cachedSearcher // indexer id -> built searcher

	now func() time.Time // replaceable in tests

	logins  loginLimiter
	posters posterFetcher

	importMu    sync.Mutex
	importRetry map[int64]time.Time // download id -> earliest next import attempt
}

// Deps are the Service's collaborators.
type Deps struct {
	Store *store.Store
	// NewRuntime builds the torrent engine (and VPN tunnel) from settings.
	NewRuntime RuntimeFactory
	// DataDir is used to suggest default paths during onboarding.
	DataDir string
	// DefinitionsDir holds the Cardigann site definitions (*.yml) indexers
	// are built from; empty or missing means none.
	DefinitionsDir string
	Log            *slog.Logger

	// Optional overrides, mainly for tests.
	Direct        netx.Network       // default netx.Direct{}
	TMDBBaseURL   string             // default TMDB API
	IPEchoURL     string             // default DefaultIPEchoURL
	PosterBaseURL string             // default DefaultPosterBaseURL
	GeoIPBaseURL  string             // default geoip.DefaultBaseURL
	VPNProviders  *provider.Registry // default DefaultVPNProviders(Direct)
}

// New creates a Service. Call Start before use.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.NewRuntime == nil || d.Log == nil || d.DataDir == "" {
		return nil, errors.New("library: Store, NewRuntime, DataDir and Log are required")
	}
	if d.Direct == nil {
		d.Direct = netx.Direct{}
	}
	if d.PosterBaseURL == "" {
		d.PosterBaseURL = DefaultPosterBaseURL
	}
	if d.IPEchoURL == "" {
		d.IPEchoURL = DefaultIPEchoURL
	}
	if d.VPNProviders == nil {
		reg, err := DefaultVPNProviders(d.Direct)
		if err != nil {
			return nil, err
		}
		d.VPNProviders = reg
	}
	s := &Service{
		store:          d.Store,
		direct:         d.Direct,
		newRuntime:     d.NewRuntime,
		tmdbBaseURL:    d.TMDBBaseURL,
		ipEchoURL:      d.IPEchoURL,
		posterBaseURL:  d.PosterBaseURL,
		vpn:            d.VPNProviders,
		geo:            geoip.New(filepath.Join(d.DataDir, "cache", "geoip"), d.GeoIPBaseURL, netx.HTTPClient(d.Direct, geoIPTimeout), d.Log),
		dataDir:        d.DataDir,
		definitionsDir: d.DefinitionsDir,
		log:            d.Log.With("component", "library"),
		rtErr:          errors.New("setup is not complete"),
		backoff:        map[int64]indexerBackoff{},
		searchers:      map[int64]cachedSearcher{},
		importRetry:    map[int64]time.Time{},
		now:            time.Now,
	}
	s.loadDefinitions()
	return s, nil
}

// Start loads the saved settings and, if onboarding is complete, starts the
// torrent engine. A runtime failure (e.g. the VPN is unreachable) is
// logged and retried by the scheduler rather than failing startup.
func (s *Service) Start(ctx context.Context) error {
	if err := s.geo.Open(); err != nil {
		s.log.Warn("peer countries unavailable until the ip geolocation database is downloaded again", "err", err)
	}
	app, err := s.store.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // onboarding pending
	}
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	s.apiKey.Store(&app.APIKey)
	s.setupDone.Store(true)
	if err := s.ApplyRuntime(ctx); err != nil {
		s.log.Error("downloads unavailable", "err", err)
	}
	return nil
}

// SetupComplete reports whether onboarding has finished.
func (s *Service) SetupComplete() bool { return s.setupDone.Load() }

// trafficNetwork is the network indexer and download traffic uses: the VPN
// whenever one is configured, never falling back to direct while its
// tunnel is down, and direct when there is none.
func (s *Service) trafficNetwork(ctx context.Context) (netx.Network, error) {
	hasVPN, err := s.vpnConfigured(ctx)
	if err != nil {
		return nil, err
	}
	if !hasVPN {
		return s.direct, nil
	}
	return s.vpnNetwork()
}

// vpnNetwork returns the live tunnel, or why there is none.
func (s *Service) vpnNetwork() (netx.Network, error) {
	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	if tun := s.rt.tunnel(); tun != nil {
		return tun, nil
	}
	if s.rtErr != nil {
		return nil, fmt.Errorf("vpn is down: %w", s.rtErr)
	}
	return nil, errors.New("no VPN is configured")
}

// tmdb returns a TMDB client using the saved API key.
func (s *Service) tmdb(ctx context.Context) (*metadata.TMDB, error) {
	app, err := s.store.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("setup is not complete")
	}
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	return &metadata.TMDB{APIKey: app.TMDBAPIKey, BaseURL: s.tmdbBaseURL, HTTP: netx.HTTPClient(s.direct, tmdbTimeout)}, nil
}
