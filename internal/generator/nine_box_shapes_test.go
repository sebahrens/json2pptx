package generator

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestIsNineBoxDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "nine_box_talent type",
			spec:     &types.DiagramSpec{Type: "nine_box_talent"},
			expected: true,
		},
		{
			name:     "swot type",
			spec:     &types.DiagramSpec{Type: "swot"},
			expected: false,
		},
		{
			name:     "panel_layout type",
			spec:     &types.DiagramSpec{Type: "panel_layout"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNineBoxDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isNineBoxDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseNineBoxCells_CellsFormat(t *testing.T) {
	data := map[string]any{
		"cells": []any{
			map[string]any{
				"position": map[string]any{"row": float64(0), "col": float64(2)},
				"label":    "Stars",
				"items": []any{
					map[string]any{"name": "Alice"},
					map[string]any{"name": "Bob"},
				},
			},
			map[string]any{
				"position": map[string]any{"row": float64(1), "col": float64(1)},
				"label":    "Core",
				"items":    []any{"Charlie"},
			},
		},
	}

	cells := parseNineBoxCells(data)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}

	// First cell: Stars at (0,2)
	if cells[0].row != 0 || cells[0].col != 2 {
		t.Errorf("cell 0: expected (0,2), got (%d,%d)", cells[0].row, cells[0].col)
	}
	if cells[0].label != "Stars" {
		t.Errorf("cell 0: expected label 'Stars', got %q", cells[0].label)
	}
	if len(cells[0].items) != 2 {
		t.Errorf("cell 0: expected 2 items, got %d", len(cells[0].items))
	}

	// Second cell: Core at (1,1) with string item
	if cells[1].row != 1 || cells[1].col != 1 {
		t.Errorf("cell 1: expected (1,1), got (%d,%d)", cells[1].row, cells[1].col)
	}
	if len(cells[1].items) != 1 || cells[1].items[0] != "Charlie" {
		t.Errorf("cell 1: expected [Charlie], got %v", cells[1].items)
	}
}

func TestParseNineBoxCells_EmployeesFormat(t *testing.T) {
	data := map[string]any{
		"employees": []any{
			map[string]any{"name": "Alice", "performance": "high", "potential": "high"},
			map[string]any{"name": "Bob", "performance": "high", "potential": "high"},
			map[string]any{"name": "Carol", "performance": "low", "potential": "low"},
		},
	}

	cells := parseNineBoxCells(data)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells (grouped), got %d", len(cells))
	}

	// Find the high/high cell (row=0, col=2)
	found := false
	for _, c := range cells {
		if c.row == 0 && c.col == 2 {
			found = true
			if len(c.items) != 2 {
				t.Errorf("high/high cell: expected 2 items, got %d", len(c.items))
			}
		}
	}
	if !found {
		t.Error("did not find high/high cell at (0,2)")
	}
}

