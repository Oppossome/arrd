// Package mediaservertest fakes Plex and Jellyfin servers for tests.
package mediaservertest

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Token is the credential the fakes accept.
const Token = "fake-media-token"

// Fake is a fake media server that records the requests it accepts.
type Fake struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string // "METHOD /path" of authorized requests
	status   int      // when non-zero, every request fails with it
}

// Requests returns the authorized requests so far, as "METHOD /path".
func (f *Fake) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

// Fail makes every request answer status; 0 restores normal answers.
func (f *Fake) Fail(status int) {
	f.mu.Lock()
	f.status = status
	f.mu.Unlock()
}

func newFake(t *testing.T, authorized func(*http.Request) bool, routes map[string]string) *Fake {
	t.Helper()
	f := &Fake{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		status := f.status
		f.mu.Unlock()
		if status != 0 {
			http.Error(w, http.StatusText(status), status)
			return
		}
		if !authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		key := r.Method + " " + r.URL.Path
		body, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, key)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.Close)
	return f
}

// Plex fakes a Plex server with two library sections, keys "1" and "7".
func Plex(t *testing.T) *Fake {
	return newFake(t,
		func(r *http.Request) bool {
			return r.Header.Get("X-Plex-Token") == Token && r.Header.Get("Accept") == "application/json"
		},
		map[string]string{
			"GET /library/sections": `{"MediaContainer":{"size":2,"Directory":[` +
				`{"key":"1","type":"movie","title":"Films"},{"key":"7","type":"show","title":"Shows"}]}}`,
			"GET /library/sections/1/refresh": "",
			"GET /library/sections/7/refresh": "",
		})
}

// Jellyfin fakes a Jellyfin server.
func Jellyfin(t *testing.T) *Fake {
	return newFake(t,
		func(r *http.Request) bool { return r.Header.Get("Authorization") == `MediaBrowser Token="`+Token+`"` },
		map[string]string{
			"GET /System/Info":      `{"ServerName":"fake","Version":"10.0.0"}`,
			"POST /Library/Refresh": "",
		})
}
