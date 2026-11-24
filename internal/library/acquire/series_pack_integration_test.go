package acquire_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/libtest"
)

func TestSeriesPacks(t *testing.T) {
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

	// Not in the library: searched from TMDB's seasons, by title.
	res, err := svc.Acquire.SearchDiscoveredSeriesPacks(ctx, 800001)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]acquire.ReleaseResult{}
	for _, r := range res.Releases {
		byTitle[r.Title] = r
	}
	multi := byTitle[libtest.MultiSeasonName]
	if res.Releases[0].Title != libtest.MultiSeasonName || !multi.Accepted || *multi.Season != 1 || multi.LastSeason == nil || *multi.LastSeason != 2 {
		t.Fatalf("multi-season pack = %+v; ranked %+v", multi, res.Releases)
	}
	if c := byTitle["Harbor.Watch.Complete.Series.720p.HDTV.x264-PIKA"]; !c.Accepted || !c.CompleteSeries || c.LastSeason != nil {
		t.Errorf("complete series = %+v", c)
	}
	for title, why := range map[string]string{libtest.PackName: "only season 1", libtest.SingleName: "individual episodes"} {
		if r := byTitle[title]; r.Accepted || !strings.Contains(strings.Join(r.Rejections, ";"), why) {
			t.Errorf("%s = %+v, want rejected for %q", title, r, why)
		}
	}

	// Grabbing adds the series and expects every regular episode of both
	// seasons (not the special).
	dl, err := svc.Acquire.GrabDiscoveredSeries(ctx, 800001, multi)
	if err != nil {
		t.Fatal(err)
	}
	queue, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{})
	if len(queue) != 1 || queue[0].ID != dl.ID {
		t.Fatalf("queue = %+v", queue)
	}
	if want := []string{"S01E01", "S01E02", "S01E03", "S02E01", "S02E02"}; !slices.Equal(queue[0].Episodes, want) {
		t.Errorf("expected episodes = %v, want %v", queue[0].Episodes, want)
	}

	// In the library, the search is the same.
	if dl.SeriesID == nil {
		t.Fatal("download has no series")
	}
	lib, err := svc.Acquire.SearchSeriesPacks(ctx, *dl.SeriesID)
	if err != nil || len(lib.Releases) != len(res.Releases) || lib.Releases[0].Title != libtest.MultiSeasonName {
		t.Errorf("library search = %+v, %v", lib, err)
	}
}
