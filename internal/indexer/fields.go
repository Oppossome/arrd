package indexer

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	cdef "github.com/Kcchouette/cardigann-go/definition"
	"github.com/Kcchouette/cardigann-go/template/filters"
)

// bytesFilter is registered with the library: the engine reads a release's
// size as a plain integer, but sites (and the definitions written for
// them) show sizes like "1.5 GB".
const bytesFilter = "arrd_bytes"

// Registered at init, before any searcher runs: the registry is a plain
// map the library reads without locking.
func init() {
	if _, taken := filters.Registry[bytesFilter]; taken {
		panic("cardigann: filter " + bytesFilter + " already registered")
	}
	filters.Registry[bytesFilter] = func(v string, _ []string) (string, error) {
		n, err := parseSize(v)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(n, 10), nil
	}
}

var sizeRE = regexp.MustCompile(`(?i)^([0-9][0-9.,]*)\s*([kmgtp]?i?b|bytes?)?$`)

// thousandsRE matches a comma used as a thousands separator ("1,234").
var thousandsRE = regexp.MustCompile(`,\d{3}$`)

// parseSize reads a size the way Cardigann definitions expect: units are binary
// (1 GB = 1024³ bytes); "," is a thousands separator, unless it is the only
// separator and followed by other than three digits ("1,5 GB").
func parseSize(s string) (int64, error) {
	m := sizeRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("size %q not understood", s)
	}
	num := m[1]
	if strings.Contains(num, ",") && !strings.Contains(num, ".") && !thousandsRE.MatchString(num) {
		num = strings.ReplaceAll(num, ",", ".")
	}
	num = strings.ReplaceAll(num, ",", "")
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("size %q not understood", s)
	}
	exp := 0
	if unit := strings.ToLower(m[2]); unit != "" && !strings.HasPrefix(unit, "byte") && unit != "b" {
		exp = strings.IndexByte("kmgtp", unit[0]) + 1
	}
	return int64(math.Round(f * math.Pow(1024, float64(exp)))), nil
}

// Helper fields holding the definition's own download and magnet fields
// when download falls back to the magnet. The engine extracts fields in
// name order and renders a not-yet-extracted .Result key as "", so the
// helpers' "_" prefix makes them resolve before "download" reads them.
const (
	siteDownloadField = "_arrd_site_download"
	siteMagnetField   = "_arrd_magnet"
)

// downloadOrMagnet is the download field when a site has both: its own
// download link if the row has one, else the magnet.
var downloadOrMagnet = fmt.Sprintf(`{{ if .Result.%[1]s }}{{ .Result.%[1]s }}{{ else }}{{ .Result.%[2]s }}{{ end }}`,
	siteDownloadField, siteMagnetField)

// adaptFields fills gaps between the definitions and the library's
// release extraction: sizes are converted to bytes, and the magnet link is
// used when a row has no .torrent link, because the library reads only
// the download field. That covers sites without a download field and
// sites whose download field is empty by choice (Nyaa's "prefer magnet
// links" setting leaves it empty).
func adaptFields(def cdef.IndexerDefinition) cdef.IndexerDefinition {
	fields := maps.Clone(def.Search.Fields)
	if size, ok := fields["size"]; ok {
		size.Filters = append(slices.Clone(size.Filters), cdef.FilterStep{Name: bytesFilter})
		fields["size"] = size
	}
	magnet, hasMagnet := fields["magnet"]
	download, hasDownload := fields["download"]
	switch {
	case hasMagnet && !hasDownload:
		fields["download"] = magnet
	case hasMagnet && def.Download == nil:
		// With a download block the download field feeds the block's
		// request chain, which a magnet must not reach.
		download.Optional, magnet.Optional = true, true
		fields[siteDownloadField], fields[siteMagnetField] = download, magnet
		fields["download"] = cdef.Field{Text: downloadOrMagnet, Optional: true}
	}
	def.Search.Fields = fields
	return def
}
