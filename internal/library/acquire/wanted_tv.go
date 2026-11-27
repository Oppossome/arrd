package acquire

import (
	"context"
	"fmt"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// episodesWant is a season (episode nil) or one episode of a series. Only
// the series' titles, year and TVDB id are searched by.
type episodesWant struct {
	s       *Service
	series  db.Series
	want    release.EpisodeWant
	episode *int
}

func (w episodesWant) text() string { return tvText(w.series, w.want.Season, w.episode) }
func (w episodesWant) tv() bool     { return true }

// query is a TVDB-id lookup when possible, else text.
func (w episodesWant) query(ix indexers.Indexer) (indexer.Query, bool) {
	s, season, episode := w.series, w.want.Season, w.episode
	q := indexer.Query{Categories: ix.TVCategories}
	switch caps := ix.Caps; {
	case caps == nil:
		q.Type, q.Q = indexer.SearchGeneric, w.text()
	case caps.TVSearch.Available && caps.TVSearch.Supports("season") && (episode == nil || caps.TVSearch.Supports("ep")):
		q.Type, q.Season, q.Episode = indexer.SearchTV, &season, episode
		if s.TVDBID != nil && caps.TVSearch.Supports("tvdbid") {
			q.TVDBID = *s.TVDBID
		} else {
			q.Q = s.Title
		}
	case caps.TVSearch.Available:
		q.Type, q.Q = indexer.SearchTV, w.text()
	case caps.Search.Available:
		q.Type, q.Q = indexer.SearchGeneric, w.text()
	default:
		return indexer.Query{}, false
	}
	return q, true
}

func (w episodesWant) evaluate(c release.Candidate, lim release.Limits) release.Decision {
	return release.EvaluateEpisode(c, seriesTarget(w.series), w.want, lim)
}

func (w episodesWant) claim(ctx context.Context, info release.Info) (grabTarget, error) {
	return w.s.claimEpisodes(ctx, w.series, info)
}

// seriesPackWant is whole-series packs: multi-season and "Complete Series"
// releases (for a one-season series, its season pack).
type seriesPackWant struct {
	s      *Service
	series db.Series
	want   release.SeriesPackWant
}

// text is the title alone: whole-series packs carry no single season or
// episode number to search by.
func (w seriesPackWant) text() string { return w.series.Title }
func (w seriesPackWant) tv() bool     { return true }

// query searches by title: an id lookup tends to return only recent
// episodes.
func (w seriesPackWant) query(ix indexers.Indexer) (indexer.Query, bool) {
	q := indexer.Query{Categories: ix.TVCategories, Q: w.series.Title}
	switch caps := ix.Caps; {
	case caps == nil:
		q.Type = indexer.SearchGeneric
	case caps.TVSearch.Available:
		q.Type = indexer.SearchTV
	case caps.Search.Available:
		q.Type = indexer.SearchGeneric
	default:
		return indexer.Query{}, false
	}
	return q, true
}

func (w seriesPackWant) evaluate(c release.Candidate, lim release.Limits) release.Decision {
	return release.EvaluateSeriesPack(c, seriesTarget(w.series), w.want, lim)
}

func (w seriesPackWant) claim(ctx context.Context, info release.Info) (grabTarget, error) {
	return w.s.claimEpisodes(ctx, w.series, info)
}

// seriesWant builds a whole-series want from the library's episodes, or
// from TMDB's seasons (meta) for a series not in the library.
func (s *Service) seriesWant(ctx context.Context, series db.Series, meta *metadata.Series) (wanted, error) {
	counts := map[int]int{}
	if meta != nil {
		for _, ss := range meta.Seasons {
			if ss.Number > 0 && ss.EpisodeCount > 0 {
				counts[ss.Number] = ss.EpisodeCount
			}
		}
	} else {
		eps, err := s.store.ListEpisodes(ctx, series.ID)
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		for _, e := range eps {
			if e.Season > 0 {
				counts[int(e.Season)]++
			}
		}
	}
	if len(counts) == 0 {
		return nil, fmt.Errorf("series %q has no regular seasons: %w", series.Title, apperr.ErrNotFound)
	}
	return seriesPackWant{s: s, series: series, want: release.SeriesPackWant{SeasonEpisodes: counts}}, nil
}

// seasonWant builds a season want: season packs and releases containing
// any wanted episode are accepted. In the library, episodes that already
// have a file are not wanted; otherwise every episode TMDB lists is.
func (s *Service) seasonWant(ctx context.Context, series db.Series, meta *metadata.Series, season int) (wanted, error) {
	want := release.EpisodeWant{Season: season, AllowPack: true}
	if meta != nil {
		want.PackEpisodes = seasonEpisodeCount(meta, season)
		for e := 1; e <= want.PackEpisodes; e++ {
			want.Episodes = append(want.Episodes, e)
		}
	} else {
		eps, err := s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: series.ID, Season: int64(season)})
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		want.PackEpisodes = len(eps)
		for _, e := range eps {
			if e.FilePath == nil {
				want.Episodes = append(want.Episodes, int(e.Episode))
			}
		}
	}
	if want.PackEpisodes <= 0 {
		return nil, fmt.Errorf("series %q season %d: %w", series.Title, season, apperr.ErrNotFound)
	}
	return episodesWant{s: s, series: series, want: want}, nil
}

