package acquire

// Searching and grabbing titles from TMDB that may not be in the library yet.
// Grabbing adds the title to the library, unmonitored, so the download is
// imported like any other.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// SearchDiscoveredMovie searches the indexers for a TMDB movie. A movie in
// the library is searched as such.
func (s *Service) SearchDiscoveredMovie(ctx context.Context, tmdbID int64) (SearchResult, error) {
	if row, err := s.store.GetMovieByTMDBID(ctx, tmdbID); err == nil {
		return s.searchMovie(ctx, row)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SearchResult{}, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.media.TMDBMovie(ctx, tmdbID)
	if err != nil {
		return SearchResult{}, err
	}
	return s.searchMovie(ctx, db.Movie{
		TMDBID: meta.TMDBID, IMDBID: media.OptString(meta.IMDBID), Title: meta.Title,
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
	meta, err := s.media.TMDBSeries(ctx, tmdbID)
	if err != nil {
		return SearchResult{}, err
	}
	count := seasonEpisodeCount(meta, season)
	if count <= 0 {
		return SearchResult{}, fmt.Errorf("series %d season %d: %w", tmdbID, season, apperr.ErrNotFound)
	}
	want := release.EpisodeWant{Season: season, AllowPack: true, PackEpisodes: count}
	for e := 1; e <= count; e++ {
		want.Episodes = append(want.Episodes, e)
	}
	return s.searchSeason(ctx, discoveredSeriesRow(meta), want)
}

// SearchDiscoveredEpisode searches the indexers for one episode of a TMDB
// series. Season packs are rejected. A series in the library is searched as
// such.
func (s *Service) SearchDiscoveredEpisode(ctx context.Context, tmdbID int64, season, episode int) (SearchResult, error) {
	if season < 0 || episode < 1 {
		return SearchResult{}, apperr.Invalid("episode", "season must be >= 0 and episode >= 1")
	}
	if row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID); err == nil {
		return s.searchEpisode(ctx, row, season, episode)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SearchResult{}, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.media.TMDBSeries(ctx, tmdbID)
	if err != nil {
		return SearchResult{}, err
	}
	if episode > seasonEpisodeCount(meta, season) {
		return SearchResult{}, fmt.Errorf("series %d S%02dE%02d: %w", tmdbID, season, episode, apperr.ErrNotFound)
	}
	return s.searchEpisode(ctx, discoveredSeriesRow(meta), season, episode)
}

// GrabDiscoveredMovie downloads r for a TMDB movie, first adding the movie
// to the library (unmonitored, in the default root folder) if needed.
func (s *Service) GrabDiscoveredMovie(ctx context.Context, tmdbID int64, r ReleaseResult) (Download, error) {
	id, err := s.media.EnsureMovie(ctx, tmdbID)
	if err != nil {
		return Download{}, err
	}
	return s.GrabMovieRelease(ctx, id, r)
}

// GrabDiscoveredSeries downloads a TV release for a TMDB series, first
// adding the series to the library (unmonitored, in the default root
// folder) if needed.
func (s *Service) GrabDiscoveredSeries(ctx context.Context, tmdbID int64, r ReleaseResult) (Download, error) {
	id, err := s.media.EnsureSeries(ctx, tmdbID)
	if err != nil {
		return Download{}, err
	}
	return s.GrabSeriesRelease(ctx, id, r)
}

// seasonEpisodeCount is how many episodes TMDB lists for season; -1 if the
// season is unknown.
func seasonEpisodeCount(meta *metadata.Series, season int) int {
	for _, ss := range meta.Seasons {
		if ss.Number == season {
			return ss.EpisodeCount
		}
	}
	return -1
}

// discoveredSeriesRow is a series that is not in the library, shaped like a
// library row so it can be searched for.
func discoveredSeriesRow(meta *metadata.Series) db.Series {
	var tvdb *int64
	if meta.TVDBID != 0 {
		tvdb = &meta.TVDBID
	}
	return db.Series{
		TMDBID: meta.TMDBID, TVDBID: tvdb, Title: meta.Title,
		OriginalTitle: meta.OriginalTitle, Year: int64(meta.Year),
	}
}

// SearchDiscoveredSeriesPacks searches the indexers for whole-series packs
// of a TMDB series. A series in the library is searched as such.
func (s *Service) SearchDiscoveredSeriesPacks(ctx context.Context, tmdbID int64) (SearchResult, error) {
	if row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID); err == nil {
		return s.SearchSeriesPacks(ctx, row.ID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SearchResult{}, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.media.TMDBSeries(ctx, tmdbID)
	if err != nil {
		return SearchResult{}, err
	}
	counts := map[int]int{}
	for _, ss := range meta.Seasons {
		if ss.Number > 0 && ss.EpisodeCount > 0 {
			counts[ss.Number] = ss.EpisodeCount
		}
	}
	return s.searchSeriesPacks(ctx, discoveredSeriesRow(meta), release.SeriesPackWant{SeasonEpisodes: counts})
}
