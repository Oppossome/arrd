package acquire

import (
	"context"
	"fmt"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// SearchSeriesPacks searches all indexers for releases of the whole series:
// multi-season and "Complete Series" packs (or, for a one-season series,
// that season's pack). text replaces arrd's own query when not blank.
func (s *Service) SearchSeriesPacks(ctx context.Context, seriesID int64, text string) (SearchResult, error) {
	series, err := s.store.GetSeries(ctx, seriesID)
	if err != nil {
		return SearchResult{}, apperr.NotFound(err, "series", seriesID)
	}
	eps, err := s.store.ListEpisodes(ctx, seriesID)
	if err != nil {
		return SearchResult{}, fmt.Errorf("list episodes: %w", err)
	}
	counts := map[int]int{}
	for _, e := range eps {
		if e.Season > 0 {
			counts[int(e.Season)]++
		}
	}
	return s.searchSeriesPacks(ctx, series, release.SeriesPackWant{SeasonEpisodes: counts}, text)
}

// searchSeriesPacks searches for whole-series releases of series, which
// need not be in the library: only its titles, year and TVDB id are used.
// text replaces arrd's own query when not blank.
func (s *Service) searchSeriesPacks(ctx context.Context, series db.Series, want release.SeriesPackWant, text string) (SearchResult, error) {
	if len(want.SeasonEpisodes) == 0 {
		return SearchResult{}, fmt.Errorf("series %q has no regular seasons: %w", series.Title, apperr.ErrNotFound)
	}
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	text, custom := searchText(text, series.Title)
	found, failures, err := s.indexers.Search(ctx, func(ix indexers.Indexer) (indexer.Query, bool) {
		if custom {
			return textQuery(text, ix, true)
		}
		return seriesPackQuery(series, ix)
	})
	if err != nil {
		return SearchResult{}, err
	}
	target, limits := seriesTarget(series), q.Limits()
	res := rankReleases(found, failures, func(c release.Candidate) release.Decision {
		return release.EvaluateSeriesPack(c, target, want, limits)
	})
	res.Query = text
	return res, nil
}

// seriesPackQuery searches by title alone: whole-series packs carry no
// single season or episode number to search by, and an id lookup tends to
// return only recent episodes.
func seriesPackQuery(s db.Series, ix indexers.Indexer) (indexer.Query, bool) {
	q := indexer.Query{Categories: ix.TVCategories, Q: s.Title}
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
