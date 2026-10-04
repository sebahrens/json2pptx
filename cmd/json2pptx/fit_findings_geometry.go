package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"image/color"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// Deterministic geometry findings (go-slide-creator-t5gw): TEXT_EXCEEDS_SHAPE,
// SPARSE_FILL and SLIDE_UNDERUSED. They run on the resolved shape_grid
// geometry (the same resolver generation uses) — including the grids named
// patterns expand to — and measure text with real font metrics, so a slide
// that "validates clean" but renders with clipped chevron labels, huge empty
// boxes or 70% blank space no longer scores as perfect.

const (
	// sparseFillMinShapeSlideFrac: only filled shapes larger than this share
	// of the slide area are checked for SPARSE_FILL.
	sparseFillMinShapeSlideFrac = 0.10
	// sparseFillMaxTextFrac: SPARSE_FILL fires when the estimated text block
	// covers less than this share of the filled shape.
	sparseFillMaxTextFrac = 0.20
	// slideUnderusedMaxFrac: SLIDE_UNDERUSED fires when the content ink
	// bounding box covers less than this share of the safe content area. It
	// applies to a band the AUTHOR capped (bounds / max_height_pct): they chose
	// the height, so "the cap is too tight" is advice they can act on.
	slideUnderusedMaxFrac = 0.45
	// Pattern-owned bands are measured by their actual ink, not the enclosing
	// grid bounds. A 29% threshold catches sparse cards and empty columns while
	// leaving deliberately compact but well-populated bands alone.
	slideUnderusedPatternMaxFrac = 0.29
	// slideUnderusedBoxPatternMaxFrac is the threshold for patterns whose
	// cards / panels are content-sized and middle-anchored by policy
	// (contentSizedBoxPatterns): a row of four KPI cards at their natural
	// ~1.6x-content height covers ~25% of the zone, and stretching them
	// to reach 29% is exactly the empty-box look go-slide-creator-wntyw
	// removed. The finding keeps firing for a genuinely sparse box slide.
	slideUnderusedBoxPatternMaxFrac = 0.20
	// slideUnderusedStripPatternMaxFrac is the threshold for content-sized
	// time-line strips hung from the body line (contentSizedStripPatterns).
	slideUnderusedStripPatternMaxFrac = 0.22
	// slideUnderusedHeroPatternMaxFrac is the threshold for a single hero
	// statement (heroStatementPatterns): a figure and its label cover 17-25%
	// of the zone at their designed sizes.
	slideUnderusedHeroPatternMaxFrac = 0.15
	// textExceedsTolerance absorbs rounding/kerning noise before flagging.
	textExceedsTolerance = 1.02

	emuPerPt           = 12700.0
	shapeDefaultTextPt = 14.0 // shapegrid defaultTextSizeHPt
	geometryLineHeight = tokens.LineHeight
	mmToPt             = 72.0 / 25.4
	fallbackMeasureFnt = "Arial"
)

// inlineTagRe strips the inline markup ConvertMarkdownEmphasis leaves in
// pattern-generated text (e.g. <b>…</b>) before measuring.
var inlineTagRe = regexp.MustCompile(`<[^>]+>`)

// geomParagraph is one measured paragraph of shape text.
type geomParagraph struct {
	text   string
	sizePt float64
	bold   bool
}

// geomText is the parsed text of a shape cell.
type geomText struct {
	paragraphs []geomParagraph
	insets     [4]int64 // written L,T,R,B in EMU before the degenerate-shape clamp
	// body is the resolved text body the writer emits; its insets and the
	// clamp (pptx.EffectiveTextInsets) are what the text really gets.
	body   *pptx.TextBody
	align  string
	vAlign string
	// vert is the OOXML text-direction ("vert270" for bottom-to-top). Rotated
	// text runs along the shape's HEIGHT, so the axes swap for every
	// measurement below. Without this a thin band with a rotated label — the
	// conventional way to draw a cross-cutting concern — measured its label
	// against the band's 20pt width and reported TEXT_EXCEEDS_SHAPE
	// (go-slide-creator-pr3g).
	vert string
}

// rotated reports whether the text runs along the shape's height rather than
// its width.
func (t geomText) rotated() bool {
	return strings.HasPrefix(t.vert, "vert")
}

// collectGeometryFindings walks every slide's resolved grid geometry and
// emits TEXT_EXCEEDS_SHAPE, SPARSE_FILL and SLIDE_UNDERUSED findings. Shape
// findings are aggregated to one per code per slide (params.cells lists every
// offending cell path) so a row of identical cards does not flood the budget.
func collectGeometryFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) []patterns.FitFinding {
	return collectGeometry(input, layouts, slideWidth, slideHeight, theme, nil)
}

