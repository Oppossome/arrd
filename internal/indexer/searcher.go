package indexer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	cdef "github.com/Kcchouette/cardigann-go/definition"
	"github.com/Kcchouette/cardigann-go/engine"
	"github.com/Kcchouette/cardigann-go/httpclient"
	"github.com/Kcchouette/cardigann-go/release"

	"generic-arr-project/internal/formspec"
)

// Searcher runs one configured site. It keeps the site's sign-in session
// (cookies in memory), so callers should reuse it.
type Searcher struct {
	def      cdef.IndexerDefinition
	provider *engine.Provider
	client   *http.Client // signed-in, on the indexer's network
}

// newSearcher builds a searcher for def with normalized settings v. Every
// request goes through client's transport.
func newSearcher(def cdef.IndexerDefinition, v formspec.Values, client *http.Client) (*Searcher, error) {
	base := v[baseURLKey]
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, formspec.Fieldf(baseURLKey, "must be an http(s) URL")
	}
	ic, err := httpclient.New(httpclient.Options{
		IndexerID:    def.ID,
		RequestDelay: time.Duration(def.RequestDelay * float64(time.Second)),
		Timeout:      client.Timeout,
		MaxRetries:   1,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", def.Name, err)
	}
	// The library's own transport would ignore the indexer's network (and
	// honour proxy environment variables); use the indexer's instead.
	signedIn := ic.HTTPClient()
	signedIn.Transport = client.Transport
	if signedIn.Transport == nil {
		signedIn.Transport = http.DefaultTransport
	}
	p := engine.NewAuthenticatedProvider(def, strings.TrimRight(base, "/")+"/", config(def, v), ic)
	// Download chains can have side effects (e.g. posting a "thanks" to
	// unlock a link) and cost a request per row: run them only on grab.
	p.SkipSearchDownload()
	return &Searcher{def: def, provider: p, client: signedIn}, nil
}

// config is the values the definition's templates see: settings without
// arrd's own, and toggles that are off left out (any non-empty string is
// true in a template).
func config(def cdef.IndexerDefinition, v formspec.Values) map[string]string {
	toggles := map[string]bool{}
	for _, s := range def.Settings {
		if s.Type == "checkbox" {
			toggles[s.Name] = true
		}
	}
	out := map[string]string{}
	for k, val := range v {
		if k == baseURLKey || (toggles[k] && val != "true") {
			continue
		}
		out[k] = val
	}
	// Keep a default-on toggle the user turned off from being re-enabled
	// by the engine's defaults.
	for _, s := range def.Settings {
		if toggles[s.Name] && v[s.Name] == "false" && toggleDefault(s.Default) == "true" {
			out[s.Name] = ""
		}
	}
	return out
}

// Caps reports the site's search modes and categories, from its
// definition.
func (s *Searcher) Caps() *Caps { return caps(s.def) }

func caps(def cdef.IndexerDefinition) *Caps {
	c := &Caps{
		Search:      searchCap(def.Caps.Modes, "search"),
		MovieSearch: searchCap(def.Caps.Modes, "movie-search"),
		TVSearch:    searchCap(def.Caps.Modes, "tv-search"),
	}
	ids := map[int]bool{}
	for _, m := range def.Caps.CategoryMappings {
		if id, ok := newznabID(m.Cat); ok {
			ids[id] = true
		}
	}
	for _, native := range def.Caps.Categories.Keys() {
		if id, ok := newznabID(def.Caps.Categories.Value(native)); ok {
			ids[id] = true
		}
	}
	c.Categories = categoryTree(ids)
	return c
}

func searchCap(modes map[string][]string, mode string) SearchCap {
	params, ok := modes[mode]
	if !ok {
		return SearchCap{}
	}
	return SearchCap{Available: true, Params: slices.Clone(params)}
}

// categoryTree groups Newznab IDs under their parents (2040 under 2000).
func categoryTree(ids map[int]bool) []Category {
	names := make(map[int]string, len(newznabCategories))
	for name, id := range newznabCategories {
		names[id] = name
	}
	name := func(id int) string {
		if n, ok := names[id]; ok {
			return n
		}
		return strconv.Itoa(id)
	}
	parents := map[int]*Category{}
	var order []int
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		parent := id / 1000 * 1000
		p, ok := parents[parent]
		if !ok {
			p = &Category{ID: parent, Name: name(parent)}
			parents[parent] = p
			order = append(order, parent)
		}
		if id != parent {
			p.Subcats = append(p.Subcats, Category{ID: id, Name: name(id)})
		}
	}
	out := make([]Category, 0, len(order))
	for _, id := range order {
		out = append(out, *parents[id])
	}
	return out
}

