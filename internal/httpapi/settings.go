package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/library/settings"
)

func (h *handlers) GetGeneralSettings(ctx context.Context, _ apigen.GetGeneralSettingsRequestObject) (apigen.GetGeneralSettingsResponseObject, error) {
	out, err := h.svc.Settings.GeneralSettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetGeneralSettings200JSONResponse(toAPIGeneralSettings(out)), nil
}

func (h *handlers) PutGeneralSettings(ctx context.Context, req apigen.PutGeneralSettingsRequestObject) (apigen.PutGeneralSettingsResponseObject, error) {
	out, err := h.svc.Settings.UpdateGeneralSettings(ctx, settings.GeneralSettingsInput{TMDBAPIKey: req.Body.TMDBAPIKey})
	if err != nil {
		return nil, err
	}
	return apigen.PutGeneralSettings200JSONResponse(toAPIGeneralSettings(out)), nil
}

func (h *handlers) GetNetworkSettings(ctx context.Context, _ apigen.GetNetworkSettingsRequestObject) (apigen.GetNetworkSettingsResponseObject, error) {
	out, err := h.svc.Settings.NetworkSettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetNetworkSettings200JSONResponse(toAPINetworkSettings(out)), nil
}

func (h *handlers) PutNetworkSettings(ctx context.Context, req apigen.PutNetworkSettingsRequestObject) (apigen.PutNetworkSettingsResponseObject, error) {
	out, err := h.svc.Settings.UpdateNetworkSettings(ctx, settings.NetworkSettingsInput{ExposeLAN: req.Body.ExposeLAN})
	if err != nil {
		return nil, err
	}
	return apigen.PutNetworkSettings200JSONResponse(toAPINetworkSettings(out)), nil
}

