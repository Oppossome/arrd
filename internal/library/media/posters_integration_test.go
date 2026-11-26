package media_test

import (
	"context"
	"errors"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/testutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPosterProxy(t *testing.T) {
	ctx := context.Background()
	var hits atomic.Int32
	tmdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/w342/poster1.jpg":
			time.Sleep(50 * time.Millisecond) // let concurrent requests pile up
			w.Header().Set("Content-Type", "image/jpeg")
			testutil.Write(t, w, "JPEGDATA")
		case "/w342/html.jpg":
			w.Header().Set("Content-Type", "text/html")
			testutil.Write(t, w, "<html>")
		default:
			http.NotFound(w, r)
		}
	}))
	defer tmdb.Close()

	base := t.TempDir()
	env := libtest.NewEnv(t, base, "http://unused", libtest.BasicSetup(base), nil, func(d *library.Deps) { d.PosterBaseURL = tmdb.URL })

	var wg sync.WaitGroup
	paths := make([]string, 5)
	errs := make([]error, 5)
	for i := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths[i], errs[i] = env.Svc.Media.Poster(ctx, "w342", "/poster1.jpg")
		}()
	}
	wg.Wait()
	for i := range paths {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
	}
	if b, err := os.ReadFile(paths[0]); err != nil || string(b) != "JPEGDATA" {
		t.Fatalf("cached poster = %q, %v", b, err)
	}
	if _, err := env.Svc.Media.Poster(ctx, "w342", "poster1.jpg"); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("TMDB hit %d times, want 1 (dedup + cache)", n)
	}

	for _, c := range []struct{ size, file string }{
		{"original", "poster1.jpg"},
		{"w342", "../../arrd.db"},
		{"w342", "a/b.jpg"},
		{"w342", "poster1.gif"},
	} {
		var ve *apperr.ValidationError
		if _, err := env.Svc.Media.Poster(ctx, c.size, c.file); !errors.As(err, &ve) {
			t.Errorf("Poster(%q, %q) = %v, want validation error", c.size, c.file, err)
		}
	}
	if _, err := env.Svc.Media.Poster(ctx, "w342", "html.jpg"); err == nil {
		t.Error("non-image response cached")
	}
	if _, err := env.Svc.Media.Poster(ctx, "w342", "missing.jpg"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("missing poster = %v, want not found", err)
	}
}