// collectGeometry is collectGeometryFindings; usage, when set, is told how
// much of its content area every measured slide covers and the share
// SLIDE_UNDERUSED holds it to (TestExemplarsClearTheUnderusedThreshold).
func collectGeometry(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo, usage func(slideUsage)) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}
	m := newGeomMeasurer(theme)
	rhythmGrid := resolvedValidRhythmGrid(input, layouts, slideWidth, slideHeight)
	sectionIndices := slideSectionIndices(input.Slides, layouts)
	var findings []patterns.FitFinding
	for si := range input.Slides {
		slide := input.Slides[si]
		grid := slide.ShapeGrid
		basePath := slidepath.ShapeGrid(si)
		patternName := ""
		if slide.Pattern != nil {
			patternName = slide.Pattern.Name
		}
		if grid == nil {
			grid = expandSlidePatternGrid(&slide, si, slideWidth, slideHeight, theme)
			if grid == nil {
				continue
			}
			basePath = slidepath.SlideField(si, "pattern")
			slide.ShapeGrid = grid
		}
		geom := resolveGridGeometry(slide, layouts, slideWidth, slideHeight)
		result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
		if result == nil {
			continue
		}
		acc := &geomAccumulator{m: m, slideArea: slideWidth * slideHeight, slideWidth: slideWidth, slideHeight: slideHeight, slotInk: openColumnPatterns[patternName]}
		acc.walk(grid, result, basePath, 0)
		explicitPatternBounds := slide.Pattern != nil && slide.Pattern.Bounds != nil
		findings = append(findings, acc.findings(patternName, explicitPatternBounds)...)
		if f := acc.narrowWrapFinding(patternName, &input.Slides[si], si); f != nil {
			findings = append(findings, *f)
		}
		safe := contentRelativeBoundsBase(geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
		findings = append(findings, siblingSizeFindings(m, grid, result, basePath, patternName, slideWidth, slideHeight)...)
		// Sparse overall (SLIDE_UNDERUSED, an airiness advisory) and lopsided
		// are different facts: a centred hero number is sparse and balanced,
		// a row of cards over an empty bottom half is lopsided whether or not
		// it is sparse. One imbalance finding per slide: top-to-bottom, else
		// left-to-right.
		if f := checkSlideUnderused(acc.ink, safe, &slide, si, patternName, acc.heightSensitiveOverflow()); f != nil {
			findings = append(findings, *f)
		}
		if usage != nil {
			if u, ok := measureSlideUsage(acc.ink, safe, &slide, patternName); ok {
				u.slide = si
				usage(u)
			}
		}
		ctx := balanceContext{input: input, layouts: layouts, slideWidth: slideWidth, slideHeight: slideHeight, theme: theme, rhythmGrid: rhythmGrid, sectionIndices: sectionIndices}
		if f := ctx.imbalanceFinding(acc, slide, si, grid, geom, basePath, patternName, safe); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// balanceContext carries the deck-level inputs of the imbalance checks.
type balanceContext struct {
	input                   *PresentationInput
	layouts                 []types.LayoutMetadata
	slideWidth, slideHeight int64
	theme                   *types.ThemeInfo
	rhythmGrid              *resolvedGrid
	sectionIndices          []int
}

// imbalanceFinding returns the slide's one imbalance finding: top-to-bottom,
// else left-to-right.
//
// A pattern nested in a cell (a DeckSpec regions slide's kpis or timeline) has
// no shapes until generation expands it, so its cell reads as empty. The
// balance is measured on the expanded cells, and not at all when they cannot
// be expanded: an empty band that is really a KPI row is not a finding.
func (c balanceContext) imbalanceFinding(acc *geomAccumulator, slide SlideInput, si int, grid *ShapeGridInput, geom GridGeometry, basePath, patternName string, safe pptx.RectEmu) *patterns.FitFinding {
	balance := acc
	if hasNestedCellPattern(grid) {
		balance = nil
		g, contentBounds := patternExpansionGeometry(slide, c.layouts, c.slideWidth, c.slideHeight, c.rhythmGrid)
		sectionIdx := 0
		if si < len(c.sectionIndices) {
			sectionIdx = c.sectionIndices[si]
		}
		expanded, _, err := expandNestedPatternsForReadability(grid, basePath, nestedExpansionGeometry{
			geom: g, contentBounds: contentBounds, slideWidth: c.slideWidth, slideHeight: c.slideHeight,
			theme: c.theme, strategy: patterns.AccentStrategy(c.input.AccentStrategy), slideIdx: si, sectionIdx: sectionIdx,
		})
		if err == nil && expanded != nil && !hasNestedCellPattern(expanded) {
			if res := resolveGridForStructural(expanded, geom.OverrideBounds, geom.Zone, c.slideWidth, c.slideHeight); res != nil {
				balance = &geomAccumulator{m: acc.m, slideArea: c.slideWidth * c.slideHeight, slideWidth: c.slideWidth, slideHeight: c.slideHeight, slotInk: openColumnPatterns[patternName]}
				balance.walk(expanded, res, basePath, 0)
			}
		}
	}
	if balance == nil {
		return nil
	}
	if f := checkVerticalImbalance(balance.ink, safe, &slide, si, patternName, zoneBodyTop(geom.Zone), true); f != nil {
		return f
	}
	return checkHorizontalImbalance(balance.textInk, safe, &slide, si, patternName)
}

// maxGeomNestingDepth bounds recursion into nested sub-grids.
const maxGeomNestingDepth = 4

// textExceedsHit / sparseFillHit record one offending cell.
type textExceedsHit struct {
	path            string
	word, geometry  string
	wordPt, availPt float64
	minGlyphPt      float64
}

type sparseFillHit struct {
	path               string
	textFrac, areaFrac float64
}

// geomAccumulator walks a resolved grid (recursing into nested sub-grids the
// way renderNestedSubGrids does) and gathers per-cell measurements.
type geomAccumulator struct {
	m                       *geomMeasurer
	slideArea               int64
	slideWidth, slideHeight int64
	ink                     []pptx.RectEmu
	// slotInk counts an unfilled text cell (and a standalone icon) by its
	// content-sized grid slot instead of its glyph block (openColumnPatterns).
	slotInk bool
	exceeds []textExceedsHit
	sparse  []sparseFillHit
	// narrow lists the boxes whose text wraps into a tall column of very
	// short lines (TEXT_WRAPS_NARROW).
	narrow []narrowWrapHit
	// gridCells are the resolved cells of the grid being walked: the siblings
	// of a narrow box say how many boxes its row holds.
	gridCells []shapegrid.ResolvedCell
	// textInk is ink with every unfilled text cell at its measured text block
	// (never its slot): where the eye finds content left to right. An open
	// column counts as its slot for coverage, but a slot whose text ends
	// mid-slide still leaves the far side empty (HORIZONTAL_IMBALANCE).
	textInk []pptx.RectEmu
	// openCells is set while walking an open KPI strip: its unpainted cells
	// are delimited by hairline dividers and count as content like the tiles
	// they replace, so a content-sized KPI row does not start reading as an
	// underused slide because it lost its card fills
	// (go-slide-creator-8zles).
	openCells bool
	// ruledCell is set while measuring an unfilled text cell that its
	// pattern stands beside an accent rule (ruledColumnPatterns): the cell
	// counts as its slot.
	ruledCell bool
}

// addInk records a rectangle that is content in both views: coverage (ink)
// and left-to-right position (textInk).
func (a *geomAccumulator) addInk(r pptx.RectEmu) {
	a.ink = append(a.ink, r)
	a.textInk = append(a.textInk, r)
}

// isKPIStripGrid reports whether grid is the expansion of a KPI row pattern.
func isKPIStripGrid(grid *ShapeGridInput) bool {
	return grid != nil && strings.HasPrefix(grid.Source, patternSourcePrefix+"kpi-")
}

func (a *geomAccumulator) walk(input *ShapeGridInput, result *shapegrid.ResolveResult, basePath string, depth int) {
	outerOpen, outerCells := a.openCells, a.gridCells
	a.openCells, a.gridCells = isKPIStripGrid(input), result.Cells
	defer func() { a.openCells, a.gridCells = outerOpen, outerCells }()
	// A grid a named pattern expanded says so (compose segments are stamped
	// per segment): its own pattern decides how its cells count, and the
	// enclosing grid's rule is restored once it has been walked.
	if input != nil && strings.HasPrefix(input.Source, patternSourcePrefix) {
		outer := a.slotInk
		a.slotInk = openColumnPatterns[strings.TrimPrefix(input.Source, patternSourcePrefix)]
		defer func() { a.slotInk = outer }()
	}
	ruled := input != nil && ruledColumnPatterns[strings.TrimPrefix(input.Source, patternSourcePrefix)]
	for _, cell := range result.Cells {
		cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", basePath, cell.RowIdx, cell.ColIdx)
		switch cell.Kind {
		case shapegrid.CellKindShape:
			a.ruledCell = false
			if ruled && cell.RowIdx >= 0 && cell.RowIdx < len(input.Rows) {
				src := gridCellAtResolved(input, cell.RowIdx, cell.ColIdx)
				a.ruledCell = src != nil && src.AccentBar != nil
			}
			a.shapeCell(cell, cellPath)
			a.ruledCell = false
		case shapegrid.CellKindIcon:
			a.addInk(a.slotOr(cell, cell.Bounds))
		case shapegrid.CellKindTable, shapegrid.CellKindImage, shapegrid.CellKindDiagram, shapegrid.CellKindComposite:
			a.addInk(cell.Bounds)
		case shapegrid.CellKindSubGrid:
			a.subGrid(input, cell, cellPath, depth)
		}
	}
	for _, ab := range result.AccentBars {
		a.addInk(ab.Bounds)
	}
}

// subGrid resolves a nested grid inside its placeholder bounds (same inset as
// renderNestedSubGrids) and walks it; when it cannot be resolved the
// placeholder rectangle counts as ink so SLIDE_UNDERUSED stays conservative.
func (a *geomAccumulator) subGrid(input *ShapeGridInput, cell shapegrid.ResolvedCell, cellPath string, depth int) {
	var src *GridCellInput
	if input != nil && cell.RowIdx >= 0 && cell.RowIdx < len(input.Rows) {
		src = gridCellAtResolved(input, cell.RowIdx, cell.ColIdx)
	}
	if src == nil || src.Grid == nil || depth >= maxGeomNestingDepth {
		a.addInk(cell.Bounds)
		return
	}
	inset := pptx.RectEmu{X: cell.Bounds.X + subGridInsetEMU, Y: cell.Bounds.Y + subGridInsetEMU, CX: cell.Bounds.CX - 2*subGridInsetEMU, CY: cell.Bounds.CY - 2*subGridInsetEMU}
	if inset.CX <= 0 || inset.CY <= 0 {
		inset = cell.Bounds
	}
	sub := resolveGridForStructural(src.Grid, &inset, nil, a.slideWidth, a.slideHeight)
	if sub == nil {
		a.addInk(cell.Bounds)
		return
	}
	a.walk(src.Grid, sub, cellPath+"/grid", depth+1)
}

func (a *geomAccumulator) shapeCell(cell shapegrid.ResolvedCell, cellPath string) {
	if cell.ShapeSpec == nil {
		return
	}
	txt := parseGeomText(cell.ShapeSpec.Text)
	filled := shapeIsFilled(cell.ShapeSpec.Fill)
	if len(txt.paragraphs) == 0 {
		// An explicitly empty text payload in a filled card is not content.
		// A fill-only shape with no text field can still be deliberate chrome.
		if filled && len(cell.ShapeSpec.Text) > 0 {
			shapeArea := float64(cell.Bounds.CX) * float64(cell.Bounds.CY)
			if a.slideArea > 0 && shapeArea > float64(a.slideArea)*sparseFillMinShapeSlideFrac {
				a.sparse = append(a.sparse, sparseFillHit{path: cellPath + "/shape", textFrac: 0, areaFrac: shapeArea / float64(a.slideArea)})
			}
			return
		}
		if filled {
			a.addInk(cell.Bounds)
		}
		return
	}
	if filled {
		a.addInk(cell.Bounds)
	}
	// Rotated text runs along the shape's height: the line length it has is the
	// box's height, not its width.
	tw, th := pptx.PresetTextRect(cell.ShapeSpec.Geometry, cell.ShapeSpec.Adjustments, cell.Bounds)
	in := txt.writtenInsets(pptx.RectEmu{CX: tw, CY: th}, cell.TextInsets)
	availW := geometryTextWidthEMU(cell.ShapeSpec, cell.Bounds) - in[0] - in[2]
	if txt.rotated() {
		availW = cell.Bounds.CY - in[1] - in[3]
	}
	availPt := math.Max(float64(availW)/emuPerPt, 0)
	// A word the writer shrinks onto one line (pptx.WordShrinkRescues) is
	// written whole, not broken (go-slide-creator-v74wv).
	if word, wordPt, minGlyphPt := a.m.widestWord(txt); word != "" && wordPt > availPt*textExceedsTolerance &&
		!pptx.WordShrinkRescues(txt.body, wordPt, availPt) {
		geometry := cell.ShapeSpec.Geometry
		if geometry == "" {
			geometry = "rect"
		}
		a.exceeds = append(a.exceeds, textExceedsHit{path: cellPath + "/shape/text", word: word, geometry: geometry, wordPt: wordPt, availPt: availPt, minGlyphPt: minGlyphPt})
	}
	if !txt.rotated() {
		a.noteNarrowWrap(cellPath+"/shape/text", txt, availPt, cell)
	}
	blockW, blockH := a.m.textBlockPt(txt, math.Max(availPt, 1))
	if txt.rotated() {
		// The block was measured along the text's own axis; on the slide it
		// occupies the transposed rectangle.
		blockW, blockH = blockH, blockW
	}
	if !filled {
		block := placeTextBlock(cell.Bounds, txt, blockW, blockH)
		a.textInk = append(a.textInk, block)
		if a.openCells {
			a.addInk(cell.Bounds)
			return
		}
		if a.ruledCell && cell.CellBounds.CX > 0 && cell.CellBounds.CY > 0 {
			a.ink = append(a.ink, cell.CellBounds)
			return
		}
		a.ink = append(a.ink, a.slotOr(cell, block))
		return
	}
	shapeArea := float64(cell.Bounds.CX) * float64(cell.Bounds.CY)
	switch pptx.PresetGeometry(cell.ShapeSpec.Geometry) {
	case pptx.GeomUpArrow, pptx.GeomRightArrow, pptx.GeomLeftArrow:
		// An arrow's head — the gable of a strategy-house roof, the point of
		// a process step — is the shape's form, not a box waiting for text:
		// the text belongs to the shaft, so that is the area it is measured
		// against.
		shapeArea = float64(tw) * float64(th)
	}
	if a.slideArea <= 0 || shapeArea <= float64(a.slideArea)*sparseFillMinShapeSlideFrac {
		return
	}
	if frac := blockW * blockH / (shapeArea / (emuPerPt * emuPerPt)); frac < sparseFillMaxTextFrac {
		a.sparse = append(a.sparse, sparseFillHit{path: cellPath + "/shape", textFrac: frac, areaFrac: shapeArea / float64(a.slideArea)})
	}
}

// openColumnPatterns are the patterns whose default look is open: headings,
// rules and text standing on the slide in content-sized columns and rows, with
// no tile behind them (layout nativeness, 2026-10-03). Their visual unit is
// the column or row slot, exactly the rectangle the tile they replaced
// filled, so an unfilled text cell or a standalone icon counts as its slot:
// removing a container must not turn the same content into an "underused"
// slide. The slots are sized to their content by the pattern, so a sparse
// payload is still a small block and still reports.
var openColumnPatterns = map[string]bool{
	"icon-row":             true,
	"quote-cluster":        true,
	"comparison-2col":      true,
	"stylish-panels":       true,
	"before-after":         true,
	"before-after-compact": true,
	"matrix-2x2":           true,
	"framework-grid":       true,
	"dual-org-ladder":      true,
	// Numbered rows between hairline rules: the row is the unit, as in the
	// tile list it replaced (go-slide-creator-r3gsw).
	"agenda": true,
	// Lanes are bands between full-width hairline rules; the actor label
	// stands in its band where a filled lane tile used to be
	// (go-slide-creator-jz5r9).
	"swimlane": true,
	// Numbered rows of a list — action / owner / date between hairline rules,
	// a numbered step beside its label and description: the row is the unit,
	// as in the agenda. A three-action next-steps slide set in 14pt covered
	// 27% of a wide content area by glyphs alone and reported as underused
	// (go-slide-creator-le9d0, -ttpae).
	"next-steps":          true,
	"numbered-step-strip": true,
	// Open text in content-sized columns beside or under drawn shapes
	// (go-slide-creator-am8kr): a value-chain description under its step, a
	// timeline stop's date and description around its dot, a driver-tree
	// annotation beside its rule, the image-text-split text column beside
	// its image. Counted by glyphs these exemplars sat 0.3–2 points over
	// their threshold — 0.1 under it for value-chain in p-style's Arial —
	// so a narrower face or a changed metric reported the pattern's own
	// example. The column is the unit, as above; the bare forms (steps
	// without descriptions, stops without dates) are the shape row alone and
	// still report (TestBareOpenColumnPatternsStillReportUnderused).
	"value-chain":         true,
	"timeline-horizontal": true,
	"driver-tree":         true,
	"image-text-split":    true,
}

// ruledColumnPatterns are the patterns that set open text columns beside an
// accent rule as tall as the column: hero-detail's default detail cards. The
// rule draws the column's edge the way the card fill it replaced did
// (go-slide-creator-19pp9), so a ruled cell counts as its slot — the same
// reasoning as openColumnPatterns, applied to the ruled cells only: the hero
// figure above them still counts by its glyphs. Counted by glyphs, the
// exemplar sat at 29.1% of business-template's larger content area against
// the 29% threshold (go-slide-creator-0e0en); a hero over two bare titles is
// still a small block and still reports.
var ruledColumnPatterns = map[string]bool{
	"hero-detail": true,
}

// slotOr returns the cell's grid slot when the slide's pattern counts ink by
// slot, else rect.
func (a *geomAccumulator) slotOr(cell shapegrid.ResolvedCell, rect pptx.RectEmu) pptx.RectEmu {
	if a.slotInk && cell.CellBounds.CX > 0 && cell.CellBounds.CY > 0 {
		return cell.CellBounds
	}
	return rect
}

// findings converts the accumulated hits into (at most) one
// TEXT_EXCEEDS_SHAPE and one SPARSE_FILL finding for the slide.
func (a *geomAccumulator) findings(patternName string, explicitPatternBounds bool) []patterns.FitFinding {
	var out []patterns.FitFinding
	if len(a.exceeds) > 0 {
		worst := a.exceeds[0]
		cells := make([]string, len(a.exceeds))
		words := make([]string, len(a.exceeds))
		for i, h := range a.exceeds {
			cells[i], words[i] = h.path, h.word
			if geometryOverflowRatio(h) > geometryOverflowRatio(worst) {
				worst = h
			}
		}
		ratio := geometryOverflowRatio(worst)
		action := "review"
		if ratio >= 2 {
			action = "shrink_or_split"
		}
		hint := "shorten the label, lower its text size, or give this cell more width; check the next fit report"
		if worst.availPt < worst.minGlyphPt {
			hint = "even one glyph cannot fit: widen the text area or change the shape geometry; shortening the label alone cannot work"
			if heightSensitiveGeometry(worst.geometry) {
				hint = "even one glyph cannot fit: widen the text area by reducing the height of this pointed shape or changing its geometry; shortening the label alone cannot work"
			}
		}
		params := map[string]any{
			"cells": cells, "word": worst.word, "required_pt": round1(worst.wordPt),
			"available_pt": round1(worst.availPt), "geometry": worst.geometry, "hint": hint,
		}
		if worst.availPt < worst.minGlyphPt {
			params["minimum_glyph_pt"] = round1(worst.minGlyphPt)
			if patternName != "" && heightSensitiveGeometry(worst.geometry) {
				patchField := "max_height_pct"
				if explicitPatternBounds {
					patchField = "bounds/height"
				}
				params["patch_path"] = fmt.Sprintf("/slides/%d/pattern/%s", slidepath.SlideIndex(worst.path), patchField)
				params["alternative_patch"] = "change the affected pattern geometry or style to one with a wider text area"
				if patternName == "process-flow" || patternName == "process-flow-compact" {
					params["alternative_patch"] = "change the affected pattern step type from a pointed geometry to step"
				}
			} else if patternName != "" {
				params["patch_path"] = fmt.Sprintf("/slides/%d/pattern/values", slidepath.SlideIndex(worst.path))
				params["alternative_patch"] = "give the affected steps more width or use a wider geometry"
			} else {
				params["patch_path"] = strings.TrimSuffix(worst.path, "/text") + "/geometry"
				params["alternative_patch"] = "change the affected shape geometry to rect or use fewer columns"
			}
		}
		msg := fmt.Sprintf("%q needs %.0fpt but the %s shape leaves %.0fpt of text width after geometry and insets — it will break mid-word or be clipped", worst.word, worst.wordPt, worst.geometry, worst.availPt)
		if len(a.exceeds) > 1 {
			msg = fmt.Sprintf("%d shapes have words wider than their text area (%s); worst: %s", len(a.exceeds), strings.Join(words, ", "), msg)
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: patternName,
				Path:    a.exceeds[0].path,
				Code:    patterns.ErrCodeTextExceedsShape,
				Message: msg,
				Fix: &patterns.FixSuggestion{
					Kind:   "widen_shape_text_area",
					Params: params,
				},
			},
			Action:        action,
			Measured:      &patterns.Extent{WidthEMU: int64(worst.wordPt * emuPerPt)},
			Allowed:       &patterns.Extent{WidthEMU: int64(worst.availPt * emuPerPt)},
			OverflowRatio: ratio,
		})
	}
	if len(a.sparse) > 0 {
		cells := make([]string, len(a.sparse))
		minT, maxT, maxA := 1.0, 0.0, 0.0
		for i, h := range a.sparse {
			cells[i] = h.path
			minT, maxT, maxA = math.Min(minT, h.textFrac), math.Max(maxT, h.textFrac), math.Max(maxA, h.areaFrac)
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: patternName,
				Path:    a.sparse[0].path,
				Code:    patterns.ErrCodeSparseFill,
				Message: fmt.Sprintf("%d filled shape(s) each cover >%.0f%% of the slide (up to %.0f%%) but their text fills only %.0f–%.0f%% of the box — large, mostly empty blocks", len(a.sparse), 100*sparseFillMinShapeSlideFrac, 100*maxA, 100*minT, 100*maxT),
				Fix: &patterns.FixSuggestion{
					Kind: "add_detail_or_resize",
					Params: map[string]any{
						"cells":             cells,
						"max_text_area_pct": math.Round(100 * maxT),
						"hint":              "add detail, cap the grid height (bounds / max_height_pct), or use a compact / unfilled variant",
					},
				},
			},
			Action: "review",
		})
	}
	return out
}

