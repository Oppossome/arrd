package acquire

import (
	"context"
	"fmt"
	"strings"
	"time"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/release"
	"generic-arr-project/internal/store/db"
)

// ReleaseResult is a search result with the decision about it.
type ReleaseResult struct {
	Title       string    `json:"title"`
	IndexerID   int64     `json:"indexer_id"`
	Indexer     string    `json:"indexer"`
	DownloadURL string    `json:"download_url,omitempty"`
	MagnetURI   string    `json:"magnet_uri,omitempty"`
	InfoHash    string    `json:"info_hash,omitempty"`
	Size        int64     `json:"size"`
	Seeders     int       `json:"seeders"`
	Peers       int       `json:"peers"`
	PublishDate time.Time `json:"publish_date"`
	Quality     string    `json:"quality"`
	Accepted    bool      `json:"accepted"`
	Rejections  []string  `json:"rejections,omitempty"`
	Score       int       `json:"score"`

	// TV releases only.
	Season     *int  `json:"season,omitempty"`
	Episodes   []int `json:"episodes,omitempty"`
	SeasonPack bool  `json:"season_pack,omitempty"`
	// LastSeason is the last season of a multi-season pack (Season is the
	// first); CompleteSeries is a pack of every season that names none.
	LastSeason     *int `json:"last_season,omitempty"`
	CompleteSeries bool `json:"complete_series,omitempty"`
}

// SearchResult is the outcome of searching all indexers for one item.
type SearchResult struct {
	// Query is the text searched for: the user's own, or the text form of
	// the query arrd built.
	Query    string                    `json:"query"`
	Releases []ReleaseResult           `json:"releases"`
	Failures []indexers.IndexerFailure `json:"indexer_failures"`
}

// SearchMovie queries all search-enabled indexers for a movie and ranks the
// results, best acceptable release first. text replaces arrd's own query
// when not blank.
func (s *Service) SearchMovie(ctx context.Context, movieID int64, text string) (SearchResult, error) {
	movie, err := s.store.GetMovie(ctx, movieID)
	if err != nil {
		return SearchResult{}, apperr.NotFound(err, "movie", movieID)
	}
	return s.searchMovie(ctx, movie, text)
}

