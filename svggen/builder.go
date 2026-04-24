package svggen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"regexp"
	"strconv"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/sebahrens/json2pptx/svggen/raster"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/pdf"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"github.com/tdewolff/canvas/renderers/svg"
)

// Point represents a 2D point.
type Point struct {
	X, Y float64
}

// Rect represents a rectangle with position and size.
type Rect struct {
	X, Y, W, H float64
}

// Min returns the minimum point of the rectangle.
func (r Rect) Min() Point {
	return Point{X: r.X, Y: r.Y}
}

// Max returns the maximum point of the rectangle.
func (r Rect) Max() Point {
	return Point{X: r.X + r.W, Y: r.Y + r.H}
}

// Center returns the center point of the rectangle.
func (r Rect) Center() Point {
	return Point{X: r.X + r.W/2, Y: r.Y + r.H/2}
}

// Inset returns a rectangle inset by the given margins.
func (r Rect) Inset(top, right, bottom, left float64) Rect {
	return Rect{
		X: r.X + left,
		Y: r.Y + top,
		W: r.W - left - right,
		H: r.H - top - bottom,
	}
}

// TextAlign specifies horizontal text alignment.
type TextAlign int

const (
	TextAlignLeft TextAlign = iota
	TextAlignCenter
	TextAlignRight
)

// TextBaseline specifies vertical text alignment.
type TextBaseline int

const (
	TextBaselineTop TextBaseline = iota
	TextBaselineMiddle
	TextBaselineBottom
	TextBaselineAlphabetic
)

// LineCap specifies the line cap style.
type LineCap int

const (
	LineCapButt LineCap = iota
	LineCapRound
	LineCapSquare
)

// LineJoin specifies the line join style.
type LineJoin int

const (
	LineJoinMiter LineJoin = iota
	LineJoinRound
	LineJoinBevel
)

// SVGBuilder provides a composable builder API for creating SVG documents.
// It wraps the tdewolff/canvas library to provide deterministic SVG output.
type SVGBuilder struct {
	// Dimensions
	width  float64
	height float64

	// Canvas and context
	canvas *canvas.Canvas
	ctx    *canvas.Context

	// Font settings
	fontFamily *canvas.FontFamily
	fontSize   float64
	fontStyle  canvas.FontStyle

	// Text color tracking — when non-nil, DrawText uses this instead of TextPrimary.
	// Set via SetFillColor (for backward compatibility) or SetTextColor.
	// Saved/restored by Push/Pop.
	textColor      *Color
	textColorStack []*Color

	// Style guide
	style *StyleGuide

	// minFontSizeFloor is the absolute minimum font size (in points) below which
	// text will be truncated with ellipsis rather than further shrunk.
	// Zero means use DefaultMinFontSize. Set via SetMinFontSize.
	minFontSizeFloor float64

	// fontErr records the first font-related error (nil face from safeFace).
	// Surfaced by FontErr() and Render(). Once set, all text operations no-op
	// to avoid cascading failures, but the error is never silently swallowed.
	fontErr error
}

// FontErr returns the first font-related error recorded during text operations,
// or nil if all text rendered successfully. Callers that need to distinguish
// "no text was drawn" from "font unavailable" should check this after building.
func (b *SVGBuilder) FontErr() error {
	return b.fontErr
}

// NewSVGBuilder creates a new SVGBuilder with the specified dimensions in points.
func NewSVGBuilder(width, height float64) *SVGBuilder {
	// Create canvas (canvas uses mm, so convert from points: 1pt = 0.3528mm)
	c := canvas.New(width*ptToMM, height*ptToMM)
	ctx := canvas.NewContext(c)

	b := &SVGBuilder{
		width:     width,
		height:    height,
		canvas:    c,
		ctx:       ctx,
		fontSize:  12,
		fontStyle: canvas.FontRegular,
		style:     DefaultStyleGuide(),
	}

	// Load default font
	b.loadDefaultFont()

	return b
}

// DefaultMinFontSize is the absolute minimum font size floor (in points).
// Text will never be rendered smaller than this; instead it will be truncated
// with an ellipsis. This prevents unreadable text in dense diagrams.
// The value of 7pt allows dense diagrams (SWOT, fishbone, nine-box, org chart)
// to show more text before resorting to truncation, while remaining legible.
const DefaultMinFontSize = 7.0

// Points to mm conversion factor
const ptToMM = 0.3528

// mmToPt converts millimeters to points
const mmToPt = 2.8346

// fontSizePxRe matches font-size values with px units in CSS font declarations.
// The canvas library outputs font sizes in mm but labels them as "px". We replace
// "px" with "mm" so font sizes are in the same coordinate space as the viewBox
// (which uses mm). This ensures correct rendering when SVGs are opened standalone
// or used for PNG rasterization via the canvas library.
var fontSizePxRe = regexp.MustCompile(`(\d+\.?\d*)px(\s)`)

// fixSVGFontUnits replaces "px" with "mm" in font-size declarations.
// DEPRECATED: No longer called. Kept for reference and tests.
// Using "mm" makes fonts absolute (don't scale with viewBox), causing text to
// appear oversized when SVGs are embedded in PPTX placeholders smaller than
// the native viewBox size. Leaving fonts as "px" (= SVG user units) ensures
// they scale proportionally with shapes when the SVG is resized.
func fixSVGFontUnits(svgContent []byte) []byte {
	return fontSizePxRe.ReplaceAll(svgContent, []byte("${1}mm${2}"))
}

