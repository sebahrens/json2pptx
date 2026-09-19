package textfit

import "strings"

// Autofit scale prediction (go-slide-creator-wvr0).
//
// <a:normAutofit/> with no fontScale means "shrink to fit, scale unknown".
// LibreOffice recomputes and shrinks; PowerPoint applies the STORED scale —
// 100% when the attribute is absent — and only recomputes when a human edits
// the shape. So a diagram whose text overflows rendered small in our own
// screenshots and overflowed its box for the person who opened the file, and
// every visual-QA loop that renders through soffice was inspecting pixels the
// client would never see.
//
// This is the single implementation of the prediction: internal/textcapacity
// uses it for its density model and internal/pptx uses it to write the scale
// into the shape, so the number the engine reports and the number it stores
// can never drift apart.

// AutofitParagraph is one paragraph of text at its authored font size.
type AutofitParagraph struct {
	Text   string
	FontPt float64
	// SpaceAfterPt is the paragraph's trailing space in points. It does NOT
	// scale with the font: PowerPoint's fontScale scales glyphs and
	// lnSpcReduction compresses line spacing, but a spcAft in points stays put.
	// Ignoring it predicted a 66% shrink for a block that needed far more —
	// twelve bullets at 6pt space-after carry 72pt of spacing the glyph
	// measurement never sees (go-slide-creator-wvr0).
	SpaceAfterPt float64
}

// AutofitOptions tunes the scale search.
type AutofitOptions struct {
	// FontName is the font used for measurement.
	FontName string
	// LineSpacing is the line height as a multiple of font size.
	LineSpacing float64
	// FloorScale is the smallest scale the renderer is assumed to reach. Below
	// it, text is clipped rather than shrunk.
	FloorScale float64
	// Step is the search granularity (0.02 = 2% steps).
	Step float64
}

// AutofitScale returns the largest scale in Step increments whose measured
// wrapped height fits availableHeightPt, and whether any scale at or above
// FloorScale fits.
//
// A scale of 1 with fits=true means the text already fits at its authored size.
// When the text cannot be measured the result is 1/true: an unmeasurable run is
// never reported as an overflow.
func AutofitScale(paras []AutofitParagraph, widthEMU int64, availableHeightPt float64, opts AutofitOptions) (float64, bool) {
	if availableHeightPt <= 0 || widthEMU <= 0 {
		return 1, true
	}
	if opts.LineSpacing <= 0 {
		opts.LineSpacing = 1.2
	}
	if opts.FloorScale <= 0 {
		opts.FloorScale = 0.2
	}
	if opts.Step <= 0 {
		opts.Step = 0.02
	}

	fits := func(scale float64) bool {
		total := 0.0
		for _, p := range paras {
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			pt := p.FontPt * scale
			m, err := MeasureRun(p.Text, opts.FontName, pt, widthEMU, 0)
			if err != nil {
				return true // cannot measure: do not invent an overflow
			}
			total += float64(m.Lines)*pt*opts.LineSpacing + p.SpaceAfterPt
		}
		return total <= availableHeightPt
	}

	for step := 0; ; step++ {
		scale := 1.0 - opts.Step*float64(step)
		if scale < opts.FloorScale {
			break
		}
		if fits(scale) {
			return scale, true
		}
	}
	return opts.FloorScale, false
}
