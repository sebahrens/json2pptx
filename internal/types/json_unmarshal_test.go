package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestChartSpecJSONUnmarshal_FlatData(t *testing.T) {
	// Backward compatibility: flat float64 data should still unmarshal fine
	input := `{"type":"bar","title":"Sales","data":{"Q1":100,"Q2":150,"Q3":120}}`
	var cs ChartSpec
	if err := json.Unmarshal([]byte(input), &cs); err != nil {
		t.Fatalf("unmarshal flat data: %v", err)
	}
	if cs.Type != ChartBar {
		t.Errorf("Type = %q, want %q", cs.Type, ChartBar)
	}
	if len(cs.Data) != 3 {
		t.Errorf("len(Data) = %d, want 3", len(cs.Data))
	}
	// JSON numbers unmarshal as float64
	if v, ok := cs.Data["Q1"].(float64); !ok || v != 100 {
		t.Errorf("Data[Q1] = %v (%T), want 100 (float64)", cs.Data["Q1"], cs.Data["Q1"])
	}

	// Verify conversion to DiagramSpec works
	ds := cs.ToDiagramSpec()
	if ds == nil {
		t.Fatal("ToDiagramSpec returned nil")
	}
}

func TestChartSpecJSONUnmarshal_StructuredArrayData(t *testing.T) {
	// THIS WAS THE CRASH: stacked_bar with array values in data
	input := `{"type":"stacked_bar","title":"Stacked","data":{"Q1":[10,20,30],"Q2":[15,25,35]},"data_order":["Q1","Q2"],"series_labels":["A","B","C"]}`
	var cs ChartSpec
	if err := json.Unmarshal([]byte(input), &cs); err != nil {
		t.Fatalf("unmarshal structured data (this was the crash): %v", err)
	}
	if cs.Type != ChartStackedBar {
		t.Errorf("Type = %q, want %q", cs.Type, ChartStackedBar)
	}
	if len(cs.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2", len(cs.Data))
	}

	// Data["Q1"] should be a []any
	q1, ok := cs.Data["Q1"].([]any)
	if !ok {
		t.Fatalf("Data[Q1] = %T, want []any", cs.Data["Q1"])
	}
	if len(q1) != 3 {
		t.Fatalf("len(Data[Q1]) = %d, want 3", len(q1))
	}

	// Verify conversion to DiagramSpec works
	ds := cs.ToDiagramSpec()
	if ds == nil {
		t.Fatal("ToDiagramSpec returned nil")
	}

	// Should have 3 series
	series, ok := ds.Data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", ds.Data["series"])
	}
	if len(series) != 3 {
		t.Errorf("len(series) = %d, want 3", len(series))
	}
}

func TestChartSpecJSONUnmarshal_ScatterWithPoints(t *testing.T) {
	input := `{"type":"scatter","title":"Scatter","data":{"Group A":[{"x":1,"y":2},{"x":3,"y":4}]},"data_order":["Group A"]}`
	var cs ChartSpec
	if err := json.Unmarshal([]byte(input), &cs); err != nil {
		t.Fatalf("unmarshal scatter data: %v", err)
	}

	ds := cs.ToDiagramSpec()
	if ds == nil {
		t.Fatal("ToDiagramSpec returned nil")
	}

	series, ok := ds.Data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("data[series] = %T, want []map[string]any", ds.Data["series"])
	}
	if len(series) != 1 {
		t.Fatalf("len(series) = %d, want 1", len(series))
	}
	points, ok := series[0]["points"].([]map[string]any)
	if !ok {
		t.Fatalf("series[0][points] = %T, want []map[string]any", series[0]["points"])
	}
	if len(points) != 2 {
		t.Errorf("len(points) = %d, want 2", len(points))
	}
}

