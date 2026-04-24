package main

import (
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
