package acquire

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/torrent"
)

const (
	importRetryDelay = 5 * time.Minute
	removeTimeout    = 30 * time.Second
)

// Download is a grabbed torrent with its live progress.
type Download struct {
	ID       int64  `json:"id"`
	MovieID  *int64 `json:"movie_id,omitempty"`
	SeriesID *int64 `json:"series_id,omitempty"`
	// Title, Year and the image paths describe the movie or series;
	// Episodes lists TV episode codes ("S01E02"). Filled by Queue.
	Title        string            `json:"title,omitempty"`
	Year         int               `json:"year,omitempty"`
	PosterPath   *string           `json:"poster_path,omitempty"`
	BackdropPath *string           `json:"backdrop_path,omitempty"`
	Episodes     []string          `json:"episodes,omitempty"`
	Paused       bool              `json:"paused"`
	IndexerID    *int64            `json:"indexer_id,omitempty"`
	InfoHash     string            `json:"info_hash"`
	ReleaseTitle string            `json:"release_title"`
	Quality      string            `json:"quality"`
	Size         int64             `json:"size"`
	Status       string            `json:"status"`
	Error        *string           `json:"error,omitempty"`
	AddedAt      string            `json:"added_at"`
	CompletedAt  *string           `json:"completed_at,omitempty"`
	ImportedAt   *string           `json:"imported_at,omitempty"`
	Progress     *torrent.Progress `json:"progress,omitempty"` // nil if not in the engine
	// PeerCountries counts connected peers by country, most first; nil if
	// not in the engine.
	PeerCountries []CountryCount `json:"peer_countries,omitempty"`
}

// CountryCount is how many peers are in one country. Country is an ISO
// 3166-1 alpha-2 code, or "" when the location is unknown.
type CountryCount struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}

// Peer is a connected peer of a download, located by IP.
type Peer struct {
	torrent.Peer
	// Country is an ISO 3166-1 alpha-2 code, or "" when unknown.
	Country string
}

func (s *Service) downloadFromRow(eng *torrent.Engine, r db.Download) Download {
	d := Download{
		ID: r.ID, MovieID: r.MovieID, SeriesID: r.SeriesID, IndexerID: r.IndexerID, InfoHash: r.InfoHash, ReleaseTitle: r.ReleaseTitle,
		Quality: r.Quality, Size: r.Size, Status: r.Status, Error: r.Error, AddedAt: r.AddedAt,
		CompletedAt: r.CompletedAt, ImportedAt: r.ImportedAt, Paused: r.Paused,
	}
	if eng == nil {
		return d
	}
	if dl, ok := eng.Get(r.InfoHash); ok {
		p := dl.Progress()
		d.Progress = &p
		d.PeerCountries = countPeerCountries(s.locatePeers(dl.Peers()))
	}
	return d
}

// locatePeers adds each peer's country.
func (s *Service) locatePeers(peers []torrent.Peer) []Peer {
	out := make([]Peer, len(peers))
	for i, p := range peers {
		out[i] = Peer{Peer: p, Country: s.geo.Country(p.Addr)}
	}
	return out
}

// countPeerCountries groups peers by country, most first, then by code.
func countPeerCountries(peers []Peer) []CountryCount {
	counts := map[string]int{}
	for _, p := range peers {
		counts[p.Country]++
	}
	out := make([]CountryCount, 0, len(counts))
	for c, n := range counts {
		out = append(out, CountryCount{Country: c, Count: n})
	}
	slices.SortFunc(out, func(a, b CountryCount) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Country, b.Country)
	})
	return out
}

// grabTarget says what a download is for: one movie, or episodes of a
// series.
type grabTarget struct {
	movieID    *int64
	seriesID   *int64
	episodeIDs []int64
}

// GrabMovieRelease downloads r for the movie. r normally comes from
// SearchMovie; the indexer it names supplies the network for fetching the
// .torrent file.
func (s *Service) GrabMovieRelease(ctx context.Context, movieID int64, r ReleaseResult) (Download, error) {
	if _, err := s.store.GetMovie(ctx, movieID); err != nil {
		return Download{}, apperr.NotFound(err, "movie", movieID)
	}
	return s.grab(ctx, grabTarget{movieID: &movieID}, r)
}

