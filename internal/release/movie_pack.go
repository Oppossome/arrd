package release

import (
	"regexp"
	"strconv"
	"strings"
)

// Multi-movie packs ("Skyfarer.Trilogy.2011-2015.1080p…") hold several
// movies of one collection. Parse records them in Info's pack fields and
// leaves Title and Year as they would be for a single movie, so a movie
// whose real title contains a pack word still matches as that movie.

var (
	// packWords name a release as several movies. Words that also start
	// real single-movie titles ("Saga", "Anthology") are left out.
	packWords = map[string]bool{
		"duology": true, "trilogy": true, "tetralogy": true, "quadrilogy": true,
		"pentalogy": true, "hexalogy": true, "collection": true, "boxset": true,
	}
	yearRangeToken   = regexp.MustCompile(`^((?:19|20)\d{2})-((?:19|20)\d{2})$`)
	numberRangeToken = regexp.MustCompile(`^(\d{1,2})-(\d{1,2})$`)
)

// minYearRunForPack is how many separate, ascending years ("2011.2013.2015")
// mark a pack. Two are not enough: "Title.1984.2020" is a single movie
// titled "Title 1984".
const minYearRunForPack = 3

// parseMoviePack fills Info's pack fields when tokens name a multi-movie
// pack. titleEnd is where the single-movie title ends.
func parseMoviePack(info *Info, tokens []string, titleEnd int) {
	if info.IsTV {
		return
	}
	marker, first, last := findPackMarker(tokens)
	if marker <= 0 {
		return
	}
	titleTokens := tokens[:min(marker, titleEnd)]
	for len(titleTokens) > 1 && yearToken.MatchString(titleTokens[len(titleTokens)-1]) {
		titleTokens = titleTokens[:len(titleTokens)-1]
	}
	info.MoviePack = true
	info.PackTitle = strings.Join(titleTokens, " ")
	info.PackFirstYear, info.PackLastYear = first, last
	if first == 0 {
		info.PackFirstYear, info.PackLastYear = yearSpan(tokens[marker:])
	}
}

// findPackMarker returns the index of the first token that marks a pack
// (-1 if none), and the years it spans when the marker is a year range.
// The first token never counts: it belongs to the title.
func findPackMarker(tokens []string) (index, firstYear, lastYear int) {
	for i := 1; i < len(tokens); i++ {
		lower := strings.ToLower(tokens[i])
		switch {
		case packWords[lower]:
			return i, 0, 0
		case lower == "box" && i+1 < len(tokens) && strings.EqualFold(tokens[i+1], "set"):
			return i, 0, 0
		}
		if m := yearRangeToken.FindStringSubmatch(lower); m != nil && atoi(m[1]) < atoi(m[2]) {
			return i, atoi(m[1]), atoi(m[2])
		}
		if m := numberRangeToken.FindStringSubmatch(lower); m != nil && atoi(m[1]) < atoi(m[2]) {
			return i, 0, 0
		}
		if n := ascendingYearRun(tokens[i:]); n >= minYearRunForPack {
			return i, atoi(tokens[i]), atoi(tokens[i+n-1])
		}
	}
	return -1, 0, 0
}

// ascendingYearRun counts the strictly ascending year tokens at the start
// of tokens.
func ascendingYearRun(tokens []string) int {
	n, prev := 0, 0
	for _, t := range tokens {
		if !yearToken.MatchString(t) {
			break
		}
		y, _ := strconv.Atoi(t)
		if y <= prev {
			break
		}
		n, prev = n+1, y
	}
	return n
}

// yearSpan finds a year range ("2011-2015") or ascending year run among
// the tokens after a pack word; 0, 0 if there is none.
func yearSpan(tokens []string) (first, last int) {
	for i, t := range tokens {
		if m := yearRangeToken.FindStringSubmatch(t); m != nil && atoi(m[1]) < atoi(m[2]) {
			return atoi(m[1]), atoi(m[2])
		}
		if n := ascendingYearRun(tokens[i:]); n >= 2 {
			return atoi(t), atoi(tokens[i+n-1])
		}
	}
	return 0, 0
}

// CollectionTarget identifies a wanted collection of movies.
type CollectionTarget struct {
	Name  string        // TMDB name, e.g. "Skyfarer Collection"
	Parts []MovieTarget // the released movies in it
}

// FranchiseName is the collection's name without a trailing "Collection",
// as packs are usually named: "Skyfarer Collection" -> "Skyfarer".
func FranchiseName(collection string) string {
	trimmed := strings.TrimSpace(collection)
	fields := strings.Fields(trimmed)
	if len(fields) > 1 && strings.EqualFold(fields[len(fields)-1], "collection") {
		return strings.Join(fields[:len(fields)-1], " ")
	}
	return trimmed
}

// MatchesMovieTitle reports whether a release names the movie itself
// (by title or original title), ignoring its year.
func MatchesMovieTitle(info Info, m MovieTarget) bool {
	got := NormalizeTitle(info.Title)
	return got != "" && (got == NormalizeTitle(m.Title) || got == NormalizeTitle(m.OriginalTitle))
}

// EvaluateMoviePack checks a release offered as several movies of a
// collection.
func EvaluateMoviePack(c Candidate, coll CollectionTarget, lim Limits) Decision {
	var d Decision
	reject := rejecter{&d}
	info := c.Info

	switch {
	case info.IsTV:
		reject.add("is a TV release")
	case !info.MoviePack:
		reject.add("a single movie, wanted a multi-movie pack")
	case !matchesCollection(info.PackTitle, coll):
		reject.add("title %q does not match %q", info.PackTitle, coll.Name)
	}
	if lo, hi := partYears(coll); info.PackFirstYear > 0 && lo > 0 &&
		(info.PackFirstYear < lo-1 || info.PackLastYear > hi+1) {
		reject.add("years %d-%d are outside the collection's %d-%d", info.PackFirstYear, info.PackLastYear, lo, hi)
	}
	checkQuality(c, lim, c.Size/int64(max(len(coll.Parts), 1)), reject)

	d.Accepted = len(d.Rejects) == 0
	d.Score = qualityScore(info)
	return d
}

// matchesCollection reports whether a pack title names the collection: its
// franchise name, its full name, or the title of one of its movies (packs
// are often named after the first movie).
func matchesCollection(packTitle string, coll CollectionTarget) bool {
	got := NormalizeTitle(packTitle)
	if got == "" {
		return false
	}
	if got == NormalizeTitle(FranchiseName(coll.Name)) || got == NormalizeTitle(coll.Name) {
		return true
	}
	for _, p := range coll.Parts {
		if got == NormalizeTitle(p.Title) || (p.OriginalTitle != "" && got == NormalizeTitle(p.OriginalTitle)) {
			return true
		}
	}
	return false
}

// partYears is the earliest and latest known year among the parts; 0, 0 if
// none is known.
func partYears(coll CollectionTarget) (lo, hi int) {
	for _, p := range coll.Parts {
		if p.Year == 0 {
			continue
		}
		if lo == 0 || p.Year < lo {
			lo = p.Year
		}
		hi = max(hi, p.Year)
	}
	return lo, hi
}
