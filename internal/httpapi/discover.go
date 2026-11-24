package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
)

func (h *handlers) MovieList(ctx context.Context, req apigen.MovieListRequestObject) (apigen.MovieListResponseObject, error) {
	out, err := h.svc.Media.TMDBMovieList(ctx, metadata.MovieList(req.List))
	if err != nil {
		return nil, err
	}
	return apigen.MovieList200JSONResponse(mapSlice(out, toAPIDiscoverMovie)), nil
}

func (h *handlers) MovieGenres(ctx context.Context, _ apigen.MovieGenresRequestObject) (apigen.MovieGenresResponseObject, error) {
	out, err := h.svc.Media.MovieGenres(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.MovieGenres200JSONResponse(mapSlice(out, toAPIGenre)), nil
}

func (h *handlers) BrowseMovies(ctx context.Context, req apigen.BrowseMoviesRequestObject) (apigen.BrowseMoviesResponseObject, error) {
	p := req.Params
	out, err := h.svc.Media.BrowseMovies(ctx, fromAPIBrowse(fromAPITitleFilter(p.Genre, p.Lang), p.Sort, p.Page))
	if err != nil {
		return nil, err
	}
	return apigen.BrowseMovies200JSONResponse{Page: out.Page, TotalPages: out.TotalPages, Items: mapSlice(out.Items, toAPIDiscoverMovie)}, nil
}

func (h *handlers) MoviesLike(ctx context.Context, req apigen.MoviesLikeRequestObject) (apigen.MoviesLikeResponseObject, error) {
	out, err := h.svc.Media.MoviesLike(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.MoviesLike200JSONResponse(mapSlice(out, toAPIDiscoverMovie)), nil
}

func (h *handlers) DiscoverMovie(ctx context.Context, req apigen.DiscoverMovieRequestObject) (apigen.DiscoverMovieResponseObject, error) {
	out, err := h.svc.Media.DiscoveredMovie(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	movie := toAPIDiscoverMovie(out)
	movie.Extras = toAPIMediaExtras(out.Extras, nil)
	return apigen.DiscoverMovie200JSONResponse(movie), nil
}

func (h *handlers) DiscoverMovieReleases(ctx context.Context, req apigen.DiscoverMovieReleasesRequestObject) (apigen.DiscoverMovieReleasesResponseObject, error) {
	out, err := h.svc.Acquire.SearchDiscoveredMovie(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverMovieReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) DiscoverGrabMovie(ctx context.Context, req apigen.DiscoverGrabMovieRequestObject) (apigen.DiscoverGrabMovieResponseObject, error) {
	out, err := h.svc.Acquire.GrabDiscoveredMovie(ctx, req.TMDBID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverGrabMovie201JSONResponse(toAPIDownload(out)), nil
}

func (h *handlers) SeriesList(ctx context.Context, req apigen.SeriesListRequestObject) (apigen.SeriesListResponseObject, error) {
	out, err := h.svc.Media.TMDBSeriesList(ctx, metadata.SeriesList(req.List))
	if err != nil {
		return nil, err
	}
	return apigen.SeriesList200JSONResponse(mapSlice(out, toAPIDiscoverSeries)), nil
}

func (h *handlers) SeriesGenres(ctx context.Context, _ apigen.SeriesGenresRequestObject) (apigen.SeriesGenresResponseObject, error) {
	out, err := h.svc.Media.SeriesGenres(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesGenres200JSONResponse(mapSlice(out, toAPIGenre)), nil
}

func (h *handlers) BrowseSeries(ctx context.Context, req apigen.BrowseSeriesRequestObject) (apigen.BrowseSeriesResponseObject, error) {
	p := req.Params
	out, err := h.svc.Media.BrowseSeries(ctx, fromAPIBrowse(fromAPITitleFilter(p.Genre, p.Lang), p.Sort, p.Page))
	if err != nil {
		return nil, err
	}
	return apigen.BrowseSeries200JSONResponse{Page: out.Page, TotalPages: out.TotalPages, Items: mapSlice(out.Items, toAPIDiscoverSeries)}, nil
}

func (h *handlers) SeriesLike(ctx context.Context, req apigen.SeriesLikeRequestObject) (apigen.SeriesLikeResponseObject, error) {
	out, err := h.svc.Media.SeriesLike(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesLike200JSONResponse(mapSlice(out, toAPIDiscoverSeries)), nil
}

func (h *handlers) DiscoverSeries(ctx context.Context, req apigen.DiscoverSeriesRequestObject) (apigen.DiscoverSeriesResponseObject, error) {
	out, err := h.svc.Media.DiscoveredSeries(ctx, req.TMDBID)
	if err != nil {
		return nil, err
	}
	series := toAPIDiscoverSeries(out)
	series.Extras = toAPIMediaExtras(out.Extras, out.Networks)
	return apigen.DiscoverSeries200JSONResponse(series), nil
}

func (h *handlers) DiscoverSeason(ctx context.Context, req apigen.DiscoverSeasonRequestObject) (apigen.DiscoverSeasonResponseObject, error) {
	out, err := h.svc.Media.DiscoveredSeason(ctx, req.TMDBID, req.Season)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverSeason200JSONResponse(toAPISeasonDetails(out)), nil
}

func (h *handlers) DiscoverEpisodeReleases(ctx context.Context, req apigen.DiscoverEpisodeReleasesRequestObject) (apigen.DiscoverEpisodeReleasesResponseObject, error) {
	out, err := h.svc.Acquire.SearchDiscoveredEpisode(ctx, req.TMDBID, req.Season, req.Episode)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverEpisodeReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) DiscoverSeasonReleases(ctx context.Context, req apigen.DiscoverSeasonReleasesRequestObject) (apigen.DiscoverSeasonReleasesResponseObject, error) {
	out, err := h.svc.Acquire.SearchDiscoveredSeason(ctx, req.TMDBID, req.Season)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverSeasonReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) DiscoverGrabSeries(ctx context.Context, req apigen.DiscoverGrabSeriesRequestObject) (apigen.DiscoverGrabSeriesResponseObject, error) {
	out, err := h.svc.Acquire.GrabDiscoveredSeries(ctx, req.TMDBID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverGrabSeries201JSONResponse(toAPIDownload(out)), nil
}

func toAPIDiscoverMovie(m media.DiscoverMovie) apigen.LookupMovie {
	out := toAPILookupMovie(m.Movie)
	out.LibraryID = m.LibraryID
	return out
}

func toAPIDiscoverSeries(s media.DiscoverSeries) apigen.LookupSeries {
	out := toAPILookupSeries(s.Series)
	out.LibraryID = s.LibraryID
	return out
}

func toAPIMediaExtras(e metadata.Extras, networks []string) *apigen.MediaExtras {
	out := &apigen.MediaExtras{
		Tagline:       optional(e.Tagline),
		Runtime:       optional(e.Runtime),
		VoteAverage:   optional(e.VoteAverage),
		VoteCount:     optional(e.VoteCount),
		Certification: optional(e.Certification),
		Genres:        mapSlice(e.Genres, toAPIGenre),
		Networks:      optSlice(networks, func(n string) string { return n }),
		Cast: mapSlice(e.Cast, func(c metadata.CastMember) apigen.CastMember {
			return apigen.CastMember{Name: c.Name, Character: optional(c.Character), ProfilePath: optional(c.ProfilePath)}
		}),
	}
	if e.Trailer != nil {
		out.Trailer = &apigen.Trailer{YoutubeKey: e.Trailer.Key, Name: e.Trailer.Name}
	}
	return out
}

func toAPISeasonDetails(s media.DiscoverSeason) apigen.SeasonDetails {
	return apigen.SeasonDetails{
		SeasonNumber: s.Number,
		Name:         s.Name,
		Overview:     s.Overview,
		PosterPath:   optional(s.PosterPath),
		AirDate:      optional(s.AirDate),
		Episodes: mapSlice(s.Episodes, func(e metadata.Episode) apigen.LookupEpisode {
			return apigen.LookupEpisode{
				Season: e.Season, Episode: e.Number, Title: e.Title, Overview: e.Overview,
				AirDate: optional(e.AirDate), Runtime: optional(e.Runtime),
				StillPath: optional(e.StillPath), VoteAverage: optional(e.VoteAverage),
			}
		}),
	}
}

func (h *handlers) MovieRecommendations(ctx context.Context, _ apigen.MovieRecommendationsRequestObject) (apigen.MovieRecommendationsResponseObject, error) {
	rows, err := h.svc.Media.MovieRecommendations(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.MovieRecommendations200JSONResponse(mapSlice(rows, func(r media.RecommendationRow[metadata.Movie]) apigen.MovieRecommendations {
		return apigen.MovieRecommendations{Because: toAPIRecommendationSource(r.Because), Items: mapSlice(r.Items, toAPILookupMovie)}
	})), nil
}

func (h *handlers) SeriesRecommendations(ctx context.Context, _ apigen.SeriesRecommendationsRequestObject) (apigen.SeriesRecommendationsResponseObject, error) {
	rows, err := h.svc.Media.SeriesRecommendations(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.SeriesRecommendations200JSONResponse(mapSlice(rows, func(r media.RecommendationRow[metadata.Series]) apigen.SeriesRecommendations {
		return apigen.SeriesRecommendations{Because: toAPIRecommendationSource(r.Because), Items: mapSlice(r.Items, toAPILookupSeries)}
	})), nil
}

func toAPIRecommendationSource(s media.RecommendationSource) apigen.RecommendationSource {
	return apigen.RecommendationSource{TMDBID: s.TMDBID, Title: s.Title}
}

func toAPIGenre(g metadata.Genre) apigen.Genre {
	return apigen.Genre{ID: g.ID, Name: g.Name}
}

func (h *handlers) DiscoverLanguages(ctx context.Context, _ apigen.DiscoverLanguagesRequestObject) (apigen.DiscoverLanguagesResponseObject, error) {
	out, err := h.svc.Media.Languages(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.DiscoverLanguages200JSONResponse(mapSlice(out, toAPILanguage)), nil
}

func toAPILanguage(l metadata.Language) apigen.Language {
	return apigen.Language{Code: l.Code, Name: l.Name}
}

// fromAPITitleFilter maps the genre and lang query parameters, which the
// spec has already checked.
func fromAPITitleFilter(genres *apigen.BrowseGenre, lang *apigen.BrowseLanguage) media.TitleFilter {
	return media.TitleFilter{GenreIDs: derefSlice(genres), OriginalLanguage: deref(lang)}
}

// fromAPIBrowse applies the spec's defaults (popular, page 1), which the
// generated server leaves unset.
func fromAPIBrowse(filter media.TitleFilter, sort *apigen.DiscoverSort, page *int) media.BrowseQuery {
	q := media.BrowseQuery{TitleFilter: filter, Sort: metadata.SortPopular, Page: 1}
	if sort != nil {
		q.Sort = metadata.DiscoverSort(*sort)
	}
	if page != nil {
		q.Page = *page
	}
	return q
}
