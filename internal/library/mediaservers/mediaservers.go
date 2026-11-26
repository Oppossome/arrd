// Package mediaservers manages the configured media servers (Plex,
// Jellyfin, ...) and asks them to rescan their libraries after imports.
package mediaservers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/mediaserver"
	"generic-arr-project/internal/mediaserver/jellyfin"
	"generic-arr-project/internal/mediaserver/plex"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/store/db"

	"github.com/rs/zerolog"
)

// serverTimeout bounds one test or refresh of one server.
const serverTimeout = 30 * time.Second

// DefaultRegistry is the set of server types arrd ships with.
func DefaultRegistry() (*mediaserver.Registry, error) {
	return mediaserver.NewRegistry(plex.Definition(), jellyfin.Definition())
}

// Service manages media servers and notifies them of library changes.
type Service struct {
	store    *store.Store
	registry *mediaserver.Registry
	direct   netx.Network
	log      *zerolog.Logger

	// pending is set when the library changed since the last flush.
	pending atomic.Bool
}

// Deps are the Service's collaborators.
type Deps struct {
	Store    *store.Store
	Registry *mediaserver.Registry
	// Direct reaches the servers; they are on the LAN, not behind the VPN.
	Direct netx.Network
	Log    *zerolog.Logger
}

// New creates a Service.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Registry == nil || d.Direct == nil || d.Log == nil {
		return nil, errors.New("mediaservers: Store, Registry, Direct and Log are required")
	}
	return &Service{store: d.Store, registry: d.Registry, direct: d.Direct, log: d.Log}, nil
}

// MediaServer is a configured media server. Secret settings are write-only.
type MediaServer struct {
	ID          int64
	Name        string
	Definition  string
	Settings    formspec.Values
	SecretsSet  []string
	Enabled     bool
	LastError   *string
	LastErrorAt *string
}

// Input creates or updates a media server. An empty Name defaults to the
// type's name on create. On update an empty Name or Definition and nil
// Settings keep the stored ones, and secret settings left empty keep the
// stored values when the type is unchanged.
type Input struct {
	Name       string
	Definition string
	Settings   formspec.Values
	Enabled    *bool
}

// Definitions lists the server types that can be added.
func (s *Service) Definitions() []mediaserver.Definition { return s.registry.List() }

func decodeSettings(r db.MediaServer) (formspec.Values, error) {
	var v formspec.Values
	if err := json.Unmarshal([]byte(r.Settings), &v); err != nil {
		return nil, fmt.Errorf("media server %q settings: %w", r.Name, err)
	}
	return v, nil
}

// fromRow converts a row for display. Settings of an unknown type are left
// out, since which are secret is unknown.
func (s *Service) fromRow(r db.MediaServer) (MediaServer, error) {
	m := MediaServer{
		ID: r.ID, Name: r.Name, Definition: r.Definition, Settings: formspec.Values{},
		Enabled: r.Enabled, LastError: r.LastError, LastErrorAt: r.LastErrorAt,
	}
	def, err := s.registry.Get(r.Definition)
	if err != nil {
		return m, nil
	}
	values, err := decodeSettings(r)
	if err != nil {
		return MediaServer{}, err
	}
	m.Settings, m.SecretsSet = def.Fields.Redact(values)
	return m, nil
}

