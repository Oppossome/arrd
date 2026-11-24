// Package release parses scene/P2P release names and decides whether a
// release satisfies a wanted item.
package release

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Source is where a release was captured from, ordered worst to best.
type Source int

const (
	SourceUnknown Source = iota
	SourceCam            // CAM, TS, TC, screeners: never acceptable
	SourceDVD
	SourceHDTV
	SourceWEBRip
	SourceWEBDL
	SourceBluRay
	SourceRemux
)

var sourceNames = map[Source]string{
	SourceUnknown: "Unknown", SourceCam: "CAM", SourceDVD: "DVD", SourceHDTV: "HDTV",
	SourceWEBRip: "WEBRip", SourceWEBDL: "WEBDL", SourceBluRay: "Bluray", SourceRemux: "Remux",
}

func (s Source) String() string { return sourceNames[s] }

// Info is what could be recognised in a release name.
type Info struct {
	Raw        string
	Title      string // as written in the release, separators replaced by spaces
	Year       int    // 0 if absent
	Resolution int    // vertical lines: 480, 720, 1080, 2160; 0 if unknown
	Source     Source
	Codec      string // "x264", "x265", "AV1"; empty if unknown
	Proper     bool   // PROPER / REPACK / REAL
	Group      string

	// TV fields. IsTV is set when any season/episode tag was found.
	IsTV        bool
	Season      int   // first season covered
	Episodes    []int // empty for season packs
	SeasonPack  bool  // whole seasons ("S01", "Season 1", "S01-S03")
	MultiSeason bool  // several seasons ("S01-S03"); Season is the first
	LastSeason  int   // the last season of a multi-season pack; 0 otherwise

	// CompleteSeries is a pack of every season that names no seasons
	// ("Complete Series"). It is also a MultiSeason pack.
	CompleteSeries bool
}

// Quality is a short human label such as "Bluray-1080p".
func (i Info) Quality() string {
	res := "Unknown"
	if i.Resolution > 0 {
		res = strconv.Itoa(i.Resolution) + "p"
	}
	return fmt.Sprintf("%s-%s", i.Source, res)
}

var (
	separators   = regexp.MustCompile(`[._\s]+`)
	bracketed    = regexp.MustCompile(`[\[\](){}]`)
	yearToken    = regexp.MustCompile(`^(19\d{2}|20\d{2})$`)
	groupSuffix  = regexp.MustCompile(`-([A-Za-z0-9]+)(?:\[[^\]]*\])?$`)
	groupPrefix  = regexp.MustCompile(`^\[([^\]]+)\]\s*`) // fansub style: "[Group] Title ..."
	videoExt     = regexp.MustCompile(`(?i)\.(mkv|mp4|avi|m4v|ts|wmv)$`)
	resolutionRe = map[string]int{
		"2160p": 2160, "4k": 2160, "uhd": 2160,
		"1080p": 1080, "1080i": 1080,
		"720p": 720,
		"576p": 480, "480p": 480, "sd": 480,
	}
	sourceTokens = map[string]Source{
		"remux":  SourceRemux,
		"bluray": SourceBluRay, "blu-ray": SourceBluRay, "bdrip": SourceBluRay, "brrip": SourceBluRay, "bd": SourceBluRay, "bdremux": SourceRemux,
		"web-dl": SourceWEBDL, "webdl": SourceWEBDL, "web": SourceWEBDL, "amzn": SourceWEBDL, "nf": SourceWEBDL,
		"webrip": SourceWEBRip, "web-rip": SourceWEBRip,
		"hdtv": SourceHDTV, "pdtv": SourceHDTV,
		"dvdrip": SourceDVD, "dvd": SourceDVD, "dvdr": SourceDVD, "dvd5": SourceDVD, "dvd9": SourceDVD,
		"cam": SourceCam, "hdcam": SourceCam, "camrip": SourceCam, "ts": SourceCam, "telesync": SourceCam,
		"hdts": SourceCam, "tc": SourceCam, "telecine": SourceCam, "scr": SourceCam, "screener": SourceCam, "dvdscr": SourceCam,
	}
	codecTokens = map[string]string{
		"x264": "x264", "h264": "x264", "h.264": "x264", "avc": "x264",
		"x265": "x265", "h265": "x265", "h.265": "x265", "hevc": "x265",
		"av1": "AV1",
	}
	// Tokens that end the title even if they carry no quality information.
	titleTerminators = map[string]bool{
		"proper": true, "repack": true, "real": true, "internal": true, "limited": true,
		"extended": true, "unrated": true, "remastered": true, "hdr": true, "dv": true,
		"multi": true, "dubbed": true, "subbed": true, "complete": true,
	}
)

