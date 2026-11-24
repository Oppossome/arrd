package acquire

import (
	"context"
	"fmt"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// RSSResult reports what an RSS sync did.
type RSSResult struct {
	Releases int                       `json:"releases_seen"`
	Grabbed  []Download                `json:"grabbed"`
	Failures []indexers.IndexerFailure `json:"indexer_failures"`
}

// rssCandidate is an accepted RSS release and what it would satisfy.
type rssCandidate struct {
	release    ReleaseResult
	movieID    int64   // set for movies
	seriesID   int64   // set for TV
	episodeIDs []int64 // wanted episodes it covers
}

// RSSSync reads the recent-releases feed of every RSS-enabled indexer and
// grabs releases for wanted movies and episodes.
func (s *Service) RSSSync(ctx context.Context) (RSSResult, error) {
	rows, err := s.store.ListRSSIndexers(ctx)
	if err != nil {
		return RSSResult{}, fmt.Errorf("list indexers: %w", err)
	}
	catalog := s.indexers.Catalog()
	found, failures := s.indexers.Query(ctx, rows, func(ix indexers.Indexer) (indexer.Query, bool) {
		if k, err := catalog.Get(ix.Definition); err == nil && !k.RSS {
			return indexer.Query{}, false // cannot list recent releases
		}
		cats := append(append([]int{}, ix.MovieCategories...), ix.TVCategories...)
		return indexer.Query{Type: indexer.SearchGeneric, Categories: cats}, true
	})
	out := RSSResult{Releases: len(found), Failures: failures, Grabbed: []Download{}}
	if out.Failures == nil {
		out.Failures = []indexers.IndexerFailure{}
	}
	if len(found) == 0 {
		return out, nil
	}

	matcher, err := s.newWantedMatcher(ctx)
	if err != nil {
		return out, err
	}
	var candidates []release.Ranked[rssCandidate]
	for _, f := range found {
		for _, c := range matcher.match(f) {
			candidates = append(candidates, c)
		}
	}
	release.Rank(candidates)

	// Grab greedily, best first, skipping anything already covered by an
	// earlier grab in this pass.
	grabbedMovies := map[int64]bool{}
	grabbedEpisodes := map[int64]bool{}
	for _, c := range candidates {
		cand := c.Item
		if cand.movieID != 0 {
			if grabbedMovies[cand.movieID] {
				continue
			}
			dl, err := s.GrabMovieRelease(ctx, cand.movieID, cand.release)
			if err != nil {
				s.log.Warn().Str("release", cand.release.Title).Err(err).Msg("rss grab")
				continue
			}
			grabbedMovies[cand.movieID] = true
			out.Grabbed = append(out.Grabbed, dl)
			continue
		}
		if anyGrabbed(cand.episodeIDs, grabbedEpisodes) {
			continue
		}
		dl, err := s.GrabSeriesRelease(ctx, cand.seriesID, cand.release)
		if err != nil {
			s.log.Warn().Str("release", cand.release.Title).Err(err).Msg("rss grab")
			continue
		}
		for _, id := range cand.episodeIDs {
			grabbedEpisodes[id] = true
		}
		out.Grabbed = append(out.Grabbed, dl)
	}
	if len(out.Grabbed) > 0 {
		s.log.Info().Int("count", len(out.Grabbed)).Int("seen", out.Releases).Msg("rss sync grabbed releases")
	}
	return out, nil
}

func anyGrabbed(ids []int64, grabbed map[int64]bool) bool {
	for _, id := range ids {
		if grabbed[id] {
			return true
		}
	}
	return false
}

// wantedMatcher matches feed releases against what the library wants.
type wantedMatcher struct {
	limits release.Limits
	movies []db.Movie
	series []db.Series
	// wanted episodes by series, then season
	wanted map[int64]map[int][]db.Episode
	// seasons in which every episode has aired, by series
	fullyAired map[int64]map[int]bool
	seasonSize map[int64]map[int]int
}

func (s *Service) newWantedMatcher(ctx context.Context) (*wantedMatcher, error) {
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return nil, err
	}
	movies, err := s.store.ListWantedMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list wanted movies: %w", err)
	}
	wantedEps, err := s.store.ListWantedEpisodes(ctx, s.media.Today())
	if err != nil {
		return nil, fmt.Errorf("list wanted episodes: %w", err)
	}
	m := &wantedMatcher{
		limits: q.Limits(), movies: movies,
		wanted: map[int64]map[int][]db.Episode{}, fullyAired: map[int64]map[int]bool{}, seasonSize: map[int64]map[int]int{},
	}
	for _, e := range wantedEps {
		if m.wanted[e.SeriesID] == nil {
			m.wanted[e.SeriesID] = map[int][]db.Episode{}
		}
		m.wanted[e.SeriesID][int(e.Season)] = append(m.wanted[e.SeriesID][int(e.Season)], e)
	}
	today := s.media.Today()
	for seriesID := range m.wanted {
		series, err := s.store.GetSeries(ctx, seriesID)
		if err != nil {
			return nil, fmt.Errorf("load series %d: %w", seriesID, err)
		}
		m.series = append(m.series, series)
		all, err := s.store.ListEpisodes(ctx, seriesID)
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		m.fullyAired[seriesID], m.seasonSize[seriesID] = map[int]bool{}, map[int]int{}
		for _, e := range all {
			season := int(e.Season)
			if _, seen := m.fullyAired[seriesID][season]; !seen {
				m.fullyAired[seriesID][season] = true
			}
			if !media.Aired(e, today) {
				m.fullyAired[seriesID][season] = false
			}
			m.seasonSize[seriesID][season]++
		}
	}
	return m, nil
}

