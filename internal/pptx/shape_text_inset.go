package pptx

import (
	"math"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Uniform text margin for engine-drawn shapes.
//
// Every text-bearing shape the engine draws — shape_grid cells (and so every
// named pattern, compose block and raw grid) and the native diagram shapes of
// internal/generator — keeps its text 0.5 cm away from the shape edge on all
// four sides. It is the engine-wide default on every template. Template
// placeholders (title / body / subtitle) keep the template's own margins, and
// table cells are not shapes.
//
// ShapeTextInsets is the one source of that value: emitters write it and every
// capacity / fit / readability estimator subtracts the same number, so what is
// predicted is what is written.
//
// Degenerate shapes: a shape too small to hold one line of its text plus the
// margin on an axis (a pill, a number badge, an axis label, a legend caption)
// gets that axis's margin clamped — see EffectiveTextInsets — to what still
// leaves one line of text, never below zero. The other axis keeps the full
// margin.

// ShapeTextInsetEMU is the uniform text margin on each side of a shape's text
// body: 0.5 cm = 180000 EMU ≈ 14.17pt.
const ShapeTextInsetEMU int64 = 180000

// ShapeTextInsetPt is ShapeTextInsetEMU in points.
const ShapeTextInsetPt = float64(ShapeTextInsetEMU) / 12700

// ShapeTextInsets returns the uniform [L,T,R,B] text insets in EMU.
func ShapeTextInsets() [4]int64 {
	return [4]int64{ShapeTextInsetEMU, ShapeTextInsetEMU, ShapeTextInsetEMU, ShapeTextInsetEMU}
}

// oneLineFactor is the line height the clamp reserves, as a multiple of the
// largest run size — the same 1.2 the autofit measure uses.
const oneLineFactor = autofitLineSpacing

// writtenOneLineFactor is the line height the clamp leaves in the shape it
// writes (go-slide-creator-bhbtk). Sizing, growth and the fit findings reserve
// oneLineFactor: 1.2 em, the measure every estimate shares. A renderer sets a
// line at its face's own ascent plus descent — 1.22 em in Carlito (Calibri),
// more in a serif — so a one-line box clamped to exactly 1.2 em held its
// line in the estimate and not on the slide: LibreOffice re-fitted the
// waterfall's "$120m", metric-list values, the capability-heatmap legend,
// matrix-2x2's "High" / "Low" and a chart caption by tightening their line
// spacing, read back as lnSpcReduction from its round trip. The written
// margin gives up the difference; layout is decided on the 1.2 em measure as
// before, so nothing moves but the margin of a clamped box.
const writtenOneLineFactor = 1.3

// WordFitSlack is the room a clamped axis leaves around its widest word, as a
// multiple of the word's measured width, when the word is measured in the face
// it renders in. Renderers measure with their own hinting and rounding: a word
// handed exactly its measured width wrapped its last glyph onto a second line
// ("Desig / n" under a timeline dot, go-slide-creator-v74wv).
const WordFitSlack = 1.05

// StandInWordFitSlack is WordFitSlack for a word measured in the Liberation
// Sans stand-in because its own face is host-dependent (autofitMeasureFace).
// The face the renderer substitutes can be far wider: LibreOffice draws Segoe
// UI as Verdana, whose bold is 16–21% wider, and broke "Octob / er" on
// modern-yellow.
const StandInWordFitSlack = 1.2

// wordFitSlackFor is the slack the clamp gives tb's widest word.
func wordFitSlackFor(tb *TextBody) float64 {
	if autofitMeasureFace(tb).exact {
		return WordFitSlack
	}
	return StandInWordFitSlack
}

// EffectiveTextInsets returns the insets a body is written with inside bounds.
// It is the declared insets, except on an axis where they leave less than one
// line of the body's text: there both sides shrink proportionally until one
// line fits, and to zero when even the bare shape cannot hold one. A body with
// no declared insets (renderer defaults) or no text is returned unchanged.
//
// The vertical need is one line at the largest run size; the horizontal need is
// the widest single word plus its paragraph's side margins (a bulleted item's
// text starts at marL), with WordFitSlack (StandInWordFitSlack for a
// host-dependent face) of room, since wrapping can break
// anywhere else. Vertical text (vert / vert270 / …) swaps the two axes.
func EffectiveTextInsets(tb *TextBody, bounds RectEmu) [4]int64 {
	return clampedTextInsets(tb, bounds, oneLineFactor)
}

// writtenTextInsets is EffectiveTextInsets with the one line the clamp keeps
// at writtenOneLineFactor: the insets the shape writer emits. They are never
// larger than EffectiveTextInsets, so the written text area is at least the
// one every estimate assumed.
func writtenTextInsets(tb *TextBody, bounds RectEmu) [4]int64 {
	return clampedTextInsets(tb, bounds, writtenOneLineFactor)
}

// vertLineEndInsetCapEMU is the most margin rotated text (vert / vert270)
// keeps at the two ends of its line, 3pt. LibreOffice 24.2, the release
// Ubuntu 24.04 ships, re-fits rotated text wrongly when those insets are
// large: with the shape margin's 14.17pt at each end it wrote "Market Growth"
// at 93% in a 222pt bar that holds the line twice over, and a three-word rail
// label at 75%, while a one-word label and the same labels at 3pt were left
// alone (go-slide-creator render-truth check, LibreOffice 26 and PowerPoint
// do not do this). Rotated labels are centred along a bar, so the margin
// they give up is not seen.
const vertLineEndInsetCapEMU = 38100

func clampedTextInsets(tb *TextBody, bounds RectEmu, lineFactor float64) [4]int64 {
	if tb == nil {
		return [4]int64{}
	}
	in := tb.Insets
	if in == [4]int64{} {
		return in
	}
	maxHPt, wordBound := oneLineBoundsEMU(tb)
	lineH := int64(float64(maxHPt) * lineFactor * 127)
	if lineH <= 0 {
		return in
	}
	// The word-width need is measured only when its cheap 1em-per-rune upper
	// bound does not already fit: measuring every word of every shape on each
	// fit probe would dominate generation time.
	wordW := func(room int64) int64 {
		if room >= wordBound {
			return 0
		}
		return int64(math.Ceil(float64(widestWordEMU(tb)) * wordFitSlackFor(tb)))
	}
	if tb.Vert != "" && tb.Vert != "horz" {
		in[1], in[3] = min(in[1], vertLineEndInsetCapEMU), min(in[3], vertLineEndInsetCapEMU)
		in[1], in[3] = clampInsetPair(in[1], in[3], bounds.CY, wordW(bounds.CY-in[1]-in[3]))
		in[0], in[2] = clampInsetPair(in[0], in[2], bounds.CX, lineH)
	} else {
		in[0], in[2] = clampInsetPair(in[0], in[2], bounds.CX, wordW(bounds.CX-in[0]-in[2]))
		in[1], in[3] = clampInsetPair(in[1], in[3], bounds.CY, lineH)
	}
	return in
}

// UniformInsetFor is the per-side inset a shape keeps on an axis of the given
// extent when its text needs `need` EMU of it: the uniform margin, clamped as
// EffectiveTextInsets clamps a degenerate axis. Estimators that size text
// areas without a TextBody use it to mirror the writer.
func UniformInsetFor(extent, need int64) int64 {
	a, _ := clampInsetPair(ShapeTextInsetEMU, ShapeTextInsetEMU, extent, need)
	return a
}

// clampInsetPair shrinks a pair of opposite insets proportionally so that
// extent-a-b is at least need, never going below zero.
func clampInsetPair(a, b, extent, need int64) (int64, int64) {
	if need <= 0 || a+b <= 0 || extent-a-b >= need {
		return a, b
	}
	room := extent - need
	if room <= 0 {
		return 0, 0
	}
	na := room * a / (a + b)
	return na, room - na
}

// oneLineBoundsEMU returns the body's largest run size in hundredths of a
// point and a cheap upper bound (1em per rune) on its widest word in EMU.
// Zero size means the body carries no text.
func oneLineBoundsEMU(tb *TextBody) (maxHPt int, wordBound int64) {
	for _, p := range tb.Paragraphs {
		margins := paragraphSideMarginsEMU(p)
		for _, r := range p.Runs {
			if strings.TrimSpace(r.Text) == "" {
				continue
			}
			size := runSizeHPt(r)
			if size > maxHPt {
				maxHPt = size
			}
			for _, w := range strings.FieldsFunc(r.Text, unicode.IsSpace) {
				b := int64(float64(int64(len([]rune(w)))*int64(size)*127)*StandInWordFitSlack) + margins + runTrackingEMU(r, w)
				if b > wordBound {
					wordBound = b
				}
			}
		}
	}
	if maxHPt == 0 {
		return 0, 0
	}
	return maxHPt, wordBound
}

// runTrackingEMU is the width a run's letter-spacing adds to word w.
func runTrackingEMU(r Run, w string) int64 {
	return int64(len([]rune(w))*max(r.Spacing, 0)) * 127
}

// WordFitSlackFor is the slack EffectiveTextInsets leaves around tb's widest
// word: WordFitSlack when tb is measured in its own face, StandInWordFitSlack
// otherwise. Anything that spends a clamped body's width (caps tracking)
// must leave it.
func WordFitSlackFor(tb *TextBody) float64 {
	return wordFitSlackFor(tb)
}

// paragraphSideMarginsEMU is the line width a paragraph's own left and right
// margins take from the text area.
func paragraphSideMarginsEMU(p Paragraph) int64 {
	return max(p.MarginL, 0) + max(p.MarginR, 0)
}

// widestWordEMU measures the widest single word of the body at its run size,
// plus its paragraph's side margins.
func widestWordEMU(tb *TextBody) int64 {
	var widest int64
	font := autofitMeasureFace(tb).name
	for _, p := range tb.Paragraphs {
		margins := paragraphSideMarginsEMU(p)
		for _, r := range p.Runs {
			size := runSizeHPt(r)
			for _, w := range strings.FieldsFunc(r.Text, unicode.IsSpace) {
				ww, err := textfit.MeasureStyledLineWidth(w, font, float64(size)/100, r.Bold)
				if err != nil {
					// No measurement font: a conservative 0.6em per rune.
					ww = int64(float64(len([]rune(w))) * float64(size) * 0.6 * 127)
				}
				if ww += margins + runTrackingEMU(r, w); ww > widest {
					widest = ww
				}
			}
		}
	}
	return widest
}

// runSizeHPt is a run's size in hundredths of a point (OOXML's 18pt default
// when the run declares none).
func runSizeHPt(r Run) int {
	if r.FontSize > 0 {
		return r.FontSize
	}
	return 1800
}

// chevronMaxNotchFraction caps the depth of each chevron notch as a fraction
// of the shape width. The preset default (adj 50000) sets the notch to half
// the shorter side, so a chevron less than twice as wide as it is tall loses
// its whole text rectangle to the two notches and wraps labels mid-word
// ("Dis/cov/er", go-slide-creator-5wm83). A 25% cap keeps at least half the
// width for text while leaving wide chevrons at the preset default.
const chevronMaxNotchFraction = 0.25

// DefaultChevronAdj returns the "adj" a chevron of the given size gets when
// the author set none: the preset 50000, reduced for stubby chevrons so each
// notch is at most chevronMaxNotchFraction of the width. ok is false for
// degenerate bounds.
func DefaultChevronAdj(cx, cy int64) (adj int64, ok bool) {
	if cx <= 0 || cy <= 0 {
		return 0, false
	}
	ss := min(cx, cy)
	capped := int64(chevronMaxNotchFraction * 100000 * float64(cx) / float64(ss))
	return min(int64(50000), capped), true
}

// PresetTextRectSize returns the width and height of a preset geometry's own
// text rectangle inside bounds — the box bodyPr insets are measured from. Most
// presets use the whole shape; pointed and round presets pull it in. adj is the
// "adj" adjustment (OOXML 1/100000 units), or negative for the preset default
// (for a chevron, the DefaultChevronAdj notch cap).
func PresetTextRectSize(geometry string, adj int64, bounds RectEmu) (int64, int64) {
	w, h := float64(bounds.CX), float64(bounds.CY)
	ss := math.Min(w, h)
	if ss <= 0 {
		return bounds.CX, bounds.CY
	}
	a := func(def float64) float64 {
		if adj >= 0 {
			return float64(adj)
		}
		return def
	}
	tw, th := w, h
	switch geometry {
	case "chevron":
		def := int64(50000)
		if capped, ok := DefaultChevronAdj(bounds.CX, bounds.CY); ok {
			def = capped
		}
		v := math.Min(a(float64(def)), 100000*w/ss)
		tw = w - 2*ss*v/100000
	case "homePlate":
		v := math.Min(a(50000), 100000*w/ss)
		tw = w - ss*v/100000/2
	case "diamond", "flowChartDecision":
		tw, th = w/2, h/2
	case "triangle":
		tw, th = w/2, h/2
	case "flowChartTerminator":
		// Text rectangle l=1018/21600, t=3163/21600, r=20582/21600, b=18437/21600.
		tw, th = w*19564/21600, h*15274/21600
	case "ellipse", "flowChartConnector":
		tw, th = w*math.Sqrt2/2, h*math.Sqrt2/2
	case "trapezoid":
		// il = wd3·a/maxAdj with maxAdj = 50000·w/ss, it likewise: the text
		// rectangle loses il on each side and it at the top. Measured as the
		// whole shape, a pyramid apex broke "Stra / tegy" (go-slide-creator-v74wv).
		maxAdj := 50000 * w / ss
		v := math.Min(math.Max(a(25000), 0), maxAdj)
		tw = w - 2*(w/3)*v/maxAdj
		th = h - (h/3)*v/maxAdj
	case "hexagon":
		tw = w - 2*ss*a(25000)/100000
	case "octagon":
		tw = w - ss*a(29289)/100000
	}
	return int64(math.Max(tw, 0)), int64(math.Max(th, 0))
}

// UpArrowTextRectSize returns the width and height of an upArrow's text
// rectangle inside bounds: the shaft, plus the part of the head directly above
// it that the shaft's width reaches into. adj1 is the shaft width and adj2 the
// head length (OOXML 1/100000 units; negative takes the preset default 50000).
//
// A full-width shaft (adj1 100000) is a gable pentagon — the strategy house's
// roof — whose text rectangle is the band under the slope, not the whole
// shape: measured as the whole shape, text sized to the band was predicted to
// fit a box a gable taller than the one it is drawn in.
func UpArrowTextRectSize(adj1, adj2 int64, bounds RectEmu) (int64, int64) {
	w, h := float64(bounds.CX), float64(bounds.CY)
	ss := math.Min(w, h)
	if ss <= 0 {
		return bounds.CX, bounds.CY
	}
	a1, a2 := 50000.0, 50000.0
	if adj1 >= 0 {
		a1 = float64(adj1)
	}
	if adj2 >= 0 {
		a2 = float64(adj2)
	}
	a1 = math.Min(a1, 100000)
	a2 = math.Min(a2, 100000*h/ss)
	head := ss * a2 / 100000
	shaft := w * a1 / 100000
	// The rectangle's top climbs the head's slope until it meets the shaft's
	// outer edge: y1 = head - x1·head/(w/2), with x1 the shaft's left edge.
	top := head - (w-shaft)/2*head/(w/2)
	return int64(math.Max(shaft, 0)), int64(math.Max(h-top, 0))
}

// SideArrowTextRectSize returns the width and height of a rightArrow's or
// leftArrow's text rectangle inside bounds: the shaft, plus the part of the
// head the shaft's height reaches into. adj1 is the shaft height and adj2 the
// head length (OOXML 1/100000 units; negative takes the preset default 50000).
//
// Measured as the whole shape, a label sized to fit the shape was written
// unshrunk into a shaft half as tall, and the renderer shrank it to about 3pt
// (go-slide-creator-fx48s).
func SideArrowTextRectSize(adj1, adj2 int64, bounds RectEmu) (int64, int64) {
	w, h := float64(bounds.CX), float64(bounds.CY)
	ss := math.Min(w, h)
	if ss <= 0 {
		return bounds.CX, bounds.CY
	}
	a1, a2 := 50000.0, 50000.0
	if adj1 >= 0 {
		a1 = float64(adj1)
	}
	if adj2 >= 0 {
		a2 = float64(adj2)
	}
	a1 = math.Min(a1, 100000)
	a2 = math.Min(a2, 100000*w/ss)
	head := ss * a2 / 100000
	shaft := h * a1 / 100000
	// The rectangle runs into the head until the shaft's edge meets the
	// head's slope: dx2 = y1·head/(h/2), with y1 the shaft's top edge.
	reach := (h - shaft) / 2 * head / (h / 2)
	return int64(math.Max(w-head+reach, 0)), int64(math.Max(shaft, 0))
}

// SideArrowShaftInsetEMU is the distance from a rightArrow's or leftArrow's
// bounding box to its shaft, top and bottom: where a connector that meets the
// arrow from above or below at its centre touches the outline.
func SideArrowShaftInsetEMU(adj1 int64, bounds RectEmu) int64 {
	a1 := int64(50000)
	if adj1 >= 0 {
		a1 = min(adj1, 100000)
	}
	return bounds.CY * (100000 - a1) / 200000
}

// PresetTextRect returns the size of a preset geometry's text rectangle given
// all of the shape's adjustments by name. It is PresetTextRectSize for the
// presets with a single "adj" handle, plus the two-handle upArrow, rightArrow
// and leftArrow.
func PresetTextRect(geometry string, adjustments map[string]int64, bounds RectEmu) (int64, int64) {
	get := func(name string) int64 {
		if v, ok := adjustments[name]; ok {
			return v
		}
		return -1
	}
	if geometry == string(GeomUpArrow) {
		return UpArrowTextRectSize(get("adj1"), get("adj2"), bounds)
	}
	if geometry == string(GeomRightArrow) || geometry == string(GeomLeftArrow) {
		return SideArrowTextRectSize(get("adj1"), get("adj2"), bounds)
	}
	return PresetTextRectSize(geometry, get("adj"), bounds)
}

// presetTextBounds is the text rectangle a shape's insets are clamped
// against: the preset's own text rectangle, sized as PresetTextRect says.
func presetTextBounds(opts ShapeOptions) RectEmu {
	adjustments := make(map[string]int64, len(opts.Adjustments))
	for _, av := range opts.Adjustments {
		adjustments[av.Name] = av.Value
	}
	cx, cy := PresetTextRect(string(opts.Geometry), adjustments, opts.Bounds)
	return RectEmu{X: opts.Bounds.X, Y: opts.Bounds.Y, CX: cx, CY: cy}
}

// autofitBounds is the box a shape's stored autofit shrink is measured in: the
// shape, except for an arrow, whose text lives in its shaft.
func autofitBounds(opts ShapeOptions) RectEmu {
	switch opts.Geometry {
	case GeomUpArrow, GeomRightArrow, GeomLeftArrow:
		return presetTextBounds(opts)
	}
	return opts.Bounds
}
