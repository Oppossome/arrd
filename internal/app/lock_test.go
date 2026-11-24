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
	release()
	release2, err := lockDataDir(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	release2()
}

func TestLockDataDirIndependentDirs(t *testing.T) {
	r1, err := lockDataDir(t.TempDir())
	if err != nil {
		t.Fatalf("lock dir 1: %v", err)
	}
	defer r1()
	r2, err := lockDataDir(t.TempDir())
	if err != nil {
		t.Fatalf("lock dir 2: %v", err)
	}
	r2()
}
