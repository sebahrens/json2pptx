package patterns

import "testing"

// go-slide-creator-s49nz: three to six short parallel points are a pattern
// that fills the page, not the bullets layout, which set five short lines in
// the top third of the slide at body size.
func TestRecommendVisualSteersShortPointListsToPatterns(t *testing.T) {
	reg := Default()
	ranked := func(intent string, hints *VisualHints) []VisualCandidate {
		return RecommendVisual(reg, intent, hints, 8).Candidates
	}
	index := func(cs []VisualCandidate, name string) int {
		for i, c := range cs {
			if c.Name == name {
				return i
			}
		}
		return -1
	}

	for _, intent := range []string{
		"five key points about why churn fell this quarter",
		"a bullet list of four reasons the pilot succeeded",
		"three short findings from the customer interviews",
		"summarise the main takeaways of the review as bullets",
	} {
		cs := ranked(intent, nil)
		if len(cs) == 0 || cs[0].Name != "exec-summary" {
			t.Errorf("%q: top candidate = %+v, want exec-summary", intent, cs)
			continue
		}
		for _, name := range []string{"labeled-rows", "card-grid"} {
			if index(cs, name) < 0 {
				t.Errorf("%q: %s missing from %v", intent, name, cs)
			}
		}
		if i := index(cs, "content"); i >= 0 {
			if cs[0].Score-cs[i].Score < pointListBulletsGap-0.001 {
				t.Errorf("%q: the bullets layout (%.2f) is a near tie with the lead (%.2f)", intent, cs[i].Score, cs[0].Score)
			}
			for _, name := range []string{"exec-summary", "labeled-rows", "card-grid"} {
				if index(cs, name) > i {
					t.Errorf("%q: %s ranks under the bullets layout", intent, name)
				}
			}
		}
	}

	// Six points are one more than exec-summary holds: card-grid leads.
	if cs := ranked("six key points on the operating model", nil); len(cs) == 0 || cs[0].Name != "card-grid" || index(cs, "exec-summary") == 0 {
		t.Errorf("six points: top = %+v, want card-grid", cs)
	}
	if cs := ranked("key points on the operating model", &VisualHints{ContentHints: ContentHints{ItemCount: 6}}); len(cs) == 0 || cs[0].Name != "card-grid" {
		t.Errorf("item_count 6: top = %+v, want card-grid", cs)
	}

	// The bullets layout stays the answer where it is the right one, and a
	// more specific reading of the intent is left alone.
	for intent, wantTop := range map[string]string{
		"nine detailed bullet points for the appendix": "",
		"twelve talking points for the town hall":      "content",
		"long paragraph of notes with bullets":         "content",
		"agenda for the workshop":                      "agenda",
		"show Q3 revenue trend":                        "line",
		"key risks and mitigations":                    "table",
	} {
		cs := ranked(intent, nil)
		if i := index(cs, "exec-summary"); i == 0 {
			t.Errorf("%q: exec-summary leads, want the routing to stay out (%v)", intent, cs)
		}
		if wantTop != "" && (len(cs) == 0 || cs[0].Name != wantTop) {
			t.Errorf("%q: top = %+v, want %s", intent, cs, wantTop)
		}
	}

	// Agreed actions are the next-steps pattern, whatever "team" matched.
	if cs := ranked("list six short actions the team agreed", nil); len(cs) == 0 || cs[0].Name != "next-steps" {
		t.Errorf("agreed actions: top = %+v, want next-steps", cs)
	}
	// A shortlist the caller named is scored by the same model and gains no
	// candidate it did not ask about.
	short := RecommendVisual(reg, "five key points about why churn fell", nil, 8, &RecommendOptions{Candidates: []string{"content", "kpi-3up"}}).Candidates
	if index(short, "exec-summary") >= 0 {
		t.Errorf("shortlist gained exec-summary: %v", short)
	}
}
