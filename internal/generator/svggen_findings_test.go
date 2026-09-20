package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// TestSvggenFindingsToFit pins the conversion the render path and the
// dry-render preflight now share: a finding must mean the same thing whichever
// side produced it (go-slide-creator-p142).
func TestSvggenFindingsToFit(t *testing.T) {
	got := SvggenFindingsToFit([]svggen.Finding{
		{
			Code:     "chart.legend_overflow_dropped",
			Message:  "legend overflow — 4 of 15 items dropped",
			Severity: "warning",
			Fix:      &svggen.FixSuggestion{Kind: "reduce_items", Params: map[string]any{"dropped": 4}},
		},
		{
			Code:     "chart.tick_thinned",
			Field:    "data.categories",
			Message:  "ticks thinned",
			Severity: "info",
		},
	}, "pie_chart", "/slides/0/content/body")

	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2", len(got))
	}
	if got[0].Action != "review" {
		t.Errorf("warning should map to review, got %q", got[0].Action)
	}
	if got[0].Path != "/slides/0/content/body" {
		t.Errorf("path = %q, want the content path unchanged", got[0].Path)
	}
	if got[0].Fix == nil || got[0].Fix.Kind != "reduce_items" {
		t.Errorf("fix = %+v, want the svggen suggestion carried across", got[0].Fix)
	}
	if got[0].Pattern != "pie_chart" {
		t.Errorf("pattern = %q, want the diagram type", got[0].Pattern)
	}
	// A field-scoped finding appends the field to the path.
	if got[1].Path != "/slides/0/content/body.data.categories" {
		t.Errorf("field path = %q, want the field appended", got[1].Path)
	}

	if SvggenFindingsToFit(nil, "pie_chart", "/p") != nil {
		t.Error("no findings should produce no conversions")
	}
}

// TestSvggenSeverityAction covers the severity ladder, including the fallback
// for a code this build does not know.
func TestSvggenSeverityAction(t *testing.T) {
	for severity, want := range map[string]string{
		"refuse":          "refuse",
		"shrink_or_split": "shrink_or_split",
		"warning":         "review",
		"info":            "info",
		"":                "info",
		"something-new":   "info",
	} {
		if got := SvggenSeverityAction(severity); got != want {
			t.Errorf("SvggenSeverityAction(%q) = %q, want %q", severity, got, want)
		}
	}
}
