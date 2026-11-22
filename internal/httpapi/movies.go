package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/metadata"
)

func (h *handlers) LookupMovies(ctx context.Context, req apigen.LookupMoviesRequestObject) (apigen.LookupMoviesResponseObject, error) {
	out, err := h.svc.LookupMovies(ctx, deref(req.Params.Q), deref(req.Params.Year))
	if err != nil {
		return nil, err
	}
	return apigen.LookupMovies200JSONResponse(mapSlice(out, toAPIDiscoverMovie)), nil
}

func (h *handlers) ListMovies(ctx context.Context, _ apigen.ListMoviesRequestObject) (apigen.ListMoviesResponseObject, error) {
	out, err := h.svc.Movies(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.ListMovies200JSONResponse(mapSlice(out, toAPIMovie)), nil
}

func (h *handlers) AddMovie(ctx context.Context, req apigen.AddMovieRequestObject) (apigen.AddMovieResponseObject, error) {
	out, err := h.svc.AddMovie(ctx, fromAPIAddMovie(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.AddMovie201JSONResponse(toAPIMovie(out)), nil
}

func (h *handlers) GetMovie(ctx context.Context, req apigen.GetMovieRequestObject) (apigen.GetMovieResponseObject, error) {
	out, err := h.svc.Movie(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.GetMovie200JSONResponse(toAPIMovie(out)), nil
}

func (h *handlers) UpdateMovie(ctx context.Context, req apigen.UpdateMovieRequestObject) (apigen.UpdateMovieResponseObject, error) {
	out, err := h.svc.UpdateMovie(ctx, req.ID, fromAPIUpdateMovie(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.UpdateMovie200JSONResponse(toAPIMovie(out)), nil
}

func (h *handlers) DeleteMovie(ctx context.Context, req apigen.DeleteMovieRequestObject) (apigen.DeleteMovieResponseObject, error) {
	if err := h.svc.DeleteMovie(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteMovie204Response{}, nil
}

func (h *handlers) MovieReleases(ctx context.Context, req apigen.MovieReleasesRequestObject) (apigen.MovieReleasesResponseObject, error) {
	out, err := h.svc.SearchMovie(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.MovieReleases200JSONResponse(toAPISearchResult(out)), nil
}

func (h *handlers) SearchMovie(ctx context.Context, req apigen.SearchMovieRequestObject) (apigen.SearchMovieResponseObject, error) {
	out, err := h.svc.AutoSearchMovie(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.SearchMovie200JSONResponse(toAPIAutoSearchResult(out)), nil
}

func (h *handlers) GrabMovie(ctx context.Context, req apigen.GrabMovieRequestObject) (apigen.GrabMovieResponseObject, error) {
	out, err := h.svc.GrabMovieRelease(ctx, req.ID, fromAPIRelease(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.GrabMovie201JSONResponse(toAPIDownload(out)), nil
}

func toAPIMovie(m library.Movie) apigen.Movie {
	return apigen.Movie{
		ID:            m.ID,
		TMDBID:        m.TMDBID,
		IMDBID:        m.IMDBID,
		Title:         m.Title,
		OriginalTitle: m.OriginalTitle,
		Year:          m.Year,
		Overview:      m.Overview,
		PosterPath:    m.PosterPath,
		BackdropPath:  m.BackdropPath,
		RootFolderID:  m.RootFolderID,
		Monitored:     m.Monitored,
		FilePath:      m.FilePath,
		FileQuality:   m.FileQuality,
		AddedAt:       m.AddedAt,
		Status:        apigen.MovieStatus(m.Status),
	}
}

func toAPILookupMovie(m metadata.Movie) apigen.LookupMovie {
	return apigen.LookupMovie{
		TMDBID:        m.TMDBID,
		IMDBID:        optional(m.IMDBID),
		Title:         m.Title,
		OriginalTitle: m.OriginalTitle,
		Year:          m.Year,
		Overview:      m.Overview,
		PosterPath:    optional(m.PosterPath),
	}
}

func fromAPIAddMovie(in apigen.AddMovieRequest) library.AddMovieInput {
	return library.AddMovieInput{
		TMDBID:       in.TMDBID,
		RootFolderID: in.RootFolderID,
		Monitored:    in.Monitored,
		Search:       deref(in.Search),
	}
}

func fromAPIUpdateMovie(in apigen.UpdateMovieRequest) library.UpdateMovieInput {
	return library.UpdateMovieInput{Monitored: in.Monitored, RootFolderID: in.RootFolderID}
}
