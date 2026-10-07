package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// TestTimeline_CS6_DateAccuracy reproduces bead go-slide-creator-5fni5:
// warm-coral slide 6 shows wrong milestone dates (May/Sep/Nov instead of Jun/Oct/Dec).
// The milestone track prints each date in the event's own label block.
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

	// Every milestone prints the date its author wrote, in its own label
	// block, so no axis tick can attribute it to a neighbouring month.
	for _, date := range []string{"Mar 2026", "Jun 2026", "Oct 2026", "Dec 2026"} {
		if !strings.Contains(content, ">"+date+"<") {
			t.Errorf("milestone date %q not printed with its label", date)
		}
	}

	// The old wrong dates (May, Sep, Nov) must not appear anywhere.
	for _, month := range []string{"May", "Sep", "Nov"} {
		if strings.Contains(content, month+" '26") || strings.Contains(content, month+" &#39;26") || strings.Contains(content, month+" 2026") {
			t.Errorf("unexpected date %s 2026 found", month)
		}
	}

	// Log all text for debugging
	tspanRe := regexp.MustCompile(`<tspan[^>]*>([^<]+)</tspan>`)
	for _, m := range tspanRe.FindAllStringSubmatch(content, -1) {
		t.Logf("  text: %s", m[1])
	}
}
