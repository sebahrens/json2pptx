package deckplan

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// metricBriefs each carry three or more numeric facts.
var metricBriefs = map[string]string{
	"board QBR":  "Board QBR: ARR $42M up 18%, churn 4%. Compare build vs buy for the settlement platform. Ask approve $3M.",
	"quarterly":  "Quarterly review: revenue grew 23% to $18M, net retention 118%, sales cycle down to 41 days, headcount 240.",
	"migration":  "Migration status: 2 of 3 clearers live, 14 waves complete, 0 settlement breaks, 9 months to the deadline.",
	"unit econs": "Unit economics: CAC $4,200, payback 14 months, gross margin 78%, LTV/CAC 3.4x.",
}

// A brief whose facts are numbers has to reach a big-number slide: the review
// found a 10-slide plan for "ARR $42M up 18%, churn 4%" with no KPI-family
// slide anywhere in it (go-slide-creator-8fq4).
func TestPlanPlacesAKPISlideForAMetricBrief(t *testing.T) {
	for name, brief := range metricBriefs {
		t.Run(name, func(t *testing.T) {
			for _, budget := range []int{8, 10, 14, 20} {
				res := BuildDeckPlan(patterns.Default(), Params{
					Brief: brief, SlideBudget: budget, Audience: "board",
				}, nil)
				if !planHasBigNumberSlide(res) {
					t.Errorf("budget %d: a brief with three or more metrics got no KPI-family slide: %v",
						budget, planPatterns(res))
				}
			}
		})
	}
}

func TestPlanEvidenceFollowsFactsWithoutMetricKeywords(t *testing.T) {
	brief := "Operations review. On-time delivery 94.2%; cost per order $12.40; warehouse utilisation 87%; headcount 812; North 41%, South 33%, West 26%; returns 2.1%."
	res := BuildDeckPlan(patterns.Default(), Params{Brief: brief, SlideBudget: 10}, nil)
	var numericSlides, chartSlides int
	for _, slide := range res.Slides {
		if slide.NarrativeRole != "evidence" {
			continue
		}
		if patternFamily(slide.RecommendedPattern) == "kpi" || slide.RecommendedPattern == "stat-hero" {
			numericSlides++
		}
		if slide.RecommendedPattern == "chart-insights-split" {
			chartSlides++
			for _, fact := range []string{"North 41%", "South 33%", "West 26%"} {
				if !containsStr(slide.Facts, fact) {
					t.Errorf("regional split scattered instead of reaching chart: missing %q from %v", fact, slide.Facts)
				}
			}
		}
	}
	if numericSlides == 0 || chartSlides == 0 {
		t.Fatalf("keyword-free quantities got no KPI + chart evidence: %v", planPatterns(res))
	}
}

func TestPlanEvidenceDoesNotUseRolePhraseAsKeywords(t *testing.T) {
	res := BuildDeckPlan(patterns.Default(), Params{Brief: "We sell widgets to bakeries.", SlideBudget: 10}, nil)
	for _, slide := range res.Slides {
		if slide.NarrativeRole != "evidence" {
			continue
		}
		switch slide.RecommendedPattern {
		case "before-after-compact", "process-flow-compact", "hero-detail":
			t.Fatalf("content-free brief inherited evidence-role keyword pattern %q: %v", slide.RecommendedPattern, planPatterns(res))
		}
	}
}

func TestPlanPhaseSequenceTakesPrecedenceOverIncidentalNumbers(t *testing.T) {
	brief := "Implementation plan. Phase 1 covers three hubs by October; phase 2 adds 12 routes by December; phase 3 reaches 40 teams next year."
	res := BuildDeckPlan(patterns.Default(), Params{Brief: brief, SlideBudget: 10}, nil)
	for _, slide := range res.Slides {
		if slide.NarrativeRole == "evidence" {
			if slide.RecommendedPattern != "phase-roadmap" {
				t.Fatalf("dated phase sequence was misread as generic numbers: %v", planPatterns(res))
			}
			return
		}
	}
	t.Fatal("plan has no evidence slot")
}

