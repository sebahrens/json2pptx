package textfit

import (
	"strings"
	"testing"
)

func TestCalculate_EmptyInput(t *testing.T) {
	tests := []struct {
		name   string
		params Params
	}{
		{"no paragraphs", Params{WidthEMU: 1000000, HeightEMU: 1000000}},
		{"zero width", Params{WidthEMU: 0, HeightEMU: 1000000, Paragraphs: []string{"text"}}},
		{"zero height", Params{WidthEMU: 1000000, HeightEMU: 0, Paragraphs: []string{"text"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Calculate(tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.NeedsAutofit() {
				t.Error("expected no autofit for empty/zero input")
			}
		})
	}
}

func TestCalculate_ShortTextFits(t *testing.T) {
	// A single short paragraph in a large placeholder should not need scaling.
	// Placeholder: ~5 inches wide, ~3 inches tall
	result, err := Calculate(Params{
		WidthEMU:    5 * 914400, // 5 inches
		HeightEMU:   3 * 914400, // 3 inches
		FontSizeHPt: 1800,       // 18pt
		FontName:    "Arial",
		Paragraphs:  []string{"Short text"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NeedsAutofit() {
		t.Errorf("short text should fit without scaling, got FontScale=%d LnSpcReduction=%d",
			result.FontScale, result.LnSpcReduction)
	}
}

func TestCalculate_ManyBulletsTriggerScaling(t *testing.T) {
	// 20 bullets in a small placeholder should trigger scaling.
	bullets := make([]string, 20)
	for i := range bullets {
		bullets[i] = "This is a bullet point with some text that takes up space"
	}

	result, err := Calculate(Params{
		WidthEMU:    3 * 914400, // 3 inches
		HeightEMU:   2 * 914400, // 2 inches
		FontSizeHPt: 1800,       // 18pt
		FontName:    "Arial",
		Paragraphs:  bullets,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.NeedsAutofit() {
		t.Error("20 bullets in small placeholder should need autofit")
	}
	if result.FontScale <= 0 {
		t.Error("expected positive FontScale")
	}
	if result.FontScale > 100*1000 {
		t.Errorf("FontScale %d should be <= 100000", result.FontScale)
	}
}

func TestCalculate_ExtremeOverflow(t *testing.T) {
	// Massive text in tiny placeholder should max out and flag overflow.
	paragraphs := make([]string, 50)
	for i := range paragraphs {
		paragraphs[i] = strings.Repeat("This is a very long paragraph that should definitely overflow. ", 5)
	}

	result, err := Calculate(Params{
		WidthEMU:    1 * 914400, // 1 inch
		HeightEMU:   1 * 914400, // 1 inch
		FontSizeHPt: 2400,       // 24pt
		FontName:    "Arial",
		Paragraphs:  paragraphs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Overflow {
		t.Error("extreme text should flag overflow")
	}
	if result.FontScale != minFontScalePct*1000 {
		t.Errorf("FontScale = %d, want %d", result.FontScale, minFontScalePct*1000)
	}
}

func TestFitResult_NeedsAutofit(t *testing.T) {
	tests := []struct {
		name   string
		result FitResult
		want   bool
	}{
		{"zero values", FitResult{}, false},
		{"font scale only", FitResult{FontScale: 85000}, true},
		{"line spacing only", FitResult{LnSpcReduction: 10000}, true},
		{"both", FitResult{FontScale: 50000, LnSpcReduction: 20000}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.NeedsAutofit(); got != tt.want {
				t.Errorf("NeedsAutofit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculate_DefaultValues(t *testing.T) {
	// Should work with zero FontSizeHPt and empty FontName (uses defaults).
	result, err := Calculate(Params{
		WidthEMU:   5 * 914400,
		HeightEMU:  3 * 914400,
		Paragraphs: []string{"Some text"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic and should return a valid result
	_ = result
}

func TestCalculate_PerParagraphSpacing(t *testing.T) {
	// Bullet group headers have extra spcBef (12pt) and trailing body has spcBef (24pt).
	// Without per-paragraph spacing, textfit underestimates height and doesn't scale.
	// With per-paragraph spacing, it correctly accounts for the extra height.

	// Create content mimicking bullet groups: header + 3 bullets + header + 3 bullets + trailing body
	paragraphs := []string{
		"Section A: Revenue Growth",
		"Increased enterprise sales by 45%",
		"Expanded into 3 new markets",
		"Improved customer retention to 92%",
		"Section B: Cost Optimization",
		"Reduced cloud spend by 30%",
		"Consolidated vendor contracts",
		"Automated reporting pipeline",
		"These results demonstrate strong execution across all key metrics.",
	}

	// Per-paragraph spacings: headers at indices 0,4 get 12pt, trailing body at 8 gets 24pt
	perParaSpacings := []float64{
		12.0, // header spcBef
		0,    // bullet
		0,    // bullet
		0,    // bullet
		12.0, // header spcBef
		0,    // bullet
		0,    // bullet
		0,    // bullet
		24.0, // trailing body spcBef
	}

	// Use a constrained placeholder where the extra spacing matters
	baseParams := Params{
		WidthEMU:    4 * 914400, // 4 inches
		HeightEMU:   2 * 914400, // 2 inches
		FontSizeHPt: 1800,       // 18pt
		FontName:    "Arial",
		Paragraphs:  paragraphs,
	}

	// Without per-paragraph spacing
	resultUniform, err := Calculate(baseParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With per-paragraph spacing
	paramsWithSpacing := baseParams
	paramsWithSpacing.ExtraSpacingsPt = perParaSpacings
	resultPerPara, err := Calculate(paramsWithSpacing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Per-paragraph spacing adds more height, so it should require more aggressive scaling
	// (lower FontScale) or trigger scaling when uniform doesn't
	if resultPerPara.NeedsAutofit() {
		if resultUniform.NeedsAutofit() {
			// Both need autofit — per-paragraph should have equal or lower fontScale
			if resultPerPara.FontScale > resultUniform.FontScale {
				t.Errorf("per-paragraph spacing should require equal or more scaling: uniform=%d, perPara=%d",
					resultUniform.FontScale, resultPerPara.FontScale)
			}
		}
		// If only per-paragraph needs autofit, that's the expected fix
	}
}

func TestCalculate_LeftMarginsReduceAvailableWidth(t *testing.T) {
	// Bullet paragraphs with marL should use less available width, causing more wrapping
	// and requiring more aggressive font scaling than the same content without margins.
	longBullet := "This is a fairly long bullet point that will need to wrap to multiple lines"
	paragraphs := []string{
		"Section Header",
		longBullet,
		longBullet,
		longBullet,
		longBullet,
		longBullet,
	}

	baseParams := Params{
		WidthEMU:    4 * 914400, // 4 inches
		HeightEMU:   2 * 914400, // 2 inches
		FontSizeHPt: 1800,       // 18pt
		FontName:    "Arial",
		Paragraphs:  paragraphs,
	}

	// Without left margins
	resultNoMargin, err := Calculate(baseParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With left margins on bullet paragraphs (~30pt, matching typical lvl1 marL=384048 EMU)
	paramsWithMargin := baseParams
	paramsWithMargin.LeftMarginsPt = []float64{
		0,     // header — no margin
		30.24, // bullet at level 1
		30.24,
		30.24,
		30.24,
		30.24,
	}
	resultWithMargin, err := Calculate(paramsWithMargin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With margins, either scaling is more aggressive or overflow is triggered
	if resultWithMargin.NeedsAutofit() && resultNoMargin.NeedsAutofit() {
		if resultWithMargin.FontScale > resultNoMargin.FontScale {
			t.Errorf("left margins should require equal or more scaling: noMargin=%d, withMargin=%d",
				resultNoMargin.FontScale, resultWithMargin.FontScale)
		}
	}
}

func TestCalculate_LeftMarginsNil(t *testing.T) {
	// Nil LeftMarginsPt should behave identically to no margins
	paragraphs := []string{"Line 1", "Line 2", "Line 3"}

	result1, err := Calculate(Params{
		WidthEMU:    3 * 914400,
		HeightEMU:   2 * 914400,
		FontSizeHPt: 1800,
		FontName:    "Arial",
		Paragraphs:  paragraphs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result2, err := Calculate(Params{
		WidthEMU:      3 * 914400,
		HeightEMU:     2 * 914400,
		FontSizeHPt:   1800,
		FontName:      "Arial",
		Paragraphs:    paragraphs,
		LeftMarginsPt: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1.FontScale != result2.FontScale {
		t.Errorf("nil LeftMarginsPt should match no margins: got %d vs %d",
			result1.FontScale, result2.FontScale)
	}
}

func TestMaxFontForWidth(t *testing.T) {
	// Section divider with large decorative font: CX=3462095 EMU (~272pt), lstStyle=350pt
	sectionDividerWidth := int64(3462095)

	t.Run("caps font for long word", func(t *testing.T) {
		maxFont := MaxFontForWidth("Performance Overview", sectionDividerWidth, "Arial")
		// "Performance" (11 chars) at 350pt is way too wide for 272pt placeholder.
		// Max font should be capped significantly below 350pt = 35000 hPt.
		if maxFont >= 35000 {
			t.Errorf("MaxFontForWidth should cap below 35000 for 'Performance Overview', got %d", maxFont)
		}
		// But should still be a reasonable size (>12pt = 1200 hPt)
		if maxFont < 1200 {
			t.Errorf("MaxFontForWidth should return >=1200 for readable text, got %d", maxFont)
		}
	})

	t.Run("allows larger font for short word", func(t *testing.T) {
		maxFontShort := MaxFontForWidth("Risks", sectionDividerWidth, "Arial")
		maxFontLong := MaxFontForWidth("Performance Overview", sectionDividerWidth, "Arial")
		// "Risks" (5 chars) should allow a much larger font than "Performance" (11 chars)
		if maxFontShort <= maxFontLong {
			t.Errorf("short word 'Risks' should allow larger font (%d) than 'Performance Overview' (%d)",
				maxFontShort, maxFontLong)
		}
	})

	t.Run("wider placeholder allows larger font", func(t *testing.T) {
		narrowWidth := int64(2000000) // ~157pt
		wideWidth := int64(6000000)   // ~472pt

		maxNarrow := MaxFontForWidth("Performance Overview", narrowWidth, "Arial")
		maxWide := MaxFontForWidth("Performance Overview", wideWidth, "Arial")
		if maxWide <= maxNarrow {
			t.Errorf("wider placeholder should allow larger font: narrow=%d wide=%d",
				maxNarrow, maxWide)
		}
	})

	t.Run("empty text returns 0", func(t *testing.T) {
		if got := MaxFontForWidth("", sectionDividerWidth, "Arial"); got != 0 {
			t.Errorf("empty text should return 0, got %d", got)
		}
	})

	t.Run("zero width returns 0", func(t *testing.T) {
		if got := MaxFontForWidth("Test", 0, "Arial"); got != 0 {
			t.Errorf("zero width should return 0, got %d", got)
		}
	})
}

func TestCalculate_MinFontScalePctUsedInLnSpcReductionPath(t *testing.T) {
	// When MinFontScalePct is overridden (e.g., 45 for dense bullets) and the text
	// requires line spacing reduction at that floor, the returned FontScale must use
	// the overridden value (45000), not the package constant (60000).
	// This was a bug: line spacing reduction path returned minFontScalePct*1000
	// (the constant 60) instead of minScale*1000 (the overridden value).

	// Create enough text that it doesn't fit at 45% font scale without lnSpcReduction,
	// but does fit at 45% + some lnSpcReduction.
	paragraphs := make([]string, 25)
	for i := range paragraphs {
		paragraphs[i] = "This is a medium-length bullet point for testing overflow"
	}

	result, err := Calculate(Params{
		WidthEMU:        3 * 914400,  // 3 inches
		HeightEMU:       2 * 914400,  // 2 inches
		FontSizeHPt:     2000,        // 20pt
		FontName:        "Arial",
		Paragraphs:      paragraphs,
		MinFontScalePct: 45,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The result should have FontScale based on 45% (or the correct floor),
	// not the default 60%.
	if result.FontScale > 0 && result.LnSpcReduction > 0 {
		// When both are set, FontScale must reflect the override
		if result.FontScale == minFontScalePct*1000 && result.FontScale != 45*1000 {
			t.Errorf("FontScale should use overridden MinFontScalePct (45000), got %d (package constant 60000)", result.FontScale)
		}
	}
	// If it overflows entirely, that's also acceptable — the fix ensures the
	// non-overflow lnSpcReduction path returns the correct scale.
	if result.Overflow {
		// Overflow return already uses minScale correctly (line 165)
		if result.FontScale != 45*1000 {
			t.Errorf("overflow FontScale should use overridden MinFontScalePct (45000), got %d", result.FontScale)
		}
	}
}

func TestCalculate_PerParagraphSpacingNil(t *testing.T) {
	// When ExtraSpacingsPt is nil, behavior should be identical to uniform ExtraSpacingPt
	paragraphs := []string{"Line 1", "Line 2", "Line 3"}

	result1, err := Calculate(Params{
		WidthEMU:       3 * 914400,
		HeightEMU:      2 * 914400,
		FontSizeHPt:    1800,
		FontName:       "Arial",
		Paragraphs:     paragraphs,
		ExtraSpacingPt: 12.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result2, err := Calculate(Params{
		WidthEMU:        3 * 914400,
		HeightEMU:       2 * 914400,
		FontSizeHPt:     1800,
		FontName:        "Arial",
		Paragraphs:      paragraphs,
		ExtraSpacingPt:  12.0,
		ExtraSpacingsPt: nil, // explicitly nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1.FontScale != result2.FontScale {
		t.Errorf("nil ExtraSpacingsPt should match uniform: got %d vs %d",
			result1.FontScale, result2.FontScale)
	}
}
