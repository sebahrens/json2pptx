package patterns

import (
	"reflect"
	"strings"
	"testing"
)

func TestScoreComposeCandidatePermutationsKeepBestPair(t *testing.T) {
	cases := []struct {
		name  string
		input []VisualCandidate
		want  string
		score float64
	}{
		{"equal-score pair", []VisualCandidate{{Name: "stylish-panels", Score: .9}, {Name: "pull-quote", Score: .9}, {Name: "kpi-3up", Score: .6}, {Name: "icon-row", Score: .5}}, "compose:pull-quote+stylish-panels", .9},
		{"higher compatible pair", []VisualCandidate{{Name: "stylish-panels", Score: .95}, {Name: "pull-quote", Score: .94}, {Name: "kpi-3up", Score: .6}, {Name: "icon-row", Score: .5}}, "compose:pull-quote+stylish-panels", .95},
		// stat-hero has no outgoing affinity; chart-insights-split declares it.
		{"asymmetric affinity", []VisualCandidate{{Name: "stat-hero", Score: .99}, {Name: "chart-insights-split", Score: .95}, {Name: "stylish-panels", Score: .9}, {Name: "pull-quote", Score: .89}}, "compose:chart-insights-split+stat-hero", .97},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var expected []VisualCandidate
			count := 0
			var permute func([]VisualCandidate, int)
			permute = func(input []VisualCandidate, start int) {
				if start == len(input) {
					count++
					before := append([]VisualCandidate(nil), input...)
					got := scoreCompose(Default(), "evidence", input)
					if !reflect.DeepEqual(input, before) {
						t.Fatal("scorer mutated caller candidate order")
					}
					if len(got) != 1 || got[0].Name != tc.want || got[0].Score != tc.score {
						t.Fatalf("input %+v selected %+v, want %s score%g", input, got, tc.want, tc.score)
					}

					if tc.name == "asymmetric affinity" && strings.Contains(got[0].Rationale, "mutual") {
						t.Fatal("one-sided affinity must not claim mutual declarations")
					}
					if expected == nil {
						expected = got
					} else if !reflect.DeepEqual(got, expected) {
						t.Fatalf("input permutation changed rationale/placement/composition: got %+v want %+v", got, expected)
					}
					return
				}
				for i := start; i < len(input); i++ {
					input[start], input[i] = input[i], input[start]
					permute(input, start+1)
					input[start], input[i] = input[i], input[start]
				}
			}
			permute(append([]VisualCandidate(nil), tc.input...), 0)
			if count != 24 {
				t.Fatalf("expected24 permutations, got%d", count)
			}
		})
	}
}

func TestPairCompositionSymmetryPreservesPreferredRoles(t *testing.T) {
	for _, tc := range []struct {
		a, b, intent, first string
		firstShare          float64
	}{
		{"pull-quote", "stylish-panels", "side by side", "stylish-panels", 65},
		{"pull-quote", "stylish-panels", "stacked", "stylish-panels", 50},
		{"process-flow", "kpi-3up", "stacked", "kpi-3up", 50},
		{"process-flow", "kpi-3up", "side by side", "process-flow", 65},
		{"stylish-panels", "arch-stack", "side by side", "arch-stack", 50},
		{"stylish-panels", "arch-stack", "stacked", "arch-stack", 50},
	} {
		t.Run(tc.a+"/"+tc.b+"/"+tc.intent, func(t *testing.T) {
			a := pairComposition(Default(), tc.a, tc.b, tc.intent)
			b := pairComposition(Default(), tc.b, tc.a, tc.intent)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("symmetric inputs changed region order: %+v vs %+v", a, b)
			}
			if a.Regions[0].Name != tc.first || a.Regions[0].SizePct != tc.firstShare {
				t.Fatalf("lost intended role preference: %+v", a)
			}
			sharesSum(t, a)
		})
	}
}

func TestComposeMetadataRepeatableAcrossShortlists(t *testing.T) {
	intent := "panels and quote side by side on one slide"
	name := "compose:pull-quote+stylish-panels"
	var expected *VisualCandidate
	for i := 0; i < 25; i++ {
		open := RecommendVisual(Default(), intent, nil, 1000)
		candidate := findCandidate(open.Candidates, name)
		if candidate == nil {
			t.Fatal("missing expected composition")
		}
		if expected == nil {
			copy := *candidate
			expected = &copy
		}
		short := RecommendVisual(Default(), intent, nil, 1, &RecommendOptions{Candidates: []string{name}})
		if len(short.Candidates) != 1 {
			t.Fatal("shortlist lost composition")
		}
		for _, got := range []VisualCandidate{*candidate, short.Candidates[0]} {
			if got.Score != expected.Score || got.Rationale != expected.Rationale || !reflect.DeepEqual(got.Composition, expected.Composition) || !reflect.DeepEqual(got.Placement, expected.Placement) {
				t.Fatalf("iteration%d changed scored metadata: got%+v want%+v", i, got, *expected)
			}
		}
	}
}
