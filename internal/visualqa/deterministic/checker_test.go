package deterministic

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestScoreFromFindings_NoFindings(t *testing.T) {
	ds := ScoreFromFindings(nil, 3)
	if ds.OverallScore != 100 {
		t.Errorf("overall = %d, want 100", ds.OverallScore)
	}
	if ds.Summary.ProblemSlidesCount != 0 {
		t.Errorf("problem slides = %d, want 0", ds.Summary.ProblemSlidesCount)
	}
	if ds.Summary.SlideCount != 3 {
		t.Errorf("slide count = %d, want 3", ds.Summary.SlideCount)
	}
	if len(ds.PerSlide) != 3 {
		t.Errorf("per_slide len = %d, want 3", len(ds.PerSlide))
	}
	for _, ss := range ds.PerSlide {
		if ss.Score != 100 {
			t.Errorf("slide %d score = %d, want 100", ss.Index, ss.Score)
		}
	}
}

func TestScoreFromFindings_WithFindings(t *testing.T) {
	findings := []patterns.FitFinding{
		{
			ValidationError: patterns.ValidationError{
				Path:    "/slides/0/content/body",
				Code:    "text_overflow",
				Message: "text overflows placeholder",
			},
			Action: "shrink_or_split",
		},
		{
			ValidationError: patterns.ValidationError{
				Path:    "/slides/0/shape_grid/rows/0/cells/0",
				Code:    "footer_collision",
				Message: "shape collides with footer",
			},
			Action: "review",
		},
		{
			ValidationError: patterns.ValidationError{
				Path:    "/slides/1/content/title",
				Code:    "title_wraps",
				Message: "title wraps to second line",
			},
			Action: "review",
		},
		{
			ValidationError: patterns.ValidationError{
				Path:    "/slides/0/content/body",
				Code:    "contrast_autofixed",
				Message: "auto-fixed low-contrast text",
			},
			Action: "info",
		},
	}

	ds := ScoreFromFindings(findings, 3)

	// Slide 0: shrink_or_split(-15) + review(-5) + info(0) = 80
	if ds.PerSlide[0].Score != 80 {
		t.Errorf("slide 0 score = %d, want 80", ds.PerSlide[0].Score)
	}
	// Slide 1: review(-5) = 95
	if ds.PerSlide[1].Score != 95 {
		t.Errorf("slide 1 score = %d, want 95", ds.PerSlide[1].Score)
	}
	// Slide 2: no findings = 100
	if ds.PerSlide[2].Score != 100 {
		t.Errorf("slide 2 score = %d, want 100", ds.PerSlide[2].Score)
	}
	// Overall: (80 + 95 + 100) / 3 = 91
	if ds.OverallScore != 91 {
		t.Errorf("overall = %d, want 91", ds.OverallScore)
	}
	if ds.Summary.ProblemSlidesCount != 2 {
		t.Errorf("problem slides = %d, want 2", ds.Summary.ProblemSlidesCount)
	}

	// Check top_codes.
	if len(ds.Summary.TopCodes) == 0 {
		t.Fatal("top_codes is empty")
	}
}

func TestScoreFromFindings_RefuseClamps(t *testing.T) {
	// 5 refuse findings = 5*25 = 125 deducted, clamped to 0.
	var findings []patterns.FitFinding
	for i := 0; i < 5; i++ {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    "/slides/0/content/body",
				Code:    "text_overflow",
				Message: "overflow",
			},
			Action: "refuse",
		})
	}

	ds := ScoreFromFindings(findings, 1)
	if ds.PerSlide[0].Score != 0 {
		t.Errorf("slide 0 score = %d, want 0", ds.PerSlide[0].Score)
	}
	if ds.OverallScore != 0 {
		t.Errorf("overall = %d, want 0", ds.OverallScore)
	}
}

func TestScoreFromFindings_ZeroSlides(t *testing.T) {
	ds := ScoreFromFindings(nil, 0)
	if ds.OverallScore != 100 {
		t.Errorf("overall = %d, want 100", ds.OverallScore)
	}
	if len(ds.PerSlide) != 0 {
		t.Errorf("per_slide len = %d, want 0", len(ds.PerSlide))
	}
}

func TestScoreFinding_Severity(t *testing.T) {
	tests := []struct {
		action   string
		wantSev  string
	}{
		{"refuse", "error"},
		{"shrink_or_split", "warning"},
		{"review", "warning"},
		{"info", "info"},
		{"unknown", "info"},
	}
	for _, tt := range tests {
		got := actionToSeverity(tt.action)
		if got != tt.wantSev {
			t.Errorf("actionToSeverity(%q) = %q, want %q", tt.action, got, tt.wantSev)
		}
	}
}

