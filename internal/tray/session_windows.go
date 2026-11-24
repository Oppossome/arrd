//go:build !notray

package tray

// desktopSession is always true on Windows: interactive logons have a
// notification area. (Services run in session 0 without one; arrd is not
// packaged as a service.)
func desktopSession(func(string) string, func(string) bool) bool { return true }
