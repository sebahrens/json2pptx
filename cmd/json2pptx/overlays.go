package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// resolveOverlays converts the slide-level Overlays DTO list into raw <p:sp> /
// <p:cxnSp> XML fragments. Cells (if present) are indexed by (row, col) so
// anchor_cell references can resolve to absolute EMU coordinates.
//
// Returns an error if a referenced anchor cell does not exist or if a
// required field is missing for the given overlay kind.
func resolveOverlays(
	overlays []*OverlayShapeInput,
	cells []shapegrid.ResolvedCell,
	alloc *pptx.ShapeIDAllocator,
	slideWidth, slideHeight int64,
) ([][]byte, error) {
	if len(overlays) == 0 {
		return nil, nil
	}

	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}

	// Index resolved cells by (row, col) for anchor lookup. When a cell spans
	// multiple rows/cols we still key by its top-left RowIdx/ColIdx — agents
	// reference the anchor cell by its declared coordinates.
	cellByRC := make(map[[2]int]shapegrid.ResolvedCell, len(cells))
	for _, c := range cells {
		key := [2]int{c.RowIdx, c.ColIdx}
		if _, exists := cellByRC[key]; !exists {
			cellByRC[key] = c
		}
	}

	out := make([][]byte, 0, len(overlays))
	for i, ov := range overlays {
		if ov == nil {
			continue
		}
		frag, err := renderOverlay(i, ov, cellByRC, alloc, slideWidth, slideHeight)
		if err != nil {
			return nil, fmt.Errorf("overlay %d: %w", i, err)
		}
		if frag != nil {
			out = append(out, frag)
		}
	}
	return out, nil
}

// renderOverlay dispatches to the per-kind renderer.
func renderOverlay(
	idx int,
	ov *OverlayShapeInput,
	cellByRC map[[2]int]shapegrid.ResolvedCell,
	alloc *pptx.ShapeIDAllocator,
	slideWidth, slideHeight int64,
) ([]byte, error) {
	kind := strings.ToLower(strings.TrimSpace(ov.Kind))
	switch kind {
	case "arrow":
		return renderOverlayConnector(idx, ov, cellByRC, alloc, slideWidth, slideHeight, true)
	case "line":
		return renderOverlayConnector(idx, ov, cellByRC, alloc, slideWidth, slideHeight, false)
	case "badge":
		return renderOverlayBadge(idx, ov, cellByRC, alloc, slideWidth, slideHeight)
	case "":
		return nil, fmt.Errorf("kind is required (arrow, line, or badge)")
	default:
		return nil, fmt.Errorf("unsupported kind %q (expected arrow, line, or badge)", ov.Kind)
	}
}

// renderOverlayConnector emits a p:cxnSp for a line or arrow overlay.
func renderOverlayConnector(
	idx int,
	ov *OverlayShapeInput,
	cellByRC map[[2]int]shapegrid.ResolvedCell,
	alloc *pptx.ShapeIDAllocator,
	slideWidth, slideHeight int64,
	withArrowhead bool,
) ([]byte, error) {
	if ov.From == nil {
		return nil, fmt.Errorf("%s overlay requires 'from'", ov.Kind)
	}
	if ov.To == nil {
		return nil, fmt.Errorf("%s overlay requires 'to'", ov.Kind)
	}

	startX, startY, err := resolveOverlayPoint(ov.From, cellByRC, slideWidth, slideHeight)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	endX, endY, err := resolveOverlayPoint(ov.To, cellByRC, slideWidth, slideHeight)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}

	// Compute connector bounds and required flip flags so that the line is
	// drawn from (startX,startY) to (endX,endY) regardless of direction.
	minX := startX
	if endX < minX {
		minX = endX
	}
	minY := startY
	if endY < minY {
		minY = endY
	}
	w := endX - startX
	if w < 0 {
		w = -w
	}
	h := endY - startY
	if h < 0 {
		h = -h
	}
	if w == 0 {
		w = 1
	}
	if h == 0 {
		h = 1
	}
	// straightConnector1 draws from top-left of bounds to bottom-right by default.
	// If the actual start is on the right or bottom, flip the connector so the
	// arrowhead lands at the user-specified `to` endpoint.
	flipH := endX < startX
	flipV := endY < startY

	widthPt := ov.Width
	if widthPt <= 0 {
		widthPt = 1.5
	}
	color := strings.TrimSpace(ov.Color)
	if color == "" {
		color = "000000"
	}
	line := pptx.ResolveColorLinePoints(widthPt, color)
	if d := strings.TrimSpace(ov.Dash); d != "" {
		line.Dash = d
	}

	opts := pptx.ConnectorOptions{
		ID:       alloc.Alloc(),
		Name:     fmt.Sprintf("Overlay %s %d", strings.ToLower(ov.Kind), idx+1),
		Geometry: pptx.GeomStraightConnector1,
		Bounds:   pptx.RectEmu{X: minX, Y: minY, CX: w, CY: h},
		Line:     line,
		FlipH:    flipH,
		FlipV:    flipV,
	}
	if withArrowhead {
		opts.TailEnd = &pptx.ArrowHead{Type: "triangle", W: "med", Len: "med"}
	}
	return pptx.GenerateConnector(opts)
}

