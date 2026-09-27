package patterns

import "testing"

// A bridge of small deltas between close totals printed "$2, +$0, … $2" at a
// fixed one decimal (go-slide-creator-csclk.88).
func TestLabelDecimals_KeepsSmallAndCloseValuesDistinct(t *testing.T) {
	values := []float64{1.97, 0.04, 0.05, -0.06, 0.01, 0.02, 2.03}
	d := LabelDecimals(values)
	if d != 2 {
		t.Fatalf("LabelDecimals = %d, want 2", d)
	}
	want := []string{"$1.97", "+$0.04", "+$0.05", "−$0.06", "+$0.01", "+$0.02", "$2.03"}
	for i, v := range values {
		signed := i != 0 && i != len(values)-1
		if got := FormatMagnitudeLabelDecimals(v, "$", signed, d); got != want[i] {
			t.Errorf("label(%v) = %q, want %q", v, got, want[i])
		}
	}
	// Ordinary values keep one decimal.
	if d := LabelDecimals([]float64{96.44, 12, 1240.5}); d != 1 {
		t.Errorf("LabelDecimals(ordinary) = %d, want 1", d)
	}
	// A lone small value is never printed as zero.
	if got := FormatMagnitudeLabel(0.04, "", true); got != "+0.04" {
		t.Errorf("FormatMagnitudeLabel(0.04) = %q, want +0.04", got)
	}
}
