package svggen

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// TestTimeline_CS6_DateAccuracy reproduces bead go-slide-creator-5fni5:
// warm-coral slide 6 shows wrong milestone dates (May/Sep/Nov instead of Jun/Oct/Dec).
// The fix ensures the time axis prioritises labels at activity-aligned dates.
func TestTimeline_CS6_DateAccuracy(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Key Milestones",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":        "Mar 2026",
					"title":       "M1: Foundation",
					"description": "Core platform live",
				},
				map[string]any{
					"date":        "Jun 2026",
					"title":       "M2: Migration",
					"description": "First workloads moved",
				},
				map[string]any{
					"date":        "Oct 2026",
					"title":       "M3: Retirement",
					"description": "Legacy decommissioned",
				},
				map[string]any{
					"date":        "Dec 2026",
					"title":       "M4: Operational",
					"description": "Full platform ready",
				},
			},
		},
		Output: OutputSpec{Width: 408, Height: 342},
	}

	_, doc, err := d.RenderWithBuilder(req)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	content := doc.String()

	// SVG uses HTML entities for apostrophes: &#39; instead of '
	// Check both forms for robustness
	checkDate := func(month string) bool {
		literal := fmt.Sprintf("%s '26", month)
		encoded := fmt.Sprintf("%s &#39;26", month)
		return strings.Contains(content, literal) || strings.Contains(content, encoded)
	}

	// All four milestone months MUST appear on the axis
	expectedMonths := []string{"Mar", "Jun", "Oct", "Dec"}
	for _, month := range expectedMonths {
		if !checkDate(month) {
			t.Errorf("Expected axis label for %s '26 not found in SVG", month)
		}
	}

	// The old wrong dates (May, Sep, Nov) should NOT appear
	wrongMonths := []string{"May", "Sep", "Nov"}
	for _, month := range wrongMonths {
		if checkDate(month) {
			t.Errorf("Unexpected axis label %s '26 found — activity dates should have replaced it", month)
		}
	}

	// Log all text for debugging
	tspanRe := regexp.MustCompile(`<tspan[^>]*>([^<]+)</tspan>`)
	for _, m := range tspanRe.FindAllStringSubmatch(content, -1) {
		t.Logf("  text: %s", m[1])
	}
}
