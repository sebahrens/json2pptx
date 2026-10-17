package main

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Positive in the DTO, zero after conversion to EMU: distinguish an explicit
// zero seam from the historical zero-means-default gap contract.
const composeZeroGapPt = 0.000001

func composeUsesMeasuredLayout(c *ComposeInput) bool {
	if c.GapSet && c.Gap == 0 {
		return true
	}
	if c.Gaps != nil || c.Divider || c.Banner != nil || c.Callout != nil || c.SmartCompose {
		return true
	}
	for _, s := range c.Segments {
		if composeHasNativeLeaf(s) || s.Heading != "" || s.Caption != "" || s.Size != "" {
			return true
		}
	}
	return false
}
func composeSeams(c *ComposeInput, ctx patterns.ExpandContext) []float64 {
	out := make([]float64, len(c.Segments)-1)
	for i := range out {
		out[i] = composeGapPt(c, ctx)
		if c.Gaps != nil {
			out[i] = c.Gaps[i]
		}
	}
	return out
}
func composeBandRows(c *ComposeInput, ctx patterns.ExpandContext) (before, after []jsonschema.GridRowInput) {
	if c.Banner != nil {
		before = prependBannerRow(&jsonschema.ShapeGridInput{}, c.Banner, ctx).Rows
	}
	if c.Callout != nil {
		after = appendCalloutRow(&jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), RowGap: math.Max(composeGapPt(c, ctx), composeZeroGapPt)}, c.Callout, ctx).Rows
	}
	return before, after
}
func composeContentContext(c *ComposeInput, ctx patterns.ExpandContext) (patterns.ExpandContext, error) {
	b := composeBaseBounds(ctx)
	ctx.LayoutBounds = b
	before, after := composeBandRows(c, ctx)
	rows := append(append([]jsonschema.GridRowInput(nil), before...), after...)
	used := 0.0
	for _, r := range rows {
		h, err := composeRowNeed(r, float64(b.Width)/12700, ctx)
		if err != nil {
			return ctx, err
		}
		used += h
	}
	if len(before) > 0 {
		used += float64(len(before)) * composeGapPt(c, ctx)
	}
	if len(after) > 0 {
		used += float64(len(after)) * composeGapPt(c, ctx)
	}
	ctx.LayoutBounds.Height -= int64(math.Ceil(used) * 12700)
	ctx.LayoutBounds.Y += int64(composeRowsHeight(before, float64(b.Width)/12700, ctx) * 12700)
	if len(before) > 0 {
		ctx.LayoutBounds.Y += int64(float64(len(before)) * composeGapPt(c, ctx) * 12700)
	}
	if ctx.LayoutBounds.Height <= 0 {
		return ctx, fmt.Errorf("FIT_OVERFLOW: compose banner/callout consume the whole content area")
	}
	return ctx, nil
}
func composeRowsHeight(rows []jsonschema.GridRowInput, width float64, ctx patterns.ExpandContext) float64 {
	total := 0.0
	for _, r := range rows {
		h, err := composeRowNeed(r, width, ctx)
		if err != nil {
			return math.Inf(1)
		}
		total += h
	}
	return total
}
func expandMeasuredCompose(c *ComposeInput, ctx patterns.ExpandContext, reg *patterns.Registry) (*jsonschema.ShapeGridInput, []string, error) {
	grids, sizes, warnings, err := measuredComposeSegments(c, ctx, reg)
	if err != nil {
		return nil, warnings, err
	}
	bodyCtx, err := composeContentContext(c, ctx)
	if err != nil {
		return nil, warnings, err
	}
	layoutCtx := bodyCtx
	if c.Banner != nil || c.Callout != nil {
		layoutCtx.LayoutBounds.Width -= 2 * subGridInsetEMU
		layoutCtx.LayoutBounds.Height -= 2 * subGridInsetEMU
	}
	body := mergeMeasuredPanes(c, grids, sizes, layoutCtx)
	if c.Banner == nil && c.Callout == nil {
		return body, warnings, nil
	}
	before, after := composeBandRows(c, ctx)
	width := float64(composeBaseBounds(ctx).Width) / 12700
	for i := range before {
		h, e := composeRowNeed(before[i], width, ctx)
		if e != nil {
			return nil, warnings, e
		}
		before[i].MinHeight, before[i].MaxHeight = h, h
	}
	for i := range after {
		h, e := composeRowNeed(after[i], width, ctx)
		if e != nil {
			return nil, warnings, e
		}
		after[i].MinHeight, after[i].MaxHeight = h, h
	}
	// The body is a nested frame; reserve its inset during segment measurement.
	row := jsonschema.GridRowInput{MinHeight: float64(bodyCtx.LayoutBounds.Height) / 12700, MaxHeight: float64(bodyCtx.LayoutBounds.Height) / 12700, Cells: []*jsonschema.GridCellInput{{Grid: body}}}
	rows := append(before, row)
	rows = append(rows, after...)
	return &jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), Rows: rows, RowGap: math.Max(composeGapPt(c, ctx), composeZeroGapPt), VerticalAlign: "top"}, warnings, nil
}
func measuredComposeSegments(c *ComposeInput, ctx patterns.ExpandContext, reg *patterns.Registry) ([]*jsonschema.ShapeGridInput, []float64, []string, error) {
	ctx, err := composeContentContext(c, ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if c.Banner != nil || c.Callout != nil {
		ctx.LayoutBounds.Width -= 2 * subGridInsetEMU
		ctx.LayoutBounds.Height -= 2 * subGridInsetEMU
	}
	sizes := resolveSegmentSizes(c.Segments)
	// Initial geometry is based on fixed shares, then implicit panes receive
	// measured weights at those candidate widths. Fixed shares never normalize.
	bounds := measuredPaneBounds(c, ctx, sizes)
	grids, warnings, err := expandComposeSegments(c, ctx, composeLeafBounds(c, bounds, ctx), reg)
	if err != nil {
		return nil, nil, nil, err
	}
	if c.SmartCompose || composeHasContentSize(c) || composeAllFixed(c) {
		sizes, err = measuredComposeShares(c, ctx, grids, bounds)
		if err != nil {
			return nil, nil, warnings, err
		}
		bounds = measuredPaneBounds(c, ctx, sizes)
		grids, warnings, err = expandComposeSegments(c, ctx, composeLeafBounds(c, bounds, ctx), reg)
		if err != nil {
			return nil, nil, warnings, err
		}
	}
	if c.Direction == "horizontal" {
		if e := validateHorizontalSegmentRows(grids); e != nil {
			return nil, nil, warnings, e
		}
	}
	leafBounds := composeLeafBounds(c, bounds, ctx)
	for i, g := range grids {
		if composeHasNativeLeaf(c.Segments[i]) || c.Segments[i].HasDiagram() {
			need, e := composeNaturalHeight(g, float64(leafBounds[i].Width)/12700, ctx)
			if e != nil {
				return nil, nil, warnings, e
			}
			if float64(leafBounds[i].Height)/12700+0.5 < need {
				return nil, nil, warnings, fmt.Errorf("FIT_OVERFLOW: compose segment[%d] needs%.0fpt at readable size but its pane after headings/captions holds%.0fpt", i, need, float64(leafBounds[i].Height)/12700)
			}
		}
		grids[i], err = decorateComposePane(c.Segments[i], g, bounds[i], ctx)
		if err != nil {
			return nil, nil, warnings, fmt.Errorf("compose segment[%d]: %w", i, err)
		}
	}
	return grids, sizes, warnings, nil
}
func composeHasContentSize(c *ComposeInput) bool {
	for _, s := range c.Segments {
		if s.Size == "content" {
			return true
		}
	}
	return false
}
func measuredPaneBounds(c *ComposeInput, ctx patterns.ExpandContext, sizes []float64) []patterns.LayoutBounds {
	b := composeBaseBounds(ctx)
	axis := b.Height
	pos := b.Y
	if c.Direction == "horizontal" {
		axis = b.Width
		pos = b.X
	}
	seams := composeSeams(c, ctx)
	for _, g := range seams {
		axis -= int64(math.Round(g * 12700))
	}
	out := make([]patterns.LayoutBounds, len(sizes))
	for i, share := range sizes {
		slot := b
		length := int64(math.Round(float64(axis) * share / 100))
		if c.Direction == "horizontal" {
			slot.X, slot.Width = pos, length
		} else {
			slot.Y, slot.Height = pos, length
		}
		slot.X += subGridInsetEMU
		slot.Y += subGridInsetEMU
		slot.Width -= 2 * subGridInsetEMU
		slot.Height -= 2 * subGridInsetEMU
		out[i] = slot
		pos += length
		if i < len(seams) {
			pos += int64(math.Round(seams[i] * 12700))
		}
	}
	return out
}
func composeLeafBounds(c *ComposeInput, bounds []patterns.LayoutBounds, ctx patterns.ExpandContext) []patterns.LayoutBounds {
	out := append([]patterns.LayoutBounds(nil), bounds...)
	maxHead := 0.0
	if c.Direction == "horizontal" {
		for i, s := range c.Segments {
			maxHead = math.Max(maxHead, composePaneTextHeight(s.Heading, 18, true, float64(bounds[i].Width)/12700, ctx))
		}
	}
	for i, s := range c.Segments {
		if s.Heading == "" && s.Caption == "" {
			continue
		}
		head := composePaneTextHeight(s.Heading, 18, true, float64(bounds[i].Width)/12700, ctx)
		if c.Direction == "horizontal" {
			head = maxHead
		}
		caption := composePaneTextHeight(s.Caption, 10, false, float64(bounds[i].Width)/12700, ctx)
		gap := 0.0
		if head > 0 {
			gap += ctx.Gap(4)
		}
		if caption > 0 {
			gap += ctx.Gap(4)
		}
		out[i].Height -= int64((head+caption+gap)*12700) + 2*subGridInsetEMU
		out[i].Width -= 2 * subGridInsetEMU
		topGap := 0.0
		if head > 0 {
			topGap = ctx.Gap(4)
		}
		out[i].Y += int64((head+topGap)*12700) + subGridInsetEMU
		out[i].X += subGridInsetEMU
	}
	return out
}
func measuredComposeShares(c *ComposeInput, ctx patterns.ExpandContext, grids []*jsonschema.ShapeGridInput, bounds []patterns.LayoutBounds) ([]float64, error) {
	axis := float64(ctx.LayoutBounds.Height) / 12700
	if c.Direction == "horizontal" {
		axis = float64(ctx.LayoutBounds.Width) / 12700
	}
	for _, g := range composeSeams(c, ctx) {
		axis -= g
	}
	if axis <= 0 {
		return nil, fmt.Errorf("FIT_OVERFLOW: compose seams exhaust content area")
	}
	sizes := make([]float64, len(grids))
	weights := make([]float64, len(grids))
	remaining := 100.0
	weight := 0.0
	implicit := 0
	for i, s := range c.Segments {
		need, err := composeNaturalHeight(grids[i], float64(bounds[i].Width)/12700, ctx)
		if s.Heading != "" || s.Caption != "" {
			need += composePaneTextHeight(s.Heading, 18, true, float64(bounds[i].Width)/12700, ctx) + composePaneTextHeight(s.Caption, 10, false, float64(bounds[i].Width)/12700, ctx) + 2*float64(subGridInsetEMU)/12700
			if s.Heading != "" {
				need += ctx.Gap(4)
			}
			if s.Caption != "" {
				need += ctx.Gap(4)
			}
		}
		if err != nil {
			return nil, err
		}
		switch {
		case s.SizePct > 0:
			sizes[i] = s.SizePct
			remaining -= sizes[i]
		case s.Size == "content":
			sizes[i] = (need + 2*float64(subGridInsetEMU)/12700) / axis * 100
			remaining -= sizes[i]
		default:
			weights[i] = math.Max(need, 1)
			if !c.SmartCompose {
				weights[i] = 1
			}
			weight += weights[i]
			implicit++
		}
	}
	if remaining < 0 || implicit > 0 && remaining <= 0 {
		return nil, fmt.Errorf("FIT_OVERFLOW: compose content-sized and fixed panes leave no room; shorten content or split the slide")
	}
	if implicit == 0 {
		if math.Abs(remaining) > 0.1 {
			return nil, fmt.Errorf("compose fixed/content panes must fill their allocation or leave one implicit pane")
		}
		return sizes, nil
	}
	for i := range sizes {
		if weights[i] > 0 {
			sizes[i] = remaining * weights[i] / weight
		}
	}
	if err := validateContentPaneMinimums(c, ctx, grids, bounds, sizes, axis); err != nil {
		return nil, err
	}

	return sizes, nil
}

func composePaneText(text string, size float64, bold bool) *jsonschema.GridCellInput {
	raw, _ := json.Marshal(map[string]any{"content": text, "size": size, "bold": bold, "vertical_align": "top", "align": "l"})
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: raw}}
}
func composePaneTextHeight(text string, size float64, bold bool, width float64, ctx patterns.ExpandContext) float64 {
	if text == "" {
		return 0
	}
	h, err := composeCellNeed(composePaneText(text, size, bold), width, ctx)
	if err != nil {
		return math.Inf(1)
	}
	return h
}
func decorateComposePane(seg SegmentInput, g *jsonschema.ShapeGridInput, b patterns.LayoutBounds, ctx patterns.ExpandContext) (*jsonschema.ShapeGridInput, error) {
	if seg.Heading == "" && seg.Caption == "" {
		return g, nil
	}
	width, height := float64(b.Width)/12700, float64(b.Height)/12700
	head := composePaneTextHeight(seg.Heading, 18, true, width, ctx)
	caption := composePaneTextHeight(seg.Caption, 10, false, width, ctx)
	gap := ctx.Gap(4)
	gaps := 0.0
	if head > 0 {
		gaps += gap
	}
	if caption > 0 {
		gaps += gap
	}
	available := height - head - caption - gaps
	if available <= 2*float64(subGridInsetEMU)/12700 {
		return nil, fmt.Errorf("FIT_OVERFLOW: pane heading/caption leave no readable content area")
	}
	var rows []jsonschema.GridRowInput
	if head > 0 {
		rows = append(rows, jsonschema.GridRowInput{MinHeight: head, MaxHeight: head, Cells: []*jsonschema.GridCellInput{composePaneText(seg.Heading, 18, true)}})
	}
	rows = append(rows, jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{{Grid: g}}})
	if caption > 0 {
		rows = append(rows, jsonschema.GridRowInput{MinHeight: caption, MaxHeight: caption, Cells: []*jsonschema.GridCellInput{composePaneText(seg.Caption, 10, false)}})
	}
	return &jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), Rows: rows, RowGap: gap, VerticalAlign: "top"}, nil
}
func composeNaturalHeight(g *jsonschema.ShapeGridInput, width float64, ctx patterns.ExpandContext) (float64, error) {
	if g == nil || len(g.Rows) == 0 {
		return 0, fmt.Errorf("compose leaf has no rows")
	}
	cols, err := resolveColumnsDTO(g.Columns, g.Rows)
	if err != nil {
		return 0, err
	}
	gap := g.ColGap
	if gap == 0 {
		gap = g.Gap
	}
	if gap == 0 {
		gap = ctx.GutterPt()
	}
	rowGap := g.RowGap
	if rowGap == 0 {
		rowGap = g.Gap
	}
	if rowGap == 0 {
		rowGap = ctx.GutterPt()
	}
	usable := width - gap*float64(len(cols)-1)
	if usable <= 0 {
		return 0, fmt.Errorf("FIT_OVERFLOW: pane columns and gaps leave no text width")
	}
	if err := validateComposeGridOccupancy(g); err != nil {
		return 0, err
	}
	heights := make([]float64, len(g.Rows))
	occupied := make([][]bool, len(g.Rows))
	for r := range occupied {
		occupied[r] = make([]bool, len(cols))
	}
	for r, row := range g.Rows {
		if err := measureComposeRowCells(r, row, cols, usable, gap, occupied, heights, ctx); err != nil {
			return 0, err
		}

		heights[r] = math.Max(heights[r], row.MinHeight)
		if row.MaxHeight > 0 {
			heights[r] = math.Max(heights[r], row.MaxHeight)
		}
	}
	total := rowGap * float64(max(len(heights)-1, 0))
	for _, h := range heights {
		total += h
	}
	return total, nil
}
func composeRowNeed(row jsonschema.GridRowInput, width float64, ctx patterns.ExpandContext) (float64, error) {
	need := math.Max(row.MinHeight, row.MaxHeight)
	for _, c := range row.Cells {
		h, err := composeCellNeed(c, width, ctx)
		if err != nil {
			return 0, err
		}
		need = math.Max(need, h)
	}
	return need, nil
}
func composeCellNeed(cell *jsonschema.GridCellInput, width float64, ctx patterns.ExpandContext) (float64, error) {
	if cell == nil {
		return 0, nil
	}
	if cell.Grid != nil {
		h, err := composeNaturalHeight(cell.Grid, width-2*float64(subGridInsetEMU)/12700, ctx)
		return h + 2*float64(subGridInsetEMU)/12700, err
	}
	if cell.Shape != nil && len(cell.Shape.Text) > 0 {
		tb, err := shapegrid.ResolveTextInput(cell.Shape.Text)
		if err != nil {
			return 0, err
		}
		if tb == nil {
			return 0, nil
		}
		tb.ThemeFonts = pptx.ThemeFonts{Major: ctx.Theme.TitleFont, Minor: ctx.Theme.BodyFont}
		largest := 12.0
		for _, p := range tb.Paragraphs {
			for _, r := range p.Runs {
				largest = math.Max(largest, float64(r.FontSize)/100)
			}
		}
		lo := math.Ceil(2*pptx.ShapeTextInsetPt + largest*1.25)
		hi := math.Max(lo, 2000)
		fits := func(h float64) bool {
			return pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(width * 12700), CY: int64(h * 12700)})
		}
		if !fits(hi) {
			return 0, fmt.Errorf("FIT_OVERFLOW: pane text cannot fit at readable size in %.0fpt width", width)
		}
		for hi-lo > 1 {
			mid := math.Floor((hi + lo) / 2)
			if fits(mid) {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi, nil
	}
	if cell.Table != nil {
		return float64(generator.MeasureTableHeight(cell.Table.ToTableSpec(), int64(width*12700), 0)) / 12700, nil
	}
	if cell.Image != nil || cell.Diagram != nil {
		return 72, nil
	}
	if len(cell.Layers) > 0 {
		return 48, nil
	}
	return 0, nil
}

