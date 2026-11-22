package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/metadata"
)

func (h *handlers) MovieList(ctx context.Context, req apigen.MovieListRequestObject) (apigen.MovieListResponseObject, error) {
	out, err := h.svc.TMDBMovieList(ctx, metadata.MovieList(req.List))
	if err != nil {
		return nil, err
	}
	return apigen.MovieList200JSONResponse(mapSlice(out, toAPIDiscoverMovie)), nil
}

func (h *handlers) MovieGenres(ctx context.Context, _ apigen.MovieGenresRequestObject) (apigen.MovieGenresResponseObject, error) {
	out, err := h.svc.MovieGenres(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.MovieGenres200JSONResponse(mapSlice(out, toAPIGenre)), nil
}

func (h *handlers) BrowseMovies(ctx context.Context, req apigen.BrowseMoviesRequestObject) (apigen.BrowseMoviesResponseObject, error) {
	p := req.Params
	out, err := h.svc.BrowseMovies(ctx, fromAPIBrowse(p.Genre, p.Sort, p.Page))
	if err != nil {
		return nil, err
	}
	return apigen.BrowseMovies200JSONResponse{Page: out.Page, TotalPages: out.TotalPages, Items: mapSlice(out.Items, toAPIDiscoverMovie)}, nil
}

func (h *handlers) MoviesLike(ctx context.Context, req apigen.MoviesLikeRequestObject) (apigen.MoviesLikeResponseObject, error) {
	out, err := h.svc.MoviesLike(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.MoviesLike200JSONResponse(mapSlice(out, toAPIDiscoverMovie)), nil
}

func (h *handlers) DiscoverMovie(ctx context.Context, req apigen.DiscoverMovieRequestObject) (apigen.DiscoverMovieResponseObject, error) {
	out, err := h.svc.DiscoveredMovie(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverMovie200JSONResponse(toAPIDiscoverMovie(out)), nil
}

func (h *handlers) DiscoverMovieReleases(ctx context.Context, req apigen.DiscoverMovieReleasesRequestObject) (apigen.DiscoverMovieReleasesResponseObject, error) {
	out, err := h.svc.SearchDiscoveredMovie(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverMovieReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) DiscoverGrabMovie(ctx context.Context, req apigen.DiscoverGrabMovieRequestObject) (apigen.DiscoverGrabMovieResponseObject, error) {
	out, err := h.svc.GrabDiscoveredMovie(ctx, req.TMDBID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverGrabMovie201JSONResponse(toAPIDownload(out)), nil
}

func (h *handlers) SeriesList(ctx context.Context, req apigen.SeriesListRequestObject) (apigen.SeriesListResponseObject, error) {
	out, err := h.svc.TMDBSeriesList(ctx, metadata.SeriesList(req.List))
	if err != nil {
		return nil, err
	}
	return apigen.SeriesList200JSONResponse(mapSlice(out, toAPIDiscoverSeries)), nil
}

func (h *handlers) SeriesGenres(ctx context.Context, _ apigen.SeriesGenresRequestObject) (apigen.SeriesGenresResponseObject, error) {
	out, err := h.svc.SeriesGenres(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesGenres200JSONResponse(mapSlice(out, toAPIGenre)), nil
}

func (h *handlers) BrowseSeries(ctx context.Context, req apigen.BrowseSeriesRequestObject) (apigen.BrowseSeriesResponseObject, error) {
	p := req.Params
	out, err := h.svc.BrowseSeries(ctx, fromAPIBrowse(p.Genre, p.Sort, p.Page))
	if err != nil {
		return nil, err
	}
	return apigen.BrowseSeries200JSONResponse{Page: out.Page, TotalPages: out.TotalPages, Items: mapSlice(out.Items, toAPIDiscoverSeries)}, nil
}

func (h *handlers) SeriesLike(ctx context.Context, req apigen.SeriesLikeRequestObject) (apigen.SeriesLikeResponseObject, error) {
	out, err := h.svc.SeriesLike(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesLike200JSONResponse(mapSlice(out, toAPIDiscoverSeries)), nil
}

func (h *handlers) DiscoverSeries(ctx context.Context, req apigen.DiscoverSeriesRequestObject) (apigen.DiscoverSeriesResponseObject, error) {
	out, err := h.svc.DiscoveredSeries(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverSeries200JSONResponse(toAPIDiscoverSeries(out)), nil
}

func (h *handlers) DiscoverSeasonReleases(ctx context.Context, req apigen.DiscoverSeasonReleasesRequestObject) (apigen.DiscoverSeasonReleasesResponseObject, error) {
	out, err := h.svc.SearchDiscoveredSeason(ctx, req.TMDBID, req.Season)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverSeasonReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) DiscoverGrabSeries(ctx context.Context, req apigen.DiscoverGrabSeriesRequestObject) (apigen.DiscoverGrabSeriesResponseObject, error) {
	out, err := h.svc.GrabDiscoveredSeries(ctx, req.TMDBID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverGrabSeries201JSONResponse(toAPIDownload(out)), nil
}

func toAPIDiscoverMovie(m library.DiscoverMovie) apigen.LookupMovie {
	out := toAPILookupMovie(m.Movie)
	out.LibraryID = m.LibraryID
	return out
}

func toAPIDiscoverSeries(s library.DiscoverSeries) apigen.LookupSeries {
	out := toAPILookupSeries(s.Series)
	out.LibraryID = s.LibraryID
	return out
}

func (h *handlers) MovieRecommendations(ctx context.Context, _ apigen.MovieRecommendationsRequestObject) (apigen.MovieRecommendationsResponseObject, error) {
	rows, err := h.svc.MovieRecommendations(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.MovieRecommendations200JSONResponse(mapSlice(rows, func(r library.RecommendationRow[metadata.Movie]) apigen.MovieRecommendations {
		return apigen.MovieRecommendations{Because: toAPIRecommendationSource(r.Because), Items: mapSlice(r.Items, toAPILookupMovie)}
	})), nil
}

func (h *handlers) SeriesRecommendations(ctx context.Context, _ apigen.SeriesRecommendationsRequestObject) (apigen.SeriesRecommendationsResponseObject, error) {
	rows, err := h.svc.SeriesRecommendations(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesRecommendations200JSONResponse(mapSlice(rows, func(r library.RecommendationRow[metadata.Series]) apigen.SeriesRecommendations {
		return apigen.SeriesRecommendations{Because: toAPIRecommendationSource(r.Because), Items: mapSlice(r.Items, toAPILookupSeries)}
	})), nil
}

func toAPIRecommendationSource(s library.RecommendationSource) apigen.RecommendationSource {
	return apigen.RecommendationSource{TMDBID: s.TMDBID, Title: s.Title}
}

func toAPIGenre(g metadata.Genre) apigen.Genre {
	return apigen.Genre{ID: g.ID, Name: g.Name}
}

// fromAPIBrowse applies the spec's defaults (every genre, popular, page 1),
// which the generated server leaves unset.
func fromAPIBrowse(genre *int64, sort *apigen.DiscoverSort, page *int) library.BrowseQuery {
	q := library.BrowseQuery{GenreID: deref(genre), Sort: metadata.SortPopular, Page: 1}
	if sort != nil {
		q.Sort = metadata.DiscoverSort(*sort)
	}
	if page != nil {
		q.Page = *page
	}
	return q
}
