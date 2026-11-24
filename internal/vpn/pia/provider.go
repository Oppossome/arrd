package pia

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/provider"

	"github.com/rs/zerolog"
)

// ProviderID is the provider id.
const ProviderID = "pia"

// Field keys.
const (
	fieldUsername    = "username"
	fieldPassword    = "password"
	fieldRegion      = "region"
	fieldPortForward = "port_forward"

	// TagPortForward marks regions whose servers can forward a port.
	TagPortForward = "port_forward"
)

// Port forwarding upkeep.
const (
	// bindEvery: PIA drops a forwarded port not re-bound within 15 min.
	bindEvery = 14 * time.Minute
	// renewBefore: replace the port's signature a day before it expires.
	renewBefore = 24 * time.Hour
)

// Definition returns the PIA provider backed by c.
func Definition(c *Client) provider.Definition {
	return provider.Definition{
		ID:          ProviderID,
		Name:        "Private Internet Access",
		Description: "Signs in to PIA and registers a fresh WireGuard key whenever the tunnel starts, so it keeps working after PIA expires old keys.",
		Renews:      true,
		Fields: []provider.Field{
			{Key: fieldUsername, Label: "Username", Kind: provider.KindText, Required: true, Placeholder: "p1234567"},
			{Key: fieldPassword, Label: "Password", Kind: provider.KindPassword, Required: true, Secret: true},
			{Key: fieldRegion, Label: "Region", Kind: provider.KindSelect, Required: true, Options: true},
			{
				Key: fieldPortForward, Label: "Forward a port", Kind: provider.KindToggle,
				Help: "Lets other peers connect to you, which helps speeds and seeding. Only some regions support it.",
			},
		},
		Options: func(ctx context.Context, field string) ([]provider.Option, error) {
			if field != fieldRegion {
				return nil, fmt.Errorf("pia has no options for %q", field)
			}
			return regionOptions(ctx, c)
		},
		Parse: func(v provider.Values) (provider.Provider, error) { return parse(c, v) },
		Check: func(ctx context.Context, v provider.Values) error { return check(ctx, c, v) },
	}
}

func regionOptions(ctx context.Context, c *Client) ([]provider.Option, error) {
	regions, err := c.Regions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]provider.Option, 0, len(regions))
	for _, r := range regions {
		o := provider.Option{Value: r.ID, Label: r.Name}
		if r.PortForward {
			o.Tags = []string{TagPortForward}
			o.Label += " (port forwarding)"
		}
		out = append(out, o)
	}
	return out, nil
}

// settings are parsed PIA values.
type settings struct {
	username, password, region string
	portForward                bool
}

// String omits the password.
func (s settings) String() string {
	return fmt.Sprintf("pia{user=%s region=%s port_forward=%t}", s.username, s.region, s.portForward)
}

func parseSettings(v provider.Values) (settings, error) {
	s := settings{username: v[fieldUsername], password: v[fieldPassword], region: v[fieldRegion]}
	for _, key := range []string{fieldUsername, fieldPassword, fieldRegion} {
		if v[key] == "" {
			return settings{}, provider.Fieldf(key, "is required")
		}
	}
	pf, err := v.Bool(fieldPortForward)
	if err != nil {
		return settings{}, err
	}
	s.portForward = pf
	return s, nil
}

func parse(c *Client, v provider.Values) (provider.Provider, error) {
	s, err := parseSettings(v)
	if err != nil {
		return nil, err
	}
	return &piaProvider{c: c, s: s}, nil
}

// check verifies the region and signs in.
func check(ctx context.Context, c *Client, v provider.Values) error {
	s, err := parseSettings(v)
	if err != nil {
		return err
	}
	regions, err := c.Regions(ctx)
	if err != nil {
		return fmt.Errorf("load pia regions: %w", err)
	}
	region, ok := FindRegion(regions, s.region)
	if !ok {
		return provider.Fieldf(fieldRegion, "unknown or offline region %q", s.region)
	}
	if s.portForward && !region.PortForward {
		return provider.Fieldf(fieldPortForward, "%s does not support port forwarding", region.Name)
	}
	if _, err := c.Token(ctx, s.username, s.password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return provider.Fieldf(fieldPassword, "PIA rejected the username or password")
		}
		return fmt.Errorf("check pia sign-in: %w", err)
	}
	return nil
}

type piaProvider struct {
	c *Client
	s settings
}

// Connect registers a new key with a server in the region and brings the
// tunnel up. With port forwarding requested and enabled it also gets and
// binds a port through the tunnel; if that fails the tunnel is closed, so
// the engine never runs without the port it was promised.
func (p *piaProvider) Connect(ctx context.Context, opts provider.ConnectOptions, log *zerolog.Logger) (*provider.Connection, error) {
	regions, err := p.c.Regions(ctx)
	if err != nil {
		return nil, err
	}
	region, ok := FindRegion(regions, p.s.region)
	if !ok {
		return nil, fmt.Errorf("pia region %q is offline or no longer exists; choose another in Settings", p.s.region)
	}
	forward := opts.PortForward && p.s.portForward
	if forward && !region.PortForward {
		return nil, fmt.Errorf("pia region %q no longer supports port forwarding; choose another in Settings", p.s.region)
	}
	sess, err := p.c.Provision(ctx, p.s.username, p.s.password, region)
	if err != nil {
		return nil, err
	}
	tun, err := vpn.Start(ctx, sess.Config, log, provider.HandshakeTimeout)
	if err != nil {
		return nil, fmt.Errorf("pia %s: %w", sess.Server.CN, err)
	}
	conn := &provider.Connection{
		Tunnel: tun, Server: sess.Server.CN, Summary: provider.Summarize(sess.Config), Reprovision: true,
	}
	if forward {
		fw, err := p.c.PortForward(ctx, tun.DialContext, sess.Server, sess.Token)
		if err == nil {
			err = fw.Bind(ctx)
		}
		if err != nil {
			tun.Close()
			return nil, fmt.Errorf("pia port forwarding: %w", err)
		}
		k := &portKeeper{fw: fw, lastBind: time.Now()}
		conn.ForwardedPort, conn.Maintain = fw.Port, k.maintain
	}
	log.Info().Stringer("pia", p.s).Str("server", conn.Server).Uint16("forwarded_port", conn.ForwardedPort).Msg("pia tunnel up")
	return conn, nil
}

// portKeeper renews a forwarded port.
type portKeeper struct {
	fw       *Forward
	mu       sync.Mutex
	lastBind time.Time
}

func (k *portKeeper) maintain(ctx context.Context, now time.Time) error {
	if now.After(k.fw.Expires.Add(-renewBefore)) {
		return fmt.Errorf("forwarded port %d expires at %s", k.fw.Port, k.fw.Expires.Format(time.RFC3339))
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if now.Sub(k.lastBind) < bindEvery {
		return nil
	}
	if err := k.fw.Bind(ctx); err != nil {
		return fmt.Errorf("renew forwarded port %d: %w", k.fw.Port, err)
	}
	k.lastBind = now
	return nil
}