// svgViewportMMRe matches mm-based viewport dimensions in the SVG root element.
// Used by scaleSVGToPixelCoordsSafe to convert to pixel-based viewport.
var svgViewportMMRe = regexp.MustCompile(`width="[0-9.]+mm" height="[0-9.]+mm"`)

// matrixRotateRe matches matrix transforms that represent -90° or +90° rotations.
// matrix(0,-1,1,0,tx,ty) is -90° rotation around (tx,ty).
// matrix(0,1,-1,0,tx,ty) is +90° rotation around (tx,ty).
// LibreOffice misrenders matrix-form rotations on text elements when SVG is
// embedded inside PPTX; converting to translate+rotate form fixes this.
var matrixRotateRe = regexp.MustCompile(
	`transform="matrix\((0),(-1),(1),(0),([^,]+),([^)]+)\)"`)

// fixSVGMatrixRotations replaces matrix-form rotation transforms with
// equivalent translate+rotate form for better LibreOffice compatibility.
func fixSVGMatrixRotations(svgContent []byte) []byte {
	return matrixRotateRe.ReplaceAllFunc(svgContent, func(match []byte) []byte {
		parts := matrixRotateRe.FindSubmatch(match)
		if len(parts) < 7 {
			return match
		}
		tx := string(parts[5])
		ty := string(parts[6])
		// matrix(0,-1,1,0,tx,ty) = translate(tx,ty) rotate(-90)
		return []byte(fmt.Sprintf(`transform="translate(%s,%s) rotate(-90)"`, tx, ty))
	})
}

// fontFamilyInStyleRe matches font family names at the end of CSS font shorthand
// declarations within style attributes. It captures the font-size value and the
// single font family name, so we can append generic fallbacks.
//
// Matches patterns like:
//   - font: 13.33px Arial      -> font: 13.33px Arial, Helvetica, sans-serif
//   - font: 700 24px Arial     -> font: 700 24px Arial, Helvetica, sans-serif
//   - font: italic 12px MyFont -> font: italic 12px MyFont, Helvetica, sans-serif
//
// The regex captures everything up to and including "Npx " and the family name,
// stopping at ";" or end of style attribute ("). It avoids matching families that
// already contain a comma (i.e., already have fallbacks).
var fontFamilyInStyleRe = regexp.MustCompile(`(font:\s*(?:\w+\s+)*[\d.]+px\s+)([\w-]+)([;"])`)

// fixSVGFontFamilyFallbacks adds generic font-family fallbacks to CSS font
// shorthand declarations in SVG text elements. LibreOffice's SVG renderer may
// fail to load embedded @font-face fonts, and without fallback families the text
// renders with an incorrect default font, causing visual corruption in PDF export.
//
// Before: font: 13.33px Arial;fill:#212529
// After:  font: 13.33px Arial, Helvetica, sans-serif;fill:#212529
func fixSVGFontFamilyFallbacks(svgContent []byte) []byte {
	return fontFamilyInStyleRe.ReplaceAllFunc(svgContent, func(match []byte) []byte {
		parts := fontFamilyInStyleRe.FindSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		prefix := parts[1]   // "font: [weight] Npx "
		family := parts[2]   // "Arial"
		terminator := parts[3] // ";" or "\""

		// Build replacement with fallbacks
		var buf bytes.Buffer
		buf.Write(prefix)
		buf.Write(family)
		buf.WriteString(", Helvetica, sans-serif")
		buf.Write(terminator)
		return buf.Bytes()
	})
}

// textAnchorRe matches <text x="X1" ...><tspan x="X2" ...> patterns to detect
// center-aligned text that needs text-anchor="middle" for cross-renderer compatibility.
var textAnchorRe = regexp.MustCompile(`<text x="([\d.]+)"([^>]*)><tspan x="([\d.]+)"`)

// fixSVGTextAlignment adds text-anchor="middle" to center-aligned text elements.
//
// The canvas library pre-computes tspan x positions for center alignment using its
// own font metrics, but SVG renderers (rsvg-convert, LibreOffice) use different
// (often wider) fonts. Without text-anchor, the renderer treats the pre-computed
// tspan x as a left-aligned starting point, causing bold/wide text to overflow
// rightward past the canvas edge. Adding text-anchor="middle" lets each renderer
// center text using its own metrics, distributing any width difference equally
// on both sides instead of pushing overflow to the right.
func fixSVGTextAlignment(svgContent []byte) []byte {
	return textAnchorRe.ReplaceAllFunc(svgContent, func(match []byte) []byte {
		parts := textAnchorRe.FindSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		textX, err1 := strconv.ParseFloat(string(parts[1]), 64)
		tspanX, err2 := strconv.ParseFloat(string(parts[3]), 64)
		if err1 != nil || err2 != nil {
			return match
		}

		// Center-aligned text: canvas shifted tspan left by half the measured
		// text width, so text.x > tspan.x. Threshold of 0.5mm avoids matching
		// left-aligned text where the difference is just floating-point noise.
		if textX-tspanX > 0.5 {
			attrs := string(parts[2])
			return []byte(fmt.Sprintf(`<text x="%s" text-anchor="middle"%s><tspan x="%s"`,
				string(parts[1]), attrs, string(parts[1])))
		}

		return match
	})
}

