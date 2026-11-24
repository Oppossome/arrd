package media

import "testing"

func TestTitleFilterMatches(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter TitleFilter
		genres []int64
		lang   string
		want   bool
	}{
		{"zero filter", TitleFilter{}, nil, "", true},
		{"any genre matches", TitleFilter{GenreIDs: []int64{101, 102}}, []int64{103, 102}, "qq", true},
		{"no genre matches", TitleFilter{GenreIDs: []int64{101}}, []int64{103}, "qq", false},
		{"no genres on title", TitleFilter{GenreIDs: []int64{101}}, nil, "qq", false},
		{"language matches", TitleFilter{OriginalLanguage: "qq"}, nil, "qq", true},
		{"language differs", TitleFilter{OriginalLanguage: "qq"}, nil, "zz", false},
		{"both match", TitleFilter{GenreIDs: []int64{101}, OriginalLanguage: "qq"}, []int64{101}, "qq", true},
		{"genre matches, language differs", TitleFilter{GenreIDs: []int64{101}, OriginalLanguage: "qq"}, []int64{101}, "zz", false},
	} {
		if got := tc.filter.Matches(tc.genres, tc.lang); got != tc.want {
			t.Errorf("%s: Matches = %v, want %v", tc.name, got, tc.want)
		}
	}
}
