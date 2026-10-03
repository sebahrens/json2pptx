package patterns

import (
	"strings"
	"testing"
)

func findCandidate(cands []VisualCandidate, name string) *VisualCandidate {
	for i := range cands {
		if cands[i].Name == name {
			return &cands[i]
		}
	}
	return nil
}

func sharesSum(t *testing.T, comp *VisualComposition) {
	t.Helper()
	sum := 0.0
	for _, r := range comp.Regions {
		sum += r.SizePct
		if r.Compose != nil {
			sharesSum(t, r.Compose)
		}
	}
	if sum != 100 {
		t.Errorf("%s envelope shares sum to %v, want 100: %+v", comp.Direction, sum, comp.Regions)
	}
}

// TestRecommendVisual_ComposeCandidateCarriesComposition covers
// go-slide-creator-okg00: an emitted pattern-pair compose candidate carries
// its region layout (direction, size_pct shares) for the recipe builder.
func TestRecommendVisual_ComposeCandidateCarriesComposition(t *testing.T) {
	res := RecommendVisual(Default(), "panels and quote side by side on one slide", nil, 5)
	c := findCandidate(res.Candidates, "compose:pull-quote+stylish-panels")
	if c == nil {
		t.Fatalf("no panels+quote compose candidate: %+v", res.Candidates)
	}
	if c.Composition == nil {
		t.Fatal("compose candidate has no composition")
	}
	comp := c.Composition
	if comp.Direction != "horizontal" {
		t.Errorf("side-by-side intent: direction %q, want horizontal", comp.Direction)
	}
	if len(comp.Regions) != 2 || comp.Regions[0].Name != "stylish-panels" || comp.Regions[1].Name != "pull-quote" {
		t.Fatalf("regions = %+v, want stylish-panels then pull-quote", comp.Regions)
	}
	if comp.Regions[0].SizePct <= comp.Regions[1].SizePct {
		t.Errorf("the body pattern should take the larger share: %+v", comp.Regions)
	}
	sharesSum(t, comp)

	// A stacked pair puts the banner (KPI row) on top.
	stacked := pairComposition(Default(), "process-flow", "kpi-3up", "kpi metrics and the process")
	if stacked.Direction != "vertical" || stacked.Regions[0].Name != "kpi-3up" {
		t.Errorf("stacked kpi+process = %+v, want vertical with kpi-3up on top", stacked)
	}
	sharesSum(t, stacked)
}

// TestRecommendVisual_ShortlistResolvesOwnComposeName is the
// go-slide-creator-cny8c regression: the compose name recommend_visual emits
// round-trips through the candidates shortlist as a compose candidate with
// its placement and composition, and still outranks the singleton.
func TestRecommendVisual_ShortlistResolvesOwnComposeName(t *testing.T) {
	reg := Default()
	intent := "panels and quote side by side on one slide"
	first := RecommendVisual(reg, intent, nil, 5)
	if len(first.Candidates) == 0 || first.Candidates[0].Category != VisualCategoryCompose {
		t.Fatalf("normal mode should rank the composition first: %+v", first.Candidates)
	}
	emitted := first.Candidates[0]

	short := RecommendVisual(reg, intent, nil, 5, &RecommendOptions{Candidates: []string{emitted.Name, "stylish-panels"}})
	if len(short.Candidates) != 2 {
		t.Fatalf("shortlist returned %d candidates, want 2: %+v", len(short.Candidates), short.Candidates)
	}
	top := short.Candidates[0]
	if top.Name != emitted.Name || top.Category != VisualCategoryCompose {
		t.Fatalf("shortlist top = %s %q, want compose %q: %+v", top.Category, top.Name, emitted.Name, short.Candidates)
	}
	if top.Score != emitted.Score {
		t.Errorf("shortlist score %v differs from normal-mode score %v", top.Score, emitted.Score)
	}
	if top.Placement == nil || len(top.Placement.ComposableWith) != 2 {
		t.Errorf("shortlist compose lost its placement: %+v", top.Placement)
	}
	if top.Composition == nil || len(top.Composition.Regions) != 2 {
		t.Errorf("shortlist compose lost its composition: %+v", top.Composition)
	}
	if strings.Contains(top.Rationale, "Unknown candidate") {
		t.Errorf("own compose name treated as unknown: %s", top.Rationale)
	}

	// Constituent order in the supplied name does not matter.
	swapped := RecommendVisual(reg, intent, nil, 5, &RecommendOptions{Candidates: []string{"compose:stylish-panels+pull-quote"}})
	if swapped.Candidates[0].Name != emitted.Name || swapped.Candidates[0].Score == 0 {
		t.Errorf("reordered constituents not canonicalised: %+v", swapped.Candidates[0])
	}
}

