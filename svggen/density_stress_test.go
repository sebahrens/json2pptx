package svggen

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// Density Stress Tests
// =============================================================================
//
// These tests push each chart type to its limits with high data density,
// then verify that the rendered SVG maintains structural quality. Each test
// renders a chart with far more data points than typical golden tests (which
// max out at ~11 items) to stress label placement, font sizing, bar width
// calculation, and layout algorithms.
//
// Use -short to skip these tests in quick feedback loops.
// =============================================================================

// TestDensityStress_BarChart25Categories renders a bar chart with 25 categories
// and 3 series. This stresses x-axis label rotation/thinning and bar width
// calculations at high category counts.
func TestDensityStress_BarChart25Categories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 25)
	series1 := make([]any, 25)
	series2 := make([]any, 25)
	series3 := make([]any, 25)
	for i := 0; i < 25; i++ {
		categories[i] = fmt.Sprintf("Category %d", i+1)
		series1[i] = float64(50 + (i%7)*10)
		series2[i] = float64(30 + (i%5)*15)
		series3[i] = float64(20 + (i%9)*8)
	}

	req := &RequestEnvelope{
		Type:  "bar_chart",
		Title: "Dense Bar Chart: 25 Categories x 3 Series",
		Data: map[string]any{
			"categories": categories,
			"series": []any{
				map[string]any{"name": "Product A", "values": series1},
				map[string]any{"name": "Product B", "values": series2},
				map[string]any{"name": "Product C", "values": series3},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true, ShowLegend: true, ShowValues: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense bar chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify the SVG is substantial (25 categories x 3 series = 75 bars minimum)
	if len(svg) < 5000 {
		t.Errorf("SVG output suspiciously short (%d bytes) for 25x3 bar chart", len(svg))
	}

	// At least some category labels should appear (label thinning may remove some)
	labelCount := 0
	for i := 0; i < 25; i++ {
		if strings.Contains(svg, fmt.Sprintf("Category %d", i+1)) {
			labelCount++
		}
	}
	if labelCount == 0 {
		t.Error("no category labels found in SVG output -- all labels were dropped")
	}
}

// TestDensityStress_Waterfall15Points renders a waterfall chart with 15 data
// points at half-width (760x720) to stress bar width and value label fitting.
func TestDensityStress_Waterfall15Points(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	points := make([]any, 15)
	points[0] = map[string]any{"label": "Starting Revenue", "value": 1000, "type": "total"}
	for i := 1; i < 14; i++ {
		value := 30 - (i%3)*20 // alternating +30, +10, -10
		typ := "increase"
		if value < 0 {
			typ = "decrease"
		}
		points[i] = map[string]any{
			"label": fmt.Sprintf("Line Item %d", i),
			"value": value,
			"type":  typ,
		}
	}
	points[14] = map[string]any{"label": "Final Total", "value": 1100, "type": "total"}

	req := &RequestEnvelope{
		Type: "waterfall",
		Data: map[string]any{
			"title":  "Dense Waterfall: 15 Points at Half Width",
			"points": points,
		},
		Output: OutputSpec{Preset: "half_16x9"},
		Style:  StyleSpec{ShowValues: true, ShowGrid: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense waterfall: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify the chart renders substantial content. The tdewolff/canvas library
	// renders shapes as <path> elements, not <rect>, so we check path count
	// and overall SVG byte length to confirm 15 bars were rendered.
	pathCount := strings.Count(svg, "<path")
	if pathCount < 10 {
		t.Errorf("expected at least 10 <path> elements for 15 waterfall bars, got %d", pathCount)
	}
	if len(svg) < 5000 {
		t.Errorf("SVG output suspiciously short (%d bytes) for 15-point waterfall", len(svg))
	}
}

// TestDensityStress_Scatter50Points renders a scatter chart with 50 data points
// to stress point density and label placement.
func TestDensityStress_Scatter50Points(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	xValues := make([]any, 50)
	yValues := make([]any, 50)
	for i := 0; i < 50; i++ {
		xValues[i] = float64(i*2 + (i%7)*3)
		yValues[i] = float64(i*3 + (i%11)*5)
	}

	req := &RequestEnvelope{
		Type:  "scatter_chart",
		Title: "Dense Scatter: 50 Points",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name":     "Measurements",
					"values":   yValues,
					"x_values": xValues,
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense scatter chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify we have data points in the output. The tdewolff/canvas library
	// renders circles as <path> elements, not <circle>, so we check path count
	// and overall byte length to confirm scatter points were rendered.
	pathCount := strings.Count(svg, "<path")
	if pathCount < 30 {
		t.Errorf("expected at least 30 <path> elements for 50 scatter points, got %d", pathCount)
	}
	if len(svg) < 5000 {
		t.Errorf("SVG output suspiciously short (%d bytes) for 50-point scatter chart", len(svg))
	}
}

// TestDensityStress_Pie12Slices renders a pie chart with 12 slices at
// third-width (500x720) to stress label placement in a narrow container.
func TestDensityStress_Pie12Slices(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	values := make([]any, 12)
	categories := make([]any, 12)
	for i := 0; i < 12; i++ {
		values[i] = float64(5 + (i%4)*3)
		categories[i] = fmt.Sprintf("Segment %d", i+1)
	}

	req := &RequestEnvelope{
		Type: "pie_chart",
		Data: map[string]any{
			"values":     values,
			"categories": categories,
		},
		Output: OutputSpec{Preset: "third_16x9"},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense pie chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify some segment labels appear
	labelCount := 0
	for i := 0; i < 12; i++ {
		if strings.Contains(svg, fmt.Sprintf("Segment %d", i+1)) {
			labelCount++
		}
	}
	if labelCount < 3 {
		t.Errorf("expected at least some segment labels in the SVG, found only %d of 12", labelCount)
	}
}

// TestDensityStress_OrgChart4Levels renders an org chart with 4 levels and 15+
// nodes to stress the tree layout compression algorithm.
func TestDensityStress_OrgChart4Levels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	req := &RequestEnvelope{
		Type:  "org_chart",
		Title: "Dense Org Chart: 4 Levels, 17 Nodes",
		Data: map[string]any{
			"root": map[string]any{
				"name":  "Chief Executive Officer",
				"title": "CEO",
				"children": []any{
					map[string]any{
						"name":  "Chief Technology Officer",
						"title": "CTO",
						"children": []any{
							map[string]any{
								"name":  "VP Engineering",
								"title": "Platform",
								"children": []any{
									map[string]any{"name": "Backend Lead", "title": "SWE"},
									map[string]any{"name": "Frontend Lead", "title": "SWE"},
									map[string]any{"name": "Infra Lead", "title": "SRE"},
								},
							},
							map[string]any{
								"name":  "VP Data Science",
								"title": "Data",
								"children": []any{
									map[string]any{"name": "ML Engineer", "title": "ML"},
									map[string]any{"name": "Data Analyst", "title": "Analytics"},
								},
							},
						},
					},
					map[string]any{
						"name":  "Chief Financial Officer",
						"title": "CFO",
						"children": []any{
							map[string]any{"name": "Controller", "title": "Accounting"},
							map[string]any{"name": "FP&A Manager", "title": "Planning"},
						},
					},
					map[string]any{
						"name":  "Chief Operating Officer",
						"title": "COO",
						"children": []any{
							map[string]any{"name": "Ops Manager", "title": "Operations"},
							map[string]any{"name": "Supply Chain Lead", "title": "Logistics"},
							map[string]any{"name": "Quality Director", "title": "Quality"},
						},
					},
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense org chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify key names from all 4 levels appear
	mustContain := []string{"CEO", "CTO", "CFO", "COO"}
	for _, name := range mustContain {
		if !strings.Contains(svg, name) {
			t.Errorf("org chart SVG should contain %q for 4-level tree", name)
		}
	}
}

// TestDensityStress_LineChart30Points renders a line chart with 30 x-axis points
// to stress label thinning and tick mark rendering.
func TestDensityStress_LineChart30Points(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 30)
	values := make([]any, 30)
	for i := 0; i < 30; i++ {
		categories[i] = fmt.Sprintf("Day %d", i+1)
		values[i] = float64(50 + (i%10)*7 - (i%3)*4)
	}

	req := &RequestEnvelope{
		Type:  "line_chart",
		Title: "Dense Line Chart: 30 Data Points",
		Data: map[string]any{
			"categories": categories,
			"series": []any{
				map[string]any{"name": "Daily Metric", "values": values},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense line chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify there is at least one path element (the line itself)
	if !strings.Contains(svg, "<path") {
		t.Error("line chart SVG should contain <path> element for the data line")
	}
}

// TestDensityStress_Fishbone7CategoriesDense renders a fishbone diagram with
// 7 categories and 5 causes each (35 total causes) to stress diagonal spacing
// and text fitting.
func TestDensityStress_Fishbone7CategoriesDense(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 7)
	categoryNames := []string{"People", "Process", "Materials", "Equipment", "Environment", "Management", "Measurement"}
	for i := 0; i < 7; i++ {
		causes := make([]any, 5)
		for j := 0; j < 5; j++ {
			causes[j] = fmt.Sprintf("%s Cause %d", categoryNames[i], j+1)
		}
		categories[i] = map[string]any{
			"name":   categoryNames[i],
			"causes": causes,
		}
	}

	req := &RequestEnvelope{
		Type:  "fishbone",
		Title: "Dense Fishbone: 7 Categories, 5 Causes Each",
		Data: map[string]any{
			"effect":     "Critical Quality Issue",
			"categories": categories,
		},
		Output: OutputSpec{Width: 900, Height: 600},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense fishbone: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify all category names appear in the SVG
	for _, name := range categoryNames {
		if !strings.Contains(svg, name) {
			t.Errorf("fishbone SVG should contain category name %q", name)
		}
	}
}

// TestDensityStress_StackedBar20Categories renders a stacked bar chart with
// 20 categories and 4 series. This stresses segment width and label density.
func TestDensityStress_StackedBar20Categories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 20)
	s1 := make([]any, 20)
	s2 := make([]any, 20)
	s3 := make([]any, 20)
	s4 := make([]any, 20)
	for i := 0; i < 20; i++ {
		categories[i] = fmt.Sprintf("Region %d", i+1)
		s1[i] = float64(20 + i%5*3)
		s2[i] = float64(15 + i%4*4)
		s3[i] = float64(10 + i%7*2)
		s4[i] = float64(5 + i%3*6)
	}

	req := &RequestEnvelope{
		Type:  "stacked_bar_chart",
		Title: "Dense Stacked Bar: 20 Categories x 4 Series",
		Data: map[string]any{
			"categories": categories,
			"series": []any{
				map[string]any{"name": "Direct", "values": s1},
				map[string]any{"name": "Partner", "values": s2},
				map[string]any{"name": "Online", "values": s3},
				map[string]any{"name": "Other", "values": s4},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense stacked bar: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify the chart renders substantial content. The tdewolff/canvas library
	// renders rects as <path> elements, so we check path count and byte length
	// to confirm 20 categories x 4 series = 80 segments were rendered.
	pathCount := strings.Count(svg, "<path")
	if pathCount < 30 {
		t.Errorf("expected at least 30 <path> elements for 20x4 stacked bars, got %d", pathCount)
	}
	if len(svg) < 5000 {
		t.Errorf("SVG output suspiciously short (%d bytes) for 20x4 stacked bar chart", len(svg))
	}
}

// TestDensityStress_GroupedBar15Categories renders a grouped bar chart with
// 15 categories and 3 series to stress bar group width calculations.
func TestDensityStress_GroupedBar15Categories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 15)
	s1 := make([]any, 15)
	s2 := make([]any, 15)
	s3 := make([]any, 15)
	for i := 0; i < 15; i++ {
		categories[i] = fmt.Sprintf("Month %d", i+1)
		s1[i] = float64(100 + i*5)
		s2[i] = float64(80 + i*7)
		s3[i] = float64(60 + i*3)
	}

	req := &RequestEnvelope{
		Type:  "grouped_bar_chart",
		Title: "Dense Grouped Bar: 15 Categories x 3 Series",
		Data: map[string]any{
			"categories": categories,
			"series": []any{
				map[string]any{"name": "2024", "values": s1},
				map[string]any{"name": "2025", "values": s2},
				map[string]any{"name": "2026", "values": s3},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true, ShowLegend: true, ShowValues: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense grouped bar: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)
}

// TestDensityStress_AreaChart25Points renders an area chart with 25 x-axis
// points and 3 overlapping series to stress fill area rendering.
func TestDensityStress_AreaChart25Points(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 25)
	s1 := make([]any, 25)
	s2 := make([]any, 25)
	s3 := make([]any, 25)
	for i := 0; i < 25; i++ {
		categories[i] = fmt.Sprintf("Week %d", i+1)
		s1[i] = float64(100 + (i%8)*12)
		s2[i] = float64(80 + (i%6)*15)
		s3[i] = float64(50 + (i%10)*8)
	}

	req := &RequestEnvelope{
		Type:  "area_chart",
		Title: "Dense Area Chart: 25 Points x 3 Series",
		Data: map[string]any{
			"categories": categories,
			"series": []any{
				map[string]any{"name": "Web", "values": s1},
				map[string]any{"name": "Mobile", "values": s2},
				map[string]any{"name": "API", "values": s3},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense area chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)
}

// TestDensityStress_RadarChart10Axes renders a radar chart with 10 axis
// categories and 3 series to stress polygon rendering.
func TestDensityStress_RadarChart10Axes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	categories := make([]any, 10)
	s1 := make([]any, 10)
	s2 := make([]any, 10)
	s3 := make([]any, 10)
	for i := 0; i < 10; i++ {
		categories[i] = fmt.Sprintf("Skill %d", i+1)
		s1[i] = float64(40 + (i%5)*12)
		s2[i] = float64(50 + (i%7)*8)
		s3[i] = float64(30 + (i%4)*15)
	}

	req := &RequestEnvelope{
		Type:  "radar_chart",
		Title: "Dense Radar: 10 Axes x 3 Series",
		Data: map[string]any{
			"categories": categories,
			"series": []any{
				map[string]any{"name": "Employee A", "values": s1},
				map[string]any{"name": "Employee B", "values": s2},
				map[string]any{"name": "Employee C", "values": s3},
			},
		},
		Output: OutputSpec{Width: 800, Height: 800},
		Style:  StyleSpec{ShowLegend: true},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense radar chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify at least some axis labels appear
	for i := 0; i < 10; i++ {
		if strings.Contains(svg, fmt.Sprintf("Skill %d", i+1)) {
			return // At least one label found, good enough
		}
	}
	t.Error("no axis labels found in radar chart SVG output")
}

// TestDensityStress_Funnel10Stages renders a funnel chart with 10 stages
// to stress the progressive narrowing layout.
func TestDensityStress_Funnel10Stages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	stages := make([]any, 10)
	for i := 0; i < 10; i++ {
		stages[i] = map[string]any{
			"label": fmt.Sprintf("Stage %d: %s", i+1, []string{
				"Awareness", "Interest", "Consideration", "Intent",
				"Evaluation", "Trial", "Purchase", "Onboard",
				"Adoption", "Advocacy",
			}[i]),
			"value": float64(1000 - i*90),
		}
	}

	req := &RequestEnvelope{
		Type:  "funnel_chart",
		Title: "Dense Funnel: 10 Stages",
		Data: map[string]any{
			"stages": stages,
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense funnel: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)
}

// TestDensityStress_Treemap20Items renders a treemap with 20 items to stress
// the squarified layout algorithm.
func TestDensityStress_Treemap20Items(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	nodes := make([]any, 20)
	for i := 0; i < 20; i++ {
		nodes[i] = map[string]any{
			"label": fmt.Sprintf("Item %d", i+1),
			"value": float64(100 - i*3 + (i%5)*7),
		}
	}

	req := &RequestEnvelope{
		Type:  "treemap_chart",
		Title: "Dense Treemap: 20 Items",
		Data: map[string]any{
			"nodes": nodes,
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render dense treemap: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)
}

// TestDensityStress_SmallDimensions renders several chart types at very small
// dimensions (300x200) to stress font sizing and layout at thumbnail scale.
func TestDensityStress_SmallDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping density stress test in short mode")
	}

	testCases := []struct {
		name string
		req  *RequestEnvelope
	}{
		{
			name: "bar_chart_tiny",
			req: &RequestEnvelope{
				Type:  "bar_chart",
				Title: "Tiny Bar",
				Data: map[string]any{
					"categories": []any{"A", "B", "C", "D", "E", "F", "G", "H"},
					"series": []any{
						map[string]any{"name": "S1", "values": []any{10.0, 20.0, 15.0, 25.0, 30.0, 12.0, 18.0, 22.0}},
					},
				},
				Output: OutputSpec{Width: 300, Height: 200},
				Style:  StyleSpec{ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "waterfall_tiny",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Tiny Waterfall",
					"points": []any{
						map[string]any{"label": "Start", "value": 100, "type": "total"},
						map[string]any{"label": "A", "value": 20, "type": "increase"},
						map[string]any{"label": "B", "value": -10, "type": "decrease"},
						map[string]any{"label": "C", "value": 15, "type": "increase"},
						map[string]any{"label": "D", "value": -5, "type": "decrease"},
						map[string]any{"label": "End", "value": 120, "type": "total"},
					},
				},
				Output: OutputSpec{Width: 300, Height: 200},
				Style:  StyleSpec{ShowValues: true},
			},
		},
		{
			name: "pie_chart_tiny",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{40.0, 30.0, 20.0, 10.0},
					"categories": []any{"A", "B", "C", "D"},
				},
				Output: OutputSpec{Width: 300, Height: 200},
				Style:  StyleSpec{ShowLegend: true},
			},
		},
		{
			name: "line_chart_tiny",
			req: &RequestEnvelope{
				Type:  "line_chart",
				Title: "Tiny Line",
				Data: map[string]any{
					"categories": []any{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
					"series": []any{
						map[string]any{"name": "S1", "values": []any{10.0, 20.0, 15.0, 25.0, 30.0, 22.0, 28.0, 35.0, 30.0, 40.0}},
					},
				},
				Output: OutputSpec{Width: 300, Height: 200},
				Style:  StyleSpec{ShowGrid: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Render(tc.req)
			if err != nil {
				t.Fatalf("failed to render %s: %v", tc.name, err)
			}
			AssertSVGQuality(t, string(doc.Content))
		})
	}
}
