package semantic

import (
	"fmt"
	"testing"
)

// TestChartInsightOverCap_PlanMatchesCompile guards go-slide-creator-j02x: an
// over-cap chart_insight must compile to a two-column slide (chart in body,
// insights in body_2) and the planner must report that same layout, so the
// explain projection, the listed alternative and compile agree.
func TestChartInsightOverCap_PlanMatchesCompile(t *testing.T) {
	insights := make([]any, 8)
	for i := range insights {
		insights[i] = fmt.Sprintf("insight %d", i+1)
	}
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title":    "Revenue",
			"takeaway": "It grew.",
			"chart": map[string]any{
				"type": "bar_chart",
				"data": map[string]any{"categories": []any{"A", "B"}, "series": []any{map[string]any{"name": "Rev", "values": []any{1, 2}}}},
			},
			"insights": insights,
		}}},
	}

	ir := Normalize(spec)
	if got := ir.Slides[0].Visual.Layout; got != "two-column" {
		t.Errorf("planned layout = %q, want two-column", got)
	}
	if got := ir.Slides[0].Visual.Pattern; got != "" {
		t.Errorf("planned pattern = %q, want none for over-cap", got)
	}

	input, _, err := Compile(spec, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	s := input.Slides[0]
	if s.SlideType != "two-column" {
		t.Fatalf("compiled slide_type = %q, want two-column", s.SlideType)
	}
	placements := map[string]string{}
	for _, c := range s.Content {
		placements[c.Type] = c.PlaceholderID
	}
	if placements["diagram"] != "body" || placements["bullets"] != "body_2" {
		t.Errorf("chart/insights placement = %v, want diagram->body bullets->body_2", placements)
	}
}
