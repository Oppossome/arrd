//go:build windows

package importer

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// device returns the serial number of the volume holding path.
func device(path string) (_ uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("%q: %w", path, err)
	}
	// FILE_FLAG_BACKUP_SEMANTICS is required to open directories.
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(h)) }()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return 0, fmt.Errorf("stat %q: %w", path, err)
	}
	return uint64(info.VolumeSerialNumber), nil
}

// isCrossDevice reports whether err is a link/rename failing because source
// and destination are on different volumes.
func isCrossDevice(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }
