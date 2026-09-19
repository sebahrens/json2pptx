package deterministic

import (
	"strings"
	"testing"
)

// go-slide-creator-xx9i. score_deck computed the composition (rhythm) axis and
// then evaluated the quality gate purely on fit findings, so eight consecutive
// identical kpi-3up slides scored 100 and PASSED with composition 55 and
// diagnostics [pattern_run, missing_emphasis]. The single most common LLM deck
// failure — everything looks the same — was already measured in the response
// and had no consequence.

func scoreWithComposition(score int, diags ...CompositionDiagnostic) *DeckScore {
	return &DeckScore{
		OverallScore: 100,
		PerSlide:     []SlideScore{{Index: 0, Score: 100}},
		Composition:  &CompositionResult{Score: score, Diagnostics: diags},
	}
}

func TestQualityGateFailsOnLowComposition(t *testing.T) {
	ds := scoreWithComposition(55,
		CompositionDiagnostic{Code: "pattern_run", Severity: "warning", Message: "repeats 8 consecutive slides"},
		CompositionDiagnostic{Code: "missing_emphasis", Severity: "info", Message: "no emphasis slide"},
	)
	gate := EvaluateQualityGate(ds, nil, DefaultQualityGateCriteria())

	if gate.Passed {
		t.Fatal("a deck with composition 55 passed the gate — the rhythm axis still has no consequence")
	}
	var reason string
	for _, r := range gate.Reasons {
		if strings.HasPrefix(r, "composition ") {
			reason = r
		}
	}
	if reason == "" {
		t.Fatalf("no composition reason in %v", gate.Reasons)
	}
	// The reason must name WHICH rhythm problem, not just the number.
	for _, code := range []string{"pattern_run", "missing_emphasis"} {
		if !strings.Contains(reason, code) {
			t.Errorf("reason %q does not name %s", reason, code)
		}
	}
}

func TestQualityGatePassesOnGoodComposition(t *testing.T) {
	gate := EvaluateQualityGate(scoreWithComposition(100), nil, DefaultQualityGateCriteria())
	if !gate.Passed {
		t.Errorf("a deck with composition 100 failed: %v", gate.Reasons)
	}
	// And exactly at the floor.
	gate = EvaluateQualityGate(scoreWithComposition(DefaultQualityGateMinCompositionScore), nil, DefaultQualityGateCriteria())
	if !gate.Passed {
		t.Errorf("a deck exactly at the floor failed: %v", gate.Reasons)
	}
}

// TestCompositionCriterionSkipsWhenNotMeasured: the slide_indices path leaves
// Composition nil, because a rhythm verdict over three slides is meaningless.
func TestCompositionCriterionSkipsWhenNotMeasured(t *testing.T) {
	ds := &DeckScore{OverallScore: 100, PerSlide: []SlideScore{{Index: 0, Score: 100}}}
	gate := EvaluateQualityGate(ds, nil, DefaultQualityGateCriteria())
	if !gate.Passed {
		t.Errorf("gate failed with no composition measured: %v", gate.Reasons)
	}

	// And a caller that opts out with 0 is not gated either.
	criteria := DefaultQualityGateCriteria()
	criteria.MinCompositionScore = 0
	gate = EvaluateQualityGate(scoreWithComposition(10), nil, criteria)
	if !gate.Passed {
		t.Errorf("min_composition_score 0 should disable the criterion: %v", gate.Reasons)
	}
}

// TestCompositionReasonOrder pins the documented reason order so agents can
// pattern-match on the leading reason.
func TestCompositionReasonOrder(t *testing.T) {
	ds := scoreWithComposition(10, CompositionDiagnostic{Code: "pattern_run"})
	ds.OverallScore = 10 // also trips min_score
	ds.Summary = DeckSummary{ProblemSlidesCount: 0}

	gate := EvaluateQualityGate(ds, nil, DefaultQualityGateCriteria())
	if len(gate.Reasons) < 2 {
		t.Fatalf("expected both a score and a composition reason, got %v", gate.Reasons)
	}
	if !strings.HasPrefix(gate.Reasons[0], "score ") {
		t.Errorf("first reason = %q, want the score reason", gate.Reasons[0])
	}
	if !strings.HasPrefix(gate.Reasons[len(gate.Reasons)-1], "composition ") {
		t.Errorf("last reason = %q, want the composition reason after accent_overload", gate.Reasons[len(gate.Reasons)-1])
	}
}

func TestCompositionCodes(t *testing.T) {
	if got := compositionCodes(nil); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
	if got := compositionCodes(&CompositionResult{}); got != "" {
		t.Errorf("no diagnostics = %q, want empty", got)
	}
	got := compositionCodes(&CompositionResult{Diagnostics: []CompositionDiagnostic{
		{Code: "pattern_run"}, {Code: "pattern_run"}, {Code: "accent_dominance"}, {Code: ""},
	}})
	if got != "accent_dominance, pattern_run" {
		t.Errorf("codes = %q, want deduped and sorted", got)
	}
}
