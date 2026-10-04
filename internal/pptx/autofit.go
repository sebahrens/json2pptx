package pptx

import (
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// Writing the autofit scale into the shape (go-slide-creator-wvr0).
//
// Every native diagram wrote <a:normAutofit/> with no fontScale. LibreOffice
// recomputes the shrink, so our own renders showed the text merely small;
// PowerPoint applies the STORED scale — 100% when the attribute is absent — and
// only recomputes when a human edits the shape, so the same box overflowed for
// the person who opened the deck. That also silently invalidated every visual-QA
// loop that renders through soffice: the pixels an agent inspected were not the
// pixels the client saw.

const (
	// autofitFontName is the fallback measurement font: Liberation Sans is
	// embedded and metric-compatible with Arial, so the prediction is
	// host-independent. It matches internal/textcapacity's budget font. A
	// body whose text renders in one host-independent theme face is measured
	// in that face instead (autofitMeasureFont).
	autofitFontName = "Liberation Sans"
	// autofitLineSpacing is the line height as a multiple of font size.
	autofitLineSpacing = 1.2
	// autofitFloorScale mirrors textcapacity.AutofitFloorScale: the smallest
	// shrink a renderer is assumed to reach.
	autofitFloorScale = 0.2
	// autofitDefaultInsetPt is the default body inset (0.05in top/bottom,
	// 0.1in left/right) in points, used when a body declares none.
	autofitDefaultInsetPt = 3.6
	// autofitMeasureSideEMU is the side margin textfit.MeasureRun removes
	// from every width it measures (the OOXML 0.1in default).
	autofitMeasureSideEMU = 91440
	// autofitScaleDenominator converts a 0..1 scale to OOXML's
	// percent-thousandths (0.62 -> 62000).
	autofitScaleDenominator = 100000
	// autofitMaxLnSpcReduction caps the line-spacing reduction PowerPoint is
	// asked to apply, matching what it writes itself for heavy shrinks.
	autofitMaxLnSpcReduction = 20000
	// autofitDefaultInsetLREMU is the default left/right body inset (0.1in).
	autofitDefaultInsetLREMU = 91440
	// autofitWordFloorScale is the smallest shrink applied to keep the widest
	// word on one line; a word that needs more breaks regardless.
	autofitWordFloorScale = 0.6
	// autofitWordMinPt is the readable minimum (shapegrid.MinTextSizePt) a
	// widest-word shrink never goes below.
	autofitWordMinPt = 12.0
	// autofitWordSafety leaves room for the difference between the measuring
	// font and the template's own body font.
	autofitWordSafety = 0.97
)

// applyAutofitScale fills a normAutofit body's fontScale / lnSpcReduction from
// the measured text, so the file renders the same in PowerPoint as in
// LibreOffice. It is a no-op for any other autofit mode, for a body whose scale
// the caller already set, and for a body with no measurable box.
func applyAutofitScale(tb *TextBody, bounds RectEmu) {
	if tb == nil || tb.AutoFit != "normAutofit" || tb.AutoFitFontScale != 0 {
		return
	}
	SetAutofitScale(tb, AutofitScaleFor(tb, bounds))
}

// AutofitScaleFor returns the uniform shrink (0..1] a normAutofit body needs to
// fit bounds, or 1 when it fits, is not normAutofit, or cannot be measured.
func AutofitScaleFor(tb *TextBody, bounds RectEmu) float64 {
	m, ok := autofitMeasureInput(tb, bounds)
	if !ok {
		return 1
	}
	scale, _ := textfit.AutofitScale(m.paras, m.measureW, m.availablePt, autofitOptionsFor(m.face.name))
	scale = math.Min(scale, longestWordScale(tb, m.widthEMU, m.face))
	if scale >= 1 {
		return 1
	}
	return scale
}

// AutofitFitsFor reports whether AutofitScaleFor(tb, bounds) >= 1 — the body
// is written with no shrink — measuring only the authored size. Height
// searches (writtenFitHeightPt, nativeTextNeedEMU) probe many heights and need
// only this yes/no; AutofitScaleFor would walk every 2% shrink step down to
// the floor for each probe that does not fit, which dominated schema-maximum
// measurement under -race (go-slide-creator-b7qqg.16). The answer is
// identical to AutofitScaleFor(tb, bounds) >= 1 by construction: the scale
// search returns 1 exactly when the first (scale 1) step fits.
func AutofitFitsFor(tb *TextBody, bounds RectEmu) bool {
	m, ok := autofitMeasureInput(tb, bounds)
	if !ok {
		return true
	}
	if !textfit.AutofitFits(m.paras, m.measureW, m.availablePt, autofitOptionsFor(m.face.name)) {
		return false
	}
	return longestWordScale(tb, m.widthEMU, m.face) >= 1
}

// autofitOptionsFor is the measurement configuration AutofitScaleFor and
// AutofitFitsFor share, measuring in font.
func autofitOptionsFor(font string) textfit.AutofitOptions {
	return textfit.AutofitOptions{
		FontName:    font,
		LineSpacing: autofitLineSpacing,
		FloorScale:  autofitFloorScale,
	}
}

// autofitFace is the face a body's autofit shrink is measured in. exact is
// true when it is the face the body renders in (or, for Arial, its metric
// twin), rather than a stand-in for an unknown or host-dependent face.
type autofitFace struct {
	name  string
	exact bool
}

// autofitMeasureFace returns the face a body's autofit shrink is measured in.
//
// Pattern row sizing measures the theme body font through svggen/fontcache —
// Calibri as its embedded metric clone Carlito — while the writer measured
// every body in Liberation Sans. Carlito is narrower, so text a pattern sized
// to fit in N lines needed N+1 in the writer's measure and was written with a
// stored shrink, or refused below the 12pt floor (go-slide-creator-ohhb2).
// When every run of the body renders in one theme face that measures the same
// on every host (fontcache.HostIndependent), the writer measures in that face
// — the face the pattern measured and the renderer draws. Mixed faces, a face
// the measurer would have to guess at, or a body with no theme fonts keep the
// Liberation Sans measure, so the stored bytes never depend on the host.
func autofitMeasureFace(tb *TextBody) autofitFace {
	stand := autofitFace{name: autofitFontName}
	if tb == nil || tb.ThemeFonts == (ThemeFonts{}) {
		return stand
	}
	face := ""
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if strings.TrimSpace(r.Text) == "" {
				continue
			}
			f := runTypeface(r.FontFamily, tb.ThemeFonts)
			if f == "" || (face != "" && !strings.EqualFold(f, face)) {
				return stand
			}
			face = f
		}
	}
	if face == "" || !fontcache.HostIndependent(face) {
		return stand
	}
	if strings.EqualFold(face, "Arial") {
		// Measured, as before, in its embedded metric twin.
		return autofitFace{name: autofitFontName, exact: true}
	}
	return autofitFace{name: face, exact: true}
}

