package release

import (
	"fmt"
	"sort"
)

// Limits are the user's quality constraints.
type Limits struct {
	MinResolution int
	MaxResolution int
	MaxSizeBytes  int64 // 0 = unlimited
}

// MovieTarget identifies the wanted movie.
type MovieTarget struct {
	Title         string
	OriginalTitle string
	Year          int
}

// Candidate is a release offered by an indexer.
type Candidate struct {
	Info    Info
	Size    int64
	Seeders int
}

// Decision explains whether a candidate is acceptable and how good it is.
type Decision struct {
	Accepted bool
	Rejects  []string
	Score    int
}

// qualityScore ranks resolution first, then source, then PROPER/REPACK.
func qualityScore(i Info) int {
	s := i.Resolution*100 + int(i.Source)*10
	if i.Proper {
		s++
	}
	return s
}

// Ranked pairs a candidate with its decision.
type Ranked[T any] struct {
	Item     T
	Decision Decision
	Seeders  int
}

// Rank sorts accepted items first, then by quality score, then by seeders.
func Rank[T any](items []Ranked[T]) {
	sort.SliceStable(items, func(a, b int) bool {
		x, y := items[a], items[b]
		if x.Decision.Accepted != y.Decision.Accepted {
			return x.Decision.Accepted
		}
		if x.Decision.Score != y.Decision.Score {
			return x.Decision.Score > y.Decision.Score
		}
		return x.Seeders > y.Seeders
	})
}

type rejecter struct{ d *Decision }

func (r rejecter) add(format string, args ...any) {
	r.d.Rejects = append(r.d.Rejects, fmt.Sprintf(format, args...))
}

// checkQuality applies the limits shared by movies and TV. sizePerItem is
// the size per movie or per episode.
func checkQuality(c Candidate, lim Limits, sizePerItem int64, reject rejecter) {
	switch {
	case c.Info.Source == SourceCam:
		reject.add("CAM/telesync source")
	case c.Info.Resolution == 0:
		reject.add("unknown resolution")
	case c.Info.Resolution < lim.MinResolution:
		reject.add("%dp is below the minimum %dp", c.Info.Resolution, lim.MinResolution)
	case c.Info.Resolution > lim.MaxResolution:
		reject.add("%dp is above the maximum %dp", c.Info.Resolution, lim.MaxResolution)
	}
	if lim.MaxSizeBytes > 0 && sizePerItem > lim.MaxSizeBytes {
		reject.add("size %d MB exceeds %d MB", sizePerItem>>20, lim.MaxSizeBytes>>20)
	}
	if c.Seeders <= 0 {
		reject.add("no seeders")
	}
}

// EvaluateMovie checks c against the wanted movie and limits.
func EvaluateMovie(c Candidate, m MovieTarget, lim Limits) Decision {
	var d Decision
	reject := rejecter{&d}

	if c.Info.IsTV {
		reject.add("is a TV release")
	}
	want := NormalizeTitle(m.Title)
	got := NormalizeTitle(c.Info.Title)
	if got == "" || (got != want && got != NormalizeTitle(m.OriginalTitle)) {
		reject.add("title %q does not match %q", c.Info.Title, m.Title)
	}
	switch {
	case c.Info.Year == 0:
		reject.add("no year in release name")
	case m.Year != 0 && (c.Info.Year < m.Year-1 || c.Info.Year > m.Year+1):
		reject.add("year %d does not match %d", c.Info.Year, m.Year)
	}
	checkQuality(c, lim, c.Size, reject)

	d.Accepted = len(d.Rejects) == 0
	d.Score = qualityScore(c.Info)
	return d
}

// SeriesTarget identifies a wanted series.
type SeriesTarget struct {
	Title         string
	OriginalTitle string
	Year          int // first air year; 0 if unknown
}

