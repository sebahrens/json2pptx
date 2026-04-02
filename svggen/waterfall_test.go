package svggen

import (
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
				map[string]any{"label": "End", "value": 100.0, "type": "total"},
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

func TestWaterfallChart_CalculateDomain(t *testing.T) {
	wc := &WaterfallChart{config: DefaultWaterfallChartConfig(800, 600)}

	tests := []struct {
		name     string
		points   []WaterfallDataPoint
		wantMin  float64
		wantMax  float64
		minCheck bool // if true, min must be <= wantMin
		maxCheck bool // if true, max must be >= wantMax
	}{
		{
			name: "all positive close to zero",
			points: []WaterfallDataPoint{
				// Data span is 50 (100 to 150). Min=100 is <= span=50? No.
				// But a smaller example: 10 → 60 has span=50, min=10 <= 50 → includes 0.
				{Label: "A", Value: 10, Type: WaterfallTypeIncrease},
				{Label: "B", Value: 50, Type: WaterfallTypeIncrease},
			},
			wantMin:  0,
			wantMax:  60,
			minCheck: true,
			maxCheck: true,
		},
		{
			name: "all positive far from zero uses broken axis",
			points: []WaterfallDataPoint{
				// Running totals: 100, 150. Span=50, min=100. 100 > 50 → broken axis.
				{Label: "A", Value: 100, Type: WaterfallTypeIncrease},
				{Label: "B", Value: 50, Type: WaterfallTypeIncrease},
			},
			// Broken axis: min should be < 100 but > 0
			wantMin:  100,
			wantMax:  150,
			minCheck: true, // min must be <= 100 (less than data min, with padding)
			maxCheck: true,
		},
		{
			name: "with negative",
			points: []WaterfallDataPoint{
				{Label: "A", Value: 100, Type: WaterfallTypeIncrease},
				{Label: "B", Value: -150, Type: WaterfallTypeDecrease},
			},
			wantMin:  -50,
			wantMax:  100,
			minCheck: true,
			maxCheck: true,
		},
		{
			name: "with total far from zero uses broken axis",
			points: []WaterfallDataPoint{
				// Running totals: 100, 150, 150. Span=50, min=100 > 50 → broken axis.
				{Label: "A", Value: 100, Type: WaterfallTypeIncrease},
				{Label: "B", Value: 50, Type: WaterfallTypeIncrease},
				{Label: "Total", Value: 150, Type: WaterfallTypeTotal},
			},
			wantMin:  100,
			wantMax:  150,
			minCheck: true,
			maxCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max := wc.calculateDomain(tt.points)
			if tt.minCheck && min > tt.wantMin {
				t.Errorf("calculateDomain() min = %v, want <= %v", min, tt.wantMin)
			}
			if tt.maxCheck && max < tt.wantMax {
				t.Errorf("calculateDomain() max = %v, want >= %v", max, tt.wantMax)
			}
		})
	}
}

