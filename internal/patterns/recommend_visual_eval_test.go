package patterns

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

// visualEvalCase is one entry of testdata/recommend_visual_eval.json.
type visualEvalCase struct {
	Intent string       `json:"intent"`
	Hints  *VisualHints `json:"hints,omitempty"`
	Expect []string     `json:"expect"`
	Must   bool         `json:"must,omitempty"`
}

// visualEvalMinTopOne is the floor for the evaluation set's top-1 accuracy.
// It is the accuracy measured when the set was tracked; raise it when the
// ranking improves, never lower it to make a change pass.
const visualEvalMinTopOne = 1.0

func loadVisualEval(t *testing.T) []visualEvalCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/recommend_visual_eval.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []visualEvalCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Cases
}

// TestRecommendVisualEvalTopOne is the tracked evaluation for
// go-slide-creator-bdvhj: at least 30 plain intents with the candidate a
// consultant would pick first. It reports the top-1 accuracy, fails when it
// drops below the tracked floor, and fails outright on a "must" case — the
// misrankings the agent-journey review found.
func TestRecommendVisualEvalTopOne(t *testing.T) {
	cases := loadVisualEval(t)
	if len(cases) < 30 {
		t.Fatalf("evaluation set has %d intents, want at least 30", len(cases))
	}
	reg := Default()
	hit := 0
	for _, tc := range cases {
		rec := RecommendVisual(reg, tc.Intent, tc.Hints, 5)
		top := ""
		if len(rec.Candidates) > 0 {
			top = rec.Candidates[0].Name
		}
		ok := false
		for _, want := range tc.Expect {
			ok = ok || top == want
		}
		if ok {
			hit++
			continue
		}
		var ranked []string
		for _, c := range rec.Candidates {
			ranked = append(ranked, c.Name)
		}
		if tc.Must {
			t.Errorf("%q: top = %q, want one of %v (ranked %s)", tc.Intent, top, tc.Expect, strings.Join(ranked, ", "))
		} else {
			t.Logf("miss: %q: top = %q, want one of %v (ranked %s)", tc.Intent, top, tc.Expect, strings.Join(ranked, ", "))
		}
	}
	acc := float64(hit) / float64(len(cases))
	t.Logf("recommend_visual top-1 accuracy: %d/%d = %.3f (floor %.3f)", hit, len(cases), acc, visualEvalMinTopOne)
	if acc+1e-9 < visualEvalMinTopOne {
		t.Errorf("top-1 accuracy %.3f is below the tracked floor %.3f", acc, visualEvalMinTopOne)
	}
}

// TestRecommendVisualNearTiesSayWhatDiffers: candidates within 0.02 of each
// other carry a one-line differs_by (go-slide-creator-bdvhj).
func TestRecommendVisualNearTiesSayWhatDiffers(t *testing.T) {
	reg := Default()
	ties := 0
	for _, tc := range loadVisualEval(t) {
		rec := RecommendVisual(reg, tc.Intent, tc.Hints, 5)
		for i, c := range rec.Candidates {
			near := false
			for j, o := range rec.Candidates {
				if i != j && math.Abs(c.Score-o.Score) <= differsByMargin+1e-9 {
					near = true
				}
			}
			switch {
			case near && c.DiffersBy == "":
				t.Errorf("%q: %s %q (%.2f) is within %.2f of another candidate but has no differs_by", tc.Intent, c.Category, c.Name, c.Score, differsByMargin)
			case near:
				ties++
				if strings.Contains(c.DiffersBy, "\n") || len(c.DiffersBy) > 220 {
					t.Errorf("%q: differs_by of %q is not one short line: %q", tc.Intent, c.Name, c.DiffersBy)
				}
			case c.DiffersBy != "":
				t.Errorf("%q: %q has differs_by without a near tie: %q", tc.Intent, c.Name, c.DiffersBy)
			}
		}
	}
	if ties == 0 {
		t.Fatal("the evaluation set produced no near tie; the check is vacuous")
	}

	// The two ties the review named: only one of the pair can do what was asked.
	for intent, want := range map[string][2]string{
		"a value chain analysis":             {"value-chain", "highlight"},
		"a 2x2 matrix of our strategic bets": {"matrix_2x2", "plots"},
	} {
		rec := RecommendVisual(reg, intent, nil, 5)
		found := false
		for _, c := range rec.Candidates {
			if c.Name == want[0] && strings.Contains(c.DiffersBy, want[1]) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: no %s candidate whose differs_by mentions %q: %+v", intent, want[0], want[1], rec.Candidates)
		}
	}
}
