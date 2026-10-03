package shapegrid

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// Composition of a sparse content-sized block (go-slide-creator-yhzxt).
//
// A pattern that sizes its rows to its content (rows capped by max_height)
// used to hang from the top of the content area whatever it held: a strip of
// KPIs or a row of icons over a blank lower half, the way a web page flows
// from the top. One policy now places every such block, applied by Resolve to
// a slide's own grid (Grid.Compose) whose vertical_align is "auto":
//
//  1. A block that needs ComposeSparseFill of the area or more is dense: it
//     keeps its sizes and hangs from the body line, as before.
//  2. A sparse block first takes one step up its type scale (12→14pt,
//     14→18pt, display figures ×1.2 up to the 48pt KPI step) with its row
//     heights and row gap grown with it (composeRowScales) — when the stepped
//     text still fits its cells, no word that fit a line is broken and the
//     block stays under composeMaxFill of the area. Otherwise it keeps its
//     sizes.
//  3. It is then placed at the optical centre of the area: ComposeOpticalTop
//     of the spare height above it, the rest below, never above the body line.
//
// An author's bounds / max_height_pct (which resolve to "stretch") and an
// explicit vertical_align keep the block where the author put it; a cell or
// deck pinned to type_scale "compact" keeps its text sizes. A pattern-capped
// bounds box is placed by the same rule through ComposedTopOffset
// (cmd/json2pptx alignRelativeBounds); its type is not stepped.
const (
	// ComposeSparseFill is the share of the area under which a content-sized
	// block counts as sparse.
	ComposeSparseFill = 0.75
	// ComposeOpticalTop is the share of the spare height left above a sparse
	// block: slightly above the geometric centre, where a block reads as
	// centred under a title.
	ComposeOpticalTop = 0.45
	// composeMaxFill is the largest share of the area a stepped block may
	// take: stepping up must leave the block content-sized, not full.
	composeMaxFill = 0.85
	// composeFigureScale grows display figures (KPI values, step numerals),
	// which sit off the word steps; composeFigureMaxPt is the KPI step's top.
	composeFigureScale = 1.2
	composeFigureMaxPt = tokens.TypeScaleKPIMaxPt
	// composeHairlinePt: rows and cells capped under this are rules and
	// connector lines, which keep their thickness.
	composeHairlinePt = 4.0
	// composeFitMargin keeps stepped text a little inside its cell: the fit
	// is measured in a stand-in face.
	composeFitMargin = 0.96
)

// composeRowScales are the row-height and row-gap growths tried with a type
// step, smallest first: the first at which the stepped text fits its cells is
// taken. Text that wraps onto more lines needs the later ones; past the last,
// the block keeps its type.
var composeRowScales = []float64{1.15, 1.25, 1.35}

// ComposedTopOffset is where the policy places a sparse block of usedEMU
// inside availEMU: the optical centre, but never above anchorOffset (the body
// line's distance from the area top, 0 when unknown) while the slack allows.
func ComposedTopOffset(usedEMU, availEMU, anchorOffset int64) int64 {
	slack := availEMU - usedEMU
	if slack <= 0 {
		return 0
	}
	off := int64(float64(slack) * ComposeOpticalTop)
	if anchorOffset > off {
		off = min(anchorOffset, slack)
	}
	return off
}

// IsSparseBlock reports whether a content-sized block of usedEMU is clearly
// smaller than the availEMU it sits in.
func IsSparseBlock(usedEMU, availEMU int64) bool {
	return availEMU > 0 && usedEMU > 0 && float64(usedEMU) < ComposeSparseFill*float64(availEMU)
}

// composePlan is what Resolve applies to a sparse block.
type composePlan struct {
	// place is set when the policy places the block (ComposedTopOffset).
	place bool
	// sizes maps a rendered word size (points) to its stepped size; nil when
	// the block keeps its type.
	sizes map[float64]float64
	// worst is the largest needed/available text height seen among stepped
	// cells and brokenToken whether a stepped word no longer fits its line;
	// Resolve fills both in for the caller's fit check.
	worst       float64
	brokenToken bool
	// rowScale is the row growth the plan's grid was scaled by.
	rowScale float64
}

