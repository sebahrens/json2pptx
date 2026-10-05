package patterns

import "testing"

// TestRecommendVisualConsultingIntents pins go-slide-creator-biw2e: the
// common consulting intents the 2026-10-01 review found misranked, with no
// hints (an agent usually calls recommend_visual with the intent alone).
func TestRecommendVisualConsultingIntents(t *testing.T) {
	reg := Default()
	rank := func(intent string) map[string]int {
		got := RecommendVisual(reg, intent, nil, 8)
		out := map[string]int{}
		for i, c := range got.Candidates {
			if _, seen := out[c.Name]; !seen {
				out[c.Name] = i
			}
		}
		return out
	}
	score := func(intent, name string) float64 {
		for _, c := range RecommendVisual(reg, intent, nil, 8).Candidates {
			if c.Name == name {
				return c.Score
			}
		}
		return -1
	}
	above := func(t *testing.T, intent, better, worse string) {
		t.Helper()
		r := rank(intent)
		b, okB := r[better]
		w, okW := r[worse]
		if !okB {
			t.Errorf("%q: %s missing from candidates %v", intent, better, r)
			return
		}
		if okW && w < b {
			t.Errorf("%q: %s (rank %d) outranks %s (rank %d)", intent, worse, w, better, b)
		}
	}

	t.Run("comparison and trend beat stat-hero", func(t *testing.T) {
		above(t, "compare revenue across 5 regions", "bar", "stat-hero")
		above(t, "show revenue trend over the last 8 quarters", "line", "stat-hero")
		// One number still is a hero.
		if r := rank("one big number that matters"); r["stat-hero"] != 0 {
			t.Errorf("stat-hero lost the single-number intent: %v", r)
		}
	})
	t.Run("org chart beats team-bios", func(t *testing.T) {
		above(t, "org chart of the leadership team", "org_chart", "team-bios")
		above(t, "reporting structure of the new business unit", "org_chart", "team-bios")
		if r := rank("our team"); r["team-bios"] != 0 {
			t.Errorf("team-bios lost the people intent: %v", r)
		}
	})
	t.Run("financial tables get a native table", func(t *testing.T) {
		for _, intent := range []string{"financial table of P&L for 3 years", "P&L for the last three years"} {
			if s := score(intent, "table"); s < 0.85 {
				t.Errorf("%q: table score %.2f, want >= 0.85", intent, s)
			}
			if s := score(intent, "table-highlight"); s < 0.85 {
				t.Errorf("%q: table-highlight score %.2f, want >= 0.85", intent, s)
			}
			if r := rank(intent); r["table"] != 0 {
				t.Errorf("%q: table is not first: %v", intent, r)
			}
		}
	})
	t.Run("executive summary beats bullets", func(t *testing.T) {
		above(t, "executive summary of key findings", "exec-summary", "content")
	})
	t.Run("appendix is plain", func(t *testing.T) {
		r := rank("appendix of detailed financials")
		for _, want := range []string{"section", "content", "table"} {
			if _, ok := r[want]; !ok {
				t.Errorf("appendix: %s missing: %v", want, r)
			}
		}
		if i, ok := r["hero-detail"]; ok && i < 3 {
			t.Errorf("appendix: hero-detail still ranks %d: %v", i, r)
		}
	})
	t.Run("risk matrix", func(t *testing.T) {
		r := rank("risk matrix of likelihood vs impact for 6 risks")
		if _, ok := r["matrix-2x2"]; !ok {
			t.Errorf("matrix-2x2 missing: %v", r)
		}
		if _, ok := r["table-highlight"]; !ok {
			t.Errorf("table-highlight missing: %v", r)
		}
		if _, ok := r["comparison-2col"]; ok {
			t.Errorf("comparison-2col still offered: %v", r)
		}
		above(t, "key risks and mitigations", "table", "card-grid")
	})
	// go-slide-creator-ec74l: named risks on likelihood × impact are a risk
	// heat map; the 2x2 that puts every "medium" on an axis line and the
	// numbers-only heatmap diagram rank under it.
	t.Run("risk heat map", func(t *testing.T) {
		for _, intent := range []string{
			"likelihood x impact risk heat map with six named risks",
			"risk matrix of likelihood vs impact for 6 risks",
			"risk heatmap: top risks by likelihood and impact (low / medium / high)",
		} {
			if r := rank(intent); r["risk-heatmap"] != 0 {
				t.Errorf("%q: risk-heatmap is not first: %v", intent, r)
			}
			above(t, intent, "risk-heatmap", "matrix-2x2")
			above(t, intent, "risk-heatmap", "heatmap")
			above(t, intent, "risk-heatmap", "capability-heatmap")
		}
		// Other heat maps and other matrices keep their own answer.
		if r := rank("automation potential by function"); r["capability-heatmap"] != 0 {
			t.Errorf("capability-heatmap lost its intent: %v", r)
		}
		above(t, "impact vs effort quadrant", "matrix-2x2", "risk-heatmap")
		above(t, "key risks and mitigations with an owner per risk", "table", "risk-heatmap")
	})
}

// TestRecommendVisualShortlistTableResolves: the new table candidate is a
// placeholder name callers may shortlist.
func TestRecommendVisualShortlistTableResolves(t *testing.T) {
	got := RecommendVisual(Default(), "P&L for three years", nil, 5, &RecommendOptions{Candidates: []string{"table", "stat-hero"}})
	if len(got.Candidates) != 2 || got.Candidates[0].Name != "table" || got.Candidates[0].Category != VisualCategoryPlaceholder {
		t.Fatalf("shortlist = %+v", got.Candidates)
	}
}
