package importer

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckWritableDir verifies that path is an absolute, existing, writable
// directory.
func CheckWritableDir(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%q is not an absolute path", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	f, err := os.CreateTemp(path, ".arrd-write-test-*")
	if err != nil {
		return fmt.Errorf("%q is not writable: %w", path, err)
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

// SameFilesystem reports whether a and b are on the same device, i.e.
// whether hardlinks between them can work.
func SameFilesystem(a, b string) (bool, error) {
	da, err := device(a)
	if err != nil {
		return false, err
	}
	db, err := device(b)
	if err != nil {
		return false, err
	}
	return da == db, nil
}
