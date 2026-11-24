package acquire

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/torrent"
)

// importDownload imports a completed download into the item it was grabbed
// for.
func (s *Service) importDownload(ctx context.Context, eng *torrent.Engine, r db.Download) error {
	switch {
	case r.MovieID != nil:
		return s.importMovieDownload(ctx, eng, r)
	case r.SeriesID != nil:
		return s.importSeriesDownload(ctx, eng, r)
	default:
		return s.failDownload(ctx, r.ID, errors.New("the movie or series was removed from the library; nothing to import into"))
	}
}

// completedTorrent returns the engine download for a completed row,
// re-adding it if needed.
func (s *Service) completedTorrent(ctx context.Context, eng *torrent.Engine, r db.Download) (*torrent.Download, error) {
	if err := s.ensureInEngine(ctx, eng, r); err != nil {
		return nil, fmt.Errorf("load torrent: %w", err)
	}
	dl, ok := eng.Get(r.InfoHash)
	if !ok || !dl.HaveInfo() {
		return nil, errors.New("torrent metadata is not available")
	}
	return dl, nil
}

// importMovieDownload places the main video file of a completed download in
// the movie's library folder and records it.
func (s *Service) importMovieDownload(ctx context.Context, eng *torrent.Engine, r db.Download) error {
	dl, err := s.completedTorrent(ctx, eng, r)
	if err != nil {
		return err
	}
	movie, err := s.store.GetMovie(ctx, *r.MovieID)
	if err != nil {
		return apperr.NotFound(err, "movie", *r.MovieID)
	}
	root, err := s.store.GetRootFolder(ctx, movie.RootFolderID)
	if err != nil {
		return apperr.NotFound(err, "root folder", movie.RootFolderID)
	}
	conf, err := s.settings.DownloadSettings(ctx)
	if err != nil {
		return err
	}

	files := dl.Files()
	candidates := make([]importer.File, len(files))
	for i, f := range files {
		candidates[i] = importer.File{Path: f.Path, Size: f.Size}
	}
	pick, err := importer.PickMovieFile(candidates)
	if err != nil {
		return err
	}
	src := filepath.Join(r.CompleteDir, pick.Path)
	dst := filepath.Join(root.Path,
		importer.MovieFolder(movie.Title, int(movie.Year)),
		importer.MovieFileName(movie.Title, int(movie.Year), r.Quality, filepath.Ext(pick.Path)))

	mode := conf.ImportMode
	if mode == importer.ModeMove {
		// Moving the file out from under the engine would break seeding.
		rmCtx, cancel := context.WithTimeout(ctx, removeTimeout)
		defer cancel()
		if err := dl.Remove(rmCtx); err != nil {
			return err
		}
	}
	res, err := importer.Transfer(src, dst, mode)
	if err != nil {
		return err
	}
	if res.Method != mode {
		s.log.Warn().Str("requested", string(mode)).Str("used", string(res.Method)).Str("file", dst).Msg("import used a different method")
	}
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		if err := q.SetMovieFile(ctx, db.SetMovieFileParams{ID: movie.ID, FilePath: &dst, FileQuality: &r.Quality}); err != nil {
			return err
		}
		return q.MarkDownloadImported(ctx, r.ID)
	})
	if err != nil {
		return fmt.Errorf("record import: %w", err)
	}
	s.log.Info().Int64("movie", movie.ID).Str("title", movie.Title).Str("file", dst).Str("method", string(res.Method)).Msg("imported movie")
	return nil
}
