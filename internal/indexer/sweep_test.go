//go:build sweep

// The sweep loads a real definitions directory, e.g. a checkout of
// github.com/Prowlarr/Indexers/definitions/v11, to measure compatibility.
// It makes no network requests. Run:
//
//	CARDIGANN_SWEEP_DIR=/path/to/definitions/v11 go test -tags sweep -run Sweep -v ./internal/indexer/
package indexer

import (
	"maps"
	"os"
	"slices"
	"testing"
)

func TestSweep(t *testing.T) {
	dir := os.Getenv("CARDIGANN_SWEEP_DIR")
	if dir == "" {
		t.Skip("set CARDIGANN_SWEEP_DIR")
	}
	catalog, errs := LoadDir(dir)
	for _, err := range errs {
		t.Errorf("parse: %v", err)
	}
	reasons := map[string]int{}
	usable := 0
	defs := catalog.List()
	for _, k := range defs {
		if k.Unsupported == "" {
			usable++
			continue
		}
		reasons[k.Unsupported]++
	}
	t.Logf("%d files parsed, %d failed; %d usable, %d unsupported", len(defs), len(errs), usable, len(defs)-usable)
	for _, r := range slices.Sorted(maps.Keys(reasons)) {
		t.Logf("  %4d  %s", reasons[r], r)
	}
}
