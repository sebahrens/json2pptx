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
				Path:    "slides[0].content.body",
				Code:    "text_overflow",
				Message: "text overflows placeholder",
			},
			Action: "shrink_or_split",
		},
		{
			ValidationError: patterns.ValidationError{
				Path:    "slides[0].shape_grid.rows[0].cells[0]",
				Code:    "footer_collision",
				Message: "shape collides with footer",
			},
			Action: "review",
		},
		{
			ValidationError: patterns.ValidationError{
				Path:    "slides[1].content.title",
				Code:    "title_wraps",
				Message: "title wraps to second line",
			},
			Action: "review",
		},
		{
			ValidationError: patterns.ValidationError{
				Path:    "slides[0].content.body",
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
				Path:    "slides[0].content.body",
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
		{"slides[0].content.body", 0},
		{"slides[12].shape_grid", 12},
		{"slides[?]", -1},
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
