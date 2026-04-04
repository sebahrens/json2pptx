package types

import (
	"strings"
	"testing"
)

func TestCalculateDynamicScale(t *testing.T) {
	tests := []struct {
		name                 string
		placeholderWidthEMU  EMU
		outputWidth          int
		expectedMinScale     float64
		expectedMaxScale     float64
		description          string
	}{
		{
			name:                "small placeholder uses minimum scale",
			placeholderWidthEMU: FromInches(5.0), // 5 inches
			outputWidth:         800,
			// 5 inches × 150 DPI = 750px, 750/800 = 0.9375 → clamped to 2.0
			expectedMinScale: DefaultMinScale,
			expectedMaxScale: DefaultMinScale,
			description:      "Small placeholders should use minimum 2.0 scale",
		},
		{
			name:                "medium placeholder uses calculated scale",
			placeholderWidthEMU: FromInches(12.0), // 12 inches
			outputWidth:         800,
			// 12 inches × 150 DPI = 1800px, 1800/800 = 2.25
			expectedMinScale: 2.2,
			expectedMaxScale: 2.3,
			description:      "Medium placeholders should use calculated scale",
		},
		{
			name:                "large placeholder uses higher scale",
			placeholderWidthEMU: FromInches(20.0), // 20 inches (large display)
			outputWidth:         800,
			// 20 inches × 150 DPI = 3000px, 3000/800 = 3.75
			expectedMinScale: 3.7,
			expectedMaxScale: 3.8,
			description:      "Large placeholders should use higher scale for quality",
		},
		{
			name:                "exact threshold at minimum",
			placeholderWidthEMU: FromInches(10.67), // ~10.67 inches
			outputWidth:         800,
			// 10.67 inches × 150 DPI = 1600px, 1600/800 = 2.0
			expectedMinScale: 2.0,
			expectedMaxScale: 2.05,
			description:      "At threshold, should use exactly minimum scale",
		},
		{
			name:                "standard slide width with default chart size",
			placeholderWidthEMU: 9144000, // 10 inches
			outputWidth:         DefaultChartWidth,      // 800px
			// 10 inches × 150 DPI = 1500px, 1500/800 = 1.875 → clamped to 2.0
			expectedMinScale: DefaultMinScale,
			expectedMaxScale: DefaultMinScale,
			description:      "Standard slide width with default output should use minimum",
		},
		{
			name:                "zero output width returns minimum",
			placeholderWidthEMU: FromInches(10.0),
			outputWidth:         0,
			expectedMinScale:    DefaultMinScale,
			expectedMaxScale:    DefaultMinScale,
			description:         "Zero output width should return minimum scale",
		},
		{
			name:                "negative output width returns minimum",
			placeholderWidthEMU: FromInches(10.0),
			outputWidth:         -100,
			expectedMinScale:    DefaultMinScale,
			expectedMaxScale:    DefaultMinScale,
			description:         "Negative output width should return minimum scale",
		},
		{
			name:                "zero placeholder width returns minimum",
			placeholderWidthEMU: 0,
			outputWidth:         800,
			expectedMinScale:    DefaultMinScale,
			expectedMaxScale:    DefaultMinScale,
			description:         "Zero placeholder width should return minimum scale",
		},
		{
			name:                "negative placeholder width returns minimum",
			placeholderWidthEMU: -1000,
			outputWidth:         800,
			expectedMinScale:    DefaultMinScale,
			expectedMaxScale:    DefaultMinScale,
			description:         "Negative placeholder width should return minimum scale",
		},
		{
			name:                "very small output width with normal placeholder",
			placeholderWidthEMU: FromInches(10.0), // 10 inches
			outputWidth:         100,              // Very small base output
			// 10 inches × 150 DPI = 1500px, 1500/100 = 15.0
			expectedMinScale: 14.9,
			expectedMaxScale: 15.1,
			description:      "Small output width should result in higher scale",
		},
		{
			name:                "very large output width already meets DPI",
			placeholderWidthEMU: FromInches(10.0), // 10 inches
			outputWidth:         2000,             // Large base output
			// 10 inches × 150 DPI = 1500px, 1500/2000 = 0.75 → clamped to 2.0
			expectedMinScale: DefaultMinScale,
			expectedMaxScale: DefaultMinScale,
			description:      "Large output width should still use minimum scale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale := CalculateDynamicScale(tt.placeholderWidthEMU, tt.outputWidth)

			if scale < tt.expectedMinScale || scale > tt.expectedMaxScale {
				t.Errorf("CalculateDynamicScale(%d EMU, %d) = %.4f, want between %.2f and %.2f\n%s",
					tt.placeholderWidthEMU, tt.outputWidth, scale,
					tt.expectedMinScale, tt.expectedMaxScale, tt.description)
			}
		})
	}
}