// renderOverlayBadge emits a p:sp roundRect with centered text.
func renderOverlayBadge(
	idx int,
	ov *OverlayShapeInput,
	cellByRC map[[2]int]shapegrid.ResolvedCell,
	alloc *pptx.ShapeIDAllocator,
	slideWidth, slideHeight int64,
) ([]byte, error) {
	if ov.From == nil {
		return nil, fmt.Errorf("badge overlay requires 'from'")
	}
	x0, y0, err := resolveOverlayPoint(ov.From, cellByRC, slideWidth, slideHeight)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}

	var x1, y1 int64
	if ov.To != nil {
		x1, y1, err = resolveOverlayPoint(ov.To, cellByRC, slideWidth, slideHeight)
		if err != nil {
			return nil, fmt.Errorf("to: %w", err)
		}
	} else {
		// Derive bottom-right from width/height percent-of-slide.
		wPct := ov.Width
		if wPct <= 0 {
			wPct = 12.0 // default badge width: 12% of slide width
		}
		hPct := ov.Height
		if hPct <= 0 {
			hPct = 6.0 // default badge height: 6% of slide height
		}
		x1 = x0 + int64(float64(slideWidth)*wPct/100.0)
		y1 = y0 + int64(float64(slideHeight)*hPct/100.0)
	}

	// Normalize ordering so (x0,y0) is top-left.
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	w := x1 - x0
	h := y1 - y0
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}

	color := strings.TrimSpace(ov.Color)
	if color == "" {
		color = "accent1"
	}
	fill := pptx.ResolveColorString(color)

	opts := pptx.ShapeOptions{
		ID:       alloc.Alloc(),
		Name:     fmt.Sprintf("Overlay badge %d", idx+1),
		Geometry: pptx.GeomRoundRect,
		Bounds:   pptx.RectEmu{X: x0, Y: y0, CX: w, CY: h},
		Fill:     fill,
		Line:     pptx.NoLine(),
	}
	if t := strings.TrimSpace(ov.Text); t != "" {
		opts.Text = &pptx.TextBody{
			Wrap:      "square",
			Anchor:    "ctr",
			AnchorCtr: true,
			Paragraphs: []pptx.Paragraph{{
				Align: "ctr",
				Runs: []pptx.Run{{
					Text:     t,
					FontSize: 1200, // 12pt
					Bold:     true,
					Color:    pptx.SolidFill("FFFFFF"),
				}},
			}},
		}
	}
	return pptx.GenerateShape(opts)
}

// resolveOverlayPoint converts an OverlayPointInput to absolute EMU
// coordinates. When AnchorCell is set it overrides X/Y.
func resolveOverlayPoint(
	pt *OverlayPointInput,
	cellByRC map[[2]int]shapegrid.ResolvedCell,
	slideWidth, slideHeight int64,
) (int64, int64, error) {
	if pt == nil {
		return 0, 0, fmt.Errorf("missing point")
	}
	if pt.AnchorCell != nil {
		ac := pt.AnchorCell
		cell, ok := cellByRC[[2]int{ac.Row, ac.Col}]
		if !ok {
			return 0, 0, fmt.Errorf("anchor_cell row=%d col=%d not found in shape_grid", ac.Row, ac.Col)
		}
		b := cell.Bounds
		// CellBounds is the pre-fit rectangle; use it when Bounds has been
		// shrunk by a fit mode so anchor points still hit the visual cell.
		if cell.CellBounds.CX > 0 && cell.CellBounds.CY > 0 {
			b = cell.CellBounds
		}
		x, y := pointOnRect(b, ac.At)
		return x, y, nil
	}
	x := int64(float64(slideWidth) * clampPct(pt.X) / 100.0)
	y := int64(float64(slideHeight) * clampPct(pt.Y) / 100.0)
	return x, y, nil
}

// pointOnRect returns a named anchor point on a rectangle.
func pointOnRect(r pptx.RectEmu, at string) (int64, int64) {
	cx := r.X + r.CX/2
	cy := r.Y + r.CY/2
	switch strings.ToLower(strings.TrimSpace(at)) {
	case "top-left", "tl":
		return r.X, r.Y
	case "top", "t":
		return cx, r.Y
	case "top-right", "tr":
		return r.X + r.CX, r.Y
	case "right", "r":
		return r.X + r.CX, cy
	case "bottom-right", "br":
		return r.X + r.CX, r.Y + r.CY
	case "bottom", "b":
		return cx, r.Y + r.CY
	case "bottom-left", "bl":
		return r.X, r.Y + r.CY
	case "left", "l":
		return r.X, cy
	case "", "center", "c":
		return cx, cy
	default:
		// Unknown anchor name — fall back to center rather than failing
		// silently. Callers validate kind at the schema boundary.
		return cx, cy
	}
}

// clampPct clamps a percent value into [0, 100].
func clampPct(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
