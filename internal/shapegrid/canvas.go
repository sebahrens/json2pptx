package shapegrid

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Canvas scale (go-slide-creator-ttpae).
//
// Patterns are designed in points on the standard 13.33 x 7.5in slide: row
// heights, gaps and the type scale are point values. A template with a larger
// slide (business-template, 14.7 x 8.3in) shows every point about 10% smaller
// on the same screen, so the same pattern read as small type in a mostly empty
// content area. A pattern grid now carries the slide's size relative to the
// standard one (Grid.CanvasScale, CanvasScaleFor) and Resolve applies it:
//
//  1. Point-valued row heights (min_height / max_height), cell caps and gaps
//     grow by the scale. Percentage heights already follow the area, and
//     hairlines keep their thickness.
//  2. The grid is resolved as designed (placement policy, type step, snap
//     onto the type scale), then every paragraph is written at its size times
//     the scale, rounded to a whole point (12 -> 13, 14 -> 15, 18 -> 20 at
//     1.10).
//  3. Text is scaled only where it fits no worse than as designed: no cell
//     needs more of its height than before (or it stays inside
//     composeFitMargin) and no word or short label that held its line breaks.
//     A content-sized block whose text wraps onto an extra line is retried
//     with taller rows (canvasRowGrowths); a size level that still does not
//     hold keeps its design size (resolveCanvas). A grid none of whose text
//     can move is resolved as designed, rows included.
//
// On a standard slide the scale is exactly 1 and Resolve takes the design
// path untouched. Slides smaller than the standard one are not scaled down:
// the type scale's steps are readability floors.
const (
	// canvasScaleMin is the scale under which a slide counts as standard
	// (13.33in vs 13.333in rounding in a template's presentation.xml).
	canvasScaleMin = 1.02
	// canvasScaleMax bounds the scale for an outsized slide.
	canvasScaleMax = 1.5
)

// canvasRowGrowths are the extra row growths tried, on top of the canvas
// scale, for a content-sized block whose scaled text needs another line.
var canvasRowGrowths = []float64{1, 1.12, 1.25}

// CanvasScaleFor is the canvas scale of a slide of the given size: the smaller
// of its width and height relative to the standard 16:9 slide, 1 for a
// standard or smaller slide.
func CanvasScaleFor(slideWidthEMU, slideHeightEMU int64) float64 {
	if slideWidthEMU <= 0 || slideHeightEMU <= 0 {
		return 1
	}
	c := math.Min(float64(slideWidthEMU)/float64(DefaultSlideWidthEMU), float64(slideHeightEMU)/float64(DefaultSlideHeightEMU))
	if c < canvasScaleMin {
		return 1
	}
	return math.Min(c, canvasScaleMax)
}

// resolveCanvas resolves a grid whose CanvasScale is above 1. ok is false
// when no text level of the scaled grid holds; the caller then resolves the
// grid as designed.
//
// Text is scaled a level at a time: a level is every paragraph the design
// writes at one size. A level with a word, short label or KPI value that would
// lose its line keeps its design size, as does the largest level of a cell
// that no longer holds its text once the taller rows have been tried; the
// other levels still follow the canvas. Levels are 10% apart at most, so a
// kept level never ends below a scaled one beneath it.
func resolveCanvas(grid *Grid, alloc *pptx.ShapeIDAllocator) (*ResolveResult, bool) {
	design := *grid
	design.CanvasScale = 0
	base, err := resolveDesign(&design, pptx.NewShapeIDAllocator(nil))
	if err != nil || base == nil {
		return nil, false
	}
	scales := canvasRowScales(&design, grid.CanvasScale)
	keep := map[float64]bool{}
	growth := 0
	// Every pass either returns, keeps one more level or tries the next
	// growth, so the loop is bounded by levels x growths.
	for pass := 0; pass < canvasMaxPasses; pass++ {
		scaled := canvasGrid(&design, scales[growth])
		scaled.canvasText, scaled.canvasKeep = grid.CanvasScale, keep
		trial, err := resolveDesign(scaled, pptx.NewShapeIDAllocator(nil))
		if err != nil || trial == nil || len(trial.Cells) != len(base.Cells) {
			return nil, false
		}
		check := canvasCheck(base.Cells, trial.Cells)
		switch {
		case !check.moved:
			return nil, false
		case len(check.broken) > 0:
			for pt := range check.broken {
				keep[pt] = true
			}
		case check.unfit == 0:
			res, err := resolveDesign(scaled, alloc)
			return res, err == nil && res != nil
		case growth+1 < len(scales):
			growth++
		default:
			keep[check.unfit] = true
			growth = 0
		}
	}
	return nil, false
}

