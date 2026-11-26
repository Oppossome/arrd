package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ErrAlreadyRunning is returned when another arrd serve holds the data directory.
var ErrAlreadyRunning = errors.New("another arrd instance is already using this data directory")

const lockFileName = "arrd.lock"

// lockDataDir takes an exclusive, non-blocking lock on dataDir/arrd.lock so
// only one server uses a data directory at a time. The operating system drops
// the lock when the process exits, however it exits, so it never goes stale.
// The returned func releases it.
func lockDataDir(dataDir string) (release func() error, err error) {
	if dataDir == "" {
		return nil, errors.New("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, lockFileName)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := tryLock(f); err != nil {
		if errors.Is(err, ErrAlreadyRunning) {
			err = fmt.Errorf("%w (%s)", ErrAlreadyRunning, path)
		} else {
			err = fmt.Errorf("lock %s: %w", path, err)
		}
		return nil, errors.Join(err, f.Close())
	}
	if err := writePID(f); err != nil {
		unlock(f)
		return nil, errors.Join(fmt.Errorf("write %s: %w", path, err), f.Close())
	}
	return func() error {
		unlock(f)
		return f.Close()
	}, nil
}

// writePID records the holder's PID in the lock file, for humans only.
func writePID(f *os.File) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err := f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	return err
}
