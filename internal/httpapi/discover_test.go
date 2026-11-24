package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The literal discover routes sit beside /{tmdbId}: each must resolve to its
// own operation, in the spec router (request validation) and in the mux.
func TestDiscoverRoutes(t *testing.T) {
	_, router, err := loadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"/api/v1/discover/languages":                                   "discoverLanguages",
		"/api/v1/discover/lists/movies/popular":                        "movieList",
		"/api/v1/discover/movies/genres":                               "movieGenres",
		"/api/v1/discover/movies/browse":                               "browseMovies",
		"/api/v1/discover/movies/recommendations":                      "movieRecommendations",
		"/api/v1/discover/movies/900001":                               "discoverMovie",
		"/api/v1/discover/movies/900001/recommendations":               "moviesLike",
		"/api/v1/discover/lists/series/on_the_air":                     "seriesList",
		"/api/v1/discover/series/genres":                               "seriesGenres",
		"/api/v1/discover/series/browse":                               "browseSeries",
		"/api/v1/discover/series/recommendations":                      "seriesRecommendations",
		"/api/v1/discover/series/800001/recommendations":               "seriesLike",
		"/api/v1/discover/series/800001/seasons/1":                     "discoverSeason",
		"/api/v1/discover/series/800001/seasons/1/episodes/2/releases": "discoverEpisodeReleases",
	} {
		route, _, err := router.FindRoute(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if !strings.EqualFold(route.Operation.OperationID, want) {
			t.Errorf("%s routes to %s, want %s", path, route.Operation.OperationID, want)
		}
	}
}

func TestDiscoverRejectsBadParams(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, path := range []string{
		"/api/v1/discover/lists/movies/bogus",
		"/api/v1/discover/lists/series/now_playing", // a movie-only list
		"/api/v1/discover/movies/browse?sort=random",
		"/api/v1/discover/movies/browse?page=0",
		"/api/v1/discover/series/browse?page=501",
		"/api/v1/discover/series/browse?genre=0",
		"/api/v1/discover/series/browse?genre=16&genre=0",
		"/api/v1/discover/movies/browse?lang=JA",
		"/api/v1/discover/movies/browse?lang=jpn",
		"/api/v1/movies/lookup?q=orchard&lang=j",
		"/api/v1/series/lookup?q=harbor&genre=zero",
		"/api/v1/discover/series/800001/seasons/1/episodes/0/releases",
	} {
		if code, body := do(t, srv, "GET", path, "", true); code != http.StatusBadRequest {
			t.Errorf("%s = %d %v, want 400", path, code, body)
		}
	}
}
