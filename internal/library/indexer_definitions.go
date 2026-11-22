package library

import (
	"errors"
	"fmt"

	"generic-arr-project/internal/indexer"
)

// IndexerDefinitions lists the sites indexers can be added for and the
// settings each needs.
func (s *Service) IndexerDefinitions() []indexer.Definition { return s.definitions.Load().List() }

// ReloadResult reports a reload of the indexer definitions.
type ReloadResult struct {
	Definitions []indexer.Definition `json:"definitions"`
	Errors      []string             `json:"errors"` // definition files that could not be read
}

// ReloadIndexerDefinitions re-reads the definitions directory. Indexers of
// a definition that changed get a fresh searcher on next use; indexers of
// a definition that is gone fail until it is restored.
func (s *Service) ReloadIndexerDefinitions() ReloadResult {
	s.definitionsMu.Lock()
	defer s.definitionsMu.Unlock()
	catalog, errs := s.loadDefinitions()
	return ReloadResult{Definitions: catalog.List(), Errors: errorStrings(errs)}
}

// UploadResult reports definitions added from an upload.
type UploadResult struct {
	Saved       []string             `json:"saved"`       // ids written
	Errors      []string             `json:"errors"`      // files in the upload that were skipped
	Definitions []indexer.Definition `json:"definitions"` // all definitions now loaded
}

// UploadIndexerDefinitions saves the definitions in data (one definition
// file, or a zip of them) to the definitions directory and reloads it.
func (s *Service) UploadIndexerDefinitions(data []byte) (UploadResult, error) {
	s.definitionsMu.Lock()
	defer s.definitionsMu.Unlock()
	res, err := indexer.Import(s.definitionsDir, data)
	if errors.Is(err, indexer.ErrInvalidUpload) {
		return UploadResult{}, invalid("file", "%v", err)
	}
	if err != nil {
		return UploadResult{}, err
	}
	catalog, _ := s.loadDefinitions() // problems with other files are logged and shown on reload
	s.log.Info("indexer definitions uploaded", "saved", len(res.Saved), "skipped", len(res.Errors))
	return UploadResult{Saved: res.Saved, Errors: errorStrings(res.Errors), Definitions: catalog.List()}, nil
}

// DeleteIndexerDefinition removes the file of a definition and reloads.
// Indexers of the site fail until a definition for it is added again.
func (s *Service) DeleteIndexerDefinition(id string) error {
	s.definitionsMu.Lock()
	defer s.definitionsMu.Unlock()
	err := indexer.Remove(s.definitionsDir, id)
	if errors.Is(err, indexer.ErrUnknownDefinition) {
		return fmt.Errorf("site definition %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return err
	}
	s.loadDefinitions()
	return nil
}

// loadDefinitions reads the definitions directory and installs the
// catalog, logging definition files that could not be read.
func (s *Service) loadDefinitions() (*indexer.Catalog, []error) {
	catalog, errs := indexer.LoadDir(s.definitionsDir)
	for _, err := range errs {
		s.log.Warn("indexer definition skipped", "err", err)
	}
	s.definitions.Store(catalog)
	s.log.Info("indexer definitions loaded", "dir", s.definitionsDir, "definitions", len(catalog.List()), "skipped", len(errs))
	return catalog, errs
}

func errorStrings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, err.Error())
	}
	return out
}
