//go:build !notray

package tray

import (
	"bytes"
	"encoding/binary"
)

// trayIcon returns the icon as an .ico file (one PNG-compressed image),
// which the Windows backend requires.
func trayIcon() []byte { return pngToICO(iconPNG(), iconSize) }

func pngToICO(pngData []byte, size int) []byte {
	var buf bytes.Buffer
	le := binary.LittleEndian
	buf.Write(le.AppendUint16(nil, 0))              // reserved
	buf.Write(le.AppendUint16(nil, 1))              // type: icon
	buf.Write(le.AppendUint16(nil, 1))              // image count
	buf.Write([]byte{byte(size), byte(size), 0, 0}) // width, height, palette, reserved
	buf.Write(le.AppendUint16(nil, 1))              // colour planes
	buf.Write(le.AppendUint16(nil, 32))             // bits per pixel
	buf.Write(le.AppendUint32(nil, uint32(len(pngData))))
	buf.Write(le.AppendUint32(nil, 6+16)) // image data offset
	buf.Write(pngData)
	return buf.Bytes()
}
