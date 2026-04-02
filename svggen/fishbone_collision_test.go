package svggen

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// Fishbone Label Collision Detection Tests
// =============================================================================
//
// These tests verify that fishbone diagrams with 6+ categories handle label
// collisions correctly. The collision detection system uses a multi-pass
// approach:
//   1. Spread branch X positions further apart
//   2. Reduce font size for labels that still overlap
//   3. Collapse excess categories (10+) behind a "+N more" indicator
// =============================================================================

// makeFishboneCategories builds n fishbone categories, each with the given
// number of causes. Category names use the classic 6M Ishikawa naming.
func makeFishboneCategories(n, causesPerCat int) []any {
	names := []string{
		"People", "Process", "Materials", "Equipment",
		"Environment", "Management", "Measurement", "Methods",
		"Machines", "Motivation", "Maintenance", "Money",
		"Markets", "Metrics", "Morale", "Milestones",
	}
	cats := make([]any, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("Category %d", i+1)
		if i < len(names) {
			name = names[i]
		}
		causes := make([]any, causesPerCat)
		for j := 0; j < causesPerCat; j++ {
			causes[j] = fmt.Sprintf("%s cause %d", name, j+1)
		}
		cats[i] = map[string]any{
			"name":   name,
			"causes": causes,
		}
	}
	return cats
}

