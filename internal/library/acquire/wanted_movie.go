package acquire

import (
	"context"
	"fmt"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// movieWant is one movie. Only its titles, year and IMDb id are searched
// by, so it need not be in the library until it is grabbed.
type movieWant struct {
	s     *Service
	movie db.Movie
}

func (w movieWant) text() string { return movieText(w.movie) }
func (w movieWant) tv() bool     { return false }

// query is an IMDb lookup if the indexer supports one, otherwise a
// title + year text search.
func (w movieWant) query(ix indexers.Indexer) (indexer.Query, bool) {
	m := w.movie
	q := indexer.Query{Categories: ix.MovieCategories}
	switch caps := ix.Caps; {
	case caps == nil:
		// Not tested yet: a generic text search works everywhere.
		q.Type, q.Q = indexer.SearchGeneric, movieText(m)
	case caps.MovieSearch.Available && m.IMDBID != nil && caps.MovieSearch.Supports("imdbid"):
		q.Type, q.IMDBID = indexer.SearchMovie, *m.IMDBID
	case caps.MovieSearch.Available:
		q.Type, q.Q = indexer.SearchMovie, movieText(m)
	case caps.Search.Available:
		q.Type, q.Q = indexer.SearchGeneric, movieText(m)
	default:
		return indexer.Query{}, false
	}
	return q, true
}

func (w movieWant) evaluate(c release.Candidate, lim release.Limits) release.Decision {
	return release.EvaluateMovie(c, movieTargetOf(w.movie), lim)
}

// claim refuses a multi-movie pack: it would be imported as this one movie.
func (w movieWant) claim(_ context.Context, info release.Info) (grabTarget, error) {
	if w.movie.ID == 0 {
		return grabTarget{}, fmt.Errorf("movie %q is not in the library: %w", w.movie.Title, apperr.ErrNotFound)
	}
	if info.MoviePack && !release.MatchesMovieTitle(info, movieTargetOf(w.movie)) {
		return grabTarget{}, apperr.Invalid("title", "%q is a multi-movie pack; download it from the collection instead", info.Raw)
	}
	id := w.movie.ID
	return grabTarget{movieID: &id, movieIDs: []int64{id}}, nil
}

func movieTargetOf(m db.Movie) release.MovieTarget {
	return release.MovieTarget{Title: m.Title, OriginalTitle: m.OriginalTitle, Year: int(m.Year)}
}

// movieText is a text search for m: its title and year.
func movieText(m db.Movie) string {
	if m.Year > 0 {
		return fmt.Sprintf("%s %d", m.Title, m.Year)
	}
	return m.Title
}

// collectionWant is multi-movie packs of a TMDB collection, searched for
// from anchor, one of its movies. Grabbing adds the collection's released
// movies to the library, unmonitored, in anchor's root folder.
type collectionWant struct {
	s      *Service
	anchor db.Movie
	coll   *metadata.Collection
}

// text is the franchise name alone: a pack has no single IMDb id or year.
func (w collectionWant) text() string { return release.FranchiseName(w.coll.Name) }
func (w collectionWant) tv() bool     { return false }

func (w collectionWant) query(ix indexers.Indexer) (indexer.Query, bool) {
	q := indexer.Query{Categories: ix.MovieCategories, Q: w.text()}
	switch caps := ix.Caps; {
	case caps == nil:
		q.Type = indexer.SearchGeneric
	case caps.MovieSearch.Available:
		q.Type = indexer.SearchMovie
	case caps.Search.Available:
		q.Type = indexer.SearchGeneric
	default:
		return indexer.Query{}, false
	}
	return q, true
}

func (w collectionWant) evaluate(c release.Candidate, lim release.Limits) release.Decision {
	t := release.CollectionTarget{Name: w.coll.Name, Parts: make([]release.MovieTarget, len(w.coll.Parts))}
	for i, p := range w.coll.Parts {
		t.Parts[i] = release.MovieTarget{Title: p.Title, OriginalTitle: p.OriginalTitle, Year: p.Year}
	}
	return release.EvaluateMoviePack(c, t, lim)
}

func (w collectionWant) claim(ctx context.Context, info release.Info) (grabTarget, error) {
	if w.anchor.ID == 0 {
		return grabTarget{}, fmt.Errorf("movie %q is not in the library: %w", w.anchor.Title, apperr.ErrNotFound)
	}
	if !info.MoviePack {
		return grabTarget{}, apperr.Invalid("title", "%q is not a multi-movie pack", info.Raw)
	}
	ids := make([]int64, 0, len(w.coll.Parts)+1)
	seen := map[int64]bool{}
	for _, p := range w.coll.Parts {
		id, err := w.s.media.EnsureMovieIn(ctx, p.TMDBID, &w.anchor.RootFolderID)
		if err != nil {
			return grabTarget{}, fmt.Errorf("add %q from collection %q: %w", p.Title, w.coll.Name, err)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if !seen[w.anchor.ID] {
		// TMDB may list the movie under a collection that does not list it
		// back; it is still the movie the pack was grabbed for.
		ids = append(ids, w.anchor.ID)
	}
	anchor := w.anchor.ID
	return grabTarget{movieID: &anchor, movieIDs: ids}, nil
}

// movieCollection fetches the collection a TMDB movie belongs to, keeping
// only its released movies.
func (s *Service) movieCollection(ctx context.Context, tmdbID int64) (*metadata.Collection, error) {
	meta, err := s.media.TMDBMovie(ctx, tmdbID)
	if err != nil {
		return nil, err
	}
	if meta.Collection == nil {
		return nil, fmt.Errorf("movie %q is not part of a collection: %w", meta.Title, apperr.ErrNotFound)
	}
	coll, err := s.media.TMDBCollection(ctx, meta.Collection.ID)
	if err != nil {
		return nil, err
	}
	coll.Parts = releasedParts(coll.Parts, s.media.ThisYear())
	if len(coll.Parts) == 0 {
		return nil, fmt.Errorf("collection %q has no released movies: %w", coll.Name, apperr.ErrNotFound)
	}
	return coll, nil
}

// releasedParts keeps the movies with a known year no later than thisYear.
func releasedParts(parts []metadata.Movie, thisYear int) []metadata.Movie {
	out := make([]metadata.Movie, 0, len(parts))
	for _, p := range parts {
		if p.Year > 0 && p.Year <= thisYear {
			out = append(out, p)
		}
	}
	return out
}

// SearchWantedMovies auto-searches every monitored movie without a file or
// active download.
func (s *Service) SearchWantedMovies(ctx context.Context) error {
	wanted, err := s.store.ListWantedMovies(ctx)
	if err != nil {
		return fmt.Errorf("list wanted movies: %w", err)
	}
	for _, m := range wanted {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := s.autoSearch(ctx, movieWant{s: s, movie: m}, nil); err != nil {
			s.log.Warn().Int64("movie", m.ID).Str("title", m.Title).Err(err).Msg("wanted search")
		}
	}
	return nil
}
