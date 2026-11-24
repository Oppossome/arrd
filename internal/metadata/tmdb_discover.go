package metadata

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Discovery: TMDB's curated lists (trending, popular, ...), its genres, and
// languages, and its /discover endpoint for browsing by genre and language.

// MovieList names one of TMDB's curated movie lists.
type MovieList string

const (
	MovieTrending   MovieList = "trending"
	MoviePopular    MovieList = "popular"
	MovieNowPlaying MovieList = "now_playing"
	MovieUpcoming   MovieList = "upcoming"
	MovieTopRated   MovieList = "top_rated"
)

var movieListPaths = map[MovieList]string{
	MovieTrending:   "/trending/movie/week",
	MoviePopular:    "/movie/popular",
	MovieNowPlaying: "/movie/now_playing",
	MovieUpcoming:   "/movie/upcoming",
	MovieTopRated:   "/movie/top_rated",
}

// SeriesList names one of TMDB's curated TV lists.
type SeriesList string

const (
	SeriesTrending    SeriesList = "trending"
	SeriesPopular     SeriesList = "popular"
	SeriesAiringToday SeriesList = "airing_today"
	SeriesOnTheAir    SeriesList = "on_the_air"
	SeriesTopRated    SeriesList = "top_rated"
)

var seriesListPaths = map[SeriesList]string{
	SeriesTrending:    "/trending/tv/week",
	SeriesPopular:     "/tv/popular",
	SeriesAiringToday: "/tv/airing_today",
	SeriesOnTheAir:    "/tv/on_the_air",
	SeriesTopRated:    "/tv/top_rated",
}

// MovieList fetches the first page (about 20) of a curated movie list.
func (c *TMDB) MovieList(ctx context.Context, list MovieList) ([]Movie, error) {
	path, ok := movieListPaths[list]
	if !ok {
		return nil, fmt.Errorf("tmdb: unknown movie list %q", list)
	}
	return c.movieList(ctx, path)
}

// SeriesList fetches the first page (about 20) of a curated TV list.
func (c *TMDB) SeriesList(ctx context.Context, list SeriesList) ([]Series, error) {
	path, ok := seriesListPaths[list]
	if !ok {
		return nil, fmt.Errorf("tmdb: unknown series list %q", list)
	}
	return c.seriesList(ctx, path)
}

// Genre is a TMDB genre.
type Genre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// MovieGenres lists TMDB's movie genres.
func (c *TMDB) MovieGenres(ctx context.Context) ([]Genre, error) {
	return c.genres(ctx, "/genre/movie/list")
}

// SeriesGenres lists TMDB's TV genres.
func (c *TMDB) SeriesGenres(ctx context.Context) ([]Genre, error) {
	return c.genres(ctx, "/genre/tv/list")
}

func (c *TMDB) genres(ctx context.Context, path string) ([]Genre, error) {
	var resp struct {
		Genres []Genre `json:"genres"`
	}
	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Genres, nil
}

// Language is an original language TMDB knows.
type Language struct {
	Code string // ISO 639-1
	Name string // English name
}

