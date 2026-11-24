package vpnconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/provider"
)

// VPN health check timings.
const (
	// StaleAfter: a tunnel that renews its credentials and has had no
	// WireGuard handshake for this long has been dropped by the server.
	// (WireGuard re-handshakes at least every ~2.5 min with a 25 s
	// keepalive and rejects sessions older than 3 min.)
	StaleAfter = 4 * time.Minute
)

// VPNInput selects a provider and its settings. Secret values left empty
// keep the saved ones when the provider is unchanged.
type VPNInput struct {
	Provider string          `json:"provider"`
	Values   provider.Values `json:"values"`
}

// VPNProviders lists the available providers and their fields.
func (s *Service) VPNProviders() []provider.Definition { return s.engine.Providers().List() }

// VPNOptions lists the choices for a provider's select field.
func (s *Service) VPNOptions(ctx context.Context, providerID, field string) ([]provider.Option, error) {
	def, err := s.engine.Providers().Get(providerID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperr.ErrNotFound, err)
	}
	f, ok := def.Field(field)
	if !ok || !f.Options {
		return nil, fmt.Errorf("%s field %q has no options: %w", def.ID, field, apperr.ErrNotFound)
	}
	return def.Options(ctx, field)
}

// prepareVPN validates input for saving or testing: known provider, values
// matching its fields (saved secrets filled in for the same provider), and
// the provider's own online check. Field errors name the value's key.
func (s *Service) prepareVPN(ctx context.Context, in VPNInput, saved *engine.SavedVPN) (provider.Definition, provider.Values, error) {
	def, err := s.engine.Providers().Get(in.Provider)
	if err != nil {
		return provider.Definition{}, nil, apperr.Invalid("provider", "%v", err)
	}
	values := in.Values
	if saved != nil && saved.Def.ID == def.ID {
		values = def.MergeSecrets(values, saved.Values)
	}
	values, err = def.Normalize(values)
	if err == nil {
		err = def.Check(ctx, values)
	}
	if err != nil {
		return provider.Definition{}, nil, apperr.FromFieldError(err)
	}
	return def, values, nil
}

func upsertVPNParams(def provider.Definition, values provider.Values) (db.UpsertVPNSettingsParams, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return db.UpsertVPNSettingsParams{}, fmt.Errorf("encode %s settings: %w", def.ID, err)
	}
	return db.UpsertVPNSettingsParams{Provider: def.ID, Settings: string(raw)}, nil
}

// SetVPN validates and saves the VPN, replacing any previous one, then
// restarts the tunnel and engine with it.
func (s *Service) SetVPN(ctx context.Context, in VPNInput) (VPNSettings, error) {
	saved, err := engine.LoadSavedVPN(ctx, s.store, s.engine.Providers())
	if err != nil {
		return VPNSettings{}, err
	}
	def, values, err := s.prepareVPN(ctx, in, saved)
	if err != nil {
		return VPNSettings{}, err
	}
	params, err := upsertVPNParams(def, values)
	if err != nil {
		return VPNSettings{}, err
	}
	if err := s.store.UpsertVPNSettings(ctx, params); err != nil {
		return VPNSettings{}, fmt.Errorf("save vpn settings: %w", err)
	}
	s.log.Info().Str("provider", def.ID).Msg("vpn settings saved")
	if err := s.engine.Apply(ctx); err != nil {
		s.log.Warn().Err(err).Msg("vpn settings saved but runtime failed")
	}
	return s.VPNSettings(ctx)
}

// RemoveVPN removes the VPN. Indexer searches and downloads then use the
// normal connection; the UI warns before calling this.
func (s *Service) RemoveVPN(ctx context.Context) error {
	if err := s.store.DeleteVPNSettings(ctx); err != nil {
		return fmt.Errorf("remove vpn: %w", err)
	}
	s.log.Warn().Msg("vpn removed; indexer and download traffic now uses the direct connection")
	return s.engine.Apply(ctx)
}

// VPNSettings describes the configured VPN and the live tunnel. Secret
// values (private keys, passwords) are never included.
type VPNSettings struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider,omitempty"`
	// Values are the saved non-secret values; SecretsSet names the secret
	// fields that have a saved value.
	Values        provider.Values `json:"values,omitempty"`
	SecretsSet    []string        `json:"secrets_set,omitempty"`
	TunnelUp      bool            `json:"tunnel_up"`
	LastHandshake *time.Time      `json:"last_handshake,omitempty"`
	// TunnelError explains why the configured tunnel is not up.
	TunnelError   string   `json:"tunnel_error,omitempty"`
	Server        string   `json:"server,omitempty"`
	ForwardedPort int      `json:"forwarded_port,omitempty"`
	Addresses     []string `json:"addresses,omitempty"`
	Endpoints     []string `json:"endpoints,omitempty"`
	DNS           []string `json:"dns,omitempty"`
	IgnoredKeys   []string `json:"ignored_keys,omitempty"`
}

