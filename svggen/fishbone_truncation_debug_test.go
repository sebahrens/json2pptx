package svggen

import (
	"strings"
	"testing"
)

// TestFishbone_EffectTextNotTruncated verifies that the effect text (the "head"
// of the fishbone) is never truncated with ellipsis, even with long text.
// Regression test for go-slide-creator-cpw0f.
func TestFishbone_EffectTextNotTruncated(t *testing.T) {
	tests := []struct {
		name   string
		effect string
		preset string
	}{
		{"long effect third_16x9", "Declining Customer Satisfaction", "third_16x9"},
		{"long effect half_16x9", "Declining Customer Satisfaction", "half_16x9"},
		{"very long effect", "Significantly Declining Customer Satisfaction Across All Regions", "third_16x9"},
		{"long effect default", "Declining Customer Satisfaction", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": tt.effect,
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Training gaps", "High turnover"}},
						map[string]any{"name": "Process", "causes": []any{"Slow response time"}},
						map[string]any{"name": "Technology", "causes": []any{"Outdated CRM"}},
					},
				},
			}
			if tt.preset != "" {
				req.Output = OutputSpec{Preset: tt.preset}
			}

			doc, err := Render(req)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			svg := string(doc.Content)
			AssertSVGQuality(t, svg)

			// Every word of the effect text must appear in the SVG
			for _, word := range strings.Fields(tt.effect) {
				if !strings.Contains(svg, word) {
					t.Errorf("effect word %q missing from SVG (truncated?)", word)
				}
			}

			// Must NOT contain truncation ellipsis in the effect area
			// (the ellipsis character "…" should not appear as part of the effect text)
			if strings.Contains(svg, "Declining…") || strings.Contains(svg, "Customer…") || strings.Contains(svg, "Satisfaction…") {
				t.Errorf("effect text appears truncated with ellipsis")
			}
		})
	}
}

// TestFishbone_EffectTextNotTruncatedAtSmallSizes verifies that the effect text
// is never truncated at intermediate canvas sizes where the effect box is narrow
// and text wraps to multiple lines. Previously, a floating-point discrepancy
// between the sizing pass and the DrawWrappedText re-wrapping caused sub-pixel
// height overflow and truncation of the last line.
// Regression test for pptx-7q9.
func TestFishbone_EffectTextNotTruncatedAtSmallSizes(t *testing.T) {
	sizes := []struct {
		name string
		w, h int
	}{
		{"500x375", 500, 375},
		{"550x400", 550, 400},
		{"600x450", 600, 450},
		{"480x360", 480, 360},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": "Declining Customer Satisfaction",
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Training gaps", "High turnover"}},
						map[string]any{"name": "Process", "causes": []any{"Slow response time", "Complex escalation"}},
						map[string]any{"name": "Technology", "causes": []any{"Outdated CRM", "System downtime"}},
						map[string]any{"name": "Policy", "causes": []any{"Rigid refund rules", "Limited support hours"}},
					},
				},
				Output: OutputSpec{Width: sz.w, Height: sz.h},
			}

			doc, err := Render(req)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			svg := string(doc.Content)
			AssertSVGQuality(t, svg)

			// Every word of the effect text must appear in the SVG
			for _, word := range strings.Fields("Declining Customer Satisfaction") {
				if !strings.Contains(svg, word) {
					t.Errorf("effect word %q missing from SVG at %dx%d (truncated?)", word, sz.w, sz.h)
				}
			}

			// Must NOT contain truncation ellipsis on effect text words
			if strings.Contains(svg, "Declining…") || strings.Contains(svg, "Customer…") || strings.Contains(svg, "Satisfaction…") {
				t.Errorf("effect text appears truncated with ellipsis at %dx%d", sz.w, sz.h)
			}
		})
	}
}

// TestFishbone_ThirdWidthNoTruncation verifies that cause labels are not
// truncated at third_16x9 (500x720) viewport size.
// Regression test for go-slide-creator-253ks.
func TestFishbone_ThirdWidthNoTruncation(t *testing.T) {
	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Declining Customer Satisfaction",
		Data: map[string]any{
			"effect": "Declining Customer Satisfaction",
			"categories": []any{
				map[string]any{"name": "People", "causes": []any{"Training gaps", "High turnover"}},
				map[string]any{"name": "Process", "causes": []any{"Slow response time", "Complex escalation"}},
				map[string]any{"name": "Technology", "causes": []any{"Outdated CRM", "System downtime"}},
				map[string]any{"name": "Policy", "causes": []any{"Rigid refund rules", "Limited support hours"}},
			},
		},
		Output: OutputSpec{Preset: "third_16x9"},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// All cause label words must appear (possibly wrapped across tspan elements)
	for _, word := range []string{
		"Training", "gaps", "High", "turnover",
		"Slow", "response", "time", "Complex", "escalation",
		"Outdated", "CRM", "System", "downtime",
		"Rigid", "refund", "rules", "Limited", "support", "hours",
	} {
		if !strings.Contains(svg, word) {
			t.Errorf("cause label word %q missing from SVG (truncated?)", word)
		}
	}
}

