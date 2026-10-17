package patterns

import "testing"

// Exercise discovery thresholds against the actual layout decision, so these
// are useful placement instructions rather than merely duplicated numbers.
func TestSegmentFitMatchesLayoutTransitions(t *testing.T) {
	t.Run("figure-eight hard minimum", func(t *testing.T) {
		p := &cycleFigureEight{}
		fit := SegmentFitFor(p)
		for _, tc := range []struct {
			w, h    float64
			refused bool
		}{
			{fit.MinContentWidthPt, fit.MinContentHeightPt, false},
			{fit.MinContentWidthPt - 2, fit.MinContentHeightPt, true},
			{fit.MinContentWidthPt, fit.MinContentHeightPt - 2, true},
		} {
			_, err := p.Expand(cycleRingCtx(tc.w, tc.h), cfeValues(4, 0), nil, nil)
			if (err != nil) != tc.refused {
				t.Errorf("%.0f x %.0f: error %v, want refused %v", tc.w, tc.h, err, tc.refused)
			}
		}
	})
	t.Run("ring default transition and explicit mode", func(t *testing.T) {
		fit := SegmentFitFor(&cycleRing{})
		for _, tc := range []struct {
			w      float64
			labels string
			legend bool
		}{
			{fit.FallbackBelowWidthPt - 1, "", true},
			{fit.FallbackBelowWidthPt, "", false},
			{fit.FallbackBelowWidthPt - 1, cycleRingLabelsOutside, false},
		} {
			lay, err := cycleRingMeasure(cycleRingCtx(tc.w, 400), cycleRingValues(4), &CycleRingOverrides{Labels: tc.labels})
			if err != nil {
				t.Fatal(err)
			}
			if lay.legend != tc.legend {
				t.Errorf("width %v labels %q: legend %v, want %v", tc.w, tc.labels, lay.legend, tc.legend)
			}
		}
	})
	t.Run("node label minimum is not a segment minimum", func(t *testing.T) {
		fit := SegmentFitFor(&cycleNodes{})
		if fit.MinContentWidthPt != 0 || fit.FallbackBelowWidthPt != 0 {
			t.Fatal("geometry-dependent transition must not claim a fixed segment threshold")
		}
		const w, gap = 480.0, 10.0
		for _, h := range []float64{100, 400} {
			labelW := (w-cycleNodesRingSide(w, h, cycleNodesRingWidthFrac))/2 - gap - cycleNodesCueColPt
			got := cycleNodesMode(&CycleNodesOverrides{}, w, h, gap)
			if (got == fit.NarrowFallback) != (labelW < fit.MinLabelColumnWidthPt) {
				t.Errorf("height %v: mode %q contradicts label minimum %+v", h, got, fit)
			}
		}
	})
}