func TestCalculateDynamicScaleConstants(t *testing.T) {
	// Verify the constants are sensible
	if DefaultMinScale < 1.0 {
		t.Errorf("DefaultMinScale = %.2f, should be at least 1.0", DefaultMinScale)
	}

	if TargetDPI < 72 {
		t.Errorf("TargetDPI = %.2f, should be at least 72 (screen DPI)", TargetDPI)
	}

	// DefaultMinScale of 2.0 is a common choice for retina displays
	if DefaultMinScale != 2.0 {
		t.Errorf("DefaultMinScale = %.2f, expected 2.0", DefaultMinScale)
	}

	// 150 DPI is a good target for print quality
	if TargetDPI != 150.0 {
		t.Errorf("TargetDPI = %.2f, expected 150.0", TargetDPI)
	}
}

func TestChartSpecScale(t *testing.T) {
	// Test that ChartSpec with Scale field is properly converted to DiagramSpec
	chartSpec := &ChartSpec{
		Type:  ChartBar,
		Title: "Test Chart",
		Data: map[string]any{
			"A": 10,
			"B": 20,
		},
		Width:  800,
		Height: 600,
		Scale:  3.5,
	}

	diagramSpec := chartSpec.ToDiagramSpec()

	if diagramSpec.Scale != chartSpec.Scale {
		t.Errorf("DiagramSpec.Scale = %.2f, want %.2f", diagramSpec.Scale, chartSpec.Scale)
	}
}

func TestToDiagramSpec_DiagramTypes(t *testing.T) {
	// Diagram types (pyramid, swot, etc.) should map to their raw svggen type ID,
	// NOT with a "_chart" suffix appended.
	tests := []struct {
		chartType ChartType
		wantType  string
	}{
		{"pyramid", "pyramid"},
		{"swot", "swot"},
		{"timeline", "timeline"},
		{"process_flow", "process_flow"},
		{"matrix_2x2", "matrix_2x2"},
		{"org_chart", "org_chart"},
		{"venn", "venn"},
		{"gantt", "gantt"},
		{"value_chain", "value_chain"},
		{"business_model_canvas", "business_model_canvas"},
		{"kpi_dashboard", "kpi_dashboard"},
		{"heatmap", "heatmap"},
		{"fishbone", "fishbone"},
		{"pestel", "pestel"},
		// Standard chart types still work
		{ChartBar, "bar_chart"},
		{ChartLine, "line_chart"},
		{ChartPie, "pie_chart"},
	}

	for _, tt := range tests {
		t.Run(string(tt.chartType), func(t *testing.T) {
			cs := &ChartSpec{
				Type: tt.chartType,
				Data: map[string]any{"A": 1},
			}
			ds := cs.ToDiagramSpec()
			if ds.Type != tt.wantType {
				t.Errorf("ToDiagramSpec().Type = %q, want %q", ds.Type, tt.wantType)
			}
		})
	}
}

func TestBuildChartData_TimeData(t *testing.T) {
	// TimeData should be used when Data is empty.
	spec := &ChartSpec{
		Type: ChartLine,
		TimeData: map[string]any{
			"Q1 2024": 100,
			"Q2 2024": 120,
			"Q3 2024": 140,
		},
		TimeOrder: []string{"Q1 2024", "Q2 2024", "Q3 2024"},
	}

	ds := spec.ToDiagramSpec()
	data := ds.Data

	categories, ok := data["categories"].([]string)
	if !ok {
		t.Fatalf("data[categories] = %T, want []string", data["categories"])
	}
	if len(categories) != 3 {
		t.Fatalf("len(categories) = %d, want 3", len(categories))
	}
	if categories[0] != "Q1 2024" {
		t.Errorf("categories[0] = %q, want %q", categories[0], "Q1 2024")
	}

	series, ok := data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", data["series"])
	}
	vals := series[0]["values"].([]float64)
	if vals[0] != 100 || vals[1] != 120 || vals[2] != 140 {
		t.Errorf("values = %v, want [100 120 140]", vals)
	}
}

