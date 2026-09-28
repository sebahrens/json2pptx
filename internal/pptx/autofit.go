package pptx

import (
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
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
	// autofitFontName is the measurement font: Liberation Sans is embedded and
	// metric-compatible with Arial, so the prediction is host-independent. It
	// matches internal/textcapacity's budget font.
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
	if tb == nil || tb.AutoFit != "normAutofit" {
		return 1
	}
	widthEMU, heightEMU := textAreaEMU(tb, bounds)
	if widthEMU <= 0 || heightEMU <= 0 {
		return 1
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
		return 1
	}

	availablePt := float64(heightEMU) / float64(types.EMUPerPoint)
	if tb.Insets == [4]int64{} {
		// No declared insets: the renderer still applies its defaults.
		availablePt -= 2 * autofitDefaultInsetPt
	}
	measureW := widthEMU
	if tb.Insets != [4]int64{} {
		// textfit.MeasureRun removes the OOXML default 0.1in sides from the
		// width it is handed. A body with declared insets has already had its
		// own removed (textAreaEMU), so hand the measurement that allowance
		// back — otherwise every shape is measured 14.4pt narrower than the
		// text area it writes, the same correction internal/textcapacity makes
		// at its measurement boundary.
		measureW += 2 * autofitMeasureSideEMU
	}
	scale, _ := textfit.AutofitScale(paras, measureW, availablePt, textfit.AutofitOptions{
		FontName:    autofitFontName,
		LineSpacing: autofitLineSpacing,
		FloorScale:  autofitFloorScale,
	})
	scale = math.Min(scale, longestWordScale(tb, widthEMU))
	if scale >= 1 {
		return 1
	}
	return scale
}

// longestWordScale returns the shrink at which the body's widest word fits
// on one line, or 1 when every word already fits. A normAutofit body only
// shrinks for height, so a label whose box was tall enough still broke a word
// it could not hold ("Managemen / t") at full size (go-slide-creator-n83ml).
// The shrink never takes text below the readable minimum (a grown label
// shrinks back toward it; 12pt text does not shrink), and a word that still
// breaks there is left to the height fit rather than shrunk for nothing.
func longestWordScale(tb *TextBody, widthEMU int64) float64 {
	if tb.Insets == [4]int64{} {
		widthEMU -= 2 * autofitDefaultInsetLREMU
	}
	// fontScale shrinks every paragraph alike, so the smallest text in the
	// body sets how far the whole body may shrink.
	minPt := math.Inf(1)
	for _, p := range tb.Paragraphs {
		if text, pt := paragraphTextAndSize(p); strings.TrimSpace(text) != "" && pt > 0 {
			minPt = math.Min(minPt, pt)
		}
	}
	if minPt <= autofitWordMinPt || math.IsInf(minPt, 1) {
		return 1
	}
	floor := math.Max(autofitWordFloorScale, autofitWordMinPt/minPt)
	scale := 1.0
	for _, p := range tb.Paragraphs {
		text, pt := paragraphTextAndSize(p)
		avail := float64(widthEMU-p.MarginL) * autofitWordSafety
		if pt <= 0 || avail <= 0 {
			continue
		}
		for _, word := range strings.Fields(text) {
			if len([]rune(word)) < 2 {
				continue
			}
			w, err := textfit.MeasureLineWidth(word, autofitFontName, pt)
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
// after the body's insets.
func textAreaEMU(tb *TextBody, bounds RectEmu) (width, height int64) {
	width, height = bounds.CX, bounds.CY
	if in := EffectiveTextInsets(tb, bounds); in != [4]int64{} {
		width -= in[0] + in[2]
		height -= in[1] + in[3]
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
