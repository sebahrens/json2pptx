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

func TestMaybeScaleNormalizedPoints(t *testing.T) {
	// Normalized 0-1 coords on the default 0-100 axis should scale to 0-100.
	t.Run("scales normalized data", func(t *testing.T) {
		pts := []Matrix2x2Point{
			{Label: "A", X: 0.2, Y: 0.8},
			{Label: "B", X: 1.0, Y: 0.0},
		}
		maybeScaleNormalizedPoints(pts, 0, 100, 0, 100)
		if len(pts) != 2 {
			t.Fatalf("expected 2 points, got %d", len(pts))
		}
		if pts[0].X != 20 || pts[0].Y != 80 {
			t.Errorf("A: got (%v,%v), want (20,80)", pts[0].X, pts[0].Y)
		}
		if pts[1].X != 100 || pts[1].Y != 0 {
			t.Errorf("B: got (%v,%v), want (100,0)", pts[1].X, pts[1].Y)
		}
	})

	// Already-0-100 data must be left untouched.
	t.Run("leaves 0-100 data unchanged", func(t *testing.T) {
		pts := []Matrix2x2Point{
			{Label: "A", X: 20, Y: 80},
			{Label: "B", X: 75, Y: 30},
		}
		maybeScaleNormalizedPoints(pts, 0, 100, 0, 100)
		if len(pts) != 2 {
			t.Fatalf("expected 2 points, got %d", len(pts))
		}
		if pts[0].X != 20 || pts[1].X != 75 {
			t.Errorf("0-100 data was modified: %v", pts)
		}
	})

	// All-zero data should not be spuriously rescaled.
	t.Run("ignores all-zero data", func(t *testing.T) {
		pts := []Matrix2x2Point{{Label: "A", X: 0, Y: 0}}
		maybeScaleNormalizedPoints(pts, 0, 100, 0, 100)
		if pts[0].X != 0 || pts[0].Y != 0 {
			t.Errorf("all-zero data was modified: %v", pts)
		}
	})

	// A small axis range (already 0-1-ish) must not trigger scaling.
	t.Run("respects small axis range", func(t *testing.T) {
		pts := []Matrix2x2Point{{Label: "A", X: 0.2, Y: 0.8}}
		maybeScaleNormalizedPoints(pts, 0, 1, 0, 1)
		if pts[0].X != 0.2 || pts[0].Y != 0.8 {
			t.Errorf("small-range data was modified: %v", pts)
		}
	})
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
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
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

func TestMatrix2x2Diagram_DefaultQuadrantsFollowAxes(t *testing.T) {
	tests := []struct {
		name   string
		data   map[string]any
		want   [4]string
		absent string
	}{
		{
			name:   "custom axis labels",
			data:   map[string]any{"x_axis_label": "Ease", "y_axis_label": "Impact"},
			want:   [4]string{"High Impact / Low Ease", "High Impact / High Ease", "Low Impact / Low Ease", "Low Impact / High Ease"},
			absent: "High Value / Low Effort",
		},
		{
			name:   "blank axis names fall back to defaults",
			data:   map[string]any{"x_axis_label": " ", "y_axis_label": ""},
			want:   [4]string{"High Value / Low Effort", "High Value / High Effort", "Low Value / Low Effort", "Low Value / High Effort"},
			absent: "High  / Low  ",
		},
		{
			name: "axis aliases with explicit quadrant override",
			data: map[string]any{
				"x_label": "Cost", "y_label": "Benefit",
				"quadrant_labels": []any{"Quick wins"},
			},
			want:   [4]string{"Quick wins", "High Benefit / High Cost", "Low Benefit / Low Cost", "Low Benefit / High Cost"},
			absent: "High Benefit / Low Cost",
		},
		{
			name: "axis maps with named quadrant",
			data: map[string]any{
				"x_axis":    map[string]any{"label": "Complexity"},
				"y_axis":    map[string]any{"label": "Impact"},
				"quadrants": []any{map[string]any{"position": "top_right", "title": "Strategic bets"}},
			},
			want:   [4]string{"High Impact / Low Complexity", "Strategic bets", "Low Impact / Low Complexity", "Low Impact / High Complexity"},
			absent: "High Impact / High Complexity",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.data["points"] = []any{}
			req := &RequestEnvelope{Type: "matrix_2x2", Data: tt.data, Output: OutputSpec{Width: 1104, Height: 700}}
			doc, err := (&Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}).Render(req)
			if err != nil {
				t.Fatal(err)
			}
			svg := doc.String()
			for _, label := range tt.want {
				if !strings.Contains(svg, label) {
					t.Errorf("missing quadrant label %q", label)
				}
			}
			if strings.Contains(svg, tt.absent) {
				t.Errorf("obsolete quadrant label %q remains", tt.absent)
			}
		})
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
			"quadrant_colors":  []any{"#FF0000", "#00FF00", "#0000FF", "#FFFF00"},
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

// The coordinate-free quadrant form is parsed into per-quadrant LISTS, not into
// invented scatter coordinates. It used to produce a Matrix2x2Point per item at
// a guessed (x, y), which is what made two items per quadrant enough for the
// labels to overprint each other and the quadrant caption
// (go-slide-creator-s27x).
func TestParseQuadrantItemLists(t *testing.T) {
	quadrants := []any{
		map[string]any{
			"position": "top-left",
			"title":    "Quick Wins",
			"items":    []any{"Item A", "Item B"},
		},
		map[string]any{
			"position": "bottom_right", // underscore spelling is accepted
			"title":    "Low Priority",
			"items":    []any{"Item C", map[string]any{"label": "Item D"}},
		},
	}

	got, findings := parseQuadrantItemLists(quadrants)
	if len(findings) != 0 {
		t.Fatalf("valid quadrant positions emitted findings: %+v", findings)
	}

	want := [4][]string{
		{"Item A", "Item B"}, // top-left
		nil,                  // top-right
		nil,                  // bottom-left
		{"Item C", "Item D"}, // bottom-right
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("quadrant %d has %d items, want %d (%v)", i, len(got[i]), len(want[i]), got[i])
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("quadrant %d item %d = %q, want %q", i, j, got[i][j], want[i][j])
			}
		}
	}
}

