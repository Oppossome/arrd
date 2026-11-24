// Package setup runs onboarding: it validates and saves everything the
// setup wizard collects in one go.
package setup

import (
	"errors"

	"generic-arr-project/internal/library/auth"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/library/vpnconfig"
	"generic-arr-project/internal/store"

	"github.com/rs/zerolog"
)

// Service completes onboarding.
type Service struct {
	store    *store.Store
	auth     *auth.Service
	settings *settings.Service
	vpn      *vpnconfig.Service
	indexers *indexers.Service
	engine   *engine.Manager
	dataDir  string
	log      *zerolog.Logger
}

// Deps are the Service's collaborators.
type Deps struct {
	Store    *store.Store
	Auth     *auth.Service
	Settings *settings.Service
	VPN      *vpnconfig.Service
	Indexers *indexers.Service
	Engine   *engine.Manager // started once setup is saved
	// DataDir is used to suggest default paths.
	DataDir string
	Log     *zerolog.Logger
}

// New creates a Service.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Auth == nil || d.Settings == nil || d.VPN == nil || d.Indexers == nil || d.Engine == nil || d.DataDir == "" || d.Log == nil {
		return nil, errors.New("setup: Store, Auth, Settings, VPN, Indexers, Engine, DataDir and Log are required")
	}
	return &Service{
		store: d.Store, auth: d.Auth, settings: d.Settings, vpn: d.VPN, indexers: d.Indexers,
		engine: d.Engine, dataDir: d.DataDir, log: d.Log,
	}, nil
}
