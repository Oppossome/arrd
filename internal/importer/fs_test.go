package importer

import (
	"errors"
	"os"
	"testing"
)

func TestSameFilesystemMissingPath(t *testing.T) {
	if _, err := SameFilesystem(t.TempDir(), "/nonexistent-arrd-test-path"); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}

func TestIsCrossDeviceIgnoresOtherErrors(t *testing.T) {
	if isCrossDevice(&os.LinkError{Op: "link", Err: os.ErrNotExist}) {
		t.Fatal("ErrNotExist reported as cross-device")
	}
	if isCrossDevice(errors.New("boom")) {
		t.Fatal("plain error reported as cross-device")
	}
}
