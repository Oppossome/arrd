package httpapi

import (
	"context"
	"errors"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/mediaservers"
	"generic-arr-project/internal/mediaserver"
)

func (h *handlers) ListMediaServers(ctx context.Context, _ apigen.ListMediaServersRequestObject) (apigen.ListMediaServersResponseObject, error) {
	out, err := h.svc.MediaServers.List(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.ListMediaServers200JSONResponse(mapSlice(out, toAPIMediaServer)), nil
}

func (h *handlers) CreateMediaServer(ctx context.Context, req apigen.CreateMediaServerRequestObject) (apigen.CreateMediaServerResponseObject, error) {
	out, err := h.svc.MediaServers.Create(ctx, fromAPIMediaServerInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.CreateMediaServer201JSONResponse(toAPIMediaServer(out)), nil
}

func (h *handlers) GetMediaServer(ctx context.Context, req apigen.GetMediaServerRequestObject) (apigen.GetMediaServerResponseObject, error) {
	out, err := h.svc.MediaServers.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.GetMediaServer200JSONResponse(toAPIMediaServer(out)), nil
}

func (h *handlers) UpdateMediaServer(ctx context.Context, req apigen.UpdateMediaServerRequestObject) (apigen.UpdateMediaServerResponseObject, error) {
	out, err := h.svc.MediaServers.Update(ctx, req.ID, fromAPIMediaServerInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.UpdateMediaServer200JSONResponse(toAPIMediaServer(out)), nil
}

func (h *handlers) DeleteMediaServer(ctx context.Context, req apigen.DeleteMediaServerRequestObject) (apigen.DeleteMediaServerResponseObject, error) {
	if err := h.svc.MediaServers.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteMediaServer204Response{}, nil
}

func (h *handlers) TestMediaServer(ctx context.Context, req apigen.TestMediaServerRequestObject) (apigen.TestMediaServerResponseObject, error) {
	out, err := h.svc.MediaServers.Test(ctx, req.ID)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		// A failed test is a normal outcome, not a server error.
		return apigen.TestMediaServer502JSONResponse{Error: err.Error()}, nil
	}
	return apigen.TestMediaServer200JSONResponse(toAPIMediaServer(out)), nil
}

func (h *handlers) MediaServerDefinitions(context.Context, apigen.MediaServerDefinitionsRequestObject) (apigen.MediaServerDefinitionsResponseObject, error) {
	return apigen.MediaServerDefinitions200JSONResponse(mapSlice(h.svc.MediaServers.Definitions(), toAPIMediaServerDefinition)), nil
}

func toAPIMediaServerDefinition(d mediaserver.Definition) apigen.MediaServerDefinition {
	return apigen.MediaServerDefinition{
		ID:          d.ID,
		Name:        d.Name,
		Description: optional(d.Description),
		Fields:      mapSlice(d.Fields, toAPISettingField),
	}
}

func toAPIMediaServer(m mediaservers.MediaServer) apigen.MediaServer {
	return apigen.MediaServer{
		ID:          m.ID,
		Name:        m.Name,
		Definition:  m.Definition,
		Settings:    apigen.SettingValues(nonNilMap(m.Settings)),
		SecretsSet:  optSlice(m.SecretsSet, identity[string]),
		Enabled:     m.Enabled,
		LastError:   m.LastError,
		LastErrorAt: m.LastErrorAt,
	}
}

func fromAPIMediaServerInput(in apigen.MediaServerInput) mediaservers.Input {
	return mediaservers.Input{
		Name:       deref(in.Name),
		Definition: deref(in.Definition),
		Settings:   formspec.Values(deref(in.Settings)),
		Enabled:    in.Enabled,
	}
}
