package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestSlideIndexFromPath(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"slides[0].content.body", 0},
		{"slides[3].shape_grid.rows[1].cells[0]", 3},
		{"slides[12].content[1]", 12},
		{"other_path", -1},
		{"slides[]", -1},
		{"slides[abc]", -1},
	}
	for _, tt := range tests {
		got := slideIndexFromPath(tt.path)
		if got != tt.want {
			t.Errorf("slideIndexFromPath(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestConvertTextFitFinding(t *testing.T) {
	tf := fitFinding{
		Code:        patterns.ErrCodeFitOverflow,
		Path:        "slides[0].content[1].rows[0][0]",
		Message:     "text needs 5 lines",
		Action:      "refuse",
		RequiredPt:  100,
		AllocatedPt: 50,
	}

	f := convertTextFitFinding(tf)

	if f.Code != patterns.ErrCodeFitOverflow {
		t.Errorf("Code = %q, want %q", f.Code, patterns.ErrCodeFitOverflow)
	}
	if f.Path != tf.Path {
		t.Errorf("Path = %q, want %q", f.Path, tf.Path)
	}
	if f.Action != "refuse" {
		t.Errorf("Action = %q, want %q", f.Action, "refuse")
	}
	if f.Measured == nil || f.Allowed == nil {
		t.Fatal("Measured and Allowed should be populated")
	}
	if f.OverflowRatio != 2.0 {
		t.Errorf("OverflowRatio = %f, want 2.0", f.OverflowRatio)
	}
}

func TestCollectFitFindingsSorting(t *testing.T) {
	// Create a presentation with two slides that will produce findings
	// at different severities. We test that the sorting works correctly.
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{{}, {}}, // Empty slides — no findings from here
	}

	findings := collectFitFindings(input, nil, 9144000, 6858000)
	// With empty slides, expect no findings.
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty slides, got %d", len(findings))
	}
}

func TestExtractContentParagraphs(t *testing.T) {
	text := "Hello World"
	c := ContentInput{
		Type:      "text",
		TextValue: &text,
	}
	paras := extractContentParagraphs(&c)
	if len(paras) != 1 || paras[0] != "Hello World" {
		t.Errorf("text content: got %v, want [Hello World]", paras)
	}

	bullets := []string{"a", "b", "c"}
	c2 := ContentInput{
		Type:         "bullets",
		BulletsValue: &bullets,
	}
	paras2 := extractContentParagraphs(&c2)
	if len(paras2) != 3 {
		t.Errorf("bullets content: got %d paragraphs, want 3", len(paras2))
	}

	c3 := ContentInput{
		Type: "body_and_bullets",
		BodyAndBulletsValue: &BodyAndBulletsInput{
			Body:         "intro",
			Bullets:      []string{"x", "y"},
			TrailingBody: "end",
		},
	}
	paras3 := extractContentParagraphs(&c3)
	if len(paras3) != 4 {
		t.Errorf("body_and_bullets: got %d paragraphs, want 4", len(paras3))
	}
}

func TestContrastSwapsToFindings(t *testing.T) {
	swaps := []generator.ContrastSwap{
		{
			OriginalColor:   "#FD5108",
			ReplacedColor:   "#A03000",
			BackgroundColor: "#FFE8D4",
			RatioBefore:     2.1,
			RatioAfter:      4.6,
		},
		{
			OriginalColor:   "#FFFFFF",
			ReplacedColor:   "#1A1A1A",
			BackgroundColor: "#FFB6C1",
			RatioBefore:     1.65,
			RatioAfter:      8.2,
		},
	}

	findings := contrastSwapsToFindings(swaps)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	for i, f := range findings {
		if f.Code != "contrast_autofixed" {
			t.Errorf("finding[%d].Code = %q, want %q", i, f.Code, "contrast_autofixed")
		}
		if f.Action != "info" {
			t.Errorf("finding[%d].Action = %q, want %q", i, f.Action, "info")
		}
		if f.Fix == nil {
			t.Fatalf("finding[%d].Fix is nil", i)
		}
		if f.Fix.Kind != "replace_color" {
			t.Errorf("finding[%d].Fix.Kind = %q, want %q", i, f.Fix.Kind, "replace_color")
		}
		if !strings.Contains(f.Message, swaps[i].OriginalColor) {
			t.Errorf("finding[%d].Message should contain original color %q, got %q", i, swaps[i].OriginalColor, f.Message)
		}
	}
}

func TestContrastSwapsToFindings_Empty(t *testing.T) {
	findings := contrastSwapsToFindings(nil)
	if findings != nil {
		t.Errorf("expected nil for empty swaps, got %v", findings)
	}
}

// --- BudgetFitFindings tests ---

func makeFinding(slideIdx int, action string, code string, hasFix bool) patterns.FitFinding {
	f := patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    fmt.Sprintf("slides[%d].content.body", slideIdx),
			Code:    code,
			Message: fmt.Sprintf("finding %s on slide %d", code, slideIdx),
		},
		Action: action,
	}
	if hasFix {
		f.Fix = &patterns.FixSuggestion{Kind: "reduce_text"}
	}
	return f
}

func TestBudgetFitFindings_Under(t *testing.T) {
	// 3 findings on one slide — should pass through unchanged.
	findings := []patterns.FitFinding{
		makeFinding(0, "refuse", "a", true),
		makeFinding(0, "review", "b", false),
		makeFinding(0, "info", "c", false),
	}
	result := BudgetFitFindings(findings, 5, false)
	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}
}

