package patterns

import "testing"

// TestPieOverloadedAboveFiveCategories covers go-slide-creator-ihlsr:
// recommend_visual stops ranking a pie / donut once it would carry more than
// five slices, where a sorted bar chart reads better.
func TestPieOverloadedAboveFiveCategories(t *testing.T) {
	for _, name := range []string{"pie", "donut"} {
		if chartOverloaded(name, &VisualHints{DataPoints: 5, SeriesCount: 1}, "market share") {
			t.Errorf("%s with 5 slices should still be recommendable", name)
		}
		if !chartOverloaded(name, &VisualHints{DataPoints: 6, SeriesCount: 1}, "market share") {
			t.Errorf("%s with 6 slices should be demoted below the bar chart", name)
		}
	}
}
