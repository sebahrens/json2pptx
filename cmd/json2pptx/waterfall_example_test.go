package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestBundledWaterfallExamplesHaveCorrectBalances(t *testing.T) {
	tests := []struct {
		file string
		want []float64
	}{
		{"diagrams.json", []float64{21.3, 14.6, 14.6, 4.5, 4.5}},
		{"full-showcase.json", []float64{2.0, 3.2, 3.7, 4.0, 4.2, 4.2}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join("..", "..", "examples", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			var input PresentationInput
			if err := json.Unmarshal(contents, &input); err != nil {
				t.Fatal(err)
			}
			var points []any
			for _, slide := range input.Slides {
				for _, item := range slide.Content {
					if item.ChartValue != nil && item.ChartValue.Type == "waterfall" {
						if points != nil {
							t.Fatal("example has more than one waterfall chart")
						}
						points, _ = item.ChartValue.Data["points"].([]any)
					}
				}
			}
			if len(points) != len(tt.want) {
				t.Fatalf("got %d waterfall points, want %d", len(points), len(tt.want))
			}
			var balance float64
			for i, raw := range points {
				point, ok := raw.(map[string]any)
				if !ok {
					t.Fatalf("point %d has type %T", i, raw)
				}
				value, ok := point["value"].(float64)
				if !ok {
					t.Fatalf("point %d has non-numeric value %v", i, point["value"])
				}
				switch point["type"] {
				case "total", "subtotal":
					balance = value
				case "increase", "decrease":
					balance += value
				default:
					t.Fatalf("point %d has unsupported type %v", i, point["type"])
				}
				if math.Abs(balance-tt.want[i]) > 1e-9 {
					t.Errorf("point %d (%v) balance = %.2f, want %.2f", i, point["label"], balance, tt.want[i])
				}
			}
		})
	}
}
