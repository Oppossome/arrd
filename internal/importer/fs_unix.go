//go:build unix

package importer

import (
	"errors"
	"os"
	"syscall"
)

// device returns the ID of the filesystem holding path.
func device(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("filesystem device ids are not available on this platform")
	}
	return uint64(st.Dev), nil //nolint:unconvert // Dev's width varies by platform
}

// isCrossDevice reports whether err is a link/rename failing because source
// and destination are on different filesystems.
func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
