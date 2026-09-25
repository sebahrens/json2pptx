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

// go-slide-creator-s1uvj.32: a threshold beyond max (or below min) mapped to a
// ratio outside [0,1], so its zone swept past the end of the dial and wrapped
// round through the gap at the bottom of the gauge. Zones must stay inside the
// dial: ratios clamp to [0,1], and the dial stops at max.
func TestGaugeThresholdZones_ClampToDial(t *testing.T) {
	cases := []struct {
		name       string
		thresholds []float64
	}{
		{"threshold beyond max", []float64{50, 150}},
		{"thresholds beyond max continue", []float64{50, 150, 200}},
		{"threshold below min", []float64{-50, 60}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultGaugeChartConfig(400, 300)
			cfg.MinValue, cfg.MaxValue = 0, 100
			for _, v := range tc.thresholds {
				cfg.Thresholds = append(cfg.Thresholds, GaugeThreshold{Value: v, Color: MustParseColor("#AA0000")})
			}
			zones := NewGaugeChart(NewSVGBuilder(400, 300), cfg).thresholdZones()
			if len(zones) == 0 {
				t.Fatal("no zones")
			}
			prevEnd := 0.0
			for i, z := range zones {
				if z.startRatio < 0 || z.endRatio > 1 || z.endRatio <= z.startRatio {
					t.Errorf("zone %d = [%v, %v], want a non-empty span inside [0, 1]", i, z.startRatio, z.endRatio)
				}
				if z.startRatio < prevEnd {
					t.Errorf("zone %d starts at %v, overlapping the previous zone ending at %v", i, z.startRatio, prevEnd)
				}
				prevEnd = z.endRatio
			}
			if prevEnd != 1 {
				t.Errorf("zones end at %v, want the dial filled to 1", prevEnd)
			}
		})
	}

	// End to end: the gauge still renders without error or NaN.
	doc, err := Render(&RequestEnvelope{
		Type: "gauge_chart",
		Data: map[string]any{
			"value": 65.0, "min": 0.0, "max": 100.0,
			"thresholds": []any{
				map[string]any{"value": 50.0, "color": "#59A14F"},
				map[string]any{"value": 150.0, "color": "#E15759"},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(doc.Content), "NaN") {
		t.Error("rendered gauge contains NaN")
	}
}
