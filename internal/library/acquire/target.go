package acquire

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

// Kind is what a release is searched for or grabbed for.
type Kind string

const (
	KindMovie      Kind = "movie"
	KindCollection Kind = "collection" // multi-movie packs of a movie's collection
	KindSeries     Kind = "series"     // whole-series packs
	KindSeason     Kind = "season"
	KindEpisode    Kind = "episode"
)

// Target names what a release is wanted for, by TMDB id, whether or not
// the title is in the library: a title in the library is searched as such.
type Target struct {
	Kind Kind `json:"kind"`
	// TMDBID is the movie's (for a collection, any movie in it) or the
	// series'.
	TMDBID  int64 `json:"tmdb_id"`
	Season  *int  `json:"season,omitempty"`  // season and episode targets
	Episode *int  `json:"episode,omitempty"` // episode targets
}

// MovieTarget, SeasonTarget and the like build targets.
func MovieTarget(tmdbID int64) Target      { return Target{Kind: KindMovie, TMDBID: tmdbID} }
func CollectionTarget(tmdbID int64) Target { return Target{Kind: KindCollection, TMDBID: tmdbID} }
func SeriesTarget(tmdbID int64) Target     { return Target{Kind: KindSeries, TMDBID: tmdbID} }
func SeasonTarget(tmdbID int64, season int) Target {
	return Target{Kind: KindSeason, TMDBID: tmdbID, Season: &season}
}
func EpisodeTarget(tmdbID int64, season, episode int) Target {
	return Target{Kind: KindEpisode, TMDBID: tmdbID, Season: &season, Episode: &episode}
}

// Validate checks that t has what its kind needs.
func (t Target) Validate() error {
	if t.TMDBID <= 0 {
		return apperr.Invalid("tmdb_id", "is required")
	}
	switch t.Kind {
	case KindMovie, KindCollection, KindSeries:
		if t.Season != nil || t.Episode != nil {
			return apperr.Invalid("season", "only season and episode targets take a season or episode")
		}
	case KindSeason:
		if t.Season == nil || *t.Season < 0 {
			return apperr.Invalid("season", "a season >= 0 is required")
		}
		if t.Episode != nil {
			return apperr.Invalid("episode", "only episode targets take an episode")
		}
	case KindEpisode:
		if t.Season == nil || *t.Season < 0 || t.Episode == nil || *t.Episode < 1 {
			return apperr.Invalid("episode", "a season >= 0 and an episode >= 1 are required")
		}
	default:
		return apperr.Invalid("kind", "must be movie, collection, series, season or episode")
	}
	return nil
}

func (t Target) isMovie() bool { return t.Kind == KindMovie || t.Kind == KindCollection }

// resolve builds what t wants: from the library when the title is there,
// else from TMDB.
func (s *Service) resolve(ctx context.Context, t Target) (wanted, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	switch t.Kind {
	case KindMovie:
		movie, err := s.movieRow(ctx, t.TMDBID)
		if err != nil {
			return nil, err
		}
		return movieWant{s: s, movie: movie}, nil
	case KindCollection:
		movie, err := s.movieRow(ctx, t.TMDBID)
		if err != nil {
			return nil, err
		}
		coll, err := s.movieCollection(ctx, t.TMDBID)
		if err != nil {
			return nil, err
		}
		return collectionWant{s: s, anchor: movie, coll: coll}, nil
	}

	series, meta, err := s.seriesRow(ctx, t.TMDBID)
	if err != nil {
		return nil, err
	}
	switch t.Kind {
	case KindSeries:
		return s.seriesWant(ctx, series, meta)
	case KindSeason:
		return s.seasonWant(ctx, series, meta, *t.Season)
	default:
		return s.episodeWant(ctx, series, meta, *t.Season, *t.Episode)
	}
}

// movieRow is the library's row for a TMDB movie, or one built from TMDB
// (ID 0) when it is not in the library.
func (s *Service) movieRow(ctx context.Context, tmdbID int64) (db.Movie, error) {
	row, err := s.store.GetMovieByTMDBID(ctx, tmdbID)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return db.Movie{}, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.media.TMDBMovie(ctx, tmdbID)
	if err != nil {
		return db.Movie{}, err
	}
	return db.Movie{
		TMDBID: meta.TMDBID, IMDBID: media.OptString(meta.IMDBID), Title: meta.Title,
		OriginalTitle: meta.OriginalTitle, Year: int64(meta.Year),
	}, nil
}

// seriesRow is the library's row for a TMDB series (meta nil), or one
// built from TMDB (ID 0) along with TMDB's view of it.
func (s *Service) seriesRow(ctx context.Context, tmdbID int64) (db.Series, *metadata.Series, error) {
	row, err := s.store.GetSeriesByTMDBID(ctx, tmdbID)
	if err == nil {
		return row, nil, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return db.Series{}, nil, fmt.Errorf("check library: %w", err)
	}
	meta, err := s.media.TMDBSeries(ctx, tmdbID)
	if err != nil {
		return db.Series{}, nil, err
	}
	var tvdb *int64
	if meta.TVDBID != 0 {
		tvdb = &meta.TVDBID
	}
	return db.Series{
		TMDBID: meta.TMDBID, TVDBID: tvdb, Title: meta.Title,
		OriginalTitle: meta.OriginalTitle, Year: int64(meta.Year),
	}, meta, nil
}

// libraryWant resolves t from the library only: the title must be there.
func (s *Service) libraryWant(ctx context.Context, t Target) (wanted, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	var err error
	if t.isMovie() {
		_, err = s.store.GetMovieByTMDBID(ctx, t.TMDBID)
	} else {
		_, err = s.store.GetSeriesByTMDBID(ctx, t.TMDBID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("TMDB id %d is not in the library: %w", t.TMDBID, apperr.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("check library: %w", err)
	}
	return s.resolve(ctx, t)
}

// ensureInLibrary adds t's movie or series to the library (unmonitored, in
// the default root folder) if it is not there yet.
func (s *Service) ensureInLibrary(ctx context.Context, t Target) error {
	if err := t.Validate(); err != nil {
		return err
	}
	var err error
	if t.isMovie() {
		_, err = s.media.EnsureMovie(ctx, t.TMDBID)
	} else {
		_, err = s.media.EnsureSeries(ctx, t.TMDBID)
	}
	return err
}

// parsed is a release name's parse, for claim.
func parsed(r ReleaseResult) release.Info { return release.Parse(r.Title) }
