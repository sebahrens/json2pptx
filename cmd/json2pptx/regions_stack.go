package main

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// A regions slide's main_left / main_right stack splits its column by shares
// the compiler writes without font metrics: a stat over a text region took
// 40% / 60% whatever the text needed. Three two-line bullets then missed their
// 60% by a few points, were written with a shrink the renderers apply
// differently, and ended above the main region's bottom edge with a blank band
// under them; two one-line bullets left most of their share empty while the
// figure above stayed small (go-slide-creator-18dqh, journey f-A12).
//
// When the stack is expanded, the text region is measured in its real column:
// it takes the height the writer needs to store it unshrunk, and the region
// stacked with it — a stat, KPI row or timeline, which sizes itself to its
// cell — takes the rest of the main region's extent. The stat-hero figure
// grows to the cell it is given and is centred in it. Text that needs more
// than the pattern region can give up (regionsStackVisualMinPct) keeps the
// split its author wrote: the slide holds too much, and the finding belongs
// on the text.

// regionsStackVisualMinPct is the least share of the stack the measured text
// leaves the pattern region stacked with it, by pattern: the shares these
// regions read in on the shortest shipped content area
// (slides.regionMinHeightPct). A region under a heading needs
// regionsStackHeadingPct more.
var regionsStackVisualMinPct = map[string]float64{"stat-hero": 35, "timeline-horizontal": 45}

const (
	// regionsStackKPIMinPct is the minimum of a KPI row.
	regionsStackKPIMinPct = 30.0
	// regionsStackOtherMinPct is the minimum of any other pattern region.
	regionsStackOtherMinPct = 45.0
	regionsStackHeadingPct  = 10.0
	// regionsStackTextSlackPt is the air the text row keeps over its written
	// fit, so a renderer that measures a glyph a little wider does not shrink
	// it.
	regionsStackTextSlackPt = 4.0
	// regionsStackMinChangePct is the smallest change of share worth making.
	regionsStackMinChangePct = 1.0
	// regionsStackDefaultGapPt is the shape grid's default row gap.
	regionsStackDefaultGapPt = 8.0
)

// rebalanceRegionsStack re-splits a regions stack (two rows of one cell each)
// between its text region and its pattern region. It changes nothing for any
// other grid, for a stack without exactly one text region and one pattern
// region, and when the text cannot be measured.
func rebalanceRegionsStack(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, bounds pptx.RectEmu) {
	if grid == nil || grid.Source != jsonschema.CompilerRegionsStackSource || len(grid.Rows) != 2 || bounds.CX <= 0 || bounds.CY <= 0 {
		return
	}
	textRow, visualRow := -1, -1
	for i, row := range grid.Rows {
		if len(row.Cells) != 1 || row.Cells[0] == nil || row.Height <= 0 {
			return
		}
		switch {
		case regionCellHasPattern(row.Cells[0]):
			visualRow = i
		case regionTextCell(row.Cells[0]) != nil:
			textRow = i
		}
	}
	if textRow < 0 || visualRow < 0 {
		return
	}

	// The compiler's points follow the canvas on a larger slide; measure in
	// design points and compare against the stack's design height.
	k := gridCanvasScale(grid, ctx.SlideWidth, ctx.SlideHeight)
	if k <= 0 {
		k = 1
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}
	if rowGap == 0 {
		rowGap = regionsStackDefaultGapPt
	}
	widthPt := float64(bounds.CX) / emuPerPt / k
	availPt := float64(bounds.CY)/emuPerPt/k - rowGap
	if availPt <= 0 {
		return
	}
	text, ok := measureRegionText(grid.Rows[textRow].Cells[0], widthPt)
	if !ok {
		return
	}
	total := grid.Rows[textRow].Height + grid.Rows[visualRow].Height
	visual := grid.Rows[visualRow].Cells[0]
	visualMin := math.Min(grid.Rows[visualRow].Height, regionVisualMinPct(visual)*total/100)
	// The slack is the first thing a tight stack gives up.
	share := math.Min(math.Ceil((text.needPt+regionsStackTextSlackPt)/availPt*total*10)/10, total-visualMin)
	if share <= 0 || math.Abs(share-grid.Rows[textRow].Height) < regionsStackMinChangePct {
		return
	}
	if share > grid.Rows[textRow].Height {
		// The text takes height from the pattern region only when that fits
		// both: the text readable in what it is given, and the pattern still
		// holding its own content in what is left. Otherwise the slide holds
		// too much for the stack and keeps the split its author wrote, so the
		// finding names the region that overflows.
		if !text.readableIn(share / total * availPt) {
			return
		}
		rest := pptx.RectEmu{CX: bounds.CX, CY: int64((total - share) / total * availPt * k * emuPerPt)}
		if !regionPatternFits(visual, ctx, rest) {
			return
		}
	}
	grid.Rows[textRow].Height = share
	grid.Rows[visualRow].Height = total - share
}