// Languages lists the languages TMDB knows, sorted by English name. Entries
// without an English name (TMDB has a few placeholders) are left out.
func (c *TMDB) Languages(ctx context.Context) ([]Language, error) {
	var resp []struct {
		Code        string `json:"iso_639_1"`
		EnglishName string `json:"english_name"`
	}
	if err := c.get(ctx, "/configuration/languages", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Language, 0, len(resp))
	for _, l := range resp {
		if l.Code == "" || l.EnglishName == "" || l.EnglishName == "No Language" {
			continue
		}
		out = append(out, Language{Code: l.Code, Name: l.EnglishName})
	}
	slices.SortFunc(out, func(a, b Language) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// DiscoverSort orders a browse.
type DiscoverSort string

const (
	SortPopular  DiscoverSort = "popular"
	SortTopRated DiscoverSort = "top_rated"
	SortNewest   DiscoverSort = "newest"
)

// minTopRatedVotes keeps titles with a handful of perfect votes out of "top rated".
const minTopRatedVotes = 200

// MaxDiscoverPage is the last page TMDB serves; it rejects higher ones.
const MaxDiscoverPage = 500

// DiscoverQuery is a browse of TMDB's catalogue.
type DiscoverQuery struct {
	// GenreIDs narrows to titles with any of these genres; empty for every genre.
	GenreIDs []int64
	// OriginalLanguage narrows to one ISO 639-1 original language; empty for every language.
	OriginalLanguage string
	Sort             DiscoverSort
	Page             int // from 1
	// Today bounds "newest", so titles not yet released are left out.
	Today time.Time
}

// Page is one page of a TMDB result list.
type Page[T any] struct {
	Results    []T
	Page       int
	TotalPages int // at most MaxDiscoverPage
}

// DiscoverMovies browses movies by genre and sort order.
func (c *TMDB) DiscoverMovies(ctx context.Context, q DiscoverQuery) (Page[Movie], error) {
	params, err := q.params("primary_release_date")
	if err != nil {
		return Page[Movie]{}, err
	}
	return fetchPage(ctx, c, "/discover/movie", params, tmdbMovie.toMovie)
}

// DiscoverSeries browses TV series by genre and sort order.
func (c *TMDB) DiscoverSeries(ctx context.Context, q DiscoverQuery) (Page[Series], error) {
	params, err := q.params("first_air_date")
	if err != nil {
		return Page[Series]{}, err
	}
	return fetchPage(ctx, c, "/discover/tv", params, tmdbSeries.toSeries)
}

// params turns q into /discover query parameters; dateField is the
// release-date field of the kind being browsed.
func (q DiscoverQuery) params(dateField string) (url.Values, error) {
	if q.Page < 1 || q.Page > MaxDiscoverPage {
		return nil, fmt.Errorf("tmdb: discover page %d is outside 1-%d", q.Page, MaxDiscoverPage)
	}
	p := url.Values{"page": {strconv.Itoa(q.Page)}}
	if len(q.GenreIDs) > 0 {
		ids := make([]string, len(q.GenreIDs))
		for i, id := range q.GenreIDs {
			ids[i] = strconv.FormatInt(id, 10)
		}
		// TMDB reads "|" as OR and "," as AND.
		p.Set("with_genres", strings.Join(ids, "|"))
	}
	if q.OriginalLanguage != "" {
		p.Set("with_original_language", q.OriginalLanguage)
	}
	switch q.Sort {
	case SortPopular:
		p.Set("sort_by", "popularity.desc")
	case SortTopRated:
		p.Set("sort_by", "vote_average.desc")
		p.Set("vote_count.gte", strconv.Itoa(minTopRatedVotes))
	case SortNewest:
		if q.Today.IsZero() {
			return nil, fmt.Errorf("tmdb: sorting by newest needs today's date")
		}
		p.Set("sort_by", dateField+".desc")
		p.Set(dateField+".lte", q.Today.Format(time.DateOnly))
	default:
		return nil, fmt.Errorf("tmdb: unknown sort %q", q.Sort)
	}
	return p, nil
}

func (c *TMDB) movieList(ctx context.Context, path string) ([]Movie, error) {
	p, err := fetchPage(ctx, c, path, nil, tmdbMovie.toMovie)
	return p.Results, err
}

func (c *TMDB) seriesList(ctx context.Context, path string) ([]Series, error) {
	p, err := fetchPage(ctx, c, path, nil, tmdbSeries.toSeries)
	return p.Results, err
}

// fetchPage fetches one page of a TMDB result list, converting each result.
func fetchPage[R, T any](ctx context.Context, c *TMDB, path string, params url.Values, convert func(R) T) (Page[T], error) {
	var resp struct {
		Results    []R `json:"results"`
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	}
	if err := c.get(ctx, path, params, &resp); err != nil {
		return Page[T]{}, err
	}
	out := Page[T]{Results: make([]T, 0, len(resp.Results)), Page: resp.Page, TotalPages: min(resp.TotalPages, MaxDiscoverPage)}
	for _, r := range resp.Results {
		out.Results = append(out.Results, convert(r))
	}
	return out, nil
}
