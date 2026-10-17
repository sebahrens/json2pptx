package patterns

import (
	"reflect"
	"testing"
)

func TestRecommendVisualFilteringPreservesGlobalScores(t *testing.T) {
	cases := []struct {
		intent string
		names  []string
	}{
		{waveRiskAppetite, []string{"table-highlight"}},
		{waveRiskAppetite, []string{"table-highlight", "kpi-inline"}},
		{waveRiskAppetite, []string{"status-board", "table-highlight", "kpi-inline"}},
		{"stakeholder map around the programme office", []string{"radial-hub", "stakeholder-map"}},
		{"numeric heatmap for error rates by team and week", []string{"numeric-heatmap", "risk-heatmap"}},
	}
	for _, tc := range cases {
		open := RecommendVisual(Default(), tc.intent, nil, 1000)
		short := RecommendVisual(Default(), tc.intent, nil, 1000, &RecommendOptions{Candidates: tc.names})
		if len(short.Candidates) != len(tc.names) {
			t.Fatalf("shortlist changed requested candidate count: %+v", short.Candidates)
		}
		for _, candidate := range short.Candidates {
			requested := false
			for _, name := range tc.names {
				requested = requested || candidate.Name == name
			}
			if !requested {
				t.Fatalf("unrequested candidate leaked: %s", candidate.Name)
			}
			for _, scored := range open.Candidates {
				if scored.Name == candidate.Name && scored.Category == candidate.Category && (scored.Score != candidate.Score || scored.ConfidenceBand != candidate.ConfidenceBand || scored.Rationale != candidate.Rationale) {
					t.Errorf("%s %s: open%v shortlist%v", tc.intent, candidate.Name, scored.Score, candidate.Score)
				}
			}
		}
	}
}
func TestRecommendConsultingWrappersHaveMeaningfulIntents(t *testing.T) {
	for intent, want := range map[string]string{
		"root cause categories for a quality defect":           "fishbone-framework",
		"numeric heatmap for error rates by team and week":     "numeric-heatmap",
		"pestel external environment forces":                   "pestel-framework",
		"performance potential employee calibration":           "talent-nine-box",
		"overlapping sets for customer needs and capabilities": "venn-framework",
	} {
		result := RecommendVisual(Default(), intent, nil, 5)
		if len(result.Candidates) == 0 || result.Candidates[0].Name != want {
			t.Errorf("%s: want%s got%+v", intent, want, result.Candidates)
		}
	}
}

func TestRecommendShortlistRetainsScoredMetadataAcrossCategories(t *testing.T) {
	intents := map[VisualCategory]string{VisualCategoryPattern: waveRiskAppetite, VisualCategoryChart: "quarterly revenue trend over time", VisualCategoryDiagram: "organization chart reporting hierarchy", VisualCategoryCompose: "panels and quote side by side on one slide"}
	for category, intent := range intents {
		open := RecommendVisual(Default(), intent, nil, 1000)
		var expected *VisualCandidate
		for i := range open.Candidates {
			if open.Candidates[i].Category == category {
				expected = &open.Candidates[i]
				break
			}
		}
		if expected == nil {
			t.Fatalf("no %s candidate for %s", category, intent)
		}
		short := RecommendVisual(Default(), intent, nil, 1, &RecommendOptions{Candidates: []string{expected.Name}})
		if len(short.Candidates) != 1 {
			t.Fatalf("lost %s shortlist", category)
		}
		got := short.Candidates[0]
		if got.Category != expected.Category || got.Score != expected.Score || got.ConfidenceBand != expected.ConfidenceBand || got.Rationale != expected.Rationale || !reflect.DeepEqual(got.Composition, expected.Composition) {
			t.Errorf("%s scored metadata changed: got%+v want%+v", category, got, *expected)
		}
	}
}

func TestRecommendExplicitComposeFallbackRemainsAuthorable(t *testing.T) {
	intent := "kpi metrics with a process flow combined"
	short := RecommendVisual(Default(), intent, nil, 5, &RecommendOptions{Candidates: []string{"compose:kpi-3up+process-flow", "kpi-3up"}})
	if len(short.Candidates) != 2 {
		t.Fatalf("lost requested views: %+v", short.Candidates)
	}
	var composition *VisualCandidate
	for i := range short.Candidates {
		if short.Candidates[i].Category == VisualCategoryCompose {
			composition = &short.Candidates[i]
		}
	}
	if composition == nil || composition.Composition == nil || composition.Score <= 0 {
		t.Fatal("caller-only explicit composition lost its scored runnable structure")
	}
}
