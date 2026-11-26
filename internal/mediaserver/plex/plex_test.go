package plex_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/mediaserver"
	"generic-arr-project/internal/mediaserver/mediaservertest"
	"generic-arr-project/internal/mediaserver/plex"
)

func parse(t *testing.T, url, token string) mediaserver.Server {
	t.Helper()
	srv, err := plex.Definition().Parse(formspec.Values{"url": url, "token": token}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestRefreshLibraryRefreshesEverySection(t *testing.T) {
	fake := mediaservertest.Plex(t)
	srv := parse(t, fake.URL+"/", mediaservertest.Token)
	if err := srv.RefreshLibrary(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /library/sections", "GET /library/sections/1/refresh", "GET /library/sections/7/refresh"}
	if got := fake.Requests(); !slices.Equal(got, want) {
		t.Errorf("requests = %q, want %q", got, want)
	}
}

func TestTestChecksToken(t *testing.T) {
	fake := mediaservertest.Plex(t)
	if err := parse(t, fake.URL, mediaservertest.Token).Test(context.Background()); err != nil {
		t.Errorf("good token: %v", err)
	}
	err := parse(t, fake.URL, "wrong-token").Test(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("bad token: %v", err)
	}
	if strings.Contains(err.Error(), "wrong-token") {
		t.Errorf("error leaks the token: %v", err)
	}
}

func TestRefreshLibraryReportsServerError(t *testing.T) {
	fake := mediaservertest.Plex(t)
	fake.Fail(http.StatusInternalServerError)
	err := parse(t, fake.URL, mediaservertest.Token).RefreshLibrary(context.Background())
	if err == nil || !strings.Contains(err.Error(), "/library/sections") || !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v", err)
	}
}

func TestParseRejectsBadURL(t *testing.T) {
	for _, u := range []string{"", "localhost:32400", "ftp://media.example", "http://media.example/?x=1"} {
		_, err := plex.Definition().Parse(formspec.Values{"url": u, "token": "t"}, http.DefaultClient)
		var fe *formspec.FieldError
		if !errors.As(err, &fe) || fe.Field != "url" {
			t.Errorf("url %q: err = %v", u, err)
		}
	}
}
