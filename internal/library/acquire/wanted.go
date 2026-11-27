package acquire

import (
	"context"
	"fmt"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/release"
)

// wanted is something releases are searched for and grabbed for: a movie,
// a collection, a series, a season or an episode. Everything that differs
// between them lives behind this interface; searching, ranking, grabbing
// and auto-searching are shared.
type wanted interface {
	// text is arrd's own text query, such as "Title 1999" or "Title S01".
	text() string
	// query is the best query the indexer supports; false skips it.
	query(ix indexers.Indexer) (indexer.Query, bool)
	// tv says which categories a user's own text is searched in.
	tv() bool
	evaluate(c release.Candidate, lim release.Limits) release.Decision
	// claim says what grabbing a release is for: the library items it is
	// expected to contain. It needs the title in the library.
	claim(ctx context.Context, info release.Info) (grabTarget, error)
}

// AutoSearchResult reports what an automatic search did.
type AutoSearchResult struct {
	Grabbed  []Download                `json:"grabbed"`
	Searched int                       `json:"releases_considered"`
	Failures []indexers.IndexerFailure `json:"indexer_failures"`
}

func (a *AutoSearchResult) add(b AutoSearchResult) {
	a.Grabbed = append(a.Grabbed, b.Grabbed...)
	a.Searched += b.Searched
	a.Failures = append(a.Failures, b.Failures...)
}

// Search queries all search-enabled indexers for t and ranks the results,
// best acceptable release first. text replaces arrd's own query when not
// blank.
func (s *Service) Search(ctx context.Context, t Target, text string) (SearchResult, error) {
	w, err := s.resolve(ctx, t)
	if err != nil {
		return SearchResult{}, err
	}
	return s.searchWanted(ctx, w, text)
}

// Grab downloads r for t, first adding t's movie or series to the library
// (unmonitored, in the default root folder) if it is not there. r normally
// comes from Search; the indexer it names supplies the network for
// fetching the .torrent file.
func (s *Service) Grab(ctx context.Context, t Target, r ReleaseResult) (Download, error) {
	if err := s.ensureInLibrary(ctx, t); err != nil {
		return Download{}, err
	}
	w, err := s.resolve(ctx, t)
	if err != nil {
		return Download{}, err
	}
	return s.grabFor(ctx, w, r)
}

// AutoSearch searches for t, which must be in the library, and grabs the
// best acceptable release. A season tries a season pack first, then each
// episode still wanted; a series does that for every season with wanted
// episodes.
func (s *Service) AutoSearch(ctx context.Context, t Target) (AutoSearchResult, error) {
	switch t.Kind {
	case KindSeason, KindSeries:
		if err := t.Validate(); err != nil {
			return AutoSearchResult{}, err
		}
		series, err := s.store.GetSeriesByTMDBID(ctx, t.TMDBID)
		if err != nil {
			return AutoSearchResult{}, apperr.NotFound(err, "library series with TMDB id", t.TMDBID)
		}
		if t.Kind == KindSeason {
			return s.autoSearchSeason(ctx, series.ID, *t.Season)
		}
		return s.searchWantedEpisodes(ctx, &series.ID)
	}
	w, err := s.libraryWant(ctx, t)
	if err != nil {
		return AutoSearchResult{}, err
	}
	return s.autoSearch(ctx, w, nil)
}

// searchWanted searches for w. text replaces arrd's own query when not
// blank.
func (s *Service) searchWanted(ctx context.Context, w wanted, text string) (SearchResult, error) {
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	text, custom := searchText(text, w.text())
	found, failures, err := s.indexers.Search(ctx, func(ix indexers.Indexer) (indexer.Query, bool) {
		if custom {
			return textQuery(text, ix, w.tv())
		}
		return w.query(ix)
	})
	if err != nil {
		return SearchResult{}, err
	}
	limits := q.Limits()
	res := rankReleases(found, failures, func(c release.Candidate) release.Decision {
		return w.evaluate(c, limits)
	})
	res.Query = text
	return res, nil
}

// grabFor downloads r for what w claims it contains.
func (s *Service) grabFor(ctx context.Context, w wanted, r ReleaseResult) (Download, error) {
	target, err := w.claim(ctx, parsed(r))
	if err != nil {
		return Download{}, err
	}
	return s.grab(ctx, target, r)
}

// autoSearch searches for w and grabs the best acceptable release that
// keep allows (nil: any), trying the next one if a grab fails.
func (s *Service) autoSearch(ctx context.Context, w wanted, keep func(ReleaseResult) bool) (AutoSearchResult, error) {
	res, err := s.searchWanted(ctx, w, "")
	if err != nil {
		return AutoSearchResult{}, err
	}
	if keep != nil {
		kept := res.Releases[:0:0]
		for _, r := range res.Releases {
			if keep(r) {
				kept = append(kept, r)
			}
		}
		res.Releases = kept
	}
	out := AutoSearchResult{Grabbed: []Download{}, Searched: len(res.Releases), Failures: res.Failures}
	dl, err := s.grabBest(ctx, res, func(r ReleaseResult) (Download, error) { return s.grabFor(ctx, w, r) })
	if dl != nil {
		out.Grabbed = append(out.Grabbed, *dl)
	}
	if err == nil && dl == nil {
		s.log.Info().Str("query", w.text()).Int("considered", len(res.Releases)).Int("indexer_failures", len(res.Failures)).Msg("no acceptable release found")
	}
	return out, err
}

// grabBest grabs the first accepted release in ranked results, trying the
// next one when a grab fails.
func (s *Service) grabBest(ctx context.Context, res SearchResult, grab func(ReleaseResult) (Download, error)) (*Download, error) {
	var grabErrs []error
	for _, r := range res.Releases {
		if !r.Accepted {
			break // ranked: everything after is rejected too
		}
		dl, err := grab(r)
		if err != nil {
			s.log.Warn().Str("release", r.Title).Err(err).Msg("grab failed, trying next release")
			grabErrs = append(grabErrs, fmt.Errorf("%s: %w", r.Title, err))
			continue
		}
		return &dl, nil
	}
	if len(grabErrs) > 0 {
		return nil, fmt.Errorf("every acceptable release failed to grab: %v", grabErrs)
	}
	return nil, nil
}
