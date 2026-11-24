package acquire_test

import (
	"context"
	"errors"
	"fmt"
	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/metadata"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withPopular serves TMDB's popular lists in front of another fake TMDB.
func withPopular(t *testing.T, next *httptest.Server) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/movie/popular":
			fmt.Fprint(w, `{"page":1,"results":[{"id":900001,"title":"The Glass Orchard","original_title":"The Glass Orchard","release_date":"1999-03-31"},
				{"id":900002,"title":"Paper Lanterns","original_title":"Paper Lanterns","release_date":"2031-01-01"}]}`)
		case "/search/movie":
			fmt.Fprint(w, `{"page":1,"results":[{"id":900001,"title":"The Glass Orchard","original_title":"The Glass Orchard","release_date":"1999-03-31","genre_ids":[101],"original_language":"qq"}]}`)
		case "/tv/popular":
			fmt.Fprint(w, `{"page":1,"results":[{"id":800001,"name":"Harbor Watch","first_air_date":"2019-09-01"}]}`)
		default:
			http.Redirect(w, r, next.URL+r.URL.String(), http.StatusTemporaryRedirect)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverMovie(t *testing.T) {
	ctx := context.Background()
	magnet := libtest.SeedMovie(t)
	idx := libtest.FakeIndexer(t, magnet)
	tmdb := withPopular(t, libtest.FakeTMDB(t))
	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)
	svc := env.Svc
	env.UseSites(t, idx.URL+"/")
	ix, err := svc.Indexers.CreateIndexer(ctx, libtest.SiteIndexer("good", idx.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Indexers.TestIndexer(ctx, ix.ID); err != nil {
		t.Fatal(err)
	}

	popular, err := svc.Media.TMDBMovieList(ctx, metadata.MoviePopular)
	if err != nil || len(popular) != 2 || popular[0].TMDBID != 900001 || popular[0].LibraryID != nil {
		t.Fatalf("popular = %+v, %v", popular, err)
	}

	// Searching does not add the movie.
	res, err := svc.Acquire.SearchDiscoveredMovie(ctx, 900001, "")
	if err != nil || len(res.Releases) == 0 || res.Releases[0].Title != libtest.ReleaseName || !res.Releases[0].Accepted {
		t.Fatalf("search = %+v, %v", res, err)
	}
	if movies, _ := svc.Media.Movies(ctx); len(movies) != 0 {
		t.Fatalf("search added movies: %+v", movies)
	}

	// Grabbing adds it, unmonitored, and starts the download.
	dl, err := svc.Acquire.GrabDiscoveredMovie(ctx, 900001, res.Releases[0])
	if err != nil {
		t.Fatal(err)
	}
	movies, _ := svc.Media.Movies(ctx)
	if len(movies) != 1 || movies[0].TMDBID != 900001 || movies[0].Monitored || dl.MovieID == nil || *dl.MovieID != movies[0].ID {
		t.Fatalf("after grab: movies = %+v, download = %+v", movies, dl)
	}
	if m, err := svc.Media.DiscoveredMovie(ctx, 900001); err != nil || m.LibraryID == nil || *m.LibraryID != movies[0].ID || m.IMDBID != "tt9999901" {
		t.Errorf("discovered = %+v, %v", m, err)
	}
	if popular, _ := svc.Media.TMDBMovieList(ctx, metadata.MoviePopular); popular[0].LibraryID == nil || popular[1].LibraryID != nil {
		t.Errorf("popular after grab = %+v", popular)
	}
	if found, err := svc.Media.LookupMovies(ctx, "glass orchard", 0, media.TitleFilter{}); err != nil || len(found) != 1 || found[0].LibraryID == nil || *found[0].LibraryID != movies[0].ID {
		t.Errorf("lookup after grab = %+v, %v", found, err)
	}
	if found, err := svc.Media.LookupMovies(ctx, "glass orchard", 0, media.TitleFilter{GenreIDs: []int64{101}, OriginalLanguage: "qq"}); err != nil || len(found) != 1 {
		t.Errorf("lookup with matching filter = %+v, %v", found, err)
	}
	if found, err := svc.Media.LookupMovies(ctx, "glass orchard", 0, media.TitleFilter{OriginalLanguage: "zz"}); err != nil || len(found) != 0 {
		t.Errorf("lookup with other language = %+v, %v", found, err)
	}
	// A second grab of the same torrent reuses the library movie and is refused as a duplicate.
	if _, err := svc.Acquire.GrabDiscoveredMovie(ctx, 900001, res.Releases[0]); err == nil {
		t.Error("grabbing the same torrent twice succeeded")
	}
	if movies, _ := svc.Media.Movies(ctx); len(movies) != 1 {
		t.Errorf("movies after second grab = %d", len(movies))
	}

	if _, err := svc.Media.DiscoveredMovie(ctx, 123); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("unknown movie = %v", err)
	}
}

func TestDiscoverSeries(t *testing.T) {
	ctx := context.Background()
	packMagnet, singleMagnet := libtest.SeedTV(t)
	idx := libtest.FakeTVIndexer(t, packMagnet, singleMagnet)
	tmdb := withPopular(t, libtest.FakeTVTMDB(t))
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

	popular, err := svc.Media.TMDBSeriesList(ctx, metadata.SeriesPopular)
	if err != nil || len(popular) != 1 || popular[0].TMDBID != 800001 {
		t.Fatalf("popular = %+v, %v", popular, err)
	}
	show, err := svc.Media.DiscoveredSeries(ctx, 800001)
	if err != nil || len(show.Seasons) != 3 || show.LibraryID != nil {
		t.Fatalf("series = %+v, %v", show, err)
	}

	// Season 1 is searched by TVDB id; the pack and a single episode are both accepted.
	res, err := svc.Acquire.SearchDiscoveredSeason(ctx, 800001, 1, "")
	if err != nil || len(res.Releases) != 2 || res.Releases[0].Title != libtest.PackName || !res.Releases[0].SeasonPack || !res.Releases[0].Accepted {
		t.Fatalf("season search = %+v, %v", res, err)
	}
	if _, err := svc.Acquire.SearchDiscoveredSeason(ctx, 800001, 9, ""); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("missing season = %v", err)
	}
	season, err := svc.Media.DiscoveredSeason(ctx, 800001, 1)
	if err != nil || len(season.Episodes) != 3 || season.Episodes[1].Title != "Low Tide" {
		t.Fatalf("season = %+v, %v", season, err)
	}
	if _, err := svc.Media.DiscoveredSeason(ctx, 800001, 9); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("missing season details = %v", err)
	}

	// One episode: only that episode is accepted, the season pack is not.
	epRes, err := svc.Acquire.SearchDiscoveredEpisode(ctx, 800001, 1, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	accepted := map[string]bool{}
	for _, r := range epRes.Releases {
		accepted[r.Title] = r.Accepted
	}
	if !accepted["Harbor.Watch.S01E02.720p.HDTV.x264-PIKA"] || accepted[libtest.PackName] {
		t.Errorf("episode search = %+v", epRes.Releases)
	}
	if _, err := svc.Acquire.SearchDiscoveredEpisode(ctx, 800001, 1, 9, ""); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("missing episode = %v", err)
	}
	if list, _ := svc.Media.SeriesList(ctx); len(list) != 0 {
		t.Fatalf("search added series: %+v", list)
	}

	dl, err := svc.Acquire.GrabDiscoveredSeries(ctx, 800001, res.Releases[0])
	if err != nil {
		t.Fatal(err)
	}
	list, _ := svc.Media.SeriesList(ctx)
	if len(list) != 1 || list[0].Monitored || dl.SeriesID == nil || *dl.SeriesID != list[0].ID {
		t.Fatalf("after grab: series = %+v, download = %+v", list, dl)
	}
	if q, _ := svc.Acquire.Queue(ctx, acquire.QueueFilter{SeriesID: &list[0].ID}); len(q) != 1 || len(q[0].Episodes) != 3 {
		t.Errorf("queue = %+v", q)
	}
	// Once in the library, the season is searched as a library season.
	if _, err := svc.Acquire.SearchDiscoveredSeason(ctx, 800001, 1, ""); err != nil {
		t.Errorf("search of library series = %v", err)
	}
	if _, err := svc.Acquire.SearchDiscoveredEpisode(ctx, 800001, 1, 2, ""); err != nil {
		t.Errorf("episode search of library series = %v", err)
	}
	if show, _ := svc.Media.DiscoveredSeries(ctx, 800001); show.LibraryID == nil || *show.LibraryID != list[0].ID {
		t.Errorf("discovered after grab = %+v", show)
	}
}
