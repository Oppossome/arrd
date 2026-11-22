package library

import (
	"context"
	"fmt"

	"generic-arr-project/internal/store/db"
)

// BackfillBackdrops fetches TMDB backdrops for movies and series stored
// without one (added before backdrops were kept, or TMDB had none then).
// Items TMDB still has no backdrop for are left empty and retried on the
// next run; per-item failures are logged and do not stop the run.
func (s *Service) BackfillBackdrops(ctx context.Context) error {
	movies, err := s.store.ListMoviesMissingBackdrop(ctx)
	if err != nil {
		return fmt.Errorf("list movies without backdrop: %w", err)
	}
	series, err := s.store.ListSeriesMissingBackdrop(ctx)
	if err != nil {
		return fmt.Errorf("list series without backdrop: %w", err)
	}
	if len(movies) == 0 && len(series) == 0 {
		return nil
	}
	var failed int
	for _, m := range movies {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.backfillMovieBackdrop(ctx, m.ID, m.TMDBID); err != nil {
			failed++
			s.log.Warn("backfill movie backdrop", "movie", m.ID, "title", m.Title, "err", err)
		}
	}
	for _, r := range series {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.backfillSeriesBackdrop(ctx, r.ID, r.TMDBID); err != nil {
			failed++
			s.log.Warn("backfill series backdrop", "series", r.ID, "title", r.Title, "err", err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("backfill backdrops: %d of %d items failed", failed, len(movies)+len(series))
	}
	return nil
}

func (s *Service) backfillMovieBackdrop(ctx context.Context, id, tmdbID int64) error {
	meta, err := s.tmdbMovie(ctx, tmdbID)
	if err != nil {
		return err
	}
	if meta.BackdropPath == "" {
		return nil
	}
	return s.store.SetMovieBackdrop(ctx, db.SetMovieBackdropParams{ID: id, BackdropPath: meta.BackdropPath})
}

func (s *Service) backfillSeriesBackdrop(ctx context.Context, id, tmdbID int64) error {
	meta, err := s.tmdbSeries(ctx, tmdbID)
	if err != nil {
		return err
	}
	if meta.BackdropPath == "" {
		return nil
	}
	return s.store.SetSeriesBackdrop(ctx, db.SetSeriesBackdropParams{ID: id, BackdropPath: meta.BackdropPath})
}
