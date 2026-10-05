package svggen

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"testing"
)

func TestWaterfallChart_BasicRender(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.ShowValues = true
	config.ShowGrid = true

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title: "Revenue Bridge",
		Points: []WaterfallDataPoint{
			{Label: "Start", Value: 100, Type: WaterfallTypeIncrease},
			{Label: "Growth", Value: 30, Type: WaterfallTypeIncrease},
			{Label: "Churn", Value: -15, Type: WaterfallTypeDecrease},
			{Label: "End", Value: 115, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	content := svg.String()
	if content == "" {
		t.Error("Expected non-empty SVG content")
	}

	// Check that SVG contains expected elements
	if !strings.Contains(content, "svg") {
		t.Error("Expected SVG content to contain 'svg' tag")
	}
}

func TestWaterfallChart_NegativeValues(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.ShowValues = true

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title: "Cost Breakdown",
		Points: []WaterfallDataPoint{
			{Label: "Revenue", Value: 1000, Type: WaterfallTypeIncrease},
			{Label: "COGS", Value: -400, Type: WaterfallTypeDecrease},
			{Label: "OpEx", Value: -200, Type: WaterfallTypeDecrease},
			{Label: "Tax", Value: -100, Type: WaterfallTypeDecrease},
			{Label: "Profit", Value: 300, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with negative values: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if svg.Width != 1067 || svg.Height != 800 {
		t.Errorf("Expected dimensions 1067x800 (800x600pt in CSS pixels), got %.0fx%.0f", svg.Width, svg.Height)
	}
}

func TestWaterfallChart_Subtotal(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title: "Quarterly Bridge",
		Points: []WaterfallDataPoint{
			{Label: "Q1", Value: 100, Type: WaterfallTypeIncrease},
			{Label: "+Sales", Value: 50, Type: WaterfallTypeIncrease},
			{Label: "-Costs", Value: -20, Type: WaterfallTypeDecrease},
			{Label: "Q1 End", Value: 130, Type: WaterfallTypeSubtotal},
			{Label: "+Q2 Sales", Value: 60, Type: WaterfallTypeIncrease},
			{Label: "Q2 End", Value: 190, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with subtotal: %v", err)
	}

	_, err = builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

func TestWaterfallChart_CustomColors(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.IncreaseColor = MustParseColor("#00FF00")
	config.DecreaseColor = MustParseColor("#FF0000")
	config.TotalColor = MustParseColor("#0000FF")

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title: "Custom Colors",
		Points: []WaterfallDataPoint{
			{Label: "A", Value: 100},
			{Label: "B", Value: 20},
			{Label: "C", Value: -10},
			{Label: "Total", Value: 110, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with custom colors: %v", err)
	}
}

func TestWaterfallChart_PerPointColor(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	chart := NewWaterfallChart(builder, config)

	customColor := MustParseColor("#FF00FF")

	data := WaterfallData{
		Points: []WaterfallDataPoint{
			{Label: "A", Value: 100, Color: &customColor},
			{Label: "B", Value: 50},
			{Label: "Total", Value: 150, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with per-point color: %v", err)
	}
}

// TestWaterfallChart_AllLabelsShownAtTypicalDensity is a regression test for
// go-slide-creator-h6ok. An 11-point waterfall at full width (900pt) used to
// thin value labels by index (showing only every other bar's value), which
// produced inconsistent and confusing label placement. The fix replaces
// index-based thinning with measurement-based thinning, so labels are shown
// for every bar when they fit in the per-bar slot.
func TestWaterfallChart_AllLabelsShownAtTypicalDensity(t *testing.T) {
	builder := NewSVGBuilder(900, 500)
	config := DefaultWaterfallChartConfig(900, 500)
	config.ShowValues = true
	config.ShowConnectors = true

	chart := NewWaterfallChart(builder, config)
	err := chart.Draw(WaterfallData{
		Title: "Investment Breakdown",
		Points: []WaterfallDataPoint{
			{Label: "Data Centers", Value: -3.2, Type: WaterfallTypeDecrease},
			{Label: "GPU Clusters", Value: -4.8, Type: WaterfallTypeDecrease},
			{Label: "Cloud Platform", Value: -2.4, Type: WaterfallTypeDecrease},
			{Label: "AI/ML Stack", Value: -1.6, Type: WaterfallTypeDecrease},
			{Label: "Security/Gov", Value: -0.8, Type: WaterfallTypeDecrease},
			{Label: "Talent & Ops", Value: -1.2, Type: WaterfallTypeDecrease},
			{Label: "Total Investment", Value: -14.0, Type: WaterfallTypeSubtotal},
			{Label: "Avoided Costs", Value: 8.5, Type: WaterfallTypeIncrease},
			{Label: "Revenue Creation", Value: 12.2, Type: WaterfallTypeIncrease},
			{Label: "Strategic Value", Value: 24.1, Type: WaterfallTypeIncrease},
			{Label: "Net Value", Value: 30.8, Type: WaterfallTypeTotal},
		},
	})
	if err != nil {
		t.Fatalf("Draw failed: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	content := svg.String()

	// Every bar's value label must appear in the SVG output. The negative-
	// delta labels render with a leading true minus sign U+2212 (e.g.,
	// "\u22124.8"); positive deltas use a "+" prefix (e.g., "+8.5");
	// totals/subtotals use unsigned values.
	expected := []string{"\u22123.2", "\u22124.8", "\u22122.4", "\u22121.6", "\u22120.8", "\u22121.2", "\u221214.0", "+8.5", "+12.2", "+24.1", "30.8"}
	for _, want := range expected {
		if !strings.Contains(content, want) {
			t.Errorf("SVG output missing value label %q (regression: all bars must be labeled)", want)
		}
	}
}

// TestWaterfallChart_ConnectorIntoSubtotal verifies that a connector is drawn
// at the running-total level when transitioning from a delta bar into a
// subtotal bar. Previously the connector was skipped, leaving a visible "gap"
// in the bridge flow at the subtotal column. Regression: go-slide-creator-h6ok.
func TestWaterfallChart_ConnectorIntoSubtotal(t *testing.T) {
	builder := NewSVGBuilder(800, 500)
	config := DefaultWaterfallChartConfig(800, 500)
	config.ShowConnectors = true
	config.ConnectorDash = false

	chart := NewWaterfallChart(builder, config)
	err := chart.Draw(WaterfallData{
		Points: []WaterfallDataPoint{
			{Label: "A", Value: -5, Type: WaterfallTypeDecrease},
			{Label: "B", Value: -3, Type: WaterfallTypeDecrease},
			{Label: "Sub", Value: -8, Type: WaterfallTypeSubtotal},
			{Label: "C", Value: 10, Type: WaterfallTypeIncrease},
		},
	})
	if err != nil {
		t.Fatalf("Draw failed: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	content := svg.String()

	// Count connector lines: each connector is a stroked path with the
	// connector color. We expect three: A→B, B→Sub, Sub→C. The pre-fix
	// behavior produced only two (it skipped B→Sub).
	//
	// Connectors share the ConnectorColor hex; count occurrences as a coarse
	// proxy (other strokes use axis/grid colors instead).
	connectorMarker := "stroke:#6c757d" // DefaultThemeTextMutedHex
	count := strings.Count(content, connectorMarker)
	if count < 3 {
		t.Errorf("expected at least 3 connector strokes (A→B, B→Sub, Sub→C); got %d", count)
	}
}

func TestWaterfallChart_NoConnectors(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.ShowConnectors = false

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Points: []WaterfallDataPoint{
			{Label: "Start", Value: 100},
			{Label: "Change", Value: 25},
			{Label: "End", Value: 125, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart without connectors: %v", err)
	}
}

func TestWaterfallChart_DashedConnectors(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.ShowConnectors = true
	config.ConnectorDash = true

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Points: []WaterfallDataPoint{
			{Label: "Start", Value: 100},
			{Label: "Up", Value: 30},
			{Label: "Down", Value: -20},
			{Label: "End", Value: 110, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with dashed connectors: %v", err)
	}
}

func TestWaterfallChart_EmptyData(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title:  "Empty",
		Points: []WaterfallDataPoint{},
	}

	err := chart.Draw(data)
	if err == nil {
		t.Error("Expected error for empty data points")
	}
}

func TestWaterfallChart_SinglePoint(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.ShowValues = true

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Points: []WaterfallDataPoint{
			{Label: "Total", Value: 100, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with single point: %v", err)
	}
}

func TestWaterfallChart_WithFootnote(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title:    "Revenue Analysis",
		Subtitle: "FY 2024",
		Points: []WaterfallDataPoint{
			{Label: "Q1", Value: 100},
			{Label: "Q2", Value: 30},
			{Label: "Year", Value: 130, Type: WaterfallTypeTotal},
		},
		Footnote: "Source: Internal data",
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with footnote: %v", err)
	}
}

func TestWaterfallDiagram_Validate(t *testing.T) {
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name:    "nil data",
			req:     &RequestEnvelope{Type: "waterfall", Data: nil},
			wantErr: true,
		},
		{
			name:    "missing points",
			req:     &RequestEnvelope{Type: "waterfall", Data: map[string]any{}},
			wantErr: true,
		},
		{
			name:    "empty points",
			req:     &RequestEnvelope{Type: "waterfall", Data: map[string]any{"points": []any{}}},
			wantErr: true,
		},
		{
			name: "valid points",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "Start", "value": 100.0},
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

func TestWaterfallPublicRenderRejectsMalformedPoints(t *testing.T) {
	tests := []struct {
		name  string
		point any
		want  string
	}{
		{"missing value", map[string]any{"label": "Cost"}, "finite numeric value"},
		{"string value", map[string]any{"label": "Cost", "value": "-40"}, "finite numeric value"},
		{"blank label", map[string]any{"label": " ", "value": 40}, "nonempty string label"},
		{"missing label", map[string]any{"value": 40}, "nonempty string label"},
		{"invalid type", map[string]any{"label": "Cost", "value": -40, "type": "banana"}, "invalid type"},
		{"nonnumeric type", map[string]any{"label": "Cost", "value": -40, "type": 5}, "type must be a string"},
		{"not an object", "Cost", "must be an object"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &RequestEnvelope{Type: "waterfall", Data: map[string]any{
				"points": []any{map[string]any{"label": "Start", "value": 100}, tc.point},
			}, Output: OutputSpec{Width: 800, Height: 600}}
			_, err := Render(req)
			if err == nil || !strings.Contains(err.Error(), "point 1") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Render error = %v, want point 1 %q", err, tc.want)
			}
		})
	}
	// The registry deliberately clamps nonfinite numbers before dispatch for
	// all diagram types. Direct diagram validation still rejects them.
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}
	if err := diagram.Validate(&RequestEnvelope{Type: "waterfall", Data: map[string]any{
		"points": []any{map[string]any{"label": "Cost", "value": math.Inf(1)}},
	}}); err == nil || !strings.Contains(err.Error(), "finite numeric value") {
		t.Fatalf("direct Validate did not reject nonfinite value: %v", err)
	}
	// Direct diagram rendering bypasses registry Validate, so the parser must
	// also refuse malformed values instead of drawing a zero-height bar.
	if _, err := diagram.Render(&RequestEnvelope{Type: "waterfall", Data: map[string]any{
		"points": []any{map[string]any{"label": "Cost", "value": "-40"}},
	}, Output: OutputSpec{Width: 800, Height: 600}}); err == nil || !strings.Contains(err.Error(), "finite numeric value") {
		t.Fatalf("direct Render did not reject string value: %v", err)
	}
}

func TestWaterfallPublicRenderAcceptsNumericVariantsAndInferredType(t *testing.T) {
	req := &RequestEnvelope{Type: "waterfall", Data: map[string]any{
		"points": []any{
			map[string]any{"label": "Start", "value": json.Number("100")},
			map[string]any{"label": "Flat", "value": 0},
			map[string]any{"label": "Cost", "value": int64(-40), "type": "NEGATIVE"},
			map[string]any{"label": "End", "value": float32(60), "type": "total"},
		},
	}, Output: OutputSpec{Width: 800, Height: 600}}
	doc, err := Render(req)
	if err != nil || doc == nil || len(doc.Content) == 0 {
		t.Fatalf("valid numeric waterfall did not render: doc=%v err=%v", doc, err)
	}
}

func TestWaterfallShorthandDoesNotDropUnmatchedBars(t *testing.T) {
	tests := []struct {
		name   string
		labels []any
		values []any
		want   string
	}{
		{"extra label", []any{"Start", "Cost", "End"}, []any{100, -40}, "3 labels, 2 values"},
		{"extra value", []any{"Start", "Cost"}, []any{100, -40, 60}, "2 labels, 3 values"},
		{"empty", []any{}, []any{}, "nonempty"},
		{"bad value", []any{"Start"}, []any{"100"}, "values must be numbers"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &RequestEnvelope{Type: "waterfall", Data: map[string]any{
				"labels": tc.labels, "values": tc.values,
			}, Output: OutputSpec{Width: 800, Height: 600}}
			_, err := Render(req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Render error = %v, want %q", err, tc.want)
			}
			if _, mutated := req.Data["points"]; mutated {
				t.Fatal("invalid shorthand was normalized into truncated points")
			}
		})
	}
	req := &RequestEnvelope{Type: "waterfall", Data: map[string]any{
		"labels": []any{"Start", "Cost", "End"}, "values": []any{100, -40, 60},
	}, Output: OutputSpec{Width: 800, Height: 600}}
	doc, err := Render(req)
	if err != nil || doc == nil || len(doc.Content) == 0 {
		t.Fatalf("equal-length shorthand did not render: doc=%v err=%v", doc, err)
	}
	if !strings.Contains(string(doc.Content), "End") {
		t.Fatal("equal-length shorthand dropped the final End bar label")
	}
}

func TestWaterfallDiagram_Render(t *testing.T) {
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}

	req := &RequestEnvelope{
		Type:  "waterfall",
		Title: "Budget Variance",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Budget", "value": 1000.0, "type": "increase"},
				map[string]any{"label": "Savings", "value": 200.0, "type": "increase"},
				map[string]any{"label": "Overrun", "value": -150.0, "type": "decrease"},
				map[string]any{"label": "Actual", "value": 1050.0, "type": "total"},
			},
		},
		Output: OutputSpec{
			Width:  800,
			Height: 600,
		},
		Style: StyleSpec{
			ShowValues: true,
			ShowGrid:   true,
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render waterfall diagram: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	if len(svg.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestWaterfallDiagram_RenderWithCustomColors(t *testing.T) {
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}

	req := &RequestEnvelope{
		Type: "waterfall",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Start", "value": 100.0},
				map[string]any{"label": "Drop", "value": -10.0},
				map[string]any{"label": "End", "value": 90.0, "type": "total"},
			},
			"colors": map[string]any{
				"increase": "#00FF00",
				"decrease": "#FF0000",
				"total":    "#0000FF",
			},
		},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render waterfall diagram with custom colors: %v", err)
	}

	if svg == nil {
		t.Fatal("Expected non-nil SVG document")
	}
	for _, color := range []string{"#0f0", "#f00", "#00f"} {
		if !strings.Contains(strings.ToLower(svg.String()), color) {
			t.Errorf("explicit waterfall color %s was not rendered", color)
		}
	}
}

func TestWaterfallDiagram_UsesSemanticThemeAccents(t *testing.T) {
	// go-slide-creator-sdxii: a bridge tells its story through the decreases,
	// so they take the chart's first accent, totals stay a neutral dk1 tint
	// (60%) and increases a legible 55% accent1 tint (go-slide-creator-rmm0x),
	// whatever semantic accents the template declares.
	req := &RequestEnvelope{
		Type: "waterfall",
		Data: map[string]any{"points": []any{
			map[string]any{"label": "Gain", "value": 100.0},
			map[string]any{"label": "Loss", "value": -25.0},
			map[string]any{"label": "Total", "value": 75.0, "type": "total"},
		}},
		Style: StyleSpec{
			ThemeColors: []ThemeColorInput{
				{Name: "dk1", RGB: "#000000"},
				{Name: "lt1", RGB: "#FFFFFF"},
				{Name: "accent1", RGB: "#B54C27"},
				{Name: "accent2", RGB: "#58718A"},
				{Name: "accent3", RGB: "#1A7B3C"},
			},
			SemanticAccents: SemanticAccentSpec{Positive: "accent3", Negative: "accent1", Neutral: "accent2"},
		},
	}
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}
	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.ToLower(svg.String())
	for _, color := range []string{"#b54c27", "#666", "#cb8268"} {
		if !strings.Contains(out, `fill="`+color+`"`) {
			t.Errorf("waterfall fill %s was not rendered; fills %v", color, regexp.MustCompile(`fill="#[0-9a-f]+"`).FindAllString(out, -1))
		}
	}
	for _, color := range []string{"#1a7b3c", "#58718a"} {
		if strings.Contains(out, `fill="`+color+`"`) {
			t.Errorf("waterfall still paints a semantic accent %s", color)
		}
	}
}

func TestWaterfallDiagram_Type(t *testing.T) {
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}
	if diagram.Type() != "waterfall" {
		t.Errorf("Expected type 'waterfall', got '%s'", diagram.Type())
	}
}

func TestCreateWaterfallPoints(t *testing.T) {
	labels := []string{"Start", "Add", "Sub", "Total"}
	values := []float64{100, 30, -20, 110}

	points := CreateWaterfallPoints(labels, values, true)

	if len(points) != 4 {
		t.Errorf("Expected 4 points, got %d", len(points))
	}

	// Check types
	if points[0].Type != WaterfallTypeIncrease {
		t.Errorf("Expected first point to be increase, got %s", points[0].Type)
	}
	if points[1].Type != WaterfallTypeIncrease {
		t.Errorf("Expected second point to be increase, got %s", points[1].Type)
	}
	if points[2].Type != WaterfallTypeDecrease {
		t.Errorf("Expected third point to be decrease, got %s", points[2].Type)
	}
	if points[3].Type != WaterfallTypeTotal {
		t.Errorf("Expected fourth point to be total, got %s", points[3].Type)
	}
}

func TestCreateWaterfallPoints_NoTotal(t *testing.T) {
	labels := []string{"A", "B", "C"}
	values := []float64{10, 20, -5}

	points := CreateWaterfallPoints(labels, values, false)

	if len(points) != 3 {
		t.Errorf("Expected 3 points, got %d", len(points))
	}

	// Last point should NOT be a total
	if points[2].Type == WaterfallTypeTotal {
		t.Error("Expected last point to NOT be a total")
	}
}

func TestDrawWaterfallFromData(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	points := []WaterfallDataPoint{
		{Label: "Start", Value: 100, Type: WaterfallTypeIncrease},
		{Label: "Growth", Value: 50, Type: WaterfallTypeIncrease},
		{Label: "End", Value: 150, Type: WaterfallTypeTotal},
	}

	err := DrawWaterfallFromData(builder, "Test Chart", points)
	if err != nil {
		t.Fatalf("Failed to draw waterfall from data: %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if svg == nil || len(svg.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestCalculateRunningTotals(t *testing.T) {
	points := []WaterfallDataPoint{
		{Label: "Start", Value: 100, Type: WaterfallTypeIncrease},
		{Label: "Add", Value: 30, Type: WaterfallTypeIncrease},
		{Label: "Sub", Value: -20, Type: WaterfallTypeDecrease},
		{Label: "Total", Value: 110, Type: WaterfallTypeTotal},
	}

	totals := calculateRunningTotals(points)

	expected := []float64{100, 130, 110, 110}
	if len(totals) != len(expected) {
		t.Fatalf("Expected %d totals, got %d", len(expected), len(totals))
	}

	for i, exp := range expected {
		if totals[i] != exp {
			t.Errorf("Total at index %d: expected %v, got %v", i, exp, totals[i])
		}
	}
}

func TestWaterfallChart_RoundedCorners(t *testing.T) {
	builder := NewSVGBuilder(800, 600)

	config := DefaultWaterfallChartConfig(800, 600)
	config.BarCornerRadius = 4

	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Points: []WaterfallDataPoint{
			{Label: "A", Value: 100},
			{Label: "B", Value: 50},
			{Label: "Total", Value: 150, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with rounded corners: %v", err)
	}
}

func TestDefaultWaterfallChartConfig(t *testing.T) {
	config := DefaultWaterfallChartConfig(800, 600)

	if config.Width != 800 {
		t.Errorf("Expected width 800, got %v", config.Width)
	}
	if config.Height != 600 {
		t.Errorf("Expected height 600, got %v", config.Height)
	}
	if !config.ShowConnectors {
		t.Error("Expected ShowConnectors to be true by default")
	}
	if config.BarPadding <= 0 {
		t.Error("Expected BarPadding to be positive")
	}
}

func TestWaterfallChart_DecimalPrecision(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	config := DefaultWaterfallChartConfig(800, 600)
	chart := NewWaterfallChart(b, config)

	data := WaterfallData{
		Title: "Revenue to EBITDA ($M)",
		Points: []WaterfallDataPoint{
			{Label: "Revenue", Value: 4.8, Type: WaterfallTypeIncrease},
			{Label: "COGS", Value: -1.4, Type: WaterfallTypeDecrease},
			{Label: "S&M", Value: -1.2, Type: WaterfallTypeDecrease},
			{Label: "R&D", Value: -0.8, Type: WaterfallTypeDecrease},
			{Label: "G&A", Value: -0.5, Type: WaterfallTypeDecrease},
			{Label: "EBITDA", Value: 0.9, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}

	svg := string(doc.Content)
	// Decimal values should be preserved (e.g., "4.8" not "5")
	if !strings.Contains(svg, "4.8") {
		t.Error("SVG should contain '4.8' - decimal value lost")
	}
	if !strings.Contains(svg, "1.4") {
		t.Error("SVG should contain '1.4' - decimal value lost")
	}
	if !strings.Contains(svg, "0.9") {
		t.Error("SVG should contain '0.9' - decimal value lost")
	}
}

func TestWaterfallChart_IntegerDataKeepsDefaultFormat(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	config := DefaultWaterfallChartConfig(800, 600)
	chart := NewWaterfallChart(b, config)

	data := WaterfallData{
		Title: "Revenue to Profit",
		Points: []WaterfallDataPoint{
			{Label: "Revenue", Value: 1000, Type: WaterfallTypeIncrease},
			{Label: "COGS", Value: -400, Type: WaterfallTypeDecrease},
			{Label: "Net Income", Value: 600, Type: WaterfallTypeTotal},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}

	svg := string(doc.Content)
	// Integer values should NOT have decimal points (e.g., "1000" not "1000.0")
	if strings.Contains(svg, "1000.0") {
		t.Error("SVG should contain '1000' not '1000.0' for integer data")
	}
}

func TestWaterfallChart_LongLabelsPreferRotationOverTruncation(t *testing.T) {
	// Verifies that long labels like "Sales & Marketing" are rotated rather
	// than aggressively truncated to unreadable stubs. Rotation preserves
	// the full label text while avoiding horizontal overlap.
	builder := NewSVGBuilder(900, 500)

	config := DefaultWaterfallChartConfig(900, 500)
	config.ShowValues = true
	chart := NewWaterfallChart(builder, config)

	data := WaterfallData{
		Title:    "Profit Bridge Analysis",
		Subtitle: "Revenue to Net Income",
		Points: []WaterfallDataPoint{
			{Label: "Revenue", Value: 1000, Type: WaterfallTypeTotal},
			{Label: "COGS", Value: -380, Type: WaterfallTypeDecrease},
			{Label: "Gross Profit", Value: 620, Type: WaterfallTypeSubtotal},
			{Label: "Sales & Marketing", Value: -180, Type: WaterfallTypeDecrease},
			{Label: "R&D", Value: -120, Type: WaterfallTypeDecrease},
			{Label: "G&A", Value: -80, Type: WaterfallTypeDecrease},
			{Label: "Operating Income", Value: 240, Type: WaterfallTypeSubtotal},
			{Label: "Other Income", Value: 25, Type: WaterfallTypeIncrease},
			{Label: "Interest", Value: -15, Type: WaterfallTypeDecrease},
			{Label: "Taxes", Value: -62, Type: WaterfallTypeDecrease},
			{Label: "Net Income", Value: 188, Type: WaterfallTypeTotal},
		},
		Footnote: "All values in millions USD",
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw waterfall chart with long labels: %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := string(doc.Content)

	// Long labels that exceed bandwidth should be rotated (not truncated
	// to unreadable stubs). Check that labels remain substantially intact.
	// "Sales & Marketing" (17 chars) should not be chopped to 5-6 chars.
	if !strings.Contains(svg, "Sales") {
		t.Error("SVG should contain 'Sales' — labels should not be truncated to stubs")
	}
	if !strings.Contains(svg, "Operating") {
		t.Error("SVG should contain 'Operating' — labels should not be truncated to stubs")
	}

	// The SVG should render without error (already confirmed above).
	if len(svg) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// TestWaterfallDiagram_TypeCaseInsensitive verifies that waterfall item type
// strings are case-insensitive: "Increase", "DECREASE", "Total" should all
// be normalized to lowercase before comparison (bug d5f3).
func TestWaterfallDiagram_TypeCaseInsensitive(t *testing.T) {
	diagram := &WaterfallDiagram{NewBaseDiagram("waterfall")}

	req := &RequestEnvelope{
		Type:  "waterfall",
		Title: "Case Insensitive Types",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Start", "value": 100.0, "type": "Increase"},
				map[string]any{"label": "Growth", "value": 50.0, "type": "INCREASE"},
				map[string]any{"label": "Loss", "value": -30.0, "type": "Decrease"},
				map[string]any{"label": "Subtotal", "value": 120.0, "type": "Subtotal"},
				map[string]any{"label": "End", "value": 120.0, "type": "TOTAL"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	svg, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Failed to render waterfall with mixed-case types: %v", err)
	}
	if svg == nil || len(svg.Content) == 0 {
		t.Fatal("Expected non-empty SVG document")
	}
}

// TestParseWaterfallData_TypeNormalization verifies that parseWaterfallData
// normalizes type strings to lowercase.
func TestParseWaterfallData_TypeNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected WaterfallChartType
	}{
		{"increase", WaterfallTypeIncrease},
		{"Increase", WaterfallTypeIncrease},
		{"INCREASE", WaterfallTypeIncrease},
		{"decrease", WaterfallTypeDecrease},
		{"Decrease", WaterfallTypeDecrease},
		{"DECREASE", WaterfallTypeDecrease},
		{"total", WaterfallTypeTotal},
		{"Total", WaterfallTypeTotal},
		{"TOTAL", WaterfallTypeTotal},
		{"subtotal", WaterfallTypeSubtotal},
		{"Subtotal", WaterfallTypeSubtotal},
		{"SUBTOTAL", WaterfallTypeSubtotal},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			req := &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "Test", "value": 100.0, "type": tt.input},
					},
				},
			}
			data, err := parseWaterfallData(req)
			if err != nil {
				t.Fatalf("parseWaterfallData() error = %v", err)
			}
			if len(data.Points) != 1 {
				t.Fatalf("Expected 1 point, got %d", len(data.Points))
			}
			if data.Points[0].Type != tt.expected {
				t.Errorf("Type = %q, want %q", data.Points[0].Type, tt.expected)
			}
		})
	}
}
