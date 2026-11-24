//go:build windows

package lan

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// reuseAddr lets the mDNS socket share port 5353 with Windows' own mDNS
// responder.
func reuseAddr(_, _ string, c syscall.RawConn) error {
	var sockErr error
	err := c.Control(func(fd uintptr) {
		sockErr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return sockErr
}