func TestBuildChartData_DataTakesPrecedence(t *testing.T) {
	// When both Data and TimeData are set, Data should be used.
	spec := &ChartSpec{
		Type: ChartLine,
		Data: map[string]any{
			"A": 10,
		},
		DataOrder: []string{"A"},
		TimeData: map[string]any{
			"Q1": 100,
		},
	}

	ds := spec.ToDiagramSpec()
	data := ds.Data

	categories := data["categories"].([]string)
	if len(categories) != 1 || categories[0] != "A" {
		t.Errorf("categories = %v, want [A] (Data should take precedence when non-empty)", categories)
	}
}

func TestChartSpecZeroScale(t *testing.T) {
	// Test that ChartSpec with zero Scale passes through zero (renderer will use default)
	chartSpec := &ChartSpec{
		Type:  ChartBar,
		Title: "Test Chart",
		Data: map[string]any{
			"A": 10,
		},
		Width:  800,
		Height: 600,
		Scale:  0, // Explicit zero
	}

	diagramSpec := chartSpec.ToDiagramSpec()

	if diagramSpec.Scale != 0 {
		t.Errorf("DiagramSpec.Scale = %.2f, want 0 (to use renderer default)", diagramSpec.Scale)
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
	}{
		{"float64", float64(42.5), 42.5},
		{"int", int(42), 42.0},
		{"int64", int64(42), 42.0},
		{"int32", int32(42), 42.0},
		{"string returns 0", "not a number", 0.0},
		{"nil returns 0", nil, 0.0},
		{"slice returns 0", []any{1, 2}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloat64(tt.input)
			if result != tt.expected {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasStructuredValues(t *testing.T) {
	t.Run("flat numeric data", func(t *testing.T) {
		data := map[string]any{"A": 10.0, "B": 20.0}
		if hasStructuredValues(data) {
			t.Error("hasStructuredValues returned true for flat numeric data")
		}
	})

	t.Run("array values", func(t *testing.T) {
		data := map[string]any{"Q1": []any{10.0, 20.0}, "Q2": []any{15.0, 25.0}}
		if !hasStructuredValues(data) {
			t.Error("hasStructuredValues returned false for array data")
		}
	})

	t.Run("mixed values", func(t *testing.T) {
		data := map[string]any{"A": 10.0, "B": []any{1.0, 2.0}}
		if !hasStructuredValues(data) {
			t.Error("hasStructuredValues returned false for mixed data")
		}
	})

	t.Run("empty data", func(t *testing.T) {
		data := map[string]any{}
		if hasStructuredValues(data) {
			t.Error("hasStructuredValues returned true for empty data")
		}
	})

	t.Run("string values are not structured", func(t *testing.T) {
		data := map[string]any{"key": "string_value"}
		if hasStructuredValues(data) {
			t.Error("hasStructuredValues returned true for string value — strings are non-numeric scalars, not structured data")
		}
	})

	t.Run("bool values are not structured", func(t *testing.T) {
		data := map[string]any{"key": true}
		if hasStructuredValues(data) {
			t.Error("hasStructuredValues returned true for bool value — bools are non-numeric scalars, not structured data")
		}
	})

	t.Run("nil values are not structured", func(t *testing.T) {
		data := map[string]any{"key": nil}
		if hasStructuredValues(data) {
			t.Error("hasStructuredValues returned true for nil value — nil is a non-numeric scalar, not structured data")
		}
	})

	t.Run("slice values are structured", func(t *testing.T) {
		data := map[string]any{"key": []any{1.0, 2.0}}
		if !hasStructuredValues(data) {
			t.Error("hasStructuredValues returned false for []any value — slices are structured data")
		}
	})

	t.Run("map values are structured", func(t *testing.T) {
		data := map[string]any{"key": map[string]any{"a": 1}}
		if !hasStructuredValues(data) {
			t.Error("hasStructuredValues returned false for map[string]any value — maps are structured data")
		}
	})
}

func TestBuildChartData_StackedBarStructured(t *testing.T) {
	// Stacked bar with array data: each category maps to an array of series values
	spec := &ChartSpec{
		Type: ChartStackedBar,
		Data: map[string]any{
			"Q1": []any{10.0, 20.0, 30.0},
			"Q2": []any{15.0, 25.0, 35.0},
		},
		DataOrder:    []string{"Q1", "Q2"},
		SeriesLabels: []string{"Product A", "Product B", "Product C"},
	}

	ds := spec.ToDiagramSpec()
	data := ds.Data

	// Check categories
	categories, ok := data["categories"].([]string)
	if !ok {
		t.Fatalf("data[categories] = %T, want []string", data["categories"])
	}
	if len(categories) != 2 || categories[0] != "Q1" || categories[1] != "Q2" {
		t.Errorf("categories = %v, want [Q1 Q2]", categories)
	}

	// Check series
	series, ok := data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", data["series"])
	}
	if len(series) != 3 {
		t.Fatalf("len(series) = %d, want 3", len(series))
	}

	// Series names should come from SeriesLabels
	if series[0]["name"] != "Product A" {
		t.Errorf("series[0].name = %q, want %q", series[0]["name"], "Product A")
	}
	if series[1]["name"] != "Product B" {
		t.Errorf("series[1].name = %q, want %q", series[1]["name"], "Product B")
	}

	// Series 0 ("Product A") should have values [10, 15] (first element from each category)
	vals0, ok := series[0]["values"].([]float64)
	if !ok {
		t.Fatalf("series[0].values = %T, want []float64", series[0]["values"])
	}
	if vals0[0] != 10.0 || vals0[1] != 15.0 {
		t.Errorf("series[0].values = %v, want [10 15]", vals0)
	}
}

func TestBuildChartData_GroupedBarStructured(t *testing.T) {
	// Grouped bar with array data uses the same path as stacked bar
	spec := &ChartSpec{
		Type: ChartGroupedBar,
		Data: map[string]any{
			"2023": []any{100.0, 200.0},
			"2024": []any{120.0, 180.0},
		},
		DataOrder:    []string{"2023", "2024"},
		SeriesLabels: []string{"Revenue", "Costs"},
	}

	ds := spec.ToDiagramSpec()
	data := ds.Data

	series, ok := data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", data["series"])
	}
	if len(series) != 2 {
		t.Fatalf("len(series) = %d, want 2", len(series))
	}
	if series[0]["name"] != "Revenue" {
		t.Errorf("series[0].name = %q, want %q", series[0]["name"], "Revenue")
	}
}

func TestBuildChartData_ScatterStructured(t *testing.T) {
	// Scatter with structured point data
	spec := &ChartSpec{
		Type: ChartScatter,
		Data: map[string]any{
			"Group A": []any{
				map[string]any{"x": 1.0, "y": 2.0},
				map[string]any{"x": 3.0, "y": 4.0},
			},
		},
		DataOrder: []string{"Group A"},
	}

	ds := spec.ToDiagramSpec()
	data := ds.Data

	series, ok := data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", data["series"])
	}
	if len(series) != 1 {
		t.Fatalf("len(series) = %d, want 1", len(series))
	}
	if series[0]["name"] != "Group A" {
		t.Errorf("series[0].name = %q, want %q", series[0]["name"], "Group A")
	}

	points, ok := series[0]["points"].([]map[string]any)
	if !ok {
		t.Fatalf("series[0].points = %T, want []map[string]any", series[0]["points"])
	}
	if len(points) != 2 {
		t.Fatalf("len(points) = %d, want 2", len(points))
	}
	if points[0]["x"] != 1.0 || points[0]["y"] != 2.0 {
		t.Errorf("points[0] = %v, want {x:1, y:2}", points[0])
	}
}

func TestBuildChartData_ScatterAxisTitlesPreserved(t *testing.T) {
	t.Run("structured scatter with axis titles", func(t *testing.T) {
		spec := &ChartSpec{
			Type: ChartScatter,
			Data: map[string]any{
				"Initiatives": []any{
					map[string]any{"x": 2.5, "y": 10.7, "label": "API Platform"},
				},
				"x_label": "Effort",
				"y_label": "Impact",
			},
			DataOrder: []string{"x_label", "y_label", "Initiatives"},
		}
		ds := spec.ToDiagramSpec()
		data := ds.Data

		// Axis titles must survive the transformation
		if data["x_label"] != "Effort" {
			t.Errorf("x_label = %v, want %q", data["x_label"], "Effort")
		}
		if data["y_label"] != "Impact" {
			t.Errorf("y_label = %v, want %q", data["y_label"], "Impact")
		}

		// Axis title keys must NOT create spurious series entries
		series, ok := data["series"].([]map[string]any)
		if !ok {
			t.Fatalf("data[series] = %T, want []map[string]any", data["series"])
		}
		if len(series) != 1 {
			t.Errorf("len(series) = %d, want 1 (axis title keys should not become series)", len(series))
		}
		if series[0]["name"] != "Initiatives" {
			t.Errorf("series[0].name = %q, want %q", series[0]["name"], "Initiatives")
		}
	})

	t.Run("flat scatter with axis titles", func(t *testing.T) {
		spec := &ChartSpec{
			Type: ChartScatter,
			Data: map[string]any{
				"A":              10.0,
				"B":              20.0,
				"x_axis_title":   "Complexity",
				"y_axis_title":   "Value",
			},
		}
		ds := spec.ToDiagramSpec()
		data := ds.Data

		if data["x_axis_title"] != "Complexity" {
			t.Errorf("x_axis_title = %v, want %q", data["x_axis_title"], "Complexity")
		}
		if data["y_axis_title"] != "Value" {
			t.Errorf("y_axis_title = %v, want %q", data["y_axis_title"], "Value")
		}
	})

	t.Run("svggen-format scatter preserves axis titles", func(t *testing.T) {
		spec := &ChartSpec{
			Type: ChartScatter,
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name":   "Data",
						"points": []any{map[string]any{"x": 1.0, "y": 2.0}},
					},
				},
				"x_label": "Hours",
				"y_label": "Score",
			},
		}
		ds := spec.ToDiagramSpec()
		data := ds.Data

		// svggen-format data is passed through as-is, so axis titles survive
		if data["x_label"] != "Hours" {
			t.Errorf("x_label = %v, want %q", data["x_label"], "Hours")
		}
		if data["y_label"] != "Score" {
			t.Errorf("y_label = %v, want %q", data["y_label"], "Score")
		}
	})
}