func geometryOverflowRatio(h textExceedsHit) float64 {
	// Zero usable width is the most severe case, not a zero overflow ratio.
	return overflowRatio(h.wordPt, math.Max(h.availPt, 0.1))
}

func (a *geomAccumulator) heightSensitiveOverflow() bool {
	for _, h := range a.exceeds {
		if heightSensitiveGeometry(h.geometry) {
			return true
		}
	}
	return false
}

func heightSensitiveGeometry(geometry string) bool {
	switch geometry {
	case "chevron", "homePlate", "hexagon", "octagon":
		return true
	}
	return false
}

// contentSizedBoxPatterns are the patterns whose filled cards / panels hug
// their content instead of stretching to fill the zone
// (go-slide-creator-wntyw); kpi-Nup is matched by prefix.
var contentSizedBoxPatterns = map[string]bool{
	"card-grid":            true,
	"before-after":         true,
	"before-after-compact": true,
	"strategy-house":       true,
	// Content-sized since the restrained-accent pass (go-slide-creator-xb06p,
	// -3nsll, -x0b82). process-flow-compact stays at the pattern threshold:
	// its shallow band is supporting context, and alone on a
	// slide it should be paired with a zone.
	"process-flow":    true,
	"arch-stack":      true,
	"comparison-2col": true,
}

