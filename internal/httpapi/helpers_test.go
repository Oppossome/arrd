package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"generic-arr-project/internal/library"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/torrent"
)

type v4Only struct{ netx.Direct }

func (v4Only) HasIPv6() bool { return false }

const testSetupToken = "setup-token"

// testServer is an httptest server plus what tests need to call it.
type testServer struct {
	*httptest.Server
	base   string
	apiKey string
}

// newUnconfiguredServer starts arrd's API before onboarding.
func newUnconfiguredServer(t *testing.T) *testServer {
	t.Helper()
	return newUnconfiguredServerUI(t, nil)
}

// newUnconfiguredServerUI is newUnconfiguredServer with an embedded UI.
func newUnconfiguredServerUI(t *testing.T, ui fs.FS) *testServer {
	t.Helper()
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, filepath.Join(dataDir, "arrd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	factory := func(context.Context, engine.Settings) (*engine.Runtime, error) {
		eng, err := torrent.New(torrent.Config{StateDir: filepath.Join(dataDir, "state"), Network: v4Only{}}, log)
		if err != nil {
			return nil, err
		}
		return &engine.Runtime{Engine: eng, Close: eng.Close}, nil
	}
	defsDir := filepath.Join(dataDir, "definitions")
	if err := os.MkdirAll(defsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defsDir, "examplesite.yml"), []byte(testSiteDefinition), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := library.New(library.Deps{Store: st, NewRuntime: factory, DataDir: dataDir, DefinitionsDir: defsDir, Log: log, Direct: v4Only{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Stop)
	h, err := New(svc, testSetupToken, log, ui)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	checkContract(t, srv.Client())
	return &testServer{Server: srv, base: base}
}

func setupBody(base string) string {
	return fmt.Sprintf(`{"admin_password":"correct horse battery","tmdb_api_key":"k","downloads":{"incomplete_dir":%q,"complete_dir":%q},"movie_root":%q,"create_missing":true}`,
		filepath.Join(base, "data", "dl", "incomplete"), filepath.Join(base, "data", "dl", "complete"), filepath.Join(base, "Movies"))
}

// newTestServer returns a server that has completed onboarding over HTTP.
func newTestServer(t *testing.T) (*testServer, string) {
	t.Helper()
	srv := newUnconfiguredServer(t)
	code, body := doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), map[string]string{"X-Setup-Token": testSetupToken})
	if code != http.StatusCreated {
		t.Fatalf("setup = %d %v", code, body)
	}
	srv.apiKey = body["api_key"].(string)
	return srv, srv.base
}

func doWith(t *testing.T, srv *testServer, method, path, body string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
	}
	return resp.StatusCode, out
}

func do(t *testing.T, srv *testServer, method, path, body string, auth bool) (int, map[string]any) {
	t.Helper()
	headers := map[string]string{}
	if auth {
		headers["X-Api-Key"] = srv.apiKey
	}
	return doWith(t, srv, method, path, body, headers)
}

// rawDo returns the status and body of a request.
func rawDo(t *testing.T, srv *testServer, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// browser is an HTTP client with a cookie jar that sends Origin like a
// browser does for same-origin fetches.
type browser struct {
	t      *testing.T
	srv    *testServer
	client *http.Client
}

func newBrowser(t *testing.T, srv *testServer) *browser {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := *srv.Client()
	c.Jar = jar
	return &browser{t: t, srv: srv, client: &c}
}

func (b *browser) do(method, path, body string, headers map[string]string) *http.Response {
	b.t.Helper()
	req, err := http.NewRequest(method, b.srv.URL+path, strings.NewReader(body))
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("Origin", b.srv.URL)
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp
}

// testSiteDefinition is a fictional public tracker every test server knows.
const testSiteDefinition = `---
id: examplesite
name: Example Site
description: A fictional public tracker for tests.
language: en-US
type: public
encoding: UTF-8
links: ["http://site.invalid/"]
caps:
  categorymappings:
    - {id: 1, cat: Movies/HD, desc: Movies}
  modes:
    search: [q]
  allowrawsearch: true
settings:
  - {name: apikey, type: password, label: API key}
search:
  paths:
    - path: search
  inputs:
    q: "{{ .Keywords }}"
  rows:
    selector: tr.row
  fields:
    title: {selector: a.title}
    download: {selector: a.dl, attribute: href}
    size: {selector: td.size}
    seeders: {selector: td.seeders}
`
