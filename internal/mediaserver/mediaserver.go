// Package mediaserver describes media servers (Plex, Jellyfin, ...) that
// arrd tells about library changes, so they pick up imports without
// waiting for their own scheduled scan.
//
// A server type declares the settings it needs as formspec Fields, so a
// new type needs no schema, API or frontend changes.
package mediaserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"generic-arr-project/internal/formspec"
)

// Definition describes a kind of media server and how to reach one.
type Definition struct {
	ID          string
	Name        string
	Description string
	Fields      formspec.Fields
	// Parse turns saved values into a Server without network access.
	// Problems with a field are *formspec.FieldError.
	Parse func(v formspec.Values, client *http.Client) (Server, error)
}

// Normalize checks values against the declared fields.
func (d Definition) Normalize(v formspec.Values) (formspec.Values, error) {
	return d.Fields.Normalize(d.Name, v)
}

// Server is a configured media server.
type Server interface {
	// Test checks the server is reachable and accepts the credentials.
	Test(ctx context.Context) error
	// RefreshLibrary asks the server to scan all its libraries.
	RefreshLibrary(ctx context.Context) error
}

// Registry is the set of available server types, in display order.
type Registry struct {
	defs []Definition
}

// NewRegistry checks and registers definitions.
func NewRegistry(defs ...Definition) (*Registry, error) {
	seen := map[string]bool{}
	for _, d := range defs {
		if d.ID == "" || d.Name == "" || d.Parse == nil {
			return nil, fmt.Errorf("media server %q: ID, Name and Parse are required", d.ID)
		}
		if seen[d.ID] {
			return nil, fmt.Errorf("media server %q registered twice", d.ID)
		}
		seen[d.ID] = true
	}
	return &Registry{defs: slices.Clone(defs)}, nil
}

// ErrUnknownDefinition is returned for an unregistered server type.
var ErrUnknownDefinition = errors.New("unknown media server type")

// Get returns the definition for id.
func (r *Registry) Get(id string) (Definition, error) {
	for _, d := range r.defs {
		if d.ID == id {
			return d, nil
		}
	}
	return Definition{}, fmt.Errorf("%w %q", ErrUnknownDefinition, id)
}

// List returns all definitions in display order.
func (r *Registry) List() []Definition { return slices.Clone(r.defs) }

// ParseBaseURL checks a server URL field and returns it without a
// trailing slash.
func ParseBaseURL(field, raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, formspec.Fieldf(field, "must be an http:// or https:// URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, formspec.Fieldf(field, "must not have a query or fragment")
	}
	return u, nil
}

// Do sends req and fails on a non-2xx status. The error names the method
// and path but never the query, where some servers take tokens. If out is
// non-nil the body is passed to it.
func Do(client *http.Client, req *http.Request, out func(io.Reader) error) error {
	resp, err := client.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err // url.Error repeats the full URL
		}
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%s %s: %s (check the token)", req.Method, req.URL.Path, resp.Status)
		}
		return fmt.Errorf("%s %s: %s", req.Method, req.URL.Path, resp.Status)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return nil
	}
	if err := out(io.LimitReader(resp.Body, 8<<20)); err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
