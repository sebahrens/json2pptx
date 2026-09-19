package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-r87g: the legibility ceiling read the chart data map's KEYS,
// so a 15-slice pie authored in the STRUCTURED form SKILL.md documents —
// {categories: [...], values: [...]} — counted as two categories and sailed
// through. The detector was blind to exactly the wide datasets it exists for.
func TestChartCategoryLabels_ReadsTheStructuredForm(t *testing.T) {
	cats := make([]any, 15)
	for i := range cats {
		cats[i] = fmt.Sprintf("Site %d", i+1)
	}

	structured := &types.ChartSpec{ //nolint:staticcheck // the authored chart shape
		Type:      "pie",
		Data:      map[string]any{"categories": cats, "values": []any{1, 2, 3}},
		DataOrder: []string{"categories", "values"}, // what the decoder records for this shape
	}
	if got := chartCategoryLabels(structured); len(got) != 15 {
		t.Errorf("structured form read %d categories, want 15 (got %v)", len(got), got)
	}

	// The flat map form still works, and its authored order still wins.
	flat := &types.ChartSpec{ //nolint:staticcheck
		Type:      "pie",
		Data:      map[string]any{"B": 2.0, "A": 1.0},
		DataOrder: []string{"B", "A"},
	}
	if got := chartCategoryLabels(flat); len(got) != 2 || got[0] != "B" {
		t.Errorf("flat form = %v, want the authored order [B A]", got)
	}
}

// The ceiling has to fire on the structured form end to end, not just count it.
func TestChartLegibility_FiresOnStructuredPie(t *testing.T) {
	cats := make([]any, 15)
	for i := range cats {
		cats[i] = fmt.Sprintf("Segment %d", i+1)
	}
	var in PresentationInput
	raw := `{"template":"t","slides":[{"slide_type":"chart","content":[
		{"placeholder_id":"body","type":"chart","chart_value":{"type":"pie","data":{"categories":[],"values":[]}}}]}]}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	in.Slides[0].Content[0].ChartValue.Data = map[string]any{"categories": cats, "values": cats}
	in.Slides[0].Content[0].ChartValue.DataOrder = []string{"categories", "values"}

	var found bool
	for _, f := range collectChartLegibilityFindings(&in) {
		if f.Code == patterns.ErrCodeChartOverloaded {
			found = true
			if !strings.Contains(f.Message, "15 categories") {
				t.Errorf("message does not report the real count: %s", f.Message)
			}
		}
	}
	if !found {
		t.Error("a 15-slice pie in the structured form was not reported")
	}
}
