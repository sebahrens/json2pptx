package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// A 35% insights column carrying one short bullet leaves a large empty block
// under it while the chart is squeezed into 55% of the slide
// (go-slide-creator-pyxn).
func TestSparseInsightsWidenTheChart(t *testing.T) {
	chart := &types.DiagramSpec{Type: "pie", Data: map[string]any{
		"categories": []any{"R&D", "S&M"}, "values": []any{69.0, 31.0},
	}}

	cases := []struct {
		name string
		vals *ChartInsightsSplitValues
		want float64
	}{
		{
			name: "one short insight",
			vals: &ChartInsightsSplitValues{Insights: []string{"R&D and S&M are 69% of spend"}},
			want: chartInsightsWidePct,
		},
		{
			name: "two short insights",
			vals: &ChartInsightsSplitValues{Insights: []string{"DACH carries the growth", "Nordics is flat"}},
			want: chartInsightsWidePct,
		},
		{
			name: "three insights fill the column",
			vals: &ChartInsightsSplitValues{Insights: []string{"One", "Two", "Three"}},
			want: chartInsightsDefaultPct,
		},
		{
			name: "two long insights fill the column",
			vals: &ChartInsightsSplitValues{Insights: []string{
				strings.Repeat("a", 90), strings.Repeat("b", 90),
			}},
			want: chartInsightsDefaultPct,
		},
		{
			name: "a headline gives the column a reason to be wide",
			vals: &ChartInsightsSplitValues{
				Insights: []string{"R&D and S&M are 69% of spend"},
				Headline: &ChartInsightsHeadline{Value: "69%", Label: "of spend"},
			},
			want: chartInsightsDefaultPct,
		},
		{
			name: "so what does too",
			vals: &ChartInsightsSplitValues{
				Insights: []string{"R&D and S&M are 69% of spend"},
				SoWhat:   "Shift the next increment to R&D.",
			},
			want: chartInsightsDefaultPct,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sparseInsightsChartPct(c.vals); got != c.want {
				t.Errorf("sparseInsightsChartPct = %v, want %v", got, c.want)
			}

			// The default must reach the expanded grid's column widths.
			c.vals.Chart = chart
			grid, err := (&chartInsightsSplit{}).Expand(ExpandContext{}, c.vals, nil, nil)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			var cols []float64
			if err := json.Unmarshal(grid.Columns, &cols); err != nil {
				t.Fatalf("columns %s: %v", grid.Columns, err)
			}
			if len(cols) != 2 {
				t.Fatalf("expected 2 columns, got %v", cols)
			}
			if cols[0] != c.want {
				t.Errorf("chart column = %v, want %v", cols[0], c.want)
			}
		})
	}
}

// An explicit chart_width_pct still wins: the sparse rule only chooses the
// default.
func TestChartWidthOverrideBeatsTheSparseDefault(t *testing.T) {
	vals := &ChartInsightsSplitValues{
		Insights: []string{"R&D and S&M are 69% of spend"},
		Chart: &types.DiagramSpec{Type: "pie", Data: map[string]any{
			"categories": []any{"R&D"}, "values": []any{100.0},
		}},
	}
	grid, err := (&chartInsightsSplit{}).Expand(ExpandContext{}, vals, &ChartInsightsSplitOverrides{ChartWidthPct: 50}, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatalf("columns %s: %v", grid.Columns, err)
	}
	if cols[0] != 50 {
		t.Errorf("chart column = %v, want the requested 50", cols[0])
	}
}
