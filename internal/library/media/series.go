package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/store/db"
)

// Series is a TV series in the library.
type Series struct {
	ID              int64          `json:"id"`
	TMDBID          int64          `json:"tmdb_id"`
	TVDBID          *int64         `json:"tvdb_id,omitempty"`
	IMDBID          *string        `json:"imdb_id,omitempty"`
	Title           string         `json:"title"`
	OriginalTitle   string         `json:"original_title"`
	Year            int            `json:"year"`
	Overview        string         `json:"overview"`
	PosterPath      *string        `json:"poster_path,omitempty"`
	BackdropPath    *string        `json:"backdrop_path,omitempty"`
	Status          string         `json:"status"`
	RootFolderID    int64          `json:"root_folder_id"`
	Monitored       bool           `json:"monitored"`
	AddedAt         string         `json:"added_at"`
	LastRefreshedAt *string        `json:"last_refreshed_at,omitempty"`
	Seasons         []SeasonStatus `json:"seasons"`
}

// SeasonStatus summarises one season.
type SeasonStatus struct {
	Number     int  `json:"season"`
	Episodes   int  `json:"episodes"`
	Downloaded int  `json:"downloaded"`
	Monitored  int  `json:"monitored"` // monitored episodes
	Aired      int  `json:"aired"`
	Complete   bool `json:"complete"` // every aired episode has a file
}

// Episode is one episode in the library.
type Episode struct {
	ID          int64   `json:"id"`
	SeriesID    int64   `json:"series_id"`
	Season      int     `json:"season"`
	Episode     int     `json:"episode"`
	Title       string  `json:"title"`
	AirDate     *string `json:"air_date,omitempty"`
	Monitored   bool    `json:"monitored"`
	FilePath    *string `json:"file_path,omitempty"`
	FileQuality *string `json:"file_quality,omitempty"`
	Downloading bool    `json:"downloading"`
}

func episodeFromRow(e db.Episode, downloading bool) Episode {
	return Episode{
		ID: e.ID, SeriesID: e.SeriesID, Season: int(e.Season), Episode: int(e.Episode), Title: e.Title,
		AirDate: e.AirDate, Monitored: e.Monitored, FilePath: e.FilePath, FileQuality: e.FileQuality,
		Downloading: downloading,
	}
}

func (s *Service) seriesFromRow(r db.Series, eps []db.Episode) Series {
	out := Series{
		ID: r.ID, TMDBID: r.TMDBID, TVDBID: r.TVDBID, IMDBID: r.IMDBID, Title: r.Title,
		OriginalTitle: r.OriginalTitle, Year: int(r.Year), Overview: r.Overview, Status: r.Status, PosterPath: r.PosterPath, BackdropPath: OptString(r.BackdropPath),
		RootFolderID: r.RootFolderID, Monitored: r.Monitored, AddedAt: r.AddedAt,
		LastRefreshedAt: r.LastRefreshedAt, Seasons: []SeasonStatus{},
	}
	today := s.Today()
	idx := map[int]int{}
	for _, e := range eps {
		i, ok := idx[int(e.Season)]
		if !ok {
			i = len(out.Seasons)
			idx[int(e.Season)] = i
			out.Seasons = append(out.Seasons, SeasonStatus{Number: int(e.Season)})
		}
		st := &out.Seasons[i]
		st.Episodes++
		if e.FilePath != nil {
			st.Downloaded++
		}
		if e.Monitored {
			st.Monitored++
		}
		if Aired(e, today) {
			st.Aired++
		}
	}
	for i := range out.Seasons {
		st := &out.Seasons[i]
		st.Complete = st.Aired > 0 && st.Downloaded >= st.Aired
	}
	return out
}

// Aired reports whether e has aired by today (YYYY-MM-DD).
func Aired(e db.Episode, today string) bool {
	return e.AirDate != nil && *e.AirDate != "" && *e.AirDate <= today
}

// Today is the current date as stored in air dates (UTC, YYYY-MM-DD).
func (s *Service) Today() string { return s.now().UTC().Format("2006-01-02") }

