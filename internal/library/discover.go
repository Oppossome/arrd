package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// Discover lets the user browse TMDB's popular movies and series, search
// the indexers for one without adding it first, and download a release.
// Downloading adds the title to the library, unmonitored, so the download
// is imported like any other.

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
	tm, err := s.tmdb(ctx)
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
	tm, err := s.tmdb(ctx)
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
	meta, err := s.tmdbMovie(ctx, tmdbID)
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
	meta, err := s.tmdbSeries(ctx, tmdbID)
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

// SearchDiscoveredMovie searches the indexers for a TMDB movie. A movie in
// the library is searched as such.
func (s *Service) SearchDiscoveredMovie(ctx context.Context, tmdbID int64) (SearchResult, error) {
	if row, err := s.store.GetMovieByTMDBID(ctx, tmdbID); err == nil {
		return s.searchMovie(ctx, row)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SearchResult{}, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.tmdbMovie(ctx, tmdbID)
	if err != nil {
		return SearchResult{}, err
	}
	return s.searchMovie(ctx, db.Movie{
		TMDBID: meta.TMDBID, IMDBID: optString(meta.IMDBID), Title: meta.Title,
		OriginalTitle: meta.OriginalTitle, Year: int64(meta.Year),
	})
}

// SearchDiscoveredSeason searches the indexers for a season of a TMDB
// series. A series in the library is searched as such (episodes that
// already have a file are not wanted); otherwise every episode is.
func (s *Service) SearchDiscoveredSeason(ctx context.Context, tmdbID int64, season int) (SearchResult, error) {
	if row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID); err == nil {
		return s.SearchSeason(ctx, row.ID, season)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SearchResult{}, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.tmdbSeries(ctx, tmdbID)
	if err != nil {
		return SearchResult{}, err
	}
	count := -1
	for _, ss := range meta.Seasons {
		if ss.Number == season {
			count = ss.EpisodeCount
		}
	}
	if count <= 0 {
		return SearchResult{}, fmt.Errorf("series %d season %d: %w", tmdbID, season, ErrNotFound)
	}
	want := release.EpisodeWant{Season: season, AllowPack: true, PackEpisodes: count}
	for e := 1; e <= count; e++ {
		want.Episodes = append(want.Episodes, e)
	}
	var tvdb *int64
	if meta.TVDBID != 0 {
		tvdb = &meta.TVDBID
	}
	return s.searchSeason(ctx, db.Series{
		TMDBID: meta.TMDBID, TVDBID: tvdb, Title: meta.Title,
		OriginalTitle: meta.OriginalTitle, Year: int64(meta.Year),
	}, want)
}

// GrabDiscoveredMovie downloads r for a TMDB movie, first adding the movie
// to the library (unmonitored, in the default root folder) if needed.
func (s *Service) GrabDiscoveredMovie(ctx context.Context, tmdbID int64, r ReleaseResult) (Download, error) {
	id, err := s.ensureMovie(ctx, tmdbID)
	if err != nil {
		return Download{}, err
	}
	return s.GrabMovieRelease(ctx, id, r)
}

// GrabDiscoveredSeries downloads a TV release for a TMDB series, first
// adding the series to the library (unmonitored, in the default root
// folder) if needed.
func (s *Service) GrabDiscoveredSeries(ctx context.Context, tmdbID int64, r ReleaseResult) (Download, error) {
	id, err := s.ensureSeries(ctx, tmdbID)
	if err != nil {
		return Download{}, err
	}
	return s.GrabSeriesRelease(ctx, id, r)
}

// ensureMovie returns the library id of a TMDB movie, adding it unmonitored.
func (s *Service) ensureMovie(ctx context.Context, tmdbID int64) (int64, error) {
	row, err := s.store.GetMovieByTMDBID(ctx, tmdbID)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("check library: %w", err)
	}
	monitored := false
	m, err := s.AddMovie(ctx, AddMovieInput{TMDBID: tmdbID, Monitored: &monitored})
	var ce *ConflictError
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

// ensureSeries returns the library id of a TMDB series, adding it unmonitored.
func (s *Service) ensureSeries(ctx context.Context, tmdbID int64) (int64, error) {
	row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("check library: %w", err)
	}
	sr, err := s.AddSeries(ctx, AddSeriesInput{TMDBID: tmdbID, Monitor: MonitorNone})
	var ce *ConflictError
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

func (s *Service) tmdbMovie(ctx context.Context, tmdbID int64) (*metadata.Movie, error) {
	tm, err := s.tmdb(ctx)
	if err != nil {
		return nil, err
	}
	m, err := tm.GetMovie(ctx, tmdbID)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("movie with TMDB id %d: %w", tmdbID, ErrNotFound)
	}
	return m, err
}

func (s *Service) tmdbSeries(ctx context.Context, tmdbID int64) (*metadata.Series, error) {
	tm, err := s.tmdb(ctx)
	if err != nil {
		return nil, err
	}
	sr, err := tm.GetSeries(ctx, tmdbID)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("series with TMDB id %d: %w", tmdbID, ErrNotFound)
	}
	return sr, err
}