// canvasMaxPasses bounds resolveCanvas: more size levels than any pattern
// writes, times the row growths.
const canvasMaxPasses = 24

// canvasRowScales lists the row scales resolveCanvas tries, smallest first:
// the canvas scale, then the canvas scale with each of canvasRowGrowths. A
// stretch grid has the one: its rows fill their bounds whatever they ask for.
// A content-sized block that hangs from the body line stays under it — its
// rows grow no further than the area beneath the line holds — and a block
// takes a further growth only while that leaves its bounds unfilled.
func canvasRowScales(design *Grid, c float64) []float64 {
	scales := []float64{c}
	if effectiveVAlign(design) == VAlignStretch {
		return scales
	}
	limit := 0.0
	if fill, ok := blockFill(design); ok && design.AnchorY > design.Bounds.Y && design.Bounds.CY > 0 {
		under := 1 - float64(design.AnchorY-design.Bounds.Y)/float64(design.Bounds.CY)
		// A block already too tall to start on the line keeps its height:
		// growing it would only lift it further above the line.
		limit = math.Max(under/fill*canvasAnchorMargin, 1)
	}
	if limit > 0 && c > limit {
		return []float64{limit}
	}
	for _, g := range canvasRowGrowths[1:] {
		k := c * g
		if limit > 0 && k > limit {
			k = limit
		}
		if _, room := blockFill(canvasGrid(design, k)); !room || k <= scales[len(scales)-1] {
			break
		}
		scales = append(scales, k)
	}
	return scales
}

// canvasAnchorMargin keeps a block grown to the area under the body line a
// little inside it: hairline rows do not scale, so the estimate runs high.
const canvasAnchorMargin = 0.99

// canvasGrid returns a copy of grid whose point-valued heights and gaps are k
// times as large. Percentage heights and hairlines are kept.
func canvasGrid(grid *Grid, k float64) *Grid {
	out := *grid
	out.RowGap = canvasGap(effectiveRowGap(grid), k)
	colGap := grid.ColGap
	if colGap == 0 {
		colGap = effectiveRowGap(&Grid{DefaultGapPt: grid.DefaultGapPt})
	}
	out.ColGap = canvasGap(colGap, k)
	out.Rows = make([]Row, len(grid.Rows))
	for i, row := range grid.Rows {
		if row.MaxHeight >= composeHairlinePt || row.MaxHeight == 0 {
			row.MinHeight *= k
			row.MaxHeight *= k
		}
		cells := make([]Cell, len(row.Cells))
		for j, c := range row.Cells {
			if c.MaxHeight >= composeHairlinePt {
				c.MaxHeight *= k
			}
			c.BleedTop *= k
			c.BleedLeft *= k
			c.InsetTop *= k
			c.InsetBottom *= k
			cells[j] = c
		}
		row.Cells = cells
		out.Rows[i] = row
	}
	return &out
}

// canvasGap scales a gap; a hairline / sentinel gap (0.01pt "no gap", a 1pt
// rule spacing) is part of the drawing and keeps its size.
func canvasGap(pt, k float64) float64 {
	if pt <= 1 {
		return pt
	}
	return pt * k
}

// canvasSize is the size a paragraph designed at pt is written at.
func canvasSize(pt, scale float64) float64 {
	return math.Max(pt, math.Round(pt*scale))
}

