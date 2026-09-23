package main

import "testing"

func TestClampedRenderDensity(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		args                  map[string]any
		fallback, min, maxDPI int
		want                  int
	}{
		{"slide default", nil, 100, 50, 300, 100},
		{"slide low", map[string]any{"density": float64(10)}, 100, 50, 300, 50},
		{"slide high", map[string]any{"density": float64(9999)}, 100, 50, 300, 300},
		{"slide normal", map[string]any{"density": float64(144)}, 100, 50, 300, 144},
		{"slide wrong type", map[string]any{"density": "high"}, 100, 50, 300, 100},
		{"thumbnail default", nil, 50, 25, 150, 50},
		{"thumbnail low", map[string]any{"density": float64(1)}, 50, 25, 150, 25},
		{"thumbnail high", map[string]any{"density": float64(9999)}, 50, 25, 150, 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampedRenderDensity(tc.args, tc.fallback, tc.min, tc.maxDPI); got != tc.want {
				t.Errorf("density = %d, want %d", got, tc.want)
			}
		})
	}
}