// resolveRelease fetches what a release points at through the indexer's
// signed-in session, on the traffic network.
func (s *Service) resolveRelease(ctx context.Context, ixRow db.Indexer, r indexer.Release) (indexer.TorrentSource, error) {
	if r.MagnetURI != "" {
		return indexer.TorrentSource{Magnet: r.MagnetURI}, nil
	}
	searcher, err := s.indexers.Searcher(ctx, ixRow)
	if err != nil {
		return indexer.TorrentSource{}, err
	}
	return searcher.Resolve(ctx, r)
}

func (s *Service) grab(ctx context.Context, target grabTarget, r ReleaseResult) (Download, error) {
	if strings.TrimSpace(r.Title) == "" {
		return Download{}, apperr.Invalid("title", "is required")
	}
	if r.MagnetURI == "" && r.DownloadURL == "" {
		return Download{}, apperr.Invalid("download_url", "a download_url or magnet_uri is required")
	}
	ixRow, err := s.store.GetIndexer(ctx, r.IndexerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Download{}, apperr.Invalid("indexer_id", "indexer %d does not exist", r.IndexerID)
		}
		return Download{}, fmt.Errorf("load indexer: %w", err)
	}
	dirs, err := s.settings.DownloadSettings(ctx)
	if err != nil {
		return Download{}, err
	}

	src, err := s.resolveRelease(ctx, ixRow, indexer.Release{
		Title: r.Title, MagnetURI: r.MagnetURI, DownloadURL: r.DownloadURL, InfoHash: r.InfoHash,
	})
	if err != nil {
		return Download{}, fmt.Errorf("fetch release from %s: %w", ixRow.Name, err)
	}
	var out Download
	err = s.engine.WithEngine(func(eng *torrent.Engine) error {
		var err error
		out, err = s.addAndRecord(ctx, eng, target, r, ixRow, dirs, src)
		return err
	})
	return out, err
}

// addAndRecord adds a resolved release to the engine and records it.
func (s *Service) addAndRecord(ctx context.Context, eng *torrent.Engine, target grabTarget, r ReleaseResult, ixRow db.Indexer, dirs settings.DownloadSettings, src indexer.TorrentSource) (Download, error) {
	var dl *torrent.Download
	var magnet *string
	var err error
	if src.Magnet != "" {
		magnet = &src.Magnet
		dl, err = eng.AddMagnet(src.Magnet, dirs.IncompleteDir)
	} else {
		dl, err = eng.AddTorrentBytes(src.MetaInfo, dirs.IncompleteDir)
	}
	if err != nil {
		if strings.Contains(err.Error(), "already in the engine") {
			return Download{}, apperr.Conflict("%v", err)
		}
		return Download{}, err
	}

	quality := release.Parse(r.Title).Quality()
	ixID := ixRow.ID
	var row db.Download
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		var err error
		row, err = q.CreateDownload(ctx, db.CreateDownloadParams{
			MovieID: target.movieID, SeriesID: target.seriesID, IndexerID: &ixID, InfoHash: dl.InfoHash(),
			ReleaseTitle: r.Title, Quality: quality, Size: r.Size, Magnet: magnet, Metainfo: src.MetaInfo,
			IncompleteDir: dirs.IncompleteDir, CompleteDir: dirs.CompleteDir,
		})
		if err != nil {
			return err
		}
		for _, epID := range target.episodeIDs {
			if err := q.AddDownloadEpisode(ctx, db.AddDownloadEpisodeParams{DownloadID: row.ID, EpisodeID: epID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), removeTimeout)
		defer cancel()
		if rmErr := dl.Remove(rmCtx); rmErr != nil {
			s.log.Error("remove torrent after failed grab", "info_hash", dl.InfoHash(), "err", rmErr)
		}
		if apperr.IsUniqueViolation(err) {
			return Download{}, apperr.Conflict("torrent %s was already grabbed", dl.InfoHash())
		}
		return Download{}, fmt.Errorf("record download: %w", err)
	}
	s.log.Info("grabbed release", "release", r.Title, "indexer", ixRow.Name, "info_hash", row.InfoHash,
		"movie", deref(target.movieID), "series", deref(target.seriesID), "episodes", len(target.episodeIDs))
	return s.downloadFromRow(eng, row), nil
}

