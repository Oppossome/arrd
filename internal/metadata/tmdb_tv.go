package metadata

import (
	"context"
	"net/url"
	"strconv"
)

// Series is TMDB's view of a TV series.
type Series struct {
	TMDBID        int64           `json:"tmdb_id"`
	TVDBID        int64           `json:"tvdb_id,omitempty"`
	IMDBID        string          `json:"imdb_id,omitempty"`
	Title         string          `json:"title"`
	OriginalTitle string          `json:"original_title"`
	Year          int             `json:"year"` // first air year; 0 if unknown
	Overview      string          `json:"overview"`
	Status        string          `json:"status,omitempty"` // "Returning Series", "Ended", ...
	PosterPath    string          `json:"poster_path,omitempty"`
	BackdropPath  string          `json:"backdrop_path,omitempty"`
	Seasons       []SeasonSummary `json:"seasons,omitempty"` // only from GetSeries
	// GenreIDs and OriginalLanguage (ISO 639-1) come with search, list and
	// discover results, not with GetSeries.
	GenreIDs         []int64 `json:"genre_ids,omitempty"`
	OriginalLanguage string   `json:"original_language,omitempty"`
	Networks         []string `json:"networks,omitempty"` // only from GetSeries
	Extras
}

// SeasonSummary lists a season without its episodes.
type SeasonSummary struct {
	Number       int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count"`
	Name         string `json:"name,omitempty"`
	PosterPath   string `json:"poster_path,omitempty"`
	AirDate      string `json:"air_date,omitempty"` // YYYY-MM-DD; empty if unknown
}

// Season is one season with its episodes.
type Season struct {
	Number     int       `json:"season_number"`
	Name       string    `json:"name"`
	Overview   string    `json:"overview"`
	PosterPath string    `json:"poster_path,omitempty"`
	AirDate    string    `json:"air_date,omitempty"`
	Episodes   []Episode `json:"episodes"`
}

// Episode is one episode of a season.
type Episode struct {
	Season      int     `json:"season"`
	Number      int     `json:"episode"`
	Title       string  `json:"title"`
	AirDate     string  `json:"air_date,omitempty"` // YYYY-MM-DD; empty if unknown
	Overview    string  `json:"overview"`
	Runtime     int     `json:"runtime,omitempty"` // minutes
	StillPath   string  `json:"still_path,omitempty"`
	VoteAverage float64 `json:"vote_average,omitempty"`
}

type tmdbSeries struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	FirstAirDate     string  `json:"first_air_date"`
	Overview         string  `json:"overview"`
	Status           string  `json:"status"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	GenreIDs         []int64 `json:"genre_ids"`
	OriginalLanguage string  `json:"original_language"`
	Seasons          []struct {
		SeasonNumber int    `json:"season_number"`
		EpisodeCount int    `json:"episode_count"`
		Name         string `json:"name"`
		PosterPath   string `json:"poster_path"`
		AirDate      string `json:"air_date"`
	} `json:"seasons"`
	ExternalIDs struct {
		TVDBID int64  `json:"tvdb_id"`
		IMDBID string `json:"imdb_id"`
	} `json:"external_ids"`
	// Detail-only fields (GetSeries).
	Tagline        string  `json:"tagline"`
	EpisodeRunTime []int   `json:"episode_run_time"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int64   `json:"vote_count"`
	Genres         []Genre `json:"genres"`
	Networks       []struct {
		Name string `json:"name"`
	} `json:"networks"`
	Videos           tmdbVideos         `json:"videos"`
	AggregateCredits tmdbCredits        `json:"aggregate_credits"`
	ContentRatings   tmdbContentRatings `json:"content_ratings"`
}

