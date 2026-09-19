package slides

import "testing"

// A lone insight that IS the takeaway printed the same sentence twice on one
// slide: once as the only Key Insight bullet and once verbatim in the takeaway
// bar (go-slide-creator-pyxn).
func TestDropDuplicateTakeaway(t *testing.T) {
	const line = "R&D and S&M are 69% of spend"
	cases := []struct {
		name     string
		takeaway string
		insights []string
		want     string
	}{
		{"lone insight is the takeaway", line, []string{line}, ""},
		{"ignores case and padding", line, []string{"  r&d AND s&m are 69% OF spend "}, ""},
		{"a different takeaway is kept", "Shift spend to R&D", []string{line}, "Shift spend to R&D"},
		{"a takeaway over several insights is kept", line, []string{line, "G&A is flat"}, line},
		{"no takeaway stays empty", "", []string{line}, ""},
		{"no insights leaves it alone", line, nil, line},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dropDuplicateTakeaway(c.takeaway, c.insights); got != c.want {
				t.Errorf("dropDuplicateTakeaway(%q, %v) = %q, want %q", c.takeaway, c.insights, got, c.want)
			}
		})
	}
}

// End to end through the compiler: the band is dropped for the duplicate and
// kept when it says something the bullets do not.
func TestCompileChartInsightDropsTheDuplicateBand(t *testing.T) {
	const line = "R&D and S&M are 69% of spend"
	chart := map[string]any{
		"type": "pie",
		"data": map[string]any{"categories": []any{"R&D", "S&M"}, "values": []any{69.0, 31.0}},
	}

	dup := Input{
		Takeaway: line,
		Body:     map[string]any{"title": "Where the money goes", "chart": chart, "insights": []any{line}},
	}
	slide, _, err := CompileChartInsight(dup)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Takeaway != "" {
		t.Errorf("takeaway = %q; the same sentence is already the only insight", slide.Takeaway)
	}

	distinct := Input{
		Takeaway: "Shift spend to R&D",
		Body:     map[string]any{"title": "Where the money goes", "chart": chart, "insights": []any{line}},
	}
	slide, _, err = CompileChartInsight(distinct)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Takeaway != "Shift spend to R&D" {
		t.Errorf("takeaway = %q; a takeaway that is not the insight must survive", slide.Takeaway)
	}
}
