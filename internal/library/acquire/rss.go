package acquire

import (
	"context"
	"fmt"
	"maps"
	"slices"

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
		candidates = append(candidates, matcher.match(f)...)
	}
	release.Rank(candidates)

	// Grab greedily, best first, skipping anything already covered by an
	// earlier grab in this pass.
	grabbed := map[downloadItem]bool{}
	for _, c := range candidates {
		target, err := c.Item.want.claim(ctx, parsed(c.Item.release))
		if err != nil {
			s.log.Warn().Str("release", c.Item.release.Title).Err(err).Msg("rss grab")
			continue
		}
		if target.coversAny(grabbed) {
			continue
		}
		dl, err := s.grab(ctx, target, c.Item.release)
		if err != nil {
			s.log.Warn().Str("release", c.Item.release.Title).Err(err).Msg("rss grab")
			continue
		}
		target.markIn(grabbed)
		out.Grabbed = append(out.Grabbed, dl)
	}
	if len(out.Grabbed) > 0 {
		s.log.Info().Int("count", len(out.Grabbed)).Int("seen", out.Releases).Msg("rss sync grabbed releases")
	}
	return out, nil
}

// downloadItem is a movie or an episode a download brings in.
type downloadItem struct {
	movie bool
	id    int64
}

func (t grabTarget) items() []downloadItem {
	out := make([]downloadItem, 0, len(t.movieIDs)+len(t.episodeIDs))
	for _, id := range t.movieIDs {
		out = append(out, downloadItem{movie: true, id: id})
	}
	for _, id := range t.episodeIDs {
		out = append(out, downloadItem{id: id})
	}
	return out
}

func (t grabTarget) coversAny(set map[downloadItem]bool) bool {
	for _, it := range t.items() {
		if set[it] {
			return true
		}
	}
	return false
}

func (t grabTarget) markIn(set map[downloadItem]bool) {
	for _, it := range t.items() {
		set[it] = true
	}
}

// wantedMatcher matches feed releases against everything the library
// wants: each wanted movie, and each season with wanted episodes.
type wantedMatcher struct {
	limits release.Limits
	wants  []wanted
}

func (s *Service) newWantedMatcher(ctx context.Context) (*wantedMatcher, error) {
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return nil, err
	}
	m := &wantedMatcher{limits: q.Limits()}
	movies, err := s.store.ListWantedMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list wanted movies: %w", err)
	}
	for _, mv := range movies {
		m.wants = append(m.wants, movieWant{s: s, movie: mv})
	}
	seasons, err := s.wantedSeasons(ctx)
	if err != nil {
		return nil, err
	}
	m.wants = append(m.wants, seasons...)
	return m, nil
}

// wantedSeasons is a want per season with wanted episodes. A season pack
// is accepted only once every episode of the season has aired and at
// least two are wanted.
func (s *Service) wantedSeasons(ctx context.Context) ([]wanted, error) {
	today := s.media.Today()
	wantedEps, err := s.store.ListWantedEpisodes(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("list wanted episodes: %w", err)
	}
	bySeries := map[int64]map[int][]db.Episode{}
	var seriesOrder []int64
	for _, e := range wantedEps {
		if bySeries[e.SeriesID] == nil {
			bySeries[e.SeriesID] = map[int][]db.Episode{}
			seriesOrder = append(seriesOrder, e.SeriesID)
		}
		bySeries[e.SeriesID][int(e.Season)] = append(bySeries[e.SeriesID][int(e.Season)], e)
	}
	var out []wanted
	for _, seriesID := range seriesOrder {
		series, err := s.store.GetSeries(ctx, seriesID)
		if err != nil {
			return nil, fmt.Errorf("load series %d: %w", seriesID, err)
		}
		all, err := s.store.ListEpisodes(ctx, seriesID)
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		fullyAired, size := map[int]bool{}, map[int]int{}
		for _, e := range all {
			season := int(e.Season)
			if _, seen := fullyAired[season]; !seen {
				fullyAired[season] = true
			}
			if !media.Aired(e, today) {
				fullyAired[season] = false
			}
			size[season]++
		}
		for _, season := range slices.Sorted(maps.Keys(bySeries[seriesID])) {
			eps := bySeries[seriesID][season]
			want := release.EpisodeWant{
				Season:       season,
				AllowPack:    fullyAired[season] && len(eps) >= 2,
				PackEpisodes: size[season],
			}
			for _, e := range eps {
				want.Episodes = append(want.Episodes, int(e.Episode))
			}
			out = append(out, episodesWant{s: s, series: series, want: want})
		}
	}
	return out, nil
}

// rssCandidate is an accepted RSS release and what it was accepted for.
type rssCandidate struct {
	release ReleaseResult
	want    wanted
}

func (m *wantedMatcher) match(f indexers.FoundRelease) []release.Ranked[rssCandidate] {
	if int64(f.Seeders) < f.MinSeeders {
		return nil
	}
	info := release.Parse(f.Title)
	cand := release.Candidate{Info: info, Size: f.Size, Seeders: f.Seeders}
	var out []release.Ranked[rssCandidate]
	for _, w := range m.wants {
		d := w.evaluate(cand, m.limits)
		if !d.Accepted {
			continue
		}
		rel := ReleaseResult{
			Title: f.Title, IndexerID: f.IndexerID, Indexer: f.Indexer, DownloadURL: f.DownloadURL,
			MagnetURI: f.MagnetURI, InfoHash: f.InfoHash, Size: f.Size, Seeders: f.Seeders, Peers: f.Peers,
			PublishDate: f.PublishDate, Quality: info.Quality(), Accepted: true, Score: d.Score,
		}
		out = append(out, release.Ranked[rssCandidate]{Item: rssCandidate{release: rel, want: w}, Decision: d, Seeders: f.Seeders})
	}
	return out
}
