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
	// is measured in a stand-in face, or in the template's own by a measurer
	// whose line breaks a renderer need not share.
	composeFitMargin = 0.96
	// ComposeLabelMaxWords: a paragraph of at most this many words is a
	// label, which the step must not wrap onto a second line.
	ComposeLabelMaxWords = 3
	// composeGrowFill is the share of the area a lone row grows towards
	// (Grid.ComposeGrow) and composeGrowMax how far past the height its
	// content needs. A filled box may take 1.6x before it reads as an empty
	// panel (go-slide-creator-wntyw, patterns.contentStretchMax); the grown
	// row's cells are unpainted, so its surplus is air between hairlines, and
	// 1.8x is what a row of six one-line KPIs needs to leave under a third of
	// a tall content area beneath it on every shipped template.
	composeGrowFill = 0.5
	composeGrowMax  = 1.8
	// composeBandTarget is the share of the area a band-scaled block
	// (Grid.ComposeBand) is grown to leave under it: clear of the lower
	// third a slide may not leave empty. composeBandMaxScale is how far its
	// rows may grow for that — the 1.6x a filled box takes before it reads
	// as an empty panel (go-slide-creator-wntyw). A block that would need
	// more keeps its height: it is a strip, and is reported as one.
	composeBandTarget   = 0.28
	composeBandMaxScale = 1.6
	// composeBandCardAspect is how tall a text box that may grow into a card
	// (Grid.ComposeBandSquare) gets relative to its width: a 4:5 portrait
	// card. A box holding a sentence reads as a card up to there and as a
	// slab past it.
	composeBandCardAspect = 1.25
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
	chosen, plan := grid, &composePlan{place: true}
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
				chosen, plan = scaled, &composePlan{place: true, sizes: sizes, rowScale: k}
				break
			}
		}
	}
	chosen, plan = bandScaled(grid, chosen, plan, fill)
	return resolveGrid(grownLoneRow(chosen, plan), alloc, plan)
}

// bandScaled returns the grid and plan of a sparse block whose rows are grown
// until the block, set at the optical centre, leaves composeBandTarget of the
// area under it (Grid.ComposeBand). chosen and plan are what the type step
// settled on and fill the share of the area the unscaled block takes. The
// block keeps them when it is already that tall, when it would need more
// than its limit (bandMaxScale), or when the taller rows do not resolve. The
// taller rows are tried with the type step first: a step the block had no
// room for at composeRowScales often fits them.
func bandScaled(grid, chosen *Grid, plan *composePlan, fill float64) (*Grid, *composePlan) {
	if !grid.ComposeBand || fill <= 0 {
		return chosen, plan
	}
	k := (1 - composeBandTarget/(1-ComposeOpticalTop)) / fill
	if k <= math.Max(plan.rowScale, 1) || fill*k > composeMaxFill || k > bandMaxScale(grid) {
		return chosen, plan
	}
	scaled := scaledGrid(grid, k)
	for _, sizes := range []map[float64]float64{typeStep(grid), plan.sizes} {
		trial := &composePlan{place: true, sizes: sizes, rowScale: k}
		if _, err := resolveGrid(scaled, pptx.NewShapeIDAllocator(nil), trial); err == nil && !trial.brokenToken && trial.worst <= composeFitMargin {
			return scaled, &composePlan{place: true, sizes: sizes, rowScale: k}
		}
	}
	return chosen, plan
}

// bandMaxScale is how far the rows of a band-scaled grid may grow:
// composeBandMaxScale, or for a grid of text boxes that may become cards
// (Grid.ComposeBandSquare) the growth at which its narrowest box is
// composeBandCardAspect times as tall as it is wide, when that is more.
func bandMaxScale(grid *Grid) float64 {
	limit := composeBandMaxScale
	if !grid.ComposeBandSquare {
		return limit
	}
	res, err := resolveGrid(grid, pptx.NewShapeIDAllocator(nil), &composePlan{place: true})
	if err != nil || res == nil {
		return limit
	}
	square := math.Inf(1)
	for _, cell := range res.Cells {
		if cell.Kind != CellKindShape || cell.ShapeSpec == nil || !hasNonEmptyText(cell.ShapeSpec.Text) || cell.Bounds.CY <= 0 {
			continue
		}
		square = math.Min(square, composeBandCardAspect*float64(cell.Bounds.CX)/float64(cell.Bounds.CY))
	}
	if math.IsInf(square, 1) {
		return limit
	}
	return math.Max(limit, square)
}

