package release

import (
	"reflect"
	"testing"
)

func TestParseTV(t *testing.T) {
	tests := []struct {
		name string
		want Info
	}{
		{"Harbor.Watch.S02E05.1080p.WEB-DL.x264-QUILL", Info{Title: "Harbor Watch", Resolution: 1080, Source: SourceWEBDL, Codec: "x264", Group: "QUILL", IsTV: true, Season: 2, Episodes: []int{5}}},
		{"Harbor.Watch.2019.S01E01E02.720p.HDTV.x264-PIKA", Info{Title: "Harbor Watch", Year: 2019, Resolution: 720, Source: SourceHDTV, Codec: "x264", Group: "PIKA", IsTV: true, Season: 1, Episodes: []int{1, 2}}},
		{"Harbor.Watch.S01E02-E04.1080p.WEBRip-MOSS", Info{Title: "Harbor Watch", Resolution: 1080, Source: SourceWEBRip, Group: "MOSS", IsTV: true, Season: 1, Episodes: []int{2, 3, 4}}},
		{"Harbor Watch 3x07 HDTV", Info{Title: "Harbor Watch", Source: SourceHDTV, IsTV: true, Season: 3, Episodes: []int{7}}},
		{"Harbor.Watch.S03.1080p.BluRay.x265-FERN", Info{Title: "Harbor Watch", Resolution: 1080, Source: SourceBluRay, Codec: "x265", Group: "FERN", IsTV: true, Season: 3, SeasonPack: true}},
		{"Harbor Watch Season 4 Complete 720p WEB-DL", Info{Title: "Harbor Watch", Resolution: 720, Source: SourceWEBDL, IsTV: true, Season: 4, SeasonPack: true}},
		{"Harbor.Watch.S01-S03.1080p.WEB-DL", Info{Title: "Harbor Watch", Resolution: 1080, Source: SourceWEBDL, IsTV: true, Season: 1, SeasonPack: true, MultiSeason: true}},
		// Ambiguous: the parser reads 2087 as a year; series matching accepts
		// "title + year" too (see TestEvaluateEpisode).
		{"Station.2087.S01E01-E02", Info{Title: "Station", Year: 2087, IsTV: true, Season: 1, Episodes: []int{1, 2}}},
		{"1965.S01E03.720p.WEB", Info{Title: "1965", Resolution: 720, Source: SourceWEBDL, IsTV: true, Season: 1, Episodes: []int{3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.name)
			tt.want.Raw = tt.name
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q)\n got  %+v\n want %+v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCovers(t *testing.T) {
	ep := Parse("Show.S01E02E03.720p")
	pack := Parse("Show.S01.720p")
	multi := Parse("Show.S01-S02.720p")
	cases := []struct {
		info    Info
		s, e    int
		covered bool
	}{
		{ep, 1, 2, true}, {ep, 1, 3, true}, {ep, 1, 4, false}, {ep, 2, 2, false},
		{pack, 1, 9, true}, {pack, 2, 1, false},
		{multi, 1, 1, false}, // multi-season packs are not supported yet
		{Parse("Show.2020.1080p"), 1, 1, false},
	}
	for _, c := range cases {
		if got := c.info.Covers(c.s, c.e); got != c.covered {
			t.Errorf("%s Covers(%d,%d) = %v", c.info.Raw, c.s, c.e, got)
		}
	}
}
