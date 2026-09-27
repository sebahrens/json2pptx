package patterns

import "testing"

// Regression for go-slide-creator-csclk.105 / .106 / .107.
func TestRecommend_PatternLimitsAndFeatureKeywords(t *testing.T) {
	top := func(intent string, hints *ContentHints) Candidate {
		t.Helper()
		res := Recommend(Default(), intent, hints, 3)
		if len(res.Candidates) == 0 {
			t.Fatalf("%q: no candidates", intent)
		}
		return res.Candidates[0]
	}
	if c := top("numbered step strip", &ContentHints{ItemCount: 7}); c.PatternName != "numbered-step-strip" {
		t.Errorf("7 numbered steps should rank numbered-step-strip first, got %s (%.2f)", c.PatternName, c.Score)
	}
	if c := top("from-to shifts, one arrow per row", nil); c.PatternName != "comparison-2col" {
		t.Errorf("row-mapped from-to shifts should rank comparison-2col first, got %s", c.PatternName)
	}
	if c := top("phased roadmap with in-parallel workstreams", &ContentHints{ItemCount: 4}); c.PatternName != "phase-roadmap" {
		t.Errorf("in-parallel workstreams should rank phase-roadmap first, got %s", c.PatternName)
	}
	for _, c := range Recommend(Default(), "contact directory", &ContentHints{ItemCount: 30}, 5).Candidates {
		if c.PatternName == "contact-directory" && c.Score >= 1.0 {
			t.Errorf("contact-directory should not score %.2f for 30 people (limit 24)", c.Score)
		}
	}
}