// heroStatementPatterns are the patterns that set one statement — a figure, a
// quote — alone at the optical centre of the slide. The white space around it
// is the composition, so the slide reports only when even the statement is
// small (go-slide-creator-ttpae).
var heroStatementPatterns = map[string]bool{
	"stat-hero":  true,
	"pull-quote": true,
}

// contentSizedStripPatterns are the time-line patterns whose rows are
// content-sized (go-slide-creator-7km8, -n1muf) and hang from the template's
// body line (go-slide-creator-e17xy): a three-phase roadmap or a four-stop
// timeline with one-line descriptions covers ~24-29% of the zone at its
// natural height, and the only ways to 29% were stretching the phase boxes
// past contentStretchMax or centring a thin band mid-slide — the dead space
// the 2026-10-01 review flagged on every template. A roadmap of bare phase
// names (~20%: the header boxes alone) or a timeline of bare labels still
// reports.
var contentSizedStripPatterns = map[string]bool{
	"phase-roadmap":       true,
	"timeline-horizontal": true,
}

// slideUsage is how much of its safe content area a slide's ink covers
// (frac) against the share SLIDE_UNDERUSED holds it to (threshold), for the
// band-cap source that chose the threshold.
type slideUsage struct {
	slide     int
	pattern   string
	frac      float64
	threshold float64
	source    string
}