// composable reports whether the composition policy governs grid.
func composable(grid *Grid) bool {
	return grid != nil && grid.Compose && grid.VAlign == VAlignAuto && effectiveVAlign(grid) != VAlignStretch
}

// blockFill returns the share of the grid's height its content-sized row
// block takes (rows plus row gaps); ok is false when the rows fill the bounds.
func blockFill(grid *Grid) (fill float64, ok bool) {
	rowGapEMU := PtToEMU(effectiveRowGap(grid))
	gaps := rowGapEMU * int64(len(grid.Rows)-1)
	availH := grid.Bounds.CY - gaps
	if availH <= 0 || grid.Bounds.CY <= 0 {
		return 0, false
	}
	var sum float64
	for _, p := range resolveRowHeights(grid.Rows, availH) {
		sum += p
	}
	if sum <= 0 || sum >= 100-1e-6 {
		return 0, false
	}
	used := int64(float64(availH)*sum/100) + gaps
	return float64(used) / float64(grid.Bounds.CY), true
}

func effectiveRowGap(grid *Grid) float64 {
	if grid.RowGap != 0 {
		return grid.RowGap
	}
	if grid.DefaultGapPt > 0 {
		return grid.DefaultGapPt
	}
	return 8.0
}

// resolveComposed resolves a grid the composition policy governs.
func resolveComposed(grid *Grid, alloc *pptx.ShapeIDAllocator) (*ResolveResult, error) {
	fill, ok := blockFill(grid)
	if !ok || fill >= ComposeSparseFill {
		return resolveGrid(grid, alloc, nil)
	}
	plan := &composePlan{place: true}
	if sizes := typeStep(grid); len(sizes) > 0 {
		// The first row growth at which the stepped text fits is taken; a
		// trial resolve (throwaway shape IDs) measures each.
		for _, k := range composeRowScales {
			if fill*k > composeMaxFill {
				break
			}
			trial := &composePlan{place: true, sizes: sizes, rowScale: k}
			scaled := scaledGrid(grid, k)
			_, err := resolveGrid(scaled, pptx.NewShapeIDAllocator(nil), trial)
			if err != nil || trial.brokenToken {
				break
			}
			if trial.worst <= composeFitMargin {
				return resolveGrid(scaled, alloc, &composePlan{place: true, sizes: sizes, rowScale: k})
			}
		}
	}
	return resolveGrid(grid, alloc, plan)
}

// scaledGrid returns a copy of grid whose content-sized rows, capped cells and
// row gap are k times as tall. Hairlines keep their thickness.
func scaledGrid(grid *Grid, k float64) *Grid {
	out := *grid
	out.RowGap = effectiveRowGap(grid) * k
	out.Rows = make([]Row, len(grid.Rows))
	for i, row := range grid.Rows {
		if row.MaxHeight >= composeHairlinePt || row.MaxHeight == 0 {
			row.Height *= k
			row.MinHeight *= k
			row.MaxHeight *= k
		}
		cells := make([]Cell, len(row.Cells))
		for j, c := range row.Cells {
			if c.MaxHeight >= composeHairlinePt {
				c.MaxHeight *= k
			}
			c.BleedTop *= k
			c.InsetTop *= k
			c.InsetBottom *= k
			cells[j] = c
		}
		row.Cells = cells
		out.Rows[i] = row
	}
	return &out
}