// One pane per track keeps a child's lattice independent. Two half-gap
// spacer tracks put a native column rule at the centre of each seam.
func mergeMeasuredPanes(c *ComposeInput, grids []*jsonschema.ShapeGridInput, sizes []float64, ctx patterns.ExpandContext) *jsonschema.ShapeGridInput {
	b := composeBaseBounds(ctx)
	seams := composeSeams(c, ctx)
	axis := float64(b.Height) / 12700
	if c.Direction == "horizontal" {
		axis = float64(b.Width) / 12700
	}
	available := axis
	for _, g := range seams {
		available -= g
	}
	grid := &jsonschema.ShapeGridInput{ColGap: composeZeroGapPt, RowGap: composeZeroGapPt, VerticalAlign: "top"}
	var widths []float64
	var cells []*jsonschema.GridCellInput
	var rules []int
	for i, g := range grids {
		pane := &jsonschema.GridCellInput{Grid: g}
		length := available * sizes[i] / 100
		if c.Direction == "horizontal" {
			widths = append(widths, length)
			cells = append(cells, pane)
		} else {
			grid.Rows = append(grid.Rows, jsonschema.GridRowInput{Height: length / axis * 100, Cells: []*jsonschema.GridCellInput{pane}})
		}
		if i == len(grids)-1 {
			continue
		}
		if c.Direction == "horizontal" {
			widths = append(widths, math.Max(seams[i]/2, composeZeroGapPt), math.Max(seams[i]/2, composeZeroGapPt))
			cells = append(cells, &jsonschema.GridCellInput{}, &jsonschema.GridCellInput{})
			if c.Divider {
				rules = append(rules, len(widths)-2)
			}
		} else {
			gap := seams[i] / 2
			first := jsonschema.GridRowInput{Height: gap / axis * 100, Cells: []*jsonschema.GridCellInput{{}}}
			if gap == 0 {
				first.MinHeight, first.MaxHeight = composeZeroGapPt, composeZeroGapPt
			}
			if c.Divider {
				first.Rule = "below"
			}
			grid.Rows = append(grid.Rows, first, jsonschema.GridRowInput{Height: gap / axis * 100, MinHeight: first.MinHeight, MaxHeight: first.MaxHeight, Cells: []*jsonschema.GridCellInput{{}}})
		}
	}
	if c.Direction == "horizontal" {
		grid.Columns, _ = json.Marshal(widths)
		grid.Rows = []jsonschema.GridRowInput{{Cells: cells}}
		if c.Divider {
			grid.ColRule, _ = json.Marshal(rules)
		}
	} else {
		grid.Columns = json.RawMessage(`1`)
	}
	return grid
}