// measureSlideUsage is the measurement behind SLIDE_UNDERUSED; ok is false
// for a slide the finding does not judge.
func measureSlideUsage(ink []pptx.RectEmu, safe pptx.RectEmu, slide *SlideInput, patternName string) (slideUsage, bool) {
	if safe.CX <= 0 || safe.CY <= 0 || hasBodyPlaceholderContent(slide) {
		return slideUsage{}, false
	}
	// A restrictive author cap uses the stricter threshold. An uncapped raw
	// grid needs different advice from a content-sized pattern.
	u := slideUsage{pattern: patternName, frac: inkCoverageFraction(ink, safe), source: bandCapSource(slide), threshold: slideUnderusedPatternMaxFrac}
	switch u.source {
	case "author":
		u.threshold = slideUnderusedMaxFrac
	case "pattern":
		switch {
		case contentSizedBoxPatterns[patternName] || strings.HasPrefix(patternName, "kpi-"):
			u.threshold = slideUnderusedBoxPatternMaxFrac
		case contentSizedStripPatterns[patternName]:
			u.threshold = slideUnderusedStripPatternMaxFrac
		case heroStatementPatterns[patternName]:
			u.threshold = slideUnderusedHeroPatternMaxFrac
		}
	}
	return u, true
}

func checkSlideUnderused(ink []pptx.RectEmu, safe pptx.RectEmu, slide *SlideInput, si int, patternName string, heightSensitiveOverflow bool) *patterns.FitFinding {
	u, ok := measureSlideUsage(ink, safe, slide, patternName)
	if !ok {
		return nil
	}
	frac, threshold, source := u.frac, u.threshold, u.source
	hint := "add detail to the grid, pair it with supporting content, or merge with another slide"
	switch source {
	case "author":
		hint = "raise or remove the bounds / max_height_pct cap on this slide, add a supporting zone, or merge with another slide"
		if heightSensitiveOverflow {
			hint = "a taller band would narrow the pointed shape's text area further; widen or replace that shape, add a supporting zone, or merge with another slide"
		}
	case "pattern":
		hint = "this block sizes itself to its content — add detail to it, pair it with a supporting zone using compose, or choose a denser pattern"
	}
	// A content-sized, middle-anchored block (a KPI row, before-after panels)
	// leaves equal bands above and below it by design: boxes are not stretched
	// to fill the zone (go-slide-creator-wntyw). Only the ink share counts —
	// the former KPI "empty band >= 0.75in" clause pushed cards back towards
	// the 270pt-for-60pt-of-content stretch and is gone. A lopsided band is
	// VERTICAL_IMBALANCE's business.
	if frac >= threshold {
		return nil
	}

	reason := "the rendered content leaves most of the slide's content zone empty"
	switch source {
	case "author":
		reason = "the bounds / max_height_pct cap leaves too little rendered content"
	case "pattern":
		reason = "the pattern's visible content occupies too little of the slide"
	}
	message := fmt.Sprintf("slide content covers %.0f%% of the safe content area (threshold %.0f%%) — %s", 100*frac, 100*threshold, reason)
	params := map[string]any{
		"content_area_pct": math.Round(100 * frac),
		"threshold_pct":    math.Round(100 * threshold),
		"band_capped_by":   source,
		"hint":             hint,
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    slidepath.Slide(si),
			Code:    patterns.ErrCodeSlideUnderused,
			Message: message,
			Fix: &patterns.FixSuggestion{
				Kind:   "add_detail_or_resize",
				Params: params,
			},
		},
		Action: "review",
	}
}

