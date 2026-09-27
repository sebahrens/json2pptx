package shapegrid

import (
	"math"
	"testing"
)

// An over-committed grid keeps rows held at their point min_height and takes
// the deficit from the others (go-slide-creator-n1muf): the pinned 22pt date
// row of a phase roadmap was otherwise scaled to 21.5pt and its labels stored
// below the body floor.
func TestResolveRowHeightsReservesMinHeightRowsWhenOvercommitted(t *testing.T) {
	avail := int64(270 * 12700)
	rows := []Row{
		{Height: 60},
		{MinHeight: 22, MaxHeight: 22},
		{MinHeight: 94, MaxHeight: 94},
	}
	heights := resolveRowHeights(rows, avail)
	sum := 0.0
	for _, h := range heights {
		sum += h
	}
	if math.Abs(sum-100) > 1e-6 {
		t.Fatalf("heights sum to %.3f%%, want 100", sum)
	}
	for i, pt := range []float64{22, 94} {
		if got := heights[i+1] / 100 * 270; math.Abs(got-pt) > 1e-6 {
			t.Errorf("pinned row %d = %.2fpt, want %.0fpt", i+1, got, pt)
		}
	}
	if got := heights[0] / 100 * 270; got >= 60.0/100*270 {
		t.Errorf("percentage row did not give way: %.2fpt", got)
	}

	// A grid that is not over-committed is unchanged.
	fitting := resolveRowHeights([]Row{{Height: 40}, {MinHeight: 22, MaxHeight: 22}}, avail)
	if math.Abs(fitting[0]-40) > 1e-9 {
		t.Errorf("fitting grid changed: %v", fitting)
	}
}
