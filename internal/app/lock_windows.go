//go:build windows

package app

import (
	"errors"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

// The lock covers a byte range far past the PID text: Windows byte-range
// locks are mandatory, so locking offset 0 would block our own PID write.
const lockOffset = math.MaxUint32

func tryLock(f *os.File) error {
	ol := windows.Overlapped{Offset: lockOffset}
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrAlreadyRunning
	}
	return err
}

func unlock(f *os.File) {
	ol := windows.Overlapped{Offset: lockOffset}
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol) // Close releases it anyway.
}
