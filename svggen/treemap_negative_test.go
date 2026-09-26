package svggen

import (
	"strings"
	"testing"
)

// go-slide-creator-s1uvj.33: the squarified layout turns each value into an
// area share of the total. A negative value produced a negative area, so its
// tile (and the tiles laid out after it) were drawn off the canvas. A treemap
// cannot show a negative quantity, so negative values are rejected with a
// clear error naming the offending node; zero values stay allowed.
func TestTreemap_RejectsNegativeValues(t *testing.T) {
	diagram := &TreemapDiagram{NewBaseDiagram("treemap_chart")}
	cases := []struct {
		name      string
		data      map[string]any
		wantLabel string
	}{
		{
			name: "flat nodes",
			data: map[string]any{"nodes": []any{
				map[string]any{"label": "Growth", "value": 100.0},
				map[string]any{"label": "Churn", "value": -40.0},
			}},
			wantLabel: "Churn",
		},
		{
			name: "nested child",
			data: map[string]any{"nodes": []any{
				map[string]any{"label": "EMEA", "children": []any{
					map[string]any{"label": "UK", "value": 30.0},
					map[string]any{"label": "Returns", "value": -5.0},
				}},
				map[string]any{"label": "APAC", "value": 50.0},
			}},
			wantLabel: "Returns",
		},
		{
			name: "values with categories",
			data: map[string]any{
				"categories": []any{"A", "B"},
				"values":     []any{10.0, -3.0},
			},
			wantLabel: "B",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &RequestEnvelope{Type: "treemap_chart", Data: tc.data, Output: OutputSpec{Width: 600, Height: 400}}
			err := diagram.Validate(req)
			if err == nil {
				t.Fatal("Validate accepted a negative treemap value")
			}
			if !strings.Contains(err.Error(), "negative") || !strings.Contains(err.Error(), tc.wantLabel) {
				t.Errorf("error %q should say the value is negative and name %q", err, tc.wantLabel)
			}
			if _, err := Render(req); err == nil {
				t.Error("Render accepted a negative treemap value")
			}
		})
	}

	t.Run("zero values still render", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "treemap_chart",
			Data: map[string]any{"nodes": []any{
				map[string]any{"label": "A", "value": 10.0},
				map[string]any{"label": "B", "value": 0.0},
			}},
			Output: OutputSpec{Width: 600, Height: 400},
		}
		if _, err := Render(req); err != nil {
			t.Fatalf("zero value rejected: %v", err)
		}
	})

	t.Run("Draw rejects negative leaf", func(t *testing.T) {
		chart := NewTreemapChart(NewSVGBuilder(600, 400), DefaultTreemapChartConfig(600, 400))
		err := chart.Draw(TreemapData{Nodes: []*TreemapNode{{Label: "A", Value: 10}, {Label: "B", Value: -1}}})
		if err == nil {
			t.Fatal("Draw accepted a negative leaf value")
		}
	})
}
