package testutil

import (
	"bytes"
	"errors"
	"testing"
)

// recorder captures Errorf calls so failures can be asserted on.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, format)
}

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestClose(t *testing.T) {
	ok := &recorder{TB: t}
	Close(ok, closeFunc(func() error { return nil }))
	if len(ok.errors) != 0 {
		t.Errorf("successful close reported %v", ok.errors)
	}

	failed := &recorder{TB: t}
	Close(failed, closeFunc(func() error { return errors.New("close failed") }))
	if len(failed.errors) != 1 {
		t.Errorf("failed close reported %d errors, want 1", len(failed.errors))
	}
}

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	ok := &recorder{TB: t}
	Write(ok, &buf, "hello")
	if buf.String() != "hello" || len(ok.errors) != 0 {
		t.Errorf("wrote %q with errors %v, want %q and none", buf.String(), ok.errors, "hello")
	}

	failed := &recorder{TB: t}
	Write(failed, failingWriter{}, "hello")
	if len(failed.errors) != 1 {
		t.Errorf("failed write reported %d errors, want 1", len(failed.errors))
	}
}