// loadDefaultFont loads the default Arial font family from cache.
// Uses the shared fontcache package to avoid repeated expensive font loading.
// Sets fontErr if no font could be loaded (including embedded fallbacks).
func (b *SVGBuilder) loadDefaultFont() {
	b.fontFamily = fontcache.Get("Arial", "Helvetica")
	if b.fontFamily == nil {
		b.fontErr = fmt.Errorf("svggen: failed to load any font (requested Arial, fallback Helvetica, embedded Liberation Sans all unavailable)")
	}
}

// SetStyleGuide sets the style guide for the builder.
// Also updates the font family if the style guide's typography specifies one.
func (b *SVGBuilder) SetStyleGuide(style *StyleGuide) *SVGBuilder {
	b.style = style
	// Update font family from style guide's typography settings
	if style != nil && style.Typography != nil && style.Typography.FontFamily != "" {
		// Always use Arial as fallback since it's available on most systems.
		// The user's requested font will be tried first, falling back to Arial if unavailable.
		b.fontFamily = fontcache.Get(style.Typography.FontFamily, "Arial")
	}
	return b
}

// StyleGuide returns the current style guide.
func (b *SVGBuilder) StyleGuide() *StyleGuide {
	return b.style
}

// SetFontFamily sets the font family by name.
// Uses the shared fontcache package to avoid repeated expensive font loading.
// Sets fontErr if the requested font (and all fallbacks) could not be loaded.
func (b *SVGBuilder) SetFontFamily(name string) *SVGBuilder {
	b.fontFamily = fontcache.Get(name, "")
	if b.fontFamily == nil {
		b.fontErr = fmt.Errorf("svggen: font %q unavailable (no system or embedded fallback found)", name)
	}
	return b
}

// SetFontSize sets the font size in points.
func (b *SVGBuilder) SetFontSize(size float64) *SVGBuilder {
	b.fontSize = size
	return b
}

// FontSize returns the current font size in points.
func (b *SVGBuilder) FontSize() float64 {
	return b.fontSize
}

// SetMinFontSize sets the absolute minimum font size floor (in points).
// ClampFontSize and ClampFontSizeForRect will never return a value below this.
// Set to 0 to use DefaultMinFontSize.
func (b *SVGBuilder) SetMinFontSize(size float64) *SVGBuilder {
	b.minFontSizeFloor = size
	return b
}

// MinFontSize returns the effective minimum font size floor.
// If no custom value has been set, returns DefaultMinFontSize.
func (b *SVGBuilder) MinFontSize() float64 {
	if b.minFontSizeFloor > 0 {
		return b.minFontSizeFloor
	}
	return DefaultMinFontSize
}

// SetFontWeight sets the font weight.
func (b *SVGBuilder) SetFontWeight(weight int) *SVGBuilder {
	switch {
	case weight <= 300:
		b.fontStyle = canvas.FontLight
	case weight <= 400:
		b.fontStyle = canvas.FontRegular
	case weight <= 500:
		b.fontStyle = canvas.FontMedium
	case weight <= 600:
		b.fontStyle = canvas.FontSemiBold
	default:
		b.fontStyle = canvas.FontBold
	}
	return b
}

// colorToRGBA converts our Color type to a standard library color.
// We use color.NRGBA (non-premultiplied alpha) because our Color stores
// straight alpha: R, G, B are full-intensity values independent of A.
// Using color.RGBA (premultiplied) would misinterpret the values, causing
// semi-transparent colors to render as white.
func colorToRGBA(c Color) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: uint8(c.A * 255)}
}

// SetFillColor sets the fill color for shapes (rectangles, circles, polygons).
// For text color, use SetTextColor instead.
func (b *SVGBuilder) SetFillColor(c Color) *SVGBuilder {
	b.ctx.SetFillColor(colorToRGBA(c))
	return b
}

// SetTextColor sets the text color for subsequent DrawText calls.
// Resets on Pop() to the value saved by the corresponding Push().
func (b *SVGBuilder) SetTextColor(c Color) *SVGBuilder {
	cc := c
	b.textColor = &cc
	return b
}

// SetStrokeColor sets the stroke color.
func (b *SVGBuilder) SetStrokeColor(c Color) *SVGBuilder {
	b.ctx.SetStrokeColor(colorToRGBA(c))
	return b
}

// SetStrokeWidth sets the stroke width in points.
func (b *SVGBuilder) SetStrokeWidth(width float64) *SVGBuilder {
	b.ctx.SetStrokeWidth(width * ptToMM)
	return b
}

// SetLineCap sets the line cap style.
func (b *SVGBuilder) SetLineCap(cap LineCap) *SVGBuilder {
	switch cap {
	case LineCapButt:
		b.ctx.SetStrokeCapper(canvas.ButtCap)
	case LineCapRound:
		b.ctx.SetStrokeCapper(canvas.RoundCap)
	case LineCapSquare:
		b.ctx.SetStrokeCapper(canvas.SquareCap)
	}
	return b
}

// SetLineJoin sets the line join style.
func (b *SVGBuilder) SetLineJoin(join LineJoin) *SVGBuilder {
	switch join {
	case LineJoinMiter:
		b.ctx.SetStrokeJoiner(canvas.MiterJoiner{Limit: 4.0})
	case LineJoinRound:
		b.ctx.SetStrokeJoiner(canvas.RoundJoiner{})
	case LineJoinBevel:
		b.ctx.SetStrokeJoiner(canvas.BevelJoiner{})
	}
	return b
}

