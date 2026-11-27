package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library/acquire"
)

func (h *handlers) SearchReleases(ctx context.Context, req apigen.SearchReleasesRequestObject) (apigen.SearchReleasesResponseObject, error) {
	p := req.Params
	t := acquire.Target{Kind: acquire.Kind(p.Kind), TMDBID: p.TMDBID, Season: p.Season, Episode: p.Episode}
	out, err := h.svc.Acquire.Search(ctx, t, deref(p.Q))
	if err != nil {
		return nil, err
	}
	return apigen.SearchReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) GrabRelease(ctx context.Context, req apigen.GrabReleaseRequestObject) (apigen.GrabReleaseResponseObject, error) {
	out, err := h.svc.Acquire.Grab(ctx, fromAPITarget(req.Body.Target), fromAPIRelease(req.Body.Release))
	if err != nil {
		return nil, err
	}
	return apigen.GrabRelease201JSONResponse(toAPIDownload(out)), nil
}

func (h *handlers) AutoSearch(ctx context.Context, req apigen.AutoSearchRequestObject) (apigen.AutoSearchResponseObject, error) {
	out, err := h.svc.Acquire.AutoSearch(ctx, fromAPITarget(req.Body.Target))
	if err != nil {
		return nil, err
	}
	return apigen.AutoSearch200JSONResponse(toAPIAutoSearchResult(out)), nil
}

func fromAPITarget(t apigen.ReleaseTarget) acquire.Target {
	return acquire.Target{Kind: acquire.Kind(t.Kind), TMDBID: t.TMDBID, Season: t.Season, Episode: t.Episode}
}
