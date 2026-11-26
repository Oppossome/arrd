package library_test

import (
	"context"
	"errors"
	"fmt"
	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/testutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// siteDefinition is a fictional private tracker in Cardigann format; SITE
// is the test server and NAME its display name.
const siteDefinition = `---
id: examplebay
name: NAME
description: A fictional private tracker.
language: en-US
type: private
encoding: UTF-8
links: [SITE/]
caps:
  categorymappings:
    - {id: 11, cat: Movies/HD, desc: "Films HD"}
    - {id: 22, cat: TV/HD, desc: "Series HD"}
  modes:
    search: [q]
    movie-search: [q, imdbid]
settings:
  - {name: username, type: text, label: Username}
  - {name: password, type: password, label: Password}
login:
  path: login.php
  method: post
  inputs:
    username: "{{ .Config.username }}"
    password: "{{ .Config.password }}"
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
  rows:
    selector: tr.row
  fields:
    title: {selector: a.title}
    details: {selector: a.title, attribute: href}
    download: {selector: a.dl, attribute: href}
    size: {selector: td.size}
    seeders: {selector: td.seeders}
    leechers: {selector: td.leechers}
`

// privateSite serves siteDefinition's tracker: one movie release whose
// .torrent can only be fetched with the session cookie.
type privateSite struct {
	*httptest.Server
	logins  atomic.Int32
	queries chan string
}

func newPrivateSite(t *testing.T, torrentFile []byte) *privateSite {
	t.Helper()
	s := &privateSite{queries: make(chan string, 16)}
	authed := func(r *http.Request) bool {
		c, err := r.Cookie("session")
		return err == nil && c.Value == "ok"
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login.php":
			if r.Method == http.MethodPost && r.FormValue("username") == "tester" && r.FormValue("password") == "fictional-pass" {
				s.logins.Add(1)
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
				http.Redirect(w, r, "/index.php", http.StatusFound)
				return
			}
			testutil.Write(t, w, `<div class="error">Wrong password</div>`)
		case "/index.php":
			if authed(r) {
				testutil.Write(t, w, `<a href="logout.php">Log out</a>`)
			}
		case "/browse.php":
			if !authed(r) {
				http.Redirect(w, r, "/login.php", http.StatusFound)
				return
			}
			s.queries <- r.URL.RawQuery
			testutil.Write(t, w, fmt.Sprintf(`<table><tr class="row"><td><a class="title" href="details.php?id=7">%s</a></td>
<td><a class="dl" href="download.php?id=7">get</a></td><td class="size">60 MB</td><td class="seeders">9</td><td class="leechers">1</td></tr></table>`, libtest.ReleaseName))
		case "/download.php":
			if !authed(r) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			testutil.Write(t, w, string(torrentFile))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

// smallTorrent is a valid .torrent for a one-file release.
func smallTorrent(t *testing.T) []byte {
	t.Helper()
	root := filepath.Join(t.TempDir(), libtest.ReleaseName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "movie.mkv"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	info := metainfo.Info{PieceLength: 1 << 14}
	if err := info.BuildFromFilePath(root); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	out, err := bencode.Marshal(metainfo.MetaInfo{InfoBytes: infoBytes})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeDefinition(t *testing.T, env *libtest.Env, site *privateSite, name string) {
	t.Helper()
	dir := filepath.Join(env.DataDir, "definitions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReplacer("SITE", site.URL, "NAME", name).Replace(siteDefinition)
	if err := os.WriteFile(filepath.Join(dir, "examplebay.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateSiteEndToEnd(t *testing.T) {
	ctx := context.Background()
	site := newPrivateSite(t, smallTorrent(t))
	tmdb := libtest.FakeTMDB(t)
	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc

	// Definitions are picked up on reload.
	writeDefinition(t, env, site, "Example Bay")
	if err := os.WriteFile(filepath.Join(env.DataDir, "definitions", "broken.yml"), []byte("id: [oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	reload := svc.Indexers.ReloadIndexerDefinitions()
	// Sorted by name: "Example Bay" before libtest.NewEnv's "Other Site" and "Test Site".
	if len(reload.Definitions) != 3 || reload.Definitions[0].ID != "examplebay" || len(reload.Errors) != 1 || !strings.Contains(reload.Errors[0], "broken.yml") {
		t.Fatalf("reload = %+v", reload)
	}

	in := indexers.IndexerInput{
		Name: "bay", Definition: "examplebay",
		Settings: formspec.Values{"base_url": site.URL + "/", "username": "tester", "password": "fictional-pass"},
	}
	// The site cannot list recent releases, so RSS cannot be turned on.
	rssOn := true
	withRSS := in
	withRSS.EnableRSS = &rssOn
	var ve *apperr.ValidationError
	if _, err := svc.Indexers.CreateIndexer(ctx, withRSS); !errors.As(err, &ve) || ve.Field != "enable_rss" {
		t.Fatalf("create with rss = %v", err)
	}
	ix, err := svc.Indexers.CreateIndexer(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if ix.EnableRSS || ix.Settings["password"] != "" || len(ix.SecretsSet) != 1 {
		t.Errorf("indexer = %+v", ix)
	}
	if ix, err = svc.Indexers.TestIndexer(ctx, ix.ID); err != nil || ix.Caps == nil || !ix.Caps.MovieSearch.Supports("imdbid") {
		t.Fatalf("test = %+v, %v", ix, err)
	}

	movie, err := svc.Media.AddMovie(ctx, media.AddMovieInput{TMDBID: 900001})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Acquire.SearchMovie(ctx, movie.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Releases) != 1 || !res.Releases[0].Accepted || res.Releases[0].Size != 60<<20 {
		t.Fatalf("search = %+v", res)
	}
	<-site.queries // TestIndexer's query
	q := <-site.queries
	if !strings.Contains(q, "imdb=9999901") || !strings.Contains(q, "cats=11") {
		t.Errorf("search query = %s", q)
	}

	// The .torrent needs the signed-in session the search used.
	dl, err := svc.Acquire.GrabMovieRelease(ctx, movie.ID, res.Releases[0])
	if err != nil {
		t.Fatal(err)
	}
	if dl.ReleaseTitle != libtest.ReleaseName {
		t.Errorf("download = %+v", dl)
	}
	if n := site.logins.Load(); n != 1 {
		t.Errorf("signed in %d times across test, search and grab; want 1", n)
	}

	// A changed definition gets a fresh searcher (and session).
	writeDefinition(t, env, site, "Example Bay v2")
	svc.Indexers.ReloadIndexerDefinitions()
	if _, err := svc.Acquire.SearchMovie(ctx, movie.ID, ""); err != nil {
		t.Fatal(err)
	}
	if n := site.logins.Load(); n != 2 {
		t.Errorf("logins after reload = %d, want 2", n)
	}

	// RSS sync skips it rather than failing it.
	rss, err := svc.Acquire.RSSSync(ctx)
	if err != nil || len(rss.Failures) != 0 {
		t.Errorf("rss = %+v, %v", rss, err)
	}
}
