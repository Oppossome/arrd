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
	// S01-S03
	multiSeasonTag = regexp.MustCompile(`^s(\d{1,2})-s(\d{1,2})$`)
	// a bare number following "Season"
	seasonNumber = regexp.MustCompile(`^\d{1,2}$`)
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
		info.IsTV, info.SeasonPack, info.MultiSeason = true, true, true
		info.Season = atoi(m[1])
		return true
	}
	if m := seasonTag.FindStringSubmatch(tok); m != nil {
		info.IsTV, info.SeasonPack = true, true
		info.Season = atoi(m[1])
		return true
	}
	if tok == "season" && seasonNumber.MatchString(next) {
		info.IsTV, info.SeasonPack = true, true
		info.Season = atoi(next)
		return true
	}
	return false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// Covers reports whether the release contains the given episode.
func (i Info) Covers(season, episode int) bool {
	if !i.IsTV || i.MultiSeason || i.Season != season {
		return false
	}
	if i.SeasonPack {
		return true
	}
	for _, e := range i.Episodes {
		if e == episode {
			return true
		}
	}
	return false
}
