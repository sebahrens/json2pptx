package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRecommendVisualNarrowCyclePlacement(t *testing.T) {
	for _, intent := range []string{
		"Show a two-loop learning cycle beside explanatory text on one slide; the cycle occupies half the slide width.",
		"Figure-eight cycle beside explanatory text",
		"Commentary next to a figure-eight cycle",
		"Figure-eight cycle in a narrow column",
		"A half-width figure-eight cycle",
		"Figure-eight cycle on the left half of the slide",
		"Figure-eight cycle occupies 40% of the slide width",
		"Figure-eight cycle takes one third of the slide width",
	} {
		t.Run(intent, func(t *testing.T) {
			hints := &VisualHints{ContentHints: ContentHints{ItemCount: 6}}
			result := RecommendVisual(Default(), intent, hints, 5)
			if len(result.Candidates) == 0 || result.Candidates[0].Name != "cycle-ring" {
				t.Fatalf("narrow cycle top choice: %+v", result.Candidates)
			}
			if !strings.Contains(strings.Join(result.DisambiguatingQuestions, " "), "Must both coupled loops remain?") {
				t.Error("ambiguous alternative needs an explicit preserve-or-simplify question")
			}
			for _, detail := range []string{"does not preserve", "two coupled lobes", "580pt", "140pt", "above/below", "expand_pattern"} {
				if !strings.Contains(result.Candidates[0].Rationale, detail) {
					t.Errorf("alternative omits %q: %s", detail, result.Candidates[0].Rationale)
				}
			}
			short := RecommendVisual(Default(), intent, hints, 1, &RecommendOptions{Candidates: []string{"cycle-figure-eight", "cycle-ring"}})
			if len(short.Candidates) != 2 || short.Candidates[0].Name != "cycle-ring" || short.Candidates[1].Score >= 0.5 {
				t.Fatalf("shortlist lost entries or ignored placement: %+v", short.Candidates)
			}
			if !strings.Contains(short.Candidates[1].Rationale, "requested narrow placement") || short.Candidates[1].SegmentFit.MinContentWidthPt != 580 {
				t.Errorf("shortlist hides actual placement constraint: %+v", short.Candidates[1])
			}
			only := RecommendVisual(Default(), intent, hints, 5, &RecommendOptions{Candidates: []string{"cycle-figure-eight"}})
			if len(only.Candidates) != 1 || only.Candidates[0].Name != "cycle-figure-eight" || only.Candidates[0].Score >= 0.5 {
				t.Fatalf("single-choice shortlist must retain and qualify authored choice: %+v", only.Candidates)
			}
		})
	}
}

func TestRecommendVisualCyclePlacementDoesNotInferNarrowWidth(t *testing.T) {
	for _, intent := range []string{
		"Figure-eight cycle at full width, explanatory text below",
		"Full-width figure-eight cycle above a half-width text panel",
		"Figure-eight cycle above explanatory text",
		"Figure-eight cycle in the upper half of the slide, text below",
		"Figure-eight cycle, 50% of the slide height, text below",
		"Figure-eight cycle for the second half of the year",
		"Figure-eight cycle to narrow the skills gap and choose the right approach",
		"Figure-eight cycle with two loops beside each other",
		"Figure-eight with two loops next to one another",
		"Figure-eight with two coupled loops side by side",
		"Figure-eight phases on the left lobe",
	} {
		t.Run(intent, func(t *testing.T) {
			result := RecommendVisual(Default(), intent, nil, 5, &RecommendOptions{Candidates: []string{"cycle-figure-eight", "cycle-ring"}})
			figure := findCandidate(result.Candidates, "cycle-figure-eight")
			if figure == nil || figure.Score < 0.5 || strings.Contains(figure.Rationale, "requested narrow placement") {
				t.Fatalf("intent has no narrow cycle width: %+v", result.Candidates)
			}
		})
	}
}

func TestRecommendVisualFigureEightCompositionPlacement(t *testing.T) {
	// An invalid all-pattern pair is already refused for missing affinity.
	// A heterogeneous shortlist can construct a horizontal layout, so it
	// must report the width requirement on the root, not just its leaf.
	result := RecommendVisual(Default(), "figure-eight beside a bar chart", nil, 5, &RecommendOptions{Candidates: []string{"compose:cycle-figure-eight+chart:bar"}})
	if len(result.Candidates) != 1 || result.Candidates[0].Composition == nil || result.Candidates[0].Score >= 0.5 || !strings.Contains(result.Candidates[0].Rationale, "580pt") || len(result.DisambiguatingQuestions) == 0 {
		t.Fatalf("horizontal composition bypassed placement guidance: %+v", result)
	}
	// A vertical composition gives the figure full width; metadata alone
	// must not lower its score or claim a horizontal placement conflict.
	c := VisualCandidate{Category: VisualCategoryCompose, Score: 0.9, Composition: &VisualComposition{Direction: "vertical", Regions: []VisualRegion{
		{Category: VisualCategoryPattern, Name: "cycle-figure-eight", SizePct: 50},
		{Category: VisualCategoryPattern, Name: "labeled-rows", SizePct: 50},
	}}}
	got := applyPatternPlacementGuidance(Default(), "figure-eight above text", &VisualHints{}, []VisualCandidate{c}, true)
	if got[0].Score != c.Score || got[0].placementConflict {
		t.Fatalf("vertical composition wrongly demoted: %+v", got)
	}
}

func TestRecommendVisualCarriesPatternPlacementContract(t *testing.T) {
	result := RecommendVisual(Default(), "inline KPIs", nil, 5, &RecommendOptions{Candidates: []string{"kpi-inline"}})
	if len(result.Candidates) != 1 || result.Candidates[0].SegmentFit == nil || !result.Candidates[0].SegmentFit.SegmentOnly || !strings.Contains(result.Candidates[0].Rationale, "standalone recipe illustrates") {
		t.Fatalf("supporting-band recipe must carry its actual authoring context: %+v", result.Candidates)
	}
	composition := &VisualComposition{Direction: "vertical", Regions: []VisualRegion{
		{Category: VisualCategoryPattern, Name: "cycle-figure-eight"},
		{Compose: &VisualComposition{Regions: []VisualRegion{{Category: VisualCategoryPattern, Name: "kpi-inline"}}}},
		{Category: VisualCategoryPattern, Name: "card-grid"},
	}}
	annotateCompositionFit(Default(), composition)
	if composition.Regions[0].SegmentFit.MinContentWidthPt != 580 || !composition.Regions[1].Compose.Regions[0].SegmentFit.SegmentOnly {
		t.Fatalf("composition leaves lost placement constraints: %+v", composition)
	}
	if composition.Regions[2].SegmentFit != nil {
		t.Error("unconstrained composition leaf should omit empty fit metadata")
	}
	// Optional omission keeps the small recommendation reply useful, while
	// real limits (including reference-only budgets) remain available.
	choices := RecommendVisual(Default(), "cards", nil, 5, &RecommendOptions{Candidates: []string{"card-grid", "kpi-5up"}})
	for _, c := range choices.Candidates {
		wire, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if c.Name == "card-grid" && strings.Contains(string(wire), `"segment_fit"`) {
			t.Error("unconstrained candidate must omit empty fit metadata on the wire")
		}
		if c.Name == "kpi-5up" && (c.SegmentFit == nil || c.SegmentFit.ReferenceValueDigitBudget == 0) {
			t.Error("compact recommendations must retain reference digit budgets")
		}
	}
}
