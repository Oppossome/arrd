package acquire

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/torrent"
)

// fileImport is one file to place in the library, and the movie or
// episodes it holds.
type fileImport struct {
	src, dst   string
	size       int64
	movieID    int64   // a movie file
	episodeIDs []int64 // an episode file (several for multi-episode files)
}

// importDownload places the files of a completed download in the library:
// each file is matched to the movies or episodes the download is expected
// to contain, then everything is transferred and recorded together.
func (s *Service) importDownload(ctx context.Context, eng *torrent.Engine, r db.Download) error {
	movies, err := s.store.ListDownloadItemMovies(ctx, r.ID)
	if err != nil {
		return fmt.Errorf("list download movies: %w", err)
	}
	episodes, err := s.store.ListDownloadItemEpisodes(ctx, r.ID)
	if err != nil {
		return fmt.Errorf("list download episodes: %w", err)
	}
	if len(movies) == 0 && len(episodes) == 0 {
		return s.failDownload(ctx, r.ID, errors.New("the movie or series was removed from the library; nothing to import into"))
	}
	dl, err := s.completedTorrent(ctx, eng, r)
	if err != nil {
		return err
	}
	files := dl.Files()
	candidates := make([]importer.File, len(files))
	for i, f := range files {
		candidates[i] = importer.File{Path: f.Path, Size: f.Size}
	}

	var plan []fileImport
	var skipped []string
	if len(movies) > 0 {
		plan, skipped, err = s.planMovies(ctx, r, candidates, movies)
	} else {
		plan, skipped, err = s.planEpisodes(ctx, r, candidates, episodes)
	}
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		return fmt.Errorf("no importable files (%s)", strings.Join(skipped, "; "))
	}
	if err := s.executeImport(ctx, dl, r, plan); err != nil {
		return err
	}
	if missing := missingItems(plan, movies, episodes); len(missing) > 0 {
		s.setDownloadError(ctx, r.ID, fmt.Errorf("imported, but these expected items were not in the download: %s", strings.Join(missing, ", ")))
	}
	return nil
}

// executeImport transfers every planned file, then records them together
// so a failure part-way leaves the download "completed" and the whole
// import is retried (Transfer is idempotent for files already placed).
func (s *Service) executeImport(ctx context.Context, dl *torrent.Download, r db.Download, plan []fileImport) error {
	conf, err := s.settings.DownloadSettings(ctx)
	if err != nil {
		return err
	}
	mode := conf.ImportMode
	if mode == importer.ModeMove {
		// Moving files out from under the engine would break seeding.
		rmCtx, cancel := context.WithTimeout(ctx, removeTimeout)
		defer cancel()
		if err := dl.Remove(rmCtx); err != nil {
			return err
		}
	}
	for _, p := range plan {
		res, err := importer.Transfer(p.src, p.dst, mode)
		if err != nil {
			return err
		}
		if res.Method != mode {
			s.log.Warn().Str("requested", string(mode)).Str("used", string(res.Method)).Str("file", p.dst).Msg("import used a different method")
		}
	}
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		for _, p := range plan {
			dst := p.dst
			if p.movieID != 0 {
				if err := q.SetMovieFile(ctx, db.SetMovieFileParams{ID: p.movieID, FilePath: &dst, FileQuality: &r.Quality}); err != nil {
					return err
				}
			}
			for _, id := range p.episodeIDs {
				if err := q.SetEpisodeFile(ctx, db.SetEpisodeFileParams{ID: id, FilePath: &dst, FileQuality: &r.Quality}); err != nil {
					return err
				}
			}
		}
		return q.MarkDownloadImported(ctx, r.ID)
	})
	if err != nil {
		return fmt.Errorf("record import: %w", err)
	}
	s.log.Info().Str("release", r.ReleaseTitle).Int("files", len(plan)).Int64("download", r.ID).Str("method", string(mode)).Msg("imported download")
	s.notifier.LibraryChanged()
	return nil
}

// missingItems names the expected items without a file that the import
// did not bring in. A single movie is never "missing": its import either
// placed it or failed.
func missingItems(plan []fileImport, movies []db.Movie, episodes []db.Episode) []string {
	imported := map[downloadItem]bool{}
	for _, p := range plan {
		if p.movieID != 0 {
			imported[downloadItem{movie: true, id: p.movieID}] = true
		}
		for _, id := range p.episodeIDs {
			imported[downloadItem{id: id}] = true
		}
	}
	var missing []string
	if len(movies) > 1 {
		for _, m := range movies {
			if !imported[downloadItem{movie: true, id: m.ID}] && m.FilePath == nil {
				missing = append(missing, movieLabel(m))
			}
		}
	}
	for _, e := range episodes {
		if !imported[downloadItem{id: e.ID}] && e.FilePath == nil {
			missing = append(missing, fmt.Sprintf("S%02dE%02d", e.Season, e.Episode))
		}
	}
	return missing
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
