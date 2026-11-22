// Package indexer searches tracker sites directly, driven by Cardigann YAML
// definitions: the format Prowlarr and Jackett use
// (github.com/Prowlarr/Indexers). Each definition describes one site — how
// to sign in, search and read results — and becomes a Definition in the
// Catalog; a configured indexer is a definition plus the user's settings.
//
// Definitions are executed by github.com/Kcchouette/cardigann-go, pinned
// in go.mod. That library builds its own HTTP transport (honouring proxy
// environment variables); Searcher replaces it with the indexer's netx
// transport so VPN-routed indexers cannot leak. The library sends every
// request through the one client it exposes, which
// TestNoRequestBypassesTransport guards. Re-audit before bumping the pin.
package indexer

import "time"

// Caps lists what the indexer supports.
type Caps struct {
	Search      SearchCap  `json:"search"`
	MovieSearch SearchCap  `json:"movie_search"`
	TVSearch    SearchCap  `json:"tv_search"`
	Categories  []Category `json:"categories"`
}

// SearchCap describes one search mode.
type SearchCap struct {
	Available bool     `json:"available"`
	Params    []string `json:"params"` // e.g. q, imdbid, tvdbid, season, ep
}

// Supports reports whether param is a supported parameter.
func (s SearchCap) Supports(param string) bool {
	for _, p := range s.Params {
		if p == param {
			return true
		}
	}
	return false
}

// Category is a Newznab category with its subcategories.
type Category struct {
	ID      int        `json:"id"`
	Name    string     `json:"name"`
	Subcats []Category `json:"subcats,omitempty"`
}

// SearchType is the Newznab search mode ("t").
type SearchType string

const (
	SearchGeneric SearchType = "search"
	SearchMovie   SearchType = "movie"
	SearchTV      SearchType = "tvsearch"
)

// Query is a search request.
type Query struct {
	Type       SearchType
	Q          string
	IMDBID     string // with or without the "tt" prefix
	TVDBID     int64
	Season     *int
	Episode    *int
	Categories []int
}

// Release is one search result.
type Release struct {
	Title       string
	GUID        string
	DownloadURL string // .torrent link; may redirect to a magnet
	MagnetURI   string
	InfoHash    string
	Size        int64
	Seeders     int
	Peers       int
	PublishDate time.Time
}
