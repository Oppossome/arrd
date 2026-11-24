package metadata

import "slices"

// maxCast is how many top-billed cast members a detail lookup keeps.
const maxCast = 12

// Extras is the detail-page information TMDB returns for a single movie or
// series lookup (GetMovie, GetSeries); search and list results leave it zero.
type Extras struct {
	Tagline     string  `json:"tagline,omitempty"`
	Runtime     int     `json:"runtime,omitempty"` // minutes; per episode for series
	VoteAverage float64 `json:"vote_average,omitempty"`
	VoteCount   int64   `json:"vote_count,omitempty"`
	// Certification is the US age rating ("PG-13", "TV-MA"); empty if unknown.
	Certification string       `json:"certification,omitempty"`
	Genres        []Genre      `json:"genres,omitempty"`
	Cast          []CastMember `json:"cast,omitempty"`
	Trailer       *Video       `json:"trailer,omitempty"`
}

// CastMember is one billed actor.
type CastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
}

// Video is a trailer hosted on YouTube.
type Video struct {
	Key  string `json:"key"` // YouTube video id
	Name string `json:"name"`
}

type tmdbVideo struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Site     string `json:"site"`
	Type     string `json:"type"`
	Official bool   `json:"official"`
}

type tmdbVideos struct {
	Results []tmdbVideo `json:"results"`
}

// pickTrailer returns the best YouTube trailer: an official trailer, then any
// trailer, then a teaser. Nil if there is none.
func pickTrailer(videos []tmdbVideo) *Video {
	rank := func(v tmdbVideo) int {
		switch {
		case v.Site != "YouTube" || v.Key == "":
			return 0
		case v.Type == "Trailer" && v.Official:
			return 3
		case v.Type == "Trailer":
			return 2
		case v.Type == "Teaser":
			return 1
		}
		return 0
	}
	best, bestRank := tmdbVideo{}, 0
	for _, v := range videos {
		if r := rank(v); r > bestRank {
			best, bestRank = v, r
		}
	}
	if bestRank == 0 {
		return nil
	}
	return &Video{Key: best.Key, Name: best.Name}
}

type tmdbCast struct {
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
	// Roles is set instead of Character by aggregate_credits (series).
	Roles []struct {
		Character string `json:"character"`
	} `json:"roles"`
}

type tmdbCredits struct {
	Cast []tmdbCast `json:"cast"`
}

// topCast returns the first maxCast cast members in billing order.
func topCast(cast []tmdbCast) []CastMember {
	cast = slices.Clone(cast)
	slices.SortStableFunc(cast, func(a, b tmdbCast) int { return a.Order - b.Order })
	out := make([]CastMember, 0, min(len(cast), maxCast))
	for _, c := range cast[:min(len(cast), maxCast)] {
		character := c.Character
		if character == "" && len(c.Roles) > 0 {
			character = c.Roles[0].Character
		}
		out = append(out, CastMember{Name: c.Name, Character: character, ProfilePath: c.ProfilePath})
	}
	return out
}

// certificationCountry is whose age ratings are shown.
const certificationCountry = "US"

type tmdbReleaseDates struct {
	Results []struct {
		Country      string `json:"iso_3166_1"`
		ReleaseDates []struct {
			Certification string `json:"certification"`
		} `json:"release_dates"`
	} `json:"results"`
}

// movieCertification is the first non-empty US certification.
func (r tmdbReleaseDates) certification() string {
	for _, c := range r.Results {
		if c.Country != certificationCountry {
			continue
		}
		for _, d := range c.ReleaseDates {
			if d.Certification != "" {
				return d.Certification
			}
		}
	}
	return ""
}

type tmdbContentRatings struct {
	Results []struct {
		Country string `json:"iso_3166_1"`
		Rating  string `json:"rating"`
	} `json:"results"`
}

func (r tmdbContentRatings) certification() string {
	for _, c := range r.Results {
		if c.Country == certificationCountry && c.Rating != "" {
			return c.Rating
		}
	}
	return ""
}