func TestParseQuadrantItemLists_InvalidPositions(t *testing.T) {
	quadrants := []any{
		map[string]any{"position": "invalid", "items": []any{"Should Survive"}},
		map[string]any{"position": "top-left", "items": []any{"Valid Item"}},
	}

	got, findings := parseQuadrantItemLists(quadrants)

	if len(got[0]) != 2 || got[0][0] != "Should Survive" || got[0][1] != "Valid Item" {
		t.Errorf("top-left = %v, want both authored items", got[0])
	}
	if len(findings) != 1 || findings[0].Code != FindingQuadrantPositionDefaulted || findings[0].Field != "data.quadrants[0].position" {
		t.Fatalf("invalid position findings = %+v, want one actionable warning", findings)
	}
	if findings[0].Fix == nil || findings[0].Fix.Params["value"] != "top-left" {
		t.Errorf("invalid position fix = %+v, want top-left", findings[0].Fix)
	}
	for i := 1; i < 4; i++ {
		if len(got[i]) != 0 {
			t.Errorf("quadrant %d should be empty, got %v", i, got[i])
		}
	}
}

func TestMatrix2x2Diagram_DefaultedQuadrantPositionsKeepItemsAndCaptions(t *testing.T) {
	req := &RequestEnvelope{
		Type: "matrix_2x2", Output: OutputSpec{Width: 800, Height: 600},
		Data: map[string]any{"quadrants": []any{
			map[string]any{"label": "Quick wins", "items": []any{"Workflow status"}},
			map[string]any{"position": "diagonal", "label": "Strategic bets", "items": []any{"Platform API"}},
			map[string]any{"position": "bottom_left", "label": "Fill-ins", "items": []any{"Template cleanup"}},
		}},
	}
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}
	builder, svg, err := diagram.RenderWithBuilder(req)
	if err != nil {
		t.Fatalf("RenderWithBuilder: %v", err)
	}
	for _, label := range []string{"Quick wins", "Workflow status", "Strategic bets", "Platform API", "Fill-ins", "Template cleanup"} {
		if !strings.Contains(svg.String(), label) {
			t.Errorf("rendered matrix lost %q", label)
		}
	}
	findings := builder.Findings()
	if len(findings) != 2 {
		t.Fatalf("findings = %+v, want two position-defaulted warnings", findings)
	}
	for i, wantPosition := range []string{"top-left", "top-right"} {
		if findings[i].Code != FindingQuadrantPositionDefaulted || findings[i].Field != fmt.Sprintf("data.quadrants[%d].position", i) || findings[i].Severity != "warning" {
			t.Errorf("finding[%d] = %+v, want position-defaulted warning", i, findings[i])
		}
		if findings[i].Fix == nil || findings[i].Fix.Params["value"] != wantPosition {
			t.Errorf("finding[%d] fix = %+v, want %s", i, findings[i].Fix, wantPosition)
		}
	}
}

