package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textfit"
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
		SlideIndex: 0,
		Path:       "slides[0].body",
		// 6 paragraphs (calibrated for point-sized glyph measurement): ~1.4x
		// the frame at 20pt, fits at the 60% floor.
		Paragraphs:  makeOverflowParagraphs(6),
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

// Title placeholder dimensions: 8.5" × 0.6" (typical title area).
const (
	testTitleWidthEMU  = 7772400 // 8.5"
	testTitleHeightEMU = 548640  // 0.6"
)

func TestDetectTitleWraps_SingleLine(t *testing.T) {
	// Short title that fits on one line → no finding.
	input := TitleWrapsInput{
		SlideIndex:  0,
		Path:        "slides[0].content.title",
		Title:       "Short Title",
		WidthEMU:    testTitleWidthEMU,
		HeightEMU:   testTitleHeightEMU,
		FontSizeHPt: 3600, // 36pt
		FontName:    "Arial",
	}

	finding := DetectTitleWraps(input)
	if finding != nil {
		t.Errorf("expected no finding for single-line title, got code=%s", finding.Code)
	}
}

func TestDetectTitleWraps_SubstitutedFontBorderline(t *testing.T) {
	const title = "Q2 FY26 Business Review"
	const width = int64(10 * 914400)
	m, err := textfit.MeasureRun(title, "Lato", 60, width, 0)
	if err != nil {
		t.Fatal(err)
	}
	measuredWidth, err := textfit.MeasureLineWidth(title, "Lato", 60)
	if err != nil {
		t.Fatal(err)
	}
	if float64(measuredWidth) > float64(width-int64(14.4*12700))*1.03 {
		t.Skip("substituted font is not a borderline-width reproduction on this host")
	}
	if !m.FontSubstituted || m.Lines != 2 {
		t.Skip("reproduction requires Lato to be substituted and borderline-wrap on this host")
	}
	finding := DetectTitleWraps(TitleWrapsInput{Title: title, WidthEMU: width, HeightEMU: 900000, FontSizeHPt: 6000, FontName: "Lato"})
	if finding != nil && finding.Action != "info" {
		t.Fatalf("borderline wrap under font substitution must not be actionable: %+v", finding)
	}
	if finding != nil && finding.Fix != nil {
		t.Fatalf("borderline wrap under font substitution must not suggest a truncating fix: %+v", finding.Fix)
	}
}

func TestDetectTitleWraps_SubstitutedFontToleranceBoundary(t *testing.T) {
	const title = "Revenue growth and margin expansion"
	const font = "unavailable-title-font-for-testing"
	lineWidth, err := textfit.MeasureLineWidth(title, font, 36)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		ratio      float64
		wantAction string
	}{
		{name: "two percent over", ratio: 1.02},
		{name: "six percent over", ratio: 1.06, wantAction: "review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			width := int64(float64(lineWidth)/tc.ratio) + int64(14.4*12700)
			measurement, err := textfit.MeasureRun(title, font, 36, width, 0)
			if err != nil || !measurement.FontSubstituted || measurement.Lines != 2 {
				t.Fatalf("invalid substituted-wrap fixture: measurement=%+v, err=%v", measurement, err)
			}
			finding := DetectTitleWraps(TitleWrapsInput{Title: title, WidthEMU: width, HeightEMU: testTitleHeightEMU, FontSizeHPt: 3600, FontName: font})
			if tc.wantAction == "" {
				if finding != nil {
					t.Fatalf("small substitute-font discrepancy must not warn: %+v", finding)
				}
			} else if finding == nil || finding.Action != tc.wantAction {
				t.Fatalf("real wrap must remain actionable: %+v", finding)
			}
		})
	}
}

func TestDetectTitleWraps_MultiLine(t *testing.T) {
	// Long title that forces wrapping → finding emitted with code title_wraps, action review.
	input := TitleWrapsInput{
		SlideIndex:  0,
		Path:        "slides[0].content.title",
		Title:       "This Is a Long Title That Will Wrap Onto a Second Line Here",
		WidthEMU:    testTitleWidthEMU,
		HeightEMU:   testTitleHeightEMU,
		FontSizeHPt: 3600, // 36pt
		FontName:    "Arial",
	}

	finding := DetectTitleWraps(input)
	if finding == nil {
		t.Fatal("expected finding for multi-line title")
	}

	if finding.Code != patterns.ErrCodeTitleWraps {
		t.Errorf("Code = %q, want %q", finding.Code, patterns.ErrCodeTitleWraps)
	}
	if finding.Action != "review" {
		t.Errorf("Action = %q, want %q", finding.Action, "review")
	}
	if finding.OverflowRatio <= 1.0 {
		t.Errorf("OverflowRatio = %.2f, want > 1.0", finding.OverflowRatio)
	}
	if finding.Measured == nil {
		t.Error("Measured extent should be non-nil")
	}
	if finding.Allowed == nil {
		t.Error("Allowed extent should be non-nil")
	}
	if finding.Fix == nil || finding.Fix.Kind != "shorten_title" {
		t.Fatalf("actionable wrap must provide a shortening directive: %+v", finding.Fix)
	}
	budget, ok := finding.Fix.Params["max_chars"].(int)
	if !ok || budget <= 0 || budget >= len([]rune(input.Title)) {
		t.Errorf("measured max_chars = %v, want a positive budget below title length", finding.Fix.Params["max_chars"])
	}
}