// TestFishbone_DenseCauseLabelNoTruncation verifies that cause labels are not
// truncated with ellipsis when 6 categories are rendered at landscape widths.
// At 600x540, causes on branch tips are far from the left plot edge; labels
// must use center-aligned positioning to avoid truncation.
// Regression test for go-slide-creator-h7f4k.
func TestFishbone_DenseCauseLabelNoTruncation(t *testing.T) {
	categories := []any{
		map[string]any{"name": "People", "causes": []any{"High turnover rate", "Training gaps", "Skill mismatches"}},
		map[string]any{"name": "Process", "causes": []any{"Slow response time", "Complex escalation procedures"}},
		map[string]any{"name": "Technology", "causes": []any{"System downtime", "Outdated CRM system"}},
		map[string]any{"name": "Policy", "causes": []any{"Rigid refund rules", "Limited support hours"}},
		map[string]any{"name": "Environment", "causes": []any{"Client availability", "Regulatory changes"}},
		map[string]any{"name": "Management", "causes": []any{"Competing priorities", "Budget constraints"}},
	}

	sizes := []struct {
		name string
		w, h int
	}{
		{"600x540", 600, 540},
		{"700x540", 700, 540},
		{"760x540", 760, 540},
		{"900x540", 900, 540},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect":     "Declining Customer Satisfaction",
					"categories": categories,
				},
				Output: OutputSpec{Width: sz.w, Height: sz.h},
			}

			doc, err := Render(req)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			svg := string(doc.Content)

			// Check for truncation ellipsis (exclude overflow "… +N more")
			for _, line := range strings.Split(svg, "\n") {
				if strings.Contains(line, "…") && !strings.Contains(line, "+") && !strings.Contains(line, "more") {
					t.Errorf("truncated label found in SVG")
				}
			}
		})
	}
}

// TestFishbone_VeryNarrow_AllCategoriesVisible verifies that at very narrow
// widths (< 500pt), all 6 categories render with readable labels and a
// "(N causes)" badge instead of truncated individual cause labels.
// Regression test for go-slide-creator-huksj.
func TestFishbone_VeryNarrow_AllCategoriesVisible(t *testing.T) {
	categories := []any{
		map[string]any{"name": "People", "causes": []any{"Training gaps", "High turnover", "Skill mismatches"}},
		map[string]any{"name": "Process", "causes": []any{"Slow response time", "Complex escalation"}},
		map[string]any{"name": "Technology", "causes": []any{"Outdated CRM", "System downtime"}},
		map[string]any{"name": "Policy", "causes": []any{"Rigid refund rules", "Limited support hours"}},
		map[string]any{"name": "Environment", "causes": []any{"Client availability", "Regulatory changes"}},
		map[string]any{"name": "Management", "causes": []any{"Competing priorities", "Budget constraints"}},
	}

	// Test at very narrow widths typical of two-column layouts
	sizes := []struct {
		name string
		w, h int
	}{
		{"480x540_half_width", 480, 540},
		{"450x400_very_narrow", 450, 400},
		{"490x540_just_under", 490, 540},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect":     "Declining Customer Satisfaction",
					"categories": categories,
				},
				Output: OutputSpec{Width: sz.w, Height: sz.h},
			}

			doc, err := Render(req)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			svg := string(doc.Content)
			AssertSVGQuality(t, svg)

			// All 6 category names must appear in the SVG
			for _, name := range []string{"People", "Process", "Technology", "Policy", "Environment", "Management"} {
				if !strings.Contains(svg, name) {
					t.Errorf("category %q missing from SVG at %dx%d", name, sz.w, sz.h)
				}
			}

			// At very narrow widths, cause badges should appear instead of
			// individual cause labels
			causesBadgeCount := strings.Count(svg, "causes)")
			if causesBadgeCount < 4 {
				t.Errorf("expected cause count badges at very narrow width, found %d", causesBadgeCount)
			}

			// Should NOT have truncated category names
			for _, line := range strings.Split(svg, "\n") {
				if strings.Contains(line, "People…") || strings.Contains(line, "Process…") ||
					strings.Contains(line, "Technol…") || strings.Contains(line, "Policy…") {
					t.Errorf("category name truncated at %dx%d: %s", sz.w, sz.h, line)
				}
			}
		})
	}
}
