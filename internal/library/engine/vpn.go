package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/vpn/pia"
	"generic-arr-project/internal/vpn/provider"
	"generic-arr-project/internal/vpn/provider/wireguard"
)

// DefaultVPNProviders is the registry arrd ships with: a static WireGuard
// config and PIA. Provider APIs are reached over direct (a tunnel cannot
// be used to set itself up).
func DefaultVPNProviders(direct netx.Network) (*provider.Registry, error) {
	c, err := pia.NewClient(direct)
	if err != nil {
		return nil, err
	}
	return provider.NewRegistry(wireguard.Definition(), pia.Definition(c))
}

// VPNStore reads the saved VPN settings.
type VPNStore interface {
	GetVPNSettings(context.Context) (db.VPNSetting, error)
}

// SavedVPN is the stored VPN choice.
type SavedVPN struct {
	Def    provider.Definition
	Values provider.Values
}

// LoadSavedVPN returns the saved VPN, or nil if none is configured.
func LoadSavedVPN(ctx context.Context, q VPNStore, reg *provider.Registry) (*SavedVPN, error) {
	row, err := q.GetVPNSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load vpn settings: %w", err)
	}
	def, err := reg.Get(row.Provider)
	if err != nil {
		return nil, fmt.Errorf("saved vpn settings: %w", err)
	}
	var values provider.Values
	if err := json.Unmarshal([]byte(row.Settings), &values); err != nil {
		return nil, fmt.Errorf("saved %s settings: %w", def.ID, err)
	}
	return &SavedVPN{Def: def, Values: values}, nil
}

// LoadVPNProvider returns the saved VPN, parsed and ready to connect, or
// nil if none is configured. It needs no network access.
func LoadVPNProvider(ctx context.Context, q VPNStore, reg *provider.Registry) (provider.Provider, error) {
	saved, err := LoadSavedVPN(ctx, q, reg)
	if err != nil || saved == nil {
		return nil, err
	}
	p, err := saved.Def.Parse(saved.Values)
	if err != nil {
		return nil, fmt.Errorf("saved %s settings: %w", saved.Def.ID, err)
	}
	return p, nil
}