// TestRecommendVisual_ShortlistRejectsMalformedCompose: malformed or unknown
// constituents return a compose entry with score 0 and the precise reason,
// and every requested name is still returned.
func TestRecommendVisual_ShortlistRejectsMalformedCompose(t *testing.T) {
	cases := map[string]string{
		"compose:":                         "lists no constituents",
		"compose:kpi-3up":                  "has one constituent",
		"compose:kpi-3up+no-such-pattern":  `"no-such-pattern" is not a named pattern`,
		"compose:kpi-3up+line":             `write the chart as "chart:line"`,
		"compose:chart:sankey+kpi-3up":     "not a ready chart type",
		"compose:diagram:nope+kpi-3up":     "not a ready diagram type",
		"compose:kpi-3up+kpi-3up":          "repeats constituent",
		"compose:kpi-3up++stat-hero":       "empty constituent",
		"compose:agenda+matrix-2x2":        "compose affinity",
		"compose:a+b+c+d+e":                "at most 4 regions",
		"compose:chart:line+stat-hero+pie": `write the chart as "chart:pie"`,
	}
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	res := RecommendVisual(Default(), "kpis with a chart on one slide", nil, 5, &RecommendOptions{Candidates: names})
	if len(res.Candidates) != len(names) {
		t.Fatalf("got %d candidates, want all %d", len(res.Candidates), len(names))
	}
	for name, want := range cases {
		c := findCandidate(res.Candidates, name)
		if c == nil {
			t.Errorf("%q missing from the shortlist result", name)
			continue
		}
		if c.Category != VisualCategoryCompose || c.Score != 0 {
			t.Errorf("%q: category %s score %v, want compose 0", name, c.Category, c.Score)
		}
		if !strings.Contains(c.Rationale, want) {
			t.Errorf("%q: rationale %q does not name the problem (%q)", name, c.Rationale, want)
		}
	}
}

// TestRecommendVisual_CompoundIntentComposesRegions covers
// go-slide-creator-lp0o8: an explicit same-slide chart + KPI + timeline brief
// yields one heterogeneous composition ranked above every partial view, with
// the nested layout and the explicit share preserved.
func TestRecommendVisual_CompoundIntentComposesRegions(t *testing.T) {
	reg := Default()
	hints := &VisualHints{ContentHints: ContentHints{ItemCount: 3, HasMetrics: true, HasChart: true, Columns: 2}}
	for _, intent := range []string{
		"line chart left 65%, KPI upper-right, timeline lower-right",
		"a single executive review slide divided into a large left revenue trend chart, a top-right kpi tile and a bottom-right action timeline. preserve all three different content types in their own regions.",
	} {
		res := RecommendVisual(reg, intent, hints, 5)
		// The DeckSpec regions kind holds these three views, so it leads and
		// the raw composition follows it (go-slide-creator-3ujfq).
		if kind := res.Candidates[0]; kind.Category != VisualCategoryKind || kind.Name != RegionsKindName {
			t.Fatalf("%q: top = %s %q, want the regions kind: %+v", intent, kind.Category, kind.Name, res.Candidates)
		}
		top := res.Candidates[1]
		if top.Category != VisualCategoryCompose || top.Name != "compose:chart:line+diagram:timeline+stat-hero" {
			t.Fatalf("%q: second = %s %q, want the chart+KPI+timeline composition: %+v", intent, top.Category, top.Name, res.Candidates)
		}
		for _, c := range res.Candidates[2:] {
			if c.Score >= top.Score {
				t.Errorf("%q: partial view %s %q (%v) ties or beats the composition (%v)", intent, c.Category, c.Name, c.Score, top.Score)
			}
		}
		comp := top.Composition
		if comp == nil || comp.Direction != "horizontal" || len(comp.Regions) != 2 {
			t.Fatalf("%q: composition = %+v, want horizontal outer split", intent, comp)
		}
		left, right := comp.Regions[0], comp.Regions[1]
		if left.Category != VisualCategoryChart || left.Name != "line" || left.Position != "left" {
			t.Errorf("%q: left region = %+v, want the line chart", intent, left)
		}
		if right.Compose == nil || right.Compose.Direction != "vertical" || len(right.Compose.Regions) != 2 {
			t.Fatalf("%q: right region = %+v, want a vertical KPI / timeline stack", intent, right)
		}
		up, low := right.Compose.Regions[0], right.Compose.Regions[1]
		if up.Name != "stat-hero" || up.Position != "upper-right" || low.Name != "timeline" || low.Category != VisualCategoryDiagram || low.Position != "lower-right" {
			t.Errorf("%q: right stack = %+v / %+v", intent, up, low)
		}
		sharesSum(t, comp)
		if top.Placement == nil || len(top.Placement.ComposableWith) != 3 {
			t.Errorf("%q: placement = %+v", intent, top.Placement)
		}
	}

	// The explicit 65% share survives.
	res := RecommendVisual(reg, "line chart left 65%, KPI upper-right, timeline lower-right", hints, 5)
	for _, c := range res.Candidates[:2] {
		if got := c.Composition.Regions[0].SizePct; got != 65 {
			t.Errorf("%s: explicit left share = %v, want 65", c.Name, got)
		}
	}
}