func validateContentPaneMinimums(c *ComposeInput, ctx patterns.ExpandContext, grids []*jsonschema.ShapeGridInput, bounds []patterns.LayoutBounds, sizes []float64, axis float64) error {
	if c.Direction != "vertical" || !composeHasContentSize(c) {
		return nil
	}

	for i, g := range grids {
		need, e := composeNaturalHeight(g, float64(bounds[i].Width)/12700, ctx)
		if e != nil {
			return e
		}
		if sizes[i]*axis/100+0.5 < need+2*float64(subGridInsetEMU)/12700 {
			return fmt.Errorf("FIT_OVERFLOW: content-sized compose leaves segment[%d] %.0fpt but its readable minimum is%.0fpt; split the slide", i, sizes[i]*axis/100, need)
		}
	}
	return nil
}

func measureComposeRowCells(r int, row jsonschema.GridRowInput, cols []float64, usable, gap float64, occupied [][]bool, heights []float64, ctx patterns.ExpandContext) error {
	col := 0
	for _, cell := range row.Cells {
		for col < len(cols) && occupied[r][col] {
			col++
		}
		span := 1
		if cell != nil {
			span = max(cell.ColSpan, 1)
		}
		if col+span > len(cols) {
			return fmt.Errorf("COMPOSE_HORIZONTAL_TRUNCATION: row%d exceeds allocated columns; split or reduce cells", r)
		}
		w := gap * float64(span-1)
		for j := col; j < col+span; j++ {
			w += usable * cols[j] / 100
		}
		need, e := composeCellNeed(cell, w, ctx)
		if e != nil {
			return e
		}
		rows := 1
		if cell != nil {
			rows = max(cell.RowSpan, 1)
		}
		for j := r; j < min(r+rows, len(heights)); j++ {
			heights[j] = math.Max(heights[j], need/float64(rows))
			for x := col; x < col+span; x++ {
				occupied[j][x] = true
			}
		}
		col += span
	}
	return nil
}

func composeAllFixed(c *ComposeInput) bool {
	for _, s := range c.Segments {
		if s.SizePct <= 0 {
			return false
		}
	}
	return true
}
