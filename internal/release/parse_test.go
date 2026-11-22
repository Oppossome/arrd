package release

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		want Info
	}{
		{"The.Glass.Orchard.1999.1080p.BluRay.x264-QUILL", Info{Title: "The Glass Orchard", Year: 1999, Resolution: 1080, Source: SourceBluRay, Codec: "x264", Group: "QUILL"}},
		{"Station.2087.2023.2160p.UHD.BluRay.REMUX.HDR.HEVC.Atmos-LuMEN", Info{Title: "Station 2087", Year: 2023, Resolution: 2160, Source: SourceRemux, Codec: "x265", Group: "LuMEN"}},
		{"1965.2019.720p.WEB-DL.H264-GROUP", Info{Title: "1965", Year: 2019, Resolution: 720, Source: SourceWEBDL, Codec: "x264", Group: "GROUP"}},
		{"Harbor Lights Part Two (2024) [1080p] [WEBRip] [x265]", Info{Title: "Harbor Lights Part Two", Year: 2024, Resolution: 1080, Source: SourceWEBRip, Codec: "x265"}},
		{"Moth-Girl.Never.Again.2021.PROPER.1080p.WEB.H264-PIKA", Info{Title: "Moth-Girl Never Again", Year: 2021, Resolution: 1080, Source: SourceWEBDL, Codec: "x264", Proper: true, Group: "PIKA"}},
		{"Some.Movie.2023.HDCAM.x264-NOGRP", Info{Title: "Some Movie", Year: 2023, Source: SourceCam, Codec: "x264", Group: "NOGRP"}},
		{"2031.2009.1080p.BluRay.x264-MOSS", Info{Title: "2031", Year: 2009, Resolution: 1080, Source: SourceBluRay, Codec: "x264", Group: "MOSS"}},
		{"Movie Without Year 1080p WEB-DL", Info{Title: "Movie Without Year", Resolution: 1080, Source: SourceWEBDL}},
		{"The.Movie.2020.1080p.WEB-DL.DDP5.1.H.264-FERN.mkv", Info{Title: "The Movie", Year: 2020, Resolution: 1080, Source: SourceWEBDL, Codec: "x264", Group: "FERN"}},
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

func TestQualityLabel(t *testing.T) {
	if q := Parse("X.2020.2160p.BluRay.REMUX-G").Quality(); q != "Remux-2160p" {
		t.Errorf("Quality = %q", q)
	}
	if q := Parse("X 2020").Quality(); q != "Unknown-Unknown" {
		t.Errorf("Quality = %q", q)
	}
}

func TestNormalizeTitle(t *testing.T) {
	if NormalizeTitle("Moth-Girl: Never Again") != NormalizeTitle("Moth.Girl.Never.Again") {
		t.Error("punctuation should not matter")
	}
	if NormalizeTitle("Salt & Pepper") != NormalizeTitle("Salt and Pepper") {
		t.Error("& should equal and")
	}
}
