package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/library"
)

func (h *handlers) GetGeneralSettings(ctx context.Context, _ apigen.GetGeneralSettingsRequestObject) (apigen.GetGeneralSettingsResponseObject, error) {
	out, err := h.svc.GeneralSettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetGeneralSettings200JSONResponse(toAPIGeneralSettings(out)), nil
}

func (h *handlers) PutGeneralSettings(ctx context.Context, req apigen.PutGeneralSettingsRequestObject) (apigen.PutGeneralSettingsResponseObject, error) {
	out, err := h.svc.UpdateGeneralSettings(ctx, library.GeneralSettingsInput{TMDBAPIKey: req.Body.TMDBAPIKey})
	if err != nil {
		return nil, err
	}
	return apigen.PutGeneralSettings200JSONResponse(toAPIGeneralSettings(out)), nil
}

func (h *handlers) RegenerateAPIKey(ctx context.Context, _ apigen.RegenerateAPIKeyRequestObject) (apigen.RegenerateAPIKeyResponseObject, error) {
	key, err := h.svc.RegenerateAPIKey(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.RegenerateAPIKey200JSONResponse{APIKey: key}, nil
}

func (h *handlers) GetDownloadSettings(ctx context.Context, _ apigen.GetDownloadSettingsRequestObject) (apigen.GetDownloadSettingsResponseObject, error) {
	s, err := h.svc.DownloadSettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetDownloadSettings200JSONResponse(toAPIDownloadSettings(s)), nil
}

func (h *handlers) PutDownloadSettings(ctx context.Context, req apigen.PutDownloadSettingsRequestObject) (apigen.PutDownloadSettingsResponseObject, error) {
	out, err := h.svc.UpdateDownloadSettings(ctx, fromAPIDownloadSettings(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.PutDownloadSettings200JSONResponse(toAPISettingsUpdate(out)), nil
}

func (h *handlers) GetQualitySettings(ctx context.Context, _ apigen.GetQualitySettingsRequestObject) (apigen.GetQualitySettingsResponseObject, error) {
	s, err := h.svc.QualitySettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetQualitySettings200JSONResponse(toAPIQualitySettings(s)), nil
}

func (h *handlers) PutQualitySettings(ctx context.Context, req apigen.PutQualitySettingsRequestObject) (apigen.PutQualitySettingsResponseObject, error) {
	out, err := h.svc.UpdateQualitySettings(ctx, fromAPIQualitySettings(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.PutQualitySettings200JSONResponse(toAPIQualitySettings(out)), nil
}

func (h *handlers) ListRootFolders(ctx context.Context, req apigen.ListRootFoldersRequestObject) (apigen.ListRootFoldersResponseObject, error) {
	mt := library.MediaMovie
	if req.Params.MediaType != nil {
		mt = library.MediaType(*req.Params.MediaType)
	}
	out, err := h.svc.RootFolders(ctx, mt)
	if err != nil {
		return nil, err
	}
	return apigen.ListRootFolders200JSONResponse(mapSlice(out, toAPIRootFolder)), nil
}

func (h *handlers) CreateRootFolder(ctx context.Context, req apigen.CreateRootFolderRequestObject) (apigen.CreateRootFolderResponseObject, error) {
	out, err := h.svc.AddRootFolder(ctx, library.MediaType(req.Body.MediaType), req.Body.Path)
	if err != nil {
		return nil, err
	}
	return apigen.CreateRootFolder201JSONResponse(toAPIRootFolder(out)), nil
}

func (h *handlers) SetDefaultRootFolder(ctx context.Context, req apigen.SetDefaultRootFolderRequestObject) (apigen.SetDefaultRootFolderResponseObject, error) {
	if err := h.svc.SetDefaultRootFolder(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.SetDefaultRootFolder204Response{}, nil
}

func (h *handlers) DeleteRootFolder(ctx context.Context, req apigen.DeleteRootFolderRequestObject) (apigen.DeleteRootFolderResponseObject, error) {
	if err := h.svc.DeleteRootFolder(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteRootFolder204Response{}, nil
}

func toAPIGeneralSettings(s library.GeneralSettings) apigen.GeneralSettings {
	return apigen.GeneralSettings{TMDBAPIKeySet: s.TMDBAPIKeySet}
}

func toAPIDownloadSettings(s library.DownloadSettings) apigen.DownloadSettings {
	return apigen.DownloadSettings{
		IncompleteDir: s.IncompleteDir,
		CompleteDir:   s.CompleteDir,
		ImportMode:    apigen.ImportMode(s.ImportMode),
	}
}

func fromAPIDownloadSettings(in apigen.DownloadSettings) library.DownloadSettings {
	return library.DownloadSettings{
		IncompleteDir: in.IncompleteDir,
		CompleteDir:   in.CompleteDir,
		ImportMode:    importer.Mode(in.ImportMode),
	}
}

// fromAPIDownloadSettingsInput leaves omitted fields empty, for the library
// to fill with defaults.
func fromAPIDownloadSettingsInput(in *apigen.DownloadSettingsInput) library.DownloadSettings {
	if in == nil {
		return library.DownloadSettings{}
	}
	return library.DownloadSettings{
		IncompleteDir: deref(in.IncompleteDir),
		CompleteDir:   deref(in.CompleteDir),
		ImportMode:    importer.Mode(deref(in.ImportMode)),
	}
}

func toAPISettingsUpdate(u library.SettingsUpdate) apigen.SettingsUpdate {
	return apigen.SettingsUpdate{
		Settings: toAPIDownloadSettings(u.Settings),
		Warnings: optSlice(u.Warnings, identity[string]),
	}
}

func toAPIQualitySettings(q library.QualitySettings) apigen.QualitySettings {
	return apigen.QualitySettings{MinResolution: q.MinResolution, MaxResolution: q.MaxResolution, MaxSizeMB: q.MaxSizeMB}
}

func fromAPIQualitySettings(in apigen.QualitySettingsInput) library.QualitySettings {
	return library.QualitySettings{MinResolution: in.MinResolution, MaxResolution: in.MaxResolution, MaxSizeMB: deref(in.MaxSizeMB)}
}

func toAPIRootFolder(f library.RootFolder) apigen.RootFolder {
	return apigen.RootFolder{ID: f.ID, MediaType: apigen.MediaType(f.MediaType), Path: f.Path, IsDefault: f.IsDefault}
}