// grownLoneRow returns grid with its one content-sized row grown into the
// free height (Grid.ComposeGrow): to composeGrowFill of the area, and at most
// composeGrowMax times the height its content needs. The cells keep their
// shapes at the content's height, so the values still share a baseline; the
// shapes sit where the row's tallest text block is centred in the taller row.
// What grows with the row is what the cell draws around its shape — the
// hairline dividers of an open KPI strip. Any other grid is returned as
// given. plan is the plan the grid resolves under.
func grownLoneRow(grid *Grid, plan *composePlan) *Grid {
	if !grid.ComposeGrow || len(grid.Rows) != 1 {
		return grid
	}
	row := grid.Rows[0]
	if row.MaxHeight < composeHairlinePt || row.Height > 0 || row.AutoHeight {
		return grid
	}
	target := math.Min(row.MaxHeight*composeGrowMax, composeGrowFill*float64(grid.Bounds.CY)/12700.0)
	extra := math.Floor(target - row.MaxHeight)
	if extra < 2 {
		return grid
	}
	top := math.Min(extra, (extra+loneRowTextSlackPt(grid, plan))/2)
	out := *grid
	row.MaxHeight += extra
	if row.MinHeight > 0 {
		row.MinHeight += extra
	}
	cells := make([]Cell, len(row.Cells))
	for i, c := range row.Cells {
		c.InsetTop += top
		c.InsetBottom += extra - top
		cells[i] = c
	}
	row.Cells = cells
	out.Rows = []Row{row}
	return &out
}

