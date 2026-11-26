package indexer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	cdef "github.com/Kcchouette/cardigann-go/definition"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/testutil"
)

// fictionalDefinition is an invented private tracker. SITE is replaced
// with the test server's address.
const fictionalDefinition = `---
id: examplebay
name: Example Bay
description: A fictional private tracker for tests.
language: en-US
type: private
encoding: UTF-8
links:
  - SITE/
caps:
  categorymappings:
    - {id: 11, cat: Movies/HD, desc: "Films HD"}
    - {id: 22, cat: TV/HD, desc: "Series HD"}
  modes:
    search: [q]
    movie-search: [q, imdbid]
    tv-search: [q, season, ep]
settings:
  - {name: username, type: text, label: Username}
  - {name: password, type: password, label: Password}
  - {name: freeleech, type: checkbox, label: Freeleech only, default: false}
  - {name: sort, type: select, label: Sort, default: added, options: {added: Newest, seeders: Seeders}}
  - {name: notice, type: info, label: Accounts are invite-only.}
login:
  path: login.php
  method: post
  inputs:
    username: "{{ .Config.username }}"
    password: "{{ .Config.password }}"
  error:
    - selector: div.error
  test:
    path: index.php
    selector: a[href="logout.php"]
search:
  paths:
    - path: browse.php
  inputs:
    q: "{{ .Keywords }}"
    imdb: "{{ .Query.IMDBIDShort }}"
    cats: "{{ join .Categories \",\" }}"
    free: "{{ if .Config.freeleech }}1{{ else }}{{ end }}"
  rows:
    selector: table.torrents tr.row
  fields:
    title:
      selector: a.title
    details:
      selector: a.title
      attribute: href
    download:
      selector: a.dl
      attribute: href
    magnet:
      selector: a.magnet
      attribute: href
      optional: true
    size:
      selector: td.size
    seeders:
      selector: td.seeders
    leechers:
      selector: td.leechers
    category:
      selector: td.cat
`

// site is the fictional tracker. Searches and downloads need the session
// cookie that a correct login sets.
type site struct {
	*httptest.Server
	mu       sync.Mutex
	requests int
	logins   int
	queries  []string
}

const (
	fictionalUser    = "tester"
	fictionalPass    = "fictional-pass"
	fictionalTorrent = "d8:announce22:http://tracker.invalid4:infod4:name4:testee"
)

func newSite(t *testing.T) *site {
	t.Helper()
	s := &site{}
	authed := func(r *http.Request) bool {
		c, err := r.Cookie("session")
		return err == nil && c.Value == "ok"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/login.php", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.mu.Lock()
			s.logins++
			s.mu.Unlock()
			if r.FormValue("username") == fictionalUser && r.FormValue("password") == fictionalPass {
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
				http.Redirect(w, r, "/index.php", http.StatusFound)
				return
			}
		}
		testutil.Write(t, w, `<html><body><div class="error">Wrong username or password</div></body></html>`)
	})
	mux.HandleFunc("/index.php", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			testutil.Write(t, w, `<html><body><a href="login.php">Log in</a></body></html>`)
			return
		}
		testutil.Write(t, w, `<html><body><a href="logout.php">Log out</a></body></html>`)
	})
	mux.HandleFunc("/browse.php", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			http.Redirect(w, r, "/login.php", http.StatusFound)
			return
		}
		s.mu.Lock()
		s.queries = append(s.queries, r.URL.RawQuery)
		s.mu.Unlock()
		testutil.Write(t, w, `<html><body><table class="torrents">
<tr class="row"><td class="cat">11</td><td><a class="title" href="details.php?id=1">Example.Film.2001.1080p.BluRay.x264-NOGRP</a></td>
<td><a class="dl" href="download.php?id=1">get</a></td><td class="size">1.5 GB</td><td class="seeders">12</td><td class="leechers">3</td></tr>
<tr class="row"><td class="cat">11</td><td><a class="title" href="details.php?id=2">Example.Film.2001.720p.WEB-DL-NOGRP</a></td>
<td><a class="dl" href="download.php?id=2">get</a><a class="magnet" href="magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567">m</a></td><td class="size">700 MB</td><td class="seeders">4</td><td class="leechers">0</td></tr>
</table></body></html>`)
	})
	mux.HandleFunc("/download.php", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/x-bittorrent")
		testutil.Write(t, w, fictionalTorrent)
	})
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests++
		s.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *site) definition(t *testing.T) cdef.IndexerDefinition {
	t.Helper()
	def, err := cdef.Parse([]byte(strings.ReplaceAll(fictionalDefinition, "SITE", s.URL)))
	if err != nil {
		t.Fatal(err)
	}
	return def
}

