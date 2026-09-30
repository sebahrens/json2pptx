package generator

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// ReadabilityEvidence is the measurement behind an unreadable-text refusal:
// the text role, the size the text renders at and the floor it missed, and
// the paragraph text itself so a caller holding the authored source can say
// which field to edit.
type ReadabilityEvidence struct {
	Role              string  `json:"role,omitempty"`
	ActualPt          float64 `json:"actual_pt,omitempty"`
	MinPt             float64 `json:"min_pt,omitempty"`
	ViewingMode       string  `json:"viewing_mode,omitempty"`
	MeasurementSource string  `json:"measurement_source,omitempty"`
	Text              string  `json:"text,omitempty"`
}

// ReadabilityRefusal is the error generation returns when it refuses text
// below its readable floor. It unwraps to the refusal's
// *patterns.ValidationError, so errors.As for that type keeps working, and it
// reads exactly like it: the evidence travels beside the message instead of
// changing it (go-slide-creator-b7qqg.4).
type ReadabilityRefusal struct {
	Loss     *patterns.ValidationError
	Evidence ReadabilityEvidence
}

func (r *ReadabilityRefusal) Error() string { return r.Loss.Error() }

// Unwrap exposes the underlying refusal.
func (r *ReadabilityRefusal) Unwrap() error { return r.Loss }

// recordReadabilityEvidence remembers the measurement behind the finding at
// path.
func (ctx *singlePassContext) recordReadabilityEvidence(path string, ev ReadabilityEvidence) {
	if ctx.readabilityEvidence == nil {
		ctx.readabilityEvidence = map[string]ReadabilityEvidence{}
	}
	ctx.readabilityEvidence[path] = ev
}

// readabilityEvidenceFor returns the measurement behind a refusal finding:
// the grid evidence recorded at emission, else the readability params a
// native-body finding carries on its fix.
func (ctx *singlePassContext) readabilityEvidenceFor(f patterns.FitFinding) ReadabilityEvidence {
	if ev, ok := ctx.readabilityEvidence[f.Path]; ok {
		return ev
	}
	var ev ReadabilityEvidence
	if f.Fix == nil {
		return ev
	}
	p := f.Fix.Params
	ev.Role, _ = p["role"].(string)
	ev.ActualPt, _ = p["actual_pt"].(float64)
	ev.MinPt, _ = p["min_pt"].(float64)
	ev.ViewingMode, _ = p["viewing_mode"].(string)
	ev.MeasurementSource, _ = p["measurement_source"].(string)
	ev.Text, _ = p["rendered_shape_text"].(string)
	return ev
}
