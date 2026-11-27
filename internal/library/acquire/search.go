package acquire

import (
	"fmt"
	"strings"
	"time"

	"generic-arr-project/internal/indexer"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/release"
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

	// MoviePack is a release of several movies of one collection.
	MoviePack bool `json:"movie_pack,omitempty"`
}

// SearchResult is the outcome of searching all indexers for one item.
type SearchResult struct {
	// Query is the text searched for: the user's own, or the text form of
	// the query arrd built.
	Query    string                    `json:"query"`
	Releases []ReleaseResult           `json:"releases"`
	Failures []indexers.IndexerFailure `json:"indexer_failures"`
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
			MoviePack: info.MoviePack,
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

