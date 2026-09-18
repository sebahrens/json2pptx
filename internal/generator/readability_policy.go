package generator

import (
	"fmt"

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
					"strategy":     strategy,
					"role":         string(role),
					"actual_pt":    actualPt,
					"min_pt":       minPt,
					"viewing_mode": modeName,
				},
			},
		},
		Action: "review",
	}
}

// emitReadabilityFinding reports a placeholder autofit result that shrank the
// text below the policy floor. Template-native sizes that are already below
// the floor are not reported: shortening would not change them.
func emitReadabilityFinding(cfg *autofitConfig, params textfit.Params, result textfit.FitResult, paragraphs int) {
	if cfg.findings == nil || cfg.textRole == "" || result.FontScale <= 0 {
		return
	}
	baseHPt := params.FontSizeHPt
	if baseHPt <= 0 {
		baseHPt = tokens.BodyDefaultHPt // the size Calculate assumed
	}
	check := textfit.CheckReadability(baseHPt, result.FontScale, cfg.viewingMode, cfg.textRole)
	f := NewReadabilityFinding(ReadabilityFindingInput{
		Path:         cfg.findingPath,
		Mode:         cfg.viewingMode,
		Role:         cfg.textRole,
		EffectiveHPt: check.EffectiveHPt,
		Paragraphs:   paragraphs,
		Context:      fmt.Sprintf("autofit %d%% of %.0fpt", result.FontScale/1000, float64(baseHPt)/100.0),
	})
	if f != nil {
		*cfg.findings = append(*cfg.findings, *f)
	}
}