func (h *handlers) RegenerateAPIKey(ctx context.Context, _ apigen.RegenerateAPIKeyRequestObject) (apigen.RegenerateAPIKeyResponseObject, error) {
	key, err := h.svc.Auth.RegenerateAPIKey(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.RegenerateAPIKey200JSONResponse{APIKey: key}, nil
}

func (h *handlers) GetDownloadSettings(ctx context.Context, _ apigen.GetDownloadSettingsRequestObject) (apigen.GetDownloadSettingsResponseObject, error) {
	s, err := h.svc.Settings.DownloadSettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetDownloadSettings200JSONResponse(toAPIDownloadSettings(s)), nil
}

func (h *handlers) PutDownloadSettings(ctx context.Context, req apigen.PutDownloadSettingsRequestObject) (apigen.PutDownloadSettingsResponseObject, error) {
	out, err := h.svc.Settings.UpdateDownloadSettings(ctx, fromAPIDownloadSettings(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.PutDownloadSettings200JSONResponse(toAPISettingsUpdate(out)), nil
}

func (h *handlers) GetQualitySettings(ctx context.Context, _ apigen.GetQualitySettingsRequestObject) (apigen.GetQualitySettingsResponseObject, error) {
	s, err := h.svc.Settings.QualitySettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetQualitySettings200JSONResponse(toAPIQualitySettings(s)), nil
}

func (h *handlers) PutQualitySettings(ctx context.Context, req apigen.PutQualitySettingsRequestObject) (apigen.PutQualitySettingsResponseObject, error) {
	out, err := h.svc.Settings.UpdateQualitySettings(ctx, fromAPIQualitySettings(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.PutQualitySettings200JSONResponse(toAPIQualitySettings(out)), nil
}

func (h *handlers) ListRootFolders(ctx context.Context, req apigen.ListRootFoldersRequestObject) (apigen.ListRootFoldersResponseObject, error) {
	mt := settings.MediaMovie
	if req.Params.MediaType != nil {
		mt = settings.MediaType(*req.Params.MediaType)
	}
	out, err := h.svc.Settings.RootFolders(ctx, mt)
	if err != nil {
		return nil, err
	}
	return apigen.ListRootFolders200JSONResponse(mapSlice(out, toAPIRootFolder)), nil
}

func (h *handlers) CreateRootFolder(ctx context.Context, req apigen.CreateRootFolderRequestObject) (apigen.CreateRootFolderResponseObject, error) {
	out, err := h.svc.Settings.AddRootFolder(ctx, settings.MediaType(req.Body.MediaType), req.Body.Path)
	if err != nil {
		return nil, err
	}
	return apigen.CreateRootFolder201JSONResponse(toAPIRootFolder(out)), nil
}

func (h *handlers) SetDefaultRootFolder(ctx context.Context, req apigen.SetDefaultRootFolderRequestObject) (apigen.SetDefaultRootFolderResponseObject, error) {
	if err := h.svc.Settings.SetDefaultRootFolder(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.SetDefaultRootFolder204Response{}, nil
}

func (h *handlers) DeleteRootFolder(ctx context.Context, req apigen.DeleteRootFolderRequestObject) (apigen.DeleteRootFolderResponseObject, error) {
	if err := h.svc.Settings.DeleteRootFolder(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteRootFolder204Response{}, nil
}

func toAPIGeneralSettings(s settings.GeneralSettings) apigen.GeneralSettings {
	out := apigen.GeneralSettings{TMDBAPIKeySet: s.TMDBAPIKeySet, APIKeyHint: s.APIKeyHint}
	if s.TMDBAPIKeyHint != "" {
		out.TMDBAPIKeyHint = &s.TMDBAPIKeyHint
	}
	return out
}

func toAPIDownloadSettings(s settings.DownloadSettings) apigen.DownloadSettings {
	return apigen.DownloadSettings{
		IncompleteDir: s.IncompleteDir,
		CompleteDir:   s.CompleteDir,
		ImportMode:    apigen.ImportMode(s.ImportMode),
	}
}

func fromAPIDownloadSettings(in apigen.DownloadSettings) settings.DownloadSettings {
	return settings.DownloadSettings{
		IncompleteDir: in.IncompleteDir,
		CompleteDir:   in.CompleteDir,
		ImportMode:    importer.Mode(in.ImportMode),
	}
}

// fromAPIDownloadSettingsInput leaves omitted fields empty, for the library
// to fill with defaults.
func fromAPIDownloadSettingsInput(in *apigen.DownloadSettingsInput) settings.DownloadSettings {
	if in == nil {
		return settings.DownloadSettings{}
	}
	return settings.DownloadSettings{
		IncompleteDir: deref(in.IncompleteDir),
		CompleteDir:   deref(in.CompleteDir),
		ImportMode:    importer.Mode(deref(in.ImportMode)),
	}
}

func toAPISettingsUpdate(u settings.SettingsUpdate) apigen.SettingsUpdate {
	return apigen.SettingsUpdate{
		Settings: toAPIDownloadSettings(u.Settings),
		Warnings: optSlice(u.Warnings, identity[string]),
	}
}

func toAPIQualitySettings(q settings.QualitySettings) apigen.QualitySettings {
	return apigen.QualitySettings{MinResolution: q.MinResolution, MaxResolution: q.MaxResolution, MaxSizeMB: q.MaxSizeMB}
}

func fromAPIQualitySettings(in apigen.QualitySettingsInput) settings.QualitySettings {
	return settings.QualitySettings{MinResolution: in.MinResolution, MaxResolution: in.MaxResolution, MaxSizeMB: deref(in.MaxSizeMB)}
}

func toAPIRootFolder(f settings.RootFolder) apigen.RootFolder {
	return apigen.RootFolder{ID: f.ID, MediaType: apigen.MediaType(f.MediaType), Path: f.Path, IsDefault: f.IsDefault}
}

func toAPINetworkSettings(s settings.NetworkSettings) apigen.NetworkSettings {
	out := apigen.NetworkSettings{ExposeLAN: s.ExposeLAN, Hostname: s.Hostname}
	if s.LANURL != "" {
		out.LANURL = &s.LANURL
	}
	return out
}