// countingTransport counts the requests that go through it.
type countingTransport struct {
	base http.RoundTripper
	n    atomic.Int64
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n.Add(1)
	return c.base.RoundTrip(r)
}

func newTestSearcher(t *testing.T, s *site, settings formspec.Values) (*Searcher, *countingTransport) {
	t.Helper()
	d := newDefinition(s.definition(t))
	if d.Unsupported != "" {
		t.Fatalf("unsupported: %s", d.Unsupported)
	}
	v, err := d.Normalize(settings)
	if err != nil {
		t.Fatal(err)
	}
	tr := &countingTransport{base: http.DefaultTransport}
	srch, err := d.NewSearcher(v, &http.Client{Transport: tr})
	if err != nil {
		t.Fatal(err)
	}
	return srch, tr
}

func goodSettings(s *site) formspec.Values {
	return formspec.Values{"base_url": s.URL + "/", "username": fictionalUser, "password": fictionalPass}
}

func TestDefinitionFields(t *testing.T) {
	s := newSite(t)
	k := newDefinition(s.definition(t))
	if k.ID != "examplebay" || k.Name != "Example Bay" || k.RSS || k.Unsupported != "" {
		t.Fatalf("definition = %+v", k)
	}
	byKey := map[string]formspec.Field{}
	for _, f := range k.Fields {
		byKey[f.Key] = f
	}
	if f := byKey["base_url"]; f.Kind != formspec.KindSelect || !f.Required || f.Default != s.URL+"/" || !strings.Contains(f.Help, "invite-only") {
		t.Errorf("base_url field = %+v", f)
	}
	if f := byKey["password"]; !f.Secret || !f.Required {
		t.Errorf("password field = %+v", f)
	}
	if f := byKey["freeleech"]; f.Kind != formspec.KindToggle || f.Default != "false" || f.Required {
		t.Errorf("freeleech field = %+v", f)
	}
	if f := byKey["sort"]; f.Kind != formspec.KindSelect || len(f.Choices) != 2 || f.Choices[1].Value != "seeders" || f.Choices[1].Label != "Seeders" {
		t.Errorf("sort field = %+v", f)
	}
	if _, ok := byKey["notice"]; ok {
		t.Error("info setting became an input")
	}
	// Settings are checked against the definition.
	if _, err := k.Normalize(formspec.Values{"base_url": "http://elsewhere.invalid/", "username": "u", "password": "p"}); err == nil {
		t.Error("a site address the definition does not list was accepted")
	}
	if _, err := k.Normalize(formspec.Values{"base_url": s.URL + "/"}); err == nil {
		t.Error("missing credentials for a private site accepted")
	}
}

func TestCapsFromDefinition(t *testing.T) {
	s := newSite(t)
	srch, _ := newTestSearcher(t, s, goodSettings(s))
	caps := srch.Caps()
	if !caps.MovieSearch.Supports("imdbid") || !caps.TVSearch.Supports("ep") || caps.TVSearch.Supports("tvdbid") {
		t.Errorf("caps = %+v", caps)
	}
	got := fmt.Sprint(caps.Categories)
	if got != "[{2000 Movies [{2040 Movies/HD []}]} {5000 TV [{5040 TV/HD []}]}]" {
		t.Errorf("categories = %s", got)
	}
}