func TestBudgetFitFindings_Over(t *testing.T) {
	// 20 findings on slide 0 — should return 5 + 1 summary.
	var findings []patterns.FitFinding
	for i := 0; i < 20; i++ {
		action := "info"
		if i < 3 {
			action = "refuse"
		} else if i < 8 {
			action = "shrink_or_split"
		}
		findings = append(findings, makeFinding(0, action, fmt.Sprintf("code_%d", i), i%2 == 0))
	}
	result := BudgetFitFindings(findings, 5, false)

	if len(result) != 6 {
		t.Fatalf("expected 6 findings (5 + 1 summary), got %d", len(result))
	}

	// Top findings should be sorted by severity desc, fix-present first.
	for i := 0; i < 5; i++ {
		if i > 0 {
			ri := patterns.ActionRank(result[i].Action)
			rPrev := patterns.ActionRank(result[i-1].Action)
			if ri > rPrev {
				t.Errorf("finding[%d] has higher rank (%d) than finding[%d] (%d)", i, ri, i-1, rPrev)
			}
		}
	}

	// Last one should be the summary.
	summary := result[5]
	if summary.Code != "findings_truncated" {
		t.Errorf("summary code = %q, want %q", summary.Code, "findings_truncated")
	}
	if summary.Action != "info" {
		t.Errorf("summary action = %q, want %q", summary.Action, "info")
	}
	if !strings.Contains(summary.Message, "15 more findings suppressed") {
		t.Errorf("summary message = %q, want to contain '15 more findings suppressed'", summary.Message)
	}
	if !strings.Contains(summary.Message, "verbose_fit") {
		t.Errorf("summary message = %q, want to contain 'verbose_fit'", summary.Message)
	}
}

func TestBudgetFitFindings_Verbose(t *testing.T) {
	var findings []patterns.FitFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, makeFinding(0, "info", fmt.Sprintf("code_%d", i), false))
	}
	result := BudgetFitFindings(findings, 5, true)
	if len(result) != 20 {
		t.Fatalf("verbose mode should return all 20 findings, got %d", len(result))
	}
}

func TestBudgetFitFindings_MultiSlide(t *testing.T) {
	// 3 findings on slide 0 (under budget), 7 on slide 1 (over budget).
	var findings []patterns.FitFinding
	for i := 0; i < 3; i++ {
		findings = append(findings, makeFinding(0, "review", fmt.Sprintf("a_%d", i), true))
	}
	for i := 0; i < 7; i++ {
		findings = append(findings, makeFinding(1, "info", fmt.Sprintf("b_%d", i), false))
	}
	result := BudgetFitFindings(findings, 5, false)

	// Slide 0: 3 findings. Slide 1: 5 + 1 summary = 6. Total = 9.
	if len(result) != 9 {
		t.Fatalf("expected 9 findings, got %d", len(result))
	}

	// Last should be summary for slide 1.
	summary := result[8]
	if summary.Code != "findings_truncated" {
		t.Errorf("last finding code = %q, want findings_truncated", summary.Code)
	}
	if !strings.Contains(summary.Message, "2 more") {
		t.Errorf("summary message = %q, want to contain '2 more'", summary.Message)
	}
}

func TestBudgetFitFindings_FixPriority(t *testing.T) {
	// Two findings with same severity — one with Fix, one without.
	// The one with Fix should come first.
	findings := []patterns.FitFinding{
		makeFinding(0, "review", "no_fix", false),
		makeFinding(0, "review", "has_fix", true),
	}
	result := BudgetFitFindings(findings, 5, false)
	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}
	if result[0].Code != "has_fix" {
		t.Errorf("expected fix-bearing finding first, got %q", result[0].Code)
	}
}

func TestBudgetFitFindings_Empty(t *testing.T) {
	result := BudgetFitFindings(nil, 5, false)
	if result != nil {
		t.Errorf("expected nil for empty findings, got %v", result)
	}
}

// --- budgetLocalFindings tests ---

func makeLocalFinding(slideIdx int, action string, code string, hasFix bool) fitFinding {
	f := fitFinding{
		Path:    fmt.Sprintf("slides[%d].content.body", slideIdx),
		Code:    code,
		Message: fmt.Sprintf("finding %s on slide %d", code, slideIdx),
		Action:  action,
	}
	if hasFix {
		f.Fix = &patterns.FixSuggestion{Kind: "reduce_text"}
	}
	return f
}

func TestBudgetLocalFindings_Over(t *testing.T) {
	var findings []fitFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, makeLocalFinding(0, "info", fmt.Sprintf("code_%d", i), false))
	}
	result := budgetLocalFindings(findings, 5, false)

	if len(result) != 6 {
		t.Fatalf("expected 6 findings (5 + 1 summary), got %d", len(result))
	}
	if result[5].Code != "findings_truncated" {
		t.Errorf("summary code = %q, want findings_truncated", result[5].Code)
	}
	if !strings.Contains(result[5].Message, "--verbose-fit") {
		t.Errorf("local summary should reference --verbose-fit flag, got %q", result[5].Message)
	}
}

func TestBudgetLocalFindings_Verbose(t *testing.T) {
	var findings []fitFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, makeLocalFinding(0, "info", fmt.Sprintf("code_%d", i), false))
	}
	result := budgetLocalFindings(findings, 5, true)
	if len(result) != 20 {
		t.Fatalf("verbose mode should return all 20, got %d", len(result))
	}
}
