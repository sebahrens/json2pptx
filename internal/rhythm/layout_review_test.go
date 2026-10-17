package rhythm

import (
	"fmt"
	"testing"
)

func layoutReviewCodes(result *Result) map[string]Recommendation {
	out := map[string]Recommendation{}
	for _, rec := range result.Recommendations {
		out[rec.Code] = rec
	}
	return out
}

func TestLayoutReviewSideAndAreaRuns(t *testing.T) {
	slides := make([]Slide, 3)
	for i := range slides {
		slides[i] = Slide{Title: fmt.Sprintf("Finding %d", i), ContentKinds: []string{"chart", "text"}, PaneCount: 2, VisualSide: "left", SplitPct: 30, VisualAreaShare: 0.3}
	}
	codes := layoutReviewCodes(Analyze(slides))
	for _, code := range []string{CodeSameSideRun, CodeLowVisualShareRun} {
		if rec, ok := codes[code]; !ok || rec.SlideIndex != 0 || rec.Severity != "info" {
			t.Errorf("missing located advisory %s: %+v", code, codes)
		}
	}
	slides[1].VisualSide = "right"
	slides[1].VisualAreaShare = 0
	codes = layoutReviewCodes(Analyze(slides))
	if _, ok := codes[CodeSameSideRun]; ok {
		t.Error("alternating sides formed a run")
	}
	if _, ok := codes[CodeLowVisualShareRun]; ok {
		t.Error("unknown geometry counted as an empty exhibit")
	}
	slides[1] = Slide{Role: "section", Title: "Next"}
	if _, ok := layoutReviewCodes(Analyze(slides))[CodeSameSideRun]; ok {
		t.Error("navigation did not break the run")
	}
}

func TestLayoutReviewTakeawayThresholds(t *testing.T) {
	slides := []Slide{
		{Title: "Revenue grew faster than costs", Takeaway: "Revenue grew faster than costs", HasTakeaway: true},
		{Title: "Retention improves", Takeaway: "Expand the pilot next quarter", HasTakeaway: true},
		{Title: "Margins improve"}, {Title: "Costs fall"},
	}
	codes := layoutReviewCodes(Analyze(slides))
	if codes[CodeTakeawayRestatesTitle].SlideIndex != 0 || codes[CodeTakeawayRestatesTitle].Code == "" {
		t.Fatal("duplicate takeaway not located")
	}
	if _, ok := codes[CodeTakeawayHeavyDeck]; ok {
		t.Error("exactly half the deck is not more than half")
	}
	slides[3].HasTakeaway = true
	if _, ok := layoutReviewCodes(Analyze(slides))[CodeTakeawayHeavyDeck]; !ok {
		t.Error("three of four takeaway bands not reported")
	}
	slides[3].Appendix = true
	slides[2].Role = "section"
	if tokenOverlap("Costs fell by 20 percent", "Review hiring") != 0 {
		t.Error("unrelated takeaway overlaps")
	}
}

func TestLayoutReviewVarietyBoundAndFingerprint(t *testing.T) {
	slides := make([]Slide, 7)
	for i := range slides {
		slides[i] = Slide{Title: fmt.Sprintf("Claim %d", i), PatternName: fmt.Sprintf("family-%d", i), HasPattern: true}
	}
	r := Analyze(slides)
	if _, ok := layoutReviewCodes(r)[CodeLayoutVarietyHigh]; !ok {
		t.Fatal("seven families on a short deck were not reported")
	}
	if score := computeCompositionScore(&Result{Recommendations: []Recommendation{{Code: CodeLayoutVarietyHigh}}}); score != 95 {
		t.Fatalf("variety must not earn an uncapped bonus: %d", score)
	}
	for len(slides) < 21 {
		slides = append(slides, Slide{Title: "More evidence", PatternName: "family-0", HasPattern: true})
	}
	if _, ok := layoutReviewCodes(Analyze(slides))[CodeLayoutVarietyHigh]; ok {
		t.Error("short-deck variety limit applied to21slides")
	}
	a := Slide{PaneCount: 2, VisualSide: "left", SplitPct: 60, ContentKinds: []string{"chart", "text"}}
	b := a
	b.VisualSide = "right"
	if FingerprintKey(a) == FingerprintKey(b) {
		t.Error("opposite visual sides have the same fingerprint")
	}
	b = a
	b.SplitPct = 70
	if FingerprintKey(a) == FingerprintKey(b) {
		t.Error("different splits have the same fingerprint")
	}
}
