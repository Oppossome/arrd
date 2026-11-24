package acquire

import (
	"context"
	"fmt"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

func seriesTarget(s db.Series) release.SeriesTarget {
	return release.SeriesTarget{Title: s.Title, OriginalTitle: s.OriginalTitle, Year: int(s.Year)}
}

// tvQuery builds the best query the indexer supports for a season (episode
// nil) or a single episode: a TVDB-id lookup when possible, else text.
func tvQuery(s db.Series, ix indexers.Indexer, season int, episode *int) (indexer.Query, bool) {
	text := fmt.Sprintf("%s S%02d", s.Title, season)
	if episode != nil {
		text = fmt.Sprintf("%s S%02dE%02d", s.Title, season, *episode)
	}
	q := indexer.Query{Categories: ix.TVCategories}
	caps := ix.Caps
	switch {
	case caps == nil:
		q.Type, q.Q = indexer.SearchGeneric, text
	case caps.TVSearch.Available && caps.TVSearch.Supports("season") && (episode == nil || caps.TVSearch.Supports("ep")):
		q.Type, q.Season, q.Episode = indexer.SearchTV, &season, episode
		if s.TVDBID != nil && caps.TVSearch.Supports("tvdbid") {
			q.TVDBID = *s.TVDBID
		} else {
			q.Q = s.Title
		}
	case caps.TVSearch.Available:
		q.Type, q.Q = indexer.SearchTV, text
	case caps.Search.Available:
		q.Type, q.Q = indexer.SearchGeneric, text
	default:
		return indexer.Query{}, false
	}
	return q, true
}

// SearchEpisode searches all indexers for one episode. Season packs are
// rejected: use SearchSeason for those.
func (s *Service) SearchEpisode(ctx context.Context, episodeID int64) (SearchResult, error) {
	ep, series, err := s.episodeWithSeries(ctx, episodeID)
	if err != nil {
		return SearchResult{}, err
	}
	return s.searchEpisode(ctx, series, int(ep.Season), int(ep.Episode))
}

// searchEpisode searches for one episode of series, which need not be in the
// library: only its titles, year and TVDB id are used.
func (s *Service) searchEpisode(ctx context.Context, series db.Series, season, number int) (SearchResult, error) {
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	found, failures, err := s.indexers.Search(ctx, func(ix indexers.Indexer) (indexer.Query, bool) {
		return tvQuery(series, ix, season, &number)
	})
	if err != nil {
		return SearchResult{}, err
	}
	target, limits := seriesTarget(series), q.Limits()
	want := release.EpisodeWant{Season: season, Episodes: []int{number}}
	return rankReleases(found, failures, func(c release.Candidate) release.Decision {
		return release.EvaluateEpisode(c, target, want, limits)
	}), nil
}

// SearchSeason searches all indexers for a season. Season packs and
// releases containing any wanted episode of the season are accepted.
func (s *Service) SearchSeason(ctx context.Context, seriesID int64, season int) (SearchResult, error) {
	series, err := s.store.GetSeries(ctx, seriesID)
	if err != nil {
		return SearchResult{}, apperr.NotFound(err, "series", seriesID)
	}
	eps, err := s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: seriesID, Season: int64(season)})
	if err != nil {
		return SearchResult{}, fmt.Errorf("list episodes: %w", err)
	}
	if len(eps) == 0 {
		return SearchResult{}, fmt.Errorf("series %d season %d: %w", seriesID, season, apperr.ErrNotFound)
	}
	want := release.EpisodeWant{Season: season, AllowPack: true, PackEpisodes: len(eps)}
	for _, e := range eps {
		if e.FilePath == nil {
			want.Episodes = append(want.Episodes, int(e.Episode))
		}
	}
	return s.searchSeason(ctx, series, want)
}

// searchSeason searches for a season of series, which need not be in the
// library: only its titles, year and TVDB id are used. want says which
// episodes are wanted.
func (s *Service) searchSeason(ctx context.Context, series db.Series, want release.EpisodeWant) (SearchResult, error) {
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	season := want.Season
	found, failures, err := s.indexers.Search(ctx, func(ix indexers.Indexer) (indexer.Query, bool) {
		return tvQuery(series, ix, season, nil)
	})
	if err != nil {
		return SearchResult{}, err
	}
	target, limits := seriesTarget(series), q.Limits()
	return rankReleases(found, failures, func(c release.Candidate) release.Decision {
		return release.EvaluateEpisode(c, target, want, limits)
	}), nil
}

func (s *Service) episodeWithSeries(ctx context.Context, episodeID int64) (db.Episode, db.Series, error) {
	ep, err := s.store.GetEpisode(ctx, episodeID)
	if err != nil {
		return db.Episode{}, db.Series{}, apperr.NotFound(err, "episode", episodeID)
	}
	series, err := s.store.GetSeries(ctx, ep.SeriesID)
	if err != nil {
		return db.Episode{}, db.Series{}, apperr.NotFound(err, "series", ep.SeriesID)
	}
	return ep, series, nil
}

