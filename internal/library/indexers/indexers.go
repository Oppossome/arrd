package indexers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store/db"
)

const (
	indexerTimeout = 30 * time.Second
	// After backoffThreshold consecutive failures an indexer is skipped
	// for backoffBase, doubling up to backoffMax.
	backoffThreshold = 3
	backoffBase      = 5 * time.Minute
	backoffMax       = 6 * time.Hour
)

// Indexer is a configured indexer. Secret settings are write-only.
type Indexer struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Definition      string          `json:"definition"`
	Settings        formspec.Values `json:"settings"`
	SecretsSet      []string        `json:"secrets_set,omitempty"`
	MovieCategories []int           `json:"movie_categories"`
	TVCategories    []int           `json:"tv_categories"`
	Enabled         bool            `json:"enabled"`
	EnableRSS       bool            `json:"enable_rss"`
	EnableSearch    bool            `json:"enable_search"`
	Priority        int64           `json:"priority"`
	MinSeeders      int64           `json:"min_seeders"`
	Caps            *indexer.Caps   `json:"caps,omitempty"`
	LastError       *string         `json:"last_error,omitempty"`
	LastErrorAt     *string         `json:"last_error_at,omitempty"`
}

// IndexerInput creates or updates an indexer. A site has at most one
// indexer. An empty Name defaults to the site's name on create. On update
// an empty Name or Definition and nil Settings keep the stored ones, and
// secret settings left empty keep the stored values when the site is
// unchanged.
type IndexerInput struct {
	Name            string          `json:"name"`
	Definition      string          `json:"definition"`
	Settings        formspec.Values `json:"settings"`
	MovieCategories []int           `json:"movie_categories"`
	TVCategories    []int           `json:"tv_categories"`
	Enabled         *bool           `json:"enabled"`
	EnableRSS       *bool           `json:"enable_rss"`
	EnableSearch    *bool           `json:"enable_search"`
	Priority        *int64          `json:"priority"`
	MinSeeders      *int64          `json:"min_seeders"`
}

func decodeIndexerSettings(r db.Indexer) (formspec.Values, error) {
	var v formspec.Values
	if err := json.Unmarshal([]byte(r.Settings), &v); err != nil {
		return nil, fmt.Errorf("indexer %q settings: %w", r.Name, err)
	}
	return v, nil
}

// indexerFromRow converts a row for display. Settings of a definition that is
// no longer loaded are left out, since which are secret is unknown.
func (s *Service) indexerFromRow(r db.Indexer) (Indexer, error) {
	ix := Indexer{
		ID: r.ID, Name: r.Name, Definition: r.Definition, Settings: formspec.Values{},
		Enabled: r.Enabled, EnableRSS: r.EnableRSS, EnableSearch: r.EnableSearch,
		Priority: r.Priority, MinSeeders: r.MinSeeders,
		LastError: r.LastError, LastErrorAt: r.LastErrorAt,
	}
	if def, err := s.definitions.Load().Get(r.Definition); err == nil {
		values, err := decodeIndexerSettings(r)
		if err != nil {
			return Indexer{}, err
		}
		ix.Settings, ix.SecretsSet = def.Fields.Redact(values)
	}
	var err error
	if ix.MovieCategories, err = parseCategories(r.MovieCategories); err != nil {
		return Indexer{}, fmt.Errorf("indexer %d movie categories: %w", r.ID, err)
	}
	if ix.TVCategories, err = parseCategories(r.TVCategories); err != nil {
		return Indexer{}, fmt.Errorf("indexer %d tv categories: %w", r.ID, err)
	}
	if r.CapsJSON != nil {
		ix.Caps = &indexer.Caps{}
		if err := json.Unmarshal([]byte(*r.CapsJSON), ix.Caps); err != nil {
			return Indexer{}, fmt.Errorf("indexer %d caps: %w", r.ID, err)
		}
	}
	return ix, nil
}