// RemoveDownload stops a download and forgets it. With deleteData, the
// torrent's files in the download directories are deleted too (library
// files are never touched).
func (s *Service) RemoveDownload(ctx context.Context, id int64, deleteData bool) error {
	row, err := s.store.GetDownload(ctx, id)
	if err != nil {
		return apperr.NotFound(err, "download", id)
	}
	// Hold the runtime for the whole removal so the engine is not swapped.
	s.engine.Inspect(func(rt *engine.Runtime, _ error) { err = s.removeDownload(ctx, rt, row, deleteData) })
	return err
}

func (s *Service) removeDownload(ctx context.Context, rt *engine.Runtime, row db.Download, deleteData bool) error {
	name := ""
	if rt != nil {
		if dl, ok := rt.Engine.Get(row.InfoHash); ok {
			name = dl.Name()
			rmCtx, cancel := context.WithTimeout(ctx, removeTimeout)
			defer cancel()
			if err := dl.Remove(rmCtx); err != nil {
				return err
			}
		}
	}
	if deleteData {
		if name == "" && row.Metainfo != nil {
			name, _ = torrentName(row.Metainfo)
		}
		if err := deleteTorrentData(name, row.IncompleteDir, row.CompleteDir); err != nil {
			return err
		}
	}
	if _, err := s.store.DeleteDownload(ctx, row.ID); err != nil {
		return fmt.Errorf("delete download: %w", err)
	}
	return nil
}

func deleteTorrentData(name string, dirs ...string) error {
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return fmt.Errorf("refusing to delete data: unsafe or unknown torrent name %q", name)
	}
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		for _, candidate := range []string{p, p + ".part"} {
			if err := os.RemoveAll(candidate); err != nil {
				return fmt.Errorf("delete %s: %w", candidate, err)
			}
		}
	}
	return nil
}

func torrentName(metainfoBytes []byte) (string, error) {
	mi, err := metainfo.Load(strings.NewReader(string(metainfoBytes)))
	if err != nil {
		return "", err
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return "", err
	}
	return info.BestName(), nil
}

// RestoreDownloads re-adds every unfinished or seeding download to a new
// engine.
func (s *Service) RestoreDownloads(ctx context.Context, eng *torrent.Engine) error {
	rows, err := s.store.ListDownloads(ctx)
	if err != nil {
		return fmt.Errorf("list downloads: %w", err)
	}
	for _, r := range rows {
		if r.Status == media.DownloadFailed {
			continue
		}
		if r.Status == media.DownloadImported && !seedDataPresent(r) {
			// Imported with mode "move" (or cleaned up by the user): there is
			// nothing left to seed, and re-adding would download it again.
			continue
		}
		if err := s.ensureInEngine(ctx, eng, r); err != nil {
			s.log.Error("restore download", "download", r.ID, "release", r.ReleaseTitle, "err", err)
			s.setDownloadError(ctx, r.ID, err)
		}
	}
	return nil
}

// ensureInEngine adds r to the engine if it is not there, in the directory
// that matches its status. A downloading row whose data already sits in
// complete_dir (a crash between moving and recording it) is promoted.
func (s *Service) ensureInEngine(ctx context.Context, eng *torrent.Engine, r db.Download) error {
	if _, ok := eng.Get(r.InfoHash); ok {
		return nil
	}
	dir := r.CompleteDir
	if r.Status == media.DownloadDownloading {
		dir = r.IncompleteDir
		if r.Metainfo != nil && r.IncompleteDir != r.CompleteDir {
			if name, err := torrentName(r.Metainfo); err == nil && !exists(filepath.Join(r.IncompleteDir, name)) && exists(filepath.Join(r.CompleteDir, name)) {
				dir = r.CompleteDir
				if err := s.store.MarkDownloadCompleted(ctx, r.ID); err != nil {
					return fmt.Errorf("mark completed: %w", err)
				}
			}
		}
	}
	var dl *torrent.Download
	var err error
	if r.Metainfo != nil {
		dl, err = eng.AddTorrentBytes(r.Metainfo, dir)
	} else if r.Magnet != nil {
		dl, err = eng.AddMagnet(*r.Magnet, dir)
	} else {
		err = errors.New("download has neither metainfo nor magnet")
	}
	if err != nil {
		return err
	}
	// A row promoted to completed above is no longer pausable.
	if r.Paused && r.Status == media.DownloadDownloading && dir == r.IncompleteDir {
		dl.Pause()
	}
	return nil
}

