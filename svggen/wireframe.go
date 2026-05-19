// Package svggen — wireframe.go renders annotated wireframe diagrams of a
// resolved slide plan. Unlike full chart/diagram renderers, the wireframe
// emits structural rectangles (slide frame, placeholders, grid cells) with
// labels and fit-finding markers, so agents can preview slide geometry
// without depending on LibreOffice. Both SVG bytes and PNG bytes are
// produced in-process via SVGBuilder + the canvas rasterizer.
package svggen

import (
	"fmt"
	"math"
	"sort"
)

// WireframeRect is a rectangle in the same coordinate space as
// WireframeRequest.SlideWidth / SlideHeight (typically EMUs, but any
// consistent unit works — the renderer scales to the target canvas).
type WireframeRect struct {
	X, Y, W, H float64
}

// WireframeCell is one resolved grid cell. Coordinates are in the same
// unit as WireframeRequest.SlideWidth / SlideHeight.
type WireframeCell struct {
	Row  int
	Col  int
	Rect WireframeRect
	Kind string // e.g. "shape", "table", "chart", "diagram", "text"
}

// WireframePlaceholder describes one placeholder rectangle from a layout.
// ID is the placeholder's resolved id (e.g. "title", "body").
type WireframePlaceholder struct {
	ID       string
	Rect     WireframeRect
	Remapped bool // true if the input_id differed from resolved_id
}

// WireframeFinding is one fit finding to overlay on the wireframe. When
// HasCell is true, the finding is attached to the (Row, Col) grid cell so
// the renderer paints a marker badge in that cell; otherwise the finding
// is placed in a side strip below the slide.
type WireframeFinding struct {
	Code     string // e.g. "table_overflow_height"
	Action   string // "info" | "review" | "shrink_or_split" | "refuse"
	Message  string // human-readable summary (truncated)
	HasCell  bool
	Row, Col int
}

// WireframeOccupancy is an optional density indicator surfaced in the
// header strip. FilledPct is 0..100.
type WireframeOccupancy struct {
	FilledPct   float64
	FilledSlots int
	TotalSlots  int
}

// WireframeRequest is the input to RenderWireframe.
//
// SlideWidth and SlideHeight establish the input coordinate space; the
// renderer scales every rectangle into a fixed-width pixel canvas with the
// same aspect ratio. OutputWidthPx is the desired canvas width in points
// (1pt ≈ 1px at the canvas library's default DPI). When zero, defaults to
// 960.
type WireframeRequest struct {
	Title         string
	SlideIndex    int
	LayoutID      string
	LayoutName    string
	SlideType     string
	TemplateName  string
	SlideWidth    float64
	SlideHeight   float64
	OutputWidthPx float64

	Cells        []WireframeCell
	Placeholders []WireframePlaceholder
	Findings     []WireframeFinding
	Occupancy    *WireframeOccupancy
}

// WireframeOutput is the result of RenderWireframe.
type WireframeOutput struct {
	SVG    []byte
	PNG    []byte
	Width  int // canvas width in pixels (rounded)
	Height int // canvas height in pixels (rounded)
}

// RenderWireframeOptions controls which output formats are produced.
type RenderWireframeOptions struct {
	IncludeSVG bool
	IncludePNG bool
	PNGScale   float64 // multiplier passed to SVGBuilder.RenderPNG; defaults to 1.0
}

