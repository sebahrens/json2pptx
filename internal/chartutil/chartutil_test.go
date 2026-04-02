package chartutil

import (
	"reflect"
	"testing"
)

func TestMapChartTypeToSvggen(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bar", "bar_chart"},
		{"line", "line_chart"},
		{"pie", "pie_chart"},
		{"donut", "donut_chart"},
		{"area", "area_chart"},
		{"radar", "radar_chart"},
		{"scatter", "scatter_chart"},
		{"stacked_bar", "stacked_bar_chart"},
		{"waterfall", "waterfall"},
		{"funnel", "funnel_chart"},
		{"gauge", "gauge_chart"},
		{"treemap", "treemap_chart"},
		{"bubble", "bubble_chart"},
		{"stacked_area", "stacked_area_chart"},
		{"grouped_bar", "grouped_bar_chart"},
		// default passthrough
		{"unknown_type", "unknown_type"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MapChartTypeToSvggen(tt.input)
			if got != tt.want {
				t.Errorf("MapChartTypeToSvggen(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePyramid_StringLevels(t *testing.T) {
	data := map[string]any{
		"levels": []any{"Top", "Mid", "Base"},
	}
	got := NormalizeDiagramData("pyramid", data)
	levels, ok := got["levels"].([]any)
	if !ok {
		t.Fatal("levels should be []any")
	}
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	for i, want := range []string{"Top", "Mid", "Base"} {
		m, ok := levels[i].(map[string]any)
		if !ok {
			t.Fatalf("level %d should be map[string]any", i)
		}
		if m["label"] != want {
			t.Errorf("level %d label = %q, want %q", i, m["label"], want)
		}
	}
}

func TestNormalizePyramid_AlreadyCanonical(t *testing.T) {
	data := map[string]any{
		"levels": []any{
			map[string]any{"label": "Top", "description": "desc"},
			map[string]any{"label": "Base"},
		},
	}
	got := NormalizeDiagramData("pyramid", data)
	levels := got["levels"].([]any)
	if len(levels) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(levels))
	}
	m := levels[0].(map[string]any)
	if m["description"] != "desc" {
		t.Error("should preserve existing object format")
	}
}

func TestNormalizeHeatmap_LabeledRows(t *testing.T) {
	data := map[string]any{
		"rows": []any{
			map[string]any{"label": "Enterprise", "values": []any{95, 88}},
			map[string]any{"label": "SMB", "values": []any{85, 55}},
		},
		"columns": []any{"Core", "Analytics"},
	}
	got := NormalizeDiagramData("heatmap", data)

	// Should have "values", "row_labels", "col_labels"
	if _, ok := got["rows"]; ok {
		t.Error("'rows' should be removed")
	}
	if _, ok := got["columns"]; ok {
		t.Error("'columns' should be renamed to 'col_labels'")
	}

	rowLabels := got["row_labels"].([]any)
	if !reflect.DeepEqual(rowLabels, []any{"Enterprise", "SMB"}) {
		t.Errorf("row_labels = %v", rowLabels)
	}

	colLabels := got["col_labels"].([]any)
	if !reflect.DeepEqual(colLabels, []any{"Core", "Analytics"}) {
		t.Errorf("col_labels = %v", colLabels)
	}

	values := got["values"].([]any)
	if len(values) != 2 {
		t.Fatalf("expected 2 value rows, got %d", len(values))
	}
}

func TestNormalizeHeatmap_AlreadyCanonical(t *testing.T) {
	data := map[string]any{
		"values": []any{
			[]any{1.0, 2.0},
			[]any{3.0, 4.0},
		},
		"row_labels": []any{"A", "B"},
		"col_labels": []any{"X", "Y"},
	}
	got := NormalizeDiagramData("heatmap", data)
	if _, ok := got["values"]; !ok {
		t.Error("values should remain")
	}
	if _, ok := got["row_labels"]; !ok {
		t.Error("row_labels should remain")
	}
}

func TestNormalizeVenn_SetsToCircles(t *testing.T) {
	data := map[string]any{
		"sets": []any{
			map[string]any{"label": "A", "items": []any{"x"}},
			map[string]any{"label": "B", "items": []any{"y"}},
		},
		"intersections": []any{
			map[string]any{
				"sets":  []any{"A", "B"},
				"items": []any{"shared"},
			},
		},
	}
	got := NormalizeDiagramData("venn", data)

	if _, ok := got["sets"]; ok {
		t.Error("'sets' should be removed")
	}
	circles, ok := got["circles"].([]any)
	if !ok {
		t.Fatal("should have 'circles'")
	}
	if len(circles) != 2 {
		t.Fatalf("expected 2 circles, got %d", len(circles))
	}

	inters, ok := got["intersections"].(map[string]any)
	if !ok {
		t.Fatal("intersections should be map[string]any")
	}
	ab, ok := inters["ab"].(map[string]any)
	if !ok {
		t.Fatal("should have 'ab' intersection")
	}
	items := ab["items"].([]any)
	if len(items) != 1 || items[0] != "shared" {
		t.Errorf("ab items = %v, want [shared]", items)
	}
}

func TestNormalizeVenn_ThreeCircles(t *testing.T) {
	data := map[string]any{
		"sets": []any{
			map[string]any{"label": "Technology"},
			map[string]any{"label": "Channel"},
			map[string]any{"label": "Product"},
		},
		"intersections": []any{
			map[string]any{"sets": []any{"Technology", "Channel"}, "items": []any{"MSPs"}},
			map[string]any{"sets": []any{"Technology", "Product"}, "items": []any{"Extensions"}},
			map[string]any{"sets": []any{"Channel", "Product"}, "items": []any{"Bundles"}},
			map[string]any{"sets": []any{"Technology", "Channel", "Product"}, "items": []any{"Strategic"}},
		},
	}
	got := NormalizeDiagramData("venn", data)
	inters := got["intersections"].(map[string]any)

	for _, key := range []string{"ab", "ac", "bc", "abc"} {
		if _, ok := inters[key]; !ok {
			t.Errorf("missing intersection key %q", key)
		}
	}
}

func TestNormalizeOrgChart_WrapInRoot(t *testing.T) {
	data := map[string]any{
		"name":  "CRO",
		"title": "Chief Revenue Officer",
		"children": []any{
			map[string]any{"name": "VP Sales", "title": "Sales"},
		},
	}
	got := NormalizeDiagramData("org_chart", data)

	root, ok := got["root"].(map[string]any)
	if !ok {
		t.Fatal("should have 'root' key")
	}
	if root["name"] != "CRO" {
		t.Errorf("root.name = %v, want CRO", root["name"])
	}
	if root["title"] != "Chief Revenue Officer" {
		t.Errorf("root.title = %v", root["title"])
	}
	children, ok := root["children"].([]any)
	if !ok || len(children) != 1 {
		t.Error("root should have 1 child")
	}

	// Top-level should NOT have org fields anymore
	if _, ok := got["name"]; ok {
		t.Error("top-level 'name' should be removed")
	}
}

func TestNormalizeOrgChart_AlreadyCanonical(t *testing.T) {
	data := map[string]any{
		"root": map[string]any{
			"name":     "CEO",
			"children": []any{},
		},
	}
	got := NormalizeDiagramData("org_chart", data)
	if _, ok := got["root"]; !ok {
		t.Error("root should remain")
	}
}

func TestTryParseMultiSeriesRadar(t *testing.T) {
	rawData := []interface{}{
		map[string]interface{}{
			"series": "Our Product",
			"values": []interface{}{
				map[string]interface{}{"category": "Performance", "value": 85},
				map[string]interface{}{"category": "Reliability", "value": 90},
				map[string]interface{}{"category": "Usability", "value": 75},
			},
		},
	}
	got := tryParseMultiSeriesChart(rawData)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	cats, ok := got["categories"].([]string)
	if !ok {
		t.Fatal("categories should be []string")
	}
	if !reflect.DeepEqual(cats, []string{"Performance", "Reliability", "Usability"}) {
		t.Errorf("categories = %v", cats)
	}
	series, ok := got["series"].([]map[string]any)
	if !ok || len(series) != 1 {
		t.Fatal("expected 1 series")
	}
	if series[0]["name"] != "Our Product" {
		t.Errorf("series name = %v", series[0]["name"])
	}
	vals := series[0]["values"].([]float64)
	if !reflect.DeepEqual(vals, []float64{85, 90, 75}) {
		t.Errorf("values = %v", vals)
	}
}

func TestNormalizeDiagramData_UnknownType(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	got := NormalizeDiagramData("timeline", data)
	if got["foo"] != "bar" {
		t.Error("unknown types should pass through unchanged")
	}
}

func TestBuildChartDataPayload_ScatterArrayOfPoints(t *testing.T) {
	// Simulates YAML-parsed scatter data as array of {label, x, y} objects.
	chartData := map[string]interface{}{
		"type":  "scatter",
		"title": "Price vs Volume",
		"data": []interface{}{
			map[string]interface{}{"label": "Product A", "x": 50.0, "y": 200.0},
			map[string]interface{}{"label": "Product B", "x": 80.0, "y": 150.0},
		},
	}
	payload := BuildChartDataPayload(chartData, "")

	series, ok := payload["series"].([]map[string]any)
	if !ok {
		t.Fatalf("payload[series] = %T, want []map[string]any", payload["series"])
	}
	if len(series) != 1 {
		t.Fatalf("len(series) = %d, want 1", len(series))
	}
	points, ok := series[0]["points"].([]map[string]any)
	if !ok {
		t.Fatalf("series[0][points] = %T, want []map[string]any", series[0]["points"])
	}
	if len(points) != 2 {
		t.Fatalf("len(points) = %d, want 2", len(points))
	}
	if points[0]["x"] != 50.0 || points[0]["y"] != 200.0 {
		t.Errorf("points[0] = %v, want x=50 y=200", points[0])
	}
	if points[0]["label"] != "Product A" {
		t.Errorf("points[0][label] = %v, want 'Product A'", points[0]["label"])
	}
}

func TestBuildChartDataPayload_ScatterMapFormat(t *testing.T) {
	// Existing map format should still work.
	chartData := map[string]interface{}{
		"type":  "scatter",
		"title": "Test",
		"data": map[string]interface{}{
			"A": 10.0,
			"B": 20.0,
		},
	}
	payload := BuildChartDataPayload(chartData, "")

	series, ok := payload["series"].([]map[string]any)
	if !ok {
		t.Fatalf("payload[series] = %T, want []map[string]any", payload["series"])
	}
	if len(series) != 1 {
		t.Fatalf("len(series) = %d, want 1", len(series))
	}
}

func TestBuildChartDataPayload_AxisTitlesCopied(t *testing.T) {
	// Axis title keys at the chart root level should be copied into the
	// data payload so that bar/line/area chart renderers can display them.
	tests := []struct {
		name      string
		chartType string
	}{
		{"bar", "bar"},
		{"line", "line"},
		{"area", "area"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartData := map[string]interface{}{
				"type":    tt.chartType,
				"title":   "Test",
				"y_label": "Revenue ($M)",
				"x_label": "Quarter",
				"data": []interface{}{
					map[string]interface{}{"label": "Q1", "value": 100.0},
					map[string]interface{}{"label": "Q2", "value": 200.0},
				},
			}
			payload := BuildChartDataPayload(chartData, "")

			if payload["y_label"] != "Revenue ($M)" {
				t.Errorf("y_label = %v, want %q", payload["y_label"], "Revenue ($M)")
			}
			if payload["x_label"] != "Quarter" {
				t.Errorf("x_label = %v, want %q", payload["x_label"], "Quarter")
			}
		})
	}
}