// ParagraphFitFace returns the face a fit decision for one paragraph is
// measured in, and whether it is the face the paragraph renders in (or, for
// Arial, its metric twin). It is autofitMeasureFace for a single paragraph:
// the grid's type step and canvas scale decide a level at a time, and a card
// whose heading and body use the theme's two faces still has one face per
// paragraph (go-slide-creator-5x4w4). A paragraph with mixed faces, a face
// that measures differently from host to host, or no theme fonts is measured
// in Liberation Sans and reported as not exact.
func ParagraphFitFace(p Paragraph, fonts ThemeFonts) (name string, exact bool) {
	if fonts == (ThemeFonts{}) {
		return autofitFontName, false
	}
	face := ""
	for _, r := range p.Runs {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		f := runTypeface(r.FontFamily, fonts)
		if f == "" || (face != "" && !strings.EqualFold(f, face)) {
			return autofitFontName, false
		}
		face = f
	}
	if face == "" || !fontcache.HostIndependent(face) {
		return autofitFontName, false
	}
	if strings.EqualFold(face, "Arial") {
		return autofitFontName, true
	}
	return face, true
}

// runTypeface resolves a run's latin typeface against the theme fonts: the
// theme references to their typefaces, an unset typeface to the body font a
// shape's text inherits, anything else as written.
func runTypeface(family string, fonts ThemeFonts) string {
	switch strings.TrimSpace(family) {
	case "", "+mn-lt":
		return strings.TrimSpace(fonts.Minor)
	case "+mj-lt":
		return strings.TrimSpace(fonts.Major)
	}
	return strings.TrimSpace(family)
}

