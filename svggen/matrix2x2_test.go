package svggen

import (
	"fmt"
	"strings"
	"testing"
)

func TestMatrix2x2Chart_BasicRender(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Title: "Priority Matrix",
		Points: []Matrix2x2Point{
			{Label: "Project A", X: 20, Y: 80},
			{Label: "Project B", X: 80, Y: 70},
			{Label: "Project C", X: 30, Y: 30},
			{Label: "Project D", X: 70, Y: 20},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix 2x2 chart: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()
	if content == "" {
		t.Error("Expected non-empty SVG content")
	}

	if !strings.Contains(content, "svg") {
		t.Error("Expected SVG content to contain 'svg' tag")
	}
}

func TestMatrix2x2Chart_EmptyPoints(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Title:  "Empty Matrix",
		Points: []Matrix2x2Point{},
	}

	// Should render without error - empty matrix is valid
	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw empty matrix: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if svg.Width != 1067 || svg.Height != 800 {
		t.Errorf("Expected dimensions 1067x800 (800x600pt in CSS pixels), got %.0fx%.0f", svg.Width, svg.Height)
	}
}

func TestMatrix2x2Chart_CustomColors(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	config.QuadrantColors = [4]Color{
		MustParseColor("#FF0000").WithAlpha(0.2),
		MustParseColor("#00FF00").WithAlpha(0.2),
		MustParseColor("#0000FF").WithAlpha(0.2),
		MustParseColor("#FFFF00").WithAlpha(0.2),
	}

	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Item 1", X: 25, Y: 75},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with custom colors: %v", err)
	}
}

func TestMatrix2x2Chart_PointColors(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	customColor := MustParseColor("#FF00FF")

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Custom", X: 50, Y: 50, Color: &customColor},
			{Label: "Default", X: 30, Y: 70},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with point colors: %v", err)
	}
}

func TestMatrix2x2Chart_NoGridLines(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	config.ShowGridLines = false

	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "A", X: 50, Y: 50},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix without grid lines: %v", err)
	}
}

func TestMatrix2x2Chart_DashedGridLines(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	config.ShowGridLines = true
	config.GridLineDash = true

	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "A", X: 50, Y: 50},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with dashed grid: %v", err)
	}
}

func TestMatrix2x2Chart_CustomAxisLabels(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	config.XAxisLabel = "Risk"
	config.YAxisLabel = "Reward"

	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Title: "Risk vs Reward",
		Points: []Matrix2x2Point{
			{Label: "Low Risk High Reward", X: 20, Y: 80},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with custom axis labels: %v", err)
	}
}

func TestMatrix2x2Chart_CustomAxisRange(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	config.XAxisMin = -10
	config.XAxisMax = 10
	config.YAxisMin = -10
	config.YAxisMax = 10

	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Origin", X: 0, Y: 0},
			{Label: "Positive", X: 5, Y: 5},
			{Label: "Negative", X: -5, Y: -5},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with custom axis range: %v", err)
	}
}

func TestMatrix2x2Chart_PointShapes(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Circle", X: 20, Y: 80, Shape: MarkerCircle},
			{Label: "Square", X: 80, Y: 80, Shape: MarkerSquare},
			{Label: "Diamond", X: 20, Y: 20, Shape: MarkerDiamond},
			{Label: "Triangle", X: 80, Y: 20, Shape: MarkerTriangle},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with point shapes: %v", err)
	}
}

func TestMatrix2x2Chart_PointSizes(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Small", X: 25, Y: 75, Size: 8},
			{Label: "Medium", X: 50, Y: 50, Size: 16},
			{Label: "Large", X: 75, Y: 25, Size: 24},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with point sizes: %v", err)
	}
}

func TestMatrix2x2Chart_WithFootnote(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Title:    "Analysis Matrix",
		Subtitle: "Q4 2024",
		Points: []Matrix2x2Point{
			{Label: "Item A", X: 30, Y: 70},
		},
		Footnote: "Source: Internal analysis",
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with footnote: %v", err)
	}
}

func TestMatrix2x2Chart_NoLabels(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	config.ShowPointLabels = false

	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Should not show", X: 50, Y: 50},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix without labels: %v", err)
	}
}