// errNoRawSearch is returned for an empty query to a site that cannot list
// recent releases; such kinds have RSS off.
var errNoRawSearch = errors.New("the site cannot list recent releases without a search term")

// Search runs q on the site. Only torrent results are returned.
func (s *Searcher) Search(ctx context.Context, q Query) ([]Release, error) {
	eq := engine.Query{
		Keywords:   q.Q,
		Type:       string(q.Type),
		Categories: expandCategories(q.Categories, s.def),
	}
	if q.IMDBID != "" {
		short := strings.TrimPrefix(q.IMDBID, "tt")
		eq.IMDBID, eq.IMDBIDShort = "tt"+short, short
	}
	if q.TVDBID > 0 {
		eq.TVDBID = strconv.FormatInt(q.TVDBID, 10)
	}
	if q.Season != nil {
		eq.Season = strconv.Itoa(*q.Season)
	}
	if q.Episode != nil {
		eq.Ep = strconv.Itoa(*q.Episode)
	}
	if eq.Keywords == "" && eq.IMDBID == "" && eq.TVDBID == "" && eq.Season == "" && eq.Ep == "" && !s.def.Caps.AllowRawSearch {
		return nil, errNoRawSearch
	}
	rels, err := s.provider.Search(ctx, eq)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.def.Name, err)
	}
	out := make([]Release, 0, len(rels))
	// Only torrent sites load (see unsupported). The per-result protocol is
	// not used: the library guesses it with the same usenet heuristic,
	// which misreads normalized categories.
	for _, r := range rels {
		if ir, ok := toRelease(r); ok {
			out = append(out, ir)
		}
	}
	return out, nil
}

// toRelease converts a result; one with no way to fetch it is dropped.
func toRelease(r release.Release) (Release, bool) {
	out := Release{
		Title: strings.TrimSpace(r.Title), GUID: r.Details, InfoHash: strings.ToLower(r.InfoHash),
		Size: r.Size, Seeders: r.Seeders, Peers: r.Peers, PublishDate: r.PublishDate,
	}
	switch {
	case strings.HasPrefix(r.DownloadLink, "magnet:"):
		out.MagnetURI = r.DownloadLink
	case r.DownloadLink != "":
		out.DownloadURL = r.DownloadLink
	case out.InfoHash != "":
		out.MagnetURI = "magnet:?xt=urn:btih:" + out.InfoHash + "&dn=" + url.QueryEscape(out.Title)
	}
	if out.Title == "" || (out.MagnetURI == "" && out.DownloadURL == "") {
		return Release{}, false
	}
	if out.GUID == "" {
		out.GUID = out.DownloadURL + out.MagnetURI
	}
	return out, true
}

// Resolve runs the definition's download chain for r and fetches the
// .torrent with the signed-in session.
func (s *Searcher) Resolve(ctx context.Context, r Release) (TorrentSource, error) {
	if r.MagnetURI != "" {
		return TorrentSource{Magnet: r.MagnetURI}, nil
	}
	resolved, err := s.provider.ResolveDownloadLink(ctx, release.Release{
		Title: r.Title, DownloadLink: r.DownloadURL, InfoHash: r.InfoHash,
	})
	if err != nil {
		return TorrentSource{}, fmt.Errorf("%s: %w", s.def.Name, err)
	}
	if strings.HasPrefix(resolved.DownloadLink, "magnet:") {
		return TorrentSource{Magnet: resolved.DownloadLink}, nil
	}
	return Resolve(ctx, s.client, Release{DownloadURL: resolved.DownloadLink})
}

// Test signs in (if the site needs it) and runs a query, as Prowlarr does:
// recent releases where the site lists them, else a keyword search.
func (s *Searcher) Test(ctx context.Context) error {
	q := Query{Type: SearchGeneric}
	if !s.def.Caps.AllowRawSearch {
		q.Q = "test"
	}
	_, err := s.Search(ctx, q)
	return err
}

// expandCategories adds the site's subcategories of each requested parent
// category (2000 also asks for 2040), as Newznab parents include their
// children; the engine matches categories exactly.
func expandCategories(requested []int, def cdef.IndexerDefinition) []int {
	if len(requested) == 0 {
		return nil
	}
	out := slices.Clone(requested)
	mapped := map[int]bool{}
	for _, m := range def.Caps.CategoryMappings {
		if id, ok := newznabID(m.Cat); ok {
			mapped[id] = true
		}
	}
	for _, native := range def.Caps.Categories.Keys() {
		if id, ok := newznabID(def.Caps.Categories.Value(native)); ok {
			mapped[id] = true
		}
	}
	for _, c := range requested {
		if c%1000 != 0 {
			continue
		}
		for id := range mapped {
			if id/1000*1000 == c && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	slices.Sort(out)
	return out
}
