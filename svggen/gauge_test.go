package svggen

import (
	"testing"
)

func TestGaugeChart_Draw(t *testing.T) {
	tests := []struct {
		name    string
		config  GaugeChartConfig
		data    GaugeData
		wantErr bool
	}{
		{
			name:   "basic gauge",
			config: DefaultGaugeChartConfig(800, 600),
			data: GaugeData{
				Title: "Performance Score",
				Value: 75,
				Unit:  "%",
			},
			wantErr: false,
		},
		{
			name: "gauge with thresholds",
			config: func() GaugeChartConfig {
				c := DefaultGaugeChartConfig(800, 600)
				c.Thresholds = []GaugeThreshold{
					{Value: 33, Color: MustParseColor("#E15759")},
					{Value: 66, Color: MustParseColor("#EDC948")},
					{Value: 100, Color: MustParseColor("#59A14F")},
				}
				return c
			}(),
			data: GaugeData{
				Title: "Health Score",
				Value: 85,
			},
			wantErr: false,
		},
		{
			name: "gauge with custom range",
			config: func() GaugeChartConfig {
				c := DefaultGaugeChartConfig(800, 600)
				c.MinValue = -50
				c.MaxValue = 50
				return c
			}(),
			data: GaugeData{
				Title: "Temperature",
				Value: 15,
				Unit:  "°C",
			},
			wantErr: false,
		},
		{
			name: "gauge value at minimum",
			config: func() GaugeChartConfig {
				c := DefaultGaugeChartConfig(800, 600)
				c.MinValue = 0
				c.MaxValue = 100
				return c
			}(),
			data: GaugeData{
				Title: "Progress",
				Value: 0,
				Unit:  "%",
			},
			wantErr: false,
		},
		{
			name: "gauge value at maximum",
			config: func() GaugeChartConfig {
				c := DefaultGaugeChartConfig(800, 600)
				c.MinValue = 0
				c.MaxValue = 100
				return c
			}(),
			data: GaugeData{
				Title: "Progress",
				Value: 100,
				Unit:  "%",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSVGBuilder(800, 600)
			chart := NewGaugeChart(builder, tt.config)
			err := chart.Draw(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("GaugeChart.Draw() error = %v, wantErr %v", err, tt.wantErr)
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

func TestGaugeDiagram_Validate(t *testing.T) {
	diagram := &GaugeDiagram{NewBaseDiagram("gauge_chart")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid with value",
			req: &RequestEnvelope{
				Type: "gauge_chart",
				Data: map[string]any{
					"value": 75.0,
				},
			},
			wantErr: false,
		},
		{
			name: "valid with value and range",
			req: &RequestEnvelope{
				Type: "gauge_chart",
				Data: map[string]any{
					"value": 50.0,
					"min":   0.0,
					"max":   100.0,
				},
			},
			wantErr: false,
		},
		{
			name: "missing data",
			req: &RequestEnvelope{
				Type: "gauge_chart",
				Data: nil,
			},
			wantErr: true,
		},
		{
			name: "missing value",
			req: &RequestEnvelope{
				Type: "gauge_chart",
				Data: map[string]any{
					"min": 0.0,
					"max": 100.0,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagram.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GaugeDiagram.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGaugeDiagram_Render(t *testing.T) {
	diagram := &GaugeDiagram{NewBaseDiagram("gauge_chart")}

	req := &RequestEnvelope{
		Type:  "gauge_chart",
		Title: "Speed",
		Data: map[string]any{
			"value": 65.0,
			"min":   0.0,
			"max":   120.0,
			"unit":  "mph",
			"thresholds": []any{
				map[string]any{"value": 40, "color": "#59A14F"},
				map[string]any{"value": 80, "color": "#EDC948"},
				map[string]any{"value": 120, "color": "#E15759"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowValues: true},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("GaugeDiagram.Render() error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	// Gauge charts fill the full container (no forced 1:1 square).
	// With 800x600 container at scale, doc dimensions are 1067x800.
	if doc.Width != 1067 || doc.Height != 800 {
		t.Errorf("Expected dimensions 1067x800, got %vx%v", doc.Width, doc.Height)
	}

	// Container dimensions should be stored (pre-scale values)
	if doc.ContainerWidth != 800 || doc.ContainerHeight != 600 {
		t.Errorf("Expected container dimensions 800x600, got %vx%v", doc.ContainerWidth, doc.ContainerHeight)
	}

	// FitMode should be auto-applied to "contain"
	if doc.FitMode != "contain" {
		t.Errorf("Expected FitMode 'contain' (auto-applied for all charts), got %q", doc.FitMode)
	}
}

func TestGaugeDiagram_AutoDetectFractionalScale(t *testing.T) {
	diagram := &GaugeDiagram{NewBaseDiagram("gauge_chart")}

	// When value is in [0,1] and no explicit min/max is provided,
	// the gauge should auto-detect a 0-1 scale instead of 0-100.
	// This prevents fractional values like 0.73 from rendering near zero.
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "value 0.73 without min/max",
			data: map[string]any{"value": 0.73},
		},
		{
			name: "value 0.92 without min/max",
			data: map[string]any{"value": 0.92},
		},
		{
			name: "value 1.0 without min/max",
			data: map[string]any{"value": 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:   "gauge_chart",
				Title:  "Test Gauge",
				Data:   tt.data,
				Output: OutputSpec{Width: 800, Height: 600},
			}

			doc, err := diagram.Render(req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if doc == nil || len(doc.Content) == 0 {
				t.Fatal("Expected non-empty SVG document")
			}
		})
	}
}

func TestGaugeDiagram_ExplicitMinMaxPreserved(t *testing.T) {
	diagram := &GaugeDiagram{NewBaseDiagram("gauge_chart")}

	// When explicit min/max are provided, they should be used even for [0,1] values.
	req := &RequestEnvelope{
		Type:  "gauge_chart",
		Title: "Test Gauge",
		Data: map[string]any{
			"value": 0.5,
			"min":   0.0,
			"max":   100.0,
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Fatal("Expected non-empty SVG document")
	}
}

func TestDefaultGaugeNeedleStyle(t *testing.T) {
	style := DefaultGaugeNeedleStyle()

	if style.Width <= 0 {
		t.Error("Expected positive needle width")
	}
	if style.Length <= 0 || style.Length > 1 {
		t.Error("Expected needle length between 0 and 1")
	}
	if !style.ShowPivot {
		t.Error("Expected ShowPivot to be true by default")
	}
	if style.PivotRadius <= 0 {
		t.Error("Expected positive pivot radius")
	}
}
