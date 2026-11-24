package release

import (
	"regexp"
	"strconv"
)

var (
	// S01E02, S01E02E03, S01E02-E04, S01E02-03
	episodeTag = regexp.MustCompile(`^s(\d{1,2})e(\d{1,3})((?:-?e?\d{1,3})*)$`)
	episodeRun = regexp.MustCompile(`(-?)e?(\d{1,3})`)
	// 1x02
	crossTag = regexp.MustCompile(`^(\d{1,2})x(\d{2,3})$`)
	// S01
	seasonTag = regexp.MustCompile(`^s(\d{1,2})$`)
	// S01-S03, S01-03
	multiSeasonTag = regexp.MustCompile(`^s(\d{1,2})-s?(\d{1,2})$`)
	// a bare number following "Season"
	seasonNumber = regexp.MustCompile(`^\d{1,2}$`)
	// a range following "Season" or "Seasons": "1-3"
	seasonRange = regexp.MustCompile(`^(\d{1,2})-(\d{1,2})$`)
	// "-E03" at the end of a name is part of an episode tag, not a group.
	episodeSuffix = regexp.MustCompile(`(?i)^-e?\d{1,3}$`)
)

// parseTVToken recognises a season/episode tag in tok (lower-cased); next
// is the following token, used for "Season 1". It reports whether tok was
// a TV tag.
func parseTVToken(info *Info, tok, next string) bool {
	if m := episodeTag.FindStringSubmatch(tok); m != nil {
		info.IsTV = true
		info.Season = atoi(m[1])
		first := atoi(m[2])
		info.Episodes = []int{first}
		prev := first
		for _, run := range episodeRun.FindAllStringSubmatch(m[3], -1) {
			ep := atoi(run[2])
			if run[1] == "-" && ep > prev {
				// A hyphen means a range: E02-E04 is 2, 3 and 4.
				for e := prev + 1; e < ep; e++ {
					info.Episodes = append(info.Episodes, e)
				}
			}
			if ep != prev {
				info.Episodes = append(info.Episodes, ep)
			}
			prev = ep
		}
		return true
	}
	if m := crossTag.FindStringSubmatch(tok); m != nil {
		info.IsTV = true
		info.Season = atoi(m[1])
		info.Episodes = []int{atoi(m[2])}
		return true
	}
	if m := multiSeasonTag.FindStringSubmatch(tok); m != nil {
		setSeasonRange(info, atoi(m[1]), atoi(m[2]))
		return true
	}
	if m := seasonTag.FindStringSubmatch(tok); m != nil {
		info.IsTV, info.SeasonPack = true, true
		info.Season = atoi(m[1])
		return true
	}
	if tok == "season" || tok == "seasons" {
		if m := seasonRange.FindStringSubmatch(next); m != nil {
			setSeasonRange(info, atoi(m[1]), atoi(m[2]))
			return true
		}
	}
	if tok == "season" && seasonNumber.MatchString(next) {
		info.IsTV, info.SeasonPack = true, true
		info.Season = atoi(next)
		return true
	}
	return false
}

// setSeasonRange records a pack of seasons first to last. A range of one
// season ("S02-S02") is an ordinary season pack.
func setSeasonRange(info *Info, first, last int) {
	info.IsTV, info.SeasonPack = true, true
	info.Season = min(first, last)
	if first != last {
		info.MultiSeason = true
		info.LastSeason = max(first, last)
	}
}

// parseCompleteSeries marks a release with no season tag that calls itself
// a "Complete Series" as a pack of every season.
func parseCompleteSeries(info *Info, tokens []string) {
	if info.IsTV {
		return
	}
	for i := 0; i+1 < len(tokens); i++ {
		if containsFold(tokens[i:i+1], "complete") && containsFold(tokens[i+1:i+2], "series") {
			info.IsTV, info.SeasonPack, info.MultiSeason, info.CompleteSeries = true, true, true, true
			return
		}
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// CoversSeason reports whether the release is a pack containing all of the
// given season.
func (i Info) CoversSeason(season int) bool {
	switch {
	case !i.IsTV || !i.SeasonPack:
		return false
	case i.CompleteSeries:
		return true
	case i.MultiSeason:
		return season >= i.Season && season <= i.LastSeason
	default:
		return season == i.Season
	}
}

// Covers reports whether the release contains the given episode.
func (i Info) Covers(season, episode int) bool {
	if i.SeasonPack {
		return i.CoversSeason(season)
	}
	if !i.IsTV || i.Season != season {
		return false
	}
	for _, e := range i.Episodes {
		if e == episode {
			return true
		}
	}
	return false
}
