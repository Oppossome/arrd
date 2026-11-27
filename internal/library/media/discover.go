package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/metadata"
)

// Discover lets the user browse TMDB's popular movies and series, marking
// those already in the library.

// DiscoverMovie is a TMDB movie and, if it is in the library, its id there.
type DiscoverMovie struct {
	metadata.Movie
	LibraryID *int64 `json:"library_id,omitempty"`
}

// DiscoverSeries is a TMDB series and, if it is in the library, its id there.
type DiscoverSeries struct {
	metadata.Series
	LibraryID *int64 `json:"library_id,omitempty"`
}

// TMDBMovieList returns one of TMDB's curated movie lists (trending, popular, ...).
func (s *Service) TMDBMovieList(ctx context.Context, list metadata.MovieList) ([]DiscoverMovie, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	movies, err := tm.MovieList(ctx, list)
	if err != nil {
		return nil, err
	}
	return s.annotateMovies(ctx, movies)
}

// annotateMovies adds the library id of each TMDB movie that is in the library.
func (s *Service) annotateMovies(ctx context.Context, movies []metadata.Movie) ([]DiscoverMovie, error) {
	rows, err := s.store.ListMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	inLibrary := map[int64]int64{}
	for _, r := range rows {
		inLibrary[r.TMDBID] = r.ID
	}
	out := make([]DiscoverMovie, len(movies))
	for i, m := range movies {
		out[i] = DiscoverMovie{Movie: m, LibraryID: lookupID(inLibrary, m.TMDBID)}
	}
	return out, nil
}

// TMDBSeriesList returns one of TMDB's curated TV lists (trending, popular, ...).
func (s *Service) TMDBSeriesList(ctx context.Context, list metadata.SeriesList) ([]DiscoverSeries, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	series, err := tm.SeriesList(ctx, list)
	if err != nil {
		return nil, err
	}
	return s.annotateSeries(ctx, series)
}

// annotateSeries adds the library id of each TMDB series that is in the library.
func (s *Service) annotateSeries(ctx context.Context, series []metadata.Series) ([]DiscoverSeries, error) {
	rows, err := s.store.ListSeries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	inLibrary := map[int64]int64{}
	for _, r := range rows {
		inLibrary[r.TMDBID] = r.ID
	}
	out := make([]DiscoverSeries, len(series))
	for i, sr := range series {
		out[i] = DiscoverSeries{Series: sr, LibraryID: lookupID(inLibrary, sr.TMDBID)}
	}
	return out, nil
}

func lookupID(ids map[int64]int64, tmdbID int64) *int64 {
	if id, ok := ids[tmdbID]; ok {
		return &id
	}
	return nil
}

// DiscoveredMovie returns a movie from TMDB.
func (s *Service) DiscoveredMovie(ctx context.Context, tmdbID int64) (DiscoverMovie, error) {
	meta, err := s.TMDBMovie(ctx, tmdbID)
	if err != nil {
		return DiscoverMovie{}, err
	}
	out := DiscoverMovie{Movie: *meta}
	row, err := s.store.GetMovieByTMDBID(ctx, tmdbID)
	switch {
	case err == nil:
		out.LibraryID = &row.ID
	case !errors.Is(err, sql.ErrNoRows):
		return DiscoverMovie{}, fmt.Errorf("check library: %w", err)
	}
	return out, nil
}

// DiscoveredSeries returns a series from TMDB, with its seasons.
func (s *Service) DiscoveredSeries(ctx context.Context, tmdbID int64) (DiscoverSeries, error) {
	meta, err := s.TMDBSeries(ctx, tmdbID)
	if err != nil {
		return DiscoverSeries{}, err
	}
	out := DiscoverSeries{Series: *meta}
	row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID)
	switch {
	case err == nil:
		out.LibraryID = &row.ID
	case !errors.Is(err, sql.ErrNoRows):
		return DiscoverSeries{}, fmt.Errorf("check library: %w", err)
	}
	return out, nil
}

// DiscoverSeason is a TMDB season with its episodes.
type DiscoverSeason = metadata.Season

// DiscoveredSeason returns one season of a TMDB series with its episodes.
func (s *Service) DiscoveredSeason(ctx context.Context, tmdbID int64, season int) (DiscoverSeason, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return DiscoverSeason{}, err
	}
	out, err := tm.GetSeasonDetails(ctx, tmdbID, season)
	if errors.Is(err, metadata.ErrNotFound) {
		return DiscoverSeason{}, fmt.Errorf("series with TMDB id %d season %d: %w", tmdbID, season, apperr.ErrNotFound)
	}
	if err != nil {
		return DiscoverSeason{}, err
	}
	return *out, nil
}

// EnsureMovie returns the library id of a TMDB movie, adding it unmonitored
// in the default movie root folder.
func (s *Service) EnsureMovie(ctx context.Context, tmdbID int64) (int64, error) {
	return s.EnsureMovieIn(ctx, tmdbID, nil)
}

// EnsureMovieIn returns the library id of a TMDB movie, adding it
// unmonitored in rootFolderID (nil: the default movie root folder). A movie
// already in the library stays where it is.
func (s *Service) EnsureMovieIn(ctx context.Context, tmdbID int64, rootFolderID *int64) (int64, error) {
	row, err := s.store.GetMovieByTMDBID(ctx, tmdbID)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("check library: %w", err)
	}
	monitored := false
	m, err := s.AddMovie(ctx, AddMovieInput{TMDBID: tmdbID, RootFolderID: rootFolderID, Monitored: &monitored})
	var ce *apperr.ConflictError
	if errors.As(err, &ce) {
		// Added concurrently (e.g. a double click): use that one.
		if row, err := s.store.GetMovieByTMDBID(ctx, tmdbID); err == nil {
			return row.ID, nil
		}
	}
	if err != nil {
		return 0, err
	}
	return m.ID, nil
}

// EnsureSeries returns the library id of a TMDB series, adding it unmonitored.
func (s *Service) EnsureSeries(ctx context.Context, tmdbID int64) (int64, error) {
	row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("check library: %w", err)
	}
	sr, err := s.AddSeries(ctx, AddSeriesInput{TMDBID: tmdbID, Monitor: MonitorNone})
	var ce *apperr.ConflictError
	if errors.As(err, &ce) {
		if row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID); err == nil {
			return row.ID, nil
		}
	}
	if err != nil {
		return 0, err
	}
	return sr.ID, nil
}

// TMDBMovie fetches a movie from TMDB.
func (s *Service) TMDBMovie(ctx context.Context, tmdbID int64) (*metadata.Movie, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	m, err := tm.GetMovie(ctx, tmdbID)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("movie with TMDB id %d: %w", tmdbID, apperr.ErrNotFound)
	}
	return m, err
}

// TMDBCollection fetches a movie collection (franchise) from TMDB.
func (s *Service) TMDBCollection(ctx context.Context, id int64) (*metadata.Collection, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	c, err := tm.GetCollection(ctx, id)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("collection with TMDB id %d: %w", id, apperr.ErrNotFound)
	}
	return c, err
}

// TMDBSeries fetches a series from TMDB.
func (s *Service) TMDBSeries(ctx context.Context, tmdbID int64) (*metadata.Series, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	sr, err := tm.GetSeries(ctx, tmdbID)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("series with TMDB id %d: %w", tmdbID, apperr.ErrNotFound)
	}
	return sr, err
}
