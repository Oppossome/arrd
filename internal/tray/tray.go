// Package tray shows a desktop system-tray icon for arrd with entries to
// open the web UI and to quit. It is built on Linux and Windows unless the
// notray build tag is set (use it for containers and other headless builds).
package tray

import (
	"net"

	"github.com/rs/zerolog"
)

// Options configure the tray icon.
type Options struct {
	URL  string          // page opened by "Open arrd"
	Quit func()          // called by "Quit"; should trigger graceful shutdown
	Log  *zerolog.Logger // required
	// OpenOnStart opens URL in the default browser as soon as the tray is
	// up, e.g. to start first-run setup. Ignored without a desktop session.
	OpenOnStart bool
}

// BrowseURL is the http:// URL a local browser should use to reach a server
// listening on listen. Unspecified hosts map to loopback.
func BrowseURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port)
}
