package deckplan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestHousePillarCount(t *testing.T) {
	for brief, want := range map[string]int{
		"Our strategy house rests on four pillars: trust, velocity, growth and reach, on a foundation of people and data.": 4,
		"Strategy house with the pillars trust, speed and scale.":                                                          0,
		"Strategy house. Pillars: trust, speed, scale.":                                                                    3,
		"The plan has five strategic pillars and one foundation.":                                                          5,
		"Three themes: cost, growth, risk, talent and brand.":                                                              5,
		"A strategy house for the board.":                                                                                  0,
		"Pillars: a, b, c, d, e, f and g.":                                                                                 0,
	} {
		if got := housePillarCount(brief); got != want {
			t.Errorf("housePillarCount(%q) = %d, want %d", brief, got, want)
		}
	}
}

// TestPlanDraftsAHouseWithTheBriefsPillarCount pins go-slide-creator-qad87:
// the drafted house has as many pillars as the brief names themes.
func TestPlanDraftsAHouseWithTheBriefsPillarCount(t *testing.T) {
	reg := patterns.Default()
	for brief, want := range map[string]int{
		"Strategy house for FY27: an objective, a foundation, and four pillars: trust, velocity, growth and reach.":       4,
		"Strategy house for FY27: an objective, a foundation, and five pillars.":                                          5,
		"Strategy house for FY27: an objective and a foundation; the pillars are trust, velocity and disciplined growth.": 3,
	} {
		skel, err := patterns.SkeletonForPattern(reg, "strategy-house", "framework")
		if err != nil {
			t.Fatal(err)
		}
		slides := []Slide{{NarrativeRole: "framework", RecommendedPattern: "strategy-house"}}
		attachSlidePredictions(reg, slides, brief, "", nil)
		var got, base struct {
			Pattern struct {
				Values struct {
					Pillars []struct {
						Title string   `json:"title"`
						Body  []string `json:"body"`
					} `json:"pillars"`
				} `json:"values"`
			} `json:"pattern"`
		}
		if err := json.Unmarshal(slides[0].Skeleton, &got); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(skel, &base)
		if n := len(got.Pattern.Values.Pillars); n != want {
			t.Errorf("brief %q drafted %d pillars (exemplar has %d), want %d", brief, n, len(base.Pattern.Values.Pillars), want)
		}
		for i, p := range got.Pattern.Values.Pillars {
			if p.Title != patterns.FillPlaceholder {
				t.Errorf("pillar %d title = %q, want the fill token", i, p.Title)
			}
		}
	}
	// A brief that names no count keeps the exemplar's shape.
	slides := []Slide{{NarrativeRole: "framework", RecommendedPattern: "strategy-house"}}
	attachSlidePredictions(reg, slides, "A strategy house for the board.", "", nil)
	if !strings.Contains(string(slides[0].Skeleton), `"pillars"`) {
		t.Errorf("skeleton lost its pillars: %s", slides[0].Skeleton)
	}
}

func TestDeckSpecPlanNamesThePillarCount(t *testing.T) {
	plan := BuildDeckSpecPlan(Params{Brief: "We shipped the new platform this quarter. Our plan for next year has four pillars: trust, velocity, growth and reach. Revenue grew 12% and churn fell to 3%.", SlideBudget: 8})
	found := false
	for _, s := range plan.Slots {
		if s.Kind == "pillars" && strings.Contains(s.Guidance, "The brief names 4 themes: write 4 pillars") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no pillars slot names the count: %+v", plan.Slots)
	}
}
