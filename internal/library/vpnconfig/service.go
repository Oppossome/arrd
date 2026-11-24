// Package vpnconfig saves, tests and monitors the VPN configuration.
package vpnconfig

import (
	"context"
	"errors"
	"time"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/store/db"

	"github.com/rs/zerolog"
)

// DefaultIPEchoURL answers with the caller's public IP as plain text.
const DefaultIPEchoURL = "https://api.ipify.org"

// Service manages the VPN settings.
type Service struct {
	store     *store.Store
	engine    *engine.Manager
	ipEchoURL string
	log       *zerolog.Logger
	now       func() time.Time
}

// Deps are the Service's collaborators.
type Deps struct {
	Store     *store.Store
	Engine    *engine.Manager
	IPEchoURL string // default DefaultIPEchoURL
	Log       *zerolog.Logger
	Now       func() time.Time // default time.Now
}

// New creates a Service.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Engine == nil || d.Log == nil {
		return nil, errors.New("vpnconfig: Store, Engine and Log are required")
	}
	if d.IPEchoURL == "" {
		d.IPEchoURL = DefaultIPEchoURL
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{store: d.Store, engine: d.Engine, ipEchoURL: d.IPEchoURL, log: d.Log, now: d.Now}, nil
}

// SetupParams validates the VPN chosen during onboarding and returns it
// ready to save. Field errors are named "vpn.<key>".
func (s *Service) SetupParams(ctx context.Context, in VPNInput) (db.UpsertVPNSettingsParams, error) {
	def, values, err := s.prepareVPN(ctx, in, nil)
	if err != nil {
		return db.UpsertVPNSettingsParams{}, apperr.PrefixField("vpn.", err)
	}
	return upsertVPNParams(def, values)
}
