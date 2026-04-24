package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// overflowThreshold is the measured/allowed height ratio above which
// placeholder overflow is considered significant. Minor overshoot
// (e.g. 10%) is ignored to avoid false-positive spam from measurement
// imprecision across font rendering backends.
const overflowThreshold = 1.15

// PlaceholderOverflowInput describes a placeholder to check for text overflow.
type PlaceholderOverflowInput struct {
	// SlideIndex is the zero-based slide index (for JSON path generation).
	SlideIndex int
	// Path is the JSON path prefix, e.g. "slides[0].content.body".
	Path string
	// Paragraphs is the text content that will populate the placeholder.
	Paragraphs []string
	// WidthEMU is the placeholder width in EMU.
	WidthEMU int64
	// HeightEMU is the placeholder height in EMU.
	HeightEMU int64
	// FontSizeHPt is the font size in hundredths of a point (e.g. 2000 = 20pt).
	FontSizeHPt int
	// FontName is the font family name (e.g. "Arial").
	FontName string
	// AutofitMode is the OOXML autofit mode of the placeholder's bodyPr:
	// "normAutofit", "spAutoFit", "noAutofit", or "" (none configured).
	// When normAutofit or spAutoFit is active, PowerPoint handles overflow
	// by shrinking text, so overflow findings are suppressed.
	AutofitMode string
}

// DetectPlaceholderOverflow checks whether text in a placeholder overflows its
// frame using a three-condition gate. A finding is emitted only when ALL three
// conditions hold simultaneously:
//
//  1. measured_h / frame_h > 1.15 — significant overshoot, not noise.
//  2. placeholder autofit is "noAutofit" or "" — PowerPoint will not auto-shrink.
//  3. textfit.Calculate reports Overflow — even at minimum font scale, it won't fit.
//
// This prevents false-positive findings on templates that rely on normAutofit
// (condition 2) and avoids flagging borderline cases that autofit would save
// if it were enabled (condition 3).
//
// Returns nil when any condition fails.
func DetectPlaceholderOverflow(input PlaceholderOverflowInput) *patterns.FitFinding {
	if len(input.Paragraphs) == 0 || input.WidthEMU <= 0 || input.HeightEMU <= 0 {
		return nil
	}

	params := textfit.Params{
		WidthEMU:    input.WidthEMU,
		HeightEMU:   input.HeightEMU,
		FontSizeHPt: input.FontSizeHPt,
		FontName:    input.FontName,
		Paragraphs:  input.Paragraphs,
	}

	// Condition 1: measured height at 100% scale significantly exceeds frame.
	measuredEMU, err := textfit.MeasureHeight(params)
	if err != nil || measuredEMU <= 0 {
		return nil // cannot measure — skip
	}

	ratio := float64(measuredEMU) / float64(input.HeightEMU)
	if ratio <= overflowThreshold {
		return nil
	}

	// Condition 2: autofit is absent — PowerPoint won't auto-shrink text.
	if autofitPresent(input.AutofitMode) {
		return nil
	}

	// Condition 3: even at minimum autofit font scale, text still overflows.
	// This acts as a safety net: if hypothetically adding normAutofit would
	// fix it, we don't flag — the remediation is to add autofit, not split.
	fitResult, err := textfit.Calculate(params)
	if err != nil || !fitResult.Overflow {
		return nil
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "placeholder",
			Path:    input.Path,
			Code:    patterns.ErrCodePlaceholderOverflow,
			Message: fmt.Sprintf(
				"text overflows placeholder by %.0f%% (%.0fpt frame, autofit=%s); overflow persists at minimum font scale",
				(ratio-1)*100,
				float64(input.HeightEMU)/12700.0,
				autofitLabel(input.AutofitMode),
			),
			Fix: &patterns.FixSuggestion{Kind: "reduce_text"},
		},
		Action: "shrink_or_split",
		Measured: &patterns.Extent{
			WidthEMU:  input.WidthEMU,
			HeightEMU: measuredEMU,
		},
		Allowed: &patterns.Extent{
			WidthEMU:  input.WidthEMU,
			HeightEMU: input.HeightEMU,
		},
		OverflowRatio: ratio,
	}
}

// TitleWrapsInput describes a title placeholder to check for text wrapping.
type TitleWrapsInput struct {
	// SlideIndex is the zero-based slide index (for JSON path generation).
	SlideIndex int
	// Path is the JSON path prefix, e.g. "slides[0].content.title".
	Path string
	// Title is the title text string.
	Title string
	// WidthEMU is the title placeholder width in EMU.
	WidthEMU int64
	// HeightEMU is the title placeholder height in EMU.
	HeightEMU int64
	// FontSizeHPt is the font size in hundredths of a point (e.g. 3600 = 36pt).
	FontSizeHPt int
	// FontName is the font family name (e.g. "Arial").
	FontName string
}

// DetectTitleWraps checks whether a title text wraps to more than one line
// within its placeholder. Title wrapping is common and usually acceptable,
// so the finding uses action "review" (informational) rather than
// "shrink_or_split".
//
// Returns nil when the title fits on a single line or when measurement
// is not possible.
func DetectTitleWraps(input TitleWrapsInput) *patterns.FitFinding {
	if input.Title == "" || input.WidthEMU <= 0 || input.HeightEMU <= 0 {
		return nil
	}

	fontSizeHPt := input.FontSizeHPt
	if fontSizeHPt <= 0 {
		fontSizeHPt = 2000
	}

	params := textfit.Params{
		WidthEMU:    input.WidthEMU,
		HeightEMU:   input.HeightEMU,
		FontSizeHPt: fontSizeHPt,
		FontName:    input.FontName,
		Paragraphs:  []string{input.Title},
	}

	measuredEMU, err := textfit.MeasureHeight(params)
	if err != nil || measuredEMU <= 0 {
		return nil
	}

	// A single line height is fontSize * lineSpacing (default 1.2).
	// MeasureHeight uses the same default, so compute the single-line
	// threshold identically: fontPt * 1.2, converted to EMU.
	fontSizePt := float64(fontSizeHPt) / 100.0
	singleLineEMU := int64(fontSizePt * 1.2 * 12700) // 12700 EMU per point

	if measuredEMU <= singleLineEMU {
		return nil
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "placeholder",
			Path:    input.Path,
			Code:    patterns.ErrCodeTitleWraps,
			Message: fmt.Sprintf(
				"title wraps to multiple lines (%.0fpt font, %.1f\" wide placeholder)",
				fontSizePt,
				float64(input.WidthEMU)/914400.0,
			),
			Fix: &patterns.FixSuggestion{Kind: "shorten_title"},
		},
		Action: "review",
		Measured: &patterns.Extent{
			WidthEMU:  input.WidthEMU,
			HeightEMU: measuredEMU,
		},
		Allowed: &patterns.Extent{
			WidthEMU:  input.WidthEMU,
			HeightEMU: singleLineEMU,
		},
		OverflowRatio: float64(measuredEMU) / float64(singleLineEMU),
	}
}

// autofitPresent returns true when the autofit mode indicates PowerPoint will
// automatically shrink text to fit (normAutofit or spAutoFit).
func autofitPresent(mode string) bool {
	return mode == "normAutofit" || mode == "spAutoFit"
}

// autofitLabel returns a human-readable label for an autofit mode.
func autofitLabel(mode string) string {
	if mode == "" {
		return "none"
	}
	return mode
}
