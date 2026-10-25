package patterns

import "testing"

func TestVarietyPenalizesRepeatedLookAcrossNames(t *testing.T) {
	reg := Default()
	intent := "three service pillars with features in columns or a checklist"
	opts := &RecommendOptions{Candidates: []string{"icon-row", "stylish-panels", "checklist"}}
	base := Recommend(reg, intent, &ContentHints{ItemCount: 3}, 3, opts)
	opts.PreferVariety = true
	opts.RecentPatterns = []string{"icon-row", "icon-row"}
	varied := Recommend(reg, intent, &ContentHints{ItemCount: 3}, 3, opts)
	scores := func(r RecommendResult) map[string]float64 {
		m := map[string]float64{}
		for _, c := range r.Candidates {
			m[c.PatternName] = c.Score
		}
		return m
	}
	a, b := scores(base), scores(varied)
	if b["stylish-panels"] >= a["stylish-panels"] {
		t.Fatalf("same look with a fresh name got no penalty: %v -> %v", a, b)
	}
	if b["checklist"] != a["checklist"] {
		t.Fatalf("different motif unfairly penalized: %v -> %v", a, b)
	}
	// A requested shortlist also returns weak matches; recency cannot turn
	// their public 0..1 scores negative.
	unmatched := Recommend(reg, "unrelated evidence", nil, 3, opts)
	if len(unmatched.Candidates) != 3 {
		t.Fatalf("shortlist candidates missing: %+v", unmatched)
	}
	for _, c := range unmatched.Candidates {
		if c.Score < 0 || c.Score > 1 {
			t.Errorf("invalid recommendation score: %+v", c)
		}
	}
}
