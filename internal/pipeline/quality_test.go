package pipeline

import "testing"

func TestQualityEvidenceApprovalRequiresCurrentAllSlideVision(t *testing.T) {
	base := QualityEvidence{ArtifactSHA256: "a", SchemaValid: true, Generated: true, FitChecked: true, StructuralValid: true, PixelsRendered: true, InspectionBackend: "vision", TotalSlides: 2, ReviewedSlideIDs: []string{"slide-1", "slide-2"}, VisualVerdict: "approved"}
	base.Finalize()
	if !base.Approved || base.NeedsReview {
		t.Fatalf("complete vision evidence not approved: %+v", base)
	}

	for name, mutate := range map[string]func(*QualityEvidence){
		"one-unreviewed": func(e *QualityEvidence) { e.ReviewedSlideIDs = e.ReviewedSlideIDs[:1] },
		"heuristic":      func(e *QualityEvidence) { e.InspectionBackend = "heuristic" },
		"render-failed":  func(e *QualityEvidence) { e.PixelsRendered = false },
		"no-verdict":     func(e *QualityEvidence) { e.VisualVerdict = "needs-review" },
	} {
		t.Run(name, func(t *testing.T) {
			e := base
			e.Reasons = nil
			mutate(&e)
			e.Finalize()
			if e.Approved || !e.NeedsReview {
				t.Fatalf("incomplete evidence approved: %+v", e)
			}
		})
	}
}
