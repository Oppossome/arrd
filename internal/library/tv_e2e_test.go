package library

import (
	"context"
	"fmt"
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

	"generic-arr-project/internal/torrent"
)

const (
	packName   = "Harbor.Watch.S01.1080p.WEB-DL.x264-QUILL"
	singleName = "Harbor.Watch.S02E01.1080p.WEB-DL.x264-QUILL.mkv"
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
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}

// seedTV seeds a season 1 pack and a single season 2 episode.
func seedTV(t *testing.T) (packMagnet, singleMagnet string) {
	t.Helper()
	dir := t.TempDir()
	for ep := 1; ep <= 3; ep++ {
		sparseFile(t, filepath.Join(dir, packName, fmt.Sprintf("Harbor.Watch.S01E%02d.1080p.WEB-DL.x264-QUILL.mkv", ep)), int64(25+ep)<<20)
	}
	sparseFile(t, filepath.Join(dir, packName, "Sample", "harbor.watch.s01e01.sample.mkv"), 25<<20)
	sparseFile(t, filepath.Join(dir, singleName), 30<<20)

	seeder, err := torrent.New(torrent.Config{StateDir: t.TempDir(), Network: v4Only{}, Seed: true}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(seeder.Close)
	return seedPath(t, seeder, dir, packName), seedPath(t, seeder, dir, singleName)
}

func fakeTVIndexer(t *testing.T, packMagnet, singleMagnet string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	row := func(title, path string, size int64) siteRow {
		return siteRow{Title: title, Link: srv.URL + path, Size: size, Seeders: 7}
	}
	srv = fakeSite(t, func(q url.Values) []siteRow {
		switch {
		case q.Get("t") == "tvsearch" && q.Get("tvdbid") == "7700001" && q.Get("season") == "1":
			return []siteRow{
				row(packName, "/dl/pack", 80<<20),
				row("Harbor.Watch.S01E02.720p.HDTV.x264-PIKA", "/dl/none", 20<<20),
			}
		case q.Get("t") == "tvsearch":
			return nil // nothing for season 2 yet
		case q.Get("t") == "search" && q.Get("q") == "": // RSS (and the test's query)
			return []siteRow{
				row(singleName, "/dl/single", 30<<20),
				row(packName, "/dl/pack", 80<<20), // already have season 1
				row("Other.Show.S02E01.1080p.WEB-DL-G", "/dl/none", 30<<20),
				row("Harbor.Watch.S02E02.1080p.WEB-DL-G", "/dl/none", 30<<20), // not aired
			}
		default:
			t.Errorf("unexpected search %s", q.Encode())
			return nil
		}
	}, map[string]http.HandlerFunc{"/dl/pack": redirectTo(packMagnet), "/dl/single": redirectTo(singleMagnet)})
	return srv
}

func fakeTVTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tv/800001":
			fmt.Fprint(w, `{"id":800001,"name":"Harbor Watch","first_air_date":"2019-09-01","status":"Returning Series",
				"seasons":[{"season_number":0,"episode_count":1},{"season_number":1,"episode_count":3},{"season_number":2,"episode_count":2}],
				"external_ids":{"tvdb_id":7700001,"imdb_id":"tt9999902"}}`)
		case "/tv/800001/season/0":
			fmt.Fprint(w, `{"episodes":[{"season_number":0,"episode_number":1,"name":"Behind the Docks","air_date":"2019-12-01"}]}`)
		case "/tv/800001/season/1":
			fmt.Fprint(w, `{"episodes":[
				{"season_number":1,"episode_number":1,"name":"Pilot","air_date":"2019-09-01"},
				{"season_number":1,"episode_number":2,"name":"Low Tide","air_date":"2019-09-08"},
				{"season_number":1,"episode_number":3,"name":"Fog Bank","air_date":"2019-09-15"}]}`)
		case "/tv/800001/season/2":
			fmt.Fprint(w, `{"episodes":[
				{"season_number":2,"episode_number":1,"name":"Return","air_date":"2024-06-10"},
				{"season_number":2,"episode_number":2,"name":"Undertow","air_date":"2024-07-01"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitFor(t *testing.T, svc *Service, what string, done func() bool) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(60 * time.Second)
	for !done() {
		if err := svc.ProcessDownloads(ctx); err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			q, _ := svc.Queue(ctx, QueueFilter{})
			t.Fatalf("timed out waiting for %s; queue = %+v", what, q)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSeriesEndToEnd(t *testing.T) {
	ctx := context.Background()
	packMagnet, singleMagnet := seedTV(t)
	idx := fakeTVIndexer(t, packMagnet, singleMagnet)
	tmdb := fakeTVTMDB(t)

	base := t.TempDir()
	env := newEnv(t, base, tmdb.URL, basicSetup(base), nil)
	svc := env.svc
	svc.now = func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) }

	env.useSites(t, idx.URL+"/")
	ix, err := svc.CreateIndexer(ctx, siteIndexer("tv", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestIndexer(ctx, ix.ID); err != nil {
		t.Fatal(err)
	}

	series, err := svc.AddSeries(ctx, AddSeriesInput{TMDBID: 800001})
	if err != nil {
		t.Fatal(err)
	}
	if series.TVDBID == nil || *series.TVDBID != 7700001 || len(series.Seasons) != 3 {
		t.Fatalf("series = %+v", series)
	}
	s1, s2 := series.Seasons[1], series.Seasons[2]
	if series.Seasons[0].Monitored != 0 || s1.Episodes != 3 || s1.Aired != 3 || s2.Aired != 1 || s2.Monitored != 2 {
		t.Fatalf("seasons = %+v", series.Seasons)
	}

	// Season 1 is fully aired with 3 wanted episodes: the pack is grabbed.
	// Season 2 has one aired episode, which no search result offers.
	if err := svc.SearchWantedEpisodes(ctx, &series.ID); err != nil {
		t.Fatal(err)
	}
	queue, _ := svc.Queue(ctx, QueueFilter{})
	if len(queue) != 1 || queue[0].ReleaseTitle != packName || queue[0].SeriesID == nil {
		t.Fatalf("after search, queue = %+v", queue)
	}
	if queue[0].Title != "Harbor Watch" || len(queue[0].Episodes) != 3 || queue[0].Episodes[0] != "S01E01" {
		t.Errorf("queue row = %+v", queue[0])
	}
	if mine, _ := svc.Queue(ctx, QueueFilter{SeriesID: &series.ID}); len(mine) != 1 {
		t.Errorf("series filter = %d rows", len(mine))
	}

	episodes := func() map[string]Episode {
		eps, err := svc.SeriesEpisodes(ctx, series.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]Episode{}
		for _, e := range eps {
			out[fmt.Sprintf("S%02dE%02d", e.Season, e.Episode)] = e
		}
		return out
	}
	waitFor(t, svc, "season pack import", func() bool { return episodes()["S01E03"].FilePath != nil })

	showDir := filepath.Join(base, "TV", "Harbor Watch (2019)")
	eps := episodes()
	for code, title := range map[string]string{"S01E01": "Pilot", "S01E02": "Low Tide", "S01E03": "Fog Bank"} {
		want := filepath.Join(showDir, "Season 01", fmt.Sprintf("Harbor Watch - %s - %s [WEBDL-1080p].mkv", code, title))
		if eps[code].FilePath == nil || *eps[code].FilePath != want {
			t.Errorf("%s file = %v, want %s", code, eps[code].FilePath, want)
			continue
		}
		src := filepath.Join(base, "complete", packName, fmt.Sprintf("Harbor.Watch.%s.1080p.WEB-DL.x264-QUILL.mkv", code))
		a, errA := os.Stat(want)
		b, errB := os.Stat(src)
		if errA != nil || errB != nil || !os.SameFile(a, b) {
			t.Errorf("%s is not a hardlink of the seeding copy (%v, %v)", code, errA, errB)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(showDir, "Season 01")); len(entries) != 3 {
		t.Errorf("season folder has %d entries, want 3 (sample skipped)", len(entries))
	}

	// RSS: the newly aired S02E01 is grabbed; the old pack, another show and
	// the unaired S02E02 are ignored.
	rss, err := svc.RSSSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rss.Releases != 4 || len(rss.Grabbed) != 1 || rss.Grabbed[0].ReleaseTitle != singleName {
		t.Fatalf("rss = %+v", rss)
	}
	// Running it again while downloading grabs nothing more.
	if again, _ := svc.RSSSync(ctx); len(again.Grabbed) != 0 {
		t.Errorf("second rss sync grabbed %+v", again.Grabbed)
	}
	waitFor(t, svc, "single episode import", func() bool { return episodes()["S02E01"].FilePath != nil })

	eps = episodes()
	want := filepath.Join(showDir, "Season 02", "Harbor Watch - S02E01 - Return [WEBDL-1080p].mkv")
	if *eps["S02E01"].FilePath != want {
		t.Errorf("S02E01 file = %s, want %s", *eps["S02E01"].FilePath, want)
	}
	if eps["S02E02"].FilePath != nil || eps["S00E01"].FilePath != nil {
		t.Error("unaired or unmonitored episode got a file")
	}
	if wanted, _ := env.store.ListWantedEpisodes(ctx, svc.today()); len(wanted) != 0 {
		t.Errorf("still wanted: %+v", wanted)
	}
	detail, _ := svc.SeriesDetail(ctx, series.ID)
	// Season 2 is "complete": every episode aired so far has a file.
	if !detail.Seasons[1].Complete || !detail.Seasons[2].Complete || detail.Seasons[2].Downloaded != 1 || detail.Seasons[2].Episodes != 2 {
		t.Errorf("season status = %+v", detail.Seasons)
	}
	queue, _ = svc.Queue(ctx, QueueFilter{})
	for _, d := range queue {
		if d.Status != statusImported || d.Error != nil {
			t.Errorf("download %s: status %s, error %v", d.ReleaseTitle, d.Status, d.Error)
		}
	}
}