func TestBuildChartData_BubbleAxisTitlesPreserved(t *testing.T) {
	spec := &ChartSpec{
		Type: ChartBubble,
		Data: map[string]any{
			"Products": []any{
				map[string]any{"x": 5.0, "y": 8.0, "size": 100.0},
			},
			"x_label": "Market Share",
			"y_label": "Growth Rate",
		},
		DataOrder: []string{"x_label", "y_label", "Products"},
	}
	ds := spec.ToDiagramSpec()
	data := ds.Data

	if data["x_label"] != "Market Share" {
		t.Errorf("x_label = %v, want %q", data["x_label"], "Market Share")
	}
	if data["y_label"] != "Growth Rate" {
		t.Errorf("y_label = %v, want %q", data["y_label"], "Growth Rate")
	}

	series, ok := data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", data["series"])
	}
	if len(series) != 1 {
		t.Errorf("len(series) = %d, want 1", len(series))
	}
}

func TestBuildChartData_FlatDataStillWorks(t *testing.T) {
	// Verify backward compat: flat float64 data still works for all chart types
	flatData := map[string]any{
		"A": 10.0,
		"B": 20.0,
		"C": 30.0,
	}

	chartTypes := []ChartType{
		ChartBar, ChartLine, ChartPie, ChartDonut, ChartFunnel,
		ChartGauge, ChartTreemap, ChartWaterfall, ChartArea,
		ChartRadar, ChartScatter,
	}

	for _, ct := range chartTypes {
		t.Run(string(ct), func(t *testing.T) {
			spec := &ChartSpec{
				Type: ct,
				Data: flatData,
			}
			ds := spec.ToDiagramSpec()
			if ds == nil {
				t.Fatal("ToDiagramSpec() returned nil")
			}
			if len(ds.Data) == 0 {
				t.Error("DiagramSpec.Data is empty")
			}
		})
	}
}

