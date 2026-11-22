// Package web embeds the built web UI (a SvelteKit single-page app).
//
// The frontend source lives next to this file. `go generate ./web` installs
// its npm dependencies and builds it into dist/, which is embedded into the
// arrd binary. dist/ is not committed (only a placeholder), so a binary
// built without generating first serves a "frontend not built" page.
package web

//go:generate sh -c "npm ci && npm run build"

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built UI, rooted at dist/.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a compile-time constant embedded above
	}
	return sub
}