// TestFishbone_SixCategories verifies that a fishbone with 6 categories
// renders without overlapping category labels. Six is the boundary where
// the classic alternating top/bottom layout starts to get crowded.
func TestFishbone_SixCategories(t *testing.T) {
	categories := makeFishboneCategories(6, 3)

	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Six Category Fishbone",
		Data: map[string]any{
			"effect":     "Quality Issues",
			"categories": categories,
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render 6-category fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// All 6 category names should appear in the SVG
	expectedNames := []string{"People", "Process", "Materials", "Equipment", "Environment", "Management"}
	for _, name := range expectedNames {
		if !strings.Contains(svg, name) {
			t.Errorf("6-category fishbone SVG should contain category name %q", name)
		}
	}

	// Verify no overlapping category boxes by parsing rect elements.
	// Category boxes are rendered as <rect> within <g> groups. The
	// AssertSVGQuality check above includes overlap detection, but we
	// also do a targeted check for fishbone-specific layout.
	verifyNoLabelCollisions(t, svg, expectedNames)
}

// TestFishbone_TenCategories verifies that a fishbone with 10 categories
// renders with proper collision handling. At this count, the diagram should
// use the overflow collapse mechanism.
func TestFishbone_TenCategories(t *testing.T) {
	categories := makeFishboneCategories(10, 2)

	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Ten Category Fishbone",
		Data: map[string]any{
			"effect":     "System Failure",
			"categories": categories,
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render 10-category fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// All 10 category names should be present (10 is at the limit)
	expectedNames := []string{
		"People", "Process", "Materials", "Equipment",
		"Environment", "Management", "Measurement", "Methods",
		"Machines", "Motivation",
	}
	for _, name := range expectedNames {
		if !strings.Contains(svg, name) {
			t.Errorf("10-category fishbone SVG should contain category name %q", name)
		}
	}

	verifyNoLabelCollisions(t, svg, expectedNames)
}

// TestFishbone_TwelveCategories_Overflow verifies that a fishbone with 12
// categories shows the first 10 and collapses the remaining 2 behind a
// "+2 more categories" overflow indicator.
func TestFishbone_TwelveCategories_Overflow(t *testing.T) {
	categories := makeFishboneCategories(12, 2)

	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Twelve Category Fishbone",
		Data: map[string]any{
			"effect":     "Total Meltdown",
			"categories": categories,
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render 12-category fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// The first 10 category names should be present
	first10 := []string{
		"People", "Process", "Materials", "Equipment",
		"Environment", "Management", "Measurement", "Methods",
		"Machines", "Motivation",
	}
	for _, name := range first10 {
		if !strings.Contains(svg, name) {
			t.Errorf("12-category fishbone SVG should contain visible category %q", name)
		}
	}

	// Categories 11 and 12 should NOT be rendered
	overflowed := []string{"Maintenance", "Money"}
	for _, name := range overflowed {
		if strings.Contains(svg, name) {
			t.Errorf("12-category fishbone SVG should NOT contain overflowed category %q", name)
		}
	}

	// The overflow indicator should be present
	if !strings.Contains(svg, "+2 more categories") {
		t.Error("12-category fishbone SVG should contain '+2 more categories' overflow indicator")
	}
}

// TestFishbone_SixCategories_Narrow tests collision handling at narrow width
// where horizontal space is very constrained.
func TestFishbone_SixCategories_Narrow(t *testing.T) {
	categories := makeFishboneCategories(6, 2)

	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Narrow Six Categories",
		Data: map[string]any{
			"effect":     "Quality Issues",
			"categories": categories,
		},
		Output: OutputSpec{Width: 500, Height: 400},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render narrow 6-category fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)
}

// TestFishbone_EightCategories tests the intermediate case between 6 and 10
// where collision detection is most critical.
func TestFishbone_EightCategories(t *testing.T) {
	categories := makeFishboneCategories(8, 3)

	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Eight Category Fishbone",
		Data: map[string]any{
			"effect":     "Performance Degradation",
			"categories": categories,
		},
		Output: OutputSpec{Width: 900, Height: 600},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render 8-category fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	expectedNames := []string{
		"People", "Process", "Materials", "Equipment",
		"Environment", "Management", "Measurement", "Methods",
	}
	for _, name := range expectedNames {
		if !strings.Contains(svg, name) {
			t.Errorf("8-category fishbone SVG should contain category name %q", name)
		}
	}
}

// TestFishbone_LongLabelsNoEllipsis verifies that fishbone diagrams with
// long category names and cause labels do not truncate text with ellipsis.
// This is a regression test for go-slide-creator-vsa8z.
func TestFishbone_LongLabelsNoEllipsis(t *testing.T) {
	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Service Outage Root Cause",
		Data: map[string]any{
			"effect": "Customer Satisfaction Decline",
			"categories": []any{
				map[string]any{
					"name":   "Technology",
					"causes": []any{"System downtime", "Slow response times", "Complex escalation paths"},
				},
				map[string]any{
					"name":   "People",
					"causes": []any{"Insufficient training", "High staff turnover", "Knowledge gaps"},
				},
				map[string]any{
					"name":   "Process",
					"causes": []any{"Outdated procedures", "Missing documentation", "Poor handoff protocol"},
				},
				map[string]any{
					"name":   "Environment",
					"causes": []any{"Legacy infrastructure", "Network instability", "Resource constraints"},
				},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render fishbone with long labels: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Category names must appear in full (no ellipsis truncation)
	for _, catName := range []string{"Technology", "People", "Process", "Environment"} {
		if !strings.Contains(svg, catName) {
			t.Errorf("category name %q should appear in full", catName)
		}
	}

	// Cause labels must not be truncated with ellipsis. Check that the
	// SVG does NOT contain partial labels ending in "…" for the key
	// long labels that triggered the original bug.
	ellipsis := "\u2026"
	for _, partialLabel := range []string{
		"System dewnt" + ellipsis,
		"Slow respons" + ellipsis,
		"Complex esca" + ellipsis,
		"Insufficient" + ellipsis,
		"High staff t" + ellipsis,
	} {
		if strings.Contains(svg, partialLabel) {
			t.Errorf("cause label should not be truncated: found %q", partialLabel)
		}
	}

	// Verify at least some full cause text appears (may be wrapped across lines)
	for _, keyword := range []string{"downtime", "response", "turnover"} {
		if !strings.Contains(svg, keyword) {
			t.Errorf("cause label keyword %q should appear in SVG (possibly wrapped)", keyword)
		}
	}
}

// TestFishbone_NarrowSlotNoTruncation verifies that cause labels are not
// truncated with ellipsis at narrow viewport sizes (third_16x9 = 667x960).
// Regression test for go-slide-creator-cb0b8 and go-slide-creator-ng419.
func TestFishbone_NarrowSlotNoTruncation(t *testing.T) {
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
		t.Fatalf("failed to render fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Cause keywords must appear in full (possibly wrapped across lines)
	for _, keyword := range []string{"response", "escalation", "downtime", "refund", "support"} {
		if !strings.Contains(svg, keyword) {
			t.Errorf("cause label keyword %q should appear in SVG (possibly wrapped)", keyword)
		}
	}
}

// verifyNoLabelCollisions performs a targeted check that category label text
// elements are all present in the SVG and that there are enough graphical
// elements (paths) to represent the category boxes. The fishbone renderer
// uses DrawRoundedRect which emits <path> elements (not <rect>), so we
// count paths as a proxy. Full geometric collision detection is done at
// layout time in resolveCollisions.
func verifyNoLabelCollisions(t *testing.T, svg string, names []string) {
	t.Helper()

	// Verify all expected names appear in the SVG output.
	for _, name := range names {
		if !strings.Contains(svg, name) {
			t.Errorf("expected category label %q to be present in SVG", name)
		}
	}

	// Count <path elements as a proxy for rendered category boxes.
	// Each category box is a rounded rect rendered as a <path>. There
	// are also other paths (spine arrowhead, branch lines become paths
	// sometimes), so we check for a minimum, not an exact count.
	pathCount := strings.Count(svg, "<path")
	if pathCount < len(names) {
		t.Errorf("expected at least %d <path> elements for %d categories, got %d",
			len(names), len(names), pathCount)
	}
}