// regionPatternFits reports whether a pattern region's pattern expands in a
// cell of the given size without refusing it or reporting text it cannot hold
// at a readable size.
func regionPatternFits(cell *jsonschema.GridCellInput, ctx patterns.ExpandContext, bounds pptx.RectEmu) bool {
	raw := cell.Pattern
	if len(raw) == 0 && cell.Grid != nil {
		// Under its heading: the nested grid's inset, the heading row and
		// the gap come off the cell.
		bounds = pptx.RectEmu{CX: bounds.CX - 2*subGridInsetEMU, CY: bounds.CY - 2*subGridInsetEMU}
		gap := cell.Grid.RowGap
		if gap == 0 {
			gap = cell.Grid.Gap
		}
		if gap == 0 {
			gap = regionsStackDefaultGapPt
		}
		k := gridCanvasScale(cell.Grid, ctx.SlideWidth, ctx.SlideHeight)
		for _, row := range cell.Grid.Rows {
			bounds.CY -= int64(row.MaxHeight * k * emuPerPt)
			for _, c := range row.Cells {
				if c != nil && len(c.Pattern) > 0 {
					raw = c.Pattern
				}
			}
		}
		bounds.CY -= int64(float64(len(cell.Grid.Rows)-1) * gap * k * emuPerPt)
	}
	var pi PatternInput
	if len(raw) == 0 || bounds.CX <= 0 || bounds.CY <= 0 || json.Unmarshal(raw, &pi) != nil {
		return false
	}
	ctx.LayoutBounds = patterns.LayoutBounds{Width: bounds.CX, Height: bounds.CY}
	_, warnings, err := expandPattern(&pi, ctx, patterns.Default())
	if err != nil {
		return false
	}
	for _, w := range warnings {
		for _, code := range []string{patterns.ErrCodeBodyTooLong, patterns.ErrCodeFitOverflow, patterns.ErrCodeTextExceedsShape, patterns.ErrCodeHeadlineTooLong} {
			if strings.HasPrefix(w, code) {
				return false
			}
		}
	}
	return true
}

// regionVisualMinPct is the least share of the stack a pattern region reads in.
func regionVisualMinPct(cell *jsonschema.GridCellInput) float64 {
	raw, heading := cell.Pattern, 0.0
	if len(raw) == 0 && cell.Grid != nil {
		heading = regionsStackHeadingPct
		for _, row := range cell.Grid.Rows {
			for _, c := range row.Cells {
				if c != nil && len(c.Pattern) > 0 {
					raw = c.Pattern
				}
			}
		}
	}
	var p struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &p)
	if pct, ok := regionsStackVisualMinPct[p.Name]; ok {
		return pct + heading
	}
	if strings.HasPrefix(p.Name, "kpi-") {
		return regionsStackKPIMinPct + heading
	}
	return regionsStackOtherMinPct + heading
}