// inkCoverageFraction measures the union of *visible* rectangles inside the
// content zone. A bounding box reports a full slide when two small labels sit
// at opposite corners, and summing areas double-counts overlapping card/text
// rectangles. The x-sweep is exact for axis-aligned resolved ink rectangles.
func inkCoverageFraction(ink []pptx.RectEmu, safe pptx.RectEmu) float64 {
	if safe.CX <= 0 || safe.CY <= 0 {
		return 0
	}
	clipped := make([]pptx.RectEmu, 0, len(ink))
	xs := make([]int64, 0, 2*len(ink))
	for _, r := range ink {
		r = intersectRect(r, safe)
		if r.CX <= 0 || r.CY <= 0 {
			continue
		}
		clipped = append(clipped, r)
		xs = append(xs, r.X, r.X+r.CX)
	}
	if len(xs) == 0 {
		return 0
	}
	slices.Sort(xs)
	var area float64
	for i := 1; i < len(xs); i++ {
		if xs[i] == xs[i-1] {
			continue
		}
		var spans [][2]int64
		for _, r := range clipped {
			if r.X < xs[i] && r.X+r.CX > xs[i-1] {
				spans = append(spans, [2]int64{r.Y, r.Y + r.CY})
			}
		}
		slices.SortFunc(spans, func(a, b [2]int64) int { return cmp.Compare(a[0], b[0]) })
		var covered, end int64
		for j, span := range spans {
			if j == 0 || span[0] > end {
				covered += span[1] - span[0]
				end = span[1]
			} else if span[1] > end {
				covered += span[1] - end
				end = span[1]
			}
		}
		area += float64(xs[i]-xs[i-1]) * float64(covered)
	}
	return area / (float64(safe.CX) * float64(safe.CY))
}

