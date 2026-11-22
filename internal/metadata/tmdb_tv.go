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
}

// SeasonSummary lists a season without its episodes.
type SeasonSummary struct {
	Number       int `json:"season_number"`
	EpisodeCount int `json:"episode_count"`
}

// Episode is one episode of a season.
type Episode struct {
	Season   int    `json:"season"`
	Number   int    `json:"episode"`
	Title    string `json:"title"`
	AirDate  string `json:"air_date,omitempty"` // YYYY-MM-DD; empty if unknown
	Overview string `json:"overview"`
}

type tmdbSeries struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	FirstAirDate string `json:"first_air_date"`
	Overview     string `json:"overview"`
	Status       string `json:"status"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	Seasons      []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	} `json:"seasons"`
	ExternalIDs struct {
		TVDBID int64  `json:"tvdb_id"`
		IMDBID string `json:"imdb_id"`
	} `json:"external_ids"`
}

func (s tmdbSeries) toSeries() Series {
	out := Series{
		TMDBID: s.ID, TVDBID: s.ExternalIDs.TVDBID, IMDBID: s.ExternalIDs.IMDBID,
		Title: s.Name, OriginalTitle: s.OriginalName, Year: yearOf(s.FirstAirDate),
		Overview: s.Overview, Status: s.Status, PosterPath: s.PosterPath, BackdropPath: s.BackdropPath,
	}
	for _, season := range s.Seasons {
		out.Seasons = append(out.Seasons, SeasonSummary{Number: season.SeasonNumber, EpisodeCount: season.EpisodeCount})
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
	params := url.Values{"append_to_response": {"external_ids"}}
	if err := c.get(ctx, "/tv/"+strconv.FormatInt(tmdbID, 10), params, &s); err != nil {
		return nil, err
	}
	series := s.toSeries()
	return &series, nil
}

// GetSeason fetches the episodes of one season.
func (c *TMDB) GetSeason(ctx context.Context, tmdbID int64, season int) ([]Episode, error) {
	var resp struct {
		Episodes []struct {
			SeasonNumber  int    `json:"season_number"`
			EpisodeNumber int    `json:"episode_number"`
			Name          string `json:"name"`
			AirDate       string `json:"air_date"`
			Overview      string `json:"overview"`
		} `json:"episodes"`
	}
	path := "/tv/" + strconv.FormatInt(tmdbID, 10) + "/season/" + strconv.Itoa(season)
	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Episode, 0, len(resp.Episodes))
	for _, e := range resp.Episodes {
		out = append(out, Episode{Season: e.SeasonNumber, Number: e.EpisodeNumber, Title: e.Name, AirDate: e.AirDate, Overview: e.Overview})
	}
	return out, nil
}

// SeriesRecommendations lists the TV series TMDB recommends to people who
// liked a series (its first page, about 20). An unknown series is ErrNotFound.
func (c *TMDB) SeriesRecommendations(ctx context.Context, tmdbID int64) ([]Series, error) {
	return c.seriesList(ctx, "/tv/"+strconv.FormatInt(tmdbID, 10)+"/recommendations")
}
