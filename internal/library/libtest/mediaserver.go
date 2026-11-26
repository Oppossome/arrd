package libtest

import (
	"context"
	"testing"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/library/mediaservers"
	"generic-arr-project/internal/mediaserver/mediaservertest"
)

// AddJellyfin adds a fake Jellyfin server as a media server.
func AddJellyfin(t *testing.T, svc *library.Services) *mediaservertest.Fake {
	t.Helper()
	fake := mediaservertest.Jellyfin(t)
	_, err := svc.MediaServers.Create(context.Background(), mediaservers.Input{
		Definition: "jellyfin", Settings: formspec.Values{"url": fake.URL, "api_key": mediaservertest.Token},
	})
	if err != nil {
		t.Fatal(err)
	}
	return fake
}

// WantRescans runs the scheduler's media server job and checks fake has
// been asked to rescan want times in all.
func WantRescans(t *testing.T, svc *library.Services, fake *mediaservertest.Fake, want int) {
	t.Helper()
	if err := svc.MediaServers.FlushPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range fake.Requests() {
		if r == "POST /Library/Refresh" {
			n++
		}
	}
	if n != want {
		t.Errorf("media server rescans = %d, want %d (requests %q)", n, want, fake.Requests())
	}
}