// canvasShapeText returns a private copy of spec with every sized paragraph at
// its canvas size; spec itself when nothing moves. A paragraph without a size
// renders at the default, which is scaled like any other. keep lists the
// design sizes (levels) that stay as designed.
func canvasShapeText(spec *ShapeSpec, scale float64, keep map[float64]bool) *ShapeSpec {
	if spec == nil || len(spec.Text) == 0 || scale <= 1 {
		return spec
	}
	sized := func(size float64, content string) (float64, bool) {
		if strings.TrimSpace(content) == "" {
			return size, false
		}
		if size <= 0 {
			size = DefaultTextSizePt
		}
		size = EffectiveTextSizePt(size)
		if keep[size] {
			return size, false
		}
		next := canvasSize(size, scale)
		return next, next != size
	}
	var s string
	var out json.RawMessage
	if json.Unmarshal(spec.Text, &s) == nil {
		next, ok := sized(0, s)
		if !ok {
			return spec
		}
		out, _ = json.Marshal(map[string]any{"content": s, "size": next})
	} else {
		var obj map[string]json.RawMessage
		if json.Unmarshal(spec.Text, &obj) != nil {
			return spec
		}
		changed := false
		if rawParas, ok := obj["paragraphs"]; ok {
			var defs []map[string]json.RawMessage
			if json.Unmarshal(rawParas, &defs) != nil {
				return spec
			}
			for i := range defs {
				var size, suffixSize float64
				var content string
				_ = json.Unmarshal(defs[i]["size"], &size)
				_ = json.Unmarshal(defs[i]["content"], &content)
				next, ok := sized(size, content)
				if !ok {
					continue
				}
				changed = true
				defs[i]["size"], _ = json.Marshal(next)
				if json.Unmarshal(defs[i]["suffix_size"], &suffixSize) == nil && suffixSize > 0 {
					defs[i]["suffix_size"], _ = json.Marshal(canvasSize(suffixSize, scale))
				}
			}
			obj["paragraphs"], _ = json.Marshal(defs)
		} else {
			var size float64
			var content string
			_ = json.Unmarshal(obj["size"], &size)
			_ = json.Unmarshal(obj["content"], &content)
			next, ok := sized(size, content)
			if ok {
				changed = true
				obj["size"], _ = json.Marshal(next)
			}
		}
		if !changed {
			return spec
		}
		out, _ = json.Marshal(obj)
	}
	if len(out) == 0 {
		return spec
	}
	copySpec := *spec
	copySpec.Text = out
	return &copySpec
}

// canvasResult is what canvasCheck finds.
type canvasResult struct {
	// moved reports that at least one paragraph is written larger.
	moved bool
	// broken lists the design sizes (levels) with a word, a short label or a
	// KPI value that held its line as designed and no longer does, which
	// taller rows cannot mend.
	broken map[float64]bool
	// unfit is the largest scaled level of the cells that need more of their
	// text height than composeFitMargin and more than they did as designed;
	// 0 when every cell holds its text.
	unfit float64
}

// canvasFitSlack is how much more of its height a cell may need than as
// designed before it counts as worse: measuring noise on a one-line cell.
const canvasFitSlack = 0.02

