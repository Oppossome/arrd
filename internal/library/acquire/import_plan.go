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
)

// rootPaths caches root folder paths by id during one import.
type rootPaths struct {
	s     *Service
	paths map[int64]string
}

func (rp *rootPaths) get(ctx context.Context, id int64) (string, error) {
	if p, ok := rp.paths[id]; ok {
		return p, nil
	}
	row, err := rp.s.store.GetRootFolder(ctx, id)
	if err != nil {
		return "", apperr.NotFound(err, "root folder", id)
	}
	if rp.paths == nil {
		rp.paths = map[int64]string{}
	}
	rp.paths[id] = row.Path
	return row.Path, nil
}

// planMovies picks the file for each expected movie. A single movie takes
// the largest video file whatever its name (and replaces any file the
// movie already has). A pack's files are matched to movies by the title
// and year in their names or folders, the largest file winning, and movies
// that already have a file are left alone.
func (s *Service) planMovies(ctx context.Context, r db.Download, files []importer.File, movies []db.Movie) ([]fileImport, []string, error) {
	roots := rootPaths{s: s}
	place := func(m db.Movie, f importer.File) (fileImport, error) {
		root, err := roots.get(ctx, m.RootFolderID)
		if err != nil {
			return fileImport{}, err
		}
		return fileImport{
			src:  filepath.Join(r.CompleteDir, f.Path),
			dst:  filepath.Join(root, importer.MovieFolder(m.Title, int(m.Year)), importer.MovieFileName(m.Title, int(m.Year), r.Quality, filepath.Ext(f.Path))),
			size: f.Size, movieID: m.ID,
		}, nil
	}

	if len(movies) == 1 {
		pick, err := importer.PickMovieFile(files)
		if err != nil {
			return nil, nil, err
		}
		p, err := place(movies[0], pick)
		return []fileImport{p}, nil, err
	}

	byMovie := map[int64]fileImport{}
	var order []int64
	var skipped []string
	for _, f := range importer.MovieFiles(files) {
		movie, ok := movieForFile(f.Path, movies)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s: matches no expected movie", f.Path))
			continue
		}
		if movie.FilePath != nil {
			continue
		}
		prev, seen := byMovie[movie.ID]
		if seen && prev.size >= f.Size {
			continue
		}
		p, err := place(movie, f)
		if err != nil {
			return nil, nil, err
		}
		if !seen {
			order = append(order, movie.ID)
		}
		byMovie[movie.ID] = p
	}
	plan := make([]fileImport, 0, len(order))
	for _, id := range order {
		plan = append(plan, byMovie[id])
	}
	return plan, skipped, nil
}

// planEpisodes maps each video file to episodes by its name, and places it
// in its series' season folder. Episodes that already have a file are left
// alone (no upgrades yet).
func (s *Service) planEpisodes(ctx context.Context, r db.Download, files []importer.File, expected []db.Episode) ([]fileImport, []string, error) {
	seriesID := expected[0].SeriesID
	series, err := s.store.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, nil, apperr.NotFound(err, "series", seriesID)
	}
	roots := rootPaths{s: s}
	root, err := roots.get(ctx, series.RootFolderID)
	if err != nil {
		return nil, nil, err
	}
	all, err := s.store.ListEpisodes(ctx, seriesID)
	if err != nil {
		return nil, nil, fmt.Errorf("list episodes: %w", err)
	}
	type episodeKey struct{ season, episode int }
	byKey := make(map[episodeKey]db.Episode, len(all))
	for _, e := range all {
		byKey[episodeKey{int(e.Season), int(e.Episode)}] = e
	}

	videos := importer.VideoFiles(files)
	releaseInfo := release.Parse(r.ReleaseTitle)
	var plan []fileImport
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
		ids := make([]int64, len(eps))
		for i, e := range eps {
			ids[i] = e.ID
		}
		plan = append(plan, fileImport{
			src: filepath.Join(r.CompleteDir, f.Path),
			dst: filepath.Join(root,
				importer.SeriesFolder(series.Title, int(series.Year)),
				importer.SeasonFolder(info.Season),
				importer.EpisodeFileName(series.Title, info.Season, info.Episodes, eps[0].Title, r.Quality, filepath.Ext(f.Path))),
			size: f.Size, episodeIDs: ids,
		})
	}
	return plan, skipped, nil
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

// movieForFile finds the expected movie a file holds, from its name, then
// its folders (innermost first).
func movieForFile(path string, expected []db.Movie) (db.Movie, bool) {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if m, ok := matchMovie(release.Parse(parts[i]), expected); ok {
			return m, true
		}
	}
	return db.Movie{}, false
}

// matchMovie finds the movie a parsed name refers to: same title (or
// original title) and a year within one of the movie's. A name without a
// year matches only if exactly one movie has that title.
func matchMovie(info release.Info, movies []db.Movie) (db.Movie, bool) {
	var found []db.Movie
	for _, m := range movies {
		if !release.MatchesMovieTitle(info, movieTargetOf(m)) {
			continue
		}
		if info.Year != 0 && m.Year != 0 && (info.Year < int(m.Year)-1 || info.Year > int(m.Year)+1) {
			continue
		}
		found = append(found, m)
	}
	if len(found) != 1 {
		return db.Movie{}, false
	}
	return found[0], true
}

// movieLabel is "Title (Year)", or the title alone if the year is unknown.
func movieLabel(m db.Movie) string {
	if m.Year > 0 {
		return fmt.Sprintf("%s (%d)", m.Title, m.Year)
	}
	return m.Title
}
