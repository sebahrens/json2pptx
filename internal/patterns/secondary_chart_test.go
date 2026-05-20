package patterns

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateSecondaryChart(t *testing.T) {
	tests := []struct {
		name    string
		sec     *SecondaryChart
		wantErr string // substring expected in joined error; empty = expect no error
	}{
		{
			name:    "nil_no_errors",
			sec:     nil,
			wantErr: "",
		},
		{
			name:    "valid_sparkline",
			sec:     &SecondaryChart{Type: "sparkline", Values: []float64{1, 2, 3}},
			wantErr: "",
		},
		{
			name:    "valid_bar_chart",
			sec:     &SecondaryChart{Type: "bar_chart", Values: []float64{10, 20}},
			wantErr: "",
		},
		{
			name:    "valid_line_chart_with_categories",
			sec:     &SecondaryChart{Type: "line_chart", Values: []float64{1, 2}, Categories: []string{"Q1", "Q2"}},
			wantErr: "",
		},
		{
			name:    "invalid_type",
			sec:     &SecondaryChart{Type: "pie", Values: []float64{1, 2, 3}},
			wantErr: "must be one of sparkline, bar_chart, line_chart",
		},
		{
			name:    "missing_type",
			sec:     &SecondaryChart{Values: []float64{1, 2, 3}},
			wantErr: "type",
		},
		{
			name:    "too_few_values",
			sec:     &SecondaryChart{Type: "sparkline", Values: []float64{1}},
			wantErr: "values",
		},
		{
			name:    "too_many_values",
			sec:     &SecondaryChart{Type: "sparkline", Values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}},
			wantErr: "values",
		},
		{
			name:    "categories_length_mismatch",
			sec:     &SecondaryChart{Type: "bar_chart", Values: []float64{1, 2, 3}, Categories: []string{"A", "B"}},
			wantErr: "categories length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateSecondaryChart("test-pattern", "secondary", tc.sec)
			joined := errors.Join(errs...)
			if tc.wantErr == "" {
				if joined != nil {
					t.Fatalf("expected no error, got: %v", joined)
				}
				return
			}
			if joined == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(joined.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", joined.Error(), tc.wantErr)
			}
		})
	}
}

func TestBuildSecondaryDiagram_SparklineMapsToLineChart(t *testing.T) {
	sec := &SecondaryChart{Type: "sparkline", Values: []float64{1.0, 2.0, 3.0}}
	ds := buildSecondaryDiagram(sec, "accent1")
	if ds == nil {
		t.Fatal("expected non-nil DiagramSpec")
	}
	if ds.Type != "line_chart" {
		t.Errorf("sparkline should map to line_chart; got %q", ds.Type)
	}
	if ds.Style == nil {
		t.Fatal("expected non-nil Style for sparkline (legend hidden)")
	}
	if ds.Style.ShowLegend {
		t.Error("sparkline should hide the legend")
	}
	// Should synthesize categories matching values length.
	cats, ok := ds.Data["categories"].([]any)
	if !ok {
		t.Fatalf("categories not []any; got %T", ds.Data["categories"])
	}
	if len(cats) != 3 {
		t.Errorf("synthesized categories length = %d, want 3", len(cats))
	}
}

func TestBuildSecondaryDiagram_PassthroughTypes(t *testing.T) {
	for _, typ := range []string{"bar_chart", "line_chart"} {
		t.Run(typ, func(t *testing.T) {
			sec := &SecondaryChart{Type: typ, Values: []float64{1, 2, 3}}
			ds := buildSecondaryDiagram(sec, "accent1")
			if ds == nil {
				t.Fatal("expected non-nil DiagramSpec")
			}
			if ds.Type != typ {
				t.Errorf("expected %q passthrough; got %q", typ, ds.Type)
			}
		})
	}
}