// VPNSettings returns the VPN configuration and tunnel state.
func (s *Service) VPNSettings(ctx context.Context) (VPNSettings, error) {
	saved, err := engine.LoadSavedVPN(ctx, s.store, s.engine.Providers())
	if err != nil || saved == nil {
		return VPNSettings{}, err
	}
	out := VPNSettings{Configured: true, Provider: saved.Def.ID}
	out.Values, out.SecretsSet = saved.Def.Redact(saved.Values)

	s.engine.Inspect(func(rt *engine.Runtime, rtErr error) {
		switch {
		case rt != nil && rt.VPN != nil:
			conn := rt.VPN
			out.TunnelUp = true
			out.LastHandshake = lastHandshake(conn.Tunnel)
			out.Server, out.ForwardedPort = conn.Server, int(conn.ForwardedPort)
			out.Addresses, out.Endpoints, out.DNS, out.IgnoredKeys = conn.Summary.Addresses, conn.Summary.Endpoints, conn.Summary.DNS, conn.Summary.IgnoredKeys
		case rtErr != nil:
			out.TunnelError = rtErr.Error()
		}
	})
	return out, nil
}

// lastHandshake returns the newest handshake with any peer, or nil.
func lastHandshake(tun *vpn.Tunnel) *time.Time {
	peers, err := tun.Status()
	if err != nil {
		return nil
	}
	var latest *time.Time
	for _, p := range peers {
		if !p.LastHandshake.IsZero() && (latest == nil || p.LastHandshake.After(*latest)) {
			hs := p.LastHandshake
			latest = &hs
		}
	}
	return latest
}

// VPNTest is the result of testing a VPN.
type VPNTest struct {
	PublicIP    string   `json:"public_ip"`
	IgnoredKeys []string `json:"ignored_keys,omitempty"`
	HasDNS      bool     `json:"dns_configured"`
}

// TestVPN checks a VPN by connecting a temporary tunnel (without port
// forwarding) and asking an IP echo service for the address traffic leaves
// from. With nil input the running tunnel is tested instead: a second
// tunnel with the same static keys would disturb it.
func (s *Service) TestVPN(ctx context.Context, in *VPNInput) (VPNTest, error) {
	if in == nil {
		return s.testRunningVPN(ctx)
	}
	saved, err := engine.LoadSavedVPN(ctx, s.store, s.engine.Providers())
	if err != nil {
		return VPNTest{}, err
	}
	def, values, err := s.prepareVPN(ctx, *in, saved)
	if err != nil {
		return VPNTest{}, err
	}
	p, err := def.Parse(values)
	if err != nil {
		return VPNTest{}, apperr.FromFieldError(err)
	}
	conn, err := p.Connect(ctx, provider.ConnectOptions{}, s.log)
	if err != nil {
		// An expected outcome of testing, not a server failure.
		return VPNTest{}, apperr.Invalid("", "could not connect with %s: %v", def.Name, err)
	}
	defer conn.Close()
	out := VPNTest{IgnoredKeys: conn.Summary.IgnoredKeys, HasDNS: len(conn.Summary.DNS) > 0}
	ip, err := netx.PublicIP(ctx, conn.Tunnel, s.ipEchoURL)
	if err != nil {
		return out, fmt.Errorf("tunnel is up but the public IP check failed: %w", err)
	}
	out.PublicIP = ip.String()
	return out, nil
}

func (s *Service) testRunningVPN(ctx context.Context) (VPNTest, error) {
	n, err := s.engine.VPNNetwork()
	if err != nil {
		return VPNTest{}, err
	}
	ip, err := netx.PublicIP(ctx, n, s.ipEchoURL)
	if err != nil {
		return VPNTest{}, err
	}
	out := VPNTest{PublicIP: ip.String(), HasDNS: true}
	s.engine.Inspect(func(rt *engine.Runtime, _ error) {
		if rt != nil && rt.VPN != nil {
			out.IgnoredKeys, out.HasDNS = rt.VPN.Summary.IgnoredKeys, len(rt.VPN.Summary.DNS) > 0
		}
	})
	return out, nil
}

// CheckVPN keeps the tunnel working. It reconnects (restarting the engine)
// when a provider that renews credentials has stopped handshaking, when
// the connection's upkeep fails (e.g. a forwarded port could not be
// renewed or is about to expire). A tunnel that could not be brought up at
// all leaves no runtime, which the scheduler retries.
func (s *Service) CheckVPN(ctx context.Context) error {
	reason, err := s.vpnRestartReason(ctx)
	if err != nil || reason == "" {
		return err
	}
	s.log.Warn().Str("reason", reason).Msg("reconnecting vpn")
	return s.engine.Apply(ctx)
}

func (s *Service) vpnRestartReason(ctx context.Context) (reason string, err error) {
	s.engine.Inspect(func(rt *engine.Runtime, _ error) { reason, err = s.restartReason(ctx, rt) })
	return reason, err
}

func (s *Service) restartReason(ctx context.Context, rt *engine.Runtime) (string, error) {
	if rt == nil {
		return "", nil // no runtime at all: the scheduler retries that
	}
	now := s.now()
	conn := rt.VPN
	if conn == nil {
		return "", nil // no VPN configured
	}
	if conn.Reprovision {
		hs := lastHandshake(conn.Tunnel)
		if hs == nil || now.Sub(*hs) > StaleAfter {
			return fmt.Sprintf("no WireGuard handshake with %s for over %s", conn.Server, StaleAfter), nil
		}
	}
	if conn.Maintain != nil {
		if err := conn.Maintain(ctx, now); err != nil {
			return err.Error(), nil
		}
	}
	return "", nil
}
