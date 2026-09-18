package deckplan

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestBuildDeckPlan_StructuralSlidesComparisonAndEmphasisCap is the
// go-slide-creator-xmpb acceptance test: for several briefs, the title slide
// uses the title layout with no pattern, the closing slide uses the closing
// layout with no pattern, comparison slots map only to the comparison family,
// and emphasis patterns never exceed ceil(n/5).
func TestBuildDeckPlan_StructuralSlidesComparisonAndEmphasisCap(t *testing.T) {
	reg := patterns.Default()
	family := map[string]bool{}
	for _, n := range comparisonFamily {
		family[n] = true
	}

	cases := []struct {
		name     string
		brief    string
		audience string
		budget   int
	}{
		{"qbr", "Q3 QBR for the executive team: revenue grew +23% year over year, churn fell to 4%, EU expansion is on track, hiring plan adds 40 engineers.", "executives", 10},
		{"series-b", "Pitch our Series B for an AI infra company", "investors", 15},
		{"transformation", "Operating model transformation: current state vs future state, and the options we considered", "board", 7},
		{"short", "Quick status update", "", 3},
		{"long", "Annual strategy review with market data, competitive comparison, and roadmap", "leadership", 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := BuildDeckPlan(reg, Params{Brief: tc.brief, SlideBudget: tc.budget, Audience: tc.audience}, nil)
			n := len(res.Slides)
			if n != tc.budget {
				t.Fatalf("got %d slides, want %d", n, tc.budget)
			}

			first, last := res.Slides[0], res.Slides[n-1]
			if first.Layout != LayoutTitle {
				t.Errorf("slide[0].layout = %q, want %q", first.Layout, LayoutTitle)
			}
			if last.Layout != LayoutClosing {
				t.Errorf("slide[last].layout = %q, want %q", last.Layout, LayoutClosing)
			}
			for _, s := range []Slide{first, last} {
				if s.RecommendedPattern != "" || s.SuggestedPattern != "" {
					t.Errorf("slide %d (%s): want no pattern, got %q", s.SlideIndex, s.NarrativeRole, s.RecommendedPattern)
				}
				var skel map[string]any
				if err := json.Unmarshal(s.Skeleton, &skel); err != nil {
					t.Fatalf("slide %d: skeleton: %v", s.SlideIndex, err)
				}
				if skel["layout_id"] != s.Layout {
					t.Errorf("slide %d: skeleton layout_id %v != %q", s.SlideIndex, skel["layout_id"], s.Layout)
				}
				if _, ok := skel["pattern"]; ok {
					t.Errorf("slide %d: structural skeleton carries a pattern", s.SlideIndex)
				}
			}

			emphasis := 0
			for _, s := range res.Slides {
				if emphasisPatterns[s.RecommendedPattern] {
					emphasis++
				}
				if s.NarrativeRole == "comparison" && !family[s.RecommendedPattern] {
					t.Errorf("slide %d: comparison slot mapped to %q, want one of %v", s.SlideIndex, s.RecommendedPattern, comparisonFamily)
				}
				if s.SlideIndex > 0 && s.SlideIndex < n-1 {
					if s.RecommendedPattern == "" {
						t.Errorf("slide %d: content slide has no pattern", s.SlideIndex)
					}
					if s.Layout != LayoutPattern {
						t.Errorf("slide %d: layout %q, want %q", s.SlideIndex, s.Layout, LayoutPattern)
					}
				}
			}
			if limit := MaxEmphasisSlides(n); emphasis > limit {
				t.Errorf("emphasis slides = %d, want <= ceil(%d/5) = %d", emphasis, n, limit)
			}
			if res.RhythmCheck.EmphasisCount != emphasis {
				t.Errorf("rhythm_check.emphasis_count = %d, counted %d", res.RhythmCheck.EmphasisCount, emphasis)
			}
			if res.RhythmCheck.LongestPatternRun > 2 {
				t.Errorf("longest pattern run = %d, want <= 2", res.RhythmCheck.LongestPatternRun)
			}
		})
	}
}

func TestMaxEmphasisSlides(t *testing.T) {
	for n, want := range map[int]int{0: 0, 1: 1, 3: 1, 5: 1, 6: 2, 10: 2, 11: 3, 30: 6} {
		if got := MaxEmphasisSlides(n); got != want {
			t.Errorf("MaxEmphasisSlides(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestCapEmphasis_DemotesExcessAndKeepsMustInclude(t *testing.T) {
	reg := patterns.Default()
	pats := []string{"", "stat-hero", "pull-quote", "stat-hero", "kpi-inline", "card-grid", "pull-quote", "stat-hero", "stat-hero", ""}
	slides := make([]Slide, len(pats))
	for i, p := range pats {
		slides[i] = Slide{SlideIndex: i, NarrativeRole: "evidence", RecommendedPattern: p}
	}
	slides[0].NarrativeRole = "opening"
	slides[9].NarrativeRole = "closing"
	slides[8].Rationale = mustIncludeRationale // must_include is always kept

	slides = capEmphasis(reg, slides, "brief")

	count := 0
	for i, s := range slides {
		if emphasisPatterns[s.RecommendedPattern] {
			count++
		}
		if i > 0 && s.RecommendedPattern != "" && s.RecommendedPattern == slides[i-1].RecommendedPattern {
			t.Errorf("slides %d/%d share pattern %q after cap", i-1, i, s.RecommendedPattern)
		}
	}
	if count > MaxEmphasisSlides(len(slides)) {
		t.Errorf("emphasis count %d exceeds cap %d", count, MaxEmphasisSlides(len(slides)))
	}
	if slides[8].RecommendedPattern != "stat-hero" {
		t.Errorf("must_include stat-hero was demoted to %q", slides[8].RecommendedPattern)
	}
}

func TestPickComparisonPattern(t *testing.T) {
	if got := pickComparisonPattern("compare vendors", nil, ""); got != "comparison-2col" {
		t.Errorf("default = %q, want comparison-2col", got)
	}
	if got := pickComparisonPattern("before and after the migration", nil, ""); got != "before-after" {
		t.Errorf("before/after brief = %q, want before-after", got)
	}
	if got := pickComparisonPattern("compare vendors", []string{"comparison-2col"}, ""); got != "before-after" {
		t.Errorf("second comparison slot = %q, want before-after (least used)", got)
	}
	if got := pickComparisonPattern("", nil, "comparison-2col"); got != "before-after" {
		t.Errorf("exclude = %q, want before-after", got)
	}
}