func parseCategories(s string) ([]int, error) {
	out := []int{}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func formatCategories(cats []int) string {
	parts := make([]string, len(cats))
	for i, c := range cats {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, ",")
}

// Indexers lists all indexers.
func (s *Service) Indexers(ctx context.Context) ([]Indexer, error) {
	rows, err := s.store.ListIndexers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list indexers: %w", err)
	}
	out := make([]Indexer, 0, len(rows))
	for _, r := range rows {
		ix, err := s.indexerFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, ix)
	}
	return out, nil
}

// Indexer returns one indexer.
func (s *Service) Indexer(ctx context.Context, id int64) (Indexer, error) {
	r, err := s.store.GetIndexer(ctx, id)
	if err != nil {
		return Indexer{}, apperr.NotFound(err, "indexer", id)
	}
	return s.indexerFromRow(r)
}

// indexerParams holds validated, defaulted input.
type indexerParams struct {
	name, definition, settings, movieCats, tvCats string
	enabled, rss, search                          bool
	priority, minSeeders                          int64
}

// validateIndexer checks and defaults indexer input without saving it.
func validateIndexer(in IndexerInput, existing *db.Indexer, catalog *indexer.Catalog) (indexerParams, error) {
	p := indexerParams{
		name: strings.TrimSpace(in.Name), definition: strings.TrimSpace(in.Definition),
		enabled: true, rss: true, search: true, priority: 25, minSeeders: 1,
		movieCats: "2000", tvCats: "5000",
	}
	if existing != nil {
		p.enabled, p.rss, p.search = existing.Enabled, existing.EnableRSS, existing.EnableSearch
		p.priority, p.minSeeders = existing.Priority, existing.MinSeeders
		p.movieCats, p.tvCats = existing.MovieCategories, existing.TVCategories
		if p.definition == "" {
			p.definition = existing.Definition
		}
		if p.name == "" {
			p.name = existing.Name
		}
	}
	def, settings, err := validateIndexerSettings(p.definition, in.Settings, existing, catalog)
	if err != nil {
		return p, err
	}
	p.settings = settings
	if p.name == "" {
		p.name = def.Name
	}
	if in.MovieCategories != nil {
		p.movieCats = formatCategories(in.MovieCategories)
	}
	if in.TVCategories != nil {
		p.tvCats = formatCategories(in.TVCategories)
	}
	if in.Enabled != nil {
		p.enabled = *in.Enabled
	}
	if in.EnableRSS != nil {
		p.rss = *in.EnableRSS
	}
	if !def.RSS {
		// RSS sync lists recent releases, which this site cannot do.
		if in.EnableRSS != nil && *in.EnableRSS {
			return p, apperr.Invalid("enable_rss", "%s cannot list recent releases", def.Name)
		}
		p.rss = false
	}
	if in.EnableSearch != nil {
		p.search = *in.EnableSearch
	}
	if in.Priority != nil {
		p.priority = *in.Priority
	}
	if in.MinSeeders != nil {
		if *in.MinSeeders < 0 {
			return p, apperr.Invalid("min_seeders", "must not be negative")
		}
		p.minSeeders = *in.MinSeeders
	}
	return p, nil
}

// validateIndexerSettings checks settings against the definition's fields
// and returns it and the settings as stored JSON. Field errors are named
// "settings.<key>".
func validateIndexerSettings(defID string, values formspec.Values, existing *db.Indexer, catalog *indexer.Catalog) (indexer.Definition, string, error) {
	if defID == "" {
		return indexer.Definition{}, "", apperr.Invalid("definition", "is required")
	}
	def, err := catalog.Get(defID)
	if err != nil {
		return indexer.Definition{}, "", apperr.Invalid("definition", "%v", err)
	}
	if def.Unsupported != "" {
		return indexer.Definition{}, "", apperr.Invalid("definition", "%s cannot be used: %s", def.Name, def.Unsupported)
	}
	if existing != nil && existing.Definition == def.ID {
		saved, err := decodeIndexerSettings(*existing)
		if err != nil {
			return indexer.Definition{}, "", err
		}
		if values == nil {
			values = saved // e.g. only switched on or off
		} else {
			values = def.Fields.MergeSecrets(values, saved)
		}
	}
	values, err = def.Normalize(values)
	if err != nil {
		return indexer.Definition{}, "", apperr.PrefixField("settings.", apperr.FromFieldError(err))
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return indexer.Definition{}, "", fmt.Errorf("encode indexer settings: %w", err)
	}
	return def, string(raw), nil
}

