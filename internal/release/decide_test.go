package release

import (
	"strings"
	"testing"
)

func TestEvaluateMovie(t *testing.T) {
	movie := MovieTarget{Title: "The Glass Orchard", Year: 1999}
	lim := Limits{MinResolution: 720, MaxResolution: 1080, MaxSizeBytes: 10 << 30}
	tests := []struct {
		release    string
		size       int64
		seeders    int
		wantReject string // empty = accepted
	}{
		{"The.Glass.Orchard.1999.1080p.BluRay.x264-G", 8 << 30, 10, ""},
		{"The.Glass.Orchard.2000.1080p.BluRay.x264-G", 8 << 30, 10, ""}, // ±1 year tolerated
		{"The.Glass.Orchard.Returns.2003.1080p.BluRay.x264-G", 8 << 30, 10, "title"},
		{"The.Glass.Orchard.1985.1080p.BluRay.x264-G", 8 << 30, 10, "year 1985"},
		{"The.Glass.Orchard.1080p.BluRay.x264-G", 8 << 30, 10, "no year"},
		{"The.Glass.Orchard.1999.2160p.BluRay.x265-G", 8 << 30, 10, "above the maximum"},
		{"The.Glass.Orchard.1999.480p.DVDRip-G", 1 << 30, 10, "below the minimum"},
		{"The.Glass.Orchard.1999.HDCAM-G", 1 << 30, 10, "CAM"},
		{"The.Glass.Orchard.1999.BluRay.x264-G", 8 << 30, 10, "unknown resolution"},
		{"The.Glass.Orchard.1999.1080p.BluRay.x264-G", 20 << 30, 10, "exceeds"},
		{"The.Glass.Orchard.1999.1080p.BluRay.x264-G", 8 << 30, 0, "no seeders"},
	}
	for _, tt := range tests {
		d := EvaluateMovie(Candidate{Info: Parse(tt.release), Size: tt.size, Seeders: tt.seeders}, movie, lim)
		if tt.wantReject == "" {
			if !d.Accepted {
				t.Errorf("%s: rejected %v, want accepted", tt.release, d.Rejects)
			}
			continue
		}
		if d.Accepted || !strings.Contains(strings.Join(d.Rejects, "; "), tt.wantReject) {
			t.Errorf("%s: rejects %v, want one containing %q", tt.release, d.Rejects, tt.wantReject)
		}
	}
}

func TestRank(t *testing.T) {
	movie := MovieTarget{Title: "X", Year: 2020}
	lim := Limits{MinResolution: 720, MaxResolution: 2160}
	mk := func(name string, seeders int) Ranked[string] {
		d := EvaluateMovie(Candidate{Info: Parse(name), Seeders: seeders}, movie, lim)
		return Ranked[string]{Item: name, Decision: d, Seeders: seeders}
	}
	items := []Ranked[string]{
		mk("X.2020.720p.WEB-DL-G", 100),
		mk("X.2020.HDCAM-G", 500),
		mk("X.2020.1080p.WEBRip-G", 5),
		mk("X.2020.1080p.BluRay-G", 3),
		mk("X.2020.1080p.BluRay-H", 50),
	}
	Rank(items)
	want := []string{"X.2020.1080p.BluRay-H", "X.2020.1080p.BluRay-G", "X.2020.1080p.WEBRip-G", "X.2020.720p.WEB-DL-G", "X.2020.HDCAM-G"}
	for i, w := range want {
		if items[i].Item != w {
			t.Errorf("rank %d = %s, want %s", i, items[i].Item, w)
		}
	}
}

func TestEvaluateEpisode(t *testing.T) {
	series := SeriesTarget{Title: "Harbor Watch", Year: 2019}
	lim := Limits{MinResolution: 720, MaxResolution: 1080, MaxSizeBytes: 4 << 30}
	single := EpisodeWant{Season: 1, Episodes: []int{2}}
	season := EpisodeWant{Season: 1, Episodes: []int{1, 2, 3}, AllowPack: true, PackEpisodes: 10}
	tests := []struct {
		release    string
		size       int64
		want       EpisodeWant
		wantReject string
	}{
		{"Harbor.Watch.S01E02.1080p.WEB-DL-G", 2 << 30, single, ""},
		{"Harbor.Watch.2019.S01E02.1080p.WEB-DL-G", 2 << 30, single, ""},
		{"Harbor.Watch.S01E01E02.1080p.WEB-DL-G", 6 << 30, single, ""}, // 3 GiB per episode
		{"Harbor.Watch.2011.S01E02.1080p.WEB-DL-G", 2 << 30, single, "year 2011"},
		{"Harbor.Watch.Live.S01E02.1080p.WEB-DL-G", 2 << 30, single, "title"},
		{"Harbor.Watch.S01E03.1080p.WEB-DL-G", 2 << 30, single, "not the wanted"},
		{"Harbor.Watch.S02E02.1080p.WEB-DL-G", 2 << 30, single, "season 2"},
		{"Harbor.Watch.S01.1080p.WEB-DL-G", 20 << 30, single, "season pack"},
		{"Harbor.Watch.S01.1080p.WEB-DL-G", 20 << 30, season, ""}, // 2 GiB per episode
		{"Harbor.Watch.S01.1080p.WEB-DL-G", 60 << 30, season, "exceeds"},
		{"Harbor.Watch.S01-S02.1080p.WEB-DL-G", 20 << 30, season, "multi-season"},
		{"Harbor.Watch.2019.1080p.WEB-DL-G", 2 << 30, single, "not a TV release"},
	}
	for _, tt := range tests {
		d := EvaluateEpisode(Candidate{Info: Parse(tt.release), Size: tt.size, Seeders: 5}, series, tt.want, lim)
		if tt.wantReject == "" {
			if !d.Accepted {
				t.Errorf("%s: rejected %v", tt.release, d.Rejects)
			}
			continue
		}
		if d.Accepted || !strings.Contains(strings.Join(d.Rejects, "; "), tt.wantReject) {
			t.Errorf("%s: rejects %v, want one containing %q", tt.release, d.Rejects, tt.wantReject)
		}
	}

	// A year-like number in the title matches either reading.
	station := SeriesTarget{Title: "Station 2087", Year: 2023}
	if ok, why := MatchesSeries(Parse("Station.2087.S01E01.720p"), station); !ok {
		t.Errorf("Station 2087 not matched: %s", why)
	}
	if ok, _ := MatchesSeries(Parse("Station.2087.2023.S01E01.720p"), station); !ok {
		t.Error("Station 2087 (2023) not matched")
	}
}

func TestEvaluateMovieRejectsTV(t *testing.T) {
	d := EvaluateMovie(Candidate{Info: Parse("The.Glass.Orchard.1999.S01E01.1080p.WEB-DL-G"), Seeders: 5},
		MovieTarget{Title: "The Glass Orchard", Year: 1999}, Limits{MinResolution: 720, MaxResolution: 1080})
	if d.Accepted {
		t.Error("TV release accepted for a movie")
	}
}