func TestBuildSecondaryDiagram_NilReturnsNil(t *testing.T) {
	if got := buildSecondaryDiagram(nil, "accent1"); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestCardGridCellSecondaryUnmarshal(t *testing.T) {
	input := `{
		"header": "Revenue",
		"body": "Q1 trend",
		"secondary": {
			"type": "sparkline",
			"values": [100, 120, 110, 145, 160]
		}
	}`
	var cell CardGridCell
	if err := json.Unmarshal([]byte(input), &cell); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cell.Secondary == nil {
		t.Fatal("expected non-nil Secondary")
	}
	if cell.Secondary.Type != "sparkline" {
		t.Errorf("type = %q, want sparkline", cell.Secondary.Type)
	}
	if len(cell.Secondary.Values) != 5 {
		t.Errorf("values length = %d, want 5", len(cell.Secondary.Values))
	}
}

func TestCardGridExpandEmitsCompositeWhenSecondarySet(t *testing.T) {
	vals := &CardGridValues{
		Columns: 2,
		Rows:    1,
		Cells: []CardGridCell{
			{Header: "A", Body: "no chart"},
			{Header: "B", Body: "has chart", Secondary: &SecondaryChart{Type: "sparkline", Values: []float64{1, 2, 3}}},
		},
	}
	cg := &cardGrid{}
	grid, err := cg.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != 2 {
		t.Fatalf("expected 1 row of 2 cells, got %d rows", len(grid.Rows))
	}
	noChart := grid.Rows[0].Cells[0]
	withChart := grid.Rows[0].Cells[1]

	if noChart.Shape == nil {
		t.Error("cell without secondary should have Shape set")
	}
	if noChart.Composite != nil {
		t.Error("cell without secondary should NOT have Composite set")
	}

	if withChart.Composite == nil {
		t.Fatal("cell with secondary should have Composite set")
	}
	if withChart.Shape != nil {
		t.Error("cell with secondary should have Shape moved into Composite.Text (top-level Shape must be nil)")
	}
	if withChart.Composite.Text == nil {
		t.Error("Composite.Text should be the original shape")
	}
	if withChart.Composite.SubDiagram == nil {
		t.Fatal("Composite.SubDiagram should be the chart")
	}
	if withChart.Composite.SubDiagram.Type != "line_chart" {
		t.Errorf("sparkline should expand to line_chart sub-diagram; got %q", withChart.Composite.SubDiagram.Type)
	}
}

func TestCardGridValidateRejectsBadSecondary(t *testing.T) {
	vals := &CardGridValues{
		Columns: 1,
		Rows:    1,
		Cells: []CardGridCell{
			{Header: "X", Body: "Y", Secondary: &SecondaryChart{Type: "pie", Values: []float64{1, 2}}},
		},
	}
	cg := &cardGrid{}
	err := cg.Validate(vals, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for invalid secondary.type")
	}
	if !strings.Contains(err.Error(), "cells[0].secondary.type") {
		t.Errorf("error should reference cells[0].secondary.type; got %v", err)
	}
}

func TestIconRowExpandEmitsCompositeWhenSecondarySet(t *testing.T) {
	items := IconRowValues{
		{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
		{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth", Secondary: &SecondaryChart{Type: "bar_chart", Values: []float64{10, 20, 30}}},
		{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
	}
	ir := &iconRow{}
	grid, err := ir.Expand(ExpandContext{}, &items, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != 3 {
		t.Fatalf("expected 1 row of 3 cells")
	}
	if grid.Rows[0].Cells[0].Composite != nil {
		t.Error("first item without secondary should not have Composite")
	}
	mid := grid.Rows[0].Cells[1]
	if mid.Composite == nil {
		t.Fatal("middle item with secondary should have Composite")
	}
	if mid.Composite.SubDiagram == nil || mid.Composite.SubDiagram.Type != "bar_chart" {
		t.Errorf("expected bar_chart sub-diagram; got %+v", mid.Composite.SubDiagram)
	}
}

func TestIconRowValidateRejectsTooManyValues(t *testing.T) {
	items := IconRowValues{
		{Icon: &IconRef{Name: "a"}, Caption: "A", Secondary: &SecondaryChart{Type: "sparkline", Values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}}},
		{Icon: &IconRef{Name: "b"}, Caption: "B"},
		{Icon: &IconRef{Name: "c"}, Caption: "C"},
	}
	ir := &iconRow{}
	err := ir.Validate(&items, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for too many values")
	}
	if !strings.Contains(err.Error(), "values[0].secondary.values") {
		t.Errorf("error should reference values[0].secondary.values; got %v", err)
	}
}