// episodeWant builds a want for one episode. Season packs are rejected:
// search the season for those.
func (s *Service) episodeWant(ctx context.Context, series db.Series, meta *metadata.Series, season, episode int) (wanted, error) {
	exists := false
	if meta != nil {
		exists = episode <= seasonEpisodeCount(meta, season)
	} else {
		eps, err := s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: series.ID, Season: int64(season)})
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		for _, e := range eps {
			exists = exists || int(e.Episode) == episode
		}
	}
	if !exists {
		return nil, fmt.Errorf("series %q S%02dE%02d: %w", series.Title, season, episode, apperr.ErrNotFound)
	}
	return episodesWant{s: s, series: series, want: release.EpisodeWant{Season: season, Episodes: []int{episode}}, episode: &episode}, nil
}

// claimEpisodes finds the episodes of series a TV release is expected to
// contain, from its name: listed episodes, or every episode without a file
// in the seasons a pack covers. Multi-season packs are not expected to
// contain specials.
func (s *Service) claimEpisodes(ctx context.Context, series db.Series, info release.Info) (grabTarget, error) {
	if series.ID == 0 {
		return grabTarget{}, fmt.Errorf("series %q is not in the library: %w", series.Title, apperr.ErrNotFound)
	}
	if !info.IsTV {
		return grabTarget{}, apperr.Invalid("title", "%q is not a TV release", info.Raw)
	}
	var eps []db.Episode
	var err error
	if info.MultiSeason {
		eps, err = s.store.ListEpisodes(ctx, series.ID)
	} else {
		eps, err = s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: series.ID, Season: int64(info.Season)})
	}
	if err != nil {
		return grabTarget{}, fmt.Errorf("list episodes: %w", err)
	}
	var ids []int64
	for _, e := range eps {
		if info.MultiSeason && e.Season == 0 || !info.Covers(int(e.Season), int(e.Episode)) {
			continue
		}
		if !info.SeasonPack || e.FilePath == nil {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return grabTarget{}, apperr.Invalid("title", "%q matches no episodes of this series that need a file", info.Raw)
	}
	id := series.ID
	return grabTarget{seriesID: &id, episodeIDs: ids}, nil
}

func seriesTarget(s db.Series) release.SeriesTarget {
	return release.SeriesTarget{Title: s.Title, OriginalTitle: s.OriginalTitle, Year: int(s.Year)}
}

