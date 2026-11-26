// Package testutil reports I/O errors that tests and test fakes would
// otherwise drop.
package testutil

import (
	"io"
	"testing"
)

// Close closes c and reports a failure as a test error.
func Close(tb testing.TB, c io.Closer) {
	tb.Helper()
	if err := c.Close(); err != nil {
		tb.Errorf("close: %v", err)
	}
}

// Write writes s to w, typically a fake server's http.ResponseWriter, and
// reports a failure as a test error. It is safe to call from a handler
// goroutine.
func Write(tb testing.TB, w io.Writer, s string) {
	tb.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		tb.Errorf("write: %v", err)
	}
}