// GrabEpisodeRelease downloads r for the series that episodeID belongs to.
func (s *Service) GrabEpisodeRelease(ctx context.Context, episodeID int64, r ReleaseResult) (Download, error) {
	ep, _, err := s.episodeWithSeries(ctx, episodeID)
	if err != nil {
		return Download{}, err
	}
	return s.GrabSeriesRelease(ctx, ep.SeriesID, r)
}

// GrabSeriesRelease downloads a TV release for a series. The episodes it is
// expected to contain come from its name: listed episodes, or every episode
// without a file in the seasons a pack covers. Multi-season packs are not
// expected to contain specials.
func (s *Service) GrabSeriesRelease(ctx context.Context, seriesID int64, r ReleaseResult) (Download, error) {
	if _, err := s.store.GetSeries(ctx, seriesID); err != nil {
		return Download{}, apperr.NotFound(err, "series", seriesID)
	}
	info := release.Parse(r.Title)
	if !info.IsTV {
		return Download{}, apperr.Invalid("title", "%q is not a TV release", r.Title)
	}
	var eps []db.Episode
	var err error
	if info.MultiSeason {
		eps, err = s.store.ListEpisodes(ctx, seriesID)
	} else {
		eps, err = s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: seriesID, Season: int64(info.Season)})
	}
	if err != nil {
		return Download{}, fmt.Errorf("list episodes: %w", err)
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
		return Download{}, apperr.Invalid("title", "%q matches no episodes of this series that need a file", r.Title)
	}
	return s.grab(ctx, grabTarget{seriesID: &seriesID, episodeIDs: ids}, r)
}

// AutoSearchEpisode searches for one episode and grabs the best release.
func (s *Service) AutoSearchEpisode(ctx context.Context, episodeID int64) (AutoSearchResult, error) {
	ep, _, err := s.episodeWithSeries(ctx, episodeID)
	if err != nil {
		return AutoSearchResult{}, err
	}
	res, err := s.SearchEpisode(ctx, episodeID)
	if err != nil {
		return AutoSearchResult{}, err
	}
	out := AutoSearchResult{Searched: len(res.Releases), Failures: res.Failures}
	out.Grabbed, err = s.grabBest(ctx, res, func(r ReleaseResult) (Download, error) {
		return s.GrabSeriesRelease(ctx, ep.SeriesID, r)
	})
	return out, err
}

// AutoSearchSeason grabs what a season still needs. When the whole season
// has aired and at least two episodes are wanted, a season pack is tried
// first; anything still missing is then searched episode by episode.
func (s *Service) AutoSearchSeason(ctx context.Context, seriesID int64, season int) ([]AutoSearchResult, error) {
	wanted, fullyAired, err := s.wantedInSeason(ctx, seriesID, season)
	if err != nil {
		return nil, err
	}
	var results []AutoSearchResult
	if fullyAired && len(wanted) >= 2 {
		res, err := s.SearchSeason(ctx, seriesID, season)
		if err != nil {
			return nil, err
		}
		packs := SearchResult{Failures: res.Failures}
		for _, r := range res.Releases {
			if r.SeasonPack {
				packs.Releases = append(packs.Releases, r)
			}
		}
		out := AutoSearchResult{Searched: len(packs.Releases), Failures: res.Failures}
		out.Grabbed, err = s.grabBest(ctx, packs, func(r ReleaseResult) (Download, error) {
			return s.GrabSeriesRelease(ctx, seriesID, r)
		})
		if err != nil {
			s.log.Warn("season pack grab", "series", seriesID, "season", season, "err", err)
		}
		results = append(results, out)
		if out.Grabbed != nil {
			return results, nil
		}
	}

	for _, ep := range wanted {
		// An earlier grab in this loop (e.g. a multi-episode release) may
		// already cover this episode.
		still, _, err := s.wantedInSeason(ctx, seriesID, season)
		if err != nil {
			return results, err
		}
		if !containsEpisode(still, ep.ID) {
			continue
		}
		res, err := s.AutoSearchEpisode(ctx, ep.ID)
		if err != nil {
			s.log.Warn("episode search", "series", seriesID, "episode", fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode), "err", err)
		}
		results = append(results, res)
	}
	return results, nil
}

// wantedInSeason returns the season's wanted episodes, and whether every
// episode of the season has aired.
func (s *Service) wantedInSeason(ctx context.Context, seriesID int64, season int) ([]db.Episode, bool, error) {
	eps, err := s.store.ListSeasonEpisodes(ctx, db.ListSeasonEpisodesParams{SeriesID: seriesID, Season: int64(season)})
	if err != nil {
		return nil, false, fmt.Errorf("list episodes: %w", err)
	}
	active, err := s.media.ActiveEpisodeIDs(ctx)
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
		if e.Monitored && e.FilePath == nil && !active[e.ID] {
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
	wanted, err := s.store.ListWantedEpisodes(ctx, s.media.Today())
	if err != nil {
		return fmt.Errorf("list wanted episodes: %w", err)
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
			return ctx.Err()
		}
		if _, err := s.AutoSearchSeason(ctx, k.series, k.season); err != nil {
			s.log.Warn("wanted season search", "series", k.series, "season", k.season, "err", err)
		}
	}
	return nil
}