// seedDataPresent reports whether an imported download's data is still in
// its complete directory.
func seedDataPresent(r db.Download) bool {
	if r.Metainfo == nil {
		return false
	}
	name, err := torrentName(r.Metainfo)
	if err != nil {
		return false
	}
	return exists(filepath.Join(r.CompleteDir, name))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ProcessDownloads advances every download one step: saving metadata,
// moving finished data to complete_dir, and importing. It is called
// periodically by the scheduler.
func (s *Service) ProcessDownloads(ctx context.Context) error {
	err := s.engine.WithEngine(func(eng *torrent.Engine) error { return s.processDownloads(ctx, eng) })
	if s.engine.RuntimeError() != nil {
		return nil // nothing to do without an engine; the scheduler retries the runtime
	}
	return err
}

func (s *Service) processDownloads(ctx context.Context, eng *torrent.Engine) error {
	downloading, err := s.store.ListDownloadsByStatus(ctx, media.DownloadDownloading)
	if err != nil {
		return fmt.Errorf("list downloads: %w", err)
	}
	for _, r := range downloading {
		if err := s.processDownloading(ctx, eng, r); err != nil {
			s.log.Error("process download", "download", r.ID, "release", r.ReleaseTitle, "err", err)
			s.setDownloadError(ctx, r.ID, err)
		}
	}
	completed, err := s.store.ListDownloadsByStatus(ctx, media.DownloadCompleted)
	if err != nil {
		return fmt.Errorf("list downloads: %w", err)
	}
	for _, r := range completed {
		s.maybeImport(ctx, eng, r)
	}
	return nil
}

func (s *Service) processDownloading(ctx context.Context, eng *torrent.Engine, r db.Download) error {
	dl, ok := eng.Get(r.InfoHash)
	if !ok {
		s.log.Warn("download missing from engine, re-adding", "download", r.ID)
		return s.ensureInEngine(ctx, eng, r)
	}
	if !dl.HaveInfo() {
		return nil
	}
	if r.Metainfo == nil {
		raw, err := dl.MetaInfo()
		if err != nil {
			return err
		}
		if err := s.store.SetDownloadMetainfo(ctx, db.SetDownloadMetainfoParams{ID: r.ID, Metainfo: raw}); err != nil {
			return fmt.Errorf("save metainfo: %w", err)
		}
		r.Metainfo = raw
	}
	if !dl.Complete() {
		return nil
	}
	return s.finishDownload(ctx, eng, r, dl)
}

// finishDownload moves the data from incomplete_dir to complete_dir,
// re-adds the torrent there so it keeps seeding, and records completion.
func (s *Service) finishDownload(ctx context.Context, eng *torrent.Engine, r db.Download, dl *torrent.Download) error {
	if r.IncompleteDir != r.CompleteDir {
		name := dl.Name()
		if name == "" || name != filepath.Base(name) {
			return fmt.Errorf("unsafe torrent name %q", name)
		}
		src, dst := filepath.Join(r.IncompleteDir, name), filepath.Join(r.CompleteDir, name)
		if exists(dst) {
			return s.failDownload(ctx, r.ID, fmt.Errorf("%s already exists in the complete directory", dst))
		}
		rmCtx, cancel := context.WithTimeout(ctx, removeTimeout)
		defer cancel()
		if err := dl.Remove(rmCtx); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			// Put the torrent back where it was so it keeps seeding.
			if _, addErr := eng.AddTorrentBytes(r.Metainfo, r.IncompleteDir); addErr != nil {
				s.log.Error("re-add after failed move", "download", r.ID, "err", addErr)
			}
			return s.failDownload(ctx, r.ID, fmt.Errorf("move to complete directory (incomplete and complete directories must be on the same filesystem): %w", err))
		}
		if _, err := eng.AddTorrentBytes(r.Metainfo, r.CompleteDir); err != nil {
			return fmt.Errorf("re-add for seeding: %w", err)
		}
	}
	if err := s.store.MarkDownloadCompleted(ctx, r.ID); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	s.log.Info("download complete", "download", r.ID, "release", r.ReleaseTitle)
	r.Status = media.DownloadCompleted
	s.maybeImport(ctx, eng, r)
	return nil
}

