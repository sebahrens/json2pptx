package pptx

import (
	"bytes"
	"fmt"
)

// Line represents a DrawingML outline (a:ln) specification.
type Line struct {
	Width    int64  // Line width in EMU
	Fill     Fill   // Line fill (solid, scheme, or noFill)
	Dash     string // Dash style: "solid", "dash", "dot", "lgDash", "dashDot", etc.
	Cap      string // End cap: "flat", "rnd", "sq"
	Compound string // Compound type: "sng", "dbl", "thickThin", "thinThick", "tri"
	Align    string // Alignment: "ctr", "in"
	Join     string // Join style: "round", "bevel", "miter"
}

// NoLine creates a line with no outline (renders <a:ln><a:noFill/></a:ln>).
func NoLine() Line {
	return Line{Fill: NoFill()}
}

// SolidLine creates a solid line with the given width (EMU) and hex color.
func SolidLine(widthEMU int64, hex string) Line {
	return Line{Width: widthEMU, Fill: SolidFill(hex)}
}

// SolidLinePoints creates a solid line with width specified in points.
// 1 point = 12700 EMU.
func SolidLinePoints(widthPt float64, hex string) Line {
	return Line{
		Width: int64(widthPt * 12700),
		Fill:  SolidFill(hex),
	}
}

// WriteTo writes the line's DrawingML XML (a:ln element) into buf.
func (l Line) WriteTo(buf *bytes.Buffer) {
	if l.Width > 0 {
		buf.WriteString(fmt.Sprintf(`<a:ln w="%d"`, l.Width))
	} else {
		buf.WriteString(`<a:ln`)
	}
	if l.Cap != "" {
		buf.WriteString(fmt.Sprintf(` cap="%s"`, l.Cap))
	}
	if l.Compound != "" {
		buf.WriteString(fmt.Sprintf(` cmpd="%s"`, l.Compound))
	}
	if l.Align != "" {
		buf.WriteString(fmt.Sprintf(` algn="%s"`, l.Align))
	}
	buf.WriteString(`>`)

	// Fill
	l.Fill.WriteTo(buf)

	// Dash
	if l.Dash != "" {
		buf.WriteString(fmt.Sprintf(`<a:prstDash val="%s"/>`, l.Dash))
	}

	// Join
	switch l.Join {
	case "round":
		buf.WriteString(`<a:round/>`)
	case "bevel":
		buf.WriteString(`<a:bevel/>`)
	case "miter":
		buf.WriteString(`<a:miter/>`)
	}

	buf.WriteString(`</a:ln>`)
}