// loneRowTextSlackPt is how much further the tallest text block of the row
// sits from the bottom of its shape than from the top (points): the text is
// top-anchored, so the air a row keeps for its longest caption falls under
// the shorter ones. Zero when no cell's text can be measured.
func loneRowTextSlackPt(grid *Grid, plan *composePlan) float64 {
	trial := *plan
	res, err := resolveGrid(grid, pptx.NewShapeIDAllocator(nil), &trial)
	if err != nil || res == nil {
		return 0
	}
	slack, found := 0.0, false
	for _, cell := range res.Cells {
		if cell.Kind != CellKindShape || cell.ShapeSpec == nil {
			continue
		}
		tb, err := ResolveTextInput(cell.ShapeSpec.Text)
		if err != nil || tb == nil {
			continue
		}
		body := *tb
		for i := range body.Insets {
			body.Insets[i] += cell.TextInsets[i]
		}
		insets := pptx.EffectiveTextInsets(&body, cell.Bounds)
		paras := scaleParagraphs(tb, cell.ShapeSpec.ThemeFonts)
		if len(paras) == 0 {
			continue
		}
		block := blockHeightPt(paras, cell.Bounds.CX-insets[0]-insets[2])
		if math.IsInf(block, 1) {
			continue
		}
		s := float64(cell.Bounds.CY-insets[1]-insets[3])/12700.0 - block + float64(insets[3]-insets[1])/12700.0
		if !found || s < slack {
			slack, found = s, true
		}
	}
	return math.Max(slack, 0)
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
func renderedSizePt(size float64, content string, keepSizes, marked bool) paragraphSize {
	if size <= 0 {
		size = DefaultTextSizePt
	}
	figure := (size >= tokens.TypeScaleLeadPt && isDisplayFigure(content)) || keepsFigureSize(size, marked)
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
		return []paragraphSize{renderedSizePt(0, s, keepSizes, false)}
	}
	var obj struct {
		Content    string  `json:"content"`
		Size       float64 `json:"size"`
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
			Figure  bool    `json:"figure"`
		} `json:"paragraphs"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	if len(obj.Paragraphs) == 0 {
		if strings.TrimSpace(obj.Content) == "" {
			return nil
		}
		return []paragraphSize{renderedSizePt(obj.Size, obj.Content, keepSizes, false)}
	}
	out := make([]paragraphSize, 0, len(obj.Paragraphs))
	for _, p := range obj.Paragraphs {
		if strings.TrimSpace(p.Content) != "" {
			out = append(out, renderedSizePt(p.Size, p.Content, keepSizes, p.Figure))
		}
	}
	return out
}

// stepShapeText returns a private copy of spec with every paragraph at its
// stepped size; spec itself when nothing moves. Like growShapeText, the copy
// is what both OOXML generation and preflight read.
func stepShapeText(spec *ShapeSpec, sizes map[float64]float64, keepSizes bool) *ShapeSpec {
	step := func(size float64, content string, marked bool) (float64, bool) {
		if strings.TrimSpace(content) == "" {
			return size, false
		}
		r := renderedSizePt(size, content, keepSizes, marked)
		next := steppedSize(r.pt, r.figure, sizes)
		return next, next != r.pt
	}
	var s string
	var out json.RawMessage
	if json.Unmarshal(spec.Text, &s) == nil {
		next, ok := step(0, s, false)
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
				var marked bool
				_ = json.Unmarshal(defs[i]["size"], &size)
				_ = json.Unmarshal(defs[i]["content"], &content)
				_ = json.Unmarshal(defs[i]["figure"], &marked)
				next, ok := step(size, content, marked)
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
			next, ok := step(size, content, false)
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
// and whether a word that fit its line no longer does. Both are measured in
// the face the paragraph renders in where the measurer has it
// (pptx.ParagraphFitFace): a stand-in that runs wider than the template face
// neither sees a value that sat on its line as designed nor leaves it there
// (go-slide-creator-5x4w4).
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
	parasAfter, parasBefore := scaleParagraphs(tbAfter, after.ThemeFonts), scaleParagraphs(tbBefore, before.ThemeFonts)
	if len(parasAfter) == 0 || len(parasAfter) != len(parasBefore) {
		return
	}
	for i, p := range parasAfter {
		line := float64(width - p.marginL)
		toks := strings.Fields(p.text)
		if p.role == "kpi-value" {
			toks = []string{strings.TrimSpace(p.text)} // a value stays with its unit
		}
		// A short label (a KPI caption, a stop name) that sat on one line
		// stays on one line: "Logo churn (SMB-" over "weighted)" at the
		// stepped size reads worse than the label whole at its own size.
		if text := strings.TrimSpace(p.text); len(strings.Fields(text)) <= ComposeLabelMaxWords && p.role != "kpi-value" {
			w0, ok0 := p.tokenWidth(text, parasBefore[i].fontPt)
			w1, ok1 := p.tokenWidth(text, p.fontPt)
			if ok0 && ok1 && w0 <= line && w1 > line {
				plan.brokenToken = true
				return
			}
		}
		for _, tok := range toks {
			w0, ok0 := p.tokenWidth(tok, parasBefore[i].fontPt)
			w1, ok1 := p.tokenWidth(tok, p.fontPt)
			if !ok0 || !ok1 {
				continue
			}
			if w0 <= line && w1 > line*p.tokenShare() {
				plan.brokenToken = true
				return
			}
		}
	}
	plan.worst = math.Max(plan.worst, steppedHeightNeed(parasBefore, parasAfter, width, float64(height)/12700, plan.rowScale))
}

// steppedHeightNeed is the share of a text rectangle availPt tall that
// stepped text needs at width. A label that fits on one line within
// RenderFaceSlack of the width is counted at the lines a renderer's wider
// face wraps it to (slackLabelHeightPt). Text that did not fit its cell at
// the pattern's own size is the pattern's business: its need is taken
// relative to what it needed before the step, in the rows as they were
// before rowScale grew them, so the step must only not make it worse.
func steppedHeightNeed(parasBefore, parasAfter []scaleParagraph, width int64, availPt, rowScale float64) float64 {
	need := blockHeightPt(parasAfter, width) / availPt
	if slack := slackLabelHeightPt(parasAfter, width) / availPt; slack > need && need <= 1 {
		need = slack
	}
	if need > 1 {
		k := math.Max(rowScale, 1)
		if was := blockHeightPt(parasBefore, width) / (availPt / k); was > 1 {
			need /= was
		}
	}
	return need
}

// RenderFaceSlack is how much wider a renderer's face may set a line than the
// face it was measured in. A label written on one line of a box one line
// tall, within this margin of the box's width, wraps in such a renderer and
// is shrunk by its autofit while its shorter siblings are not: the row reads
// at two sizes (SIBLING_SIZE_MISMATCH measures with the same margin). What
// sizes a row for a label that close to its box gives it room for the second
// line instead.
const RenderFaceSlack = 1.12

// slackLabelHeightPt is the height a one-paragraph label that sits on one
// line of width needs once a renderer's face runs RenderFaceSlack wider: the
// height of the lines it wraps to at the narrowed width. Zero for text of
// several paragraphs or one that already wraps — a wrapped paragraph has the
// lines the engine measured for it (go-slide-creator-bhoo3: the type step set
// "Always-on client portal with live exposures" at 18pt across 95% of a box
// one line tall).
func slackLabelHeightPt(paras []scaleParagraph, width int64) float64 {
	if len(paras) != 1 {
		return 0
	}
	p := paras[0]
	usable := width - p.marginL
	if usable <= 0 {
		return 0
	}
	if m, err := textfit.MeasureRun(p.text, p.face, p.fontPt, usable, 0); err != nil || m.Lines != 1 {
		return 0
	}
	return blockHeightPt(paras, p.marginL+int64(float64(usable)/RenderFaceSlack))
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
		m, err := textfit.MeasureRun(p.text, p.face, p.fontPt, usable, 0)
		if err != nil {
			return math.Inf(1)
		}
		height += float64(m.Lines)*p.fontPt*textLineHeightFactor + p.spaceAfter
	}
	return height
}
