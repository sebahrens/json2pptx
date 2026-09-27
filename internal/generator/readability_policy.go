package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// Readability policy (go-slide-creator-vbic).
//
// tokens.MinReadableHPt / textfit.CheckReadability define per-role floors for
// each viewing mode, but nothing selected a mode or a role, so every check
// ran at the 10pt report default and FitResult.Readable was never read. The
// deck-level viewing_mode ("present" default, or "read") now selects the
// policy, fit sites tag their text role, and text that ends up below its
// floor is reported as TEXT_BELOW_READABLE_MIN. The policy never changes the
// fitted size itself (see textfit.Calculate) — it reports, it does not trim.

// withReadabilityPolicy sets the viewing mode and text role the autofit
// result is judged against.
func withReadabilityPolicy(mode tokens.ViewingMode, role tokens.TextRole) autofitOption {
	return func(c *autofitConfig) {
		c.viewingMode = mode
		c.textRole = role
	}
}

// ReadabilityFindingInput describes text measured against the readability
// policy.
type ReadabilityFindingInput struct {
	Path string
	Mode tokens.ViewingMode
	Role tokens.TextRole
	// EffectiveHPt is the rendered size (hundredths of a point) after
	// autofit / predicted shrink.
	EffectiveHPt int
	// Paragraphs is the paragraph count (drives the split vs shorten hint).
	Paragraphs int
	// Context is an optional short description (e.g. "autofit 60%").
	Context string
	// MeasurementSource distinguishes exact authored/generated sizes from
	// predicted autofit, which can overstate shrinkage on pattern cells.
	MeasurementSource string
}

// NewReadabilityFinding returns a TEXT_BELOW_READABLE_MIN finding when the
// effective size is below the policy floor for the role, else nil.
func NewReadabilityFinding(in ReadabilityFindingInput) *patterns.FitFinding {
	minHPt := tokens.MinReadableHPt(in.Mode, in.Role)
	if in.EffectiveHPt <= 0 || in.EffectiveHPt >= minHPt {
		return nil
	}
	role := in.Role
	if role == "" {
		role = tokens.TextRoleBody
	}
	strategy := "shorten"
	if in.Paragraphs > 3 {
		strategy = "split"
	}
	actualPt := float64(in.EffectiveHPt) / 100.0
	minPt := float64(minHPt) / 100.0
	modeName := tokens.ViewingModeInputName(in.Mode)
	msg := fmt.Sprintf("%s text renders at %.1fpt, below the %.0fpt minimum for viewing_mode %q", role, actualPt, minPt, modeName)
	if in.Context != "" {
		msg += " (" + in.Context + ")"
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    in.Path,
			Code:    patterns.ErrCodeTextBelowReadableMin,
			Message: msg + "; " + strategy + " the text",
			Fix: &patterns.FixSuggestion{
				Kind: "reduce_text",
				Params: map[string]any{
					"strategy":           strategy,
					"role":               string(role),
					"actual_pt":          actualPt,
					"min_pt":             minPt,
					"viewing_mode":       modeName,
					"measurement_source": in.MeasurementSource,
				},
			},
		},
		// Deliberately advisory, even far below the floor: the size reported here
		// comes from the autofit PREDICTION, which over-predicts shrink on thin
		// band cells (a legible 24pt axis label on
		// examples/sovereign-ai-strategy.json slide 9 is predicted at 7.2pt).
		// Blocking a deck on a predictor that is wrong in the visible direction
		// is how go-slide-creator-lmpu started; the prediction's accuracy is its
		// own problem to fix before this can escalate.
		Action: "review",
	}
}

// emitReadabilityFinding reports placeholder autofit below the policy floor.
// Normalized ordinary native body text uses a publication refusal; generic
// predictions remain advisory.
func emitReadabilityFinding(cfg *autofitConfig, shape *shapeXML, params textfit.Params, result textfit.FitResult, paragraphs int) {
	if cfg.findings == nil || cfg.textRole == "" || result.FontScale <= 0 {
		return
	}
	baseHPt := params.FontSizeHPt
	if baseHPt <= 0 {
		baseHPt = tokens.BodyDefaultHPt // the size Calculate assumed
	}
	// Ordinary native body runs are written with this stored font scale, not
	// merely predicted against a future pattern frame. Include smaller populated
	// child runs, but never unused list levels, empty prompts or decorative text.
	nativeBody := cfg.bodyTypography && cfg.textRole == tokens.TextRoleBody
	if nativeBody && shape != nil && shape.TextBody != nil {
		for _, paragraph := range shape.TextBody.Paragraphs {
			for _, run := range paragraph.Runs {
				if strings.TrimSpace(run.Text) == "" || run.RunProperties == nil {
					continue
				}
				if size, err := strconv.Atoi(run.RunProperties.FontSize); err == nil && size > 0 && size < baseHPt {
					baseHPt = size
				}
			}
		}
	}
	check := textfit.CheckReadability(baseHPt, result.FontScale, cfg.viewingMode, cfg.textRole)
	f := NewReadabilityFinding(ReadabilityFindingInput{
		Path:              cfg.findingPath,
		Mode:              cfg.viewingMode,
		Role:              cfg.textRole,
		EffectiveHPt:      check.EffectiveHPt,
		Paragraphs:        paragraphs,
		Context:           fmt.Sprintf("autofit %d%% of %.0fpt", result.FontScale/1000, float64(baseHPt)/100.0),
		MeasurementSource: "generated",
	})
	if f != nil {
		if nativeBody {
			f.Action = "refuse"
			f.Message = strings.TrimSuffix(strings.TrimSuffix(f.Message, "; shorten the text"), "; split the text") + "; preserve all source at a readable size"
			f.Fix.Params["measurement_source"] = "generated_native_body"
		}
		*cfg.findings = append(*cfg.findings, *f)
	}
}
