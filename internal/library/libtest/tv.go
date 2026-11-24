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

const (
	PackName   = "Harbor.Watch.S01.1080p.WEB-DL.x264-QUILL"
	SingleName = "Harbor.Watch.S02E01.1080p.WEB-DL.x264-QUILL.mkv"
)

// seedPath seeds the file or folder at dir/name and returns a magnet that
// points at the seeder.
func seedPath(t *testing.T, seeder *torrent.Engine, dir, name string) string {
	t.Helper()
	info := metainfo.Info{PieceLength: 1 << 20}
	if err := info.BuildFromFilePath(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	dl, err := seeder.AddMetaInfo(&metainfo.MetaInfo{InfoBytes: infoBytes}, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := dl.WaitComplete(ctx); err != nil {
		t.Fatalf("seeding %s: %v", name, err)
	}
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&x.pe=%s", dl.InfoHash(), url.QueryEscape(name),
		url.QueryEscape("127.0.0.1:"+strconv.Itoa(seeder.ListenPort())))
}

func sparseFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
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

// SeedTV seeds a season 1 pack and a single season 2 episode.
func SeedTV(t *testing.T) (packMagnet, singleMagnet string) {
	t.Helper()
	dir := t.TempDir()
	for ep := 1; ep <= 3; ep++ {
		sparseFile(t, filepath.Join(dir, PackName, fmt.Sprintf("Harbor.Watch.S01E%02d.1080p.WEB-DL.x264-QUILL.mkv", ep)), int64(25+ep)<<20)
	}
	sparseFile(t, filepath.Join(dir, PackName, "Sample", "harbor.watch.s01e01.sample.mkv"), 25<<20)
	sparseFile(t, filepath.Join(dir, SingleName), 30<<20)

	seeder, err := torrent.New(torrent.Config{StateDir: t.TempDir(), Network: V4Only{}, Seed: true}, QuietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(seeder.Close)
	return seedPath(t, seeder, dir, PackName), seedPath(t, seeder, dir, SingleName)
}

func FakeTVIndexer(t *testing.T, packMagnet, singleMagnet string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	row := func(title, path string, size int64) SiteRow {
		return SiteRow{Title: title, Link: srv.URL + path, Size: size, Seeders: 7}
	}
	srv = FakeSite(t, func(q url.Values) []SiteRow {
		switch {
		case q.Get("t") == "tvsearch" && q.Get("tvdbid") == "7700001" && q.Get("season") == "1":
			return []SiteRow{
				row(PackName, "/dl/pack", 80<<20),
				row("Harbor.Watch.S01E02.720p.HDTV.x264-PIKA", "/dl/none", 20<<20),
			}
		case q.Get("t") == "tvsearch":
			return nil // nothing for season 2 yet
		case q.Get("t") == "search" && q.Get("q") == "": // RSS (and the test's query)
			return []SiteRow{
				row(SingleName, "/dl/single", 30<<20),
				row(PackName, "/dl/pack", 80<<20), // already have season 1
				row("Other.Show.S02E01.1080p.WEB-DL-G", "/dl/none", 30<<20),
				row("Harbor.Watch.S02E02.1080p.WEB-DL-G", "/dl/none", 30<<20), // not aired
			}
		default:
			t.Errorf("unexpected search %s", q.Encode())
			return nil
		}
	}, map[string]http.HandlerFunc{"/dl/pack": RedirectTo(packMagnet), "/dl/single": RedirectTo(singleMagnet)})
	return srv
}

func FakeTVTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tv/800001":
			_, _ = fmt.Fprint(w, `{"id":800001,"name":"Harbor Watch","first_air_date":"2019-09-01","status":"Returning Series",
				"seasons":[{"season_number":0,"episode_count":1},{"season_number":1,"episode_count":3},{"season_number":2,"episode_count":2}],
				"external_ids":{"tvdb_id":7700001,"imdb_id":"tt9999902"}}`)
		case "/tv/800001/season/0":
			_, _ = fmt.Fprint(w, `{"episodes":[{"season_number":0,"episode_number":1,"name":"Behind the Docks","air_date":"2019-12-01"}]}`)
		case "/tv/800001/season/1":
			_, _ = fmt.Fprint(w, `{"episodes":[
				{"season_number":1,"episode_number":1,"name":"Pilot","air_date":"2019-09-01"},
				{"season_number":1,"episode_number":2,"name":"Low Tide","air_date":"2019-09-08"},
				{"season_number":1,"episode_number":3,"name":"Fog Bank","air_date":"2019-09-15"}]}`)
		case "/tv/800001/season/2":
			_, _ = fmt.Fprint(w, `{"episodes":[
				{"season_number":2,"episode_number":1,"name":"Return","air_date":"2024-06-10"},
				{"season_number":2,"episode_number":2,"name":"Undertow","air_date":"2024-07-01"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}
