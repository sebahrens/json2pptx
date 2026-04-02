package svggen_test

import (
	"testing"

	"github.com/ahrens/svggen"
)

// TestNarrowWidthCharts tests chart rendering at half-width (~480pt) simulating
// two-column layout slots in slideLayout99. These tests verify that charts
// render successfully rather than producing errors that cause placeholder fallback.
func TestNarrowWidthCharts(t *testing.T) {
	tests := []struct {
		name string
		req  *svggen.RequestEnvelope
	}{
		{
			name: "bar_chart_with_labels_alias",
			req: &svggen.RequestEnvelope{
				Type:  "bar_chart",
				Title: "Revenue by Region",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"labels": []any{"North America", "EMEA", "APAC", "Latin America", "Middle East", "Africa"},
					"series": []any{
						map[string]any{"name": "FY2025", "values": []any{420.0, 310.0, 275.0, 95.0, 68.0, 42.0}},
						map[string]any{"name": "FY2026E", "values": []any{485.0, 355.0, 340.0, 118.0, 82.0, 55.0}},
					},
				},
				Style: svggen.StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "line_chart_with_labels_alias",
			req: &svggen.RequestEnvelope{
				Type:  "line_chart",
				Title: "Monthly Trend",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"labels": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
					"series": []any{
						map[string]any{"name": "Sales", "values": []any{10.0, 15.0, 13.0, 17.0, 20.0, 22.0, 19.0, 25.0, 23.0, 28.0, 30.0, 35.0}},
					},
				},
				Style: svggen.StyleSpec{ShowLegend: true},
			},
		},
		{
			name: "stacked_bar_with_labels_alias",
			req: &svggen.RequestEnvelope{
				Type:  "stacked_bar_chart",
				Title: "Revenue Breakdown",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"labels": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Product A", "values": []any{30.0, 35.0, 40.0, 45.0}},
						map[string]any{"name": "Product B", "values": []any{20.0, 25.0, 22.0, 28.0}},
					},
				},
				Style: svggen.StyleSpec{ShowLegend: true},
			},
		},
		{
			name: "grouped_bar_with_labels_alias",
			req: &svggen.RequestEnvelope{
				Type:  "grouped_bar_chart",
				Title: "Regional Comparison",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"labels": []any{"North America", "Europe", "APAC", "LATAM"},
					"series": []any{
						map[string]any{"name": "2024", "values": []any{100.0, 85.0, 120.0, 45.0}},
						map[string]any{"name": "2025", "values": []any{110.0, 92.0, 135.0, 52.0}},
					},
				},
				Style: svggen.StyleSpec{ShowLegend: true},
			},
		},
		{
			name: "org_chart_flat_nodes_with_parent",
			req: &svggen.RequestEnvelope{
				Type:  "org_chart",
				Title: "Team Structure",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"nodes": []any{
						map[string]any{"id": "1", "name": "Sarah Chen", "title": "Chief Digital Officer", "parent": ""},
						map[string]any{"id": "2", "name": "Marcus Williams", "title": "VP Engineering", "parent": "1"},
						map[string]any{"id": "3", "name": "Priya Sharma", "title": "VP Data & Analytics", "parent": "1"},
						map[string]any{"id": "4", "name": "James Liu", "title": "Lead Architect", "parent": "2"},
						map[string]any{"id": "5", "name": "Ana Costa", "title": "Data Science Lead", "parent": "3"},
					},
				},
			},
		},
		{
			name: "org_chart_flat_nodes_10_deep",
			req: &svggen.RequestEnvelope{
				Type:  "org_chart",
				Title: "Large Organization",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"nodes": []any{
						map[string]any{"id": "1", "name": "CEO", "title": "Chief Executive", "parent": ""},
						map[string]any{"id": "2", "name": "CTO", "title": "Chief Technology", "parent": "1"},
						map[string]any{"id": "3", "name": "CFO", "title": "Chief Financial", "parent": "1"},
						map[string]any{"id": "4", "name": "VP Eng", "title": "VP Engineering", "parent": "2"},
						map[string]any{"id": "5", "name": "VP Product", "title": "VP Product", "parent": "2"},
						map[string]any{"id": "6", "name": "VP Sales", "title": "VP Sales", "parent": "3"},
						map[string]any{"id": "7", "name": "Lead Dev", "title": "Lead Developer", "parent": "4"},
						map[string]any{"id": "8", "name": "Sr Dev", "title": "Senior Developer", "parent": "4"},
						map[string]any{"id": "9", "name": "PM", "title": "Product Manager", "parent": "5"},
						map[string]any{"id": "10", "name": "Designer", "title": "UX Designer", "parent": "5"},
					},
				},
			},
		},
		{
			name: "bar_chart_with_categories_still_works",
			req: &svggen.RequestEnvelope{
				Type:  "bar_chart",
				Title: "Standard Format",
				Output: svggen.OutputSpec{Width: 480, Height: 360},
				Data: map[string]any{
					"categories": []any{"A", "B", "C"},
					"series": []any{
						map[string]any{"name": "S1", "values": []any{1.0, 2.0, 3.0}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svggen.RenderMultiFormat(tt.req, "svg", "png")
			if err != nil {
				t.Fatalf("render failed: %v", err)
			}
			if len(result.PNG) == 0 {
				t.Fatal("returned empty PNG")
			}
			t.Logf("OK: SVG=%d bytes, PNG=%d bytes", len(result.SVG.Content), len(result.PNG))
		})
	}
}