// autofitInput is the measurable form of a normAutofit body in bounds.
type autofitInput struct {
	paras       []textfit.AutofitParagraph
	measureW    int64
	availablePt float64
	widthEMU    int64
	face        autofitFace
}

// autofitMeasureInput prepares a body for measurement. ok is false when the
// body is not normAutofit, has no text area, or has no measurable text — the
// cases AutofitScaleFor reports as 1.
func autofitMeasureInput(tb *TextBody, bounds RectEmu) (autofitInput, bool) {
	if tb == nil || tb.AutoFit != "normAutofit" {
		return autofitInput{}, false
	}
	widthEMU, heightEMU := textAreaEMU(tb, bounds)
	if widthEMU <= 0 || heightEMU <= 0 {
		return autofitInput{}, false
	}

	paras := make([]textfit.AutofitParagraph, 0, len(tb.Paragraphs))
	for _, p := range tb.Paragraphs {
		text, pt := paragraphTextAndSize(p)
		if text == "" || pt <= 0 {
			continue
		}
		paras = append(paras, textfit.AutofitParagraph{
			Text:         text,
			FontPt:       pt,
			SpaceAfterPt: float64(p.SpaceAfter) / 100.0,
		})
	}
	if len(paras) == 0 {
		return autofitInput{}, false
	}

	availablePt := float64(heightEMU) / float64(types.EMUPerPoint)
	if !tb.declaresInsets() {
		// No declared insets: the renderer still applies its defaults.
		availablePt -= 2 * autofitDefaultInsetPt
	}
	measureW := widthEMU
	if tb.declaresInsets() {
		// textfit.MeasureRun removes the OOXML default 0.1in sides from the
		// width it is handed. A body with declared insets has already had its
		// own removed (textAreaEMU), so hand the measurement that allowance
		// back — otherwise every shape is measured 14.4pt narrower than the
		// text area it writes, the same correction internal/textcapacity makes
		// at its measurement boundary.
		measureW += 2 * autofitMeasureSideEMU
	}
	return autofitInput{paras: paras, measureW: measureW, availablePt: availablePt, widthEMU: widthEMU, face: autofitMeasureFace(tb)}, true
}

// longestWordScale returns the shrink at which the body's widest word fits
// on one line, or 1 when every word already fits. A normAutofit body only
// shrinks for height, so a label whose box was tall enough still broke a word
// it could not hold ("Managemen / t") at full size (go-slide-creator-n83ml).
// The shrink never takes text below the readable minimum (a grown label
// shrinks back toward it; 12pt text does not shrink), and a word that still
// breaks there is left to the height fit rather than shrunk for nothing.
//
// A word measured in a stand-in face keeps autofitWordSafety of the width for
// the difference to the template's face. Measured in the body's own face it
// gets the full width and its own weight — the measure pattern sizing fits a
// KPI value or a label word to (fitSingleLineSize), so a value fitted
// edge-to-edge is not written shrunk (go-slide-creator-ohhb2).
func longestWordScale(tb *TextBody, widthEMU int64, face autofitFace) float64 {
	if !tb.declaresInsets() {
		widthEMU -= 2 * autofitDefaultInsetLREMU
	}
	floor := wordShrinkFloor(tb)
	if floor >= 1 {
		return 1
	}
	safety := autofitWordSafety
	if face.exact {
		safety = 1
	}
	scale := 1.0
	for _, p := range tb.Paragraphs {
		text, pt := paragraphTextAndSize(p)
		avail := float64(widthEMU-p.MarginL) * safety
		if pt <= 0 || avail <= 0 {
			continue
		}
		bold := face.exact && paragraphBold(p)
		for _, word := range strings.Fields(text) {
			if len([]rune(word)) < 2 {
				continue
			}
			w, err := textfit.MeasureStyledLineWidth(word, face.name, pt, bold)
			if err != nil || w <= 0 || float64(w) <= avail {
				continue
			}
			if s := math.Floor(avail/float64(w)*100) / 100; s >= floor {
				scale = math.Min(scale, s)
			}
		}
	}
	return scale
}