func TestIsAlreadySvggenFormat(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]any
		chartType ChartType
		want      bool
	}{
		{
			name:      "categories+series passes through",
			data:      map[string]any{"categories": []any{"Q1", "Q2"}, "series": []any{map[string]any{"name": "A", "values": []any{10.0, 20.0}}}},
			chartType: ChartGroupedBar,
			want:      true,
		},
		{
			name:      "series-only for bubble passes through",
			data:      map[string]any{"series": []any{map[string]any{"name": "S1", "x_values": []any{1.0}, "values": []any{2.0}, "bubble_values": []any{3.0}}}},
			chartType: ChartBubble,
			want:      true,
		},
		{
			name:      "series-only for scatter passes through",
			data:      map[string]any{"series": []any{map[string]any{"name": "S1", "points": []any{}}}},
			chartType: ChartScatter,
			want:      true,
		},
		{
			name:      "series-only for bar does NOT pass through",
			data:      map[string]any{"series": []any{map[string]any{"name": "S1", "values": []any{1.0}}}},
			chartType: ChartBar,
			want:      false,
		},
		{
			name:      "gauge value+min+max passes through",
			data:      map[string]any{"value": 0.92, "min": 0.0, "max": 1.0},
			chartType: ChartGauge,
			want:      true,
		},
		{
			name:      "gauge value+max passes through",
			data:      map[string]any{"value": 50.0, "max": 100.0},
			chartType: ChartGauge,
			want:      true,
		},
		{
			name:      "waterfall points passes through",
			data:      map[string]any{"points": []any{map[string]any{"label": "Rev", "value": 1000.0, "type": "total"}}},
			chartType: ChartWaterfall,
			want:      true,
		},
		{
			name:      "funnel stages passes through",
			data:      map[string]any{"stages": []any{map[string]any{"label": "Visitors", "value": 1000.0}}},
			chartType: ChartFunnel,
			want:      true,
		},
		{
			name:      "treemap items passes through",
			data:      map[string]any{"items": []any{map[string]any{"label": "A", "value": 55.0}}},
			chartType: ChartTreemap,
			want:      true,
		},
		{
			name:      "pyramid levels passes through",
			data:      map[string]any{"levels": []any{map[string]any{"label": "Top"}}},
			chartType: "pyramid",
			want:      true,
		},
		{
			name:      "flat numeric data does NOT pass through",
			data:      map[string]any{"North": 250.0, "South": 180.0},
			chartType: ChartBar,
			want:      false,
		},
		{
			name:      "empty data does NOT pass through",
			data:      map[string]any{},
			chartType: ChartBar,
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAlreadySvggenFormat(tt.data, tt.chartType)
			if got != tt.want {
				t.Errorf("isAlreadySvggenFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildChartData_SvggenPassthrough(t *testing.T) {
	// Verify that svggen-native data passes through buildChartData unchanged.
	tests := []struct {
		name      string
		spec      *ChartSpec
		wantKey   string
		wantCheck func(map[string]any) bool
	}{
		{
			name: "grouped_bar with categories+series",
			spec: &ChartSpec{
				Type: ChartGroupedBar,
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3"},
					"series": []any{
						map[string]any{"name": "2024", "values": []any{120.0, 150.0, 180.0}},
						map[string]any{"name": "2025", "values": []any{140.0, 170.0, 210.0}},
					},
				},
			},
			wantKey: "categories",
			wantCheck: func(d map[string]any) bool {
				cats, ok := d["categories"].([]any)
				return ok && len(cats) == 3 && cats[0] == "Q1"
			},
		},
		{
			name: "bubble with series containing x_values",
			spec: &ChartSpec{
				Type: ChartBubble,
				Data: map[string]any{
					"series": []any{
						map[string]any{
							"name":          "Products",
							"x_values":      []any{10.0, 20.0},
							"values":        []any{80.0, 60.0},
							"bubble_values": []any{100.0, 200.0},
						},
					},
				},
			},
			wantKey: "series",
			wantCheck: func(d map[string]any) bool {
				series, ok := d["series"].([]any)
				if !ok || len(series) != 1 {
					return false
				}
				s := series[0].(map[string]any)
				_, hasX := s["x_values"]
				return hasX
			},
		},
		{
			name: "gauge with value+min+max",
			spec: &ChartSpec{
				Type: ChartGauge,
				Data: map[string]any{
					"value":  0.92,
					"min":    0.0,
					"max":    1.0,
					"target": 0.99,
				},
			},
			wantKey: "value",
			wantCheck: func(d map[string]any) bool {
				v, _ := d["value"].(float64)
				min, _ := d["min"].(float64)
				max, _ := d["max"].(float64)
				return v == 0.92 && min == 0.0 && max == 1.0
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := tt.spec.ToDiagramSpec()
			if ds == nil {
				t.Fatal("ToDiagramSpec returned nil")
			}
			if _, ok := ds.Data[tt.wantKey]; !ok {
				t.Errorf("missing expected key %q in data: %v", tt.wantKey, ds.Data)
			}
			if !tt.wantCheck(ds.Data) {
				t.Errorf("data check failed for %s: %v", tt.name, ds.Data)
			}
		})
	}
}

func TestToDiagramSpec_FlatMapWarnings(t *testing.T) {
	// Chart types that auto-convert flat map data should emit warnings.
	tests := []struct {
		name        string
		spec        *ChartSpec
		wantWarning bool
		wantSubstr  string
	}{
		{
			name: "waterfall flat map emits warning",
			spec: &ChartSpec{
				Type: ChartWaterfall,
				Data: map[string]any{"Revenue": 100.0, "Costs": -40.0},
			},
			wantWarning: true,
			wantSubstr:  "waterfall chart received flat data",
		},
		{
			name: "funnel flat map emits warning",
			spec: &ChartSpec{
				Type: ChartFunnel,
				Data: map[string]any{"Leads": 1000.0, "Deals": 50.0},
			},
			wantWarning: true,
			wantSubstr:  "funnel chart received flat data",
		},
		{
			name: "gauge flat map emits warning",
			spec: &ChartSpec{
				Type: ChartGauge,
				Data: map[string]any{"score": 75.0},
			},
			wantWarning: true,
			wantSubstr:  "gauge chart received flat data",
		},
		{
			name: "treemap flat map emits warning",
			spec: &ChartSpec{
				Type: ChartTreemap,
				Data: map[string]any{"A": 10.0, "B": 20.0},
			},
			wantWarning: true,
			wantSubstr:  "treemap chart received flat data",
		},
		{
			name: "pyramid flat map emits warning",
			spec: &ChartSpec{
				Type: "pyramid",
				Data: map[string]any{"Base": 100.0, "Top": 10.0},
			},
			wantWarning: true,
			wantSubstr:  "pyramid chart received flat data",
		},
		{
			name: "scatter flat map emits warning",
			spec: &ChartSpec{
				Type: ChartScatter,
				Data: map[string]any{"A": 10.0, "B": 20.0},
			},
			wantWarning: true,
			wantSubstr:  "scatter chart received flat data",
		},
		{
			name: "bar flat map does not emit warning",
			spec: &ChartSpec{
				Type: ChartBar,
				Data: map[string]any{"Q1": 10.0, "Q2": 20.0},
			},
			wantWarning: false,
		},
		{
			name: "pie flat map does not emit warning",
			spec: &ChartSpec{
				Type: ChartPie,
				Data: map[string]any{"A": 30.0, "B": 70.0},
			},
			wantWarning: false,
		},
		{
			name: "waterfall with structured data has no warning",
			spec: &ChartSpec{
				Type: ChartWaterfall,
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "Revenue", "value": 100.0, "type": "increase"},
					},
				},
			},
			wantWarning: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := tt.spec.ToDiagramSpec()
			if ds == nil {
				t.Fatal("ToDiagramSpec returned nil")
			}
			if tt.wantWarning {
				if len(ds.Warnings) == 0 {
					t.Errorf("expected warnings but got none")
				} else {
					found := false
					for _, w := range ds.Warnings {
						if strings.Contains(w, tt.wantSubstr) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected warning containing %q, got %v", tt.wantSubstr, ds.Warnings)
					}
				}
			} else {
				if len(ds.Warnings) > 0 {
					t.Errorf("expected no warnings but got %v", ds.Warnings)
				}
			}
		})
	}
}

func TestBuildChartData_StackedBarSeriesLabelsFallback(t *testing.T) {
	// When SeriesLabels is not provided, auto-generate "Series N" names
	spec := &ChartSpec{
		Type: ChartStackedBar,
		Data: map[string]any{
			"Q1": []any{10.0, 20.0},
		},
		DataOrder: []string{"Q1"},
		// No SeriesLabels
	}

	ds := spec.ToDiagramSpec()
	data := ds.Data
	series := data["series"].([]map[string]any)
	if series[0]["name"] != "Series 1" {
		t.Errorf("series[0].name = %q, want %q", series[0]["name"], "Series 1")
	}
	if series[1]["name"] != "Series 2" {
		t.Errorf("series[1].name = %q, want %q", series[1]["name"], "Series 2")
	}
}