// LookupSeries searches TMDB for TV series, keeping those that pass filter and
// marking those in the library. TMDB search takes no genre or language, so
// the filter is applied to its results.
func (s *Service) LookupSeries(ctx context.Context, query string, year int, filter TitleFilter) ([]DiscoverSeries, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, apperr.Invalid("q", "is required")
	}
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	series, err := tm.SearchSeries(ctx, query, year)
	if err != nil {
		return nil, err
	}
	series = slices.DeleteFunc(series, func(m metadata.Series) bool { return !filter.Matches(m.GenreIDs, m.OriginalLanguage) })
	return s.annotateSeries(ctx, series)
}

// MonitorMode chooses which episodes of a new series are monitored.
type MonitorMode string

const (
	MonitorAll    MonitorMode = "all"    // every episode except specials
	MonitorFuture MonitorMode = "future" // only episodes not yet aired
	MonitorNone   MonitorMode = "none"
)

// AddSeriesInput adds a series by TMDB ID.
type AddSeriesInput struct {
	TMDBID       int64       `json:"tmdb_id"`
	RootFolderID *int64      `json:"root_folder_id"`
	Monitor      MonitorMode `json:"monitor"` // default "all"
	// Search starts an automatic search for wanted episodes after adding.
	Search bool `json:"search"`
}

// AddSeries fetches a series and all its episodes from TMDB and adds it.
func (s *Service) AddSeries(ctx context.Context, in AddSeriesInput) (Series, error) {
	if in.TMDBID <= 0 {
		return Series{}, apperr.Invalid("tmdb_id", "is required")
	}
	if in.Monitor == "" {
		in.Monitor = MonitorAll
	}
	if in.Monitor != MonitorAll && in.Monitor != MonitorFuture && in.Monitor != MonitorNone {
		return Series{}, apperr.Invalid("monitor", "must be all, future or none")
	}
	if _, err := s.store.GetSeriesByTMDBID(ctx, in.TMDBID); err == nil {
		return Series{}, apperr.Conflict("series with TMDB id %d is already in the library", in.TMDBID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Series{}, fmt.Errorf("check existing series: %w", err)
	}
	root, err := s.settings.RootFolderFor(ctx, settings.MediaTV, in.RootFolderID)
	if err != nil {
		return Series{}, err
	}
	meta, seasons, err := s.fetchSeries(ctx, in.TMDBID)
	if err != nil {
		return Series{}, err
	}

	today := s.Today()
	var row db.Series
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		var err error
		row, err = q.CreateSeries(ctx, seriesParams(meta, root.ID, in.Monitor != MonitorNone))
		if err != nil {
			return err
		}
		return upsertEpisodes(ctx, q, row.ID, seasons, func(e metadata.Episode) bool {
			switch {
			case in.Monitor == MonitorNone || e.Season == 0:
				return false
			case in.Monitor == MonitorFuture:
				return e.AirDate == "" || e.AirDate > today
			default:
				return true
			}
		})
	})
	if err != nil {
		if apperr.IsUniqueViolation(err) {
			return Series{}, apperr.Conflict("series with TMDB id %d is already in the library", in.TMDBID)
		}
		return Series{}, fmt.Errorf("add series: %w", err)
	}
	s.log.Info("series added", "series", row.ID, "title", row.Title, "seasons", len(seasons))
	if in.Search && in.Monitor != MonitorNone {
		go func() {
			if err := s.searcher.SearchNewSeries(context.WithoutCancel(ctx), row.ID); err != nil {
				s.log.Warn("search after add", "series", row.ID, "err", err)
			}
		}()
	}
	return s.SeriesDetail(ctx, row.ID)
}

// fetchSeries loads a series and every season's episodes from TMDB.
func (s *Service) fetchSeries(ctx context.Context, tmdbID int64) (*metadata.Series, [][]metadata.Episode, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, nil, err
	}
	meta, err := tm.GetSeries(ctx, tmdbID)
	if err != nil {
		if errors.Is(err, metadata.ErrNotFound) {
			return nil, nil, apperr.Invalid("tmdb_id", "no series with TMDB id %d", tmdbID)
		}
		return nil, nil, err
	}
	var seasons [][]metadata.Episode
	for _, sum := range meta.Seasons {
		eps, err := tm.GetSeason(ctx, tmdbID, sum.Number)
		if err != nil {
			return nil, nil, fmt.Errorf("season %d: %w", sum.Number, err)
		}
		seasons = append(seasons, eps)
	}
	return meta, seasons, nil
}