func TestSearchSignsInAndResolves(t *testing.T) {
	ctx := context.Background()
	s := newSite(t)
	srch, tr := newTestSearcher(t, s, goodSettings(s))

	rels, err := srch.Search(ctx, Query{Type: SearchMovie, IMDBID: "tt0000001", Categories: []int{2040}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 2 {
		t.Fatalf("releases = %+v", rels)
	}
	first := rels[0]
	if first.Title != "Example.Film.2001.1080p.BluRay.x264-NOGRP" || first.Seeders != 12 || first.Size < 1_400_000_000 ||
		!strings.HasSuffix(first.DownloadURL, "download.php?id=1") || first.MagnetURI != "" || !strings.HasSuffix(first.GUID, "details.php?id=1") {
		t.Errorf("first = %+v", first)
	}
	// With both links, the .torrent is used (it carries the private tracker's announce URL).
	if rels[1].MagnetURI != "" || !strings.HasSuffix(rels[1].DownloadURL, "download.php?id=2") || rels[1].Size != 734003200 {
		t.Errorf("second = %+v", rels[1])
	}
	s.mu.Lock()
	q := s.queries[0]
	s.mu.Unlock()
	// Newznab 2040 maps to the site's category 11; the IMDb id is passed short.
	if !strings.Contains(q, "cats=11") || !strings.Contains(q, "imdb=0000001") || strings.Contains(q, "free=1") {
		t.Errorf("query = %s", q)
	}

	// A second search reuses the session.
	if _, err := srch.Search(ctx, Query{Type: SearchGeneric, Q: "example"}); err != nil {
		t.Fatal(err)
	}
	if s.logins != 1 {
		t.Errorf("logins = %d, want 1", s.logins)
	}

	src, err := srch.Resolve(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if string(src.MetaInfo) != fictionalTorrent {
		t.Errorf("torrent = %q", src.MetaInfo)
	}

	// Every request the site saw went through the indexer's transport
	// (TestNoRequestBypassesTransport's invariant, with login and download).
	if got, want := tr.n.Load(), int64(s.requests); got != want || want == 0 {
		t.Errorf("transport carried %d requests, site saw %d", got, want)
	}
}

// The library builds its own transport; requests must use the indexer's
// instead, or a VPN-routed indexer would leak onto the host network.
func TestNoRequestBypassesTransport(t *testing.T) {
	s := newSite(t)
	// A proxy from the environment must not be used either.
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("http_proxy", "http://127.0.0.1:1")
	srch, tr := newTestSearcher(t, s, goodSettings(s))
	if _, err := srch.Search(context.Background(), Query{Type: SearchGeneric, Q: "example"}); err != nil {
		t.Fatal(err)
	}
	if got, want := tr.n.Load(), int64(s.requests); got != want || want == 0 {
		t.Errorf("transport carried %d requests, site saw %d", got, want)
	}
}

func TestBadLoginFails(t *testing.T) {
	s := newSite(t)
	settings := goodSettings(s)
	settings["password"] = "wrong"
	srch, _ := newTestSearcher(t, s, settings)
	_, err := srch.Search(context.Background(), Query{Type: SearchGeneric, Q: "example"})
	if err == nil || !strings.Contains(err.Error(), "login") {
		t.Errorf("err = %v", err)
	}
}

func TestEmptySearchWithoutRawSearch(t *testing.T) {
	s := newSite(t)
	srch, _ := newTestSearcher(t, s, goodSettings(s))
	if _, err := srch.Search(context.Background(), Query{Type: SearchGeneric}); !errors.Is(err, errNoRawSearch) {
		t.Errorf("err = %v", err)
	}
	if s.requests != 0 {
		t.Errorf("site contacted %d times for an empty search", s.requests)
	}
}

func TestUnsupportedDefinitions(t *testing.T) {
	s := newSite(t)
	base := strings.ReplaceAll(fictionalDefinition, "SITE", s.URL)
	cases := map[string]string{
		"usenet":       strings.Replace(base, "type: private", "type: usenet", 1),
		"captcha":      strings.Replace(base, "  method: post\n", "  method: post\n  captcha:\n    type: image\n    selector: img.captcha\n    input: captcha\n", 1),
		"flaresolverr": strings.Replace(base, "  - {name: notice, type: info, label: Accounts are invite-only.}\n", "  - {name: fs, type: info_flaresolverr, label: x}\n", 1),
		"setting type": strings.Replace(base, "type: password, label: Password", "type: multi-select, label: Password", 1),
	}
	for name, yml := range cases {
		def, err := cdef.Parse([]byte(yml))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		k := newDefinition(def)
		if k.Unsupported == "" {
			t.Errorf("%s: usable: %+v", name, k)
		}
		if _, err := k.Normalize(formspec.Values{}); err == nil {
			t.Errorf("%s: unsupported definition accepted settings", name)
		}
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	good := strings.ReplaceAll(fictionalDefinition, "SITE", "http://examplebay.invalid")
	write("examplebay.yml", good)
	write("again.yaml", good) // same id
	write("broken.yml", "id: [unclosed")
	write("notes.txt", "ignored")
	write("another.yml", strings.NewReplacer("id: examplebay", "id: anothersite", "name: Example Bay", "name: Another Site").Replace(good))

	catalog, errs := LoadDir(dir)
	defs := catalog.List()
	if len(defs) != 2 || defs[0].ID != "anothersite" || defs[1].ID != "examplebay" {
		t.Errorf("definitions = %+v", defs)
	}
	if _, err := catalog.Get("examplebay"); err != nil {
		t.Error(err)
	}
	if _, err := catalog.Get("nowhere"); !errors.Is(err, ErrUnknownDefinition) {
		t.Errorf("unknown = %v", err)
	}
	if len(errs) != 2 {
		t.Fatalf("errs = %v", errs)
	}
	for _, err := range errs {
		var le *LoadError
		if !errors.As(err, &le) {
			t.Errorf("not a LoadError: %v", err)
		}
	}
	if c, errs := LoadDir(filepath.Join(dir, "missing")); len(c.List()) != 0 || errs != nil {
		t.Errorf("missing dir = %v %v", c.List(), errs)
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"1.5 GB":     1610612736,
		"700 MB":     734003200,
		"700MiB":     734003200,
		"1,5 GB":     1610612736,
		"1,234.5 KB": 1264128,
		"1,234 MB":   1293942784,
		"512 B":      512,
		"2048":       2048,
		"3 bytes":    3,
		" 2.00 TB ":  2199023255552,
	}
	for in, want := range cases {
		got, err := parseSize(in)
		if err != nil || got != want {
			t.Errorf("parseSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "big", "1.5 XB", "GB"} {
		if _, err := parseSize(bad); err == nil {
			t.Errorf("parseSize(%q) accepted", bad)
		}
	}
}

// A site that offers only magnet links still yields fetchable releases.
func TestMagnetOnlySite(t *testing.T) {
	s := newSite(t)
	yml := strings.ReplaceAll(fictionalDefinition, "SITE", s.URL)
	yml = strings.Replace(yml, "    download:\n      selector: a.dl\n      attribute: href\n", "", 1)
	def, err := cdef.Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	d := newDefinition(def)
	v, err := d.Normalize(goodSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	srch, err := d.NewSearcher(v, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	rels, err := srch.Search(context.Background(), Query{Type: SearchGeneric, Q: "example"})
	if err != nil {
		t.Fatal(err)
	}
	// Row 1 has no magnet and is dropped; row 2's magnet is used.
	if len(rels) != 1 || !strings.HasPrefix(rels[0].MagnetURI, "magnet:?xt=urn:btih:0123456789abcdef") || rels[0].DownloadURL != "" {
		t.Errorf("releases = %+v", rels)
	}
}

func TestTestSignsIn(t *testing.T) {
	s := newSite(t)
	srch, _ := newTestSearcher(t, s, goodSettings(s))
	if err := srch.Test(context.Background()); err != nil || s.logins != 1 {
		t.Errorf("test = %v, logins %d", err, s.logins)
	}
	bad := goodSettings(s)
	bad["password"] = "wrong"
	srch, _ = newTestSearcher(t, s, bad)
	if err := srch.Test(context.Background()); err == nil {
		t.Error("test passed with a wrong password")
	}
}

// A parent category (2000, what arrd asks for by default) selects the
// site's categories mapped to its children (2040 -> 11), not TV's.
func TestParentCategoryExpands(t *testing.T) {
	s := newSite(t)
	srch, _ := newTestSearcher(t, s, goodSettings(s))
	if _, err := srch.Search(context.Background(), Query{Type: SearchGeneric, Q: "example", Categories: []int{2000}}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if q := s.queries[0]; !strings.Contains(q, "cats=11&") && !strings.HasSuffix(q, "cats=11") {
		t.Errorf("query = %s", q)
	}
}

// Like Nyaa: the download field is a template that is empty when the user
// prefers magnet links, so the magnet must be used instead.
func TestEmptyDownloadFallsBackToMagnet(t *testing.T) {
	s := newSite(t)
	yml := strings.ReplaceAll(fictionalDefinition, "SITE", s.URL)
	yml = strings.Replace(yml,
		"    download:\n      selector: a.dl\n      attribute: href\n",
		"    download:\n      text: \"{{ if .Config.prefer_magnet }}{{ else }}{{ .Result.title }}{{ end }}\"\n      optional: true\n", 1)
	yml = strings.Replace(yml, "settings:\n", "settings:\n  - {name: prefer_magnet, type: checkbox, label: Prefer magnet links, default: true}\n", 1)
	def, err := cdef.Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	d := newDefinition(def)
	v, err := d.Normalize(goodSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	srch, err := d.NewSearcher(v, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	rels, err := srch.Search(context.Background(), Query{Type: SearchGeneric, Q: "example"})
	if err != nil {
		t.Fatal(err)
	}
	// Row 1 has no magnet and is dropped; row 2's magnet is used.
	if len(rels) != 1 || !strings.HasPrefix(rels[0].MagnetURI, "magnet:?xt=urn:btih:0123456789abcdef") || rels[0].DownloadURL != "" {
		t.Errorf("releases = %+v", rels)
	}
}
