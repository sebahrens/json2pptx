package svggen

import "testing"

// TestWithBarTopHeadroom_ExpandsPositiveMax verifies the headroom helper only
// expands positive maxima and leaves non-positive (negative-only / all-zero)
// domains untouched so Nice() can still anchor the axis sensibly.
func TestWithBarTopHeadroom_ExpandsPositiveMax(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"positive", 100, 100 * barTopHeadroomFactor},
		{"zero", 0, 0},
		{"negative", -50, -50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withBarTopHeadroom(tc.in); got != tc.want {
				t.Errorf("withBarTopHeadroom(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestBarTopHeadroom_ClearsTickBoundaryCeiling is the regression guard for
// go-slide-creator-xv1k: when the data max lands exactly on a nice tick
// boundary, plain Nice() rounding adds no headroom and the tallest bar (plus
// its top-anchored value/direct label) crowds the plot ceiling. Applying the
// headroom factor before Nice() must leave the nice domain max strictly above
// the data max so the bar top sits below the ceiling.
func TestBarTopHeadroom_ClearsTickBoundaryCeiling(t *testing.T) {
	// 100 with a 5-tick nice step of 20 lands exactly on a boundary.
	const dataMax = 100.0

	plain := NewLinearScale(0, dataMax)
	plain.Nice(true)
	if _, plainMax := plain.DomainBounds(); plainMax > dataMax {
		t.Skipf("baseline assumption broken: Nice() already adds headroom (max=%.2f) — test no longer meaningful", plainMax)
	}

	withRoom := NewLinearScale(0, withBarTopHeadroom(dataMax))
	withRoom.Nice(true)
	_, niceMax := withRoom.DomainBounds()
	if niceMax <= dataMax {
		t.Errorf("nice domain max = %.2f, want > %.2f so the tallest bar clears the plot ceiling", niceMax, dataMax)
	}
}
