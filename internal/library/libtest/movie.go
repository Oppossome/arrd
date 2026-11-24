package libtest

import (
	"context"
	"fmt"
	"generic-arr-project/internal/torrent"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

const ReleaseName = "The.Glass.Orchard.1999.1080p.BluRay.x264-GRP"

// SeedMovie creates a 60 MiB sparse "movie" plus a sample and an .nfo,
// starts a seeder for it, and returns its magnet link pointing at the seeder.
func SeedMovie(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, ReleaseName)
	files := map[string]int64{"movie.mkv": 60 << 20, "Sample/sample.mkv": 1 << 20, "release.nfo": 512}
	for name, size := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	info := metainfo.Info{PieceLength: 1 << 20}
	if err := info.BuildFromFilePath(root); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: infoBytes}

	seeder, err := torrent.New(torrent.Config{StateDir: t.TempDir(), Network: V4Only{}, Seed: true}, QuietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(seeder.Close)
	dl, err := seeder.AddMetaInfo(mi, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := dl.WaitComplete(ctx); err != nil {
		t.Fatalf("seeder: %v", err)
	}
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&x.pe=%s",
		dl.InfoHash(), url.QueryEscape(ReleaseName), url.QueryEscape("127.0.0.1:"+strconv.Itoa(seeder.ListenPort())))
}

// FakeIndexer serves the test site with a movie search returning one good
// release (whose download link redirects to magnet) and one CAM release.
func FakeIndexer(t *testing.T, magnet string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = FakeSite(t, func(q url.Values) []SiteRow {
		if q.Get("t") != "movie" {
			return nil // the test's query
		}
		if q.Get("imdbid") != "9999901" {
			t.Errorf("site searched without imdb id: %s", q.Encode())
		}
		return []SiteRow{
			{Title: "The.Glass.Orchard.1999.HDCAM-BAD", Link: srv.URL + "/dl/cam", Size: 1000, Seeders: 900},
			{Title: ReleaseName, Link: srv.URL + "/dl/good", Size: 62914560, Seeders: 12},
		}
	}, map[string]http.HandlerFunc{"/dl/good": RedirectTo(magnet)})
	return srv
}

func FakeTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/900001" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":900001,"imdb_id":"tt9999901","title":"The Glass Orchard","original_title":"The Glass Orchard","release_date":"1999-03-31"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// FakeTMDBWithArt serves the libtest movie with a poster and *backdrop.
func FakeTMDBWithArt(t *testing.T, backdrop *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/900001" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":900001,"imdb_id":"tt9999901","title":"The Glass Orchard","original_title":"The Glass Orchard",`+
			`"release_date":"1999-03-31","poster_path":"/orchardposter.jpg","backdrop_path":%q}`, *backdrop)
	}))
	t.Cleanup(srv.Close)
	return srv
}