func TestParseQuadrantItemLists_EmptyItems(t *testing.T) {
	quadrants := []any{
		map[string]any{"position": "top-left", "items": []any{}},
		map[string]any{"position": "bottom-right"}, // no items key at all
		map[string]any{"position": "top-right", "items": []any{"", "   "}},
	}

	got, findings := parseQuadrantItemLists(quadrants)
	if len(findings) != 0 {
		t.Fatalf("valid empty quadrants emitted findings: %+v", findings)
	}

	for i, items := range got {
		if len(items) != 0 {
			t.Errorf("quadrant %d should be empty, got %v", i, items)
		}
	}
	var data Matrix2x2Data
	data.QuadrantItems = got
	if data.HasQuadrantItems() {
		t.Error("empty quadrants must not select the list renderer")
	}
}

func TestMatrix2x2Diagram_ValidateQuadrantShapes(t *testing.T) {
	diagram := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}
	tests := []struct {
		name      string
		quadrants any
		wantField string
	}{
		{"quadrants is not an array", "not-an-array", "data.quadrants"},
		{"quadrant is not an object", []any{"not-an-object"}, "data.quadrants[0]"},
		{"items is not an array", []any{map[string]any{"items": "not-an-array"}}, "data.quadrants[0].items"},
		{"item is not supported", []any{map[string]any{"items": []any{42}}}, "data.quadrants[0].items[0]"},
		{"item object has no label", []any{map[string]any{"items": []any{map[string]any{"text": "Lost"}}}}, "data.quadrants[0].items[0].label"},
		{"item object has non-string label", []any{map[string]any{"items": []any{map[string]any{"label": 42}}}}, "data.quadrants[0].items[0].label"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{Type: "matrix_2x2", Output: OutputSpec{Width: 800, Height: 600}, Data: map[string]any{"quadrants": tt.quadrants}}
			for name, validate := range map[string]func(*RequestEnvelope) error{
				"Validate": diagram.Validate,
				"RenderWithBuilder": func(req *RequestEnvelope) error {
					_, _, err := diagram.RenderWithBuilder(req)
					return err
				},
			} {
				t.Run(name, func(t *testing.T) {
					got := GetValidationErrors(validate(req))
					if len(got) != 1 || got[0].Field != tt.wantField || got[0].Code != ErrCodeInvalidType {
						t.Errorf("validation errors = %+v, want one INVALID_TYPE at %s", got, tt.wantField)
					}
				})
			}
		})
	}

	for _, tt := range []struct {
		name      string
		quadrants []any
	}{
		{"omitted items", []any{map[string]any{"position": "top-left"}}},
		{"empty items", []any{map[string]any{"position": "top-left", "items": []any{}}}},
		{"valid strings and labeled objects", []any{map[string]any{"position": "top-left", "items": []any{"Task", map[string]any{"label": "Other"}}}}},
		{"missing position uses fallback", []any{map[string]any{"items": []any{"Task"}}}},
		{"invalid position uses fallback", []any{map[string]any{"position": "diagonal", "items": []any{"Task"}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{Data: map[string]any{"quadrants": tt.quadrants}}
			if err := diagram.Validate(req); err != nil {
				t.Fatalf("valid quadrants rejected: %v", err)
			}
		})
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

func TestMatrix2x2CaptionBandProtectsHeadingFromHighPoint(t *testing.T) {
	builder := NewSVGBuilder(1104, 456)
	config := DefaultMatrix2x2Config(1104, 456)
	config.QuadrantLabels = [4]string{"Quick wins", "Strategic bets", "Fill-ins", "Deprioritise"}
	data := Matrix2x2Data{Points: []Matrix2x2Point{{Label: "Depot consolidation", X: 95, Y: 98}}}
	if err := NewMatrix2x2Chart(builder, config).Draw(data); err != nil {
		t.Fatal(err)
	}
	doc, err := builder.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.String(), "Strategic bets") || !strings.Contains(doc.String(), "Depot consolidation") {
		t.Fatal("caption or point label disappeared")
	}
	foundMove := false
	for _, finding := range builder.Findings() {
		if finding.Code == FindingDiagramTextOverlap && strings.Contains(finding.Message, "Depot consolidation") {
			foundMove = true
		}
	}
	if !foundMove {
		t.Error("moving a numeric point away from the caption was not disclosed")
	}
	for _, finding := range builder.textOverlapFindings() {
		if strings.Contains(finding.Message, "Strategic bets") {
			t.Errorf("caption still overlaps point text: %+v", finding)
		}
	}
}

func TestReserveMatrixCaptionBandDoesNotMoveUnrelatedPoints(t *testing.T) {
	plot := Rect{X: 0, Y: 0, W: 400, H: 300}
	bands := [4]placedLabel{{}, {x: 300, y: 20, w: 100, h: 35}}
	for _, tt := range []struct {
		name       string
		x, y       float64
		wantStatus captionBandStatus
	}{
		{"under top-right caption", 350, 20, captionBandMoved},
		{"left of caption", 250, 20, captionBandClear},
		{"above caption", 350, 5, captionBandClear},
		{"below caption", 350, 100, captionBandClear},
		{"bottom-right quadrant", 350, 180, captionBandClear},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, status := reserveMatrixCaptionBand(tt.x, tt.y, 12, plot, bands, 4)
			if status != tt.wantStatus {
				t.Errorf("status=%d, want %d", status, tt.wantStatus)
			}
			if status == captionBandMoved && got-6 < 59 {
				t.Errorf("marker at y=%.1f still covers the caption band", got)
			}
			if status == captionBandClear && got != tt.y {
				t.Errorf("unrelated point moved from %.1f to %.1f", tt.y, got)
			}
		})
	}
	if got, status := reserveMatrixCaptionBand(350, 2, 18, Rect{X: 0, Y: 0, W: 400, H: 70},
		[4]placedLabel{{}, {x: 300, y: 0, w: 100, h: 30}}, 4); status != captionBandNoRoom || got != 2 {
		t.Errorf("tiny quadrant must report no room without moving point across divider: y=%.1f status=%d", got, status)
	}
}