func (m *wantedMatcher) match(f indexers.FoundRelease) []release.Ranked[rssCandidate] {
	info := release.Parse(f.Title)
	cand := release.Candidate{Info: info, Size: f.Size, Seeders: f.Seeders}
	if int64(f.Seeders) < f.MinSeeders {
		return nil
	}
	rel := ReleaseResult{
		Title: f.Title, IndexerID: f.IndexerID, Indexer: f.Indexer, DownloadURL: f.DownloadURL,
		MagnetURI: f.MagnetURI, InfoHash: f.InfoHash, Size: f.Size, Seeders: f.Seeders, Peers: f.Peers,
		PublishDate: f.PublishDate, Quality: info.Quality(), Accepted: true,
	}
	var out []release.Ranked[rssCandidate]
	if !info.IsTV {
		for _, mv := range m.movies {
			d := release.EvaluateMovie(cand, release.MovieTarget{Title: mv.Title, OriginalTitle: mv.OriginalTitle, Year: int(mv.Year)}, m.limits)
			if d.Accepted {
				rel.Score = d.Score
				out = append(out, release.Ranked[rssCandidate]{Item: rssCandidate{release: rel, movieID: mv.ID}, Decision: d, Seeders: f.Seeders})
			}
		}
		return out
	}
	for _, sr := range m.series {
		if ok, _ := release.MatchesSeries(info, seriesTarget(sr)); !ok {
			continue
		}
		wanted := m.wanted[sr.ID][info.Season]
		if len(wanted) == 0 {
			continue
		}
		want := release.EpisodeWant{
			Season:       info.Season,
			AllowPack:    m.fullyAired[sr.ID][info.Season] && len(wanted) >= 2,
			PackEpisodes: m.seasonSize[sr.ID][info.Season],
		}
		var covered []int64
		for _, e := range wanted {
			want.Episodes = append(want.Episodes, int(e.Episode))
			if info.Covers(int(e.Season), int(e.Episode)) {
				covered = append(covered, e.ID)
			}
		}
		d := release.EvaluateEpisode(cand, seriesTarget(sr), want, m.limits)
		if !d.Accepted || len(covered) == 0 {
			continue
		}
		rel.Score = d.Score
		out = append(out, release.Ranked[rssCandidate]{
			Item:     rssCandidate{release: rel, seriesID: sr.ID, episodeIDs: covered},
			Decision: d, Seeders: f.Seeders,
		})
	}
	return out
}
