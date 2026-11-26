package mediaservers_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/mediaservers"
	"generic-arr-project/internal/mediaserver/mediaservertest"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"

	"github.com/rs/zerolog"
)

func newService(t *testing.T) *mediaservers.Service {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "arrd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	reg, err := mediaservers.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := mediaservers.New(mediaservers.Deps{Store: st, Registry: reg, Direct: netx.Direct{}, Log: new(zerolog.Nop())})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func jellyfinInput(url string) mediaservers.Input {
	return mediaservers.Input{Definition: "jellyfin", Settings: formspec.Values{"url": url, "api_key": mediaservertest.Token}}
}

func TestCreateRedactsAndUpdateKeepsSecrets(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	m, err := svc.Create(ctx, jellyfinInput("http://media.example:8096"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Jellyfin" || !m.Enabled || m.Settings["api_key"] != "" || !slices.Equal(m.SecretsSet, []string{"api_key"}) {
		t.Fatalf("created = %+v", m)
	}

	// The form sends the secret back empty: the saved one is kept.
	m, err = svc.Update(ctx, m.ID, mediaservers.Input{Name: "Living room", Settings: formspec.Values{"url": "http://media.example:9000", "api_key": ""}})
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Living room" || m.Settings["url"] != "http://media.example:9000" || !slices.Equal(m.SecretsSet, []string{"api_key"}) {
		t.Fatalf("updated = %+v", m)
	}

	// Switching off alone keeps the settings.
	off := false
	m, err = svc.Update(ctx, m.ID, mediaservers.Input{Enabled: &off})
	if err != nil || m.Enabled || m.Settings["url"] != "http://media.example:9000" {
		t.Fatalf("disabled = %+v, %v", m, err)
	}
}

func TestCreateValidates(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	cases := map[string]mediaservers.Input{
		"definition":       {Settings: formspec.Values{"url": "http://media.example"}},
		"settings.url":     {Definition: "plex", Settings: formspec.Values{"url": "media.example", "token": "x"}},
		"settings.token":   {Definition: "plex", Settings: formspec.Values{"url": "http://media.example"}},
		"settings.unknown": {Definition: "plex", Settings: formspec.Values{"url": "http://media.example", "token": "x", "unknown": "1"}},
	}
	for field, in := range cases {
		_, err := svc.Create(ctx, in)
		var ve *apperr.ValidationError
		if !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: err = %v", field, err)
		}
	}
	if _, err := svc.Create(ctx, jellyfinInput("http://media.example")); err != nil {
		t.Fatal(err)
	}
	var ce *apperr.ConflictError
	if _, err := svc.Create(ctx, jellyfinInput("http://other.example")); !errors.As(err, &ce) {
		t.Errorf("duplicate name: err = %v", err)
	}
}

func TestFlushPending(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	jf := mediaservertest.Jellyfin(t)
	broken := mediaservertest.Jellyfin(t)
	broken.Fail(http.StatusServiceUnavailable)
	off := mediaservertest.Jellyfin(t)

	good, err := svc.Create(ctx, jellyfinInput(jf.URL))
	if err != nil {
		t.Fatal(err)
	}
	bad := jellyfinInput(broken.URL)
	bad.Name = "Broken"
	badServer, err := svc.Create(ctx, bad)
	if err != nil {
		t.Fatal(err)
	}
	disabled := jellyfinInput(off.URL)
	disabled.Name, disabled.Enabled = "Off", new(bool)
	if _, err := svc.Create(ctx, disabled); err != nil {
		t.Fatal(err)
	}

	// Nothing imported: nothing is told.
	if err := svc.FlushPending(ctx); err != nil || len(jf.Requests()) != 0 {
		t.Fatalf("idle flush: err = %v, requests = %v", err, jf.Requests())
	}

	// Several imports make one rescan; a failing server does not stop the others.
	svc.LibraryChanged()
	svc.LibraryChanged()
	if err := svc.FlushPending(ctx); err == nil {
		t.Error("broken server's failure was not returned")
	}
	if got := jf.Requests(); !slices.Equal(got, []string{"POST /Library/Refresh"}) {
		t.Errorf("good server requests = %q", got)
	}
	if len(off.Requests()) != 0 {
		t.Error("disabled server was told")
	}
	if m, _ := svc.Get(ctx, badServer.ID); m.LastError == nil || m.LastErrorAt == nil {
		t.Errorf("broken server's error not recorded: %+v", m)
	}
	if m, _ := svc.Get(ctx, good.ID); m.LastError != nil {
		t.Errorf("good server has error %q", *m.LastError)
	}

	// The flag is cleared: the next flush is idle.
	if err := svc.FlushPending(ctx); err != nil || len(jf.Requests()) != 1 {
		t.Errorf("second flush: err = %v, requests = %v", err, jf.Requests())
	}

	// A passing test clears the recorded error.
	broken.Fail(0)
	if m, err := svc.Test(ctx, badServer.ID); err != nil || m.LastError != nil {
		t.Errorf("test after recovery = %+v, %v", m, err)
	}
}

func TestTestRecordsFailure(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	m, err := svc.Create(ctx, mediaservers.Input{Definition: "plex", Settings: formspec.Values{"url": "http://127.0.0.1:1", "token": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Test(ctx, m.ID); err == nil {
		t.Fatal("test of an unreachable server passed")
	}
	if m, _ = svc.Get(ctx, m.ID); m.LastError == nil {
		t.Error("failure not recorded")
	}
	if _, err := svc.Test(ctx, m.ID+1); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("unknown id: err = %v", err)
	}
}