func TestSlideIndexFromPath(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/slides/0/content/body", 0},
		{"/slides/12/shape_grid", 12},
		{"/slides/?", -1},
		{"other", -1},
		{"", -1},
	}
	for _, tt := range tests {
		got := slideIndexFromPath(tt.path)
		if got != tt.want {
			t.Errorf("slideIndexFromPath(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

// TestDeckScore_ContractShape verifies the JSON field names are stable.
func TestDeckScore_ContractShape(t *testing.T) {
	ds := &DeckScore{
		OverallScore: 85,
		PerSlide: []SlideScore{{
			Index: 0, Score: 85,
			Findings: []ScoreFinding{{
				Code: "text_overflow", Severity: "warning", Message: "overflow",
			}},
		}},
		Summary: DeckSummary{
			TopCodes:           []CodeCount{{Code: "text_overflow", Count: 1}},
			SlideCount:         1,
			ProblemSlidesCount: 1,
		},
		ModeUsed: "deterministic",
	}

	b, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("DeckScore JSON is not an object: %v", err)
	}

	for _, field := range []string{"overall_score", "per_slide", "summary", "mode_used"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("DeckScore JSON missing stable field %q", field)
		}
	}
}

// TestScoreFromFindings_FindingsNeverNullInJSON guards against silently
// returning `"findings": null` for slides with no findings. The score command
// must use empty arrays so consumers can distinguish "no findings found" from
// "field missing / scoring skipped".
func TestScoreFromFindings_FindingsNeverNullInJSON(t *testing.T) {
	ds := ScoreFromFindings(nil, 3)

	b, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed struct {
		PerSlide []struct {
			Findings json.RawMessage `json:"findings"`
		} `json:"per_slide"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.PerSlide) != 3 {
		t.Fatalf("per_slide len = %d, want 3", len(parsed.PerSlide))
	}
	for i, ss := range parsed.PerSlide {
		if string(ss.Findings) == "null" {
			t.Errorf("slide %d: findings marshaled as null; want []", i)
		}
		if string(ss.Findings) != "[]" {
			t.Errorf("slide %d: findings = %s, want []", i, string(ss.Findings))
		}
	}
}

// TestScoreFromFindingsForIndices_FindingsNeverNullInJSON mirrors the guard
// above for the per-slide-indices code path.
func TestScoreFromFindingsForIndices_FindingsNeverNullInJSON(t *testing.T) {
	ds := ScoreFromFindingsForIndices(nil, 5, []int{1, 3})

	b, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed struct {
		PerSlide []struct {
			Findings json.RawMessage `json:"findings"`
		} `json:"per_slide"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.PerSlide) != 2 {
		t.Fatalf("per_slide len = %d, want 2", len(parsed.PerSlide))
	}
	for i, ss := range parsed.PerSlide {
		if string(ss.Findings) != "[]" {
			t.Errorf("subset slide %d: findings = %s, want []", i, string(ss.Findings))
		}
	}
}

// TestEvaluateQualityGate covers the four-criterion ship-quality gate: score
// floor, P0/P1 caps, takeaway-on-charts, and accent_overload veto. Each case
// flips exactly one criterion so the test fails loudly if the gate logic
// silently changes its severity vocabulary or finding-code matching.
func TestEvaluateQualityGate(t *testing.T) {
	criteria := DefaultQualityGateCriteria()

	tests := []struct {
		name     string
		score    int
		findings []patterns.FitFinding
		passed   bool
		// reasonContains is a substring expected in the first reason on
		// failure; "" allowed when passed=true.
		reasonContains string
	}{
		{
			name:     "clean deck passes",
			score:    90,
			findings: nil,
			passed:   true,
		},
		{
			name:           "score below floor fails",
			score:          79,
			findings:       nil,
			passed:         false,
			reasonContains: "score 79 < min_score 80",
		},
		{
			name:  "p0 finding fails",
			score: 100,
			findings: []patterns.FitFinding{
				{ValidationError: patterns.ValidationError{Path: "/slides/0/content/body", Code: "text_overflow"}, Action: "refuse"},
			},
			passed:         false,
			reasonContains: "P0 (refuse)",
		},
		{
			name:  "p1 finding fails (zero tolerance)",
			score: 100,
			findings: []patterns.FitFinding{
				{ValidationError: patterns.ValidationError{Path: "/slides/0/content/body", Code: "text_overflow"}, Action: "shrink_or_split"},
			},
			passed:         false,
			reasonContains: "P1 (shrink_or_split)",
		},
		{
			name:  "takeaway_missing fails",
			score: 100,
			findings: []patterns.FitFinding{
				{ValidationError: patterns.ValidationError{Path: "/slides/0", Code: patterns.ErrCodeTakeawayMissing}, Action: "review"},
			},
			passed:         false,
			reasonContains: "missing takeaway",
		},
		{
			name:  "accent_overload fails",
			score: 100,
			findings: []patterns.FitFinding{
				{ValidationError: patterns.ValidationError{Path: "/slides/0/shape_grid", Code: patterns.ErrCodeAccentOverload}, Action: "review"},
			},
			passed:         false,
			reasonContains: "accent_overload",
		},
		{
			name:  "review-action findings alone are fine",
			score: 90,
			findings: []patterns.FitFinding{
				{ValidationError: patterns.ValidationError{Path: "/slides/0/content/title", Code: "title_wraps"}, Action: "review"},
			},
			passed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := &DeckScore{OverallScore: tc.score}
			gate := EvaluateQualityGate(ds, tc.findings, criteria)
			if gate == nil {
				t.Fatal("EvaluateQualityGate returned nil")
			}
			if gate.Passed != tc.passed {
				t.Errorf("Passed = %v, want %v (reasons=%v)", gate.Passed, tc.passed, gate.Reasons)
			}
			if !tc.passed {
				if len(gate.Reasons) == 0 {
					t.Fatalf("expected at least one reason on failure")
				}
				found := false
				for _, r := range gate.Reasons {
					if tc.reasonContains != "" && contains(r, tc.reasonContains) {
						found = true
						break
					}
				}
				if tc.reasonContains != "" && !found {
					t.Errorf("no reason contained %q; got %v", tc.reasonContains, gate.Reasons)
				}
			}
			if gate.Criteria.MinScore != criteria.MinScore {
				t.Errorf("Criteria.MinScore = %d, want %d (criteria not echoed)", gate.Criteria.MinScore, criteria.MinScore)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestEvaluateQualityGate_NilDeck guards the nil-safe path — score_deck never
// passes nil today, but the function is exported and callable from tests.
func TestEvaluateQualityGate_NilDeck(t *testing.T) {
	gate := EvaluateQualityGate(nil, nil, DefaultQualityGateCriteria())
	if gate == nil {
		t.Fatal("expected non-nil gate even for nil deck score")
	}
	if gate.Passed {
		t.Errorf("nil deck should not pass the gate")
	}
}

// TestQualityGate_ReasonOrderDeterministic ensures the reason list is
// reproducibly ordered (score → P0 → P1 → takeaway → accent_overload) so
// agents can pattern-match on the leading reason and the response_fingerprint
// stays stable across runs of an identical input.
func TestQualityGate_ReasonOrderDeterministic(t *testing.T) {
	findings := []patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Path: "/slides/0/shape_grid", Code: patterns.ErrCodeAccentOverload}, Action: "review"},
		{ValidationError: patterns.ValidationError{Path: "/slides/0", Code: patterns.ErrCodeTakeawayMissing}, Action: "review"},
		{ValidationError: patterns.ValidationError{Path: "/slides/0/content/body", Code: "text_overflow"}, Action: "refuse"},
		{ValidationError: patterns.ValidationError{Path: "/slides/0/content/body", Code: "text_overflow"}, Action: "shrink_or_split"},
	}
	ds := &DeckScore{OverallScore: 50}
	gate := EvaluateQualityGate(ds, findings, DefaultQualityGateCriteria())
	if gate.Passed {
		t.Fatal("expected gate to fail")
	}
	wantOrder := []string{"score ", "P0 (refuse)", "P1 (shrink_or_split)", "missing takeaway", "accent_overload"}
	if len(gate.Reasons) != len(wantOrder) {
		t.Fatalf("reasons len = %d, want %d; got %v", len(gate.Reasons), len(wantOrder), gate.Reasons)
	}
	for i, want := range wantOrder {
		if !contains(gate.Reasons[i], want) {
			t.Errorf("reasons[%d] = %q, want substring %q", i, gate.Reasons[i], want)
		}
	}
}

func TestFormatTopCodes(t *testing.T) {
	got := FormatTopCodes(nil)
	if got != "no issues" {
		t.Errorf("FormatTopCodes(nil) = %q, want 'no issues'", got)
	}

	got = FormatTopCodes([]CodeCount{{Code: "text_overflow", Count: 3}, {Code: "footer_collision", Count: 1}})
	if got != "text_overflow(3), footer_collision(1)" {
		t.Errorf("FormatTopCodes = %q", got)
	}
}