func TestMatrix2x2Diagram_Validate(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name:    "nil data",
			req:     &RequestEnvelope{Type: "matrix_2x2", Data: nil},
			wantErr: true,
		},
		{
			name:    "empty data - valid",
			req:     &RequestEnvelope{Type: "matrix_2x2", Data: map[string]any{}},
			wantErr: false, // Empty matrix is valid
		},
		{
			name: "with points",
			req: &RequestEnvelope{
				Type: "matrix_2x2",
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "A", "x": 50.0, "y": 50.0},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagram.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMatrix2x2Diagram_Render(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type:  "matrix_2x2",
		Title: "Priority Matrix",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Task A", "x": 20.0, "y": 80.0},
				map[string]any{"label": "Task B", "x": 80.0, "y": 20.0},
			},
			"x_axis_label": "Effort",
			"y_axis_label": "Impact",
		},
		Output: OutputSpec{
			Width:  800,
			Height: 600,
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 diagram: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	if len(svg.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestMatrix2x2Diagram_RenderWithQuadrantLabels(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type: "matrix_2x2",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Item", "x": 50.0, "y": 50.0},
			},
			"quadrant_labels": []any{"Q1", "Q2", "Q3", "Q4"},
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 with quadrant labels: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}
}

func TestMatrix2x2Diagram_RenderWithAxisMapFormat(t *testing.T) {
	// Test the x_axis/y_axis map format used in golden tests
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type:  "matrix_2x2",
		Title: "Strategic Matrix",
		Data: map[string]any{
			"x_axis": map[string]any{
				"label":      "Impact",
				"low_label":  "Low",
				"high_label": "High",
			},
			"y_axis": map[string]any{
				"label":      "Effort",
				"low_label":  "Low",
				"high_label": "High",
			},
			"quadrants": []any{
				map[string]any{"label": "Quick Wins", "position": "top_left"},
				map[string]any{"label": "Major Projects", "position": "top_right"},
				map[string]any{"label": "Fill-Ins", "position": "bottom_left"},
				map[string]any{"label": "Thankless Tasks", "position": "bottom_right"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 with axis map format: %v", err)
	}

	content := svg.String()

	// Verify x_axis.label is used (drawn as single text)
	if !strings.Contains(content, "Impact") {
		t.Error("Expected SVG to contain x_axis label 'Impact'")
	}
	// Y-axis label is drawn with RotateAround(-90) + DrawText (rotated text).
	if !strings.Contains(content, "Effort") {
		t.Error("Expected SVG to contain y_axis label 'Effort'")
	}

	// Verify quadrant labels with underscore positions are recognized
	quadrantLabels := []string{"Quick Wins", "Major Projects", "Fill-Ins", "Thankless Tasks"}
	for _, label := range quadrantLabels {
		if !strings.Contains(content, label) {
			t.Errorf("Expected SVG to contain quadrant label '%s'", label)
		}
	}
}

func TestMatrix2x2Diagram_RenderWithAxisRange(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type: "matrix_2x2",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Center", "x": 0.0, "y": 0.0},
			},
			"x_min": -100.0,
			"x_max": 100.0,
			"y_min": -100.0,
			"y_max": 100.0,
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 with custom axis range: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}
}

func TestMatrix2x2Diagram_Type(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}
	if diagram.Type() != "matrix_2x2" {
		t.Errorf("Expected type 'matrix_2x2', got '%s'", diagram.Type())
	}
}

func TestDrawMatrix2x2FromData(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	points := []Matrix2x2Point{
		{Label: "A", X: 25, Y: 75},
		{Label: "B", X: 75, Y: 25},
	}

	err := DrawMatrix2x2FromData(builder, "Test Matrix", points)
	if err != nil {
		t.Fatalf("Failed to draw matrix from data: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if svg == nil || len(svg.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestCreateBCGMatrixConfig(t *testing.T) {
	config := CreateBCGMatrixConfig(800, 600)

	if config.XAxisLabel != "Relative Market Share" {
		t.Errorf("Expected 'Relative Market Share', got '%s'", config.XAxisLabel)
	}
	if config.YAxisLabel != "Market Growth Rate" {
		t.Errorf("Expected 'Market Growth Rate', got '%s'", config.YAxisLabel)
	}
	if config.QuadrantLabels[0] != "Stars" {
		t.Errorf("Expected 'Stars' in top-left, got '%s'", config.QuadrantLabels[0])
	}
	// BCG has reversed x-axis
	if config.XAxisMin != 100 || config.XAxisMax != 0 {
		t.Error("BCG matrix should have reversed x-axis")
	}
}

func TestCreateEisenhowerMatrixConfig(t *testing.T) {
	config := CreateEisenhowerMatrixConfig(800, 600)

	if config.XAxisLabel != "Urgency" {
		t.Errorf("Expected 'Urgency', got '%s'", config.XAxisLabel)
	}
	if config.YAxisLabel != "Importance" {
		t.Errorf("Expected 'Importance', got '%s'", config.YAxisLabel)
	}
	if config.QuadrantLabels[0] != "Do First" {
		t.Errorf("Expected 'Do First' in top-left, got '%s'", config.QuadrantLabels[0])
	}
}

func TestDefaultMatrix2x2Config(t *testing.T) {
	config := DefaultMatrix2x2Config(800, 600)

	if config.Width != 800 {
		t.Errorf("Expected width 800, got %v", config.Width)
	}
	if config.Height != 600 {
		t.Errorf("Expected height 600, got %v", config.Height)
	}
	if !config.ShowGridLines {
		t.Error("Expected ShowGridLines to be true by default")
	}
	if !config.ShowPointLabels {
		t.Error("Expected ShowPointLabels to be true by default")
	}
	if config.PointSize <= 0 {
		t.Error("Expected PointSize to be positive")
	}

	// Check quadrant labels exist
	for i, label := range config.QuadrantLabels {
		if label == "" {
			t.Errorf("Expected quadrant label %d to be non-empty", i)
		}
	}
}

func TestMatrix2x2Chart_SinglePoint(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Only Point", X: 50, Y: 50},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with single point: %v", err)
	}
}

