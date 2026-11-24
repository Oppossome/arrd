//go:build (linux || windows) && !notray

package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
)

const iconSize = 32

// iconPNG is a generated placeholder icon: a rounded accent-coloured square
// with a white "play" triangle.
var iconPNG = sync.OnceValue(func() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	accent := color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff}
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	const radius = 7
	for y := range iconSize {
		for x := range iconSize {
			if !insideRoundedSquare(x, y, radius) {
				continue
			}
			c := accent
			if insidePlayTriangle(x, y) {
				c = white
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(fmt.Sprintf("tray: encode icon: %v", err)) // in-memory encode of a valid image cannot fail
	}
	return buf.Bytes()
})

func insideRoundedSquare(x, y, r int) bool {
	cx, cy := clamp(x, r, iconSize-1-r), clamp(y, r, iconSize-1-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// insidePlayTriangle tests a right-pointing triangle with vertices
// (11,8), (11,23), (24,15.5).
func insidePlayTriangle(x, y int) bool {
	if x < 11 || x > 24 {
		return false
	}
	half := 7.5 * float64(24-x) / 13
	return float64(y) >= 15.5-half && float64(y) <= 15.5+half
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