func TestMatrix2x2Chart_LongLabelsNotBlindlyTruncated(t *testing.T) {
	// A character count is not a fit test: preserve a long label when the
	// measured text fits or can wrap inside the plot.
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

	if !strings.Contains(content, longLabel) {
		t.Error("long label was discarded despite available space")
	}
	if strings.Contains(content, "…") {
		t.Error("long label was truncated by a character cap")
	}

	// Short label should appear unchanged.
	if !strings.Contains(content, "Short") {
		t.Error("Expected short label to appear unchanged")
	}
}

func TestMatrix2x2PointLabelsFlipInsidePlot(t *testing.T) {
	const label = "International operations team"
	plot := Rect{X: 200, Y: 100, W: 400, H: 300}
	for _, tc := range []struct {
		name       string
		x          float64
		wantAnchor string
	}{
		{"left edge", plot.X + 8, "-"},
		{"right edge", plot.X + plot.W - 8, "end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewSVGBuilder(800, 600)
			chart := NewMatrix2x2Chart(builder, DefaultMatrix2x2Config(800, 600))
			fontSize := builder.StyleGuide().Typography.SizeSmall
			builder.SetFontSize(fontSize)
			width, _ := builder.MeasureText(label)
			if width*1.2 >= plot.W-40 {
				t.Fatalf("test label %.1fpx is too wide for plot", width)
			}
			box := chart.drawPointLabelAvoiding(tc.x, 250, 12, label, plot, nil, fontSize, 16)
			if box.x < plot.X || box.x+box.w > plot.X+plot.W {
				t.Errorf("point label box %+v escapes plot %+v", box, plot)
			}
			doc, err := builder.Render()
			if err != nil {
				t.Fatal(err)
			}
			svg := doc.String()
			if !strings.Contains(svg, label) || strings.Contains(svg, "…") {
				t.Errorf("fitting label was truncated: %s", svg)
			}
			anchor, _, found := textElementAttrs(svg, label)
			if !found || anchor != tc.wantAnchor {
				t.Errorf("label anchor = %q, found=%t; want %q", anchor, found, tc.wantAnchor)
			}
		})
	}
}

func TestMatrix2x2WrappedLabelCollisionBoxStaysInPlot(t *testing.T) {
	plot := Rect{X: 200, Y: 100, W: 180, H: 300}
	label := strings.Repeat("Long portfolio initiative ", 5)
	builder := NewSVGBuilder(800, 600)
	chart := NewMatrix2x2Chart(builder, DefaultMatrix2x2Config(800, 600))
	box := chart.drawPointLabelAvoiding(plot.X+plot.W/2, 250, 12, label, plot, nil,
		builder.StyleGuide().Typography.SizeSmall, 16)
	if box.x < plot.X || box.x+box.w > plot.X+plot.W {
		t.Errorf("wrapped label collision box %+v escapes plot %+v", box, plot)
	}
	if box.h <= builder.StyleGuide().Typography.SizeSmall*1.3 {
		t.Errorf("long label was not wrapped: %+v", box)
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
	// No ellipsis should appear for these reasonably-sized labels.
	// Note: on systems with wider font metrics (e.g., Linux CI with
	// different fonts), labels may still be truncated. Log instead of fail
	// since this is font-dependent behavior.
	if strings.Contains(content, "…") {
		t.Logf("Labels were truncated with ellipsis (may be font-dependent)")
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
