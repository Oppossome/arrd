package httpapi

import (
	"context"
	"fmt"
	"io"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/indexers"
)

func (h *handlers) ListIndexers(ctx context.Context, _ apigen.ListIndexersRequestObject) (apigen.ListIndexersResponseObject, error) {
	out, err := h.svc.Indexers.Indexers(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.ListIndexers200JSONResponse(mapSlice(out, toAPIIndexer)), nil
}

func (h *handlers) CreateIndexer(ctx context.Context, req apigen.CreateIndexerRequestObject) (apigen.CreateIndexerResponseObject, error) {
	out, err := h.svc.Indexers.CreateIndexer(ctx, fromAPIIndexerInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.CreateIndexer201JSONResponse(toAPIIndexer(out)), nil
}

func (h *handlers) GetIndexer(ctx context.Context, req apigen.GetIndexerRequestObject) (apigen.GetIndexerResponseObject, error) {
	out, err := h.svc.Indexers.Indexer(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.GetIndexer200JSONResponse(toAPIIndexer(out)), nil
}

func (h *handlers) UpdateIndexer(ctx context.Context, req apigen.UpdateIndexerRequestObject) (apigen.UpdateIndexerResponseObject, error) {
	out, err := h.svc.Indexers.UpdateIndexer(ctx, req.ID, fromAPIIndexerInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.UpdateIndexer200JSONResponse(toAPIIndexer(out)), nil
}

func (h *handlers) DeleteIndexer(ctx context.Context, req apigen.DeleteIndexerRequestObject) (apigen.DeleteIndexerResponseObject, error) {
	if err := h.svc.Indexers.DeleteIndexer(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteIndexer204Response{}, nil
}

func (h *handlers) TestIndexer(ctx context.Context, req apigen.TestIndexerRequestObject) (apigen.TestIndexerResponseObject, error) {
	out, err := h.svc.Indexers.TestIndexer(ctx, req.ID)
	if err != nil {
		// A failed test is a normal outcome, not a server error.
		return apigen.TestIndexer502JSONResponse{Error: err.Error()}, nil
	}
	return apigen.TestIndexer200JSONResponse(toAPIIndexer(out)), nil
}

// IndexerDefinitions lists the sites indexers can be added for and the
// settings each needs; used by settings and the setup wizard.
func (h *handlers) IndexerDefinitions(context.Context, apigen.IndexerDefinitionsRequestObject) (apigen.IndexerDefinitionsResponseObject, error) {
	return apigen.IndexerDefinitions200JSONResponse(mapSlice(h.svc.Indexers.IndexerDefinitions(), toAPIIndexerDefinition)), nil
}

// ReloadIndexerDefinitions re-reads the definitions directory.
func (h *handlers) ReloadIndexerDefinitions(context.Context, apigen.ReloadIndexerDefinitionsRequestObject) (apigen.ReloadIndexerDefinitionsResponseObject, error) {
	out := h.svc.Indexers.ReloadIndexerDefinitions()
	return apigen.ReloadIndexerDefinitions200JSONResponse{Definitions: mapSlice(out.Definitions, toAPIIndexerDefinition), Errors: nonNil(out.Errors)}, nil
}

// UploadIndexerDefinitions adds a definition file, or a zip of them.
func (h *handlers) UploadIndexerDefinitions(_ context.Context, req apigen.UploadIndexerDefinitionsRequestObject) (apigen.UploadIndexerDefinitionsResponseObject, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read upload: %v", errBadRequest, err)
	}
	out, err := h.svc.Indexers.UploadIndexerDefinitions(data)
	if err != nil {
		return nil, err
	}
	return apigen.UploadIndexerDefinitions200JSONResponse{
		Saved:       nonNil(out.Saved),
		Errors:      nonNil(out.Errors),
		Definitions: mapSlice(out.Definitions, toAPIIndexerDefinition),
	}, nil
}

// DeleteIndexerDefinition removes a definition's file.
func (h *handlers) DeleteIndexerDefinition(_ context.Context, req apigen.DeleteIndexerDefinitionRequestObject) (apigen.DeleteIndexerDefinitionResponseObject, error) {
	if err := h.svc.Indexers.DeleteIndexerDefinition(req.DefinitionID); err != nil {
		return nil, err
	}
	return apigen.DeleteIndexerDefinition204Response{}, nil
}

func toAPIIndexerDefinition(k indexer.Definition) apigen.IndexerDefinition {
	return apigen.IndexerDefinition{
		ID:          k.ID,
		Name:        k.Name,
		Description: optional(k.Description),
		Fields:      mapSlice(k.Fields, toAPISettingField),
		Unsupported: optional(k.Unsupported),
		RSS:         k.RSS,
	}
}

func toAPIIndexer(ix indexers.Indexer) apigen.Indexer {
	return apigen.Indexer{
		ID:              ix.ID,
		Name:            ix.Name,
		Definition:      ix.Definition,
		Settings:        apigen.SettingValues(nonNilMap(ix.Settings)),
		SecretsSet:      optSlice(ix.SecretsSet, identity[string]),
		MovieCategories: nonNil(ix.MovieCategories),
		TVCategories:    nonNil(ix.TVCategories),
		Enabled:         ix.Enabled,
		EnableRSS:       ix.EnableRSS,
		EnableSearch:    ix.EnableSearch,
		Priority:        ix.Priority,
		MinSeeders:      ix.MinSeeders,
		Caps:            toAPICaps(ix.Caps),
		LastError:       ix.LastError,
		LastErrorAt:     ix.LastErrorAt,
	}
}

func toAPICaps(c *indexer.Caps) *apigen.IndexerCaps {
	if c == nil {
		return nil
	}
	return &apigen.IndexerCaps{
		Search:      toAPISearchCap(c.Search),
		MovieSearch: toAPISearchCap(c.MovieSearch),
		TVSearch:    toAPISearchCap(c.TVSearch),
		Categories:  mapSlice(c.Categories, toAPICategory),
	}
}

func toAPISearchCap(s indexer.SearchCap) apigen.SearchCap {
	return apigen.SearchCap{Available: s.Available, Params: nonNil(s.Params)}
}

func toAPICategory(c indexer.Category) apigen.IndexerCategory {
	return apigen.IndexerCategory{ID: c.ID, Name: c.Name, Subcats: optSlice(c.Subcats, toAPICategory)}
}

func fromAPIIndexerInput(in apigen.IndexerInput) indexers.IndexerInput {
	return indexers.IndexerInput{
		Name:            deref(in.Name),
		Definition:      deref(in.Definition),
		Settings:        formspec.Values(deref(in.Settings)),
		MovieCategories: derefSlice(in.MovieCategories),
		TVCategories:    derefSlice(in.TVCategories),
		Enabled:         in.Enabled,
		EnableRSS:       in.EnableRSS,
		EnableSearch:    in.EnableSearch,
		Priority:        in.Priority,
		MinSeeders:      in.MinSeeders,
	}
}
