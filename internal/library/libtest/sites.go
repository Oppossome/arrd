package libtest

import (
	"fmt"
	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/library/indexers"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSiteID is the fictional public tracker every test service knows.
const TestSiteID = "testsite"

// OtherSiteID is a second fictional tracker with the same layout, for tests
// that need two indexers (a site has at most one).
const OtherSiteID = "othersite"

// testSiteDefinition describes the fictional tracker in Cardigann format.
// Its search page takes Newznab-style parameters (t, q, imdbid, tvdbid,
// season, ep, cat), so fake sites can answer like a Torznab feed. LINKS is
// replaced with the site's addresses.
const testSiteDefinition = `---
id: testsite
name: Test Site
description: A fictional public tracker for tests.
language: en-US
type: public
encoding: UTF-8
links: LINKS
caps:
  categorymappings:
    - {id: movies, cat: Movies, desc: Movies}
    - {id: tv, cat: TV, desc: TV}
  modes:
    search: [q]
    movie-search: [q, imdbid]
    tv-search: [q, season, ep, tvdbid]
  allowrawsearch: true
settings:
  - {name: apikey, type: password, label: API key}
search:
  paths:
    - path: search
  inputs:
    t: "{{ .Query.Type }}"
    q: "{{ .Keywords }}"
    imdbid: "{{ .Query.IMDBIDShort }}"
    tvdbid: "{{ .Query.TVDBID }}"
    season: "{{ .Query.Season }}"
    ep: "{{ .Query.Ep }}"
    cat: "{{ join .Categories \",\" }}"
  rows:
    selector: tr.row
  fields:
    title: {selector: a.title}
    download: {selector: a.dl, attribute: href}
    size: {selector: td.size}
    seeders: {selector: td.seeders}
`

// DefaultSiteLinks are the test site's addresses unless a test serves it.
var DefaultSiteLinks = []string{"http://site-a.invalid/", "http://site-b.invalid/"}

// WriteTestDefinition writes the test sites' definitions, listing links as
// their addresses, into dataDir's definitions directory.
func WriteTestDefinition(t *testing.T, dataDir string, links ...string) {
	t.Helper()
	dir := filepath.Join(dataDir, "definitions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	quoted := make([]string, len(links))
	for i, l := range links {
		quoted[i] = fmt.Sprintf("%q", l)
	}
	body := strings.Replace(testSiteDefinition, "LINKS", "["+strings.Join(quoted, ", ")+"]", 1)
	other := strings.Replace(body, "id: testsite\nname: Test Site\n", "id: othersite\nname: Other Site\n", 1)
	for id, def := range map[string]string{TestSiteID: body, OtherSiteID: other} {
		if err := os.WriteFile(filepath.Join(dir, id+".yml"), []byte(def), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// UseSites makes the test site reachable at links (e.g. fake sites'
// addresses) and reloads the definitions.
func (env *Env) UseSites(t *testing.T, links ...string) {
	t.Helper()
	WriteTestDefinition(t, env.DataDir, links...)
	if res := env.Svc.Indexers.ReloadIndexerDefinitions(); len(res.Errors) != 0 {
		t.Fatalf("reload definitions: %v", res.Errors)
	}
}

// SiteIndexer is input for an indexer on the test site at baseURL.
func SiteIndexer(name, baseURL string) indexers.IndexerInput {
	return indexers.IndexerInput{Name: name, Definition: TestSiteID, Settings: formspec.Values{"base_url": baseURL}}
}

// SiteIndexerKey is SiteIndexer with an API key (a secret setting).
func SiteIndexerKey(name, baseURL, apiKey string) indexers.IndexerInput {
	in := SiteIndexer(name, baseURL)
	in.Settings["apikey"] = apiKey
	return in
}

// OnSite moves input to the site with id.
func OnSite(in indexers.IndexerInput, id string) indexers.IndexerInput {
	in.Definition = id
	return in
}

// SiteRow is one search result on a fake test site.
type SiteRow struct {
	Title, Link string
	Size        int64
	Seeders     int
}

// FakeSite serves the test site: search results from rows for each
// search's parameters, and links (e.g. downloads) from extra by path.
func FakeSite(t *testing.T, rows func(q url.Values) []SiteRow, extra map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := extra[r.URL.Path]; ok {
			h(w, r)
			return
		}
		if r.URL.Path != "/search" {
			t.Errorf("unexpected site request %s", r.URL)
			http.NotFound(w, r)
			return
		}
		var b strings.Builder
		b.WriteString("<html><body><table>")
		for _, row := range rows(r.URL.Query()) {
			fmt.Fprintf(&b, `<tr class="row"><td><a class="title">%s</a></td><td><a class="dl" href="%s">get</a></td><td class="size">%d</td><td class="seeders">%d</td></tr>`,
				html.EscapeString(row.Title), html.EscapeString(row.Link), row.Size, row.Seeders)
		}
		b.WriteString("</table></body></html>")
		_, _ = fmt.Fprint(w, b.String())
	}))
	t.Cleanup(srv.Close)
	return srv
}

// RedirectTo answers with a redirect, e.g. to a magnet link.
func RedirectTo(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target, http.StatusFound) }
}