// typeStep maps each word size the grid renders to the next step of the type
// scale. A level moves only while it stays under the level above it, so a
// block whose levels already sit on adjacent steps up to the lead step keeps
// its hierarchy (and its sizes). Cells pinned to type_scale "compact" are
// not read: they keep their text.
func typeStep(grid *Grid) map[float64]float64 {
	seen := map[float64]bool{}
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			for _, spec := range []*ShapeSpec{c.Shape, compositeText(c.Composite)} {
				if !steppable(spec, grid) {
					continue
				}
				for _, p := range textParagraphSizes(spec.Text, grid.KeepTextSizes) {
					if !p.figure {
						seen[p.pt] = true
					}
				}
			}
		}
	}
	levels := make([]float64, 0, len(seen))
	for pt := range seen {
		levels = append(levels, pt)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(levels)))
	sizes := map[float64]float64{}
	above := math.Inf(1)
	for _, pt := range levels {
		next := nextWordStep(pt)
		if next >= above {
			next = pt
		}
		if next != pt {
			sizes[pt] = next
		}
		above = next
	}
	return sizes
}

func compositeText(c *CompositeSpec) *ShapeSpec {
	if c == nil {
		return nil
	}
	return c.Text
}

// steppable reports whether spec's text takes part in the type step.
func steppable(spec *ShapeSpec, grid *Grid) bool {
	if spec == nil || len(spec.Text) == 0 {
		return false
	}
	mode := spec.TypeScale
	if mode == "" {
		mode = grid.TypeScale
	}
	return mode != "compact"
}

// nextWordStep is the type-scale step above a rendered word size, up to the
// lead step; larger text is display type and keeps its size.
func nextWordStep(pt float64) float64 {
	switch {
	case pt < tokens.TypeScaleSubheadPt:
		return tokens.TypeScaleSubheadPt
	case pt < tokens.TypeScaleLeadPt:
		return tokens.TypeScaleLeadPt
	}
	return pt
}

// steppedSize returns the size a paragraph rendered at pt takes under sizes.
func steppedSize(pt float64, figure bool, sizes map[float64]float64) float64 {
	if figure {
		return math.Max(pt, math.Min(math.Floor(pt*composeFigureScale), composeFigureMaxPt))
	}
	if next, ok := sizes[pt]; ok {
		return next
	}
	return pt
}

type paragraphSize struct {
	pt     float64 // the size the paragraph renders at
	figure bool    // a display figure: scaled, not stepped
}

// renderedSizePt is the size authored text is written at: the default when
// unset, settled onto the type scale, raised to the renderer's floor.
func renderedSizePt(size float64, content string, keepSizes bool) paragraphSize {
	if size <= 0 {
		size = DefaultTextSizePt
	}
	figure := size >= tokens.TypeScaleLeadPt && isDisplayFigure(content)
	if !figure && !keepSizes {
		size = float64(tokens.SnapTextHPt(int(math.Round(size*100)))) / 100
	}
	return paragraphSize{pt: EffectiveTextSizePt(size), figure: figure}
}

// textParagraphSizes lists the rendered size of every non-empty paragraph of
// a shape's text (string, object or paragraphs form).
func textParagraphSizes(raw json.RawMessage, keepSizes bool) []paragraphSize {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return []paragraphSize{renderedSizePt(0, s, keepSizes)}
	}
	var obj struct {
		Content    string  `json:"content"`
		Size       float64 `json:"size"`
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	if len(obj.Paragraphs) == 0 {
		if strings.TrimSpace(obj.Content) == "" {
			return nil
		}
		return []paragraphSize{renderedSizePt(obj.Size, obj.Content, keepSizes)}
	}
	out := make([]paragraphSize, 0, len(obj.Paragraphs))
	for _, p := range obj.Paragraphs {
		if strings.TrimSpace(p.Content) != "" {
			out = append(out, renderedSizePt(p.Size, p.Content, keepSizes))
		}
	}
	return out
}