func (s tmdbSeries) toSeries() Series {
	out := Series{
		TMDBID: s.ID, TVDBID: s.ExternalIDs.TVDBID, IMDBID: s.ExternalIDs.IMDBID,
		Title: s.Name, OriginalTitle: s.OriginalName, Year: yearOf(s.FirstAirDate),
		Overview: s.Overview, Status: s.Status, PosterPath: s.PosterPath, BackdropPath: s.BackdropPath,
		GenreIDs: s.GenreIDs, OriginalLanguage: s.OriginalLanguage,
		Extras: Extras{
			Tagline: s.Tagline, VoteAverage: s.VoteAverage, VoteCount: s.VoteCount,
			Certification: s.ContentRatings.certification(), Genres: s.Genres,
			Cast: topCast(s.AggregateCredits.Cast), Trailer: pickTrailer(s.Videos.Results),
		},
	}
	if len(s.EpisodeRunTime) > 0 {
		out.Runtime = s.EpisodeRunTime[0]
	}
	for _, n := range s.Networks {
		out.Networks = append(out.Networks, n.Name)
	}
	for _, season := range s.Seasons {
		out.Seasons = append(out.Seasons, SeasonSummary{
			Number: season.SeasonNumber, EpisodeCount: season.EpisodeCount,
			Name: season.Name, PosterPath: season.PosterPath, AirDate: season.AirDate,
		})
	}
	return out
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// SearchSeries searches TV series by name, optionally narrowed by first air
// year (0 = any).
func (c *TMDB) SearchSeries(ctx context.Context, query string, year int) ([]Series, error) {
	params := url.Values{"query": {query}, "include_adult": {"false"}}
	if year > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}
	var resp struct {
		Results []tmdbSeries `json:"results"`
	}
	if err := c.get(ctx, "/search/tv", params, &resp); err != nil {
		return nil, err
	}
	out := make([]Series, 0, len(resp.Results))
	for _, s := range resp.Results {
		out = append(out, s.toSeries())
	}
	return out, nil
}

// GetSeries fetches a series with its season list and external IDs.
func (c *TMDB) GetSeries(ctx context.Context, tmdbID int64) (*Series, error) {
	var s tmdbSeries
	params := url.Values{"append_to_response": {"external_ids,videos,aggregate_credits,content_ratings"}}
	if err := c.get(ctx, "/tv/"+strconv.FormatInt(tmdbID, 10), params, &s); err != nil {
		return nil, err
	}
	series := s.toSeries()
	return &series, nil
}

// GetSeason fetches the episodes of one season.
func (c *TMDB) GetSeason(ctx context.Context, tmdbID int64, season int) ([]Episode, error) {
	s, err := c.GetSeasonDetails(ctx, tmdbID, season)
	if err != nil {
		return nil, err
	}
	return s.Episodes, nil
}

// GetSeasonDetails fetches one season with its episodes.
func (c *TMDB) GetSeasonDetails(ctx context.Context, tmdbID int64, season int) (*Season, error) {
	var resp struct {
		SeasonNumber int    `json:"season_number"`
		Name         string `json:"name"`
		Overview     string `json:"overview"`
		PosterPath   string `json:"poster_path"`
		AirDate      string `json:"air_date"`
		Episodes     []struct {
			SeasonNumber  int     `json:"season_number"`
			EpisodeNumber int     `json:"episode_number"`
			Name          string  `json:"name"`
			AirDate       string  `json:"air_date"`
			Overview      string  `json:"overview"`
			Runtime       int     `json:"runtime"`
			StillPath     string  `json:"still_path"`
			VoteAverage   float64 `json:"vote_average"`
		} `json:"episodes"`
	}
	path := "/tv/" + strconv.FormatInt(tmdbID, 10) + "/season/" + strconv.Itoa(season)
	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	out := &Season{
		Number: resp.SeasonNumber, Name: resp.Name, Overview: resp.Overview,
		PosterPath: resp.PosterPath, AirDate: resp.AirDate,
		Episodes: make([]Episode, 0, len(resp.Episodes)),
	}
	for _, e := range resp.Episodes {
		out.Episodes = append(out.Episodes, Episode{
			Season: e.SeasonNumber, Number: e.EpisodeNumber, Title: e.Name, AirDate: e.AirDate,
			Overview: e.Overview, Runtime: e.Runtime, StillPath: e.StillPath, VoteAverage: e.VoteAverage,
		})
	}
	return out, nil
}

// SeriesRecommendations lists the TV series TMDB recommends to people who
// liked a series (its first page, about 20). An unknown series is ErrNotFound.
func (c *TMDB) SeriesRecommendations(ctx context.Context, tmdbID int64) ([]Series, error) {
	return c.seriesList(ctx, "/tv/"+strconv.FormatInt(tmdbID, 10)+"/recommendations")
}