// RenderWireframe builds an annotated SVG + optional PNG showing the
// slide's structural geometry: placeholders, grid cells, and fit findings.
// Coordinates inside the request are in the same unit as SlideWidth /
// SlideHeight (typically EMUs).
//
// Returns a non-nil WireframeOutput on success. SVG/PNG bytes are
// populated according to opts. Note that PNG rasterization is serialized
// internally by the canvas library (see SVGBuilder.RenderPNG); callers
// that only need vector output should pass IncludeSVG=true and
// IncludePNG=false for the cheapest path.
func RenderWireframe(req *WireframeRequest, opts RenderWireframeOptions) (*WireframeOutput, error) {
	if req == nil {
		return nil, fmt.Errorf("wireframe: nil request")
	}
	if req.SlideWidth <= 0 || req.SlideHeight <= 0 {
		return nil, fmt.Errorf("wireframe: invalid slide dimensions %v x %v", req.SlideWidth, req.SlideHeight)
	}

	// Default canvas dimensions. The header strip adds a fixed vertical
	// band above the slide; the findings strip adds another at the bottom
	// when there are off-cell findings.
	canvasW := req.OutputWidthPx
	if canvasW <= 0 {
		canvasW = 960
	}
	// Reserve a header strip and an optional findings strip.
	const headerH = 36.0
	footerLines := countOffCellFindings(req.Findings)
	footerH := 0.0
	if footerLines > 0 {
		footerH = 18.0 + float64(footerLines)*14.0
	}

	// Slide rectangle scaled to canvas width with preserved aspect ratio.
	margin := 12.0
	slideW := canvasW - 2*margin
	aspect := req.SlideHeight / req.SlideWidth
	slideH := slideW * aspect
	canvasH := headerH + slideH + footerH + 2*margin

	b := NewSVGBuilder(canvasW, canvasH)

	bg := Color{R: 0xfa, G: 0xfb, B: 0xfc, A: 1.0}
	border := Color{R: 0x6b, G: 0x72, B: 0x80, A: 1.0}
	frameBorder := Color{R: 0x37, G: 0x41, B: 0x51, A: 1.0}
	labelDark := Color{R: 0x1f, G: 0x29, B: 0x37, A: 1.0}
	labelMid := Color{R: 0x4b, G: 0x55, B: 0x63, A: 1.0}
	cellFill := Color{R: 0xe5, G: 0xe7, B: 0xeb, A: 1.0}
	cellBorder := Color{R: 0x9c, G: 0xa3, B: 0xaf, A: 1.0}
	placeholderColor := Color{R: 0x2f, G: 0x6f, B: 0xed, A: 1.0}

	// Canvas background.
	b.SetFillColor(bg).FillRect(Rect{X: 0, Y: 0, W: canvasW, H: canvasH})

	// Header strip.
	headerY := margin
	b.SetFillColor(Color{R: 0xff, G: 0xff, B: 0xff, A: 1.0}).
		FillRect(Rect{X: margin, Y: headerY, W: slideW, H: headerH - 4})
	b.SetStrokeColor(border).SetStrokeWidth(0.75).
		StrokeRect(Rect{X: margin, Y: headerY, W: slideW, H: headerH - 4})

	headerText := wireframeHeaderText(req)
	b.SetFontSize(11).SetFontWeight(600).SetTextColor(labelDark).
		DrawText(headerText, margin+8, headerY+(headerH-4)/2, TextAlignLeft, TextBaselineMiddle)

	if req.Occupancy != nil {
		b.SetFontSize(10).SetFontWeight(400).SetTextColor(labelMid).
			DrawText(formatOccupancy(req.Occupancy), margin+slideW-8, headerY+(headerH-4)/2,
				TextAlignRight, TextBaselineMiddle)
	}

	// Slide frame.
	slideOriginY := headerY + headerH
	frame := Rect{X: margin, Y: slideOriginY, W: slideW, H: slideH}
	b.SetFillColor(Color{R: 0xff, G: 0xff, B: 0xff, A: 1.0}).FillRect(frame)
	b.SetStrokeColor(frameBorder).SetStrokeWidth(1.25).StrokeRect(frame)

	// Scale factor from input units (e.g. EMUs) to canvas points.
	sx := slideW / req.SlideWidth
	sy := slideH / req.SlideHeight

	// Draw placeholder boxes first so cells render above them when both
	// exist on the same slide.
	for _, ph := range req.Placeholders {
		r := scaleRect(ph.Rect, sx, sy, margin, slideOriginY)
		drawPlaceholder(b, r, ph, placeholderColor, labelMid)
	}

	// Draw cells in row-major order. We sort to produce deterministic
	// SVG output regardless of caller-side order.
	cells := append([]WireframeCell(nil), req.Cells...)
	sort.SliceStable(cells, func(i, j int) bool {
		if cells[i].Row != cells[j].Row {
			return cells[i].Row < cells[j].Row
		}
		return cells[i].Col < cells[j].Col
	})

	// Pre-index findings by (row, col) for cell-attached markers.
	cellFindings := map[[2]int][]WireframeFinding{}
	for _, f := range req.Findings {
		if !f.HasCell {
			continue
		}
		k := [2]int{f.Row, f.Col}
		cellFindings[k] = append(cellFindings[k], f)
	}

	for _, c := range cells {
		r := scaleRect(c.Rect, sx, sy, margin, slideOriginY)
		drawCell(b, r, c, cellFill, cellBorder, labelDark, labelMid)
		if marks, ok := cellFindings[[2]int{c.Row, c.Col}]; ok {
			drawCellFindingBadges(b, r, marks)
		}
	}

	// Findings strip (off-cell findings).
	if footerLines > 0 {
		footerY := slideOriginY + slideH + 4
		b.SetFontSize(10).SetFontWeight(600).SetTextColor(labelDark).
			DrawText("Findings", margin, footerY+12, TextAlignLeft, TextBaselineMiddle)
		y := footerY + 28
		for _, f := range req.Findings {
			if f.HasCell {
				continue
			}
			drawFindingLine(b, margin, y, canvasW-2*margin, f, labelDark, labelMid)
			y += 14
		}
	}

	// Build outputs.
	out := &WireframeOutput{
		Width:  int(math.Round(canvasW * mmToPxFactor * ptToMM)),
		Height: int(math.Round(canvasH * mmToPxFactor * ptToMM)),
	}
	if opts.IncludeSVG {
		svgBytes, err := b.RenderToBytes()
		if err != nil {
			return nil, fmt.Errorf("wireframe: render svg: %w", err)
		}
		out.SVG = svgBytes
	}
	if opts.IncludePNG {
		scale := opts.PNGScale
		if scale <= 0 {
			scale = 1.0
		}
		pngBytes, err := b.RenderPNG(scale)
		if err != nil {
			return nil, fmt.Errorf("wireframe: render png: %w", err)
		}
		out.PNG = pngBytes
	}
	return out, nil
}