// Parse extracts what it can from a release name. It never fails; fields it
// cannot recognise are left at their zero value.
func Parse(name string) Info {
	info := Info{Raw: name}
	trimmed := videoExt.ReplaceAllString(strings.TrimSpace(name), "")

	if m := groupPrefix.FindStringSubmatch(trimmed); m != nil {
		info.Group = strings.TrimSpace(m[1])
		trimmed = strings.TrimPrefix(trimmed, m[0])
	}
	if m := groupSuffix.FindStringSubmatch(trimmed); m != nil && !isHyphenatedTag(trimmed, m[0]) {
		if info.Group == "" {
			info.Group = m[1]
		}
		trimmed = strings.TrimSuffix(trimmed, m[0])
	}
	cleaned := bracketed.ReplaceAllString(trimmed, " ")
	tokens := strings.Fields(separators.ReplaceAllString(cleaned, " "))

	// "WEB-DL" may arrive as one token or as "WEB" "DL" after separator
	// replacement; scan pairs as well as single tokens.
	titleEnd := len(tokens)
	for i, tok := range tokens {
		lower := strings.ToLower(tok)
		pair, dotted := "", ""
		if i+1 < len(tokens) {
			next := strings.ToLower(tokens[i+1])
			pair, dotted = lower+"-"+next, lower+"."+next // "WEB DL", "H 264"
		}
		marker := false
		if res, ok := resolutionRe[lower]; ok {
			info.Resolution = max(info.Resolution, res)
			marker = true
		}
		if src, ok := sourceTokens[pair]; ok {
			info.Source = max(info.Source, src)
			marker = true
		} else if src, ok := sourceTokens[lower]; ok && i > 0 {
			// i > 0: a release can't *start* with a source tag; this keeps
			// titles like "Cam" or "Web" intact.
			info.Source = max(info.Source, src)
			marker = true
		}
		if c, ok := codecTokens[lower]; ok {
			info.Codec = c
			marker = true
		} else if c, ok := codecTokens[dotted]; ok {
			info.Codec = c
			marker = true
		}
		if !info.IsTV {
			next := ""
			if i+1 < len(tokens) {
				next = strings.ToLower(tokens[i+1])
			}
			if parseTVToken(&info, lower, next) {
				marker = true
			}
		}
		if lower == "proper" || lower == "repack" || lower == "real" {
			info.Proper = true
		}
		if (marker || titleTerminators[lower]) && i > 0 && titleEnd == len(tokens) {
			titleEnd = i
		}
	}
	parseCompleteSeries(&info, tokens)
	// Remux implies a Blu-ray source even when "BluRay" is listed after it.
	if info.Source == SourceBluRay && containsFold(tokens, "remux") {
		info.Source = SourceRemux
	}

	// The year is the last year-like token before the quality markers, as
	// long as something precedes it (so "1965 2019" is title 1965, year 2019).
	titleTokens := tokens[:titleEnd]
	for i := len(titleTokens) - 1; i > 0; i-- {
		if yearToken.MatchString(titleTokens[i]) {
			info.Year, _ = strconv.Atoi(titleTokens[i])
			titleTokens = titleTokens[:i]
			break
		}
	}
	info.Title = strings.Join(titleTokens, " ")
	return info
}

// isHyphenatedTag reports whether the "-XXX" suffix is really the second
// half of a tag like "WEB-DL" rather than a release group.
func isHyphenatedTag(name, suffix string) bool {
	head := strings.TrimSuffix(name, suffix)
	if i := strings.LastIndexAny(head, ". _[("); i >= 0 {
		head = head[i+1:]
	}
	if episodeSuffix.MatchString(suffix) {
		return true // "S01E01-E02"
	}
	_, ok := sourceTokens[strings.ToLower(head+suffix)]
	return ok
}

func containsFold(tokens []string, want string) bool {
	for _, t := range tokens {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

// NormalizeTitle reduces a title to lowercase alphanumerics so that
// "Moth-Girl: Never Again" and "Moth.Girl.Never.Again" compare equal.
func NormalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "&", "and")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}
