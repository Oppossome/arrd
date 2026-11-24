package media

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"generic-arr-project/internal/metadata"
)

// Recommendations are TMDB's "if you liked this" lists, one row per title
// recently added to the library. Titles already in the library, and titles
// an earlier row already shows, are left out.

// recommendationSeeds is how many of the most recently added titles get a row.
const recommendationSeeds = 6

// RecommendationSource is the library title a row of recommendations is for.
type RecommendationSource struct {
	TMDBID int64
	Title  string
}

// RecommendationRow is what TMDB recommends to people who liked Because.
type RecommendationRow[T any] struct {
	Because RecommendationSource
	Items   []T
}

// MovieRecommendations returns a row of recommended movies for each of the
// most recently added library movies, newest first. Rows left empty are
// dropped.
func (s *Service) MovieRecommendations(ctx context.Context) ([]RecommendationRow[metadata.Movie], error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	owned := make(map[int64]bool, len(rows))
	seeds := make([]seed, 0, len(rows))
	for _, r := range rows {
		owned[r.TMDBID] = true
		seeds = append(seeds, seed{RecommendationSource{r.TMDBID, r.Title}, r.AddedAt, r.ID})
	}
	return recommend(ctx, newestSeeds(seeds), owned, tm.MovieRecommendations,
		func(m metadata.Movie) int64 { return m.TMDBID })
}

// SeriesRecommendations returns a row of recommended series for each of the
// most recently added library series, newest first. Rows left empty are
// dropped.
func (s *Service) SeriesRecommendations(ctx context.Context) ([]RecommendationRow[metadata.Series], error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListSeries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	owned := make(map[int64]bool, len(rows))
	seeds := make([]seed, 0, len(rows))
	for _, r := range rows {
		owned[r.TMDBID] = true
		seeds = append(seeds, seed{RecommendationSource{r.TMDBID, r.Title}, r.AddedAt, r.ID})
	}
	return recommend(ctx, newestSeeds(seeds), owned, tm.SeriesRecommendations,
		func(sr metadata.Series) int64 { return sr.TMDBID })
}

// seed is a library title that recommendations may be based on.
type seed struct {
	source  RecommendationSource
	addedAt string // RFC 3339 UTC, so it sorts as text
	id      int64  // breaks ties between titles added in the same second
}

// newestSeeds returns the sources of the most recently added titles.
func newestSeeds(seeds []seed) []RecommendationSource {
	sort.Slice(seeds, func(i, j int) bool {
		if seeds[i].addedAt != seeds[j].addedAt {
			return seeds[i].addedAt > seeds[j].addedAt
		}
		return seeds[i].id > seeds[j].id
	})
	out := make([]RecommendationSource, 0, recommendationSeeds)
	for _, sd := range seeds[:min(len(seeds), recommendationSeeds)] {
		out = append(out, sd.source)
	}
	return out
}

// recommend fetches the recommendations for every source at once and keeps,
// in source order, the titles not owned and not already shown. A source
// TMDB no longer knows gets no row; any other TMDB error fails the call.
func recommend[T any](
	ctx context.Context,
	sources []RecommendationSource,
	owned map[int64]bool,
	fetch func(context.Context, int64) ([]T, error),
	idOf func(T) int64,
) ([]RecommendationRow[T], error) {
	lists := make([][]T, len(sources))
	errs := make([]error, len(sources))
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Go(func() { lists[i], errs[i] = fetch(ctx, src.TMDBID) })
	}
	wg.Wait()

	shown := map[int64]bool{}
	out := []RecommendationRow[T]{}
	for i, src := range sources {
		if errors.Is(errs[i], metadata.ErrNotFound) {
			continue
		}
		if errs[i] != nil {
			return nil, fmt.Errorf("recommendations for %q: %w", src.Title, errs[i])
		}
		var items []T
		for _, it := range lists[i] {
			id := idOf(it)
			if owned[id] || shown[id] {
				continue
			}
			shown[id] = true
			items = append(items, it)
		}
		if len(items) > 0 {
			out = append(out, RecommendationRow[T]{Because: src, Items: items})
		}
	}
	return out, nil
}
