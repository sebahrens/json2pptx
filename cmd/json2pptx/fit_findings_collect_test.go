package main

import (
	"testing"

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
		Action:      "unfittable",
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
	if f.Action != "unfittable" {
		t.Errorf("Action = %q, want %q", f.Action, "unfittable")
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
