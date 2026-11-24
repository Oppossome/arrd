//go:build !(linux || windows) || notray

package tray

import "context"

// Run returns immediately: the tray is not built for this platform or tag.
func Run(_ context.Context, o Options) {
	o.Log.Debug().Msg("tray: not built into this binary")
}