func seriesParams(m *metadata.Series, rootID int64, monitored bool) db.CreateSeriesParams {
	p := db.CreateSeriesParams{
		TMDBID: m.TMDBID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: int64(m.Year),
		Overview: m.Overview, Status: m.Status, PosterPath: OptString(m.PosterPath), BackdropPath: m.BackdropPath, RootFolderID: rootID, Monitored: monitored,
	}
	if m.TVDBID > 0 {
		p.TVDBID = &m.TVDBID
	}
	if m.IMDBID != "" {
		p.IMDBID = &m.IMDBID
	}
	return p
}

// upsertEpisodes inserts new episodes (monitored per monitor) and updates
// titles and air dates of existing ones; existing monitoring is kept.
func upsertEpisodes(ctx context.Context, q *db.Queries, seriesID int64, seasons [][]metadata.Episode, monitor func(metadata.Episode) bool) error {
	for _, eps := range seasons {
		for _, e := range eps {
			var airDate *string
			if e.AirDate != "" {
				d := e.AirDate
				airDate = &d
			}
			_, err := q.UpsertEpisode(ctx, db.UpsertEpisodeParams{
				SeriesID: seriesID, Season: int64(e.Season), Episode: int64(e.Number),
				Title: e.Title, AirDate: airDate, Monitored: monitor(e),
			})
			if err != nil {
				return fmt.Errorf("save S%02dE%02d: %w", e.Season, e.Number, err)
			}
		}
	}
	return nil
}

// RefreshSeries re-reads a series from TMDB, updating metadata and adding
// newly announced episodes (monitored if the series is, except specials).
func (s *Service) RefreshSeries(ctx context.Context, id int64) (Series, error) {
	row, err := s.store.GetSeries(ctx, id)
	if err != nil {
		return Series{}, apperr.NotFound(err, "series", id)
	}
	meta, seasons, err := s.fetchSeries(ctx, row.TMDBID)
	if err != nil {
		return Series{}, err
	}
	p := seriesParams(meta, row.RootFolderID, row.Monitored)
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		if err := q.UpdateSeriesMetadata(ctx, db.UpdateSeriesMetadataParams{
			ID: id, TVDBID: p.TVDBID, IMDBID: p.IMDBID, Title: p.Title, OriginalTitle: p.OriginalTitle,
			Year: p.Year, Overview: p.Overview, Status: p.Status, PosterPath: p.PosterPath, BackdropPath: p.BackdropPath,
		}); err != nil {
			return err
		}
		return upsertEpisodes(ctx, q, id, seasons, func(e metadata.Episode) bool {
			return row.Monitored && e.Season != 0
		})
	})
	if err != nil {
		return Series{}, fmt.Errorf("refresh series: %w", err)
	}
	return s.SeriesDetail(ctx, id)
}

// RefreshAllSeries refreshes every monitored series, logging failures.
func (s *Service) RefreshAllSeries(ctx context.Context) error {
	rows, err := s.store.ListMonitoredSeries(ctx)
	if err != nil {
		return fmt.Errorf("list series: %w", err)
	}
	for _, r := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := s.RefreshSeries(ctx, r.ID); err != nil {
			s.log.Warn("refresh series", "series", r.ID, "title", r.Title, "err", err)
		}
	}
	return nil
}