// WordShrinkFloor is the smallest fontScale the writer applies to keep a
// normAutofit body's widest word on one line (longestWordScale): 1 when the
// body does not shrink for a word — not normAutofit, or its smallest text is
// already at the readable minimum.
func WordShrinkFloor(tb *TextBody) float64 {
	if tb == nil || tb.AutoFit != "normAutofit" {
		return 1
	}
	return wordShrinkFloor(tb)
}

// WordShrinkRescues reports whether the writer shrinks tb until a word
// wordW wide fits a line availW wide (both in the same unit, measured at the
// authored size) — so the word is written whole, not broken. Fit detectors
// that measure at the authored size use it to report only the words the
// writer cannot save (go-slide-creator-v74wv).
func WordShrinkRescues(tb *TextBody, wordW, availW float64) bool {
	floor := WordShrinkFloor(tb)
	safety := autofitWordSafety
	if autofitMeasureFace(tb).exact {
		safety = 1
	}
	return floor < 1 && wordW*floor <= availW*safety
}

// wordShrinkFloor is WordShrinkFloor without the autofit-mode gate. fontScale
// shrinks every paragraph alike, so the smallest text in the body sets how
// far the whole body may shrink.
func wordShrinkFloor(tb *TextBody) float64 {
	minPt := math.Inf(1)
	for _, p := range tb.Paragraphs {
		if text, pt := paragraphTextAndSize(p); strings.TrimSpace(text) != "" && pt > 0 {
			minPt = math.Min(minPt, pt)
		}
	}
	if minPt <= autofitWordMinPt || math.IsInf(minPt, 1) {
		return 1
	}
	return math.Max(autofitWordFloorScale, autofitWordMinPt/minPt)
}

// paragraphBold reports whether any of the paragraph's text is bold — the
// wider weight, so a mixed paragraph's word is never measured narrower than
// it renders.
func paragraphBold(p Paragraph) bool {
	for _, r := range p.Runs {
		if r.Bold && strings.TrimSpace(r.Text) != "" {
			return true
		}
	}
	return false
}

// SetAutofitScale records a shrink on a normAutofit body. A scale >= 1 leaves
// the bare element, which means "no shrink" to every renderer. Sibling cells
// share one scale through this so a row does not render at uneven sizes
// (go-slide-creator-csclk.99).
func SetAutofitScale(tb *TextBody, scale float64) {
	if tb == nil || tb.AutoFit != "normAutofit" || scale >= 1 || scale <= 0 {
		return
	}
	tb.AutoFitFontScale = int(scale*autofitScaleDenominator + 0.5)
	// PowerPoint pairs a font shrink with a line-spacing reduction; mirroring it
	// keeps a heavily shrunk block from looking airier than the renderer's own
	// fit. Scaled with the shrink and capped.
	reduction := int((1 - scale) * autofitScaleDenominator / 2)
	if reduction > autofitMaxLnSpcReduction {
		reduction = autofitMaxLnSpcReduction
	}
	tb.AutoFitLnSpcReduction = reduction
}

// textAreaEMU returns the width and height available to text inside a shape,
// after the body's insets, in the text's own direction: rotated text
// (vert / vert270) runs its lines along the shape's height, so its line
// measure is the shape's height and its stack the shape's width. Measured
// unrotated, a matrix-2x2 axis label "Market Growth" in a 69pt-wide side
// column was written at 92% for a word that fits its 170pt line
// (go-slide-creator-ohhb2).
func textAreaEMU(tb *TextBody, bounds RectEmu) (width, height int64) {
	width, height = bounds.CX, bounds.CY
	if in := EffectiveTextInsets(tb, bounds); in != [4]int64{} {
		width -= in[0] + in[2]
		height -= in[1] + in[3]
	}
	if tb.Vert != "" && tb.Vert != "horz" {
		return height, width
	}
	return width, height
}

// paragraphTextAndSize joins a paragraph's run text and returns its smallest
// declared size in points. Measuring at the smallest size is deliberate: it is
// the one that wraps least, so the predicted scale never claims more room than
// the paragraph really needs.
func paragraphTextAndSize(p Paragraph) (string, float64) {
	text := ""
	smallest := 0
	for _, r := range p.Runs {
		text += r.Text
		if r.FontSize > 0 && (smallest == 0 || r.FontSize < smallest) {
			smallest = r.FontSize
		}
	}
	if smallest == 0 {
		return text, 0
	}
	return text, float64(smallest) / 100.0
}
