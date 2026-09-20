package patterns

import "testing"

// go-slide-creator-m2u2: recommend_pattern ranks named patterns only, so an
// intent whose best answer is a chart or diagram still came back with the best
// PATTERN — at "high". A high band on an answer the tool cannot even see is
// worse than no answer.

// TestRecommendCapsWhenADiagramIsTheRealAnswer is the bead's headline case.
func TestRecommendCapsWhenADiagramIsTheRealAnswer(t *testing.T) {
	reg := Default()
	got := Recommend(reg, "org chart of the leadership team", nil, 3)
	if len(got.Candidates) == 0 {
		t.Fatal("no candidates")
	}
	if got.BeyondPatterns == nil {
		t.Fatal("org_chart was not reported as the answer beyond the pattern universe")
	}
	if got.BeyondPatterns.Name != "org_chart" {
		t.Errorf("beyond_patterns = %q, want org_chart", got.BeyondPatterns.Name)
	}
	if band := got.Candidates[0].ConfidenceBand; band == confidenceHigh {
		t.Errorf("top candidate still reports %q; the tool cannot return the better answer", band)
	}
	if got.NextToolCall == nil || got.NextToolCall.Tool != "recommend_visual" {
		t.Errorf("next_tool_call = %+v, want recommend_visual", got.NextToolCall)
	}
	if got.NextToolCall.ArgsTemplate["intent"] != "org chart of the leadership team" {
		t.Errorf("next_tool_call should carry the intent, got %+v", got.NextToolCall.ArgsTemplate)
	}
}

// An intent a pattern genuinely answers keeps its high band and says nothing.
func TestRecommendLeavesGoodPatternAnswersAlone(t *testing.T) {
	reg := Default()
	for _, intent := range []string{
		"agenda for the session",
		"three big number KPIs",
	} {
		t.Run(intent, func(t *testing.T) {
			got := Recommend(reg, intent, nil, 3)
			if len(got.Candidates) == 0 {
				t.Fatal("no candidates")
			}
			if got.BeyondPatterns != nil {
				t.Errorf("flagged %+v; a pattern is the right answer here", got.BeyondPatterns)
			}
			if got.NextToolCall != nil {
				t.Errorf("appended %+v; there is nothing better to point at", got.NextToolCall)
			}
			if got.Candidates[0].Score >= 0.85 && got.Candidates[0].ConfidenceBand != confidenceHigh {
				t.Errorf("score %.2f was capped to %q for no reason",
					got.Candidates[0].Score, got.Candidates[0].ConfidenceBand)
			}
		})
	}
}

// Several frameworks exist as both a pattern and a native diagram. Pointing at
// the diagram there sends the agent to the same picture under another name.
func TestRecommendDoesNotRedirectToTheSameVisual(t *testing.T) {
	reg := Default()
	for _, intent := range []string{
		"business model canvas",
	} {
		t.Run(intent, func(t *testing.T) {
			if got := Recommend(reg, intent, nil, 3); got.BeyondPatterns != nil {
				t.Errorf("redirected to %q, which is the same visual as %q",
					got.BeyondPatterns.Name, got.Candidates[0].PatternName)
			}
		})
	}
}

func TestSameVisual(t *testing.T) {
	cases := []struct {
		pattern, other string
		want           bool
	}{
		{"process-flow", "process_flow", true},
		{"timeline-horizontal", "timeline", true},
		{"matrix-2x2", "matrix_2x2", true},
		{"bmc-canvas", "business_model_canvas", true},
		{"team-bios", "org_chart", false},
		{"comparison-2col", "bar", false},
		{"", "org_chart", false},
		{"team-bios", "", false},
	}
	for _, c := range cases {
		if got := sameVisual(c.pattern, c.other); got != c.want {
			t.Errorf("sameVisual(%q, %q) = %v, want %v", c.pattern, c.other, got, c.want)
		}
	}
}