// List returns all media servers.
func (s *Service) List(ctx context.Context) ([]MediaServer, error) {
	rows, err := s.store.ListMediaServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list media servers: %w", err)
	}
	out := make([]MediaServer, 0, len(rows))
	for _, r := range rows {
		m, err := s.fromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// Get returns one media server.
func (s *Service) Get(ctx context.Context, id int64) (MediaServer, error) {
	r, err := s.store.GetMediaServer(ctx, id)
	if err != nil {
		return MediaServer{}, apperr.NotFound(err, "media server", id)
	}
	return s.fromRow(r)
}

// params is validated, defaulted input ready to save.
type params struct {
	name, definition, settings string
	enabled                    bool
}

// validate checks and defaults input without saving it.
func (s *Service) validate(in Input, existing *db.MediaServer) (params, error) {
	p := params{name: strings.TrimSpace(in.Name), definition: strings.TrimSpace(in.Definition), enabled: true}
	if existing != nil {
		p.enabled = existing.Enabled
		if p.name == "" {
			p.name = existing.Name
		}
		if p.definition == "" {
			p.definition = existing.Definition
		}
	}
	if in.Enabled != nil {
		p.enabled = *in.Enabled
	}
	if p.definition == "" {
		return p, apperr.Invalid("definition", "is required")
	}
	def, err := s.registry.Get(p.definition)
	if err != nil {
		return p, apperr.Invalid("definition", "%v", err)
	}
	if p.name == "" {
		p.name = def.Name
	}
	values := in.Settings
	if existing != nil && existing.Definition == def.ID {
		saved, err := decodeSettings(*existing)
		if err != nil {
			return p, err
		}
		if values == nil {
			values = saved // e.g. only switched on or off
		} else {
			values = def.Fields.MergeSecrets(values, saved)
		}
	}
	values, err = def.Normalize(values)
	if err != nil {
		return p, apperr.PrefixField("settings.", apperr.FromFieldError(err))
	}
	if _, err := def.Parse(values, nil); err != nil {
		return p, apperr.PrefixField("settings.", apperr.FromFieldError(err))
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return p, fmt.Errorf("encode media server settings: %w", err)
	}
	p.settings = string(raw)
	return p, nil
}

// Create validates and adds a media server.
func (s *Service) Create(ctx context.Context, in Input) (MediaServer, error) {
	p, err := s.validate(in, nil)
	if err != nil {
		return MediaServer{}, err
	}
	row, err := s.store.CreateMediaServer(ctx, db.CreateMediaServerParams{
		Name: p.name, Definition: p.definition, Settings: p.settings, Enabled: p.enabled,
	})
	if err != nil {
		return MediaServer{}, conflict(err, p, "create")
	}
	return s.fromRow(row)
}

// Update validates and saves changes; omitted fields keep their stored
// values.
func (s *Service) Update(ctx context.Context, id int64, in Input) (MediaServer, error) {
	existing, err := s.store.GetMediaServer(ctx, id)
	if err != nil {
		return MediaServer{}, apperr.NotFound(err, "media server", id)
	}
	p, err := s.validate(in, &existing)
	if err != nil {
		return MediaServer{}, err
	}
	row, err := s.store.UpdateMediaServer(ctx, db.UpdateMediaServerParams{
		ID: id, Name: p.name, Definition: p.definition, Settings: p.settings, Enabled: p.enabled,
	})
	if err != nil {
		return MediaServer{}, conflict(err, p, "update")
	}
	return s.fromRow(row)
}

func conflict(err error, p params, op string) error {
	if apperr.IsUniqueViolation(err) {
		return apperr.Conflict("a media server named %q already exists", p.name)
	}
	return fmt.Errorf("%s media server: %w", op, err)
}

// Delete removes a media server.
func (s *Service) Delete(ctx context.Context, id int64) error {
	n, err := s.store.DeleteMediaServer(ctx, id)
	if err != nil {
		return fmt.Errorf("delete media server: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("media server %d: %w", id, apperr.ErrNotFound)
	}
	return nil
}

// Test checks a saved server is reachable with its credentials. The
// outcome is recorded as the server's last error, and a failure returned.
func (s *Service) Test(ctx context.Context, id int64) (MediaServer, error) {
	row, err := s.store.GetMediaServer(ctx, id)
	if err != nil {
		return MediaServer{}, apperr.NotFound(err, "media server", id)
	}
	if err := s.run(ctx, row, mediaserver.Server.Test); err != nil {
		return MediaServer{}, err
	}
	return s.Get(ctx, id)
}

// LibraryChanged notes that files were imported. It does not block; the
// servers are told on the next FlushPending.
func (s *Service) LibraryChanged() { s.pending.Store(true) }

// FlushPending asks every enabled server to rescan if the library changed
// since the last call. Batching this way turns a burst of imports (e.g. a
// season pack) into one scan. Each server's failure is recorded on it and
// does not stop the others.
func (s *Service) FlushPending(ctx context.Context) error {
	if !s.pending.Swap(false) {
		return nil
	}
	rows, err := s.store.ListEnabledMediaServers(ctx)
	if err != nil {
		s.pending.Store(true) // try again next time
		return fmt.Errorf("list media servers: %w", err)
	}
	var errs []error
	for _, row := range rows {
		if err := s.run(ctx, row, mediaserver.Server.RefreshLibrary); err != nil {
			errs = append(errs, err)
			continue
		}
		s.log.Info().Str("media_server", row.Name).Msg("asked media server to rescan")
	}
	return errors.Join(errs...)
}

// run builds row's server and calls op on it with a timeout, recording
// the outcome as the server's last error.
func (s *Service) run(ctx context.Context, row db.MediaServer, op func(mediaserver.Server, context.Context) error) error {
	err := s.call(ctx, row, op)
	record := context.WithoutCancel(ctx)
	if err != nil {
		msg := err.Error()
		if rerr := s.store.RecordMediaServerError(record, db.RecordMediaServerErrorParams{ID: row.ID, LastError: &msg}); rerr != nil {
			s.log.Error().Int64("media_server", row.ID).Err(rerr).Msg("record media server error")
		}
		return fmt.Errorf("media server %q: %w", row.Name, err)
	}
	if row.LastError != nil {
		if cerr := s.store.ClearMediaServerError(record, row.ID); cerr != nil {
			s.log.Error().Int64("media_server", row.ID).Err(cerr).Msg("clear media server error")
		}
	}
	return nil
}

func (s *Service) call(ctx context.Context, row db.MediaServer, op func(mediaserver.Server, context.Context) error) error {
	def, err := s.registry.Get(row.Definition)
	if err != nil {
		return err
	}
	values, err := decodeSettings(row)
	if err != nil {
		return err
	}
	srv, err := def.Parse(values, netx.HTTPClient(s.direct, serverTimeout))
	if err != nil {
		return fmt.Errorf("setting %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, serverTimeout)
	defer cancel()
	return op(srv, ctx)
}