func TestMatrix2x2Chart_ExtremePositions(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: "Top-Left", X: 0, Y: 100},
			{Label: "Top-Right", X: 100, Y: 100},
			{Label: "Bottom-Left", X: 0, Y: 0},
			{Label: "Bottom-Right", X: 100, Y: 0},
			{Label: "Center", X: 50, Y: 50},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with extreme positions: %v", err)
	}
}

func TestMatrix2x2Diagram_RenderWithQuadrantColors(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type:  "matrix_2x2",
		Title: "Custom Color Matrix",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Item", "x": 50.0, "y": 50.0},
			},
			"quadrant_colors": []any{"#FF0000", "#00FF00", "#0000FF", "#FFFF00"},
			"quadrant_opacity": 0.25,
		},
		Output: OutputSpec{
			Width:  800,
			Height: 600,
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 with custom colors: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	content := svg.String()

	// Verify that custom colors are applied (they should appear as fill colors with alpha)
	// The colors will be converted to rgba format
	if !strings.Contains(content, "rgba") {
		t.Error("Expected SVG to contain rgba colors for quadrant backgrounds")
	}
}

func TestMatrix2x2Diagram_RenderWithQuadrantColorsPartial(t *testing.T) {
	// Test that partial colors (fewer than 4) are applied correctly
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type: "matrix_2x2",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Test", "x": 25.0, "y": 75.0},
			},
			"quadrant_colors": []any{"#FF0000", "#00FF00"}, // Only 2 colors
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 with partial colors: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}
}

