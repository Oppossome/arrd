package acquire

import (
	"errors"
	"testing"

	"generic-arr-project/internal/library/apperr"
)

func TestTargetValidate(t *testing.T) {
	one, zero := 1, 0
	tests := []struct {
		name   string
		target Target
		field  string // "" = valid
	}{
		{"movie", MovieTarget(900001), ""},
		{"collection", CollectionTarget(900011), ""},
		{"series", SeriesTarget(800001), ""},
		{"season", SeasonTarget(800001, 1), ""},
		{"specials", SeasonTarget(800001, 0), ""},
		{"episode", EpisodeTarget(800001, 1, 2), ""},
		{"no TMDB id", Target{Kind: KindMovie}, "tmdb_id"},
		{"unknown kind", Target{Kind: "trailer", TMDBID: 1}, "kind"},
		{"movie with a season", Target{Kind: KindMovie, TMDBID: 1, Season: &one}, "season"},
		{"season without a number", Target{Kind: KindSeason, TMDBID: 1}, "season"},
		{"season with an episode", Target{Kind: KindSeason, TMDBID: 1, Season: &one, Episode: &one}, "episode"},
		{"episode without one", Target{Kind: KindEpisode, TMDBID: 1, Season: &one}, "episode"},
		{"episode zero", Target{Kind: KindEpisode, TMDBID: 1, Season: &one, Episode: &zero}, "episode"},
	}
	for _, tt := range tests {
		err := tt.target.Validate()
		var ve *apperr.ValidationError
		switch {
		case tt.field == "" && err != nil:
			t.Errorf("%s: %v, want valid", tt.name, err)
		case tt.field != "" && (!errors.As(err, &ve) || ve.Field != tt.field):
			t.Errorf("%s: %v, want a %s validation error", tt.name, err, tt.field)
		}
	}
}
