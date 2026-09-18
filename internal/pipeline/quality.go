package pipeline

import "sort"

// QualityEvidence reports independent stages. A score is never promoted into
// pixel evidence; visual approval requires current, complete all-slide review.
type QualityEvidence struct {
	ArtifactSHA256    string   `json:"artifact_sha256,omitempty"`
	Revision          string   `json:"revision,omitempty"`
	SchemaValid       bool     `json:"schema_valid"`
	Generated         bool     `json:"generated"`
	FitChecked        bool     `json:"fit_checked"`
	StructuralValid   bool     `json:"structural_valid"`
	PixelsRendered    bool     `json:"pixels_rendered"`
	VisuallyInspected bool     `json:"visually_inspected"`
	InspectionBackend string   `json:"inspection_backend,omitempty"`
	ReviewedSlideIDs  []string `json:"reviewed_slide_ids"`
	TotalSlides       int      `json:"total_slides"`
	VisualVerdict     string   `json:"visual_verdict,omitempty"`
	Approved          bool     `json:"approved"`
	NeedsReview       bool     `json:"needs_review"`
	Reasons           []string `json:"reasons,omitempty"`
}

func (e *QualityEvidence) Finalize() {
	if e.ReviewedSlideIDs == nil {
		e.ReviewedSlideIDs = []string{}
	}
	sort.Strings(e.ReviewedSlideIDs)
	allSlides := e.TotalSlides > 0 && len(e.ReviewedSlideIDs) == e.TotalSlides
	e.VisuallyInspected = e.PixelsRendered && allSlides
	e.Approved = e.SchemaValid && e.Generated && e.FitChecked && e.StructuralValid && e.VisuallyInspected && approvingBackend(e.InspectionBackend) && e.VisualVerdict == "approved"
	e.NeedsReview = !e.Approved
	if !e.PixelsRendered {
		e.Reasons = appendUnique(e.Reasons, "current artifact was not pixel-rendered")
	}
	if e.PixelsRendered && !allSlides {
		e.Reasons = appendUnique(e.Reasons, "visual inspection did not cover every slide")
	}
	if e.VisuallyInspected && !approvingBackend(e.InspectionBackend) {
		e.Reasons = appendUnique(e.Reasons, "inspection backend was heuristic, not vision")
	}
	if e.VisualVerdict != "approved" {
		e.Reasons = appendUnique(e.Reasons, "no explicit visual approval verdict for the current artifact")
	}
}

// approvingBackend reports whether an inspection backend can carry a visual
// approval: a vision provider, or an explicitly recorded host/manual review
// (submit_visual_review). Heuristic pixel checks never approve.
func approvingBackend(backend string) bool {
	switch backend {
	case "vision", "provider", "host", "manual":
		return true
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
