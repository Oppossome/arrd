package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
)

func (h *handlers) LookupSeries(ctx context.Context, req apigen.LookupSeriesRequestObject) (apigen.LookupSeriesResponseObject, error) {
	out, err := h.svc.Media.LookupSeries(ctx, deref(req.Params.Q), deref(req.Params.Year), fromAPITitleFilter(req.Params.Genre, req.Params.Lang))
	if err != nil {
		return nil, err
	}
	return apigen.LookupSeries200JSONResponse(mapSlice(out, toAPIDiscoverSeries)), nil
}

func (h *handlers) ListSeries(ctx context.Context, _ apigen.ListSeriesRequestObject) (apigen.ListSeriesResponseObject, error) {
	out, err := h.svc.Media.SeriesList(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.ListSeries200JSONResponse(mapSlice(out, toAPISeries)), nil
}

func (h *handlers) AddSeries(ctx context.Context, req apigen.AddSeriesRequestObject) (apigen.AddSeriesResponseObject, error) {
	out, err := h.svc.Media.AddSeries(ctx, fromAPIAddSeries(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.AddSeries201JSONResponse(toAPISeries(out)), nil
}

func (h *handlers) GetSeries(ctx context.Context, req apigen.GetSeriesRequestObject) (apigen.GetSeriesResponseObject, error) {
	out, err := h.svc.Media.SeriesDetail(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.GetSeries200JSONResponse(toAPISeries(out)), nil
}

func (h *handlers) UpdateSeries(ctx context.Context, req apigen.UpdateSeriesRequestObject) (apigen.UpdateSeriesResponseObject, error) {
	out, err := h.svc.Media.UpdateSeries(ctx, req.ID, fromAPIUpdateSeries(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.UpdateSeries200JSONResponse(toAPISeries(out)), nil
}

func (h *handlers) DeleteSeries(ctx context.Context, req apigen.DeleteSeriesRequestObject) (apigen.DeleteSeriesResponseObject, error) {
	if err := h.svc.Media.DeleteSeries(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteSeries204Response{}, nil
}

func (h *handlers) RefreshSeries(ctx context.Context, req apigen.RefreshSeriesRequestObject) (apigen.RefreshSeriesResponseObject, error) {
	out, err := h.svc.Media.RefreshSeries(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.RefreshSeries200JSONResponse(toAPISeries(out)), nil
}

func (h *handlers) SeriesEpisodes(ctx context.Context, req apigen.SeriesEpisodesRequestObject) (apigen.SeriesEpisodesResponseObject, error) {
	out, err := h.svc.Media.SeriesEpisodes(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesEpisodes200JSONResponse(mapSlice(out, toAPIEpisode)), nil
}

func (h *handlers) SearchSeriesWanted(ctx context.Context, req apigen.SearchSeriesWantedRequestObject) (apigen.SearchSeriesWantedResponseObject, error) {
	if _, err := h.svc.Media.SeriesDetail(ctx, req.ID); err != nil {
		return nil, err
	}
	id := req.ID
	if err := h.svc.Acquire.SearchWantedEpisodes(ctx, &id); err != nil {
		return nil, err
	}
	return apigen.SearchSeriesWanted204Response{}, nil
}

func (h *handlers) GrabSeries(ctx context.Context, req apigen.GrabSeriesRequestObject) (apigen.GrabSeriesResponseObject, error) {
	out, err := h.svc.Acquire.GrabSeriesRelease(ctx, req.ID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.GrabSeries201JSONResponse(toAPIDownload(out)), nil
}

func (h *handlers) UpdateSeason(ctx context.Context, req apigen.UpdateSeasonRequestObject) (apigen.UpdateSeasonResponseObject, error) {
	if err := h.svc.Media.SetSeasonMonitored(ctx, req.ID, req.Season, req.Body.Monitored); err != nil {
		return nil, err
	}
	return apigen.UpdateSeason204Response{}, nil
}

func (h *handlers) SeasonReleases(ctx context.Context, req apigen.SeasonReleasesRequestObject) (apigen.SeasonReleasesResponseObject, error) {
	out, err := h.svc.Acquire.SearchSeason(ctx, req.ID, req.Season)
	if err != nil {
		return nil, err
	}
	return apigen.SeasonReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) SearchSeason(ctx context.Context, req apigen.SearchSeasonRequestObject) (apigen.SearchSeasonResponseObject, error) {
	out, err := h.svc.Acquire.AutoSearchSeason(ctx, req.ID, req.Season)
	if err != nil {
		return nil, err
	}
	return apigen.SearchSeason200JSONResponse(mapSlice(out, toAPIAutoSearchResult)), nil
}

func (h *handlers) UpdateEpisode(ctx context.Context, req apigen.UpdateEpisodeRequestObject) (apigen.UpdateEpisodeResponseObject, error) {
	out, err := h.svc.Media.SetEpisodeMonitored(ctx, req.ID, req.Body.Monitored)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateEpisode200JSONResponse(toAPIEpisode(out)), nil
}

func (h *handlers) EpisodeReleases(ctx context.Context, req apigen.EpisodeReleasesRequestObject) (apigen.EpisodeReleasesResponseObject, error) {
	out, err := h.svc.Acquire.SearchEpisode(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.EpisodeReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) SearchEpisode(ctx context.Context, req apigen.SearchEpisodeRequestObject) (apigen.SearchEpisodeResponseObject, error) {
	out, err := h.svc.Acquire.AutoSearchEpisode(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.SearchEpisode200JSONResponse(toAPIAutoSearchResult(out)), nil
}

func (h *handlers) GrabEpisode(ctx context.Context, req apigen.GrabEpisodeRequestObject) (apigen.GrabEpisodeResponseObject, error) {
	out, err := h.svc.Acquire.GrabEpisodeRelease(ctx, req.ID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.GrabEpisode201JSONResponse(toAPIDownload(out)), nil
}

func toAPISeries(s media.Series) apigen.Series {
	return apigen.Series{
		ID:              s.ID,
		TMDBID:          s.TMDBID,
		TVDBID:          s.TVDBID,
		IMDBID:          s.IMDBID,
		Title:           s.Title,
		OriginalTitle:   s.OriginalTitle,
		Year:            s.Year,
		Overview:        s.Overview,
		Status:          s.Status,
		PosterPath:      s.PosterPath,
		BackdropPath:    s.BackdropPath,
		RootFolderID:    s.RootFolderID,
		Monitored:       s.Monitored,
		AddedAt:         s.AddedAt,
		LastRefreshedAt: s.LastRefreshedAt,
		Seasons:         mapSlice(s.Seasons, toAPISeasonStatus),
	}
}

func toAPISeasonStatus(s media.SeasonStatus) apigen.SeasonStatus {
	return apigen.SeasonStatus{
		Season:     s.Number,
		Episodes:   s.Episodes,
		Downloaded: s.Downloaded,
		Monitored:  s.Monitored,
		Aired:      s.Aired,
		Complete:   s.Complete,
	}
}

func toAPIEpisode(e media.Episode) apigen.Episode {
	return apigen.Episode{
		ID:          e.ID,
		SeriesID:    e.SeriesID,
		Season:      e.Season,
		Episode:     e.Episode,
		Title:       e.Title,
		AirDate:     e.AirDate,
		Monitored:   e.Monitored,
		FilePath:    e.FilePath,
		FileQuality: e.FileQuality,
		Downloading: e.Downloading,
	}
}

func toAPILookupSeries(s metadata.Series) apigen.LookupSeries {
	return apigen.LookupSeries{
		TMDBID:        s.TMDBID,
		TVDBID:        optional(s.TVDBID),
		IMDBID:        optional(s.IMDBID),
		Title:         s.Title,
		OriginalTitle: s.OriginalTitle,
		Year:          s.Year,
		Overview:      s.Overview,
		Status:        optional(s.Status),
		PosterPath:    optional(s.PosterPath),
		BackdropPath:  optional(s.BackdropPath),
		Seasons: optSlice(s.Seasons, func(ss metadata.SeasonSummary) apigen.LookupSeason {
			return apigen.LookupSeason{
				SeasonNumber: ss.Number, EpisodeCount: ss.EpisodeCount,
				Name: optional(ss.Name), PosterPath: optional(ss.PosterPath), AirDate: optional(ss.AirDate),
			}
		}),
	}
}

func fromAPIAddSeries(in apigen.AddSeriesRequest) media.AddSeriesInput {
	return media.AddSeriesInput{
		TMDBID:       in.TMDBID,
		RootFolderID: in.RootFolderID,
		Monitor:      media.MonitorMode(deref(in.Monitor)),
		Search:       deref(in.Search),
	}
}

func fromAPIUpdateSeries(in apigen.UpdateSeriesRequest) media.UpdateSeriesInput {
	return media.UpdateSeriesInput{Monitored: in.Monitored, RootFolderID: in.RootFolderID}
}

func (h *handlers) SeriesReleases(ctx context.Context, req apigen.SeriesReleasesRequestObject) (apigen.SeriesReleasesResponseObject, error) {
	out, err := h.svc.Acquire.SearchSeriesPacks(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesReleases200JSONResponse(toAPISearchResult(out)), nil
}
