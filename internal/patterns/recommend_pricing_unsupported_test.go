package patterns

import "testing"

func TestPricingIntentsPreferCardsWithoutDemotingArchitecture(t *testing.T) {
	reg := Default()
	for _, intent := range []string{
		"pricing tiers for three plans",
		"compare pricing plans",
		"three subscription tiers",
		"subscription plans for businesses",
	} {
		t.Run(intent, func(t *testing.T) {
			pattern := Recommend(reg, intent, nil, 3)
			if len(pattern.Candidates) == 0 || pattern.Candidates[0].PatternName != "card-grid" {
				t.Fatalf("pattern candidates = %+v, want card-grid first", pattern.Candidates)
			}
			for _, c := range pattern.Candidates {
				if c.PatternName == "arch-stack" && c.ConfidenceBand == confidenceHigh {
					t.Errorf("architecture falsely scored high: %+v", c)
				}
			}
			visual := RecommendVisual(reg, intent, nil, 5)
			if len(visual.Candidates) == 0 || visual.Candidates[0].Name != "card-grid" {
				t.Fatalf("visual candidates = %+v, want card-grid first", visual.Candidates)
			}
		})
	}
	for _, intent := range []string{"three-tier architecture", "technology stack layers", "platform infrastructure"} {
		t.Run(intent, func(t *testing.T) {
			got := Recommend(reg, intent, nil, 3)
			if len(got.Candidates) == 0 || got.Candidates[0].PatternName != "arch-stack" || got.Candidates[0].ConfidenceBand != confidenceHigh {
				t.Fatalf("architecture candidates = %+v", got.Candidates)
			}
		})
	}
	agenda := Recommend(reg, "agenda for the session", nil, 3)
	if len(agenda.Candidates) == 0 || agenda.Candidates[0].ConfidenceBand != confidenceHigh {
		t.Fatalf("agenda lost high confidence: %+v", agenda.Candidates)
	}
}

func TestSankeyIntentExplicitlyUnsupported(t *testing.T) {
	reg := Default()
	for _, intent := range []string{"sankey of cost flows", "Show a Sankey diagram of energy transfers"} {
		t.Run(intent, func(t *testing.T) {
			for _, opts := range []*RecommendOptions{nil, {Candidates: []string{"process-flow", "card-grid"}}} {
				pattern := Recommend(reg, intent, nil, 3, opts)
				if pattern.UnsupportedVisual != "sankey" || len(pattern.Candidates) != 0 || pattern.BeyondPatterns != nil {
					t.Errorf("pattern result = %+v", pattern)
				}
				visual := RecommendVisual(reg, intent, nil, 5, opts)
				if visual.UnsupportedVisual != "sankey" || len(visual.Candidates) != 0 {
					t.Errorf("visual result = %+v", visual)
				}
			}
		})
	}
	for _, intent := range []string{"sankeyed flows", "show cost flows"} {
		if got := RecommendVisual(reg, intent, nil, 5); got.UnsupportedVisual != "" {
			t.Errorf("%q incorrectly marked unsupported", intent)
		}
	}
}
