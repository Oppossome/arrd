package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogFileCreatesDirAndAppends(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "nested", "arrd")

	for _, line := range []string{"first\n", "second\n"} {
		f, err := openLogFile(dataDir)
		if err != nil {
			t.Fatalf("openLogFile: %v", err)
		}
		if _, err := f.WriteString(line); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "arrd.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(got) != "first\nsecond\n" {
		t.Fatalf("log = %q, want both runs appended", got)
	}
}
