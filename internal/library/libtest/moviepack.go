package libtest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"generic-arr-project/internal/torrent"
)

// The fictional "Skyfarer" collection: three released movies and one
// announced. The seeded pack holds only the first two, plus a sample and a
// bonus feature that matches no movie.
const (
	MoviePackName       = "Skyfarer.Trilogy.2011-2015.1080p.BluRay.x264-QUILL"
	MoviePackFirstFile  = "Skyfarer.2011.1080p.BluRay.x264-QUILL.mkv"
	MoviePackSecondFile = "Skyfarer.Ascent.2013.1080p.BluRay.x264-QUILL.mkv"

	SkyfarerTMDBID   = 900011 // "Skyfarer" (2011), the movie packs are grabbed from
	SkyfarerCollID   = 700001
	SkyfarerMovies   = 3 // released movies in the collection
	SkyfarerQuery    = "Skyfarer"
	SkyfarerSingle   = "Skyfarer.2011.1080p.WEB-DL.x264-PIKA"
	SkyfarerWrongHit = "Moth.Girl.Trilogy.1080p.BluRay.x264-QUILL"
)

// SeedMoviePack seeds MoviePackName and returns a magnet that points at
// the seeder.
func SeedMoviePack(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, MoviePackName)
	sparseFile(t, filepath.Join(root, MoviePackFirstFile), 55<<20)
	sparseFile(t, filepath.Join(root, MoviePackSecondFile), 56<<20)
	sparseFile(t, filepath.Join(root, "Sample", "skyfarer.2011.sample.mkv"), 52<<20)
	sparseFile(t, filepath.Join(root, "Extras", "Behind.The.Clouds.mkv"), 51<<20)

	seeder, err := torrent.New(torrent.Config{StateDir: t.TempDir(), Network: V4Only{}, Seed: true}, QuietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(seeder.Close)
	return seedPath(t, seeder, dir, MoviePackName)
}

// FakeMoviePackIndexer answers a collection search for "Skyfarer" with the
// pack, a single movie and another franchise's pack.
func FakeMoviePackIndexer(t *testing.T, packMagnet string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = FakeSite(t, func(q url.Values) []SiteRow {
		switch {
		case q.Get("t") == "movie" && q.Get("q") == SkyfarerQuery && q.Get("imdbid") == "":
			return []SiteRow{
				{Title: SkyfarerWrongHit, Link: srv.URL + "/dl/none", Size: 30 << 30, Seeders: 40},
				{Title: SkyfarerSingle, Link: srv.URL + "/dl/none", Size: 6 << 30, Seeders: 20},
				{Title: MoviePackName, Link: srv.URL + "/dl/pack", Size: 24 << 30, Seeders: 9},
			}
		case q.Get("t") == "search" && q.Get("q") == "": // the indexer test's query
			return nil
		default:
			t.Errorf("unexpected search %s", q.Encode())
			return nil
		}
	}, map[string]http.HandlerFunc{"/dl/pack": RedirectTo(packMagnet)})
	return srv
}

// FakeMoviePackTMDB serves the Skyfarer movies and their collection.
func FakeMoviePackTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	movie := func(id int, title, date string) string {
		return fmt.Sprintf(`{"id":%d,"imdb_id":"tt99%05d","title":%q,"original_title":%q,"release_date":%q,`+
			`"belongs_to_collection":{"id":%d,"name":"Skyfarer Collection"}}`, id, id%100000, title, title, date, SkyfarerCollID)
	}
	pages := map[string]string{
		"/movie/900011": movie(900011, "Skyfarer", "2011-05-20"),
		"/movie/900012": movie(900012, "Skyfarer: Ascent", "2013-06-07"),
		"/movie/900013": movie(900013, "Skyfarer: Homeward", "2015-07-03"),
		"/movie/900014": movie(900014, "Skyfarer: Horizon", "2031-01-01"),
		fmt.Sprintf("/collection/%d", SkyfarerCollID): `{"id":700001,"name":"Skyfarer Collection","parts":[
			{"id":900014,"title":"Skyfarer: Horizon","release_date":"2031-01-01"},
			{"id":900013,"title":"Skyfarer: Homeward","release_date":"2015-07-03"},
			{"id":900012,"title":"Skyfarer: Ascent","release_date":"2013-06-07"},
			{"id":900011,"title":"Skyfarer","release_date":"2011-05-20"}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}
