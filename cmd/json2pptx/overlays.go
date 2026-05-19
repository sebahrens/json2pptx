package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// overlayThemeColors returns the theme color palette from the per-slide
// diagram context, or nil when no theme is available (e.g., headless tests).
// The theme is consulted when flipping arrow stroke colors for contrast.
func overlayThemeColors(diagCtx *GridDiagramContext) []types.ThemeColor {
	if diagCtx == nil {
		return nil
	}
	return diagCtx.ThemeColors
}

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
	themeColors []types.ThemeColor,
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
		frag, err := renderOverlay(i, ov, cellByRC, alloc, slideWidth, slideHeight, themeColors)
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
	themeColors []types.ThemeColor,
) ([]byte, error) {
	kind := strings.ToLower(strings.TrimSpace(ov.Kind))
	switch kind {
	case "arrow":
		return renderOverlayConnector(idx, ov, cellByRC, alloc, slideWidth, slideHeight, true, themeColors)
	case "line":
		return renderOverlayConnector(idx, ov, cellByRC, alloc, slideWidth, slideHeight, false, themeColors)
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
	themeColors []types.ThemeColor,
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

	// Reroute arrow endpoints around cell-center text labels. When both
	// endpoints reference grid cells with anchor_cell at "center" and those
	// cells contain text, anchoring the arrowhead on the label center buries
	// the head under the text. Snap each endpoint to the cell corner facing
	// the opposite endpoint so the arrow travels through the inter-cell gap
	// instead of across labels.
	if withArrowhead && isCenterAnchoredCell(ov.From) && isCenterAnchoredCell(ov.To) {
		fromCell, fromOK := lookupAnchorCell(ov.From, cellByRC)
		toCell, toOK := lookupAnchorCell(ov.To, cellByRC)
		if fromOK && toOK && cellHasText(fromCell) && cellHasText(toCell) {
			startX, startY = snapAnchorToCornerToward(anchorRect(fromCell), [2]int64{endX, endY}, 0.10)
			endX, endY = snapAnchorToCornerToward(anchorRect(toCell), [2]int64{startX, startY}, 0.10)
		}
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
	// Flip stroke color when it has poor contrast against either endpoint
	// cell's fill (e.g., dark stroke on a dark accent quadrant). Only applies
	// to anchor_cell endpoints with a resolvable fill — free-floating
	// percent-positioned arrows are left untouched.
	color = adjustOverlayStrokeForContrast(color, ov, cellByRC, themeColors)
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

// isCenterAnchoredCell reports whether an overlay endpoint references a grid
// cell with the "center" anchor (or its empty/alias forms). Endpoints that
// target specific edges or corners are returned as-is.
func isCenterAnchoredCell(pt *OverlayPointInput) bool {
	if pt == nil || pt.AnchorCell == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(pt.AnchorCell.At)) {
	case "", "center", "c":
		return true
	}
	return false
}

// lookupAnchorCell returns the resolved cell referenced by an overlay
// endpoint, if any.
func lookupAnchorCell(pt *OverlayPointInput, cellByRC map[[2]int]shapegrid.ResolvedCell) (shapegrid.ResolvedCell, bool) {
	if pt == nil || pt.AnchorCell == nil {
		return shapegrid.ResolvedCell{}, false
	}
	c, ok := cellByRC[[2]int{pt.AnchorCell.Row, pt.AnchorCell.Col}]
	return c, ok
}

// anchorRect returns the rectangle used to compute anchor points: the
// pre-fit CellBounds when present (so anchors hit the visual cell even when
// the shape inside has been shrunk by a fit mode), otherwise the shape Bounds.
func anchorRect(cell shapegrid.ResolvedCell) pptx.RectEmu {
	if cell.CellBounds.CX > 0 && cell.CellBounds.CY > 0 {
		return cell.CellBounds
	}
	return cell.Bounds
}

// cellHasText reports whether a resolved shape cell carries any text content
// that an overlay arrowhead could obscure.
func cellHasText(cell shapegrid.ResolvedCell) bool {
	if cell.ShapeSpec == nil {
		return false
	}
	raw := strings.TrimSpace(string(cell.ShapeSpec.Text))
	if raw == "" || raw == "null" || raw == `""` {
		return false
	}
	return true
}

// snapAnchorToCornerToward returns a point on the cell rectangle adjacent to
// the corner facing `toward`, inset by `inset` (as a fraction of the cell
// dimensions). Used to route arrow endpoints away from cell-center labels.
func snapAnchorToCornerToward(rect pptx.RectEmu, toward [2]int64, inset float64) (int64, int64) {
	if inset < 0 {
		inset = 0
	}
	if inset > 0.45 {
		inset = 0.45
	}
	cx := rect.X + rect.CX/2
	cy := rect.Y + rect.CY/2
	insetX := int64(float64(rect.CX) * inset)
	insetY := int64(float64(rect.CY) * inset)

	var x, y int64
	switch {
	case toward[0] > cx:
		x = rect.X + rect.CX - insetX
	case toward[0] < cx:
		x = rect.X + insetX
	default:
		x = cx
	}
	switch {
	case toward[1] > cy:
		y = rect.Y + rect.CY - insetY
	case toward[1] < cy:
		y = rect.Y + insetY
	default:
		y = cy
	}
	return x, y
}

// adjustOverlayStrokeForContrast returns a replacement stroke color when the
// requested color has poor contrast (< 3:1, WCAG AA Large) against any
// endpoint cell fill. Free-floating endpoints (no anchor_cell) are skipped
// so manually positioned arrows preserve the author's color choice.
func adjustOverlayStrokeForContrast(
	color string,
	ov *OverlayShapeInput,
	cellByRC map[[2]int]shapegrid.ResolvedCell,
	themeColors []types.ThemeColor,
) string {
	strokeHex := resolveColorRefToHex(color, themeColors)
	if strokeHex == "" {
		return color
	}
	strokeColor, err := svggen.ParseColor(strokeHex)
	if err != nil {
		return color
	}

	endpointFills := collectEndpointFillHex(ov, cellByRC, themeColors)
	if len(endpointFills) == 0 {
		return color
	}

	// Compute worst-case contrast against any endpoint fill.
	worstRatio := 21.0
	for _, fillHex := range endpointFills {
		fc, ferr := svggen.ParseColor(fillHex)
		if ferr != nil {
			continue
		}
		ratio := strokeColor.ContrastWith(fc)
		if ratio < worstRatio {
			worstRatio = ratio
		}
	}
	const minRatio = 3.0 // WCAG AA Large / non-text graphic
	if worstRatio >= minRatio {
		return color
	}

	// Pick whichever neutral (white or near-black) has the best worst-case
	// contrast across all endpoint fills.
	candidates := []string{"FFFFFF", "1A1A1A"}
	best := color
	bestWorst := worstRatio
	for _, cand := range candidates {
		cc, cerr := svggen.ParseColor("#" + cand)
		if cerr != nil {
			continue
		}
		candWorst := 21.0
		for _, fillHex := range endpointFills {
			fc, ferr := svggen.ParseColor(fillHex)
			if ferr != nil {
				continue
			}
			if r := cc.ContrastWith(fc); r < candWorst {
				candWorst = r
			}
		}
		if candWorst > bestWorst {
			bestWorst = candWorst
			best = cand
		}
	}
	return best
}

// collectEndpointFillHex returns the resolved fill hex for each endpoint that
// references a grid cell with a shape fill.
func collectEndpointFillHex(
	ov *OverlayShapeInput,
	cellByRC map[[2]int]shapegrid.ResolvedCell,
	themeColors []types.ThemeColor,
) []string {
	var out []string
	for _, pt := range []*OverlayPointInput{ov.From, ov.To} {
		cell, ok := lookupAnchorCell(pt, cellByRC)
		if !ok || cell.ShapeSpec == nil {
			continue
		}
		if hex := extractFillHex(cell.ShapeSpec.Fill, themeColors); hex != "" {
			out = append(out, hex)
		}
	}
	return out
}

// extractFillHex pulls a hex color from a ShapeSpec.Fill JSON blob. Supports
// both string ("accent1", "#FF0000") and object ({"color": "..."}) forms.
// Returns empty when the fill is missing, "none", or unparseable.
func extractFillHex(raw json.RawMessage, themeColors []types.ThemeColor) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return resolveColorRefToHex(s, themeColors)
	}
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return resolveColorRefToHex(obj.Color, themeColors)
	}
	return ""
}

// resolveColorRefToHex normalizes a color reference (scheme name, "#RRGGBB",
// or bare "RRGGBB") to a "#RRGGBB" string. Returns empty for "none" / empty.
func resolveColorRefToHex(s string, themeColors []types.ThemeColor) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "none") {
		return ""
	}
	if strings.HasPrefix(s, "#") {
		return s
	}
	// Scheme color lookup (accent1..accent6, dk1, lt1, ...).
	schemeAliases := map[string]string{
		"tx1": "dk1", "tx2": "dk2",
		"bg1": "lt1", "bg2": "lt2",
	}
	name := s
	if alias, ok := schemeAliases[name]; ok {
		name = alias
	}
	for _, tc := range themeColors {
		if tc.Name == name {
			hex := tc.RGB
			if !strings.HasPrefix(hex, "#") {
				hex = "#" + hex
			}
			return hex
		}
	}
	// Bare hex without '#'.
	if len(s) == 6 || len(s) == 8 {
		return "#" + s
	}
	return ""
}
