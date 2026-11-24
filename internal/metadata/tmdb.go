// Package metadata looks up movie and TV information from TMDB.
package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// DefaultTMDBBaseURL is the TMDB v3 API root.
const DefaultTMDBBaseURL = "https://api.themoviedb.org/3"

// ErrNotFound is returned when TMDB has no such item.
var ErrNotFound = errors.New("not found on TMDB")

// TMDB is a TMDB API client.
type TMDB struct {
	// APIKey is either a v3 API key or a v4 read access token (JWT).
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

// Movie is TMDB's view of a movie.
type Movie struct {
	TMDBID        int64  `json:"tmdb_id"`
	IMDBID        string `json:"imdb_id,omitempty"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Year          int    `json:"year"` // 0 if unreleased/unknown
	Overview      string `json:"overview"`
	PosterPath    string `json:"poster_path,omitempty"`
	BackdropPath  string `json:"backdrop_path,omitempty"`
	// GenreIDs and OriginalLanguage (ISO 639-1) come with search, list and
	// discover results, not with GetMovie.
	GenreIDs         []int64 `json:"genre_ids,omitempty"`
	OriginalLanguage string  `json:"original_language,omitempty"`
	Extras
}

// SearchMovies searches by title, optionally narrowed by year (0 = any).
func (c *TMDB) SearchMovies(ctx context.Context, query string, year int) ([]Movie, error) {
	params := url.Values{"query": {query}, "include_adult": {"false"}}
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}
	var resp struct {
		Results []tmdbMovie `json:"results"`
	}
	if err := c.get(ctx, "/search/movie", params, &resp); err != nil {
		return nil, err
	}
	out := make([]Movie, 0, len(resp.Results))
	for _, m := range resp.Results {
		out = append(out, m.toMovie())
	}
	return out, nil
}

// GetMovie fetches one movie, including its IMDb ID and Extras.
func (c *TMDB) GetMovie(ctx context.Context, tmdbID int64) (*Movie, error) {
	var m tmdbMovie
	params := url.Values{"append_to_response": {"videos,credits,release_dates"}}
	if err := c.get(ctx, "/movie/"+strconv.FormatInt(tmdbID, 10), params, &m); err != nil {
		return nil, err
	}
	movie := m.toMovie()
	return &movie, nil
}

type tmdbMovie struct {
	ID               int64   `json:"id"`
	IMDBID           string  `json:"imdb_id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	ReleaseDate      string  `json:"release_date"`
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	GenreIDs         []int64 `json:"genre_ids"`
	OriginalLanguage string  `json:"original_language"`
	// Detail-only fields (GetMovie).
	Tagline      string           `json:"tagline"`
	Runtime      int              `json:"runtime"`
	VoteAverage  float64          `json:"vote_average"`
	VoteCount    int64            `json:"vote_count"`
	Genres       []Genre          `json:"genres"`
	Videos       tmdbVideos       `json:"videos"`
	Credits      tmdbCredits      `json:"credits"`
	ReleaseDates tmdbReleaseDates `json:"release_dates"`
}

func (m tmdbMovie) toMovie() Movie {
	year := 0
	if len(m.ReleaseDate) >= 4 {
		year, _ = strconv.Atoi(m.ReleaseDate[:4])
	}
	return Movie{
		TMDBID: m.ID, IMDBID: m.IMDBID, Title: m.Title, OriginalTitle: m.OriginalTitle,
		Year: year, Overview: m.Overview, PosterPath: m.PosterPath, BackdropPath: m.BackdropPath,
		GenreIDs: m.GenreIDs, OriginalLanguage: m.OriginalLanguage,
		Extras: Extras{
			Tagline: m.Tagline, Runtime: m.Runtime, VoteAverage: m.VoteAverage, VoteCount: m.VoteCount,
			Certification: m.ReleaseDates.certification(), Genres: m.Genres,
			Cast: topCast(m.Credits.Cast), Trailer: pickTrailer(m.Videos.Results),
		},
	}
}

func (c *TMDB) get(ctx context.Context, path string, params url.Values, out any) error {
	if c.APIKey == "" {
		return errors.New("tmdb: metadata.tmdb_api_key is not configured")
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultTMDBBaseURL
	}
	if params == nil {
		params = url.Values{}
	}
	bearer := strings.HasPrefix(c.APIKey, "eyJ") // v4 read access token
	if !bearer {
		params.Set("api_key", c.APIKey)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if bearer {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("tmdb %s: %s", path, strings.ReplaceAll(err.Error(), c.APIKey, "REDACTED"))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("tmdb %s: read body: %w", path, err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode != http.StatusOK:
		var e struct {
			StatusMessage string `json:"status_message"`
		}
		_ = json.Unmarshal(body, &e)
		return fmt.Errorf("tmdb %s: %s: %s", path, resp.Status, e.StatusMessage)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("tmdb %s: decode: %w", path, err)
	}
	return nil
}

// Validate checks that the API key is accepted by TMDB.
func (c *TMDB) Validate(ctx context.Context) error {
	var out struct{}
	return c.get(ctx, "/configuration", nil, &out)
}

// MovieRecommendations lists the movies TMDB recommends to people who liked
// a movie (its first page, about 20). An unknown movie is ErrNotFound.
func (c *TMDB) MovieRecommendations(ctx context.Context, tmdbID int64) ([]Movie, error) {
	return c.movieList(ctx, "/movie/"+strconv.FormatInt(tmdbID, 10)+"/recommendations")
}
