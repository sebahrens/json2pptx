// Package generator: preflight predictors for text-autofit render-time
// findings (text_trimmed, readability_trimmed).
//
// These detectors mirror the trimming logic in applySmartAutofitWithOptions
// without rendering. Given a placeholder's geometry, font, and the paragraphs
// to be populated, they predict whether the engine would trim trailing
// paragraphs to meet the readability floor (62.5%) or to fit the placeholder.
package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// readabilityMinScale matches text_autofit.go: 62.5% (62500/1000) — below
// this scale, trimForReadability removes trailing paragraphs.
const readabilityMinScale = 62500

// readabilityTrimMinParas matches trimForReadability: needs > 6 paragraphs to
// consider readability trimming.
const readabilityTrimMinParas = 6

// overflowTrimMinParas matches trimOverflowParagraphs: needs > 2 paragraphs
// to consider trimming trailing content.
const overflowTrimMinParas = 2

// TextAutofitPreflightInput describes a placeholder text body whose
// render-time autofit behaviour should be predicted.
type TextAutofitPreflightInput struct {
	// Path is the JSON pointer to the populated placeholder.
	Path string
	// Paragraphs is the list of paragraph strings that will populate the placeholder.
	Paragraphs []string
	// WidthEMU is the placeholder width in EMU.
	WidthEMU int64
	// HeightEMU is the placeholder height in EMU.
	HeightEMU int64
	// FontSizeHPt is the font size in hundredths of a point (e.g. 2000 = 20pt).
	FontSizeHPt int
	// FontName is the font family name.
	FontName string
}

// DetectTextAutofitPreflight predicts whether applySmartAutofitWithOptions
// would emit readability_trimmed or text_trimmed for the given placeholder
// content. Returns a slice with zero, one, or both findings.
//
// Prediction mirrors text_autofit.go:
//   - If textfit.Calculate reports Overflow=true AND paragraphs > 2, the
//     engine will trim trailing paragraphs (text_trimmed).
//   - Else if the resulting FontScale < 62500 AND paragraphs > 6, the engine
//     will trim for readability (readability_trimmed).
//
// When textfit.Calculate is unavailable (font cache miss) or any required
// dimension is zero, returns nil — these are conservative-no-finding paths
// in the renderer too.
func DetectTextAutofitPreflight(input TextAutofitPreflightInput) []patterns.FitFinding {
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

	result, err := textfit.Calculate(params)
	if err != nil {
		return nil
	}

	paraCount := len(input.Paragraphs)

	// Overflow path takes precedence — when content overflows even at maximum
	// scaling, the engine trims first.
	if result.Overflow && paraCount > overflowTrimMinParas {
		return []patterns.FitFinding{{
			ValidationError: patterns.ValidationError{
				Path: input.Path,
				Code: patterns.ErrCodeTextTrimmed,
				Message: fmt.Sprintf(
					"predicted: trailing paragraphs will be trimmed to fit placeholder (%d paragraphs, overflow at max scaling)",
					paraCount,
				),
				Fix: &patterns.FixSuggestion{
					Kind:   "reduce_text",
					Params: map[string]any{"paragraphs": paraCount},
				},
			},
			Action: "review",
		}}
	}

	// Readability path — content fits but at a scale below 62.5%, and there
	// are enough paragraphs (>6) that the engine will proactively trim.
	if result.FontScale > 0 && result.FontScale < readabilityMinScale && paraCount > readabilityTrimMinParas {
		return []patterns.FitFinding{{
			ValidationError: patterns.ValidationError{
				Path: input.Path,
				Code: patterns.ErrCodeReadabilityTrimmed,
				Message: fmt.Sprintf(
					"predicted: paragraphs will be trimmed for readability (font would scale to %d%%, threshold %d%%)",
					result.FontScale/1000, readabilityMinScale/1000,
				),
				Fix: &patterns.FixSuggestion{
					Kind:   "reduce_text",
					Params: map[string]any{"paragraphs": paraCount, "predicted_font_scale_pct": result.FontScale / 1000},
				},
			},
			Action: "info",
		}}
	}

	return nil
}
