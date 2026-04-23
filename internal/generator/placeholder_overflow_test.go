package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Placeholder dimensions for tests: 7.5" × 4.5" body placeholder.
// (standard 16:9 slide body area)
const (
	testPHWidthEMU  = 6858000 // 7.5"
	testPHHeightEMU = 4114800 // 4.5"
)

// makeOverflowParagraphs generates n paragraphs of long text that will overflow
// a standard body placeholder at the given font size.
func makeOverflowParagraphs(n int) []string {
	paras := make([]string, n)
	for i := range paras {
		paras[i] = "This is a very long paragraph of text that will force multiple lines of wrapping within the placeholder bounds to generate overflow for testing purposes"
	}
	return paras
}

func TestDetectPlaceholderOverflow_Below115(t *testing.T) {
	// Condition 1 fails: overshoot < 15% → no finding.
	// Short text that slightly exceeds frame but stays under 1.15 ratio.
	input := PlaceholderOverflowInput{
		SlideIndex:  0,
		Path:        "slides[0].body",
		Paragraphs:  []string{"Short paragraph", "Another short paragraph"},
		WidthEMU:    testPHWidthEMU,
		HeightEMU:   testPHHeightEMU,
		FontSizeHPt: 2000, // 20pt
		FontName:    "Arial",
		AutofitMode: "", // no autofit
	}

	finding := DetectPlaceholderOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding for short text (ratio < 1.15), got code=%s", finding.Code)
	}
}

func TestDetectPlaceholderOverflow_NormAutofitPresent(t *testing.T) {
	// Condition 2 fails: normAutofit is present → PowerPoint handles overflow.
	// Use enough text to exceed 1.15 ratio.
	input := PlaceholderOverflowInput{
		SlideIndex:  0,
		Path:        "slides[0].body",
		Paragraphs:  makeOverflowParagraphs(40),
		WidthEMU:    testPHWidthEMU,
		HeightEMU:   testPHHeightEMU,
		FontSizeHPt: 2000,
		FontName:    "Arial",
		AutofitMode: "normAutofit",
	}

	finding := DetectPlaceholderOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding when normAutofit is present, got code=%s", finding.Code)
	}
}

func TestDetectPlaceholderOverflow_AutofitNoneButFitsAtMin(t *testing.T) {
	// Condition 3 fails: autofit is none, overshoot > 15%, but text fits at
	// minimum font scale (60%). Hypothetical autofit would rescue it.
	//
	// Use a moderate amount of text that overflows at 100% (20pt) but fits
	// when shrunk to 60% (12pt). Shrinking font from 20pt to 12pt reduces
	// wrapped lines by ~40%, so we need text that's about 1.4× the frame at 20pt.
	input := PlaceholderOverflowInput{
		SlideIndex:  0,
		Path:        "slides[0].body",
		Paragraphs:  makeOverflowParagraphs(15),
		WidthEMU:    testPHWidthEMU,
		HeightEMU:   testPHHeightEMU,
		FontSizeHPt: 2000,
		FontName:    "Arial",
		AutofitMode: "noAutofit",
	}

	finding := DetectPlaceholderOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding when text fits at min font scale, got code=%s ratio=%.2f", finding.Code, finding.OverflowRatio)
	}
}

func TestDetectPlaceholderOverflow_AllThreeConditions(t *testing.T) {
	// All three conditions met:
	// 1. Massive overshoot (well above 1.15)
	// 2. No autofit
	// 3. Even at min font scale (60%), text overflows
	input := PlaceholderOverflowInput{
		SlideIndex:  0,
		Path:        "slides[0].body",
		Paragraphs:  makeOverflowParagraphs(60),
		WidthEMU:    testPHWidthEMU,
		HeightEMU:   testPHHeightEMU,
		FontSizeHPt: 2000,
		FontName:    "Arial",
		AutofitMode: "",
	}

	finding := DetectPlaceholderOverflow(input)
	if finding == nil {
		t.Fatal("expected finding when all three conditions are met")
	}

	if finding.Code != patterns.ErrCodePlaceholderOverflow {
		t.Errorf("Code = %q, want %q", finding.Code, patterns.ErrCodePlaceholderOverflow)
	}
	if finding.Action != "shrink_or_split" {
		t.Errorf("Action = %q, want %q", finding.Action, "shrink_or_split")
	}
	if finding.OverflowRatio <= overflowThreshold {
		t.Errorf("OverflowRatio = %.2f, want > %.2f", finding.OverflowRatio, overflowThreshold)
	}
	if finding.Measured == nil {
		t.Error("Measured extent should be non-nil")
	}
	if finding.Allowed == nil {
		t.Error("Allowed extent should be non-nil")
	}
	if finding.Measured != nil && finding.Measured.HeightEMU <= finding.Allowed.HeightEMU {
		t.Error("Measured height should exceed Allowed height")
	}
}

func TestDetectPlaceholderOverflow_EmptyInput(t *testing.T) {
	finding := DetectPlaceholderOverflow(PlaceholderOverflowInput{})
	if finding != nil {
		t.Error("expected nil for empty input")
	}
}

func TestDetectPlaceholderOverflow_SpAutoFitSuppresses(t *testing.T) {
	// spAutoFit also counts as autofit-present (condition 2 fails).
	input := PlaceholderOverflowInput{
		SlideIndex:  0,
		Path:        "slides[0].body",
		Paragraphs:  makeOverflowParagraphs(60),
		WidthEMU:    testPHWidthEMU,
		HeightEMU:   testPHHeightEMU,
		FontSizeHPt: 2000,
		FontName:    "Arial",
		AutofitMode: "spAutoFit",
	}

	finding := DetectPlaceholderOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding with spAutoFit, got code=%s", finding.Code)
	}
}

func TestAutofitPresent(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"normAutofit", true},
		{"spAutoFit", true},
		{"noAutofit", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := autofitPresent(tt.mode); got != tt.want {
			t.Errorf("autofitPresent(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestAutofitLabel(t *testing.T) {
	if got := autofitLabel(""); got != "none" {
		t.Errorf("autofitLabel(\"\") = %q, want \"none\"", got)
	}
	if got := autofitLabel("normAutofit"); got != "normAutofit" {
		t.Errorf("autofitLabel(\"normAutofit\") = %q, want \"normAutofit\"", got)
	}
}