func TestChartSpecJSONUnmarshal_ScatterArrayOfPoints(t *testing.T) {
	// This is the format documented in CHART_REFERENCE.md:
	// data is an array of {label, x, y} objects, not a map.
	input := `{"type":"scatter","title":"Price vs Volume","data":[{"label":"Product A","x":50,"y":200},{"label":"Product B","x":80,"y":150}]}`
	var cs ChartSpec
	if err := json.Unmarshal([]byte(input), &cs); err != nil {
		t.Fatalf("unmarshal scatter array data: %v", err)
	}
	if cs.Type != ChartScatter {
		t.Errorf("Type = %q, want %q", cs.Type, ChartScatter)
	}

	// Data should have been converted to series format
	series, ok := cs.Data["series"]
	if !ok {
		t.Fatal("Data should contain 'series' key after array-to-map conversion")
	}
	seriesSlice, ok := series.([]map[string]any)
	if !ok {
		t.Fatalf("series = %T, want []map[string]any", series)
	}
	if len(seriesSlice) != 1 {
		t.Fatalf("len(series) = %d, want 1", len(seriesSlice))
	}
	points, ok := seriesSlice[0]["points"].([]map[string]any)
	if !ok {
		t.Fatalf("series[0][points] = %T, want []map[string]any", seriesSlice[0]["points"])
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

	// Verify full pipeline: ToDiagramSpec → extractScatterChartData should work
	ds := cs.ToDiagramSpec()
	if ds == nil {
		t.Fatal("ToDiagramSpec returned nil")
	}
	dsSeries, ok := ds.Data["series"].([]map[string]any)
	if !ok {
		t.Fatalf("DiagramSpec data[series] = %T, want []map[string]any", ds.Data["series"])
	}
	if len(dsSeries) != 1 {
		t.Fatalf("DiagramSpec series count = %d, want 1", len(dsSeries))
	}
}

func TestChartSpecJSONUnmarshal_BubbleWithPoints(t *testing.T) {
	input := `{"type":"bubble","title":"Bubble","data":{"Series":[{"x":1,"y":2,"size":10}]},"data_order":["Series"]}`
	var cs ChartSpec
	if err := json.Unmarshal([]byte(input), &cs); err != nil {
		t.Fatalf("unmarshal bubble data: %v", err)
	}

	ds := cs.ToDiagramSpec()
	if ds == nil {
		t.Fatal("ToDiagramSpec returned nil")
	}
}

func TestChartSpecJSONUnmarshal_PreservesKeyOrder(t *testing.T) {
	// JSON key order must be preserved for line/area charts so that
	// chronological categories (months, quarters) render in the correct order.
	tests := []struct {
		name      string
		input     string
		wantOrder []string
	}{
		{
			name:      "chronological months",
			input:     `{"type":"line","title":"Monthly","data":{"Jan":10,"Feb":20,"Mar":30,"Apr":40}}`,
			wantOrder: []string{"Jan", "Feb", "Mar", "Apr"},
		},
		{
			name:      "chronological quarters",
			input:     `{"type":"area","title":"Quarterly","data":{"Q1 2024":100,"Q2 2024":150,"Q3 2024":120,"Q4 2024":180,"Q1 2025":200,"Q2 2025":220}}`,
			wantOrder: []string{"Q1 2024", "Q2 2024", "Q3 2024", "Q4 2024", "Q1 2025", "Q2 2025"},
		},
		{
			name:      "bar chart order preserved",
			input:     `{"type":"bar","title":"Custom","data":{"Zebra":1,"Apple":2,"Mango":3}}`,
			wantOrder: []string{"Zebra", "Apple", "Mango"},
		},
		{
			name:      "explicit data_order takes precedence",
			input:     `{"type":"line","title":"Explicit","data":{"B":2,"A":1,"C":3},"data_order":["C","B","A"]}`,
			wantOrder: []string{"C", "B", "A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cs ChartSpec
			if err := json.Unmarshal([]byte(tt.input), &cs); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(cs.DataOrder) != len(tt.wantOrder) {
				t.Fatalf("DataOrder length = %d, want %d; got %v", len(cs.DataOrder), len(tt.wantOrder), cs.DataOrder)
			}
			for i, want := range tt.wantOrder {
				if cs.DataOrder[i] != want {
					t.Errorf("DataOrder[%d] = %q, want %q (full: %v)", i, cs.DataOrder[i], want, cs.DataOrder)
				}
			}

			// Verify categories come out in the right order via ToDiagramSpec
			ds := cs.ToDiagramSpec()
			if ds == nil {
				t.Fatal("ToDiagramSpec returned nil")
			}
			if cats, ok := ds.Data["categories"]; ok {
				catSlice, ok := cats.([]string)
				if !ok {
					// May be []any from structured path
					if anySlice, ok2 := cats.([]any); ok2 {
						catSlice = make([]string, len(anySlice))
						for i, v := range anySlice {
							catSlice[i] = fmt.Sprintf("%v", v)
						}
					}
				}
				if len(catSlice) > 0 {
					got := strings.Join(catSlice, ",")
					want := strings.Join(tt.wantOrder, ",")
					if got != want {
						t.Errorf("categories order = %q, want %q", got, want)
					}
				}
			}
		})
	}
}
