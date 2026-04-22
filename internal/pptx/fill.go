package pptx

import (
	"bytes"
	"fmt"
	"strings"
)

// fillType distinguishes between the different fill kinds in DrawingML.
type fillType int

const (
	fillNone    fillType = iota
	fillSolid
	fillScheme
	fillGradient
	fillPattern
	fillPicture
)

// gradientType distinguishes between linear and radial gradients.
type gradientType int

const (
	gradientLinear gradientType = iota
	gradientRadial
)

// ColorMod represents a color modifier element (lumMod, lumOff, alpha, tint, shade).
type ColorMod struct {
	name string
	val  int // percentage × 1000 (e.g. 75000 = 75%)
}

// LumMod creates a luminance modulation modifier.
// val is in thousandths of a percent (e.g., 75000 = 75%).
func LumMod(val int) ColorMod { return ColorMod{name: "lumMod", val: val} }

// LumOff creates a luminance offset modifier.
// val is in thousandths of a percent (e.g., 25000 = 25%).
func LumOff(val int) ColorMod { return ColorMod{name: "lumOff", val: val} }

// Alpha creates an alpha transparency modifier.
// val is in thousandths of a percent (e.g., 50000 = 50%).
func Alpha(val int) ColorMod { return ColorMod{name: "alpha", val: val} }

// Tint creates a tint modifier.
// val is in thousandths of a percent (e.g., 60000 = 60%).
func Tint(val int) ColorMod { return ColorMod{name: "tint", val: val} }

// Shade creates a shade modifier.
// val is in thousandths of a percent (e.g., 80000 = 80%).
func Shade(val int) ColorMod { return ColorMod{name: "shade", val: val} }

// GradientStop defines a single color stop in a gradient fill.
type GradientStop struct {
	Position int  // Position in thousandths of a percent (0–100000)
	Color    Fill // Must be a solid or scheme fill for the color
}

// Fill represents a DrawingML fill specification.
type Fill struct {
	set    bool       // true if explicitly constructed (not zero value)
	typ    fillType
	color  string     // hex color (without #) for solid fills
	scheme string     // scheme color name for scheme fills (e.g. "accent1", "dk1")
	mods   []ColorMod // color modifiers

	// Gradient fill fields
	gradType gradientType
	stops    []GradientStop
	angle    int64 // angle in 60000ths of a degree for linear gradients
	scaled   bool

	// Pattern fill fields
	pattern   string // preset pattern name (e.g. "pct5", "horz", "ltVert")
	patternFG string // foreground hex color
	patternBG string // background hex color

	// Picture fill fields
	rID string // relationship ID to embedded image
}

// NoFill creates a fill that renders as <a:noFill/>.
func NoFill() Fill {
	return Fill{set: true, typ: fillNone}
}

// SolidFill creates a solid fill with the given hex color (e.g. "4472C4").
func SolidFill(hex string) Fill {
	return Fill{set: true, typ: fillSolid, color: hex}
}

// SolidFillWithAlpha creates a solid fill with alpha transparency.
// alpha is in thousandths of a percent (e.g., 50000 = 50%).
func SolidFillWithAlpha(hex string, alpha int) Fill {
	return Fill{set: true, typ: fillSolid, color: hex, mods: []ColorMod{Alpha(alpha)}}
}

// SchemeFill creates a fill referencing a theme color scheme (e.g. "accent1", "dk1").
// Optional color modifiers adjust luminance, alpha, etc.
func SchemeFill(scheme string, mods ...ColorMod) Fill {
	return Fill{set: true, typ: fillScheme, scheme: scheme, mods: mods}
}

// LinearGradient creates a linear gradient fill.
// angle is in 60000ths of a degree (e.g. 5400000 = 90°).
func LinearGradient(angle int64, stops ...GradientStop) Fill {
	return Fill{set: true, typ: fillGradient, gradType: gradientLinear, angle: angle, scaled: true, stops: stops}
}

// RadialGradient creates a radial (path) gradient fill.
func RadialGradient(stops ...GradientStop) Fill {
	return Fill{set: true, typ: fillGradient, gradType: gradientRadial, stops: stops}
}

// PatternFill creates a pattern fill with the given preset pattern name
// and foreground/background hex colors.
func PatternFill(preset, fg, bg string) Fill {
	return Fill{set: true, typ: fillPattern, pattern: preset, patternFG: fg, patternBG: bg}
}

// PictureFill creates a picture (blip) fill referencing an image by relationship ID.
func PictureFill(rID string) Fill {
	return Fill{set: true, typ: fillPicture, rID: rID}
}

// SchemeColorNames is the set of valid OOXML theme color scheme names.
var SchemeColorNames = map[string]bool{
	"accent1": true, "accent2": true, "accent3": true,
	"accent4": true, "accent5": true, "accent6": true,
	"dk1": true, "dk2": true, "lt1": true, "lt2": true,
	"tx1": true, "tx2": true,
	"bg1": true, "bg2": true,
	"hlink": true, "folHlink": true,
}

