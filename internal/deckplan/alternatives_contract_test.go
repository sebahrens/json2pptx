package deckplan

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestPlanAlternativesPreserveComparisonAndAdjacency(t *testing.T) {
	for _, brief := range []string{
		"We must compare three vendor options on five criteria: cost, reach, resilience, timeline, and integration.",
		"Implementation plan. Phase 1 covers three hubs by October; phase 2 adds 12 routes by December; phase 3 reaches 40 teams next year.",
		"Implementation plan. Our team deploys the platform across 40 teams by December. Migration covers 12 routes in Q1.",
	} {
		plan := BuildDeckPlan(patterns.Default(), Params{Brief: brief, SlideBudget: 10}, nil)
		for i, s := range plan.Slides {
			if briefHasComparisonMatrix(brief) && s.NarrativeRole == "comparison" {
				if s.RecommendedPattern != "table-highlight" || s.SuggestedPatternFallback == "comparison-2col" || s.SuggestedPatternFallback == "before-after" {
					t.Errorf("three-option criteria matrix lost its contract: %+v", s)
				}
			}
			for _, a := range s.Alternatives {
				if !planningPatternCompatible(a.PatternName, s.NarrativeRole, brief) {
					t.Errorf("%s: incompatible %s for %s", brief, a.PatternName, s.NarrativeRole)
				}
				for _, j := range []int{i - 1, i + 1} {
					if j >= 0 && j < len(plan.Slides) && patternFamily(a.PatternName) == patternFamily(plan.Slides[j].RecommendedPattern) {
						t.Errorf("slide %d alternative %s repeats neighbor %d", i, a.PatternName, j)
					}
				}
			}
			if s.RecommendedPattern == "team-bios" {
				t.Errorf("incidental teams produced biography: %+v", s)
			}
		}
	}
}

func TestExplicitTeamIntroductionStillPlansPeople(t *testing.T) {
	name, _ := recommendForRole(patterns.Default(), nil, "framework", "Meet the team: our team biographies, names and roles, with headshots", "", nil, 1)
	if name != "team-bios" {
		t.Fatalf("explicit people introduction got %q", name)
	}
}
