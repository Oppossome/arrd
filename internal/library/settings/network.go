package settings

import (
	"context"
	"errors"
	"fmt"
)

// NetworkSettings control how the web UI is reachable.
type NetworkSettings struct {
	ExposeLAN bool
	Hostname  string // the mDNS hostname announced while ExposeLAN is on
	LANURL    string // the web UI's address on the local network; empty while off
}

// NetworkSettingsInput updates network settings.
type NetworkSettingsInput struct {
	ExposeLAN bool
}

// NetworkApplier puts network settings into effect. The app registers one
// with SetNetworkApplier once its HTTP server exists.
type NetworkApplier interface {
	// ApplyLAN serves on all interfaces and announces the hostname over
	// mDNS when enabled, or reverts to the configured listen address. On
	// error the previous state is kept.
	ApplyLAN(enabled bool) error
	Hostname() string
	LANURL() string
}

var errNoNetworkApplier = errors.New("network settings are unavailable: the HTTP server is not running")

// SetNetworkApplier registers the component that applies network settings.
func (s *Service) SetNetworkApplier(a NetworkApplier) {
	s.netMu.Lock()
	defer s.netMu.Unlock()
	s.network = a
}

// NetworkSettings returns the saved network settings and the live LAN URL.
func (s *Service) NetworkSettings(ctx context.Context) (NetworkSettings, error) {
	s.netMu.Lock()
	defer s.netMu.Unlock()
	return s.networkSettingsLocked(ctx)
}

func (s *Service) networkSettingsLocked(ctx context.Context) (NetworkSettings, error) {
	if s.network == nil {
		return NetworkSettings{}, errNoNetworkApplier
	}
	app, err := s.store.GetAppSettings(ctx)
	if err != nil {
		return NetworkSettings{}, fmt.Errorf("load settings: %w", err)
	}
	return NetworkSettings{ExposeLAN: app.ExposeLAN, Hostname: s.network.Hostname(), LANURL: s.network.LANURL()}, nil
}

// UpdateNetworkSettings applies the settings and then saves them. Nothing is
// saved if they cannot be applied, and a failed save undoes the apply.
func (s *Service) UpdateNetworkSettings(ctx context.Context, in NetworkSettingsInput) (NetworkSettings, error) {
	s.netMu.Lock()
	defer s.netMu.Unlock()
	current, err := s.networkSettingsLocked(ctx)
	if err != nil {
		return NetworkSettings{}, err
	}
	if err := s.network.ApplyLAN(in.ExposeLAN); err != nil {
		return NetworkSettings{}, fmt.Errorf("apply LAN access: %w", err)
	}
	if err := s.store.SetExposeLAN(ctx, in.ExposeLAN); err != nil {
		if undoErr := s.network.ApplyLAN(current.ExposeLAN); undoErr != nil {
			s.log.Error("restore LAN access after failed save", "err", undoErr)
		}
		return NetworkSettings{}, fmt.Errorf("save LAN access: %w", err)
	}
	return s.networkSettingsLocked(ctx)
}
