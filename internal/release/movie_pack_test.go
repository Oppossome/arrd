package release

import (
	"strings"
	"testing"
)

func TestParseMoviePack(t *testing.T) {
	tests := []struct {
		name        string
		pack        bool
		packTitle   string
		first, last int
	}{
		{"Skyfarer.Trilogy.2011-2015.1080p.BluRay.x264-QUILL", true, "Skyfarer", 2011, 2015},
		{"Skyfarer.Collection.1080p.BluRay.x264-QUILL", true, "Skyfarer", 0, 0},
		{"Skyfarer.Complete.Collection.1080p.BluRay.x264-QUILL", true, "Skyfarer", 0, 0},
		{"Skyfarer Duology (2011-2013) [1080p] [BluRay]", true, "Skyfarer", 2011, 2013},
		{"Skyfarer.Box.Set.2011.2013.1080p.BluRay-QUILL", true, "Skyfarer", 2011, 2013},
		{"Skyfarer.1-3.1080p.BluRay.x264-QUILL", true, "Skyfarer", 0, 0},
		{"Skyfarer.2011.2013.2015.1080p.BluRay.x264-QUILL", true, "Skyfarer", 2011, 2015},
		{"The.Lantern.Keepers.Quadrilogy.1080p.WEB-DL-FERN", true, "The Lantern Keepers", 0, 0},
		// Not packs.
		{"Skyfarer.2011.1080p.BluRay.x264-QUILL", false, "", 0, 0},
		{"Station.2087.2023.2160p.UHD.BluRay.REMUX-LuMEN", false, "", 0, 0}, // descending: title year + release year
		{"Paper.Kites.1984.2020.1080p.WEB-DL-FERN", false, "", 0, 0},        // two years: title "Paper Kites 1984"
		{"Collection.2012.1080p.BluRay-QUILL", false, "", 0, 0},             // a title that is only a pack word
		{"Harbor.Watch.Collection.S01-S03.1080p.WEB-DL-G", false, "", 0, 0}, // TV is never a movie pack
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.name)
			if got.MoviePack != tt.pack || got.PackTitle != tt.packTitle || got.PackFirstYear != tt.first || got.PackLastYear != tt.last {
				t.Errorf("Parse(%q) pack=%v %q %d-%d, want pack=%v %q %d-%d", tt.name,
					got.MoviePack, got.PackTitle, got.PackFirstYear, got.PackLastYear, tt.pack, tt.packTitle, tt.first, tt.last)
			}
		})
	}
}

func TestParseMoviePackKeepsSingleMovieFields(t *testing.T) {
	// A movie named after a pack word keeps its title and year, so it can
	// still match as itself.
	got := Parse("The.Orchard.Collection.2012.1080p.BluRay.x264-QUILL")
	if got.Title != "The Orchard Collection" || got.Year != 2012 || !got.MoviePack {
		t.Errorf("got title %q year %d pack %v", got.Title, got.Year, got.MoviePack)
	}
}

func TestFranchiseName(t *testing.T) {
	for in, want := range map[string]string{
		"Skyfarer Collection":   "Skyfarer",
		"Skyfarer":              "Skyfarer",
		"Collection":            "Collection",
		" Skyfarer collection ": "Skyfarer",
	} {
		if got := FranchiseName(in); got != want {
			t.Errorf("FranchiseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEvaluateMovieRejectsPacks(t *testing.T) {
	lim := Limits{MinResolution: 720, MaxResolution: 1080}
	d := EvaluateMovie(Candidate{Info: Parse("Skyfarer.Trilogy.2011-2015.1080p.BluRay.x264-G"), Size: 30 << 30, Seeders: 5},
		MovieTarget{Title: "Skyfarer", Year: 2011}, lim)
	if d.Accepted || !strings.Contains(strings.Join(d.Rejects, "; "), "multi-movie pack") {
		t.Errorf("pack: rejects %v, want a multi-movie pack rejection", d.Rejects)
	}
	d = EvaluateMovie(Candidate{Info: Parse("The.Orchard.Collection.2012.1080p.BluRay.x264-G"), Size: 8 << 30, Seeders: 5},
		MovieTarget{Title: "The Orchard Collection", Year: 2012}, lim)
	if !d.Accepted {
		t.Errorf("movie titled with a pack word: rejected %v", d.Rejects)
	}
}

func TestEvaluateMoviePack(t *testing.T) {
	coll := CollectionTarget{Name: "Skyfarer Collection", Parts: []MovieTarget{
		{Title: "Skyfarer", Year: 2011},
		{Title: "Skyfarer: Ascent", Year: 2013},
		{Title: "Skyfarer: Homeward", Year: 2015},
	}}
	lim := Limits{MinResolution: 720, MaxResolution: 1080, MaxSizeBytes: 10 << 30}
	tests := []struct {
		release    string
		size       int64
		wantReject string
	}{
		{"Skyfarer.Trilogy.2011-2015.1080p.BluRay.x264-G", 24 << 30, ""}, // 8 GiB per movie
		{"Skyfarer.Collection.1080p.BluRay.x264-G", 24 << 30, ""},
		{"Skyfarer.1-3.1080p.BluRay.x264-G", 24 << 30, ""},
		{"Skyfarer.Duology.2011-2013.1080p.BluRay.x264-G", 16 << 30, ""}, // partial packs are offered too
		{"Skyfarer.Trilogy.2011-2015.1080p.BluRay.x264-G", 45 << 30, "exceeds"},
		{"Skyfarer.Trilogy.1990-1994.1080p.BluRay.x264-G", 24 << 30, "outside the collection"},
		{"Moth.Girl.Trilogy.1080p.BluRay.x264-G", 24 << 30, "does not match"},
		{"Skyfarer.2011.1080p.BluRay.x264-G", 8 << 30, "single movie"},
		{"Skyfarer.Trilogy.2011-2015.480p.DVDRip-G", 6 << 30, "below the minimum"},
	}
	for _, tt := range tests {
		d := EvaluateMoviePack(Candidate{Info: Parse(tt.release), Size: tt.size, Seeders: 5}, coll, lim)
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
}
