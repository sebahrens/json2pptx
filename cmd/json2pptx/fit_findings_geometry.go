package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"regexp"
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
	// slideUnderusedPatternMaxFrac is the same test for a band nobody capped.
	// A content-sized pattern derives its own height from its content
	// (go-slide-creator-7km8), so measuring it against the whole content zone
	// and demanding 45% flagged the deliberately-shorter slides that change
	// produced — on three hand-crafted decks every firing was a false positive,
	// and the fix hint told the agent to remove a cap it never set
	// (go-slide-creator-up04). Only a band that is a genuine sliver is worth
	// reporting, and the advice for it is about content, not caps.
	slideUnderusedPatternMaxFrac = 0.22
	// textExceedsTolerance absorbs rounding/kerning noise before flagging.
	textExceedsTolerance = 1.02

	emuPerPt           = 12700.0
	defaultInsetLREMU  = 91440 // OOXML default lIns / rIns (0.1")
	defaultInsetTBEMU  = 45720 // OOXML default tIns / bIns (0.05")
	shapeDefaultTextPt = 14.0  // shapegrid defaultTextSizeHPt
	geometryLineHeight = 1.2
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
	insets     [4]int64 // authored L,T,R,B in EMU; -1 means "use OOXML default"
	align      string
	vAlign     string
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
		acc := &geomAccumulator{m: m, slideArea: slideWidth * slideHeight, slideWidth: slideWidth, slideHeight: slideHeight}
		acc.walk(grid, result, basePath, 0)
		findings = append(findings, acc.findings(patternName)...)
		safe := contentRelativeBoundsBase(geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
		if f := checkSlideUnderused(acc.ink, safe, &slide, si, patternName); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// maxGeomNestingDepth bounds recursion into nested sub-grids.
const maxGeomNestingDepth = 4

// textExceedsHit / sparseFillHit record one offending cell.
type textExceedsHit struct {
	path            string
	word, geometry  string
	wordPt, availPt float64
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
	exceeds                 []textExceedsHit
	sparse                  []sparseFillHit
}

func (a *geomAccumulator) walk(input *ShapeGridInput, result *shapegrid.ResolveResult, basePath string, depth int) {
	for _, cell := range result.Cells {
		cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", basePath, cell.RowIdx, cell.ColIdx)
		switch cell.Kind {
		case shapegrid.CellKindShape:
			a.shapeCell(cell, cellPath)
		case shapegrid.CellKindTable, shapegrid.CellKindIcon, shapegrid.CellKindImage, shapegrid.CellKindDiagram, shapegrid.CellKindComposite:
			a.ink = append(a.ink, cell.Bounds)
		case shapegrid.CellKindSubGrid:
			a.subGrid(input, cell, cellPath, depth)
		}
	}
	for _, ab := range result.AccentBars {
		a.ink = append(a.ink, ab.Bounds)
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
		a.ink = append(a.ink, cell.Bounds)
		return
	}
	inset := pptx.RectEmu{X: cell.Bounds.X + subGridInsetEMU, Y: cell.Bounds.Y + subGridInsetEMU, CX: cell.Bounds.CX - 2*subGridInsetEMU, CY: cell.Bounds.CY - 2*subGridInsetEMU}
	if inset.CX <= 0 || inset.CY <= 0 {
		inset = cell.Bounds
	}
	sub := resolveGridForStructural(src.Grid, &inset, nil, a.slideWidth, a.slideHeight)
	if sub == nil {
		a.ink = append(a.ink, cell.Bounds)
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
	if filled {
		a.ink = append(a.ink, cell.Bounds)
	}
	if len(txt.paragraphs) == 0 {
		return
	}
	// Rotated text runs along the shape's height: the line length it has is the
	// box's height, not its width.
	availW := geometryTextWidthEMU(cell.ShapeSpec, cell.Bounds) - txt.insetLR() - cell.TextInsets[0] - cell.TextInsets[2]
	if txt.rotated() {
		availW = cell.Bounds.CY - txt.insetTB() - cell.TextInsets[1] - cell.TextInsets[3]
	}
	availPt := math.Max(float64(availW)/emuPerPt, 0)
	if word, wordPt := a.m.widestWord(txt); word != "" && wordPt > availPt*textExceedsTolerance {
		geometry := cell.ShapeSpec.Geometry
		if geometry == "" {
			geometry = "rect"
		}
		a.exceeds = append(a.exceeds, textExceedsHit{path: cellPath + "/shape/text", word: word, geometry: geometry, wordPt: wordPt, availPt: availPt})
	}
	blockW, blockH := a.m.textBlockPt(txt, math.Max(availPt, 1))
	if txt.rotated() {
		// The block was measured along the text's own axis; on the slide it
		// occupies the transposed rectangle.
		blockW, blockH = blockH, blockW
	}
	if !filled {
		a.ink = append(a.ink, placeTextBlock(cell.Bounds, txt, blockW, blockH))
		return
	}
	shapeArea := float64(cell.Bounds.CX) * float64(cell.Bounds.CY)
	if a.slideArea <= 0 || shapeArea <= float64(a.slideArea)*sparseFillMinShapeSlideFrac {
		return
	}
	if frac := blockW * blockH / (shapeArea / (emuPerPt * emuPerPt)); frac < sparseFillMaxTextFrac {
		a.sparse = append(a.sparse, sparseFillHit{path: cellPath + "/shape", textFrac: frac, areaFrac: shapeArea / float64(a.slideArea)})
	}
}

// findings converts the accumulated hits into (at most) one
// TEXT_EXCEEDS_SHAPE and one SPARSE_FILL finding for the slide.
func (a *geomAccumulator) findings(patternName string) []patterns.FitFinding {
	var out []patterns.FitFinding
	if len(a.exceeds) > 0 {
		worst := a.exceeds[0]
		cells := make([]string, len(a.exceeds))
		words := make([]string, len(a.exceeds))
		for i, h := range a.exceeds {
			cells[i], words[i] = h.path, h.word
			if h.wordPt-h.availPt > worst.wordPt-worst.availPt {
				worst = h
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
					Kind: "reduce_text",
					Params: map[string]any{
						"cells":        cells,
						"word":         worst.word,
						"required_pt":  round1(worst.wordPt),
						"available_pt": round1(worst.availPt),
						"geometry":     worst.geometry,
						"hint":         "shorten the label, lower text size, or use a geometry with a wider text area (rect/homePlate instead of chevron)",
					},
				},
			},
			Action:        "review",
			Measured:      &patterns.Extent{WidthEMU: int64(worst.wordPt * emuPerPt)},
			Allowed:       &patterns.Extent{WidthEMU: int64(worst.availPt * emuPerPt)},
			OverflowRatio: overflowRatio(worst.wordPt, worst.availPt),
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

func checkSlideUnderused(ink []pptx.RectEmu, safe pptx.RectEmu, slide *SlideInput, si int, patternName string) *patterns.FitFinding {
	if len(ink) == 0 || safe.CX <= 0 || safe.CY <= 0 || hasBodyPlaceholderContent(slide) {
		return nil
	}
	u := ink[0]
	for _, r := range ink[1:] {
		u = unionRect(u, r)
	}
	// Only the part of the ink box inside the safe area counts.
	u = intersectRect(u, safe)
	frac := float64(u.CX) * float64(u.CY) / (float64(safe.CX) * float64(safe.CY))

	// Whether the band's height was the AUTHOR's decision or the pattern's
	// decides both the threshold and the advice (go-slide-creator-up04).
	capped := authorCappedBand(slide)
	threshold := slideUnderusedPatternMaxFrac
	hint := "this block sizes itself to its content — add detail to it, pair it with a supporting zone using compose, or choose a denser pattern"
	if capped {
		threshold = slideUnderusedMaxFrac
		hint = "raise or remove the bounds / max_height_pct cap on this slide, add a supporting zone, or merge with another slide"
	}
	if frac >= threshold {
		return nil
	}

	reason := "the pattern's own content-derived height leaves it a thin strip"
	if capped {
		reason = "the bounds / max_height_pct cap on this slide leaves it a thin strip"
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    slidepath.Slide(si),
			Code:    patterns.ErrCodeSlideUnderused,
			Message: fmt.Sprintf("slide content covers %.0f%% of the safe content area (threshold %.0f%%) — %s", 100*frac, 100*threshold, reason),
			Fix: &patterns.FixSuggestion{
				Kind: "add_detail_or_resize",
				Params: map[string]any{
					"content_area_pct": math.Round(100 * frac),
					"threshold_pct":    math.Round(100 * threshold),
					"band_capped_by":   bandCappedBy(capped),
					"hint":             hint,
				},
			},
		},
		Action: "review",
	}
}

// authorCappedBand reports whether the slide itself constrains the grid's
// height — an explicit bounds block or max_height_pct on the pattern, or bounds
// on a raw shape_grid. When it does not, the height came from the pattern's own
// content sizing and no amount of editing the slide JSON will change it.
func authorCappedBand(slide *SlideInput) bool {
	if slide == nil {
		return false
	}
	if slide.Pattern != nil {
		if b, _ := resolvePatternBounds(slide.Pattern); b != nil {
			return true
		}
		// Fit preflight may already have expanded the pattern into ShapeGrid.
		// Its grid bounds are pattern-owned, never an author cap.
		return false
	}
	return slide.ShapeGrid != nil && slide.ShapeGrid.Bounds != nil
}

// bandCappedBy names who chose the band's height, so an agent can branch on it
// without parsing the hint prose.
func bandCappedBy(capped bool) string {
	if capped {
		return "author"
	}
	return "pattern"
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
	w, h := float64(b.CX), float64(b.CY)
	ss := math.Min(w, h)
	adj := func(def float64) float64 {
		if v, ok := spec.Adjustments["adj"]; ok {
			return float64(v)
		}
		return def
	}
	var tw float64
	switch spec.Geometry {
	case "chevron":
		a := math.Min(adj(50000), 100000*w/ss)
		tw = w - 2*ss*a/100000
	case "homePlate":
		a := math.Min(adj(50000), 100000*w/ss)
		tw = w - ss*a/100000/2
	case "diamond", "flowChartDecision", "triangle":
		tw = w / 2
	case "ellipse", "flowChartConnector":
		tw = w * math.Sqrt2 / 2
	case "hexagon":
		tw = w - 2*ss*adj(25000)/100000
	case "octagon":
		tw = w - ss*adj(29289)/100000
	default:
		tw = w
	}
	return int64(math.Max(tw, 0))
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
	t := geomText{insets: [4]int64{-1, -1, -1, -1}}
	if len(raw) == 0 {
		return t
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
		InsetLeft     float64 `json:"inset_left"`
		InsetRight    float64 `json:"inset_right"`
		InsetTop      float64 `json:"inset_top"`
		InsetBottom   float64 `json:"inset_bottom"`
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
	if obj.InsetLeft > 0 || obj.InsetTop > 0 || obj.InsetRight > 0 || obj.InsetBottom > 0 {
		// Mirrors shapegrid.buildTextBody: any authored inset replaces all four.
		t.insets = [4]int64{int64(obj.InsetLeft * emuPerPt), int64(obj.InsetTop * emuPerPt), int64(obj.InsetRight * emuPerPt), int64(obj.InsetBottom * emuPerPt)}
	}
	if len(obj.Paragraphs) > 0 {
		for _, p := range obj.Paragraphs {
			add(p.Content, p.Size, p.Bold)
		}
		return t
	}
	add(obj.Content, obj.Size, obj.Bold)
	return t
}

// insetLR returns the horizontal text insets in EMU.
func (t geomText) insetLR() int64 {
	if t.insets[0] < 0 {
		return 2 * defaultInsetLREMU
	}
	return t.insets[0] + t.insets[2]
}

// insetTB returns the vertical text insets in EMU.
func (t geomText) insetTB() int64 {
	if t.insets[1] < 0 {
		return 2 * defaultInsetTBEMU
	}
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
	return face.TextWidth(s) * mmToPt
}

// widestWord returns the widest whitespace-delimited word across paragraphs.
func (m *geomMeasurer) widestWord(t geomText) (string, float64) {
	var best string
	var bestW float64
	for _, p := range t.paragraphs {
		for _, w := range strings.Fields(p.text) {
			if len([]rune(w)) < 2 {
				continue // a lone glyph (arrow, bullet) cannot break mid-word
			}
			if ww := m.widthPt(w, p.sizePt, p.bold); ww > bestW {
				best, bestW = w, ww
			}
		}
	}
	return best, bestW
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

func unionRect(a, b pptx.RectEmu) pptx.RectEmu {
	x0, y0 := minI64(a.X, b.X), minI64(a.Y, b.Y)
	x1, y1 := maxI64(a.X+a.CX, b.X+b.CX), maxI64(a.Y+a.CY, b.Y+b.CY)
	return pptx.RectEmu{X: x0, Y: y0, CX: x1 - x0, CY: y1 - y0}
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
