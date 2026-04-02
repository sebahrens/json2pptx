package pptx

import (
	"bytes"
	"fmt"
)

// RectFromPoints creates a RectEmu from point values.
// 1 point = 12700 EMU.
func RectFromPoints(x, y, w, h float64) RectEmu {
	return RectEmu{
		X:  int64(x * 12700),
		Y:  int64(y * 12700),
		CX: int64(w * 12700),
		CY: int64(h * 12700),
	}
}

// WriteTransform writes an a:xfrm element with position, size, and optional
// rotation/flip attributes into buf.
// Rotation is specified in 60,000ths of a degree (e.g., 5400000 = 90°).
func WriteTransform(buf *bytes.Buffer, bounds RectEmu, rotation int64, flipH, flipV bool) {
	buf.WriteString(`<a:xfrm`)
	if rotation != 0 {
		buf.WriteString(fmt.Sprintf(` rot="%d"`, rotation))
	}
	if flipH {
		buf.WriteString(` flipH="1"`)
	}
	if flipV {
		buf.WriteString(` flipV="1"`)
	}
	buf.WriteString(`>`)
	buf.WriteString(fmt.Sprintf(`<a:off x="%d" y="%d"/>`, bounds.X, bounds.Y))
	buf.WriteString(fmt.Sprintf(`<a:ext cx="%d" cy="%d"/>`, bounds.CX, bounds.CY))
	buf.WriteString(`</a:xfrm>`)
}
