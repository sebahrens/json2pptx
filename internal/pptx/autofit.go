package pptx

import (
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
	// autofitScaleDenominator converts a 0..1 scale to OOXML's
	// percent-thousandths (0.62 -> 62000).
	autofitScaleDenominator = 100000
	// autofitMaxLnSpcReduction caps the line-spacing reduction PowerPoint is
	// asked to apply, matching what it writes itself for heavy shrinks.
	autofitMaxLnSpcReduction = 20000
)

// applyAutofitScale fills a normAutofit body's fontScale / lnSpcReduction from
// the measured text, so the file renders the same in PowerPoint as in
// LibreOffice. It is a no-op for any other autofit mode, for a body whose scale
// the caller already set, and for a body with no measurable box.
func applyAutofitScale(tb *TextBody, bounds RectEmu) {
	if tb == nil || tb.AutoFit != "normAutofit" || tb.AutoFitFontScale != 0 {
		return
	}
	widthEMU, heightEMU := textAreaEMU(tb, bounds)
	if widthEMU <= 0 || heightEMU <= 0 {
		return
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
		return
	}

	availablePt := float64(heightEMU) / float64(types.EMUPerPoint)
	if tb.Insets == [4]int64{} {
		// No declared insets: the renderer still applies its defaults.
		availablePt -= 2 * autofitDefaultInsetPt
	}
	scale, _ := textfit.AutofitScale(paras, widthEMU, availablePt, textfit.AutofitOptions{
		FontName:    autofitFontName,
		LineSpacing: autofitLineSpacing,
		FloorScale:  autofitFloorScale,
	})
	if scale >= 1 {
		// The text fits as authored: leave the bare element, which means "no
		// shrink" to every renderer.
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
	if tb.Insets != [4]int64{} {
		width -= tb.Insets[0] + tb.Insets[2]
		height -= tb.Insets[1] + tb.Insets[3]
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
