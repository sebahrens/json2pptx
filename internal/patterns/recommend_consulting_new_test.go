package patterns

import (
	"strings"
	"testing"
)

func TestRecommendNewConsultingVisuals(t *testing.T) {
	cases := map[string]string{"raci responsibility matrix": "raci-matrix", "rag status board": "status-board", "issue tree mece": "issue-tree", "decision tree": "decision-tree", "vertical timeline": "timeline-vertical", "spectrum continuum": "spectrum", "bullet chart": "progress-list", "image hero": "image-hero", "input process output": "ipo-flow", "three horizons": "three-horizons", "two column prose": "prose-2col", "bibliography sources appendix": "sources-list", "requirements checklist": "checklist"}
	for intent, want := range cases {
		t.Run(want, func(t *testing.T) {
			got := RecommendVisual(Default(), intent, nil, 8)
			if len(got.Candidates) == 0 || got.Candidates[0].Name != want {
				t.Fatalf("%q: want %s first, got %+v", intent, want, got.Candidates)
			}
		})
	}
	got := RecommendVisual(Default(), "sankey", nil, 8)
	if got.UnsupportedVisual != "sankey" || !strings.Contains(got.QueryUnderstood, "stacked_bar") || !strings.Contains(got.QueryUnderstood, "waterfall") {
		t.Fatalf("missing concrete alternative: %+v", got)
	}
}