// tvText is a text search for a season of s (episode nil) or one episode,
// such as "Title S01" or "Title S01E02".
func tvText(s db.Series, season int, episode *int) string {
	if episode != nil {
		return fmt.Sprintf("%s S%02dE%02d", s.Title, season, *episode)
	}
	return fmt.Sprintf("%s S%02d", s.Title, season)
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

// autoSearchSeason grabs what a library season still needs. When the whole
// season has aired and at least two episodes are wanted, a season pack is
// tried first; anything still missing is then searched episode by episode.
func (s *Service) autoSearchSeason(ctx context.Context, seriesID int64, season int) (AutoSearchResult, error) {
	out := AutoSearchResult{Grabbed: []Download{}, Failures: []indexers.IndexerFailure{}}
	series, err := s.store.GetSeries(ctx, seriesID)
	if err != nil {
		return out, apperr.NotFound(err, "series", seriesID)
	}
	wanted, fullyAired, err := s.wantedInSeason(ctx, seriesID, season)
	if err != nil {
		return out, err
	}
	if fullyAired && len(wanted) >= 2 {
		w, err := s.seasonWant(ctx, series, nil, season)
		if err != nil {
			return out, err
		}
		packs, err := s.autoSearch(ctx, w, func(r ReleaseResult) bool { return r.SeasonPack })
		if err != nil {
			s.log.Warn().Int64("series", seriesID).Int("season", season).Err(err).Msg("season pack grab")
		}
		out.add(packs)
		if len(packs.Grabbed) > 0 {
			return out, nil
		}
	}

	for _, ep := range wanted {
		// An earlier grab in this loop (e.g. a multi-episode release) may
		// already cover this episode.
		still, _, err := s.wantedInSeason(ctx, seriesID, season)
		if err != nil {
			return out, err
		}
		if !containsEpisode(still, ep.ID) {
			continue
		}
		w, err := s.episodeWant(ctx, series, nil, season, int(ep.Episode))
		if err != nil {
			return out, err
		}
		res, err := s.autoSearch(ctx, w, nil)
		if err != nil {
			s.log.Warn().Int64("series", seriesID).Str("episode", fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)).Err(err).Msg("episode search")
		}
		out.add(res)
	}
	return out, nil
}

// wantedInSeason returns the season's wanted episodes, and whether every
// episode of the season has aired.
func (s *Service) wantedInSeason(ctx context.Context, seriesID int64, season int) ([]db.Episode, bool, error) {
	eps, err := s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: seriesID, Season: int64(season)})
	if err != nil {
		return nil, false, fmt.Errorf("list episodes: %w", err)
	}
	active, err := s.media.ActiveItems(ctx)
	if err != nil {
		return nil, false, err
	}
	today := s.media.Today()
	fullyAired := len(eps) > 0
	var wanted []db.Episode
	for _, e := range eps {
		if !media.Aired(e, today) {
			fullyAired = false
			continue
		}
		if e.Monitored && e.FilePath == nil && !active.Episodes[e.ID] {
			wanted = append(wanted, e)
		}
	}
	return wanted, fullyAired, nil
}

func containsEpisode(eps []db.Episode, id int64) bool {
	for _, e := range eps {
		if e.ID == id {
			return true
		}
	}
	return false
}

// SearchWantedEpisodes searches every season that has wanted episodes,
// optionally limited to one series.
func (s *Service) SearchWantedEpisodes(ctx context.Context, seriesID *int64) error {
	_, err := s.searchWantedEpisodes(ctx, seriesID)
	return err
}

func (s *Service) searchWantedEpisodes(ctx context.Context, seriesID *int64) (AutoSearchResult, error) {
	out := AutoSearchResult{Grabbed: []Download{}, Failures: []indexers.IndexerFailure{}}
	wanted, err := s.store.ListWantedEpisodes(ctx, s.media.Today())
	if err != nil {
		return out, fmt.Errorf("list wanted episodes: %w", err)
	}
	type seasonKey struct {
		series int64
		season int
	}
	var order []seasonKey
	seen := map[seasonKey]bool{}
	for _, e := range wanted {
		if seriesID != nil && e.SeriesID != *seriesID {
			continue
		}
		k := seasonKey{e.SeriesID, int(e.Season)}
		if !seen[k] {
			seen[k] = true
			order = append(order, k)
		}
	}
	for _, k := range order {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		res, err := s.autoSearchSeason(ctx, k.series, k.season)
		if err != nil {
			s.log.Warn().Int64("series", k.series).Int("season", k.season).Err(err).Msg("wanted season search")
		}
		out.add(res)
	}
	return out, nil
}