// canvasCheck compares every text cell of the scaled grid with the same cell
// as designed.
func canvasCheck(design, scaled []ResolvedCell) canvasResult {
	res := canvasResult{broken: map[float64]bool{}}
	for i := range scaled {
		before, after := &design[i], &scaled[i]
		if after.Kind != CellKindShape || after.ShapeSpec == nil || before.ShapeSpec == nil {
			continue
		}
		wA, hA, parasA := canvasTextBox(after)
		wB, hB, parasB := canvasTextBox(before)
		if len(parasA) == 0 || len(parasA) != len(parasB) || wA <= 0 || hA <= 0 || wB <= 0 || hB <= 0 {
			continue
		}
		largest := 0.0
		for j, p := range parasA {
			level := parasB[j].fontPt
			if p.fontPt == level {
				continue
			}
			res.moved = true
			largest = math.Max(largest, level)
			if canvasBreaksLine(p, level, float64(wA-p.marginL), float64(wB-parasB[j].marginL)) {
				res.broken[level] = true
			}
		}
		if largest == 0 {
			continue
		}
		need := blockHeightPt(parasA, wA) / (float64(hA) / 12700)
		if need > composeFitMargin && need > blockHeightPt(parasB, wB)/(float64(hB)/12700)+canvasFitSlack {
			res.unfit = math.Max(res.unfit, largest)
		}
		// The writer's own measure decides the stored autofit shrink, in the
		// theme face the pattern sized the text in: scaled text the writer
		// would shrink further than the design's is not larger on the slide,
		// and a 13pt run stored at 92% falls under the readable floor its 12pt
		// design stood on.
		if canvasAutofit(after) < canvasAutofit(before)-canvasAutofitSlack {
			res.unfit = math.Max(res.unfit, largest)
		}
	}
	return res
}

// canvasAutofitSlack absorbs rounding in the stored autofit scale.
const canvasAutofitSlack = 0.001

// canvasAutofit is the shrink the writer stores for a resolved shape cell's
// text (1 when it fits or is not autofit), measured as shareRowAutofitScale
// measures it.
func canvasAutofit(cell *ResolvedCell) float64 {
	tb, err := ResolveTextInput(cell.ShapeSpec.Text)
	if err != nil || tb == nil {
		return 1
	}
	for j := 0; j < 4; j++ {
		tb.Insets[j] += cell.TextInsets[j]
	}
	tb.ThemeFonts = cell.ShapeSpec.ThemeFonts
	return pptx.AutofitScaleFor(tb, cell.Bounds)
}

// canvasBreaksLine reports whether a paragraph designed at level points, on a
// line lineB wide, has a word, a short label or a KPI value that held that
// line and does not hold the scaled line lineA at the paragraph's scaled size.
func canvasBreaksLine(p scaleParagraph, level, lineA, lineB float64) bool {
	text := strings.TrimSpace(p.text)
	toks := strings.Fields(text)
	if len(toks) > 1 && len(toks) <= ComposeLabelMaxWords {
		toks = append(toks, text) // a short label stays on its line
	}
	if p.role == "kpi-value" {
		toks = []string{text} // a value stays with its unit
	}
	for _, tok := range toks {
		w0, err0 := textfit.MeasureStyledLineWidth(tok, "Liberation Sans", level, p.bold)
		w1, err1 := textfit.MeasureStyledLineWidth(tok, "Liberation Sans", p.fontPt, p.bold)
		if err0 != nil || err1 != nil {
			continue
		}
		limit := lineA
		if !strings.Contains(tok, " ") || p.role == "kpi-value" {
			// One token cannot wrap: it keeps the atomic-token margin the
			// stand-in face needs, unless the design already sat inside it.
			limit = math.Max(lineA*textfit.AtomicTokenWidthPct/100, float64(w0))
		}
		if float64(w0) <= lineB && float64(w1) > limit {
			return true
		}
	}
	return false
}

// canvasTextBox is the text rectangle and measured paragraphs of a resolved
// shape cell, with rotated text measured along its own axis.
func canvasTextBox(cell *ResolvedCell) (width, height int64, paras []scaleParagraph) {
	tb, err := ResolveTextInput(cell.ShapeSpec.Text)
	if err != nil || tb == nil {
		return 0, 0, nil
	}
	pw, ph := pptx.PresetTextRect(cell.ShapeSpec.Geometry, cell.ShapeSpec.Adjustments, cell.Bounds)
	width, height = scaledTextRect(pptx.RectEmu{CX: pw, CY: ph}, tb, cell.TextInsets)
	if tb.Vert != "" {
		width, height = height, width
	}
	return width, height, scaleParagraphs(tb)
}