// SetDashes sets the dash pattern. Empty pattern means solid line.
func (b *SVGBuilder) SetDashes(pattern ...float64) *SVGBuilder {
	// Convert points to mm
	mmPattern := make([]float64, len(pattern))
	for i, v := range pattern {
		mmPattern[i] = v * ptToMM
	}
	b.ctx.SetDashes(0, mmPattern...)
	return b
}

// Push saves the current state onto the stack.
func (b *SVGBuilder) Push() *SVGBuilder {
	b.ctx.Push()
	b.textColorStack = append(b.textColorStack, b.textColor)
	return b
}

// Pop restores the previous state from the stack.
func (b *SVGBuilder) Pop() *SVGBuilder {
	b.ctx.Pop()
	if n := len(b.textColorStack); n > 0 {
		b.textColor = b.textColorStack[n-1]
		b.textColorStack = b.textColorStack[:n-1]
	}
	return b
}

// DrawRect draws a rectangle.
func (b *SVGBuilder) DrawRect(r Rect) *SVGBuilder {
	// Create rectangle path
	p := &canvas.Path{}
	// Convert to mm and flip Y coordinate (canvas uses bottom-left origin)
	x := r.X * ptToMM
	y := (b.height - r.Y - r.H) * ptToMM
	w := r.W * ptToMM
	h := r.H * ptToMM

	p.MoveTo(x, y)
	p.LineTo(x+w, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()

	b.ctx.DrawPath(0, 0, p)
	return b
}

// DrawRoundedRect draws a rectangle with rounded corners.
func (b *SVGBuilder) DrawRoundedRect(r Rect, radius float64) *SVGBuilder {
	// Convert to mm
	x := r.X * ptToMM
	y := (b.height - r.Y - r.H) * ptToMM
	w := r.W * ptToMM
	h := r.H * ptToMM
	rx := radius * ptToMM

	// Clamp radius to half the smallest dimension
	maxR := min(w, h) / 2
	if rx > maxR {
		rx = maxR
	}

	p := &canvas.Path{}
	// Start at top-left corner after the radius
	p.MoveTo(x+rx, y)
	// Top edge
	p.LineTo(x+w-rx, y)
	// Top-right corner
	p.QuadTo(x+w, y, x+w, y+rx)
	// Right edge
	p.LineTo(x+w, y+h-rx)
	// Bottom-right corner
	p.QuadTo(x+w, y+h, x+w-rx, y+h)
	// Bottom edge
	p.LineTo(x+rx, y+h)
	// Bottom-left corner
	p.QuadTo(x, y+h, x, y+h-rx)
	// Left edge
	p.LineTo(x, y+rx)
	// Top-left corner
	p.QuadTo(x, y, x+rx, y)
	p.Close()

	b.ctx.DrawPath(0, 0, p)
	return b
}

// FillRect draws and fills a rectangle.
func (b *SVGBuilder) FillRect(r Rect) *SVGBuilder {
	b.ctx.Push()
	b.ctx.SetStrokeColor(color.Transparent)
	b.DrawRect(r)
	b.ctx.Pop()
	return b
}

// StrokeRect draws a rectangle with stroke only (no fill).
func (b *SVGBuilder) StrokeRect(r Rect) *SVGBuilder {
	b.ctx.Push()
	b.ctx.SetFillColor(color.Transparent)
	b.DrawRect(r)
	b.ctx.Pop()
	return b
}

// DrawCircle draws a circle centered at (cx, cy) with the given radius.
func (b *SVGBuilder) DrawCircle(cx, cy, radius float64) *SVGBuilder {
	return b.DrawEllipse(cx, cy, radius, radius)
}

// DrawEllipse draws an ellipse centered at (cx, cy) with the given radii.
func (b *SVGBuilder) DrawEllipse(cx, cy, rx, ry float64) *SVGBuilder {
	// Convert to mm and flip Y coordinate
	cxMM := cx * ptToMM
	cyMM := (b.height - cy) * ptToMM
	rxMM := rx * ptToMM
	ryMM := ry * ptToMM

	// Create ellipse path using arcs
	p := &canvas.Path{}
	// Start at rightmost point
	p.MoveTo(cxMM+rxMM, cyMM)
	// Top half (counter-clockwise in canvas coordinates)
	p.ArcTo(rxMM, ryMM, 0, false, true, cxMM-rxMM, cyMM)
	// Bottom half
	p.ArcTo(rxMM, ryMM, 0, false, true, cxMM+rxMM, cyMM)
	p.Close()

	b.ctx.DrawPath(0, 0, p)
	return b
}

// DrawLine draws a line from (x1, y1) to (x2, y2).
func (b *SVGBuilder) DrawLine(x1, y1, x2, y2 float64) *SVGBuilder {
	// Convert to mm and flip Y coordinates
	x1MM := x1 * ptToMM
	y1MM := (b.height - y1) * ptToMM
	x2MM := x2 * ptToMM
	y2MM := (b.height - y2) * ptToMM

	p := &canvas.Path{}
	p.MoveTo(x1MM, y1MM)
	p.LineTo(x2MM, y2MM)

	b.ctx.Push()
	b.ctx.SetFillColor(color.Transparent)
	b.ctx.DrawPath(0, 0, p)
	b.ctx.Pop()
	return b
}

// DrawPolyline draws a series of connected lines.
func (b *SVGBuilder) DrawPolyline(points []Point) *SVGBuilder {
	if len(points) < 2 {
		return b
	}

	p := &canvas.Path{}
	first := points[0]
	p.MoveTo(first.X*ptToMM, (b.height-first.Y)*ptToMM)

	for i := 1; i < len(points); i++ {
		pt := points[i] //nolint:gosec // G602: i is bounded by len(points)
		p.LineTo(pt.X*ptToMM, (b.height-pt.Y)*ptToMM)
	}

	b.ctx.Push()
	b.ctx.SetFillColor(color.Transparent)
	b.ctx.DrawPath(0, 0, p)
	b.ctx.Pop()
	return b
}

// DrawPolygon draws a closed polygon.
func (b *SVGBuilder) DrawPolygon(points []Point) *SVGBuilder {
	if len(points) < 3 {
		return b
	}

	p := &canvas.Path{}
	first := points[0]
	p.MoveTo(first.X*ptToMM, (b.height-first.Y)*ptToMM)

	for i := 1; i < len(points); i++ {
		pt := points[i]
		p.LineTo(pt.X*ptToMM, (b.height-pt.Y)*ptToMM)
	}
	p.Close()

	b.ctx.DrawPath(0, 0, p)
	return b
}

// DrawImage draws a raster image scaled to fit within the given rectangle.
// The image is drawn at the rectangle's position and scaled to its dimensions.
// Coordinates are in points (top-left origin); the method handles conversion
// to the canvas library's mm-based bottom-left coordinate system.
func (b *SVGBuilder) DrawImage(img image.Image, r Rect) *SVGBuilder {
	if img == nil {
		return b
	}

	// Target dimensions in mm
	drawW := r.W * ptToMM
	drawH := r.H * ptToMM

	// Image pixel dimensions
	imgW := float64(img.Bounds().Dx())
	imgH := float64(img.Bounds().Dy())
	if imgW == 0 || imgH == 0 {
		return b
	}

	// Position in mm with Y-flip (canvas origin is bottom-left)
	xMM := r.X * ptToMM
	yMM := (b.height-r.Y-r.H)*ptToMM // bottom edge of rect in canvas coords

	// Calculate dots-per-mm so the image fills the target rect.
	// Use the larger DPMM (smaller scale) to fit within bounds, then
	// let the canvas library handle the scaling.
	dpmmX := imgW / drawW
	dpmmY := imgH / drawH
	dpmm := math.Max(dpmmX, dpmmY)

	b.ctx.DrawImage(xMM, yMM, img, canvas.DPMM(dpmm))
	return b
}

// PathBuilder provides a fluent API for constructing paths.
type PathBuilder struct {
	builder *SVGBuilder
	path    *canvas.Path
	height  float64
}

// BeginPath starts a new path.
func (b *SVGBuilder) BeginPath() *PathBuilder {
	return &PathBuilder{
		builder: b,
		path:    &canvas.Path{},
		height:  b.height,
	}
}

// MoveTo moves to the specified point.
func (p *PathBuilder) MoveTo(x, y float64) *PathBuilder {
	p.path.MoveTo(x*ptToMM, (p.height-y)*ptToMM)
	return p
}

// LineTo draws a line to the specified point.
func (p *PathBuilder) LineTo(x, y float64) *PathBuilder {
	p.path.LineTo(x*ptToMM, (p.height-y)*ptToMM)
	return p
}

// QuadTo draws a quadratic bezier curve.
func (p *PathBuilder) QuadTo(cpx, cpy, x, y float64) *PathBuilder {
	p.path.QuadTo(
		cpx*ptToMM, (p.height-cpy)*ptToMM,
		x*ptToMM, (p.height-y)*ptToMM,
	)
	return p
}

// CubicTo draws a cubic bezier curve.
func (p *PathBuilder) CubicTo(cp1x, cp1y, cp2x, cp2y, x, y float64) *PathBuilder {
	p.path.CubeTo(
		cp1x*ptToMM, (p.height-cp1y)*ptToMM,
		cp2x*ptToMM, (p.height-cp2y)*ptToMM,
		x*ptToMM, (p.height-y)*ptToMM,
	)
	return p
}

// ArcTo draws an arc.
// Note: The sweep flag is inverted because we flip the Y-axis in our coordinate
// transformation. When Y is mirrored, clockwise becomes counter-clockwise and vice versa.
func (p *PathBuilder) ArcTo(rx, ry, rotation float64, largeArc, sweep bool, x, y float64) *PathBuilder {
	p.path.ArcTo(
		rx*ptToMM, ry*ptToMM,
		rotation,
		largeArc, !sweep, // Invert sweep due to Y-axis flip
		x*ptToMM, (p.height-y)*ptToMM,
	)
	return p
}

// Close closes the current path.
func (p *PathBuilder) Close() *PathBuilder {
	p.path.Close()
	return p
}

// Draw draws the path with current fill and stroke settings.
func (p *PathBuilder) Draw() *SVGBuilder {
	p.builder.ctx.DrawPath(0, 0, p.path)
	return p.builder
}

// Fill fills the path (no stroke).
func (p *PathBuilder) Fill() *SVGBuilder {
	p.builder.ctx.Push()
	p.builder.ctx.SetStrokeColor(color.Transparent)
	p.builder.ctx.SetStrokeWidth(0)
	p.builder.ctx.DrawPath(0, 0, p.path)
	p.builder.ctx.Pop()
	return p.builder
}

// Stroke strokes the path (no fill).
func (p *PathBuilder) Stroke() *SVGBuilder {
	p.builder.ctx.Push()
	p.builder.ctx.SetFillColor(color.Transparent)
	p.builder.ctx.DrawPath(0, 0, p.path)
	p.builder.ctx.Pop()
	return p.builder
}

// DrawText draws text at the specified position with alignment.
// Font sizes are clamped to MinFontSize() to enforce a legibility floor
// across all chart types. Title text (which is already large) is unaffected.
func (b *SVGBuilder) DrawText(text string, x, y float64, align TextAlign, baseline TextBaseline) *SVGBuilder {
	// Enforce minimum font size floor for legibility.
	// This is a safety net: chart code should use ClampFontSize where possible,
	// but this catch-all prevents any text from rendering below the floor.
	if floor := b.MinFontSize(); b.fontSize > 0 && b.fontSize < floor {
		b.fontSize = floor
	}

	// Determine text color: use explicitly set color, fall back to TextPrimary
	textCol := b.style.Palette.TextPrimary
	if b.textColor != nil {
		textCol = *b.textColor
	}

	// Create font face — fail loud if unavailable
	face, err := b.safeFace(colorToRGBA(textCol))
	if err != nil {
		if b.fontErr == nil {
			b.fontErr = fmt.Errorf("DrawText(%q): %w", text, err)
		}
		return b
	}

	// Determine canvas text alignment
	var canvasAlign canvas.TextAlign
	switch align {
	case TextAlignLeft:
		canvasAlign = canvas.Left
	case TextAlignCenter:
		canvasAlign = canvas.Center
	case TextAlignRight:
		canvasAlign = canvas.Right
	}

	// Create text line
	textLine := canvas.NewTextLine(face, text, canvasAlign)

	// Convert coordinates (canvas uses bottom-left origin)
	xMM := x * ptToMM
	yMM := (b.height - y) * ptToMM

	// Adjust for baseline
	metrics := face.Metrics()
	switch baseline {
	case TextBaselineTop:
		yMM -= metrics.Ascent
	case TextBaselineMiddle:
		yMM -= (metrics.Ascent - metrics.Descent) / 2
	case TextBaselineBottom:
		yMM += metrics.Descent
	case TextBaselineAlphabetic:
		// Default - no adjustment needed
	}

	b.ctx.DrawText(xMM, yMM, textLine)
	return b
}

// safeFace returns a font face for the current font family. If the font is
// unavailable, it returns nil and an error describing the failure. The canvas
// library panics when a font family has no loaded fonts; this method recovers
// from that panic and converts it to an error.
func (b *SVGBuilder) safeFace(col color.NRGBA) (face *canvas.FontFace, err error) {
	if b.fontFamily == nil {
		return nil, fmt.Errorf("svggen: font family is nil — no fonts were loaded; text cannot be rendered")
	}
	defer func() {
		if r := recover(); r != nil {
			face = nil
			err = fmt.Errorf("svggen: font face creation panicked (corrupt or incompatible font): %v", r)
		}
	}()
	// Face() expects points and converts to mm internally (size * mmPerPt).
	// Passing fontSize directly (already in pt) avoids double-conversion.
	return b.fontFamily.Face(b.fontSize, col, b.fontStyle, canvas.FontNormal), nil
}

// MeasureText returns the width and height of the text in points.
func (b *SVGBuilder) MeasureText(text string) (width, height float64) {
	face, err := b.safeFace(color.NRGBA{A: 255})
	if err != nil {
		if b.fontErr == nil {
			b.fontErr = fmt.Errorf("MeasureText(%q): %w", text, err)
		}
		return 0, 0
	}

	textLine := canvas.NewTextLine(face, text, canvas.Left)
	bounds := textLine.Bounds()

	return bounds.W() * mmToPt, bounds.H() * mmToPt
}

// ClampFontSize returns the largest font size at which text fits within maxWidth,
// capped at presetSize and floored at minSize. Uses real font metrics via MeasureText.
// The effective floor is the larger of minSize and the builder's MinFontSize() —
// text is never rendered smaller than the readability floor. Callers should use
// TruncateToWidth when the returned size equals the floor and text still overflows.
func (b *SVGBuilder) ClampFontSize(text string, maxWidth, presetSize, minSize float64) float64 {
	if text == "" || maxWidth <= 0 {
		return presetSize
	}

	// Enforce the absolute minimum font size floor for readability.
	floor := b.MinFontSize()
	if minSize < floor {
		minSize = floor
	}

	// Save and restore font size
	origSize := b.fontSize
	defer b.SetFontSize(origSize)

	// Check if preset size fits
	b.SetFontSize(presetSize)
	w, _ := b.MeasureText(text)
	if w <= maxWidth {
		return presetSize
	}

	// Binary search between minSize and presetSize
	lo, hi := minSize, presetSize
	for hi-lo > 0.5 { // 0.5pt precision is sufficient
		mid := (lo + hi) / 2
		b.SetFontSize(mid)
		w, _ = b.MeasureText(text)
		if w <= maxWidth {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// ClampFontSizeForRect returns the largest font size at which wrapped text fits
// within a bounding box (maxWidth x maxHeight), capped at presetSize and floored
// at minSize. Uses WrapText for multi-line measurement.
// The effective floor is the larger of minSize and the builder's MinFontSize().
func (b *SVGBuilder) ClampFontSizeForRect(text string, maxWidth, maxHeight, presetSize, minSize float64) float64 {
	if text == "" || maxWidth <= 0 || maxHeight <= 0 {
		return presetSize
	}

	// Enforce the absolute minimum font size floor for readability.
	floor := b.MinFontSize()
	if minSize < floor {
		minSize = floor
	}

	// Save and restore font size
	origSize := b.fontSize
	defer b.SetFontSize(origSize)

	// Check if preset size fits
	b.SetFontSize(presetSize)
	block := b.WrapText(text, maxWidth)
	if block.TotalHeight <= maxHeight {
		return presetSize
	}

	// Binary search between minSize and presetSize
	lo, hi := minSize, presetSize
	for hi-lo > 0.5 {
		mid := (lo + hi) / 2
		b.SetFontSize(mid)
		block = b.WrapText(text, maxWidth)
		if block.TotalHeight <= maxHeight {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// TruncateToWidth truncates text so it fits within maxWidth at the current
// font size, appending "…" if truncation is needed. Uses MeasureText for
// accurate font-metric-based measurement.
func (b *SVGBuilder) TruncateToWidth(text string, maxWidth float64) string {
	if text == "" || maxWidth <= 0 {
		return ""
	}
	w, _ := b.MeasureText(text)
	if w <= maxWidth {
		return text
	}
	// Measure the ellipsis width once
	ellipsis := "…"
	ew, _ := b.MeasureText(ellipsis)
	if ew >= maxWidth {
		return ellipsis
	}
	// Binary search for the longest prefix that fits with ellipsis
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		candidate := string(runes[:mid]) + ellipsis
		cw, _ := b.MeasureText(candidate)
		if cw <= maxWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ellipsis
	}
	return string(runes[:lo]) + ellipsis
}

// Group represents a group of drawing operations.
type Group struct {
	builder *SVGBuilder
}

// BeginGroup starts a new group and saves the current state.
func (b *SVGBuilder) BeginGroup() *Group {
	b.Push()
	return &Group{builder: b}
}

// EndGroup ends the group and restores the previous state.
func (g *Group) End() *SVGBuilder {
	g.builder.Pop()
	return g.builder
}

// Translate translates the coordinate system.
func (b *SVGBuilder) Translate(tx, ty float64) *SVGBuilder {
	// Canvas uses mm and Y flipped, so adjust translation
	b.ctx.Translate(tx*ptToMM, -ty*ptToMM)
	return b
}

// Rotate rotates the coordinate system around the origin by angle degrees.
// The angle is negated to convert from screen coordinates (Y-down, clockwise positive)
// to the canvas library's coordinate system (Y-up, counterclockwise positive).
func (b *SVGBuilder) Rotate(angle float64) *SVGBuilder {
	b.ctx.Rotate(-angle)
	return b
}

// RotateAround rotates the coordinate system around the point (cx, cy) by angle degrees.
// Uses the canvas library's RotateAbout directly with coordinate conversion:
// negate angle for Y-flip, convert cx/cy from points to mm, flip cy for Y-up.
func (b *SVGBuilder) RotateAround(angle, cx, cy float64) *SVGBuilder {
	b.ctx.RotateAbout(-angle, cx*ptToMM, (b.height-cy)*ptToMM)
	return b
}

// Scale scales the coordinate system.
func (b *SVGBuilder) Scale(sx, sy float64) *SVGBuilder {
	b.ctx.Scale(sx, sy)
	return b
}

// Width returns the canvas width in points.
func (b *SVGBuilder) Width() float64 {
	return b.width
}

// Height returns the canvas height in points.
func (b *SVGBuilder) Height() float64 {
	return b.height
}

// Bounds returns the canvas bounds as a Rect.
func (b *SVGBuilder) Bounds() Rect {
	return Rect{X: 0, Y: 0, W: b.width, H: b.height}
}

// Render outputs the SVG document. Returns an error if any font-related
// failures occurred during text operations (missing/corrupt font).
func (b *SVGBuilder) Render() (*SVGDocument, error) {
	if b.fontErr != nil {
		return nil, b.fontErr
	}
	var buf bytes.Buffer

	// Create SVG writer with the canvas dimensions
	widthMM := b.width * ptToMM
	heightMM := b.height * ptToMM

	svgWriter := svg.New(&buf, widthMM, heightMM, nil)
	b.canvas.RenderTo(svgWriter)
	_ = svgWriter.Close()

	content := buf.Bytes()

	// Fix rotation transforms: convert matrix-form rotations to translate+rotate
	// form BEFORE pixel scaling so translate values get scaled correctly.
	content = fixSVGMatrixRotations(content)

	// Fix text alignment: add text-anchor="middle" to center-aligned text.
	// Must run BEFORE pixel scaling so the 0.5mm threshold works correctly.
	content = fixSVGTextAlignment(content)

	// Scale all SVG coordinates from mm to CSS pixels. LibreOffice and PowerPoint
	// misinterpret font-size "px" values when the viewBox uses mm-scale coordinates,
	// causing garbled/oversized text in PDF export. The only reliable fix is to
	// output SVGs where viewBox=viewport (both in CSS pixels) and all coordinates
	// and font sizes are in actual CSS pixels.
	content = scaleSVGToPixelCoordsSafe(content, widthMM, heightMM)

	// Add font-family fallbacks to CSS font shorthand declarations. LibreOffice
	// may fail to load embedded @font-face fonts from SVG; without fallbacks,
	// text renders with an incorrect default font, causing overlay corruption
	// in PDF export. Must run AFTER pixel scaling since both operate on font
	// shorthand patterns.
	content = fixSVGFontFamilyFallbacks(content)

	widthPx := math.Round(widthMM * mmToPxFactor)
	heightPx := math.Round(heightMM * mmToPxFactor)
	viewBox := fmt.Sprintf("0 0 %.0f %.0f", widthPx, heightPx)

	return &SVGDocument{
		Content: content,
		Width:   widthPx,
		Height:  heightPx,
		ViewBox: viewBox,
	}, nil
}

// RenderToBytes renders the SVG and returns the raw bytes.
func (b *SVGBuilder) RenderToBytes() ([]byte, error) {
	doc, err := b.Render()
	if err != nil {
		return nil, err
	}
	return doc.Bytes(), nil
}

// RenderToString renders the SVG and returns it as a string.
func (b *SVGBuilder) RenderToString() (string, error) {
	doc, err := b.Render()
	if err != nil {
		return "", err
	}
	return doc.String(), nil
}

// RenderPNG outputs the document as PNG bytes.
// The scale parameter controls the resolution multiplier (e.g., 2.0 for 2x resolution).
// If scale <= 0, a default of 2.0 is used.
//
// Note: This method acquires raster.Mu to serialize rasterizer calls due to a
// race condition in the tdewolff/canvas library's path intersection algorithm.
func (b *SVGBuilder) RenderPNG(scale float64) (pngBytes []byte, renderErr error) {
	if scale <= 0 {
		scale = 2.0 // Default to 2x resolution
	}

	// Standard screen is 96 DPI, so at 1x scale we use 96 DPI
	// At 2x scale we use 192 DPI, etc.
	dpi := 96.0 * scale

	// Serialize only the rasterizer call — bentleyOttmann in path_intersection.go
	// uses package-level globals (_ps, _qs, _op, _fillRule).
	// png.Encode is thread-safe and runs outside the lock.
	var img image.Image
	func() {
		raster.Mu.Lock()
		defer raster.Mu.Unlock()
		defer func() {
			if r := recover(); r != nil {
				renderErr = fmt.Errorf("svggen: PNG rasterizer panic: %v", r)
			}
		}()
		img = rasterizer.Draw(b.canvas, canvas.DPI(dpi), nil)
	}()
	if renderErr != nil {
		return nil, renderErr
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("svggen: PNG encode failed: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderPDF outputs the document as PDF bytes.
//
// Note: This method acquires raster.Mu to serialize canvas rendering due to a
// race condition in the tdewolff/canvas library's path intersection algorithm.
func (b *SVGBuilder) RenderPDF() (pdfBytes []byte, renderErr error) {
	var buf bytes.Buffer

	pdfWriter := pdf.New(&buf, b.width*ptToMM, b.height*ptToMM, nil)

	// Serialize only RenderTo — it triggers path intersection (bentleyOttmann)
	// which uses package-level globals. pdfWriter.Close() just flushes buffers.
	func() {
		raster.Mu.Lock()
		defer raster.Mu.Unlock()
		defer func() {
			if r := recover(); r != nil {
				renderErr = fmt.Errorf("svggen: PDF renderer panic: %v", r)
			}
		}()
		b.canvas.RenderTo(pdfWriter)
	}()
	if renderErr != nil {
		return nil, renderErr
	}

	if err := pdfWriter.Close(); err != nil {
		return nil, fmt.Errorf("svggen: PDF render failed: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderResult holds the output from multi-format rendering.
type BuilderRenderResult struct {
	// SVG contains the SVG document (always populated).
	SVG *SVGDocument

	// PNG contains the PNG bytes (populated if requested).
	PNG []byte

	// PDF contains the PDF bytes (populated if requested).
	PDF []byte
}

// RenderWithFormats renders the document in multiple formats.
// The formats parameter specifies which formats to include: "svg", "png", "pdf".
// SVG is always included. PNG uses the provided scale (defaults to 2.0 if <= 0).
func (b *SVGBuilder) RenderWithFormats(formats []string, pngScale float64) (*BuilderRenderResult, error) {
	result := &BuilderRenderResult{}

	// SVG is always rendered
	svgDoc, err := b.Render()
	if err != nil {
		return nil, err
	}
	result.SVG = svgDoc

	// Check requested formats
	for _, format := range formats {
		switch format {
		case "png":
			pngBytes, err := b.RenderPNG(pngScale)
			if err != nil {
				return nil, err
			}
			result.PNG = pngBytes
		case "pdf":
			pdfBytes, err := b.RenderPDF()
			if err != nil {
				return nil, err
			}
			result.PDF = pdfBytes
		case "svg":
			// Already rendered
		default:
			return nil, fmt.Errorf("svggen: unsupported format %q", format)
		}
	}

	return result, nil
}