// TestWaterfallChart_BrokenAxisForSmallIncrements tests the fix for g512:
// when a waterfall chart has a large total but small incremental changes,
// the y-axis should zoom in on the data range instead of starting at 0.
func TestWaterfallChart_BrokenAxisForSmallIncrements(t *testing.T) {
	wc := &WaterfallChart{config: DefaultWaterfallChartConfig(800, 600)}

	t.Run("NOI bridge with small increments", func(t *testing.T) {
		// Real Estate NOI Bridge scenario from the bug report:
		// Prior Year NOI: $485M, changes of +28, +45, -18, -12, Current NOI: $528M
		points := []WaterfallDataPoint{
			{Label: "Prior Year NOI", Value: 485, Type: WaterfallTypeTotal},
			{Label: "Same-Store Growth", Value: 28, Type: WaterfallTypeIncrease},
			{Label: "Acquisitions", Value: 45, Type: WaterfallTypeIncrease},
			{Label: "Dispositions", Value: -18, Type: WaterfallTypeDecrease},
			{Label: "Vacancy Impact", Value: -12, Type: WaterfallTypeDecrease},
			{Label: "Current NOI", Value: 528, Type: WaterfallTypeTotal},
		}

		min, max := wc.calculateDomain(points)

		// The y-axis should NOT start at 0 — that would make the change bars tiny.
		// Running totals: 485, 513, 558, 540, 528, 528.
		// Data range: 485-558. Span = 73. Min=485 >> 73 → broken axis.
		if min <= 0 {
			t.Errorf("min should be > 0 for NOI bridge (got %v), y-axis should not start at 0", min)
		}
		// min should be below the lowest running total (485) with some padding
		if min >= 485 {
			t.Errorf("min should be < 485 (lowest data point) with padding, got %v", min)
		}
		// min should be reasonable — not too far below (within ~15% of span)
		if min < 470 {
			t.Errorf("min should be around 470-485 range, got %v (too much padding)", min)
		}
		// max should be above the highest running total (558)
		if max <= 558 {
			t.Errorf("max should be > 558 (highest running total), got %v", max)
		}
		// max should be reasonable
		if max > 600 {
			t.Errorf("max should be < 600, got %v (too much headroom)", max)
		}
	})

	t.Run("pharma bridge with proportional changes still includes zero", func(t *testing.T) {
		// Pharma R&D scenario: Revenue $2100, COGS -$400, OpEx -$300,
		// R&D Budget $800, Partnerships -$150, Net R&D $650
		// Here changes are large relative to total, and running total
		// approaches 0 territory (goes as low as ~650).
		points := []WaterfallDataPoint{
			{Label: "Revenue", Value: 2100, Type: WaterfallTypeTotal},
			{Label: "COGS", Value: -400, Type: WaterfallTypeDecrease},
			{Label: "OpEx", Value: -300, Type: WaterfallTypeDecrease},
			{Label: "R&D Budget", Value: -550, Type: WaterfallTypeDecrease},
			{Label: "Partnerships", Value: -150, Type: WaterfallTypeDecrease},
			{Label: "Net R&D", Value: 650, Type: WaterfallTypeTotal},
		}

		min, max := wc.calculateDomain(points)

		// Running totals: 2100, 1700, 1400, 850, 700, 650.
		// Span = 2100-650 = 1450. Min=650 <= 1450 → includes zero.
		if min > 0 {
			t.Errorf("min should be 0 for pharma bridge (got %v) — data is close enough to 0", min)
		}
		if max < 2100 {
			t.Errorf("max should be >= 2100 (highest value), got %v", max)
		}
	})

	t.Run("data crossing zero always includes zero", func(t *testing.T) {
		points := []WaterfallDataPoint{
			{Label: "Start", Value: 500, Type: WaterfallTypeTotal},
			{Label: "Big Loss", Value: -600, Type: WaterfallTypeDecrease},
			{Label: "Recovery", Value: 200, Type: WaterfallTypeIncrease},
			{Label: "End", Value: 100, Type: WaterfallTypeTotal},
		}

		min, max := wc.calculateDomain(points)

		// Running totals: 500, -100, 100, 100. Crosses zero.
		if min > -100 {
			t.Errorf("min should be <= -100 (running total goes negative), got %v", min)
		}
		if max < 500 {
			t.Errorf("max should be >= 500, got %v", max)
		}
	})

	t.Run("all decreases from high baseline", func(t *testing.T) {
		points := []WaterfallDataPoint{
			{Label: "Starting Value", Value: 1000, Type: WaterfallTypeTotal},
			{Label: "Fee A", Value: -5, Type: WaterfallTypeDecrease},
			{Label: "Fee B", Value: -3, Type: WaterfallTypeDecrease},
			{Label: "Fee C", Value: -2, Type: WaterfallTypeDecrease},
			{Label: "Net Value", Value: 990, Type: WaterfallTypeTotal},
		}

		min, max := wc.calculateDomain(points)

		// Running totals: 1000, 995, 992, 990, 990. Span=10. Min=990 >> 10 → broken axis.
		if min <= 0 {
			t.Errorf("min should be > 0 for high-baseline decreases (got %v)", min)
		}
		if min >= 990 {
			t.Errorf("min should be < 990 (lowest value), got %v", min)
		}
		if max <= 1000 {
			t.Errorf("max should be > 1000 (highest value), got %v", max)
		}
	})

	t.Run("single total bar", func(t *testing.T) {
		points := []WaterfallDataPoint{
			{Label: "Total", Value: 500, Type: WaterfallTypeTotal},
		}

		min, max := wc.calculateDomain(points)

		// Single point: span=0, should add padding around value
		if min >= 500 {
			t.Errorf("min should be < 500 for single point, got %v", min)
		}
		if max <= 500 {
			t.Errorf("max should be > 500 for single point, got %v", max)
		}
	})
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