func TestEmployeeToGridPos(t *testing.T) {
	tests := []struct {
		performance, potential string
		wantRow, wantCol       int
	}{
		{"high", "high", 0, 2},
		{"low", "low", 2, 0},
		{"medium", "medium", 1, 1},
		{"high", "low", 2, 2},
		{"low", "high", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.performance+"_"+tt.potential, func(t *testing.T) {
			row, col := employeeToGridPos(tt.performance, tt.potential)
			if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("got (%d,%d), want (%d,%d)", row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestEncodeDecodeNineBoxAxes(t *testing.T) {
	xTitle := "Performance"
	yTitle := "Potential"
	xLabels := [3]string{"Low", "Medium", "High"}
	yLabels := [3]string{"Limited", "Moderate", "High"}

	encoded := encodeNineBoxAxes(xTitle, yTitle, xLabels, yLabels)
	gotXT, gotYT, gotXL, gotYL := decodeNineBoxAxes(encoded)

	if gotXT != xTitle {
		t.Errorf("xTitle: got %q, want %q", gotXT, xTitle)
	}
	if gotYT != yTitle {
		t.Errorf("yTitle: got %q, want %q", gotYT, yTitle)
	}
	if gotXL != xLabels {
		t.Errorf("xLabels: got %v, want %v", gotXL, xLabels)
	}
	if gotYL != yLabels {
		t.Errorf("yLabels: got %v, want %v", gotYL, yLabels)
	}
}

func TestGenerateNineBoxGroupXML_Basic(t *testing.T) {
	// Build the 10-panel input (1 axis + 9 cells).
	panels := make([]nativePanelData, 10)
	panels[0] = nativePanelData{
		title: "__nine_box_axes__",
		body:  encodeNineBoxAxes("Performance", "Potential", [3]string{"Low", "Medium", "High"}, [3]string{"Limited", "Moderate", "High"}),
	}
	for i := 1; i <= 9; i++ {
		row := (i - 1) / 3
		col := (i - 1) % 3
		panels[i] = nativePanelData{
			title: nineBoxDefaultLabels[row][col],
			body:  "",
		}
	}
	// Add some items to the Star cell (row=0, col=2 -> index 3).
	panels[3] = nativePanelData{
		title: "Star",
		body:  "- Alice\n- Bob",
	}

	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 10000000, Height: 6000000}
	result := generateNineBoxGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("generateNineBoxGroupXML returned empty string")
	}

	// Should be well-formed XML.
	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}

	// Should contain group element.
	if !strings.Contains(result, "p:grpSp") {
		t.Error("should contain p:grpSp group element")
	}

	// Should contain group name.
	if !strings.Contains(result, "Nine Box Talent") {
		t.Error("should contain 'Nine Box Talent' group name")
	}

	// Should contain all 9 default cell labels.
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			label := nineBoxDefaultLabels[row][col]
			if !strings.Contains(result, label) {
				t.Errorf("should contain cell label %q", label)
			}
		}
	}

	// Should contain axis titles.
	if !strings.Contains(result, "Performance") {
		t.Error("should contain x-axis title 'Performance'")
	}
	if !strings.Contains(result, "Potential") {
		t.Error("should contain y-axis title 'Potential'")
	}

	// Should contain axis value labels.
	for _, label := range []string{"Low", "Medium", "High"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain axis label %q", label)
		}
	}

	// Should contain item names.
	if !strings.Contains(result, "Alice") {
		t.Error("should contain item name 'Alice'")
	}
	if !strings.Contains(result, "Bob") {
		t.Error("should contain item name 'Bob'")
	}

	// Should contain scheme color references (theme-aware).
	if !strings.Contains(result, "schemeClr") {
		t.Error("should use scheme colors for theme awareness")
	}

	// Should contain roundRect geometry.
	if !strings.Contains(result, "roundRect") {
		t.Error("should use roundRect preset geometry")
	}
}

func TestGenerateNineBoxGroupXML_WrongPanelCount(t *testing.T) {
	panels := []nativePanelData{{title: "only one"}}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateNineBoxGroupXML(panels, bounds, 100)
	if result != "" {
		t.Error("expected empty string for wrong panel count")
	}
}

func TestParseNineBoxAxisLabels(t *testing.T) {
	data := map[string]any{
		"x_axis_labels": []any{"Developing", "Effective", "Outstanding"},
	}

	labels := parseNineBoxAxisLabels(data, "x_axis_labels")
	if labels != [3]string{"Developing", "Effective", "Outstanding"} {
		t.Errorf("unexpected labels: %v", labels)
	}

	// Missing key returns empty.
	empty := parseNineBoxAxisLabels(data, "y_axis_labels")
	if empty != [3]string{} {
		t.Errorf("expected empty labels for missing key, got %v", empty)
	}
}

func TestGenerateNineBoxGroupXML_NoAxes(t *testing.T) {
	// Build panels with empty axis data.
	panels := make([]nativePanelData, 10)
	panels[0] = nativePanelData{
		title: "__nine_box_axes__",
		body:  encodeNineBoxAxes("", "", [3]string{}, [3]string{}),
	}
	for i := 1; i <= 9; i++ {
		panels[i] = nativePanelData{title: "Cell", body: ""}
	}

	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}
	result := generateNineBoxGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should produce valid XML even without axis labels")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}
