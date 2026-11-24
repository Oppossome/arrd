//go:build !notray

package tray

import (
	"bytes"
	"image/png"
	"testing"
)

func TestTrayIconIsICO(t *testing.T) {
	ico := trayIcon()
	if !bytes.HasPrefix(ico, []byte{0, 0, 1, 0, 1, 0}) {
		t.Fatalf("ICO header = % x", ico[:6])
	}
	if _, err := png.Decode(bytes.NewReader(ico[22:])); err != nil {
		t.Fatalf("embedded PNG: %v", err)
	}
}
