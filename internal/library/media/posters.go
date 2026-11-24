package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/netx"
)

// DefaultPosterBaseURL is TMDB's image CDN.
const DefaultPosterBaseURL = "https://image.tmdb.org/t/p"

const (
	maxPosterBytes = 10 << 20
	posterTimeout  = 30 * time.Second
)

var (
	// w300 is for episode stills; w780 and w1280 are backdrop sizes.
	posterSizes = map[string]bool{"w185": true, "w300": true, "w342": true, "w500": true, "w780": true, "w1280": true}
	// TMDB image files are short random names; anything else is refused so
	// the name can never escape the cache directory.
	posterFile = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}\.(jpg|png)$`)
)

// posterFetcher deduplicates concurrent downloads of the same poster.
type posterFetcher struct {
	mu       sync.Mutex
	inflight map[string]*posterCall
}

type posterCall struct {
	done chan struct{}
	err  error
}

// Poster returns the local path of a cached TMDB poster, downloading it on
// first use. size must be w185, w342, w500 (posters), w300 (stills), w780 or w1280
// (backdrops); file is a TMDB file name such
// as "abc123.jpg" (a leading "/" is accepted).
func (s *Service) Poster(ctx context.Context, size, file string) (string, error) {
	file = strings.TrimPrefix(file, "/")
	if !posterSizes[size] {
		return "", apperr.Invalid("size", "must be w185, w300, w342, w500, w780 or w1280")
	}
	if !posterFile.MatchString(file) {
		return "", apperr.Invalid("file", "not a TMDB image file name")
	}
	path := filepath.Join(s.dataDir, "cache", "posters", size, file)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	key := size + "/" + file
	s.posters.mu.Lock()
	if s.posters.inflight == nil {
		s.posters.inflight = map[string]*posterCall{}
	}
	if call, ok := s.posters.inflight[key]; ok {
		s.posters.mu.Unlock()
		select {
		case <-call.done:
			return path, call.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	call := &posterCall{done: make(chan struct{})}
	s.posters.inflight[key] = call
	s.posters.mu.Unlock()

	// Detached from the request: other waiters may still want the result.
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), posterTimeout)
	call.err = s.fetchPoster(fetchCtx, size, file, path)
	cancel()
	close(call.done)
	s.posters.mu.Lock()
	delete(s.posters.inflight, key)
	s.posters.mu.Unlock()
	return path, call.err
}

func (s *Service) fetchPoster(ctx context.Context, size, file, path string) error {
	url := fmt.Sprintf("%s/%s/%s", s.posterBaseURL, size, file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := netx.HTTPClient(s.direct, posterTimeout).Do(req)
	if err != nil {
		return fmt.Errorf("fetch poster: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("poster %s: %w", file, apperr.ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch poster: TMDB returned %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("fetch poster: unexpected content type %q", ct)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("poster cache: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".poster-*")
	if err != nil {
		return fmt.Errorf("poster cache: %w", err)
	}
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxPosterBytes+1))
	closeErr := tmp.Close()
	switch {
	case err != nil:
		err = fmt.Errorf("download poster: %w", err)
	case closeErr != nil:
		err = fmt.Errorf("poster cache: %w", closeErr)
	case n > maxPosterBytes:
		err = errors.New("poster is larger than 10 MiB")
	}
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("poster cache: %w", err)
	}
	return nil
}
