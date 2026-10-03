package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textfit"
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

// overlayEnvFor builds the overlay context for slide slideIdx.
func overlayEnvFor(slideIdx int, diagCtx *GridDiagramContext) overlayEnv {
	env := overlayEnv{SlideIdx: slideIdx, ThemeColors: overlayThemeColors(diagCtx)}
	if diagCtx != nil {
		env.FontFamily = diagCtx.FontFamily
	}
	return env
}

// overlayEnv carries the per-slide context overlays resolve against.
type overlayEnv struct {
	SlideIdx    int                // 0-based slide index, for finding paths
	ThemeColors []types.ThemeColor // stroke-contrast flips and callout label ink
	FontFamily  string             // theme body face, for sizing callout labels
}

// overlayCtx is the state shared by one slide's overlay renderers.
type overlayCtx struct {
	env                     overlayEnv
	cells                   []shapegrid.ResolvedCell
	cellByRC                map[[2]int]shapegrid.ResolvedCell
	slideWidth, slideHeight int64
	findings                []patterns.FitFinding
	// calloutTargets are the resolved targets of every callout on the slide
	// and calloutLabels the labels placed so far: an auto-placed label keeps
	// clear of both.
	calloutTargets [][2]int64
	calloutLabels  []pptx.RectEmu
}

