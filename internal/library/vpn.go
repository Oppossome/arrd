package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/pia"
	"generic-arr-project/internal/vpn/provider"
	"generic-arr-project/internal/vpn/provider/wireguard"
)

// VPN health check timings.
const (
	// vpnStaleAfter: a tunnel that renews its credentials and has had no
	// WireGuard handshake for this long has been dropped by the server.
	// (WireGuard re-handshakes at least every ~2.5 min with a 25 s
	// keepalive and rejects sessions older than 3 min.)
	vpnStaleAfter = 4 * time.Minute
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

// VPNInput selects a provider and its settings. Secret values left empty
// keep the saved ones when the provider is unchanged.
type VPNInput struct {
	Provider string          `json:"provider"`
	Values   provider.Values `json:"values"`
}

// savedVPN is the stored VPN choice.
type savedVPN struct {
	def    provider.Definition
	values provider.Values
}

// loadVPN returns the saved VPN, or nil if none is configured.
func loadVPN(ctx context.Context, q interface {
	GetVPNSettings(context.Context) (db.VPNSetting, error)
}, reg *provider.Registry) (*savedVPN, error) {
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
	return &savedVPN{def: def, values: values}, nil
}

// LoadVPNProvider returns the saved VPN, parsed and ready to connect, or
// nil if none is configured. It needs no network access.
func LoadVPNProvider(ctx context.Context, q interface {
	GetVPNSettings(context.Context) (db.VPNSetting, error)
}, reg *provider.Registry) (provider.Provider, error) {
	saved, err := loadVPN(ctx, q, reg)
	if err != nil || saved == nil {
		return nil, err
	}
	p, err := saved.def.Parse(saved.values)
	if err != nil {
		return nil, fmt.Errorf("saved %s settings: %w", saved.def.ID, err)
	}
	return p, nil
}

// VPNProviders lists the available providers and their fields.
func (s *Service) VPNProviders() []provider.Definition { return s.vpn.List() }

// VPNOptions lists the choices for a provider's select field.
func (s *Service) VPNOptions(ctx context.Context, providerID, field string) ([]provider.Option, error) {
	def, err := s.vpn.Get(providerID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	f, ok := def.Field(field)
	if !ok || !f.Options {
		return nil, fmt.Errorf("%s field %q has no options: %w", def.ID, field, ErrNotFound)
	}
	return def.Options(ctx, field)
}

// prepareVPN validates input for saving or testing: known provider, values
// matching its fields (saved secrets filled in for the same provider), and
// the provider's own online check. Field errors name the value's key.
func (s *Service) prepareVPN(ctx context.Context, in VPNInput, saved *savedVPN) (provider.Definition, provider.Values, error) {
	def, err := s.vpn.Get(in.Provider)
	if err != nil {
		return provider.Definition{}, nil, invalid("provider", "%v", err)
	}
	values := in.Values
	if saved != nil && saved.def.ID == def.ID {
		values = def.MergeSecrets(values, saved.values)
	}
	values, err = def.Normalize(values)
	if err == nil {
		err = def.Check(ctx, values)
	}
	if err != nil {
		return provider.Definition{}, nil, fieldValidation(err)
	}
	return def, values, nil
}

// fieldValidation turns a provider field error into a ValidationError.
func fieldValidation(err error) error {
	var fe *provider.FieldError
	if errors.As(err, &fe) {
		return invalid(fe.Field, "%s", fe.Message)
	}
	return err
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
	saved, err := loadVPN(ctx, s.store, s.vpn)
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
	s.log.Info("vpn settings saved", "provider", def.ID)
	if err := s.ApplyRuntime(ctx); err != nil {
		s.log.Warn("vpn settings saved but runtime failed", "err", err)
	}
	return s.VPNSettings(ctx)
}

// RemoveVPN removes the VPN. Indexer searches and downloads then use the
// normal connection; the UI warns before calling this.
func (s *Service) RemoveVPN(ctx context.Context) error {
	if err := s.store.DeleteVPNSettings(ctx); err != nil {
		return fmt.Errorf("remove vpn: %w", err)
	}
	s.log.Warn("vpn removed; indexer and download traffic now uses the direct connection")
	return s.ApplyRuntime(ctx)
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
	saved, err := loadVPN(ctx, s.store, s.vpn)
	if err != nil || saved == nil {
		return VPNSettings{}, err
	}
	out := VPNSettings{Configured: true, Provider: saved.def.ID}
	out.Values, out.SecretsSet = saved.def.Redact(saved.values)

	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	switch {
	case s.rt != nil && s.rt.VPN != nil:
		conn := s.rt.VPN
		out.TunnelUp = true
		out.LastHandshake = lastHandshake(conn.Tunnel)
		out.Server, out.ForwardedPort = conn.Server, int(conn.ForwardedPort)
		out.Addresses, out.Endpoints, out.DNS, out.IgnoredKeys = conn.Summary.Addresses, conn.Summary.Endpoints, conn.Summary.DNS, conn.Summary.IgnoredKeys
	case s.rtErr != nil:
		out.TunnelError = s.rtErr.Error()
	}
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
	saved, err := loadVPN(ctx, s.store, s.vpn)
	if err != nil {
		return VPNTest{}, err
	}
	def, values, err := s.prepareVPN(ctx, *in, saved)
	if err != nil {
		return VPNTest{}, err
	}
	p, err := def.Parse(values)
	if err != nil {
		return VPNTest{}, fieldValidation(err)
	}
	conn, err := p.Connect(ctx, provider.ConnectOptions{}, s.log)
	if err != nil {
		// An expected outcome of testing, not a server failure.
		return VPNTest{}, invalid("", "could not connect with %s: %v", def.Name, err)
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
	n, err := s.vpnNetwork()
	if err != nil {
		return VPNTest{}, err
	}
	ip, err := netx.PublicIP(ctx, n, s.ipEchoURL)
	if err != nil {
		return VPNTest{}, err
	}
	out := VPNTest{PublicIP: ip.String(), HasDNS: true}
	s.rtMu.RLock()
	if s.rt != nil && s.rt.VPN != nil {
		out.IgnoredKeys, out.HasDNS = s.rt.VPN.Summary.IgnoredKeys, len(s.rt.VPN.Summary.DNS) > 0
	}
	s.rtMu.RUnlock()
	return out, nil
}

// CheckVPN keeps the tunnel working. It reconnects (restarting the engine)
// when a provider that renews credentials has stopped handshaking, when
// the connection's upkeep fails (e.g. a forwarded port could not be
// renewed or is about to expire). A tunnel that could not be brought up at
// all leaves no runtime, which retryRuntime retries.
func (s *Service) CheckVPN(ctx context.Context) error {
	reason, err := s.vpnRestartReason(ctx)
	if err != nil || reason == "" {
		return err
	}
	s.log.Warn("reconnecting vpn", "reason", reason)
	return s.ApplyRuntime(ctx)
}

func (s *Service) vpnRestartReason(ctx context.Context) (string, error) {
	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	rt := s.rt
	if rt == nil {
		return "", nil // no runtime at all: retryRuntime handles that
	}
	now := s.now()
	conn := rt.VPN
	if conn == nil {
		return "", nil // no VPN configured
	}
	if conn.Reprovision {
		hs := lastHandshake(conn.Tunnel)
		if hs == nil || now.Sub(*hs) > vpnStaleAfter {
			return fmt.Sprintf("no WireGuard handshake with %s for over %s", conn.Server, vpnStaleAfter), nil
		}
	}
	if conn.Maintain != nil {
		if err := conn.Maintain(ctx, now); err != nil {
			return err.Error(), nil
		}
	}
	return "", nil
}