// bandCapSource identifies a restrictive authored bound, a pattern-owned
// content-sized band, or a raw/full-area grid with no effective cap.
func bandCapSource(slide *SlideInput) string {
	if slide == nil {
		return "none"
	}
	if slide.Pattern != nil {
		if b, _ := resolvePatternBounds(slide.Pattern); b != nil {
			if boundsConstrainArea(b) {
				return "author"
			}
			return "none"
		}
		// Fit preflight may already have expanded the pattern into ShapeGrid.
		// Its grid bounds are pattern-owned, never an author cap.
		return "pattern"
	}
	if slide.ShapeGrid != nil && boundsConstrainArea(slide.ShapeGrid.Bounds) {
		return "author"
	}
	return "none"
}

// boundsConstrainArea excludes explicit full-slide bounds: x/y may be shifted
// or width/height expanded and still cover the whole slide, but a missing edge
// is a real author-imposed restriction that can make the grid underuse it.
func boundsConstrainArea(b *GridBoundsInput) bool {
	if b == nil {
		return false
	}
	return b.X > 0 || b.Y > 0 || b.X+b.Width < 100 || b.Y+b.Height < 100
}

// hasBodyPlaceholderContent reports whether the slide puts content into a
// non-title placeholder; the grid then shares the content area and its
// bounding box alone does not describe slide usage.
func hasBodyPlaceholderContent(slide *SlideInput) bool {
	for _, c := range slide.Content {
		switch c.PlaceholderID {
		case "title", "subtitle", "":
			continue
		default:
			return true
		}
	}
	return false
}

// geometryTextWidthEMU returns the width of the preset geometry's text
// rectangle (per ECMA-376 presetShapeDefinitions) for the given bounds.
func geometryTextWidthEMU(spec *shapegrid.ShapeSpec, b pptx.RectEmu) int64 {
	switch pptx.PresetGeometry(spec.Geometry) {
	case pptx.GeomRightArrow, pptx.GeomLeftArrow:
		// Two handles: the shaft height decides how far the text rectangle
		// reaches into the head.
		w, _ := pptx.PresetTextRect(spec.Geometry, spec.Adjustments, b)
		return w
	}
	w, _ := pptx.PresetTextRectSize(spec.Geometry, geometryAdj(spec), b)
	return w
}

// geometryAdj is the spec's "adj" adjustment, or -1 for the preset default.
func geometryAdj(spec *shapegrid.ShapeSpec) int64 {
	if v, ok := spec.Adjustments["adj"]; ok {
		return v
	}
	return -1
}

// shapeIsFilled reports whether a shape's fill renders a visible colour
// block: not none/transparent, not (near-)fully transparent via alpha, and
// not the slide background colour (lt1/bg1/white), which reads as empty.
func shapeIsFilled(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return !isNoFill(s)
	}
	var obj struct {
		Color string  `json:"color"`
		Alpha float64 `json:"alpha"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	alphaPct := obj.Alpha
	if alphaPct > 0 && alphaPct <= 1 {
		alphaPct *= 100 // fractional convention (see shapegrid.ResolveFillInput)
	}
	return !isNoFill(obj.Color) && (obj.Alpha == 0 || alphaPct >= 20)
}

func isNoFill(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none", "transparent", "nofill", "lt1", "bg1", "#fff", "#ffffff", "white":
		return true
	}
	return false
}

// parseGeomText parses a shape_grid text payload (string, object with
// content, or object with paragraphs[]) into measurable paragraphs.
func parseGeomText(raw json.RawMessage) geomText {
	t := geomText{insets: pptx.ShapeTextInsets()}
	if len(raw) == 0 {
		return t
	}
	if tb, err := shapegrid.ResolveTextInput(raw); err == nil && tb != nil {
		// The writer's own resolution: uniform margin unless authored.
		t.body, t.insets = tb, tb.Insets
	}
	add := func(content string, size float64, bold bool) {
		size = shapegrid.EffectiveTextSizePt(size)
		if size <= 0 {
			size = shapeDefaultTextPt
		}
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(inlineTagRe.ReplaceAllString(strings.NewReplacer("**", "", "__", "").Replace(line), ""))
			if line != "" {
				t.paragraphs = append(t.paragraphs, geomParagraph{text: line, sizePt: size, bold: bold})
			}
		}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		add(s, 0, false)
		return t
	}
	var obj struct {
		Content       string  `json:"content"`
		Size          float64 `json:"size"`
		Bold          bool    `json:"bold"`
		Align         string  `json:"align"`
		VerticalAlign string  `json:"vertical_align"`
		Vert          string  `json:"vert"`
		Paragraphs    []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
			Bold    bool    `json:"bold"`
		} `json:"paragraphs"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return t
	}
	t.align, t.vAlign, t.vert = obj.Align, obj.VerticalAlign, obj.Vert
	if len(obj.Paragraphs) > 0 {
		for _, p := range obj.Paragraphs {
			add(p.Content, p.Size, p.Bold)
		}
		return t
	}
	add(obj.Content, obj.Size, obj.Bold)
	return t
}

// writtenInsets returns the insets the writer emits for this text inside the
// preset's text rectangle textRect, with any icon-overlay reservation added:
// the body's insets clamped on a degenerate axis exactly as
// pptx.GenerateShape clamps them.
func (t geomText) writtenInsets(textRect pptx.RectEmu, overlay [4]int64) [4]int64 {
	var body pptx.TextBody
	if t.body != nil {
		body = *t.body
	}
	body.Insets = t.insets
	for i := range body.Insets {
		body.Insets[i] += overlay[i]
	}
	return pptx.EffectiveTextInsets(&body, textRect)
}

