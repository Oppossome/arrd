package acquire

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/torrent"
)

type episodeKey struct{ season, episode int }

// episodeImport is one file to place in the library.
type episodeImport struct {
	src, dst string
	episodes []db.Episode
}

// importSeriesDownload maps each video file of a completed TV download to
// episodes by its name, and places them in the series' season folders.
// Episodes that already have a file are left alone (no upgrades yet).
func (s *Service) importSeriesDownload(ctx context.Context, eng *torrent.Engine, r db.Download) error {
	dl, err := s.completedTorrent(ctx, eng, r)
	if err != nil {
		return err
	}
	series, err := s.store.GetSeries(ctx, *r.SeriesID)
	if err != nil {
		return apperr.NotFound(err, "series", *r.SeriesID)
	}
	root, err := s.store.GetRootFolder(ctx, series.RootFolderID)
	if err != nil {
		return apperr.NotFound(err, "root folder", series.RootFolderID)
	}
	conf, err := s.settings.DownloadSettings(ctx)
	if err != nil {
		return err
	}
	all, err := s.store.ListEpisodes(ctx, series.ID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}
	byKey := make(map[episodeKey]db.Episode, len(all))
	for _, e := range all {
		byKey[episodeKey{int(e.Season), int(e.Episode)}] = e
	}

	files := dl.Files()
	candidates := make([]importer.File, len(files))
	for i, f := range files {
		candidates[i] = importer.File{Path: f.Path, Size: f.Size}
	}
	videos := importer.VideoFiles(candidates)
	releaseInfo := release.Parse(r.ReleaseTitle)

	var plan []episodeImport
	var skipped []string
	for _, f := range videos {
		info := episodeInfoForFile(f.Path, releaseInfo, len(videos) == 1)
		if !info.IsTV || info.SeasonPack || len(info.Episodes) == 0 {
			skipped = append(skipped, fmt.Sprintf("%s: no episode number in name", f.Path))
			continue
		}
		var eps []db.Episode
		for _, n := range info.Episodes {
			e, ok := byKey[episodeKey{info.Season, n}]
			if !ok {
				skipped = append(skipped, fmt.Sprintf("%s: S%02dE%02d is not a known episode", f.Path, info.Season, n))
				eps = nil
				break
			}
			eps = append(eps, e)
		}
		if eps == nil || allHaveFiles(eps) {
			continue
		}
		plan = append(plan, episodeImport{
			src: filepath.Join(r.CompleteDir, f.Path),
			dst: filepath.Join(root.Path,
				importer.SeriesFolder(series.Title, int(series.Year)),
				importer.SeasonFolder(info.Season),
				importer.EpisodeFileName(series.Title, info.Season, info.Episodes, eps[0].Title, r.Quality, filepath.Ext(f.Path))),
			episodes: eps,
		})
	}
	if len(plan) == 0 {
		return fmt.Errorf("no importable episode files (%s)", strings.Join(skipped, "; "))
	}

	mode := conf.ImportMode
	if mode == importer.ModeMove {
		rmCtx, cancel := context.WithTimeout(ctx, removeTimeout)
		defer cancel()
		if err := dl.Remove(rmCtx); err != nil {
			return err
		}
	}
	// Place every file first; record them together so a failure part-way
	// leaves the download "completed" and the whole import is retried
	// (Transfer is idempotent for files already placed).
	for _, p := range plan {
		res, err := importer.Transfer(p.src, p.dst, mode)
		if err != nil {
			return err
		}
		if res.Method != mode {
			s.log.Warn().Str("requested", string(mode)).Str("used", string(res.Method)).Str("file", p.dst).Msg("import used a different method")
		}
	}
	imported := map[int64]bool{}
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		for _, p := range plan {
			for _, e := range p.episodes {
				dst := p.dst
				if err := q.SetEpisodeFile(ctx, db.SetEpisodeFileParams{ID: e.ID, FilePath: &dst, FileQuality: &r.Quality}); err != nil {
					return err
				}
				imported[e.ID] = true
			}
		}
		return q.MarkDownloadImported(ctx, r.ID)
	})
	if err != nil {
		return fmt.Errorf("record import: %w", err)
	}
	s.log.Info().Str("series", series.Title).Int("files", len(plan)).Int64("download", r.ID).Msg("imported episodes")
	s.notifier.LibraryChanged()

	// Report expected episodes the download turned out not to contain.
	expected, err := s.store.ListDownloadEpisodes(ctx, r.ID)
	if err != nil {
		return nil // import succeeded; the report is best-effort
	}
	var missing []string
	for _, e := range expected {
		if !imported[e.ID] && e.FilePath == nil {
			missing = append(missing, fmt.Sprintf("S%02dE%02d", e.Season, e.Episode))
		}
	}
	if len(missing) > 0 {
		s.setDownloadError(ctx, r.ID, fmt.Errorf("imported, but these expected episodes were not in the download: %s", strings.Join(missing, ", ")))
	}
	return nil
}

// episodeInfoForFile finds season/episode numbers for a file: from its
// name, then its folders, then (for a single-file download) the release.
func episodeInfoForFile(path string, releaseInfo release.Info, onlyFile bool) release.Info {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		info := release.Parse(parts[i])
		if info.IsTV && !info.SeasonPack && len(info.Episodes) > 0 {
			return info
		}
	}
	if onlyFile {
		return releaseInfo
	}
	return release.Info{}
}

func allHaveFiles(eps []db.Episode) bool {
	for _, e := range eps {
		if e.FilePath == nil {
			return false
		}
	}
	return true
}
