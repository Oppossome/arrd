//go:build !notray

package tray

// trayIcon returns PNG bytes, which the StatusNotifierItem backend accepts.
func trayIcon() []byte { return iconPNG() }
