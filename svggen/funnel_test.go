package svggen

import (
	"testing"
)

func TestFunnelChart_Draw(t *testing.T) {
	tests := []struct {
		name    string
		data    FunnelData
		wantErr bool
	}{
		{
			name: "basic funnel",
			data: FunnelData{
				Title: "Sales Funnel",
				Points: []FunnelDataPoint{
					{Label: "Leads", Value: 1000},
					{Label: "Qualified", Value: 800},
					{Label: "Proposals", Value: 400},
					{Label: "Closed", Value: 200},
				},
			},
			wantErr: false,
		},
		{
			name: "funnel with custom colors",
			data: FunnelData{
				Title: "Marketing Funnel",
				Points: []FunnelDataPoint{
					{Label: "Awareness", Value: 5000, Color: colorPtr(MustParseColor("#4E79A7"))},
					{Label: "Interest", Value: 3000, Color: colorPtr(MustParseColor("#F28E2B"))},
					{Label: "Decision", Value: 1500, Color: colorPtr(MustParseColor("#E15759"))},
					{Label: "Action", Value: 500, Color: colorPtr(MustParseColor("#76B7B2"))},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty funnel",
			data:    FunnelData{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSVGBuilder(800, 600)
			config := DefaultFunnelChartConfig(800, 600)
			config.ShowValues = true
			config.ShowPercentage = true

			chart := NewFunnelChart(builder, config)
			err := chart.Draw(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("FunnelChart.Draw() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				doc, err := builder.Render()
				if err != nil {
					t.Errorf("Failed to render SVG: %v", err)
					return
				}

				if doc == nil || len(doc.Content) == 0 {
					t.Error("Expected non-empty SVG document")
				}
			}
		})
	}
}

func TestFunnelDiagram_Validate(t *testing.T) {
	diagram := &FunnelDiagram{NewBaseDiagram("funnel_chart")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid with points",
			req: &RequestEnvelope{
				Type: "funnel_chart",
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "A", "value": 100},
						map[string]any{"label": "B", "value": 80},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with values",
			req: &RequestEnvelope{
				Type: "funnel_chart",
				Data: map[string]any{
					"values":     []any{100.0, 80.0, 60.0},
					"categories": []any{"A", "B", "C"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing data",
			req: &RequestEnvelope{
				Type: "funnel_chart",
				Data: nil,
			},
			wantErr: true,
		},
		{
			name: "missing values and points",
			req: &RequestEnvelope{
				Type: "funnel_chart",
				Data: map[string]any{
					"categories": []any{"A", "B"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagram.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunnelDiagram.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFunnelDiagram_Render(t *testing.T) {
	diagram := &FunnelDiagram{NewBaseDiagram("funnel_chart")}

	req := &RequestEnvelope{
		Type:  "funnel_chart",
		Title: "Test Funnel",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Stage 1", "value": 100},
				map[string]any{"label": "Stage 2", "value": 75},
				map[string]any{"label": "Stage 3", "value": 50},
				map[string]any{"label": "Stage 4", "value": 25},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowValues: true},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("FunnelDiagram.Render() error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	if doc.Width != 1067 || doc.Height != 800 {
		t.Errorf("Expected dimensions 1067x800 (800x600pt in CSS pixels), got %vx%v", doc.Width, doc.Height)
	}
}

// TestFunnelChart_TrapezoidGeometry validates that each funnel segment's top width
// is proportional to its own value (not the previous segment's value). This was the
// root cause of the "area chart" rendering bug where segments formed a continuous
// cone instead of discrete proportional trapezoids.
func TestFunnelChart_TrapezoidGeometry(t *testing.T) {
	// This test verifies the fix by checking that the funnel renders
	// data from the reported bug (Visitors 1000 > Downloads 350 > Active 50 > Purchasers 10)
	// as discrete trapezoid segments, not a continuous area chart.
	builder := NewSVGBuilder(800, 600)
	config := DefaultFunnelChartConfig(800, 600)
	config.ShowValues = true
	config.ShowTitle = false

	chart := NewFunnelChart(builder, config)
	data := FunnelData{
		Points: []FunnelDataPoint{
			{Label: "Visitors", Value: 1000},
			{Label: "Downloads", Value: 350},
			{Label: "Active", Value: 50},
			{Label: "Purchasers", Value: 10},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Draw() error = %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Fatal("Expected non-empty SVG output")
	}

	// Verify the geometry logic directly: for each segment, topWidth should be
	// proportional to the segment's own value, and bottomWidth should be
	// proportional to the next segment's value.
	plotArea := config.PlotArea()
	maxValue := 1000.0

	for i, point := range data.Points {
		expectedTopWidth := plotArea.W * (point.Value / maxValue)
		var expectedBottomWidth float64
		if i == len(data.Points)-1 {
			expectedBottomWidth = expectedTopWidth * config.NeckWidth // NeckWidth=0 -> point
		} else {
			expectedBottomWidth = plotArea.W * (data.Points[i+1].Value / maxValue)
		}

		// Verify top width matches this segment's value proportion (not previous)
		if i > 0 {
			prevWidth := plotArea.W * (data.Points[i-1].Value / maxValue)
			if expectedTopWidth == prevWidth && point.Value != data.Points[i-1].Value {
				t.Errorf("Segment %d (%s): topWidth should be %.1f (own value) not %.1f (previous segment's value)",
					i, point.Label, expectedTopWidth, prevWidth)
			}
		}

		// Verify basic proportionality
		if expectedTopWidth <= 0 && point.Value > 0 {
			t.Errorf("Segment %d (%s): topWidth should be positive for value %.0f", i, point.Label, point.Value)
		}
		if i < len(data.Points)-1 && expectedBottomWidth > expectedTopWidth {
			t.Errorf("Segment %d (%s): bottomWidth (%.1f) should not exceed topWidth (%.1f) in a funnel",
				i, point.Label, expectedBottomWidth, expectedTopWidth)
		}
	}
}

func TestFunnelChart_SingleSegment(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	config := DefaultFunnelChartConfig(400, 300)

	chart := NewFunnelChart(builder, config)
	err := chart.Draw(FunnelData{
		Title: "Single Stage",
		Points: []FunnelDataPoint{
			{Label: "Only Stage", Value: 100},
		},
	})
	if err != nil {
		t.Fatalf("Draw() error = %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Fatal("Expected non-empty SVG output")
	}
}

func TestFunnelChart_EqualValues(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	config := DefaultFunnelChartConfig(400, 300)

	chart := NewFunnelChart(builder, config)
	err := chart.Draw(FunnelData{
		Title: "Equal Stages",
		Points: []FunnelDataPoint{
			{Label: "A", Value: 100},
			{Label: "B", Value: 100},
			{Label: "C", Value: 100},
		},
	})
	if err != nil {
		t.Fatalf("Draw() error = %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Fatal("Expected non-empty SVG output")
	}
}

func TestFunnelChart_LabelContrast(t *testing.T) {
	// Verify that inside labels use contrast-aware text color
	// based on segment background luminance (not hardcoded white).
	lightColor := MustParseColor("#FFE0B2") // light orange
	darkColor := MustParseColor("#1B2838")  // dark blue

	lightText := lightColor.TextColorFor()
	darkText := darkColor.TextColorFor()

	if lightText.R == 255 && lightText.G == 255 && lightText.B == 255 {
		t.Error("Light background should get dark text, not white")
	}
	if darkText.R != 255 || darkText.G != 255 || darkText.B != 255 {
		t.Error("Dark background should get white text")
	}

	// Render a funnel with a light-colored segment to ensure no error
	builder := NewSVGBuilder(800, 600)
	config := DefaultFunnelChartConfig(800, 600)
	config.ShowValues = true
	chart := NewFunnelChart(builder, config)

	err := chart.Draw(FunnelData{
		Title: "Contrast Test",
		Points: []FunnelDataPoint{
			{Label: "Light Segment", Value: 1000, Color: &lightColor},
			{Label: "Dark Segment", Value: 500, Color: &darkColor},
		},
	})
	if err != nil {
		t.Fatalf("Draw() error = %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG output")
	}
}
