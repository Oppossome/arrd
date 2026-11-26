package app

import (
	"errors"
	"testing"
)

func TestLockDataDirExcludesSecondHolder(t *testing.T) {
	dir := t.TempDir()
	release, err := lockDataDir(dir)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if _, err := lockDataDir(dir); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second lock: got %v, want ErrAlreadyRunning", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	release2, err := lockDataDir(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	if err := release2(); err != nil {
		t.Fatalf("release after relock: %v", err)
	}
}

func TestLockDataDirIndependentDirs(t *testing.T) {
	r1, err := lockDataDir(t.TempDir())
	if err != nil {
		t.Fatalf("lock dir 1: %v", err)
	}
	t.Cleanup(func() {
		if err := r1(); err != nil {
			t.Errorf("release dir 1: %v", err)
		}
	})
	r2, err := lockDataDir(t.TempDir())
	if err != nil {
		t.Fatalf("lock dir 2: %v", err)
	}
	if err := r2(); err != nil {
		t.Fatalf("release dir 2: %v", err)
	}
}