// TestRecommendVisual_NoCompositionWithoutRegions: whole-slide intents keep
// their singleton recommendations — no fabricated composition.
func TestRecommendVisual_NoCompositionWithoutRegions(t *testing.T) {
	for _, intent := range []string{
		"show Q3 revenue trend",
		"three KPIs for the quarter",
		"project timeline with milestones",
		"compare revenue by region",
		"revenue trend chart and the drivers behind it",
	} {
		res := RecommendVisual(Default(), intent, nil, 5)
		for _, c := range res.Candidates {
			if c.Category == VisualCategoryCompose && strings.Contains(c.Name, "chart:") {
				t.Errorf("%q: unexpected heterogeneous composition %q", intent, c.Name)
			}
		}
	}
}

// TestRecommendVisual_ShortlistCompoundRoundTrip: the heterogeneous name
// shortlisted with its singletons keeps its layout and ranks first.
func TestRecommendVisual_ShortlistCompoundRoundTrip(t *testing.T) {
	reg := Default()
	intent := "line chart left 65%, KPI upper-right, timeline lower-right"
	first := RecommendVisual(reg, intent, nil, 5)
	name := first.Candidates[1].Name
	res := RecommendVisual(reg, intent, nil, 5, &RecommendOptions{Candidates: []string{"line", name, "stat-hero", "timeline"}})
	if len(res.Candidates) != 4 {
		t.Fatalf("got %d candidates, want 4", len(res.Candidates))
	}
	top := res.Candidates[0]
	if top.Name != name || top.Category != VisualCategoryCompose {
		t.Fatalf("shortlist top = %q, want %q: %+v", top.Name, name, res.Candidates)
	}
	if top.Composition == nil || top.Composition.Regions[0].SizePct != 65 {
		t.Errorf("shortlisted composition lost its layout: %+v", top.Composition)
	}
	for _, c := range res.Candidates[1:] {
		if c.Score >= top.Score {
			t.Errorf("partial view %q (%v) not below the composition (%v)", c.Name, c.Score, top.Score)
		}
	}

	// Shortlisted beside the composition, the regions kind leads as it does
	// in normal mode; shortlisted for a one-view intent it is ranked last.
	both := RecommendVisual(reg, intent, nil, 5, &RecommendOptions{Candidates: []string{name, "line", RegionsKindName}})
	if k, c := both.Candidates[0], both.Candidates[1]; k.Category != VisualCategoryKind || k.Name != RegionsKindName || c.Name != name || k.Score <= c.Score {
		t.Errorf("shortlist with regions = %+v, want regions then the composition", both.Candidates)
	}
	if both.Candidates[0].Composition == nil {
		t.Error("shortlisted regions kind lost its composition")
	}
	one := RecommendVisual(reg, "quarterly revenue", nil, 5, &RecommendOptions{Candidates: []string{RegionsKindName, "bar"}})
	if k := findCandidate(one.Candidates, RegionsKindName); k == nil || k.Category != VisualCategoryKind || k.Score >= 0.5 {
		t.Errorf("regions for a one-view intent = %+v, want a low-scored kind candidate", k)
	}

	// A heterogeneous name the intent does not ask for is still resolved
	// (mean score, default layout), never "unknown".
	other := RecommendVisual(reg, "quarterly revenue", nil, 5, &RecommendOptions{Candidates: []string{"compose:chart:bar+kpi-3up"}})
	c := other.Candidates[0]
	if c.Category != VisualCategoryCompose || c.Composition == nil || c.Score == 0 {
		t.Errorf("default heterogeneous resolution = %+v", c)
	}
}
