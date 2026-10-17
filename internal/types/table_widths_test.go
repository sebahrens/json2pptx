package types

import (
	"math"
	"testing"
)

func TestResolveAuthoredTableColumnWidths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		weights []float64
		valid   bool
	}{
		{"relative", []float64{1, 3}, true}, {"extreme finite", []float64{math.SmallestNonzeroFloat64, math.MaxFloat64}, true},
		{"nan", []float64{1, math.NaN()}, false}, {"inf", []float64{1, math.Inf(1)}, false}, {"negative", []float64{-1, 3}, false},
		{"count", []float64{1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			widths := ResolveTableColumnWidths(tc.weights, 2, 1000000)
			if !tc.valid {
				if widths != nil {
					t.Fatalf("accepted %v", tc.weights)
				}
				return
			}
			if len(widths) != 2 || widths[0] < 1 || widths[1] < 1 || widths[0]+widths[1] != 1000000 {
				t.Fatalf("invalid native grid widths %v", widths)
			}
		})
	}
}