func TestPlanComparisonMatrixUsesTableEvidence(t *testing.T) {
	brief := "We must compare three vendor options on five criteria: cost, reach, resilience, timeline, and integration."
	res := BuildDeckPlan(patterns.Default(), Params{Brief: brief, SlideBudget: 10}, nil)
	for _, slide := range res.Slides {
		if slide.NarrativeRole == "evidence" {
			if slide.RecommendedPattern != "table-highlight" {
				t.Fatalf("multi-option criteria need table evidence: %v", planPatterns(res))
			}
			return
		}
	}
	t.Fatal("plan has no evidence slot")
}

// One pattern family used four times is a deck that reads as one idea
// repeated. Tightened for go-slide-creator-whp97: a 20-slide plan used to be
// allowed to repeat as long as rhythm_check reported it, because the role
// allocation asked for more comparison slides than the brief or the
// comparison family could supply. Roles are now bounded by both, and the
// repeat cap searches the whole registry, so a plan repeats nothing at all —
// or comes back shorter than the budget and says why.
func TestPlanCapsPatternRepeats(t *testing.T) {
	for name, brief := range metricBriefs {
		t.Run(name, func(t *testing.T) {
			for _, budget := range []int{10, 20} {
				res := BuildDeckPlan(patterns.Default(), Params{
					Brief: brief, SlideBudget: budget, Audience: "board",
				}, nil)

				counts := map[string]int{}
				for _, s := range res.Slides {
					if s.RecommendedPattern != "" {
						counts[patternFamily(s.RecommendedPattern)]++
					}
				}
				for family, n := range counts {
					if n <= maxPatternRepeats {
						continue
					}
					// Over the cap is allowed only in a plan that came back
					// short, which says in words what it gave up.
					if res.BudgetNote == "" {
						t.Errorf("budget %d: %s appears %d times in a full-length plan (rhythm_check: %v)",
							budget, family, n, res.RhythmCheck.RepeatedFamilies)
					}
					if !repeatReported(res.RhythmCheck.RepeatedFamilies, family) {
						t.Errorf("budget %d: %s appears %d times and rhythm_check does not report it (%v)",
							budget, family, n, res.RhythmCheck.RepeatedFamilies)
					}
				}
				if len(res.RhythmCheck.RepeatedFamilies) > 0 && res.BudgetNote == "" {
					t.Errorf("budget %d: plan repeats %v with no explanation",
						budget, res.RhythmCheck.RepeatedFamilies)
				}
			}
		})
	}
}

// The families a deck reads as one idea.
func TestPatternFamilyGroupsWhatTheRoomSees(t *testing.T) {
	cases := map[string]string{
		"kpi-2up":              "kpi",
		"kpi-6up":              "kpi",
		"kpi-inline":           "kpi",
		"before-after":         "before-after",
		"before-after-compact": "before-after",
		"process-flow":         "process-flow",
		"process-flow-compact": "process-flow",
		"comparison-2col":      "comparison-2col",
		"":                     "",
	}
	for in, want := range cases {
		if got := patternFamily(in); got != want {
			t.Errorf("patternFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

func planHasBigNumberSlide(res *Result) bool {
	for _, s := range res.Slides {
		if patternFamily(s.RecommendedPattern) == "kpi" || s.RecommendedPattern == "stat-hero" || s.RecommendedPattern == "hero-detail" {
			return true
		}
	}
	return false
}

func planPatterns(res *Result) []string {
	out := make([]string, 0, len(res.Slides))
	for _, s := range res.Slides {
		if s.RecommendedPattern != "" {
			out = append(out, s.RecommendedPattern)
		}
	}
	return out
}

func repeatReported(reported []string, family string) bool {
	for _, r := range reported {
		if strings.HasPrefix(r, family+" x") {
			return true
		}
	}
	return false
}
