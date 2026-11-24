package library_test

import (
	"context"
	"fmt"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitFor(t *testing.T, svc *library.Services, what string, done func() bool) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(60 * time.Second)
	for !done() {
		if err := svc.Acquire.ProcessDownloads(ctx); err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			q, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
			t.Fatalf("timed out waiting for %s; queue = %+v", what, q)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSeriesEndToEnd(t *testing.T) {
	ctx := context.Background()
	packMagnet, singleMagnet := libtest.SeedTV(t)
	idx := libtest.FakeTVIndexer(t, packMagnet, singleMagnet)
	tmdb := libtest.FakeTVTMDB(t)

	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc
	env.SetNow(func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) })

	env.UseSites(t, idx.URL+"/")
	ix, err := svc.Indexers.CreateIndexer(ctx, libtest.SiteIndexer("tv", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Indexers.TestIndexer(ctx, ix.ID); err != nil {
		t.Fatal(err)
	}

	series, err := svc.Media.AddSeries(ctx, media.AddSeriesInput{TMDBID: 800001})
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
	if err := svc.Acquire.SearchWantedEpisodes(ctx, &series.ID); err != nil {
		t.Fatal(err)
	}
	queue, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
	if len(queue) != 1 || queue[0].ReleaseTitle != libtest.PackName || queue[0].SeriesID == nil {
		t.Fatalf("after search, queue = %+v", queue)
	}
	if queue[0].Title != "Harbor Watch" || len(queue[0].Episodes) != 3 || queue[0].Episodes[0] != "S01E01" {
		t.Errorf("queue row = %+v", queue[0])
	}
	if mine, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{SeriesID: &series.ID}); len(mine) != 1 {
		t.Errorf("series filter = %d rows", len(mine))
	}

	episodes := func() map[string]media.Episode {
		eps, err := svc.Media.SeriesEpisodes(ctx, series.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]media.Episode{}
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
		src := filepath.Join(base, "complete", libtest.PackName, fmt.Sprintf("Harbor.Watch.%s.1080p.WEB-DL.x264-QUILL.mkv", code))
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
	rss, err := svc.Acquire.RSSSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rss.Releases != 4 || len(rss.Grabbed) != 1 || rss.Grabbed[0].ReleaseTitle != libtest.SingleName {
		t.Fatalf("rss = %+v", rss)
	}
	// Running it again while downloading grabs nothing more.
	if again, _ := svc.Acquire.RSSSync(ctx); len(again.Grabbed) != 0 {
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
	if wanted, _ := env.Store.ListWantedEpisodes(ctx, svc.Media.Today()); len(wanted) != 0 {
		t.Errorf("still wanted: %+v", wanted)
	}
	detail, _ := svc.Media.SeriesDetail(ctx, series.ID)
	// Season 2 is "complete": every episode aired so far has a file.
	if !detail.Seasons[1].Complete || !detail.Seasons[2].Complete || detail.Seasons[2].Downloaded != 1 || detail.Seasons[2].Episodes != 2 {
		t.Errorf("season status = %+v", detail.Seasons)
	}
	queue, _ = svc.Acquire.Queue(ctx, acquire.QueueFilter{})
	for _, d := range queue {
		if d.Status != media.DownloadImported || d.Error != nil {
			t.Errorf("download %s: status %s, error %v", d.ReleaseTitle, d.Status, d.Error)
		}
	}
}