// IsSchemeColor returns true if s is a valid OOXML scheme color name.
func IsSchemeColor(s string) bool {
	return SchemeColorNames[s]
}

// ResolveColorString returns a SchemeFill for scheme color names (e.g. "accent1"),
// a SolidFill for hex colors (with or without "#" prefix), or a zero Fill for empty strings.
func ResolveColorString(s string) Fill {
	if s == "" {
		return Fill{}
	}
	if s == "none" {
		return NoFill()
	}
	if IsSchemeColor(s) {
		return SchemeFill(s)
	}
	return SolidFill(strings.TrimPrefix(s, "#"))
}

// IsZero returns true if the Fill has not been set (zero value).
func (f Fill) IsZero() bool {
	return !f.set
}

// WriteTo writes the fill's DrawingML XML into buf.
func (f Fill) WriteTo(buf *bytes.Buffer) {
	switch f.typ {
	case fillNone:
		buf.WriteString(`<a:noFill/>`)
	case fillSolid:
		buf.WriteString(`<a:solidFill>`)
		writeColorElement(buf, "srgbClr", f.color, f.mods)
		buf.WriteString(`</a:solidFill>`)
	case fillScheme:
		buf.WriteString(`<a:solidFill>`)
		writeColorElement(buf, "schemeClr", f.scheme, f.mods)
		buf.WriteString(`</a:solidFill>`)
	case fillGradient:
		f.writeGradientFill(buf)
	case fillPattern:
		f.writePatternFill(buf)
	case fillPicture:
		f.writePictureFill(buf)
	}
}

// WriteColorTo writes just the color element (srgbClr or schemeClr) without
// the solidFill/noFill wrapper. Used for contexts like buClr that expect a
// bare color reference.
func (f Fill) WriteColorTo(buf *bytes.Buffer) {
	switch f.typ {
	case fillSolid:
		writeColorElement(buf, "srgbClr", f.color, f.mods)
	case fillScheme:
		writeColorElement(buf, "schemeClr", f.scheme, f.mods)
	}
}

// writeColorElement writes a color element with optional modifier children.
func writeColorElement(buf *bytes.Buffer, tag, val string, mods []ColorMod) {
	if len(mods) == 0 {
		fmt.Fprintf(buf, `<a:%s val="%s"/>`, tag, val)
		return
	}
	fmt.Fprintf(buf, `<a:%s val="%s">`, tag, val)
	for _, m := range mods {
		fmt.Fprintf(buf, `<a:%s val="%d"/>`, m.name, m.val)
	}
	fmt.Fprintf(buf, `</a:%s>`, tag)
}

// writeGradientFill writes a <a:gradFill> element.
func (f Fill) writeGradientFill(buf *bytes.Buffer) {
	buf.WriteString(`<a:gradFill>`)
	buf.WriteString(`<a:gsLst>`)
	for _, stop := range f.stops {
		fmt.Fprintf(buf, `<a:gs pos="%d">`, stop.Position)
		stop.Color.WriteColorTo(buf)
		buf.WriteString(`</a:gs>`)
	}
	buf.WriteString(`</a:gsLst>`)
	switch f.gradType {
	case gradientLinear:
		scaled := "0"
		if f.scaled {
			scaled = "1"
		}
		fmt.Fprintf(buf, `<a:lin ang="%d" scaled="%s"/>`, f.angle, scaled)
	case gradientRadial:
		buf.WriteString(`<a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path>`)
	}
	buf.WriteString(`</a:gradFill>`)
}

// writePatternFill writes a <a:pattFill> element.
func (f Fill) writePatternFill(buf *bytes.Buffer) {
	fmt.Fprintf(buf, `<a:pattFill prst="%s">`, f.pattern)
	writePatternColor(buf, "fgClr", f.patternFG)
	writePatternColor(buf, "bgClr", f.patternBG)
	buf.WriteString(`</a:pattFill>`)
}

// writePatternColor writes a pattern color element, using schemeClr for scheme
// names and srgbClr for hex values.
func writePatternColor(buf *bytes.Buffer, tag, color string) {
	fmt.Fprintf(buf, `<a:%s>`, tag)
	if IsSchemeColor(color) {
		fmt.Fprintf(buf, `<a:schemeClr val="%s"/>`, color)
	} else {
		fmt.Fprintf(buf, `<a:srgbClr val="%s"/>`, strings.TrimPrefix(color, "#"))
	}
	fmt.Fprintf(buf, `</a:%s>`, tag)
}

// writePictureFill writes a <a:blipFill> element.
func (f Fill) writePictureFill(buf *bytes.Buffer) {
	fmt.Fprintf(buf, `<a:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`, f.rID)
}
