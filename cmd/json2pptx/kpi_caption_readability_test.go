package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-ntvhh. A stat region writes its figure and KPI caption in
// one shape; the stored autofit (72%) takes the 14pt caption to 10.08pt, above
// the 10pt caption floor the role-aware grid check applies. The generic
// stored-autofit scan read the whole box as body text and warned "body text
// renders at 10.1pt, below the 12pt minimum", which cleared
// deterministic_ready although nothing on the slide was unreadable. The
// stat box now fits without shrink at this HEAD; the generator unit test
// (TestUnreadableAutofitHonoursGridRoles) pins the stored-autofit case.
const kpiCaptionSpec = `{"meta": {"title": "Northstar operations review", "archetype": "board_update",
  "template": "modern", "date": "3 October 2026", "audience": "Board of directors",
  "source": "Illustrative management accounts, FY26", "viewing_mode": "present", "type_scale": "comfortable",
  "chrome": {"confidentiality": "Illustrative case", "page_numbers": {"enabled": true}}},
 "slides": [{"kind": "regions", "title": "FY26's 17% EBIT margin supports a staged FY27 pilot",
  "arrangement": "main_left",
  "regions": [
   {"kind": "chart", "heading": "FY26 quarterly revenue", "unit": "$m", "size_pct": 55,
    "chart": {"type": "line_chart", "data": {"categories": ["Q1", "Q2", "Q3", "Q4"],
     "series": [{"name": "Revenue", "values": [22, 24, 26, 28]}], "data_labels": {"show_on": "all"}}}},
   {"kind": "stat", "value": "17%", "label": "FY26 EBIT margin", "size_pct": 40},
   {"kind": "timeline", "heading": "FY27 pilot, Oct–Dec",
    "milestones": [{"label": "Oct pilot"}, {"label": "Nov audit"}, {"label": "Dec scale"}], "size_pct": 60}],
  "takeaway": "Hold the December release until North passes its QA gates."}]}`

func TestKPICaptionNotReportedAsBodyText(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := semanticTestConfig(t)
	out := renderDeckSpec(t, mc, map[string]any{"spec": kpiCaptionSpec})
	for _, d := range out.Diagnostics {
		if d.Code == patterns.ErrCodeTextBelowReadableMin && strings.Contains(d.Message, "body text") &&
			strings.Contains(d.RawPath, "/rendered_shapes/") {
			t.Errorf("KPI caption reported as body text: %+v", d)
		}
	}
	if out.DeterministicReady == nil || !*out.DeterministicReady {
		var codes []string
		for _, d := range out.Diagnostics {
			codes = append(codes, d.Code+" "+d.Message)
		}
		t.Errorf("deterministic_ready = %v; diagnostics: %s", out.DeterministicReady, strings.Join(codes, "; "))
	}
}