// stepShapeText returns a private copy of spec with every paragraph at its
// stepped size; spec itself when nothing moves. Like growShapeText, the copy
// is what both OOXML generation and preflight read.
func stepShapeText(spec *ShapeSpec, sizes map[float64]float64, keepSizes bool) *ShapeSpec {
	step := func(size float64, content string) (float64, bool) {
		if strings.TrimSpace(content) == "" {
			return size, false
		}
		r := renderedSizePt(size, content, keepSizes)
		next := steppedSize(r.pt, r.figure, sizes)
		return next, next != r.pt
	}
	var s string
	var out json.RawMessage
	if json.Unmarshal(spec.Text, &s) == nil {
		next, ok := step(0, s)
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
				next, ok := step(size, content)
				if !ok {
					continue
				}
				changed = true
				defs[i]["size"], _ = json.Marshal(next)
				// A suffix run (a unit beside its figure) keeps its proportion.
				if json.Unmarshal(defs[i]["suffix_size"], &suffixSize) == nil && suffixSize > 0 && size > 0 {
					defs[i]["suffix_size"], _ = json.Marshal(math.Floor(suffixSize * next / size))
				}
			}
			obj["paragraphs"], _ = json.Marshal(defs)
		} else {
			var size float64
			var content string
			_ = json.Unmarshal(obj["size"], &size)
			_ = json.Unmarshal(obj["content"], &content)
			next, ok := step(size, content)
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

// stepFit measures stepped text against the cell it is written in and records
// the result on plan: the needed share of the text rectangle's height (a cell
// whose text did not fit before the step is held to no worse than before),
// and whether a word that fit its line no longer does.
func stepFit(plan *composePlan, before, after *ShapeSpec, bounds pptx.RectEmu, overlay [4]int64) {
	if before == after {
		return
	}
	tbAfter, err := ResolveTextInput(after.Text)
	if err != nil {
		return
	}
	tbBefore, err := ResolveTextInput(before.Text)
	if err != nil {
		return
	}
	pw, ph := pptx.PresetTextRect(after.Geometry, after.Adjustments, bounds)
	width, height := scaledTextRect(pptx.RectEmu{CX: pw, CY: ph}, tbAfter, overlay)
	if tbAfter.Vert != "" {
		width, height = height, width
	}
	if width <= 0 || height <= 0 {
		return
	}
	parasAfter, parasBefore := scaleParagraphs(tbAfter), scaleParagraphs(tbBefore)
	if len(parasAfter) == 0 || len(parasAfter) != len(parasBefore) {
		return
	}
	for i, p := range parasAfter {
		line := float64(width - p.marginL)
		toks := strings.Fields(p.text)
		if p.role == "kpi-value" {
			toks = []string{strings.TrimSpace(p.text)} // a value stays with its unit
		}
		for _, tok := range toks {
			w0, err0 := textfit.MeasureStyledLineWidth(tok, "Liberation Sans", parasBefore[i].fontPt, p.bold)
			w1, err1 := textfit.MeasureStyledLineWidth(tok, "Liberation Sans", p.fontPt, p.bold)
			if err0 != nil || err1 != nil {
				continue
			}
			if float64(w0) <= line && float64(w1) > line*textfit.AtomicTokenWidthPct/100 {
				plan.brokenToken = true
				return
			}
		}
	}
	availPt := float64(height) / 12700
	need := blockHeightPt(parasAfter, width) / availPt
	if need > 1 {
		// Text that did not fit its cell at the pattern's own size is the
		// pattern's business; the step must not make it worse.
		k := math.Max(plan.rowScale, 1)
		if was := blockHeightPt(parasBefore, width) / (availPt / k); was > 1 {
			need /= was
		}
	}
	plan.worst = math.Max(plan.worst, need)
}

// blockHeightPt is the height paras wrap to in width, measured as
// scaleBlockFits measures it.
func blockHeightPt(paras []scaleParagraph, width int64) float64 {
	height := 0.0
	for _, p := range paras {
		usable := width - p.marginL
		if usable <= 0 {
			return math.Inf(1)
		}
		m, err := textfit.MeasureRun(p.text, "Liberation Sans", p.fontPt, usable, 0)
		if err != nil {
			return math.Inf(1)
		}
		height += float64(m.Lines)*p.fontPt*textLineHeightFactor + p.spaceAfter
	}
	return height
}