// insetLR returns the horizontal text insets in EMU.
func (t geomText) insetLR() int64 {
	return t.insets[0] + t.insets[2]
}

// insetTB returns the vertical text insets in EMU.
func (t geomText) insetTB() int64 {
	return t.insets[1] + t.insets[3]
}

// geomMeasurer measures text with the template body font (falling back to
// the metric-stable embedded Liberation Sans via fontcache).
type geomMeasurer struct {
	family *canvas.FontFamily
}

func newGeomMeasurer(theme *types.ThemeInfo) *geomMeasurer {
	name := fallbackMeasureFnt
	if theme != nil && theme.BodyFont != "" {
		name = theme.BodyFont
	}
	return &geomMeasurer{family: fontcache.Get(name, fallbackMeasureFnt)}
}

// widthPt returns the rendered width of s in points (0 when no font).
func (m *geomMeasurer) widthPt(s string, sizePt float64, bold bool) float64 {
	if m.family == nil || s == "" {
		return 0
	}
	style := canvas.FontRegular
	if bold {
		style = canvas.FontBold
	}
	face := m.family.Face(sizePt, color.Black, style, canvas.FontNormal) // Face takes points; TextWidth returns mm
	return safeTextWidthMM(face, s, sizePt) * mmToPt
}

// safeTextWidthMM measures s with face.TextWidth, which shapes without
// itemising by script: harfbuzz guesses RTL for Arabic/Hebrew while canvas
// slices clusters as LTR and panics (go-slide-creator-csclk.42). On a panic it
// falls back to canvas.NewTextLine, which itemises and shapes RTL runs with an
// explicit direction, and finally to a coarse per-rune estimate.
func safeTextWidthMM(face *canvas.FontFace, s string, sizePt float64) (w float64) {
	defer func() {
		if recover() != nil {
			w = textLineWidthMM(face, s, sizePt)
		}
	}()
	return face.TextWidth(s)
}

func textLineWidthMM(face *canvas.FontFace, s string, sizePt float64) (w float64) {
	defer func() {
		if recover() != nil {
			w = 0.55 * sizePt * float64(len([]rune(s))) / mmToPt
		}
	}()
	return canvas.NewTextLine(face, s, canvas.Left).Bounds().W()
}

// widestWord returns the widest whitespace-delimited word across paragraphs.
func (m *geomMeasurer) widestWord(t geomText) (string, float64, float64) {
	var best string
	var bestW, minGlyph float64
	for _, p := range t.paragraphs {
		for _, w := range strings.Fields(p.text) {
			if len([]rune(w)) < 2 {
				continue // a lone glyph (arrow, bullet) cannot break mid-word
			}
			if ww := m.widthPt(w, p.sizePt, p.bold); ww > bestW {
				best, bestW = w, ww
				minGlyph = math.Inf(1)
				for _, r := range w {
					minGlyph = math.Min(minGlyph, m.widthPt(string(r), p.sizePt, p.bold))
				}
			}
		}
	}
	return best, bestW, minGlyph
}

// textBlockPt greedily word-wraps every paragraph at availPt and returns the
// text block's width (widest line) and height in points.
func (m *geomMeasurer) textBlockPt(t geomText, availPt float64) (float64, float64) {
	var blockW, blockH float64
	for _, p := range t.paragraphs {
		space := m.widthPt(" ", p.sizePt, p.bold)
		lines, lineW := 1, 0.0
		for _, w := range strings.Fields(p.text) {
			ww := m.widthPt(w, p.sizePt, p.bold)
			switch {
			case lineW == 0:
				lineW = ww
			case lineW+space+ww <= availPt:
				lineW += space + ww
			default:
				blockW = math.Max(blockW, math.Min(lineW, availPt))
				lines++
				lineW = ww
			}
		}
		blockW = math.Max(blockW, math.Min(lineW, availPt))
		blockH += float64(lines) * p.sizePt * geometryLineHeight
	}
	return blockW, blockH
}

// placeTextBlock positions an estimated text block inside a cell according
// to the text's alignment (shape_grid defaults: centered both ways).
func placeTextBlock(b pptx.RectEmu, t geomText, wPt, hPt float64) pptx.RectEmu {
	w := int64(wPt * emuPerPt)
	h := int64(hPt*emuPerPt) + t.insetTB()
	w = minI64(w+t.insetLR(), b.CX)
	h = minI64(h, b.CY)
	x := b.X + (b.CX-w)/2
	switch strings.ToLower(t.align) {
	case "l", "left":
		x = b.X
	case "r", "right":
		x = b.X + b.CX - w
	}
	y := b.Y + (b.CY-h)/2
	switch strings.ToLower(t.vAlign) {
	case "t", "top":
		y = b.Y
	case "b", "bottom":
		y = b.Y + b.CY - h
	}
	return pptx.RectEmu{X: x, Y: y, CX: w, CY: h}
}

func intersectRect(a, b pptx.RectEmu) pptx.RectEmu {
	x0, y0 := maxI64(a.X, b.X), maxI64(a.Y, b.Y)
	x1, y1 := minI64(a.X+a.CX, b.X+b.CX), minI64(a.Y+a.CY, b.Y+b.CY)
	if x1 <= x0 || y1 <= y0 {
		return pptx.RectEmu{}
	}
	return pptx.RectEmu{X: x0, Y: y0, CX: x1 - x0, CY: y1 - y0}
}

func overflowRatio(measured, allowed float64) float64 {
	if allowed <= 0 {
		return 0
	}
	return math.Round(measured/allowed*100) / 100
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func minI64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// zoneBodyTop is the zone's body line, 0 without a zone.
func zoneBodyTop(zone *shapegrid.ContentZone) int64 {
	if zone == nil {
		return 0
	}
	return zone.BodyTop
}