func (s *Service) failDownload(ctx context.Context, id int64, cause error) error {
	msg := cause.Error()
	if err := s.store.MarkDownloadFailed(ctx, db.MarkDownloadFailedParams{ID: id, Error: &msg}); err != nil {
		return fmt.Errorf("%w (and recording the failure: %v)", cause, err)
	}
	s.log.Error("download failed", "download", id, "err", cause)
	return nil
}

func (s *Service) setDownloadError(ctx context.Context, id int64, cause error) {
	msg := cause.Error()
	if err := s.store.SetDownloadError(ctx, db.SetDownloadErrorParams{ID: id, Error: &msg}); err != nil {
		s.log.Error("record download error", "download", id, "err", err)
	}
}

// maybeImport imports a completed download unless a recent attempt failed.
func (s *Service) maybeImport(ctx context.Context, eng *torrent.Engine, r db.Download) {
	s.importMu.Lock()
	next, waiting := s.importRetry[r.ID]
	s.importMu.Unlock()
	if waiting && time.Now().Before(next) {
		return
	}
	if err := s.importDownload(ctx, eng, r); err != nil {
		s.log.Warn("import failed, will retry", "download", r.ID, "release", r.ReleaseTitle, "retry_in", importRetryDelay, "err", err)
		s.setDownloadError(ctx, r.ID, fmt.Errorf("import: %w", err))
		s.importMu.Lock()
		s.importRetry[r.ID] = time.Now().Add(importRetryDelay)
		s.importMu.Unlock()
		return
	}
	s.importMu.Lock()
	delete(s.importRetry, r.ID)
	s.importMu.Unlock()
}

// QueueFilter narrows Queue to one movie or series; zero means all.
type QueueFilter struct {
	MovieID  *int64
	SeriesID *int64
}

// Queue lists downloads, newest first, with the title of what each is for.
func (s *Service) Queue(ctx context.Context, f QueueFilter) ([]Download, error) {
	var rows []db.Download
	var err error
	switch {
	case f.MovieID != nil && f.SeriesID != nil:
		return nil, apperr.Invalid("movie_id", "filter by movie_id or series_id, not both")
	case f.MovieID != nil:
		rows, err = s.store.ListDownloadsForMovie(ctx, f.MovieID)
	case f.SeriesID != nil:
		rows, err = s.store.ListDownloadsForSeries(ctx, f.SeriesID)
	default:
		rows, err = s.store.ListDownloads(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}

	type named struct {
		title    string
		year     int
		poster   *string
		backdrop *string
	}
	// One lookup per distinct movie or series, not per row.
	movies, series := map[int64]named{}, map[int64]named{}
	for _, r := range rows {
		if r.MovieID != nil {
			if _, seen := movies[*r.MovieID]; !seen {
				if m, err := s.store.GetMovie(ctx, *r.MovieID); err == nil {
					movies[*r.MovieID] = named{m.Title, int(m.Year), m.PosterPath, media.OptString(m.BackdropPath)}
				} else {
					movies[*r.MovieID] = named{}
				}
			}
		}
		if r.SeriesID != nil {
			if _, seen := series[*r.SeriesID]; !seen {
				if sr, err := s.store.GetSeries(ctx, *r.SeriesID); err == nil {
					series[*r.SeriesID] = named{sr.Title, int(sr.Year), sr.PosterPath, media.OptString(sr.BackdropPath)}
				} else {
					series[*r.SeriesID] = named{}
				}
			}
		}
	}

	eng := s.engine.CurrentEngine()
	out := make([]Download, len(rows))
	for i, r := range rows {
		d := s.downloadFromRow(eng, r)
		switch {
		case r.MovieID != nil:
			n := movies[*r.MovieID]
			d.Title, d.Year, d.PosterPath, d.BackdropPath = n.title, n.year, n.poster, n.backdrop
		case r.SeriesID != nil:
			n := series[*r.SeriesID]
			d.Title, d.Year, d.PosterPath, d.BackdropPath = n.title, n.year, n.poster, n.backdrop
			eps, err := s.store.ListDownloadEpisodes(ctx, r.ID)
			if err != nil {
				return nil, fmt.Errorf("list download episodes: %w", err)
			}
			for _, e := range eps {
				d.Episodes = append(d.Episodes, fmt.Sprintf("S%02dE%02d", e.Season, e.Episode))
			}
		}
		out[i] = d
	}
	return out, nil
}

// deref returns *p, or 0 for nil (for logging optional ids).
func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
