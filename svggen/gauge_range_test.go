package svggen

import (
	"strings"
	"testing"
)

// go-slide-creator-s1uvj.31: a gauge with min == max divided by a zero range,
// so drawNeedle, drawTicks and drawThresholdZones wrote NaN coordinates into
// the SVG ("MNaNNaN"). An empty or inverted range cannot be drawn and must be
// rejected up front with an error naming min and max.
func TestGaugeDiagram_RejectsEmptyOrInvertedRange(t *testing.T) {
	diagram := &GaugeDiagram{NewBaseDiagram("gauge_chart")}
	cases := []struct {
		name string
		data map[string]any
	}{
		{"min equals max", map[string]any{"value": 50.0, "min": 50.0, "max": 50.0}},
		{"min above max", map[string]any{"value": 50.0, "min": 100.0, "max": 0.0}},
		{"min above default max", map[string]any{"value": 150.0, "min": 150.0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &RequestEnvelope{Type: "gauge_chart", Data: tc.data, Output: OutputSpec{Width: 400, Height: 300}}
			err := diagram.Validate(req)
			if err == nil {
				t.Fatal("Validate accepted a gauge whose max is not greater than min")
			}
			if !strings.Contains(err.Error(), "max") || !strings.Contains(err.Error(), "min") {
				t.Errorf("error %q should name min and max", err)
			}
			if _, err := Render(req); err == nil {
				t.Error("Render accepted a gauge whose max is not greater than min")
			}
		})
	}
}

func TestGaugeChart_DrawRejectsEmptyRange(t *testing.T) {
	cfg := DefaultGaugeChartConfig(400, 300)
	cfg.MinValue, cfg.MaxValue = 50, 50
	chart := NewGaugeChart(NewSVGBuilder(400, 300), cfg)
	if err := chart.Draw(GaugeData{Value: 50}); err == nil {
		t.Fatal("Draw accepted MinValue == MaxValue; the zero range yields NaN geometry")
	}
}