// CreateIndexer validates and adds an indexer.
func (s *Service) CreateIndexer(ctx context.Context, in IndexerInput) (Indexer, error) {
	p, err := validateIndexer(in, nil, s.definitions.Load())
	if err != nil {
		return Indexer{}, err
	}
	row, err := s.store.CreateIndexer(ctx, p.createParams())
	if err != nil {
		if err := indexerConflict(err, p); err != nil {
			return Indexer{}, err
		}
		return Indexer{}, fmt.Errorf("create indexer: %w", err)
	}
	return s.indexerFromRow(row)
}

// UpdateIndexer validates and saves changes; omitted optional fields keep
// their stored values.
func (s *Service) UpdateIndexer(ctx context.Context, id int64, in IndexerInput) (Indexer, error) {
	existing, err := s.store.GetIndexer(ctx, id)
	if err != nil {
		return Indexer{}, apperr.NotFound(err, "indexer", id)
	}
	p, err := validateIndexer(in, &existing, s.definitions.Load())
	if err != nil {
		return Indexer{}, err
	}
	row, err := s.store.UpdateIndexer(ctx, db.UpdateIndexerParams{
		ID: id, Name: p.name, Definition: p.definition, Settings: p.settings,
		MovieCategories: p.movieCats, TVCategories: p.tvCats, Enabled: p.enabled,
		EnableRSS: p.rss, EnableSearch: p.search, Priority: p.priority, MinSeeders: p.minSeeders,
	})
	if err != nil {
		if err := indexerConflict(err, p); err != nil {
			return Indexer{}, err
		}
		return Indexer{}, fmt.Errorf("update indexer: %w", err)
	}
	s.clearBackoff(id)
	return s.indexerFromRow(row)
}

