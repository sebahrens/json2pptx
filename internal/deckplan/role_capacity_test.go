package deckplan

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestComparisonClauseCount counts ideas, not cue words: "compare build vs buy"
// carries three cues and is one comparison (go-slide-creator-whp97).
func TestComparisonClauseCount(t *testing.T) {
	cases := map[string]int{
		"Compare build vs buy for the settlement platform.":                      1,
		"Board QBR: ARR $42M up 18%, churn 4%. Compare build vs buy. Ask $3M.":   1,
		"Compare build vs buy. Weigh hosted versus on-prem.":                     2,
		"Quarterly review: revenue grew 23% to $18M, net retention 118%.":        0,
		"Operating model: current state vs future state, and the options we saw": 2,
		"": 0,
	}
	for brief, want := range cases {
		if got := comparisonClauseCount(brief); got != want {
			t.Errorf("comparisonClauseCount(%q) = %d, want %d", brief, got, want)
		}
	}
}

// TestRoleCapacityBoundsComparison: the comparison role can never exceed its
// closed pattern set, and a brief that proposes one choice gets one slot.
func TestRoleCapacityBoundsComparison(t *testing.T) {
	reg := patterns.Default()
	if got := roleFamilyCapacity(reg, "comparison"); got != len(comparisonFamily) {
		t.Errorf("comparison family capacity = %d, want %d", got, len(comparisonFamily))
	}
	if got := roleCapacity(reg, "comparison", "Compare build vs buy."); got != 1 {
		t.Errorf("one comparison in the brief should allow one slot, got %d", got)
	}
	if got := roleCapacity(reg, "comparison", "Compare build vs buy. Weigh hosted versus on-prem. Consider a third option too."); got != len(comparisonFamily) {
		t.Errorf("three comparisons cannot exceed the %d-pattern family, got %d", len(comparisonFamily), got)
	}
	// A brief with no comparison language still gets the arc's one decision
	// moment — the planner has always placed one.
	if got := roleCapacity(reg, "comparison", "Quarterly review: revenue grew 23%."); got != 1 {
		t.Errorf("a brief with no comparison should still allow one slot, got %d", got)
	}
	// Evidence draws on the whole registry, so it is not the binding limit.
	if got := roleFamilyCapacity(reg, "evidence"); got < 10 {
		t.Errorf("evidence family capacity = %d, expected the registry to offer many more", got)
	}
}

// TestApplyRoleCapacityMovesSurplus: slots a role cannot fill go to a role that
// can, and the plan keeps its budget.
func TestApplyRoleCapacityMovesSurplus(t *testing.T) {
	reg := patterns.Default()
	counts := map[string]int{"framework": 3, "evidence": 9, "comparison": 3, "emphasis": 2}
	order := []string{"framework", "evidence", "comparison", "emphasis"}
	before := 0
	for _, n := range counts {
		before += n
	}

	counts, note := applyRoleCapacity(reg, "Compare build vs buy.", counts, order)
	if counts["comparison"] != 1 {
		t.Errorf("comparison = %d, want 1 (the brief proposes one choice)", counts["comparison"])
	}
	after := 0
	for _, n := range counts {
		after += n
	}
	if after != before {
		t.Errorf("slots changed from %d to %d; the surplus should have moved, not vanished", before, after)
	}
	if note != "" {
		t.Errorf("the surplus fit elsewhere, so there should be no note; got %q", note)
	}
}

// TestApplyRoleCapacityShortensWhenNothingCanAbsorb: with no role able to take
// the surplus, the plan comes back shorter and says why.
func TestApplyRoleCapacityShortensWhenNothingCanAbsorb(t *testing.T) {
	reg := patterns.Default()
	// comparison alone: nothing in absorbOrder is present to take the surplus.
	counts := map[string]int{"comparison": 5}
	counts, note := applyRoleCapacity(reg, "Compare build vs buy.", counts, []string{"comparison"})
	if counts["comparison"] != 1 {
		t.Errorf("comparison = %d, want 1", counts["comparison"])
	}
	if note == "" {
		t.Fatal("dropping four slots must be explained")
	}
	for _, want := range []string{"4 fewer slide", "comparison 5→1"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q does not mention %q", note, want)
		}
	}
}

// TestDistributeRolesShortensPlanWithNote is the end-to-end of the same thing:
// a plan shorter than its budget still opens and closes properly.
func TestDistributeRolesShortensPlanWithNote(t *testing.T) {
	roles, note := distributeRoles(patterns.Default(), "Compare build vs buy.", 20)
	if note != "" {
		t.Fatalf("this brief's surplus fits in evidence; unexpected note %q", note)
	}
	if len(roles) != 20 {
		t.Fatalf("got %d roles, want 20", len(roles))
	}
	comparison := 0
	for _, r := range roles {
		if r == "comparison" {
			comparison++
		}
	}
	if comparison != 1 {
		t.Errorf("one comparison in the brief produced %d comparison slots", comparison)
	}
	if roles[0] != "opening" || roles[len(roles)-1] != "closing" {
		t.Errorf("plan must still open and close: %q … %q", roles[0], roles[len(roles)-1])
	}
}

// TestPlanKeepsComparisonSlotInItsFamily: the repeat cap may not fix a repeat by
// putting something that is not a comparison in a comparison slot, and it
// counts the locked slot first so an ordinary slide is the one that moves.
func TestPlanKeepsComparisonSlotInItsFamily(t *testing.T) {
	reg := patterns.Default()
	for name, brief := range metricBriefs {
		t.Run(name, func(t *testing.T) {
			res := BuildDeckPlan(reg, Params{Brief: brief, SlideBudget: 20, Audience: "board"}, nil)
			for _, s := range res.Slides {
				if s.NarrativeRole != "comparison" {
					continue
				}
				if !containsStr(comparisonFamily, s.RecommendedPattern) {
					t.Errorf("slide %d: comparison slot holds %q, want one of %v",
						s.SlideIndex, s.RecommendedPattern, comparisonFamily)
				}
			}
		})
	}
}
