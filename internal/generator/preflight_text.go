// Package generator: preflight predictors for text-autofit render-time
// findings (text_trimmed, readability_trimmed).
//
// These detectors mirror the trimming logic in applySmartAutofitWithOptions
// without rendering. Given a placeholder's geometry, font, and the paragraphs
// to be populated, they predict whether the engine would trim trailing
// paragraphs to meet the readability floor (62.5%) or to fit the placeholder —
// and, when the text fits but only by shrinking under the readable minimum for
// its role, that too (TEXT_BELOW_READABLE_MIN).
package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
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
	// ViewingMode and TextRole select the readability policy the predicted
	// size is judged against — the same pair the render-time autofit tags its
	// text with. Without them the prediction still reports trimming, but not
	// that the text lands under the readable floor (go-slide-creator-nlrg).
	ViewingMode tokens.ViewingMode
	TextRole    tokens.TextRole
	// ExtraSpacingPt is the inherited per-paragraph space-before in points.
	// The renderer budgets it whenever the shape carries no explicit size, so
	// a preflight that ignores it under-counts the height a dense list needs:
	// fourteen bullets on a 10pt spcBef predicted a 70% font scale where the
	// render applied 50% (go-slide-creator-nlrg).
	ExtraSpacingPt float64
}

// AutofitDensityPolicy returns the readability floor (font scale ×1000) and the
// minimum font scale percent the renderer applies to a bullet list of n
// paragraphs. One definition, used by the render path and by the preflight, so
// the prediction cannot drift from the behaviour it predicts.
func AutofitDensityPolicy(paragraphs int) (readabilityMinScalePct, minFontScalePct int) {
	switch {
	case paragraphs >= denseBulletCount:
		return 45000, 45
	case paragraphs >= moderateBulletCount:
		return 50000, 50
	}
	return readabilityMinScale, 0
}

const (
	// denseBulletCount and moderateBulletCount are the list lengths at which
	// the renderer lowers its readability floor rather than trimming: a dense
	// list is better small than short.
	denseBulletCount    = 12
	moderateBulletCount = 10
)

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

	paraCount := len(input.Paragraphs)
	readabilityFloor, minFontScalePct := AutofitDensityPolicy(paraCount)

	params := textfit.Params{
		WidthEMU:        input.WidthEMU,
		HeightEMU:       input.HeightEMU,
		FontSizeHPt:     input.FontSizeHPt,
		FontName:        input.FontName,
		Paragraphs:      input.Paragraphs,
		ExtraSpacingPt:  input.ExtraSpacingPt,
		MinFontScalePct: minFontScalePct,
	}

	result, err := textfit.Calculate(params)
	if err != nil {
		return nil
	}

	var findings []patterns.FitFinding

	// The readability verdict is independent of trimming: the renderer trims
	// AND then judges the size it ended up with, so both can be true of one
	// placeholder. Reporting only the trim is what left validate saying less
	// than generate about the same deck (go-slide-creator-nlrg).
	if f := readabilityPreflightFinding(input, params, result, paraCount); f != nil {
		findings = append(findings, *f)
	}

	// Overflow path takes precedence — when content overflows even at maximum
	// scaling, the engine trims first.
	if result.Overflow && paraCount > overflowTrimMinParas {
		return append(findings, patterns.FitFinding{
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
		})
	}

	// Readability path — content fits but at a scale below 62.5%, and there
	// are enough paragraphs (>6) that the engine will proactively trim.
	if result.FontScale > 0 && result.FontScale < readabilityFloor && paraCount > readabilityTrimMinParas {
		return append(findings, patterns.FitFinding{
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
		})
	}

	return findings
}

// readabilityPreflightFinding judges the predicted size against the policy, the
// same call emitReadabilityFinding makes at render time.
func readabilityPreflightFinding(input TextAutofitPreflightInput, params textfit.Params, result textfit.FitResult, paraCount int) *patterns.FitFinding {
	if input.TextRole == "" || result.FontScale <= 0 {
		return nil
	}
	baseHPt := params.FontSizeHPt
	if baseHPt <= 0 {
		baseHPt = tokens.BodyDefaultHPt // the size Calculate assumed
	}
	check := textfit.CheckReadability(baseHPt, result.FontScale, input.ViewingMode, input.TextRole)
	return NewReadabilityFinding(ReadabilityFindingInput{
		Path:         input.Path,
		Mode:         input.ViewingMode,
		Role:         input.TextRole,
		EffectiveHPt: check.EffectiveHPt,
		Paragraphs:   paraCount,
		Context:      fmt.Sprintf("predicted autofit %d%% of %.0fpt", result.FontScale/1000, float64(baseHPt)/100.0),
	})
}