// SeriesList lists the TV library with per-season counts.
func (s *Service) SeriesList(ctx context.Context) ([]Series, error) {
	rows, err := s.store.ListSeries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	out := make([]Series, 0, len(rows))
	for _, r := range rows {
		eps, err := s.store.ListEpisodes(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		out = append(out, s.seriesFromRow(r, eps))
	}
	return out, nil
}

// SeriesDetail returns one series with per-season counts.
func (s *Service) SeriesDetail(ctx context.Context, id int64) (Series, error) {
	r, err := s.store.GetSeries(ctx, id)
	if err != nil {
		return Series{}, apperr.NotFound(err, "series", id)
	}
	eps, err := s.store.ListEpisodes(ctx, id)
	if err != nil {
		return Series{}, fmt.Errorf("list episodes: %w", err)
	}
	return s.seriesFromRow(r, eps), nil
}

// SeriesEpisodes lists a series' episodes.
func (s *Service) SeriesEpisodes(ctx context.Context, id int64) ([]Episode, error) {
	if _, err := s.store.GetSeries(ctx, id); err != nil {
		return nil, apperr.NotFound(err, "series", id)
	}
	eps, err := s.store.ListEpisodes(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	active, err := s.ActiveEpisodeIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Episode, len(eps))
	for i, e := range eps {
		out[i] = episodeFromRow(e, active[e.ID])
	}
	return out, nil
}

// ActiveEpisodeIDs returns the episodes with a download in progress or
// awaiting import.
func (s *Service) ActiveEpisodeIDs(ctx context.Context) (map[int64]bool, error) {
	ids, err := s.store.ListActiveEpisodeIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active episodes: %w", err)
	}
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// UpdateSeriesInput changes a series' settings; nil fields are unchanged.
type UpdateSeriesInput struct {
	Monitored    *bool  `json:"monitored"`
	RootFolderID *int64 `json:"root_folder_id"`
}

// UpdateSeries applies in. A new root folder affects future imports only.
func (s *Service) UpdateSeries(ctx context.Context, id int64, in UpdateSeriesInput) (Series, error) {
	row, err := s.store.GetSeries(ctx, id)
	if err != nil {
		return Series{}, apperr.NotFound(err, "series", id)
	}
	p := db.UpdateSeriesSettingsParams{ID: id, Monitored: row.Monitored, RootFolderID: row.RootFolderID}
	if in.Monitored != nil {
		p.Monitored = *in.Monitored
	}
	if in.RootFolderID != nil {
		root, err := s.settings.RootFolderFor(ctx, settings.MediaTV, in.RootFolderID)
		if err != nil {
			return Series{}, err
		}
		p.RootFolderID = root.ID
	}
	if _, err := s.store.UpdateSeriesSettings(ctx, p); err != nil {
		return Series{}, fmt.Errorf("update series: %w", err)
	}
	return s.SeriesDetail(ctx, id)
}

// SetSeasonMonitored monitors or unmonitors every episode of a season.
func (s *Service) SetSeasonMonitored(ctx context.Context, seriesID int64, season int, monitored bool) error {
	n, err := s.store.SetSeasonMonitored(ctx, db.SetSeasonMonitoredParams{SeriesID: seriesID, Season: int64(season), Monitored: monitored})
	if err != nil {
		return fmt.Errorf("update season: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("series %d season %d: %w", seriesID, season, apperr.ErrNotFound)
	}
	return nil
}

// SetEpisodeMonitored monitors or unmonitors one episode.
func (s *Service) SetEpisodeMonitored(ctx context.Context, id int64, monitored bool) (Episode, error) {
	n, err := s.store.SetEpisodeMonitored(ctx, db.SetEpisodeMonitoredParams{ID: id, Monitored: monitored})
	if err != nil {
		return Episode{}, fmt.Errorf("update episode: %w", err)
	}
	if n == 0 {
		return Episode{}, fmt.Errorf("episode %d: %w", id, apperr.ErrNotFound)
	}
	e, err := s.store.GetEpisode(ctx, id)
	if err != nil {
		return Episode{}, apperr.NotFound(err, "episode", id)
	}
	return episodeFromRow(e, false), nil
}

// DeleteSeries removes a series and its episodes from the library. Files
// on disk and downloads in progress are left alone.
func (s *Service) DeleteSeries(ctx context.Context, id int64) error {
	n, err := s.store.DeleteSeries(ctx, id)
	if err != nil {
		return fmt.Errorf("delete series: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("series %d: %w", id, apperr.ErrNotFound)
	}
	return nil
}
