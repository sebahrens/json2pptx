package deckplan

import "testing"

// go-slide-creator-s49nz: an outline item that asks for a list of points is
// a structured points slide — never the single number its count reads as —
// and an inline list's own numbering is not part of its items.
func TestOutlinePointItemsAreStructuredSlides(t *testing.T) {
	for text, want := range map[string]string{
		"five key points on why churn fell as bullets": "card-grid",
		"four reasons the pilot succeeded":             "card-grid",
		"three findings from customer interviews":      "exec-summary",
		"key takeaways":     "exec-summary",
		"5 lessons learned": "exec-summary",
		// A more specific cue still wins, and a bare figure is still a stat.
		"revenue chart with key points": "chart-insights-split",
		"87% retention":                 "stat-hero",
		"executive summary":             "exec-summary",
	} {
		def := outlineKindFor(text)
		if def.pattern != want {
			t.Errorf("%q: pattern %q (kind %s), want %s", text, def.pattern, def.kind, want)
		}
	}
	if def := outlineKindFor("three findings from customer interviews"); def.kind != "pillars" || def.slot != "topic" {
		t.Errorf("a findings item is a pillars topic slide in a DeckSpec draft, got %+v", def)
	}

	o := parseOutline("Pilot review. Slides: 1) title; 2) five key points on why churn fell as bullets; 3) revenue chart; 4) three findings from customer interviews; 5) key takeaways; 6) next steps", 0)
	if o == nil {
		t.Fatal("the numbered inline list was not read as an outline")
	}
	var got []string
	for _, item := range o.items {
		got = append(got, item.def.pattern)
		if item.text != "" && item.text[0] >= '0' && item.text[0] <= '9' {
			t.Errorf("outline item kept its enumerator: %q", item.text)
		}
	}
	want := []string{"card-grid", "chart-insights-split", "exec-summary", "exec-summary", "next-steps"}
	if len(got) != len(want) {
		t.Fatalf("outline patterns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("outline patterns = %v, want %v", got, want)
			break
		}
	}
	if cleanOutlineItem("1.5x growth in two years") != "1.5x growth in two years" {
		t.Error("a figure that opens an item is not an enumerator")
	}
}