// MatchesSeries reports whether a release title names the series. Release
// names sometimes carry the first-air year ("Show.2019.S01E01"), and a
// year-like number may belong to the title itself ("Station.2087.S01E01"),
// so both readings are tried.
func MatchesSeries(info Info, s SeriesTarget) (bool, string) {
	names := map[string]bool{NormalizeTitle(s.Title): true}
	if s.OriginalTitle != "" {
		names[NormalizeTitle(s.OriginalTitle)] = true
	}
	if s.Year > 0 {
		names[NormalizeTitle(fmt.Sprintf("%s %d", s.Title, s.Year))] = true
	}
	if info.Year > 0 && names[NormalizeTitle(fmt.Sprintf("%s %d", info.Title, info.Year))] {
		return true, ""
	}
	if !names[NormalizeTitle(info.Title)] || NormalizeTitle(info.Title) == "" {
		return false, fmt.Sprintf("title %q does not match %q", info.Title, s.Title)
	}
	if info.Year > 0 && s.Year > 0 && (info.Year < s.Year-1 || info.Year > s.Year+1) {
		return false, fmt.Sprintf("year %d does not match %d", info.Year, s.Year)
	}
	return true, ""
}

// EpisodeWant describes what a TV search is looking for.
type EpisodeWant struct {
	Season int
	// Episodes the release must include at least one of. For a single
	// episode search this is just that episode.
	Episodes []int
	// AllowPack accepts whole-season releases; PackEpisodes is the number of
	// episodes in the season, used to judge a pack's size per episode.
	AllowPack    bool
	PackEpisodes int
}

// EvaluateEpisode checks a TV release against a series and wanted episodes.
func EvaluateEpisode(c Candidate, s SeriesTarget, want EpisodeWant, lim Limits) Decision {
	var d Decision
	reject := rejecter{&d}
	info := c.Info

	if ok, why := MatchesSeries(info, s); !ok {
		reject.add("%s", why)
	}
	items := int64(1)
	switch {
	case !info.IsTV:
		reject.add("not a TV release")
	case info.MultiSeason:
		reject.add("multi-season pack; download it from the whole series instead")
	case info.Season != want.Season:
		reject.add("season %d, wanted season %d", info.Season, want.Season)
	case info.SeasonPack:
		if !want.AllowPack {
			reject.add("season pack, wanted individual episodes")
		}
		items = int64(max(want.PackEpisodes, 1))
	default:
		covered := false
		for _, e := range want.Episodes {
			if info.Covers(want.Season, e) {
				covered = true
				break
			}
		}
		if !covered {
			reject.add("episodes %v are not the wanted %v", info.Episodes, want.Episodes)
		}
		items = int64(max(len(info.Episodes), 1))
	}
	checkQuality(c, lim, c.Size/items, reject)

	d.Accepted = len(d.Rejects) == 0
	d.Score = qualityScore(info)
	return d
}

// SeriesPackWant describes a search for a whole series in one release.
type SeriesPackWant struct {
	// SeasonEpisodes is the number of episodes in each regular season
	// (specials excluded), used to judge size per episode.
	SeasonEpisodes map[int]int
}

// EvaluateSeriesPack checks a release offered as the whole series: a
// multi-season or "Complete Series" pack covering at least one season, or,
// for a series with a single season, that season's pack.
func EvaluateSeriesPack(c Candidate, s SeriesTarget, want SeriesPackWant, lim Limits) Decision {
	var d Decision
	reject := rejecter{&d}
	info := c.Info

	if ok, why := MatchesSeries(info, s); !ok {
		reject.add("%s", why)
	}
	episodes := 0
	switch {
	case !info.IsTV:
		reject.add("not a TV release")
	case !info.SeasonPack:
		reject.add("individual episodes, wanted a series pack")
	case !info.MultiSeason && len(want.SeasonEpisodes) > 1:
		reject.add("only season %d; download it from the season instead", info.Season)
	default:
		for season, count := range want.SeasonEpisodes {
			if info.CoversSeason(season) {
				episodes += count
			}
		}
		if episodes == 0 {
			reject.add("covers no season of the series")
		}
	}
	checkQuality(c, lim, c.Size/int64(max(episodes, 1)), reject)

	d.Accepted = len(d.Rejects) == 0
	d.Score = qualityScore(info)
	return d
}