// resolveOverlays converts the slide-level Overlays DTO list into raw <p:sp> /
// <p:cxnSp> XML fragments. Cells (if present) are indexed by (row, col) so
// anchor_cell and anchor_image references can resolve to absolute EMU
// coordinates. Findings report image targets the picture's crop trims away.
//
// Returns an error if a referenced anchor cell does not exist or if a
// required field is missing for the given overlay kind.
func resolveOverlays(
	overlays []*OverlayShapeInput,
	cells []shapegrid.ResolvedCell,
	alloc *pptx.ShapeIDAllocator,
	slideWidth, slideHeight int64,
	env overlayEnv,
) ([][]byte, []patterns.FitFinding, error) {
	if len(overlays) == 0 {
		return nil, nil, nil
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

	ctx := &overlayCtx{env: env, cells: cells, cellByRC: cellByRC, slideWidth: slideWidth, slideHeight: slideHeight}
	for _, ov := range overlays {
		if ov == nil || ov.To == nil || !strings.EqualFold(strings.TrimSpace(ov.Kind), "callout") {
			continue
		}
		if target, err := ctx.resolveOverlayPoint(ov.To); err == nil {
			ctx.calloutTargets = append(ctx.calloutTargets, [2]int64{target.X, target.Y})
		}
	}
	out := make([][]byte, 0, len(overlays))
	for i, ov := range overlays {
		if ov == nil {
			continue
		}
		frags, err := ctx.renderOverlay(i, ov, alloc)
		if err != nil {
			return nil, nil, fmt.Errorf("overlay %d: %w", i, err)
		}
		out = append(out, frags...)
	}
	return out, ctx.findings, nil
}

// renderOverlay dispatches to the per-kind renderer.
func (ctx *overlayCtx) renderOverlay(idx int, ov *OverlayShapeInput, alloc *pptx.ShapeIDAllocator) ([][]byte, error) {
	kind := strings.ToLower(strings.TrimSpace(ov.Kind))
	if ov.Link != nil && kind != "badge" {
		return nil, fmt.Errorf("link is supported only on badge overlays")
	}
	var frag []byte
	var err error
	switch kind {
	case "arrow":
		frag, err = ctx.renderOverlayConnector(idx, ov, alloc, true)
	case "line":
		frag, err = ctx.renderOverlayConnector(idx, ov, alloc, false)
	case "badge":
		frag, err = ctx.renderOverlayBadge(idx, ov, alloc)
	case "callout":
		return ctx.renderOverlayCallout(idx, ov, alloc)
	case "":
		return nil, fmt.Errorf("kind is required (arrow, line, badge, or callout)")
	default:
		return nil, fmt.Errorf("unsupported kind %q (expected arrow, line, badge, or callout)", ov.Kind)
	}
	if err != nil || frag == nil {
		return nil, err
	}
	return [][]byte{frag}, nil
}

// renderOverlayConnector emits a p:cxnSp for a line or arrow overlay. An
// endpoint on an image point the crop trims away omits the connector (and
// reports OVERLAY_TARGET_CROPPED) rather than pointing at other pixels.
func (ctx *overlayCtx) renderOverlayConnector(
	idx int,
	ov *OverlayShapeInput,
	alloc *pptx.ShapeIDAllocator,
	withArrowhead bool,
) ([]byte, error) {
	if ov.From == nil {
		return nil, fmt.Errorf("%s overlay requires 'from'", ov.Kind)
	}
	if ov.To == nil {
		return nil, fmt.Errorf("%s overlay requires 'to'", ov.Kind)
	}

	start, err := ctx.resolveOverlayPoint(ov.From)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	end, err := ctx.resolveOverlayPoint(ov.To)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	omitted := fmt.Sprintf("the %s was omitted", strings.ToLower(ov.Kind))
	if ctx.reportCropped(idx, "from", start, omitted) || ctx.reportCropped(idx, "to", end, omitted) {
		return nil, nil
	}
	startX, startY, endX, endY := start.X, start.Y, end.X, end.Y

	// Reroute arrow endpoints around cell-center text labels. When both
	// endpoints reference grid cells with anchor_cell at "center" and those
	// cells contain text, anchoring the arrowhead on the label center buries
	// the head under the text. Snap each endpoint to the cell corner facing
	// the opposite endpoint so the arrow travels through the inter-cell gap
	// instead of across labels.
	if withArrowhead && isCenterAnchoredCell(ov.From) && isCenterAnchoredCell(ov.To) {
		fromCell, fromOK := lookupAnchorCell(ov.From, ctx.cellByRC)
		toCell, toOK := lookupAnchorCell(ov.To, ctx.cellByRC)
		if fromOK && toOK && cellHasText(fromCell) && cellHasText(toCell) {
			startX, startY = snapAnchorToCornerToward(anchorRect(fromCell), [2]int64{endX, endY}, 0.10)
			endX, endY = snapAnchorToCornerToward(anchorRect(toCell), [2]int64{startX, startY}, 0.10)
		}
	}

	color := strings.TrimSpace(ov.Color)
	if color == "" {
		color = "000000"
	}
	// Flip stroke color when it has poor contrast against either endpoint
	// cell's fill (e.g., dark stroke on a dark accent quadrant). Only applies
	// to anchor_cell endpoints with a resolvable fill — free-floating
	// percent-positioned arrows are left untouched.
	color = adjustOverlayStrokeForContrast(color, ov, ctx.cellByRC, ctx.env.ThemeColors)
	var head *pptx.ArrowHead
	if withArrowhead {
		head = &pptx.ArrowHead{Type: "triangle", W: "med", Len: "med"}
	}
	return overlayConnectorXML(alloc.Alloc(), fmt.Sprintf("Overlay %s %d", strings.ToLower(ov.Kind), idx+1),
		startX, startY, endX, endY, ov.Width, color, ov.Dash, head)
}

// overlayConnectorXML emits a straight connector drawn from (startX,startY)
// to (endX,endY), with an optional head at the end point.
func overlayConnectorXML(id uint32, name string, startX, startY, endX, endY int64, widthPt float64, color, dash string, head *pptx.ArrowHead) ([]byte, error) {
	// Compute connector bounds and required flip flags so that the line is
	// drawn from (startX,startY) to (endX,endY) regardless of direction.
	minX, w := spanEMU(startX, endX)
	minY, h := spanEMU(startY, endY)
	// straightConnector1 draws from top-left of bounds to bottom-right by default.
	// If the actual start is on the right or bottom, flip the connector so the
	// arrowhead lands at the user-specified `to` endpoint.
	flipH := endX < startX
	flipV := endY < startY

	if widthPt <= 0 {
		widthPt = 1.5
	}
	line := pptx.ResolveColorLinePoints(widthPt, color)
	if d := strings.TrimSpace(dash); d != "" {
		line.Dash = d
	}

	return pptx.GenerateConnector(pptx.ConnectorOptions{
		ID:       id,
		Name:     name,
		Geometry: pptx.GeomStraightConnector1,
		Bounds:   pptx.RectEmu{X: minX, Y: minY, CX: w, CY: h},
		Line:     line,
		FlipH:    flipH,
		FlipV:    flipV,
		TailEnd:  head,
	})
}

// spanEMU returns the lower of a and b and the (at least 1 EMU) distance
// between them.
func spanEMU(a, b int64) (lo, extent int64) {
	lo, hi := a, b
	if b < a {
		lo, hi = b, a
	}
	if hi-lo == 0 {
		return lo, 1
	}
	return lo, hi - lo
}

// renderOverlayBadge emits a p:sp roundRect with centered text.
func (ctx *overlayCtx) renderOverlayBadge(idx int, ov *OverlayShapeInput, alloc *pptx.ShapeIDAllocator) ([]byte, error) {
	if ov.From == nil {
		return nil, fmt.Errorf("badge overlay requires 'from'")
	}
	from, err := ctx.resolveOverlayPoint(ov.From)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	const clamped = "the badge was placed at the nearest visible picture edge"
	ctx.reportCropped(idx, "from", from, clamped)
	x0, y0 := from.X, from.Y

	var x1, y1 int64
	if ov.To != nil {
		to, err := ctx.resolveOverlayPoint(ov.To)
		if err != nil {
			return nil, fmt.Errorf("to: %w", err)
		}
		ctx.reportCropped(idx, "to", to, clamped)
		x1, y1 = to.X, to.Y
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
		x1 = x0 + int64(float64(ctx.slideWidth)*wPct/100.0)
		y1 = y0 + int64(float64(ctx.slideHeight)*hPct/100.0)
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
	if ov.Link != nil {
		opts.HyperlinkRelID = fmt.Sprintf("json2pptx_overlay_link_%d", idx)
		if ov.Link.Slide > 0 {
			opts.HyperlinkAction = "ppaction://hlinksldjump"
		}
	}
	if t := strings.TrimSpace(ov.Text); t != "" {
		opts.Text = &pptx.TextBody{
			Wrap:      "square",
			Anchor:    "ctr",
			AnchorCtr: true,
			Insets:    pptx.ShapeTextInsets(),
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

// Callout label metrics: 12pt bold native text (the body readability floor
// for an annotation), padded 0.1" left/right and 0.05" top/bottom.
const (
	calloutFontPt      = 12.0
	calloutInsetXEMU   = int64(91440)
	calloutInsetYEMU   = int64(45720)
	calloutMaxWidthPct = 35.0
)

// renderOverlayCallout emits a native label box at `from` (its top-left)
// and a leader from the box edge facing the target to `to`, ending in a dot
// on the target. The leader starts on the label's boundary, so it never
// crosses its own text. `to` is usually an anchor_image point, which keeps
// the leader on the same source pixel whatever the frame or crop. A target
// the crop trims away keeps the label (and its wording) and omits the leader.
// With `from` omitted and an anchor_image target the label is placed beside
// its target, inside the picture's frame (autoCalloutLabel).
func (ctx *overlayCtx) renderOverlayCallout(idx int, ov *OverlayShapeInput, alloc *pptx.ShapeIDAllocator) ([][]byte, error) {
	text := strings.TrimSpace(ov.Text)
	if text == "" {
		return nil, fmt.Errorf("callout overlay requires 'text'")
	}
	if ov.To == nil {
		return nil, fmt.Errorf("callout overlay requires 'to' (the target point)")
	}
	if ov.From == nil && ov.To.AnchorImage == nil {
		return nil, fmt.Errorf("callout overlay requires 'from' (the label's top-left); only an anchor_image target places its own label")
	}
	target, err := ctx.resolveOverlayPoint(ov.To)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}

	var label pptx.RectEmu
	if ov.From != nil {
		from, ferr := ctx.resolveOverlayPoint(ov.From)
		if ferr != nil {
			return nil, fmt.Errorf("from: %w", ferr)
		}
		ctx.reportCropped(idx, "from", from, "the label was placed at the nearest visible picture edge")
		label = ctx.calloutLabelRect(from.X, from.Y, text, ov.Width, ov.Height)
	} else {
		label = ctx.autoCalloutLabel(ov.To.AnchorImage, target, ctx.calloutLabelRect(0, 0, text, ov.Width, ov.Height))
	}
	ctx.calloutLabels = append(ctx.calloutLabels, label)
	color := strings.TrimSpace(ov.Color)
	if color == "" {
		color = "accent1"
	}

	var paras []pptx.Paragraph
	for _, line := range strings.Split(text, "\n") {
		paras = append(paras, pptx.Paragraph{
			Align: "ctr",
			Runs: []pptx.Run{{
				Text:     line,
				FontSize: int(calloutFontPt * 100),
				Bold:     true,
				Color:    pptx.SolidFill(calloutInk(color, ctx.env.ThemeColors)),
			}},
		})
	}
	labelXML, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       alloc.Alloc(),
		Name:     fmt.Sprintf("Overlay callout %d", idx+1),
		Geometry: pptx.GeomRoundRect,
		Bounds:   label,
		Fill:     pptx.ResolveColorString(color),
		Line:     pptx.NoLine(),
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			AnchorCtr:  true,
			Insets:     [4]int64{calloutInsetXEMU, calloutInsetYEMU, calloutInsetXEMU, calloutInsetYEMU},
			Paragraphs: paras,
		},
	})
	if err != nil {
		return nil, err
	}

	if ctx.reportCropped(idx, "to", target, "the leader was omitted; the label is kept") {
		return [][]byte{labelXML}, nil
	}
	if pointInRect(target.X, target.Y, label) {
		return nil, fmt.Errorf("callout target lies inside its own label; move 'from' so the label sits beside the target")
	}
	sx, sy := rectEdgeToward(label, target.X, target.Y)
	leaderXML, err := overlayConnectorXML(alloc.Alloc(), fmt.Sprintf("Overlay callout leader %d", idx+1),
		sx, sy, target.X, target.Y, 0, color, ov.Dash, &pptx.ArrowHead{Type: "oval", W: "med", Len: "med"})
	if err != nil {
		return nil, err
	}
	return [][]byte{labelXML, leaderXML}, nil
}

// calloutLeaderEMU is the gap an auto-placed label keeps from its target: long
// enough for the leader and its dot to read, short enough that the label
// plainly belongs to the point (0.3in).
const calloutLeaderEMU = int64(274320)

// autoCalloutLabel places a callout label of the given size beside its
// anchor_image target (go-slide-creator-n3j96). It tries the eight positions
// around the target, starting on the side facing the middle of the picture's
// frame, and takes the first that stays inside the frame and covers neither
// another callout's target nor a label already placed. When none does, the
// first position is used, kept on the slide.
func (ctx *overlayCtx) autoCalloutLabel(ai *OverlayAnchorImageInput, target resolvedPoint, size pptx.RectEmu) pptx.RectEmu {
	frame := pptx.RectEmu{CX: ctx.slideWidth, CY: ctx.slideHeight}
	if cell, ok := ctx.imageCell(ai); ok && cell.Bounds.CX >= size.CX && cell.Bounds.CY >= size.CY {
		frame = cell.Bounds
	}
	sx, sy := int64(1), int64(1)
	if target.X > frame.X+frame.CX/2 {
		sx = -1
	}
	if target.Y > frame.Y+frame.CY/2 {
		sy = -1
	}
	var fallback *pptx.RectEmu
	for _, dir := range [][2]int64{{sx, sy}, {sx, 0}, {sx, -sy}, {0, sy}, {0, -sy}, {-sx, sy}, {-sx, 0}, {-sx, -sy}} {
		r := pptx.RectEmu{X: target.X - size.CX/2, Y: target.Y - size.CY/2, CX: size.CX, CY: size.CY}
		switch dir[0] {
		case 1:
			r.X = target.X + calloutLeaderEMU
		case -1:
			r.X = target.X - calloutLeaderEMU - size.CX
		}
		switch dir[1] {
		case 1:
			r.Y = target.Y + calloutLeaderEMU
		case -1:
			r.Y = target.Y - calloutLeaderEMU - size.CY
		}
		r.X = clampEMU(r.X, frame.X, frame.X+frame.CX-size.CX)
		r.Y = clampEMU(r.Y, frame.Y, frame.Y+frame.CY-size.CY)
		if fallback == nil {
			first := r
			fallback = &first
		}
		if ctx.calloutLabelClear(r) {
			return r
		}
	}
	return *fallback
}

// clampEMU keeps v inside [lo, hi]; lo wins when the range is empty.
func clampEMU(v, lo, hi int64) int64 {
	if v > hi {
		v = hi
	}
	if v < lo {
		v = lo
	}
	return v
}

// calloutLabelClear reports whether a label rectangle covers no callout
// target (its own included: the leader needs room) and no placed label.
func (ctx *overlayCtx) calloutLabelClear(r pptx.RectEmu) bool {
	const pad = int64(45720)
	grown := pptx.RectEmu{X: r.X - pad, Y: r.Y - pad, CX: r.CX + 2*pad, CY: r.CY + 2*pad}
	for _, t := range ctx.calloutTargets {
		if pointInRect(t[0], t[1], grown) {
			return false
		}
	}
	for _, placed := range ctx.calloutLabels {
		if grown.X < placed.X+placed.CX && placed.X < grown.X+grown.CX && grown.Y < placed.Y+placed.CY && placed.Y < grown.Y+grown.CY {
			return false
		}
	}
	return true
}

// calloutLabelRect sizes a callout label at (x, y): width / height are
// slide percentages when given, otherwise the measured 12pt bold text plus
// insets, capped at calloutMaxWidthPct of the slide width (the text then
// wraps onto more lines).
func (ctx *overlayCtx) calloutLabelRect(x, y int64, text string, wPct, hPct float64) pptx.RectEmu {
	lines := strings.Split(text, "\n")
	maxW := int64(float64(ctx.slideWidth) * calloutMaxWidthPct / 100)
	var textW int64
	for _, line := range lines {
		lw, err := textfit.MeasureStyledLineWidth(line, ctx.env.FontFamily, calloutFontPt, true)
		if err != nil || lw <= 0 {
			lw = int64(float64(len([]rune(line))) * calloutFontPt * 0.6 * 12700)
		}
		textW = max(textW, lw)
	}
	// 6% slack: rendering engines kern and hint differently.
	w := textW + textW*6/100 + 2*calloutInsetXEMU
	nLines := len(lines)
	if w > maxW {
		nLines += int((w - 1) / maxW)
		w = maxW
	}
	h := int64(float64(nLines)*calloutFontPt*1.2*12700) + 2*calloutInsetYEMU
	if wPct > 0 {
		w = int64(float64(ctx.slideWidth) * wPct / 100)
	}
	if hPct > 0 {
		h = int64(float64(ctx.slideHeight) * hPct / 100)
	}
	return pptx.RectEmu{X: x, Y: y, CX: max(w, 1), CY: max(h, 1)}
}

// calloutInk returns white or near-black, whichever contrasts more with the
// label fill (white when the fill cannot be resolved).
func calloutInk(fill string, themeColors []types.ThemeColor) string {
	fc, err := svggen.ParseColor(resolveColorRefToHex(fill, themeColors))
	if err != nil {
		return "FFFFFF"
	}
	white, _ := svggen.ParseColor("#FFFFFF")
	dark, _ := svggen.ParseColor("#1A1A1A")
	if dark.ContrastWith(fc) > white.ContrastWith(fc) {
		return "1A1A1A"
	}
	return "FFFFFF"
}

// pointInRect reports whether (x, y) lies inside r (edges included).
func pointInRect(x, y int64, r pptx.RectEmu) bool {
	return x >= r.X && x <= r.X+r.CX && y >= r.Y && y <= r.Y+r.CY
}

// rectEdgeToward returns where the ray from r's centre to (tx, ty) leaves r.
func rectEdgeToward(r pptx.RectEmu, tx, ty int64) (int64, int64) {
	cx := float64(r.X) + float64(r.CX)/2
	cy := float64(r.Y) + float64(r.CY)/2
	dx := float64(tx) - cx
	dy := float64(ty) - cy
	s := math.Inf(1)
	if dx != 0 {
		s = math.Min(s, float64(r.CX)/2/math.Abs(dx))
	}
	if dy != 0 {
		s = math.Min(s, float64(r.CY)/2/math.Abs(dy))
	}
	if math.IsInf(s, 1) {
		return int64(cx), int64(cy)
	}
	return int64(math.Round(cx + dx*s)), int64(math.Round(cy + dy*s))
}

// resolvedPoint is an overlay endpoint in slide EMU. cropped is set when the
// point is an anchor_image target the picture's crop trims away; X/Y are
// then clamped to the nearest visible picture edge.
type resolvedPoint struct {
	X, Y    int64
	cropped *croppedTarget
}

// croppedTarget describes an off-crop anchor_image target for the finding.
type croppedTarget struct {
	row, col       int
	u, v           float64 // target as source fractions
	x0, x1, y0, y1 float64 // visible source interval
}

// reportCropped records OVERLAY_TARGET_CROPPED for an off-crop endpoint and
// reports whether it was one. consequence says what the renderer did.
func (ctx *overlayCtx) reportCropped(idx int, field string, pt resolvedPoint, consequence string) bool {
	c := pt.cropped
	if c == nil {
		return false
	}
	path := slidepath.SlideField(ctx.env.SlideIdx, fmt.Sprintf("overlays/%d/%s/anchor_image", idx, field))
	ctx.findings = append(ctx.findings, patterns.OverlayTargetCropped(path, idx, c.row, c.col,
		[2]float64{c.u, c.v}, [4]float64{c.x0, c.x1, c.y0, c.y1}, consequence))
	return true
}

// resolveOverlayPoint converts an OverlayPointInput to absolute EMU
// coordinates. AnchorImage or AnchorCell, when set, overrides X/Y.
func (ctx *overlayCtx) resolveOverlayPoint(pt *OverlayPointInput) (resolvedPoint, error) {
	if pt == nil {
		return resolvedPoint{}, fmt.Errorf("missing point")
	}
	if pt.AnchorImage != nil {
		if pt.AnchorCell != nil {
			return resolvedPoint{}, fmt.Errorf("set either anchor_cell or anchor_image, not both")
		}
		return ctx.resolveImageAnchor(pt.AnchorImage)
	}
	if pt.AnchorCell != nil {
		ac := pt.AnchorCell
		cell, ok := ctx.cellByRC[[2]int{ac.Row, ac.Col}]
		if !ok {
			return resolvedPoint{}, fmt.Errorf("anchor_cell row=%d col=%d not found in shape_grid", ac.Row, ac.Col)
		}
		x, y := pointOnRect(anchorRect(cell), ac.At)
		return resolvedPoint{X: x, Y: y}, nil
	}
	x := int64(float64(ctx.slideWidth) * clampPct(pt.X) / 100.0)
	y := int64(float64(ctx.slideHeight) * clampPct(pt.Y) / 100.0)
	return resolvedPoint{X: x, Y: y}, nil
}

// resolveImageAnchor maps a point on a shape_grid image cell's source picture
// to slide EMU through generator.GridImagePlacement — the transform that
// places the picture itself — so the endpoint follows its pixel through any
// frame aspect, fit or template change.
func (ctx *overlayCtx) resolveImageAnchor(ai *OverlayAnchorImageInput) (resolvedPoint, error) {
	if _, ok := ctx.cellByRC[[2]int{ai.Row, ai.Col}]; !ok {
		return resolvedPoint{}, fmt.Errorf("anchor_image row=%d col=%d not found in shape_grid", ai.Row, ai.Col)
	}
	cell, ok := ctx.imageCell(ai)
	if !ok {
		return resolvedPoint{}, fmt.Errorf("anchor_image row=%d col=%d is not an image cell", ai.Row, ai.Col)
	}
	frame := types.BoundingBox{X: cell.Bounds.X, Y: cell.Bounds.Y, Width: cell.Bounds.CX, Height: cell.Bounds.CY}
	place := generator.GridImagePlacement(cell.ImageSpec.Path, frame, cell.ImageSpec.Fit)

	u, v := ai.X, ai.Y
	switch strings.ToLower(strings.TrimSpace(ai.Units)) {
	case "", "fraction":
		if u < 0 || u > 1 || v < 0 || v > 1 {
			return resolvedPoint{}, fmt.Errorf("anchor_image x=%g y=%g outside the image; fraction units run 0–1 from the top-left corner", ai.X, ai.Y)
		}
	case "px":
		if place.PixelW <= 0 || place.PixelH <= 0 {
			return resolvedPoint{}, fmt.Errorf("anchor_image units \"px\" need a raster image whose pixel size can be read; use units \"fraction\"")
		}
		if u < 0 || u > float64(place.PixelW) || v < 0 || v > float64(place.PixelH) {
			return resolvedPoint{}, fmt.Errorf("anchor_image x=%g y=%g px outside the %dx%d image", ai.X, ai.Y, place.PixelW, place.PixelH)
		}
		u /= float64(place.PixelW)
		v /= float64(place.PixelH)
	default:
		return resolvedPoint{}, fmt.Errorf("anchor_image units %q invalid; use \"fraction\" (default) or \"px\"", ai.Units)
	}

	x, y, visible := place.SourceToSlide(u, v)
	pt := resolvedPoint{X: x, Y: y}
	if !visible {
		x0, x1, y0, y1 := place.VisibleSource()
		pt.cropped = &croppedTarget{row: ai.Row, col: ai.Col, u: u, v: v, x0: x0, x1: x1, y0: y0, y1: y1}
	}
	return pt, nil
}

// imageCell returns the image cell an anchor_image addresses: the cell at
// (row, col), or — when that cell hosts a nested grid — the one picture inside
// it. A pattern that stacks a caption under its picture (image-text-split)
// nests the picture one grid down, and the author addressing the picture's
// column should not have to know that.
func (ctx *overlayCtx) imageCell(ai *OverlayAnchorImageInput) (shapegrid.ResolvedCell, bool) {
	cell, ok := ctx.cellByRC[[2]int{ai.Row, ai.Col}]
	if !ok {
		return shapegrid.ResolvedCell{}, false
	}
	if cell.Kind == shapegrid.CellKindImage && cell.ImageSpec != nil {
		return cell, true
	}
	if cell.Kind != shapegrid.CellKindSubGrid {
		return shapegrid.ResolvedCell{}, false
	}
	var found []shapegrid.ResolvedCell
	host := cell.Bounds
	for _, c := range ctx.cells {
		if c.Kind != shapegrid.CellKindImage || c.ImageSpec == nil {
			continue
		}
		b := c.Bounds
		if b.X >= host.X && b.Y >= host.Y && b.X+b.CX <= host.X+host.CX && b.Y+b.CY <= host.Y+host.CY {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		return shapegrid.ResolvedCell{}, false
	}
	return found[0], true
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
