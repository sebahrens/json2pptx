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

// EffectiveTextInsets returns the insets a body is written with inside bounds.
// It is the declared insets, except on an axis where they leave less than one
// line of the body's text: there both sides shrink proportionally until one
// line fits, and to zero when even the bare shape cannot hold one. A body with
// no declared insets (renderer defaults) or no text is returned unchanged.
//
// The vertical need is one line at the largest run size; the horizontal need is
// the widest single word, since wrapping can break anywhere else. Vertical text
// (vert / vert270 / …) swaps the two axes.
func EffectiveTextInsets(tb *TextBody, bounds RectEmu) [4]int64 {
	if tb == nil {
		return [4]int64{}
	}
	in := tb.Insets
	if in == [4]int64{} {
		return in
	}
	lineH, wordBound := oneLineBoundsEMU(tb)
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
		return widestWordEMU(tb)
	}
	if tb.Vert != "" && tb.Vert != "horz" {
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

// oneLineBoundsEMU returns the height of one line at the body's largest run
// size and a cheap upper bound (1em per rune) on its widest word, both in EMU.
// Zero height means the body carries no text.
func oneLineBoundsEMU(tb *TextBody) (lineH, wordBound int64) {
	maxHPt := 0
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if strings.TrimSpace(r.Text) == "" {
				continue
			}
			size := runSizeHPt(r)
			if size > maxHPt {
				maxHPt = size
			}
			for _, w := range strings.FieldsFunc(r.Text, unicode.IsSpace) {
				if b := int64(len([]rune(w))) * int64(size) * 127; b > wordBound {
					wordBound = b
				}
			}
		}
	}
	if maxHPt == 0 {
		return 0, 0
	}
	return int64(float64(maxHPt) * oneLineFactor * 127), wordBound
}

// widestWordEMU measures the widest single word of the body at its run size.
func widestWordEMU(tb *TextBody) int64 {
	var widest int64
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			size := runSizeHPt(r)
			for _, w := range strings.FieldsFunc(r.Text, unicode.IsSpace) {
				ww, err := textfit.MeasureStyledLineWidth(w, autofitFontName, float64(size)/100, r.Bold)
				if err != nil {
					// No measurement font: a conservative 0.6em per rune.
					ww = int64(float64(len([]rune(w))) * float64(size) * 0.6 * 127)
				}
				if ww > widest {
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

// PresetTextRectSize returns the width and height of a preset geometry's own
// text rectangle inside bounds — the box bodyPr insets are measured from. Most
// presets use the whole shape; pointed and round presets pull it in. adj is the
// "adj" adjustment (OOXML 1/100000 units), or negative for the preset default.
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
		v := math.Min(a(50000), 100000*w/ss)
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
	case "hexagon":
		tw = w - 2*ss*a(25000)/100000
	case "octagon":
		tw = w - ss*a(29289)/100000
	}
	return int64(math.Max(tw, 0)), int64(math.Max(th, 0))
}

// presetTextBounds is the text rectangle a shape's insets are clamped
// against: the preset's own text rectangle, sized as PresetTextRectSize says.
func presetTextBounds(opts ShapeOptions) RectEmu {
	adj := int64(-1)
	for _, av := range opts.Adjustments {
		if av.Name == "adj" {
			adj = av.Value
		}
	}
	cx, cy := PresetTextRectSize(string(opts.Geometry), adj, opts.Bounds)
	return RectEmu{X: opts.Bounds.X, Y: opts.Bounds.Y, CX: cx, CY: cy}
}
