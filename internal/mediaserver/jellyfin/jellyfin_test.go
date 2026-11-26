package jellyfin_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/mediaserver"
	"generic-arr-project/internal/mediaserver/jellyfin"
	"generic-arr-project/internal/mediaserver/mediaservertest"
)

func parse(t *testing.T, url, key string) mediaserver.Server {
	t.Helper()
	srv, err := jellyfin.Definition().Parse(formspec.Values{"url": url, "api_key": key}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestRefreshLibrary(t *testing.T) {
	fake := mediaservertest.Jellyfin(t)
	if err := parse(t, fake.URL, mediaservertest.Token).RefreshLibrary(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := fake.Requests(), []string{"POST /Library/Refresh"}; !slices.Equal(got, want) {
		t.Errorf("requests = %q, want %q", got, want)
	}
}

func TestTestChecksKey(t *testing.T) {
	fake := mediaservertest.Jellyfin(t)
	if err := parse(t, fake.URL, mediaservertest.Token).Test(context.Background()); err != nil {
		t.Errorf("good key: %v", err)
	}
	err := parse(t, fake.URL, "wrong-key").Test(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "wrong-key") {
		t.Errorf("bad key: %v", err)
	}
}

func TestBaseURLWithPath(t *testing.T) {
	// Jellyfin behind a reverse proxy at a sub-path.
	fake := mediaservertest.Jellyfin(t)
	err := parse(t, fake.URL+"/jellyfin/", mediaservertest.Token).RefreshLibrary(context.Background())
	if err == nil || !strings.Contains(err.Error(), "POST /jellyfin/Library/Refresh") {
		t.Errorf("err = %v; want the sub-path kept", err)
	}
}

func TestUnreachable(t *testing.T) {
	err := parse(t, "http://127.0.0.1:1", mediaservertest.Token).RefreshLibrary(context.Background())
	if err == nil || strings.Contains(err.Error(), mediaservertest.Token) {
		t.Errorf("err = %v", err)
	}
}
