package generator

import (
	"testing"
)

// The capability entries make claims an agent budgets against, and some of them
// were false: timeline said it errors past 7 stops (14 render fine), and
// porters accepted an intensity outside the 0.0-1.0 its own hint documents and
// printed "High (150%)". The strings now describe what the engine does, and
// these tests are what keeps them describing it (go-slide-creator-umji).

// porterIntensityClamp: an out-of-range intensity used to reach the label,
// which printed "High (150%)" and "Low (-20%)" from a field whose own hint says
// 0.0-1.0.
func TestPorterIntensityIsClampedToItsDocumentedRange(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{1.5, 1.0},
		{-0.2, 0.0},
		{0.0, 0.0},
		{1.0, 1.0},
		{0.42, 0.42},
	}
	for _, c := range cases {
		if got := clampPorterIntensity(c.in); got != c.want {
			t.Errorf("clampPorterIntensity(%v) = %v, want %v", c.in, got, c.want)
		}
	}

	// End to end through the force parser, which is where the value enters.
	force := porterForceFromMap(porterForceType("rivalry"), map[string]any{"intensity": 1.5})
	if force.intensity != 1.0 {
		t.Errorf("parsed intensity = %v, want 1.0", force.intensity)
	}
	if label := porterIntensityLabel(force.intensity); label != "High" {
		t.Errorf("label = %q, want High", label)
	}
}

// kpi_dashboard's max_nodes IS enforced — the review reported otherwise, but
// the cap and its CONTENT_DROPPED finding landed before this bead was picked
// up (TestProcessKPIDashboard_EnforcesMaxMetrics covers the finding). The
// capability text says "enforced" now, so pin the grid it is enforced to.
func TestKPIDashboardCapacityMatchesItsGrid(t *testing.T) {
	cols, rows := kpiGridLayout(kpiMaxMetrics)
	if cols*rows < kpiMaxMetrics {
		t.Errorf("kpiGridLayout(%d) = %dx%d = %d cells, too few for the documented capacity",
			kpiMaxMetrics, cols, rows, cols*rows)
	}
	if parsed := parseKPIMetrics(map[string]any{"metrics": kpiMetricList(kpiMaxMetrics)}); len(parsed) != kpiMaxMetrics {
		t.Errorf("parsed %d metrics of %d at exactly the capacity", len(parsed), kpiMaxMetrics)
	}
}

// kpiMetricList builds n metric payloads.
func kpiMetricList(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{"label": "Metric", "value": "42"}
	}
	return out
}
