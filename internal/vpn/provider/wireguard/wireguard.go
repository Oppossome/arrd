// Package wireguard is the VPN provider for a static wg-quick config, as
// exported by most VPN services and self-hosted WireGuard servers.
package wireguard

import (
	"context"
	"strings"

	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/provider"

	"github.com/rs/zerolog"
)

// ID is the provider id.
const ID = "wireguard"

const fieldConfig = "config"

// Definition returns the static WireGuard provider.
func Definition() provider.Definition {
	return provider.Definition{
		ID:          ID,
		Name:        "WireGuard config",
		Description: "A wg-quick config file from your VPN service or your own WireGuard server.",
		Fields: []provider.Field{{
			Key:         fieldConfig,
			Label:       "WireGuard config",
			Kind:        provider.KindTextarea,
			Required:    true,
			Secret:      true, // holds the private key
			FileAccept:  ".conf",
			Placeholder: "[Interface]\nPrivateKey = …\nAddress = …\n\n[Peer]\nPublicKey = …\nEndpoint = …\nAllowedIPs = 0.0.0.0/0",
			Help:        "PostUp, Table and similar wg-quick entries have no effect: arrd runs the tunnel inside its own process.",
		}},
		Parse: parse,
		Check: func(_ context.Context, v provider.Values) error {
			_, err := parse(v)
			return err
		},
	}
}

func parse(v provider.Values) (provider.Provider, error) {
	cfg, err := vpn.ParseConfig(strings.NewReader(v[fieldConfig]))
	if err != nil {
		return nil, provider.Fieldf(fieldConfig, "%v", err)
	}
	return static{cfg: cfg}, nil
}

type static struct{ cfg *vpn.Config }

// Connect brings the configured tunnel up. A static config has nothing to
// renew and no forwarded port to request.
func (s static) Connect(ctx context.Context, _ provider.ConnectOptions, log *zerolog.Logger) (*provider.Connection, error) {
	for _, k := range s.cfg.IgnoredKeys {
		log.Warn().Str("entry", k).Msg("wireguard config entry has no effect in arrd")
	}
	tun, err := vpn.Start(ctx, s.cfg, log, provider.HandshakeTimeout)
	if err != nil {
		return nil, err
	}
	sum := provider.Summarize(s.cfg)
	server := ""
	if len(sum.Endpoints) > 0 {
		server = sum.Endpoints[0]
	}
	return &provider.Connection{Tunnel: tun, Server: server, Summary: sum}, nil
}
