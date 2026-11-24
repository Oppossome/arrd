// Package provider describes VPN providers generically, so arrd can bring
// up a tunnel from a static WireGuard config, from a provider that issues
// short-lived registrations (PIA), or from future integrations, through
// one code path.
//
// A provider declares the settings it needs as formspec Fields, so a new
// provider needs no schema, API or frontend changes.
package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/vpn"

	"github.com/rs/zerolog"
)

// The settings form types are shared with other integrations; see formspec.
type (
	FieldKind  = formspec.FieldKind
	Field      = formspec.Field
	Option     = formspec.Option
	Values     = formspec.Values
	FieldError = formspec.FieldError
)

const (
	KindText     = formspec.KindText
	KindPassword = formspec.KindPassword
	KindTextarea = formspec.KindTextarea
	KindSelect   = formspec.KindSelect
	KindToggle   = formspec.KindToggle
)

// Fieldf builds a FieldError.
func Fieldf(field, format string, args ...any) error { return formspec.Fieldf(field, format, args...) }

// Definition describes a provider and how to use it.
type Definition struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Fields      formspec.Fields `json:"fields"`
	// Renews means each Connect obtains fresh credentials from the
	// provider, so arrd re-connects on its own when a tunnel goes stale.
	Renews bool `json:"renews"`

	// Options lists the choices for a select field with Options set. It
	// may call the provider's API. Nil if no field has options.
	Options func(ctx context.Context, field string) ([]Option, error) `json:"-"`
	// Parse turns saved values into a Provider without network access.
	// It runs on every runtime start.
	Parse func(v Values) (Provider, error) `json:"-"`
	// Check verifies values with the provider (e.g. signs in). It runs on
	// save and test only. Problems with a field are *FieldError.
	Check func(ctx context.Context, v Values) error `json:"-"`
}

// Field returns the field with key.
func (d Definition) Field(key string) (Field, bool) { return d.Fields.Get(key) }

// Normalize checks values against the declared fields; see
// formspec.Fields.Normalize.
func (d Definition) Normalize(v Values) (Values, error) { return d.Fields.Normalize(d.Name, v) }

// MergeSecrets fills secret fields left empty in v from saved. Call it only
// when v is for the same provider as saved.
func (d Definition) MergeSecrets(v, saved Values) Values { return d.Fields.MergeSecrets(v, saved) }

// Redact splits values into what may be shown (secrets removed) and the
// keys of secret fields that have a saved value.
func (d Definition) Redact(v Values) (public Values, secretsSet []string) { return d.Fields.Redact(v) }

// Provider connects tunnels with parsed settings.
type Provider interface {
	// Connect brings up a tunnel and waits for its first handshake.
	Connect(ctx context.Context, opts ConnectOptions, log *zerolog.Logger) (*Connection, error)
}

// ConnectOptions tune a connection.
type ConnectOptions struct {
	// PortForward asks for a forwarded port if the settings enable one.
	// It is only set when the torrent engine will listen on the tunnel.
	PortForward bool
}

// Connection is a live tunnel and what arrd needs to keep it working.
type Connection struct {
	Tunnel        *vpn.Tunnel
	Server        string // the server connected to, for display
	ForwardedPort uint16 // 0 without port forwarding
	Summary       Summary
	// Reprovision means connecting again obtains fresh credentials, so a
	// tunnel whose handshakes stopped is worth replacing.
	Reprovision bool
	// Maintain does periodic upkeep (e.g. renewing a forwarded port). An
	// error means the connection should be replaced. May be nil.
	Maintain func(ctx context.Context, now time.Time) error
}

// Close shuts the tunnel down.
func (c *Connection) Close() { c.Tunnel.Close() }

// Summary is the non-secret shape of a tunnel config, for display.
type Summary struct {
	Addresses   []string
	Endpoints   []string
	DNS         []string
	IgnoredKeys []string
}

// Summarize describes cfg without its keys.
func Summarize(cfg *vpn.Config) Summary {
	var s Summary
	for _, a := range cfg.Addresses {
		s.Addresses = append(s.Addresses, a.String())
	}
	for _, d := range cfg.DNS {
		s.DNS = append(s.DNS, d.String())
	}
	for _, p := range cfg.Peers {
		s.Endpoints = append(s.Endpoints, p.Endpoint)
	}
	s.IgnoredKeys = cfg.IgnoredKeys
	return s
}

// HandshakeTimeout bounds how long Connect waits for a first handshake.
const HandshakeTimeout = 30 * time.Second

// Registry is the set of available providers, in display order.
type Registry struct {
	defs []Definition
}

// NewRegistry checks and registers definitions.
func NewRegistry(defs ...Definition) (*Registry, error) {
	seen := map[string]bool{}
	for _, d := range defs {
		if d.ID == "" || d.Name == "" || d.Parse == nil || d.Check == nil {
			return nil, fmt.Errorf("vpn provider %q: ID, Name, Parse and Check are required", d.ID)
		}
		if seen[d.ID] {
			return nil, fmt.Errorf("vpn provider %q registered twice", d.ID)
		}
		seen[d.ID] = true
		for _, f := range d.Fields {
			if f.Options && d.Options == nil {
				return nil, fmt.Errorf("vpn provider %q: field %q has options but no Options func", d.ID, f.Key)
			}
		}
	}
	return &Registry{defs: slices.Clone(defs)}, nil
}

// ErrUnknownProvider is returned for an unregistered provider id.
var ErrUnknownProvider = errors.New("unknown vpn provider")

// Get returns the definition for id.
func (r *Registry) Get(id string) (Definition, error) {
	for _, d := range r.defs {
		if d.ID == id {
			return d, nil
		}
	}
	return Definition{}, fmt.Errorf("%w %q", ErrUnknownProvider, id)
}

// List returns all definitions in display order.
func (r *Registry) List() []Definition { return slices.Clone(r.defs) }