// searchMovie searches for movie, which need not be in the library: only
// its title, year and IMDb id are used. text replaces arrd's own query
// when not blank.
func (s *Service) searchMovie(ctx context.Context, movie db.Movie, text string) (SearchResult, error) {
	q, err := s.settings.QualitySettings(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	text, custom := searchText(text, movieText(movie))
	found, failures, err := s.indexers.Search(ctx, func(ix indexers.Indexer) (indexer.Query, bool) {
		if custom {
			return textQuery(text, ix, false)
		}
		return movieQuery(movie, ix)
	})
	if err != nil {
		return SearchResult{}, err
	}
	target := release.MovieTarget{Title: movie.Title, OriginalTitle: movie.OriginalTitle, Year: int(movie.Year)}
	limits := q.Limits()
	res := rankReleases(found, failures, func(c release.Candidate) release.Decision {
		return release.EvaluateMovie(c, target, limits)
	})
	res.Query = text
	return res, nil
}

// searchText is the text to search for: the user's own when not blank
// (custom), else arrd's.
func searchText(user, own string) (text string, custom bool) {
	if user = strings.TrimSpace(user); user != "" {
		return user, true
	}
	return own, false
}

// textQuery searches for text the user typed: a movie or TV search when the
// indexer has one, else a generic search. No id, season or episode is sent,
// since the text says what is wanted.
func textQuery(text string, ix indexers.Indexer, tv bool) (indexer.Query, bool) {
	q := indexer.Query{Type: indexer.SearchMovie, Categories: ix.MovieCategories, Q: text}
	typed := ix.Caps != nil && ix.Caps.MovieSearch.Available
	if tv {
		q.Type, q.Categories = indexer.SearchTV, ix.TVCategories
		typed = ix.Caps != nil && ix.Caps.TVSearch.Available
	}
	switch {
	case typed:
	case ix.Caps == nil || ix.Caps.Search.Available:
		q.Type = indexer.SearchGeneric
	default:
		return indexer.Query{}, false
	}
	return q, true
}

// rankReleases de-duplicates, evaluates and ranks search results, best
// acceptable release first.
func rankReleases(found []indexers.FoundRelease, failures []indexers.IndexerFailure, evaluate func(release.Candidate) release.Decision) SearchResult {
	ranked := make([]release.Ranked[ReleaseResult], 0, len(found))
	seen := map[string]bool{}
	for _, f := range found {
		key := f.InfoHash
		if key == "" {
			key = f.Title + "|" + f.Indexer
		}
		if seen[key] {
			continue // same torrent from several indexers
		}
		seen[key] = true

		info := release.Parse(f.Title)
		d := evaluate(release.Candidate{Info: info, Size: f.Size, Seeders: f.Seeders})
		if int64(f.Seeders) < f.MinSeeders && d.Accepted {
			d.Accepted = false
			d.Rejects = append(d.Rejects, fmt.Sprintf("%d seeders is below the indexer minimum %d", f.Seeders, f.MinSeeders))
		}
		item := ReleaseResult{
			Title: f.Title, IndexerID: f.IndexerID, Indexer: f.Indexer, DownloadURL: f.DownloadURL,
			MagnetURI: f.MagnetURI, InfoHash: f.InfoHash, Size: f.Size, Seeders: f.Seeders, Peers: f.Peers,
			PublishDate: f.PublishDate, Quality: info.Quality(), Accepted: d.Accepted, Rejections: d.Rejects, Score: d.Score,
		}
		if info.IsTV {
			season := info.Season
			item.Season, item.Episodes, item.SeasonPack = &season, info.Episodes, info.SeasonPack
			item.CompleteSeries = info.CompleteSeries
			if info.MultiSeason && !info.CompleteSeries {
				last := info.LastSeason
				item.LastSeason = &last
			}
		}
		ranked = append(ranked, release.Ranked[ReleaseResult]{Decision: d, Seeders: f.Seeders, Item: item})
	}
	release.Rank(ranked)
	out := SearchResult{Releases: make([]ReleaseResult, len(ranked)), Failures: failures}
	for i, r := range ranked {
		out.Releases[i] = r.Item
	}
	if out.Failures == nil {
		out.Failures = []indexers.IndexerFailure{}
	}
	return out
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
			s.log.Warn("grab failed, trying next release", "release", r.Title, "err", err)
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

// movieQuery builds the best query the indexer supports: an IMDb lookup if
// possible, otherwise a title + year text search.
func movieQuery(m db.Movie, ix indexers.Indexer) (indexer.Query, bool) {
	text := movieText(m)
	q := indexer.Query{Categories: ix.MovieCategories}
	switch {
	case ix.Caps == nil:
		// Not tested yet: a generic text search works everywhere.
		q.Type, q.Q = indexer.SearchGeneric, text
	case ix.Caps.MovieSearch.Available && m.IMDBID != nil && ix.Caps.MovieSearch.Supports("imdbid"):
		q.Type, q.IMDBID = indexer.SearchMovie, *m.IMDBID
	case ix.Caps.MovieSearch.Available:
		q.Type, q.Q = indexer.SearchMovie, text
	case ix.Caps.Search.Available:
		q.Type, q.Q = indexer.SearchGeneric, text
	default:
		return indexer.Query{}, false
	}
	return q, true
}

// movieText is a text search for m: its title and year.
func movieText(m db.Movie) string {
	if m.Year > 0 {
		return fmt.Sprintf("%s %d", m.Title, m.Year)
	}
	return m.Title
}

// AutoSearchResult reports what an automatic search did.
type AutoSearchResult struct {
	Grabbed  *Download                 `json:"grabbed,omitempty"`
	Searched int                       `json:"releases_considered"`
	Failures []indexers.IndexerFailure `json:"indexer_failures"`
}

// AutoSearchMovie searches for a movie and grabs the best acceptable
// release, trying the next one if a grab fails.

// SearchWantedMovies runs AutoSearchMovie for every monitored movie
// without a file or active download.
func (s *Service) SearchWantedMovies(ctx context.Context) error {
	wanted, err := s.store.ListWantedMovies(ctx)
	if err != nil {
		return fmt.Errorf("list wanted movies: %w", err)
	}
	for _, m := range wanted {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := s.AutoSearchMovie(ctx, m.ID); err != nil {
			s.log.Warn("wanted search", "movie", m.ID, "title", m.Title, "err", err)
		}
	}
	return nil
}

// AutoSearchMovie searches for a movie and grabs the best acceptable
// release, trying the next one if a grab fails.
func (s *Service) AutoSearchMovie(ctx context.Context, movieID int64) (AutoSearchResult, error) {
	res, err := s.SearchMovie(ctx, movieID, "")
	if err != nil {
		return AutoSearchResult{}, err
	}
	out := AutoSearchResult{Searched: len(res.Releases), Failures: res.Failures}
	out.Grabbed, err = s.grabBest(ctx, res, func(r ReleaseResult) (Download, error) {
		return s.GrabMovieRelease(ctx, movieID, r)
	})
	if err == nil && out.Grabbed == nil {
		s.log.Info("no acceptable release found", "movie", movieID, "considered", len(res.Releases), "indexer_failures", len(res.Failures))
	}
	return out, err
}