// regionCellHasPattern reports whether a region's cell is a named pattern,
// directly or under its heading row.
func regionCellHasPattern(cell *jsonschema.GridCellInput) bool {
	if len(cell.Pattern) > 0 {
		return true
	}
	return cell.Grid != nil && hasNestedCellPattern(cell.Grid)
}

// regionTextCell returns the text box of a text region's cell — the cell
// itself, or the one content cell under its heading row — or nil when the
// region is not text only.
func regionTextCell(cell *jsonschema.GridCellInput) *jsonschema.GridCellInput {
	plain := func(c *jsonschema.GridCellInput) bool {
		return c != nil && c.Shape != nil && len(c.Shape.Text) > 0 && len(c.Pattern) == 0 &&
			c.Grid == nil && c.Table == nil && c.Icon == nil && c.Image == nil && c.Diagram == nil && c.Composite == nil
	}
	if plain(cell) {
		return cell
	}
	if cell.Grid == nil || len(cell.Pattern) > 0 {
		return nil
	}
	var body *jsonschema.GridCellInput
	for _, row := range cell.Grid.Rows {
		if len(row.Cells) != 1 || !plain(row.Cells[0]) {
			return nil
		}
		if row.MaxHeight > 0 {
			continue // a heading: one fixed line
		}
		if body != nil {
			return nil
		}
		body = row.Cells[0]
	}
	return body
}

// regionTextMeasure is a text region measured in its column.
type regionTextMeasure struct {
	// needPt is the height the region's cell needs so the writer stores its
	// text with no autofit shrink: the text box at its written fit plus,
	// under a heading, the heading row, the row gap and the inset a nested
	// grid is drawn with.
	needPt  float64
	fixedPt float64 // the part of needPt that is not the text box
	body    *pptx.TextBody
	widthEM int64
}

// readableIn reports whether the text is written at the readable floor or
// larger in a cell rowPt tall: generation refuses a smaller run.
func (m regionTextMeasure) readableIn(rowPt float64) bool {
	h := rowPt - m.fixedPt
	if h <= 0 {
		return false
	}
	smallest := 0
	for _, p := range m.body.Paragraphs {
		for _, run := range p.Runs {
			if strings.TrimSpace(run.Text) != "" && (smallest == 0 || run.FontSize < smallest) {
				smallest = run.FontSize
			}
		}
	}
	scale := pptx.AutofitScaleFor(m.body, pptx.RectEmu{CX: m.widthEM, CY: int64(h * emuPerPt)})
	return float64(smallest)/100*scale >= tokens.TypeScaleBodyPt
}

// measureRegionText measures a text region's cell in a widthPt-wide column.
func measureRegionText(cell *jsonschema.GridCellInput, widthPt float64) (regionTextMeasure, bool) {
	body := regionTextCell(cell)
	if body == nil {
		return regionTextMeasure{}, false
	}
	insetPt := float64(subGridInsetEMU) / emuPerPt
	fixed := 0.0
	if body != cell {
		widthPt -= 2 * insetPt
		fixed = 2 * insetPt
		gap := cell.Grid.RowGap
		if gap == 0 {
			gap = cell.Grid.Gap
		}
		if gap == 0 {
			gap = regionsStackDefaultGapPt
		}
		fixed += float64(len(cell.Grid.Rows)-1) * gap
		for _, row := range cell.Grid.Rows {
			fixed += row.MaxHeight
		}
	}
	tb, err := shapegrid.ResolveTextInput(body.Shape.Text)
	if err != nil || tb == nil || widthPt <= 0 {
		return regionTextMeasure{}, false
	}
	w := int64(widthPt * emuPerPt)
	fits := func(h float64) bool {
		return pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: w, CY: int64(h * emuPerPt)})
	}
	const maxPt = 600
	if !fits(maxPt) {
		return regionTextMeasure{}, false
	}
	lo, hi := 0.0, float64(maxPt)
	for hi-lo > 1 {
		mid := math.Floor((lo + hi) / 2)
		if fits(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return regionTextMeasure{needPt: hi + fixed, fixedPt: fixed, body: tb, widthEM: w}, true
}