func TestDetectTitleWraps_NormalTwoLineTitleIsSilent(t *testing.T) {
	const width = int64(7772400)
	var title string
	for n := 1; n <= 30; n++ {
		candidate := strings.Repeat("Revenue growth and margin expansion ", n)
		measurement, err := textfit.MeasureRun(candidate, "Arial", 36, width, 0)
		if err != nil {
			t.Fatal(err)
		}
		if measurement.Lines == 2 {
			title = candidate
			break
		}
	}
	if title == "" {
		t.Fatal("test fixture could not produce a two-line title")
	}
	input := TitleWrapsInput{Title: title, WidthEMU: width, HeightEMU: 2 * testTitleHeightEMU, FontSizeHPt: 3600, FontName: "Arial"}
	if finding := DetectTitleWraps(input); finding != nil {
		t.Errorf("two lines in a two-line box are normal, got %+v", finding)
	}
	input.HeightEMU = testTitleHeightEMU
	if finding := DetectTitleWraps(input); finding == nil || finding.Action != "review" {
		t.Errorf("two lines in a one-line box require review, got %+v", finding)
	}
	input.HeightEMU = 2 * testTitleHeightEMU
	input.MaxLines = 1
	if finding := DetectTitleWraps(input); finding == nil || finding.Action != "info" {
		t.Errorf("explicit one-line threshold is informational when the box holds the title, got %+v", finding)
	}
}

func TestDetectTitleWraps_ThreeLinesRemainInformational(t *testing.T) {
	const width = int64(7772400)
	var title string
	for n := 1; n <= 30; n++ {
		candidate := strings.Repeat("Revenue growth and margin expansion ", n)
		measurement, err := textfit.MeasureRun(candidate, "Arial", 36, width, 0)
		if err != nil {
			t.Fatal(err)
		}
		if measurement.Lines == 3 {
			title = candidate
			break
		}
	}
	if title == "" {
		t.Fatal("test fixture could not produce a three-line title")
	}
	finding := DetectTitleWraps(TitleWrapsInput{Title: title, WidthEMU: width, HeightEMU: 3 * testTitleHeightEMU, FontSizeHPt: 3600, FontName: "Arial"})
	if finding == nil || finding.Action != "info" {
		t.Errorf("three lines in a roomy title box should be informational, got %+v", finding)
	}
	if finding != nil && finding.Fix != nil {
		t.Errorf("informational wrap must not suggest destructive repair: %+v", finding.Fix)
	}
}

func TestDetectTitleWraps_ExceedsMaxLines(t *testing.T) {
	// A title beyond the line-count threshold is reported but not truncated.
	// At 36pt/8.5", Arial needs substantial text to exceed 3 wrapped lines.
	longTitle := strings.Repeat("Comprehensive Analysis of Global Market Trends and Revenue Growth ", 5)
	input := TitleWrapsInput{
		SlideIndex:  0,
		Path:        "slides[0].content.title",
		Title:       longTitle,
		WidthEMU:    testTitleWidthEMU,
		HeightEMU:   testTitleHeightEMU,
		FontSizeHPt: 3600, // 36pt
		FontName:    "Arial",
		MaxLines:    3,
	}

	finding := DetectTitleWraps(input)
	if finding == nil {
		t.Fatal("expected finding for title exceeding max lines")
	}

	if finding.Code != patterns.ErrCodeTitleWraps {
		t.Errorf("Code = %q, want %q", finding.Code, patterns.ErrCodeTitleWraps)
	}
	if finding.Action != "review" {
		t.Errorf("Action = %q, want %q", finding.Action, "review")
	}
	if strings.Contains(finding.Message, "truncat") {
		t.Errorf("wrap notice falsely claims truncation: %s", finding.Message)
	}
}

func TestDetectTitleWraps_EmptyTitle(t *testing.T) {
	finding := DetectTitleWraps(TitleWrapsInput{
		SlideIndex:  0,
		Path:        "slides[0].content.title",
		Title:       "",
		WidthEMU:    testTitleWidthEMU,
		HeightEMU:   testTitleHeightEMU,
		FontSizeHPt: 3600,
		FontName:    "Arial",
	})
	if finding != nil {
		t.Error("expected nil for empty title")
	}
}

func TestDetectTitleWraps_DistinctFromPlaceholderOverflow(t *testing.T) {
	// Verify that the code is distinct from placeholder_overflow.
	if patterns.ErrCodeTitleWraps == patterns.ErrCodePlaceholderOverflow {
		t.Errorf("ErrCodeTitleWraps (%q) must differ from ErrCodePlaceholderOverflow (%q)",
			patterns.ErrCodeTitleWraps, patterns.ErrCodePlaceholderOverflow)
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