// wireframeHeaderText composes the header strip text.
func wireframeHeaderText(req *WireframeRequest) string {
	parts := []string{fmt.Sprintf("Slide %d", req.SlideIndex+1)}
	if req.LayoutName != "" {
		parts = append(parts, "layout: "+req.LayoutName)
	} else if req.LayoutID != "" {
		parts = append(parts, "layout: "+req.LayoutID)
	}
	if req.SlideType != "" {
		parts = append(parts, "type: "+req.SlideType)
	}
	if req.TemplateName != "" {
		parts = append(parts, "template: "+req.TemplateName)
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "  ·  " + p
	}
	if req.Title != "" {
		out += "  —  " + req.Title
	}
	return out
}

func formatOccupancy(o *WireframeOccupancy) string {
	if o.TotalSlots > 0 {
		return fmt.Sprintf("fill: %.0f%% (%d/%d slots)", o.FilledPct, o.FilledSlots, o.TotalSlots)
	}
	return fmt.Sprintf("fill: %.0f%%", o.FilledPct)
}

// scaleRect converts an input-space rectangle into canvas coordinates,
// offset by the slide frame origin.
func scaleRect(r WireframeRect, sx, sy, originX, originY float64) Rect {
	return Rect{
		X: originX + r.X*sx,
		Y: originY + r.Y*sy,
		W: r.W * sx,
		H: r.H * sy,
	}
}

// drawPlaceholder renders one placeholder rectangle with a dashed outline
// and ID label in the corner.
func drawPlaceholder(b *SVGBuilder, r Rect, ph WireframePlaceholder, color, labelColor Color) {
	b.Push()
	b.SetStrokeColor(color).SetStrokeWidth(0.75).SetDashes(4, 3).StrokeRect(r)
	b.Pop()
	if r.W < 40 || r.H < 16 {
		return
	}
	label := "PH:" + ph.ID
	if ph.Remapped {
		label += " (remapped)"
	}
	b.SetFontSize(8).SetFontWeight(500).SetTextColor(labelColor).
		DrawText(label, r.X+3, r.Y+9, TextAlignLeft, TextBaselineMiddle)
}

