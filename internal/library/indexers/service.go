// Package indexers manages the configured indexers and their site
// definitions, and searches them.
package indexers

import (
	"errors"
	"sync"
	"sync/atomic"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/store/db"

	"github.com/rs/zerolog"
)

// Service manages and searches indexers.
type Service struct {
	store  *store.Store
	engine *engine.Manager
	log    *zerolog.Logger

	// definitions is replaced when indexer definitions are reloaded.
	definitions atomic.Pointer[indexer.Catalog]
	// definitionsMu serializes changes to the definitions directory.
	definitionsMu  sync.Mutex
	definitionsDir string

	backoffMu sync.Mutex
	backoff   map[int64]indexerBackoff // indexer id -> failure state

	searchersMu sync.Mutex
	searchers   map[int64]cachedSearcher // indexer id -> built searcher
}

// Deps are the Service's collaborators.
type Deps struct {
	Store  *store.Store
	Engine *engine.Manager // supplies the traffic network
	// DefinitionsDir holds the Cardigann site definitions (*.yml) indexers
	// are built from; empty or missing means none.
	DefinitionsDir string
	Log            *zerolog.Logger
}

// New creates a Service and loads the site definitions.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Engine == nil || d.Log == nil {
		return nil, errors.New("indexers: Store, Engine and Log are required")
	}
	s := &Service{
		store: d.Store, engine: d.Engine, log: d.Log, definitionsDir: d.DefinitionsDir,
		backoff: map[int64]indexerBackoff{}, searchers: map[int64]cachedSearcher{},
	}
	s.loadDefinitions()
	return s, nil
}

// Catalog returns the loaded site definitions.
func (s *Service) Catalog() *indexer.Catalog { return s.definitions.Load() }

// Params is validated indexer input, ready to save.
type Params struct{ p indexerParams }

// Name is the indexer's name.
func (p Params) Name() string { return p.p.name }

// Definition is the indexer's site definition id.
func (p Params) Definition() string { return p.p.definition }

// CreateParams returns the row to insert.
func (p Params) CreateParams() db.CreateIndexerParams { return p.p.createParams() }

// ValidateNew checks and defaults input for a new indexer without saving it.
func (s *Service) ValidateNew(in IndexerInput) (Params, error) {
	p, err := validateIndexer(in, nil, s.Catalog())
	return Params{p}, err
}