// DeleteIndexer removes an indexer.
func (s *Service) DeleteIndexer(ctx context.Context, id int64) error {
	n, err := s.store.DeleteIndexer(ctx, id)
	if err != nil {
		return fmt.Errorf("delete indexer: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("indexer %d: %w", id, apperr.ErrNotFound)
	}
	s.clearBackoff(id)
	s.forgetSearcher(id)
	return nil
}

// TestIndexer signs in to the site and runs a query, stores the
// definition's capabilities, and clears any recorded error on success. A
// failure is recorded and returned.
func (s *Service) TestIndexer(ctx context.Context, id int64) (Indexer, error) {
	row, err := s.store.GetIndexer(ctx, id)
	if err != nil {
		return Indexer{}, apperr.NotFound(err, "indexer", id)
	}
	searcher, err := s.Searcher(ctx, row)
	if err != nil {
		return Indexer{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, indexerTimeout)
	defer cancel()
	// A test signs in and runs a query; the capabilities come from the
	// definition.
	if err := searcher.Test(ctx); err != nil {
		s.recordIndexerFailure(context.WithoutCancel(ctx), id, err)
		return Indexer{}, fmt.Errorf("indexer %q: %w", row.Name, err)
	}
	raw, err := json.Marshal(searcher.Caps())
	if err != nil {
		return Indexer{}, err
	}
	capsJSON := string(raw)
	if err := s.store.SetIndexerCaps(ctx, db.SetIndexerCapsParams{ID: id, CapsJSON: &capsJSON}); err != nil {
		return Indexer{}, fmt.Errorf("save indexer caps: %w", err)
	}
	s.clearBackoff(id)
	return s.Indexer(ctx, id)
}

// cachedSearcher is a built searcher and what it was built from. Searchers
// are kept because they hold a signed-in session.
type cachedSearcher struct {
	catalog  *indexer.Catalog
	def      string
	settings string
	network  netx.Network
	searcher *indexer.Searcher
}

// matches reports whether c was built from the same inputs.
func (c cachedSearcher) matches(catalog *indexer.Catalog, row db.Indexer, n netx.Network) bool {
	return c.catalog == catalog && c.def == row.Definition && c.settings == row.Settings && sameNetwork(c.network, n)
}

// sameNetwork compares networks without panicking on an uncomparable one.
func sameNetwork(a, b netx.Network) bool {
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	return ta == tb && ta != nil && ta.Comparable() && a == b
}

// Searcher returns the searcher for a saved indexer, on the traffic
// network. It is reused until the indexer's definition, settings or the
// network (e.g. a new VPN tunnel) changes, or definitions are reloaded.
func (s *Service) Searcher(ctx context.Context, row db.Indexer) (*indexer.Searcher, error) {
	catalog := s.definitions.Load()
	def, err := catalog.Get(row.Definition)
	if err != nil {
		return nil, fmt.Errorf("indexer %q: %w", row.Name, err)
	}
	if def.Unsupported != "" {
		return nil, fmt.Errorf("indexer %q: %s cannot be used: %s", row.Name, def.Name, def.Unsupported)
	}
	n, err := s.engine.TrafficNetwork(ctx)
	if err != nil {
		return nil, fmt.Errorf("indexer %q: %w", row.Name, err)
	}
	s.searchersMu.Lock()
	defer s.searchersMu.Unlock()
	if c, ok := s.searchers[row.ID]; ok && c.matches(catalog, row, n) {
		return c.searcher, nil
	}
	values, err := decodeIndexerSettings(row)
	if err != nil {
		return nil, err
	}
	searcher, err := def.NewSearcher(values, netx.HTTPClient(n, indexerTimeout))
	if err != nil {
		var fe *formspec.FieldError
		if errors.As(err, &fe) {
			return nil, fmt.Errorf("indexer %q setting %s", row.Name, fe.Error())
		}
		return nil, fmt.Errorf("indexer %q: %w", row.Name, err)
	}
	s.searchers[row.ID] = cachedSearcher{catalog: catalog, def: row.Definition, settings: row.Settings, network: n, searcher: searcher}
	return searcher, nil
}

func (s *Service) forgetSearcher(id int64) {
	s.searchersMu.Lock()
	delete(s.searchers, id)
	s.searchersMu.Unlock()
}

// IndexerFailure is a per-indexer error from a fan-out search.
type IndexerFailure struct {
	IndexerID int64  `json:"indexer_id"`
	Indexer   string `json:"indexer"`
	Error     string `json:"error"`
}

// FoundRelease is a release together with the indexer that returned it.
type FoundRelease struct {
	indexer.Release
	IndexerID  int64
	Indexer    string
	MinSeeders int64
}

// Search runs buildQuery against every enabled search indexer in
// parallel. Failures are recorded on the indexer and returned alongside the
// results from the indexers that worked; they are never dropped silently.
func (s *Service) Search(ctx context.Context, buildQuery func(Indexer) (indexer.Query, bool)) ([]FoundRelease, []IndexerFailure, error) {
	rows, err := s.store.ListSearchIndexers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list indexers: %w", err)
	}
	found, failures := s.Query(ctx, rows, buildQuery)
	return found, failures, nil
}

// Query runs buildQuery against rows in parallel; see Search.
func (s *Service) Query(ctx context.Context, rows []db.Indexer, buildQuery func(Indexer) (indexer.Query, bool)) ([]FoundRelease, []IndexerFailure) {
	var (
		mu       sync.Mutex
		results  []FoundRelease
		failures []IndexerFailure
		wg       sync.WaitGroup
	)
	fail := func(row db.Indexer, err error) {
		mu.Lock()
		failures = append(failures, IndexerFailure{IndexerID: row.ID, Indexer: row.Name, Error: err.Error()})
		mu.Unlock()
	}
	for _, row := range rows {
		if until, skipping := s.inBackoff(row.ID); skipping {
			fail(row, fmt.Errorf("skipped after repeated failures until %s", until.Format(time.RFC3339)))
			continue
		}
		ix, err := s.indexerFromRow(row)
		if err != nil {
			fail(row, err)
			continue
		}
		q, ok := buildQuery(ix)
		if !ok {
			continue // indexer cannot serve this kind of search
		}
		searcher, err := s.Searcher(ctx, row)
		if err != nil {
			fail(row, err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			qctx, cancel := context.WithTimeout(ctx, indexerTimeout)
			defer cancel()
			rels, err := searcher.Search(qctx, q)
			if err != nil {
				s.recordIndexerFailure(context.WithoutCancel(ctx), row.ID, err)
				fail(row, err)
				return
			}
			s.recordIndexerSuccess(context.WithoutCancel(ctx), row)
			mu.Lock()
			defer mu.Unlock()
			for _, r := range rels {
				results = append(results, FoundRelease{Release: r, IndexerID: row.ID, Indexer: row.Name, MinSeeders: row.MinSeeders})
			}
		}()
	}
	wg.Wait()
	return results, failures
}

type indexerBackoff struct {
	failures int
	until    time.Time
}

func (s *Service) inBackoff(id int64) (time.Time, bool) {
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()
	b := s.backoff[id]
	return b.until, time.Now().Before(b.until)
}

func (s *Service) clearBackoff(id int64) {
	s.backoffMu.Lock()
	delete(s.backoff, id)
	s.backoffMu.Unlock()
}

func (s *Service) recordIndexerFailure(ctx context.Context, id int64, cause error) {
	msg := cause.Error()
	if err := s.store.RecordIndexerError(ctx, db.RecordIndexerErrorParams{ID: id, LastError: &msg}); err != nil {
		s.log.Error().Int64("indexer", id).Err(err).Msg("record indexer error")
	}
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()
	b := s.backoff[id]
	b.failures++
	if b.failures >= backoffThreshold {
		d := backoffBase << (b.failures - backoffThreshold)
		if d > backoffMax || d <= 0 {
			d = backoffMax
		}
		b.until = time.Now().Add(d)
		s.log.Warn().Int64("indexer", id).Int("failures", b.failures).Time("until", b.until).Err(cause).Msg("indexer backing off")
	}
	s.backoff[id] = b
}

func (s *Service) recordIndexerSuccess(ctx context.Context, row db.Indexer) {
	s.clearBackoff(row.ID)
	if row.LastError == nil {
		return
	}
	if err := s.store.ClearIndexerError(ctx, row.ID); err != nil {
		s.log.Error().Int64("indexer", row.ID).Err(err).Msg("clear indexer error")
	}
}

// indexerConflict explains a unique violation saving p, or returns nil.
func indexerConflict(err error, p indexerParams) error {
	switch {
	case !apperr.IsUniqueViolation(err):
		return nil
	case strings.Contains(err.Error(), "indexers.definition"):
		return apperr.Conflict("site %q already has an indexer", p.definition)
	default:
		return apperr.Conflict("an indexer named %q already exists", p.name)
	}
}

func (p indexerParams) createParams() db.CreateIndexerParams {
	return db.CreateIndexerParams{
		Name: p.name, Definition: p.definition, Settings: p.settings,
		MovieCategories: p.movieCats, TVCategories: p.tvCats, Enabled: p.enabled,
		EnableRSS: p.rss, EnableSearch: p.search, Priority: p.priority, MinSeeders: p.minSeeders,
	}
}