// drawCell renders one grid cell with its row,col, kind, and dimension
// labels positioned to avoid overflow.
func drawCell(b *SVGBuilder, r Rect, c WireframeCell, fill, border, labelDark, labelMid Color) {
	b.SetFillColor(fill).FillRect(r)
	b.SetStrokeColor(border).SetStrokeWidth(0.75).StrokeRect(r)

	// Top-left: "r,c" tag.
	tag := fmt.Sprintf("r%d,c%d", c.Row, c.Col)
	b.SetFontSize(9).SetFontWeight(700).SetTextColor(labelDark)
	if r.W >= 30 && r.H >= 14 {
		b.DrawText(tag, r.X+4, r.Y+8, TextAlignLeft, TextBaselineMiddle)
	}

	// Centered: kind.
	if c.Kind != "" && r.W >= 50 && r.H >= 26 {
		b.SetFontSize(11).SetFontWeight(600).SetTextColor(labelDark).
			DrawText(c.Kind, r.X+r.W/2, r.Y+r.H/2, TextAlignCenter, TextBaselineMiddle)
	}

	// Bottom-right: dimensions in inches.
	if r.W >= 60 && r.H >= 30 {
		dim := fmt.Sprintf("%.1f×%.1f in", emuToInch(c.Rect.W), emuToInch(c.Rect.H))
		b.SetFontSize(8).SetFontWeight(400).SetTextColor(labelMid).
			DrawText(dim, r.X+r.W-4, r.Y+r.H-4, TextAlignRight, TextBaselineAlphabetic)
	}
}

// drawCellFindingBadges paints small severity-coded badges at the top-right
// corner of a cell, one per attached finding, with the action code.
func drawCellFindingBadges(b *SVGBuilder, r Rect, marks []WireframeFinding) {
	x := r.X + r.W - 6
	y := r.Y + 4
	for _, m := range marks {
		bw := 22.0
		bh := 12.0
		bx := x - bw
		by := y
		b.SetFillColor(severityColor(m.Action))
		b.SetStrokeColor(Color{R: 0, G: 0, B: 0, A: 1.0}).SetStrokeWidth(0.25)
		b.DrawRoundedRect(Rect{X: bx, Y: by, W: bw, H: bh}, 2)
		b.SetFontSize(8).SetFontWeight(700).
			SetTextColor(Color{R: 0xff, G: 0xff, B: 0xff, A: 1.0}).
			DrawText(shortAction(m.Action), bx+bw/2, by+bh/2, TextAlignCenter, TextBaselineMiddle)
		y += bh + 2
	}
}

func drawFindingLine(b *SVGBuilder, x, y, maxW float64, f WireframeFinding, labelDark, labelMid Color) {
	// Severity dot.
	dotR := 4.0
	b.SetFillColor(severityColor(f.Action))
	b.DrawCircle(x+dotR, y, dotR)
	// Code + message, truncated to fit.
	label := f.Code
	if f.Message != "" {
		label += ": " + f.Message
	}
	b.SetFontSize(9).SetFontWeight(400).SetTextColor(labelDark)
	avail := maxW - dotR*2 - 6
	if avail < 50 {
		avail = 50
	}
	label = b.TruncateToWidth(label, avail)
	b.DrawText(label, x+dotR*2+4, y, TextAlignLeft, TextBaselineMiddle)
	_ = labelMid
}

func countOffCellFindings(findings []WireframeFinding) int {
	n := 0
	for _, f := range findings {
		if !f.HasCell {
			n++
		}
	}
	return n
}

func severityColor(action string) Color {
	switch action {
	case "refuse":
		return Color{R: 0xb9, G: 0x1c, B: 0x1c, A: 1.0} // red-700
	case "shrink_or_split":
		return Color{R: 0xc2, G: 0x41, B: 0x0c, A: 1.0} // orange-700
	case "review":
		return Color{R: 0xa1, G: 0x6b, B: 0x07, A: 1.0} // amber-700
	case "info":
		return Color{R: 0x37, G: 0x5e, B: 0xab, A: 1.0} // blue-700
	default:
		return Color{R: 0x4b, G: 0x55, B: 0x63, A: 1.0} // gray-600
	}
}

func shortAction(action string) string {
	switch action {
	case "refuse":
		return "REF"
	case "shrink_or_split":
		return "SHR"
	case "review":
		return "REV"
	case "info":
		return "INF"
	default:
		return "FND"
	}
}

// emuToInch converts English Metric Units to inches (914400 EMU per inch).
// When the input is already in points or another unit, the value is still
// shown — it just won't read as inches. Callers in the EMU pipeline get
// useful labels; others get a numeric scale that's still consistent.
func emuToInch(v float64) float64 {
	return v / 914400.0
}