func TestMatrix2x2Diagram_RenderWithQuadrantsFormat(t *testing.T) {
	// Test the quadrants format (alternative to points)
	// This format is commonly used in markdown test content
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}

	req := &RequestEnvelope{
		Type:  "matrix_2x2",
		Title: "Effort vs Impact",
		Data: map[string]any{
			"x_label": "Effort",
			"y_label": "Impact",
			"quadrants": []any{
				map[string]any{
					"position": "top-left",
					"title":    "Quick Wins",
					"items":    []any{"Automation", "Documentation"},
				},
				map[string]any{
					"position": "top-right",
					"title":    "Major Projects",
					"items":    []any{"Platform rebuild", "Market expansion"},
				},
				map[string]any{
					"position": "bottom-left",
					"title":    "Fill-ins",
					"items":    []any{"Minor fixes"},
				},
				map[string]any{
					"position": "bottom-right",
					"title":    "Thankless Tasks",
					"items":    []any{"Legacy maintenance"},
				},
			},
		},
		Output: OutputSpec{
			Width:  800,
			Height: 600,
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render matrix_2x2 with quadrants format: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	content := svg.String()

	// Verify that item labels appear in the SVG
	expectedLabels := []string{
		"Automation", "Documentation",
		"Platform rebuild", "Market expansion",
		"Minor fixes", "Legacy maintenance",
	}

	for _, label := range expectedLabels {
		if !strings.Contains(content, label) {
			t.Errorf("Expected SVG to contain label '%s'", label)
		}
	}

	// Verify that x_label and y_label are used for axis labels
	if !strings.Contains(content, "Effort") {
		t.Error("Expected SVG to contain x_label 'Effort'")
	}
	if !strings.Contains(content, "Impact") {
		t.Error("Expected SVG to contain y_label 'Impact'")
	}

	// Verify that quadrant titles are rendered
	quadrantTitles := []string{"Quick Wins", "Major Projects", "Fill-ins", "Thankless Tasks"}
	for _, title := range quadrantTitles {
		if !strings.Contains(content, title) {
			t.Errorf("Expected SVG to contain quadrant title '%s'", title)
		}
	}
}

func TestParseQuadrantItems(t *testing.T) {
	// Test the parseQuadrantItems helper function
	quadrants := []any{
		map[string]any{
			"position": "top-left",
			"title":    "Quick Wins",
			"items":    []any{"Item A", "Item B"},
		},
		map[string]any{
			"position": "bottom-right",
			"title":    "Low Priority",
			"items":    []any{"Item C"},
		},
	}

	points := parseQuadrantItems(quadrants)

	if len(points) != 3 {
		t.Errorf("Expected 3 points, got %d", len(points))
	}

	// Check that items are placed in correct quadrants
	// top-left should have high Y (around 75)
	// bottom-right should have low Y (around 25)
	foundTopLeft := false
	foundBottomRight := false

	for _, p := range points {
		if p.Label == "Item A" || p.Label == "Item B" {
			if p.Y > 50 && p.X < 50 {
				foundTopLeft = true
			}
		}
		if p.Label == "Item C" {
			if p.Y < 50 && p.X > 50 {
				foundBottomRight = true
			}
		}
	}

	if !foundTopLeft {
		t.Error("Expected top-left items to be positioned correctly (high Y, low X)")
	}
	if !foundBottomRight {
		t.Error("Expected bottom-right item to be positioned correctly (low Y, high X)")
	}
}

func TestParseQuadrantItems_InvalidPositions(t *testing.T) {
	// Test handling of invalid position values
	quadrants := []any{
		map[string]any{
			"position": "invalid",
			"items":    []any{"Should Skip"},
		},
		map[string]any{
			"position": "top-left",
			"items":    []any{"Valid Item"},
		},
	}

	points := parseQuadrantItems(quadrants)

	// Only the valid quadrant should be parsed
	if len(points) != 1 {
		t.Errorf("Expected 1 point (from valid quadrant), got %d", len(points))
	}

	if len(points) > 0 && points[0].Label != "Valid Item" {
		t.Errorf("Expected label 'Valid Item', got '%s'", points[0].Label)
	}
}

func TestParseQuadrantItems_EmptyItems(t *testing.T) {
	// Test handling of quadrants with no items
	quadrants := []any{
		map[string]any{
			"position": "top-left",
			"items":    []any{},
		},
		map[string]any{
			"position": "bottom-right",
			// No items key at all
		},
	}

	points := parseQuadrantItems(quadrants)

	// No points should be created from empty quadrants
	if len(points) != 0 {
		t.Errorf("Expected 0 points from empty quadrants, got %d", len(points))
	}
}

func TestMatrix2x2Chart_DenseData_NoLabelCollision(t *testing.T) {
	// Regression test: with many densely packed points, labels should not overlap.
	// The fix introduces adaptive font sizing, label truncation, and multi-direction
	// placement to prevent illegible overlapping labels.
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	// 12 points clustered in a small region — worst case for label collision.
	data := Matrix2x2Data{
		Title: "Dense Cluster Test",
		Points: []Matrix2x2Point{
			{Label: "Alpha", X: 48, Y: 52},
			{Label: "Beta", X: 50, Y: 50},
			{Label: "Gamma", X: 52, Y: 48},
			{Label: "Delta", X: 49, Y: 51},
			{Label: "Epsilon", X: 51, Y: 49},
			{Label: "Zeta", X: 47, Y: 53},
			{Label: "Eta", X: 53, Y: 47},
			{Label: "Theta", X: 50, Y: 52},
			{Label: "Iota", X: 50, Y: 48},
			{Label: "Kappa", X: 48, Y: 50},
			{Label: "Lambda", X: 52, Y: 50},
			{Label: "Mu", X: 50, Y: 51},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw dense matrix: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()

	// All labels should appear in the SVG output.
	for _, p := range data.Points {
		if !strings.Contains(content, p.Label) {
			t.Errorf("Expected SVG to contain label %q", p.Label)
		}
	}
}

func TestMatrix2x2Chart_LongLabelsTruncated(t *testing.T) {
	// Long labels should be truncated with an ellipsis to prevent them from
	// dominating the chart and colliding excessively.
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	longLabel := "This is an extremely long label that should be truncated"

	data := Matrix2x2Data{
		Points: []Matrix2x2Point{
			{Label: longLabel, X: 25, Y: 75},
			{Label: "Short", X: 75, Y: 25},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw matrix with long labels: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()

	// The full long label should NOT appear (it's truncated).
	if strings.Contains(content, longLabel) {
		t.Error("Expected long label to be truncated, but full label appeared in SVG")
	}

	// A truncated version with ellipsis should appear.
	if !strings.Contains(content, "…") {
		t.Error("Expected truncated label with ellipsis (…) in SVG")
	}

	// Short label should appear unchanged.
	if !strings.Contains(content, "Short") {
		t.Error("Expected short label to appear unchanged")
	}
}

func TestMatrix2x2Chart_NarrowCanvas_NoLabelTruncation(t *testing.T) {
	// Regression test for go-slide-creator-gy1j3:
	// In a narrow two-column layout (e.g. 400px wide), labels like
	// "Enterprise Expansion" placed in the right quadrant were truncated
	// because the direction selection only checked collisions, not SVG bounds.
	// With the fix, the algorithm prefers directions where the full label fits.
	builder := NewSVGBuilder(400, 300)

	config := DefaultMatrix2x2Config(400, 300)
	config.ShowPointLabels = true
	config.ShowGridLines = true
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Title: "Portfolio Prioritization",
		Points: []Matrix2x2Point{
			{Label: "Enterprise Expansion", X: 75, Y: 75},
			{Label: "Process Automation", X: 75, Y: 25},
			{Label: "Core Platform v2", X: 25, Y: 75},
			{Label: "Self-Service Portal", X: 25, Y: 25},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw narrow matrix: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()

	// All labels must appear untruncated (no "…" or missing suffix)
	for _, p := range data.Points {
		if !strings.Contains(content, p.Label) {
			t.Errorf("Expected SVG to contain full label %q (label was likely truncated)", p.Label)
		}
	}
}

func TestMatrix2x2Chart_VeryNarrowCanvas_LabelsWrapped(t *testing.T) {
	// Regression test for go-slide-creator-zci7z:
	// In a very narrow two-column right slot (~300px), labels like
	// "Enterprise Expansion" and "Self-Service Portal" were truncated with
	// ellipsis. With the fix, labels wrap to multiple lines instead of
	// being truncated.
	builder := NewSVGBuilder(300, 250)

	config := DefaultMatrix2x2Config(300, 250)
	config.ShowPointLabels = true
	config.ShowGridLines = true
	chart := NewMatrix2x2Chart(builder, config)

	data := Matrix2x2Data{
		Title: "Portfolio Prioritization",
		Points: []Matrix2x2Point{
			{Label: "Enterprise Expansion", X: 75, Y: 75},
			{Label: "Process Automation", X: 75, Y: 25},
			{Label: "Self-Service Portal", X: 25, Y: 75},
			{Label: "Core Platform v2", X: 25, Y: 25},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw very narrow matrix: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()

	// Labels must appear without truncation ellipsis
	for _, p := range data.Points {
		if !strings.Contains(content, p.Label) {
			t.Errorf("Expected SVG to contain full label %q (label was likely truncated)", p.Label)
		}
	}
	// No ellipsis should appear for these reasonably-sized labels
	if strings.Contains(content, "…") {
		t.Errorf("Labels should wrap, not be truncated with ellipsis")
	}
}

func TestMatrix2x2Chart_ManyPointsAdaptiveFontSize(t *testing.T) {
	// With 15+ points, the label font size should be reduced to SizeCaption.
	// This test verifies that many points renders without error and all labels
	// appear (the font size reduction is an internal rendering detail).
	builder := NewSVGBuilder(800, 600)

	config := DefaultMatrix2x2Config(800, 600)
	chart := NewMatrix2x2Chart(builder, config)

	points := make([]Matrix2x2Point, 20)
	for i := range points {
		points[i] = Matrix2x2Point{
			Label: fmt.Sprintf("P%d", i+1),
			X:     float64(5 + (i*90)/19), // spread across 5-95
			Y:     float64(5 + ((i*3)%19)*5),
		}
	}

	data := Matrix2x2Data{
		Title:  "20-Point Matrix",
		Points: points,
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw 20-point matrix: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()

	// All 20 labels should appear.
	for i := 1; i <= 20; i++ {
		label := fmt.Sprintf("P%d", i)
		if !strings.Contains(content, label) {
			t.Errorf("Expected SVG to contain label %q", label)
		}
	}
}
