package svggen

import (
	"math"
	"regexp"
	"strconv"
	"testing"
)

// TestStackLineEndLabels covers the de-collision arithmetic: labels are pushed
// apart to at least one line height, the stack stays inside the plot, and an
// impossible stack is refused so the caller can fall back to a legend
// (go-slide-creator-lntx).
func TestStackLineEndLabels(t *testing.T) {
	plot := Rect{X: 0, Y: 0, W: 400, H: 200}
	const lineH = 12.0

	t.Run("converging series are pushed apart", func(t *testing.T) {
		labels := []*lineEndLabel{
			{name: "a", y: 100},
			{name: "b", y: 101},
			{name: "c", y: 101},
		}
		if !stackLineEndLabels(labels, plot, lineH) {
			t.Fatal("three labels fit in a 200pt plot")
		}
		for i := 1; i < len(labels); i++ {
			gap := labels[i].labelY - labels[i-1].labelY
			if gap < lineH-0.01 {
				t.Errorf("labels %d and %d are %.1fpt apart, want >= %.0f", i-1, i, gap, lineH)
			}
		}
	})

	t.Run("well-separated labels are left alone", func(t *testing.T) {
		labels := []*lineEndLabel{{name: "a", y: 40}, {name: "b", y: 120}}
		if !stackLineEndLabels(labels, plot, lineH) {
			t.Fatal("two labels fit")
		}
		if labels[0].labelY != 40 || labels[1].labelY != 120 {
			t.Errorf("labels moved without needing to: %.0f, %.0f", labels[0].labelY, labels[1].labelY)
		}
	})

	t.Run("a stack overrunning the bottom is pushed back up", func(t *testing.T) {
		labels := []*lineEndLabel{{name: "a", y: 190}, {name: "b", y: 195}, {name: "c", y: 199}}
		if !stackLineEndLabels(labels, plot, lineH) {
			t.Fatal("three labels fit in a 200pt plot")
		}
		for i, l := range labels {
			if l.labelY > plot.Y+plot.H {
				t.Errorf("label %d at %.1f is below the plot", i, l.labelY)
			}
		}
		for i := 1; i < len(labels); i++ {
			if gap := labels[i].labelY - labels[i-1].labelY; gap < lineH-0.01 {
				t.Errorf("labels %d and %d are %.1fpt apart after the upward pass", i-1, i, gap)
			}
		}
	})

	t.Run("too many labels for the plot are refused", func(t *testing.T) {
		short := Rect{X: 0, Y: 0, W: 400, H: 20}
		labels := []*lineEndLabel{{y: 5}, {y: 6}, {y: 7}}
		if stackLineEndLabels(labels, short, lineH) {
			t.Error("three 12pt labels cannot fit a 20pt plot — the caller must draw a legend")
		}
	})
}

// TestLineChartConvergingSeriesLabelsDoNotOverlap is the end-to-end check the
// review asked for: three series ending within 2% of each other must produce
// non-overlapping label positions, or no direct labels at all.
func TestLineChartConvergingSeriesLabelsDoNotOverlap(t *testing.T) {
	req := &RequestEnvelope{
		Type:   "line_chart",
		Title:  "Share of workloads (%)",
		Output: OutputSpec{Width: 900, Height: 500},
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3", "Q4"},
			"series": []any{
				map[string]any{"name": "Real-time platform", "values": []any{12.0, 25.0, 36.0, 43.0}},
				map[string]any{"name": "Hybrid streaming layer", "values": []any{20.0, 30.0, 38.0, 44.0}},
				map[string]any{"name": "Batch reporting pipeline", "values": []any{68.0, 45.0, 44.0, 44.0}},
			},
		},
	}
	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Collect the y positions of the three series labels.
	svg := string(doc.Content)
	tspan := regexp.MustCompile(`<tspan x="([\d.-]+)" y="([\d.-]+)"[^>]*>([^<]*)</tspan>`)
	ys := map[string]float64{}
	for _, m := range tspan.FindAllStringSubmatch(svg, -1) {
		switch m[3] {
		case "Real-time platform", "Hybrid streaming layer", "Batch reporting pipeline":
			y, _ := strconv.ParseFloat(m[2], 64)
			ys[m[3]] = y
		}
	}
	if len(ys) == 0 {
		return // direct labels suppressed in favour of a legend — also acceptable
	}
	if len(ys) != 3 {
		t.Fatalf("expected all three series labelled or none, got %d: %v", len(ys), ys)
	}
	for a, ya := range ys {
		for bName, yb := range ys {
			if a >= bName {
				continue
			}
			if math.Abs(ya-yb) < 8 {
				t.Errorf("labels %q and %q are %.1f apart — they overprint", a, bName, math.Abs(ya-yb))
			}
		}
	}
}
