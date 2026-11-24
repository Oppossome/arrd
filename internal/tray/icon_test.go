//go:build (linux || windows) && !notray

package tray

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIconPNGDecodes(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(iconPNG()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("size = %v, want %dx%d", b.Size(), iconSize, iconSize)
	}
}
