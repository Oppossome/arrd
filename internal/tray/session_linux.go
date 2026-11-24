//go:build !notray

package tray

import "path/filepath"

// desktopSession reports whether a graphical session with a DBus session bus
// is available, which the StatusNotifierItem tray protocol needs.
func desktopSession(getenv func(string) string, exists func(string) bool) bool {
	hasBus := getenv("DBUS_SESSION_BUS_ADDRESS") != ""
	if !hasBus {
		if rt := getenv("XDG_RUNTIME_DIR"); rt != "" {
			hasBus = exists(filepath.Join(rt, "bus"))
		}
	}
	hasDisplay := getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != ""
	return hasBus && hasDisplay
}
