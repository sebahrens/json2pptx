package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func pillarsSpec(body map[string]any) *DeckSpec {
	return &DeckSpec{Meta: DeckMeta{Title: "Strategy"}, Slides: []SlideSpec{{Kind: KindPillars, Body: body}}}
}

func pillarItems() []any {
	return []any{
		map[string]any{"title": "Trust", "body": []any{"Resilience", "Transparent pricing"}},
		map[string]any{"title": "Speed", "body": []any{"Weekly releases"}},
		map[string]any{"title": "Scale", "body": []any{"Shared platform"}},
	}
}

func TestPillarsPatternParity(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		body          map[string]any
	}{
		{"strategy house", "strategy-house", map[string]any{"title": "Our strategy", "pillars": pillarItems(), "objective": "Trusted platform", "foundation": "People and data", "roof_badges": []any{"Vision"}, "takeaway": "Three pillars carry the plan."}},
		{"panels", "stylish-panels", map[string]any{"title": "Our capabilities", "pillars": pillarItems(), "takeaway": "The three capabilities reinforce each other."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := Normalize(pillarsSpec(tc.body))
			if got := plan.Slides[0].Visual.Pattern; got != tc.pattern {
				t.Fatalf("plan pattern = %q, want %q", got, tc.pattern)
			}
			input, result, err := Compile(pillarsSpec(tc.body), CompileOptions{})
			if err != nil {
				t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
			}
			p := input.Slides[0].Pattern
			if p == nil || p.Name != tc.pattern {
				t.Fatalf("compiled pattern = %+v", p)
			}
			pat, ok := patterns.Default().Get(p.Name)
			if !ok {
				t.Fatal("unregistered pattern")
			}
			values := pat.NewValues()
			if err := json.Unmarshal(p.Values, values); err != nil {
				t.Fatal(err)
			}
			if err := pat.Validate(values, nil, nil); err != nil {
				t.Fatalf("pattern values: %v", err)
			}
			if hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
				t.Fatalf("unexpected degradation: %+v", result.Diagnostics)
			}
		})
	}
}

func TestPillarsFallbackPreservesFramingAndBullets(t *testing.T) {
	body := map[string]any{"title": "Strategy", "objective": "Trusted platform", "pillars": pillarItems(), "takeaway": "Trust needs three capabilities."}
	plan := Normalize(pillarsSpec(body))
	if plan.Slides[0].Visual.Pattern != "" || plan.Slides[0].Visual.Layout != "content" {
		t.Fatalf("fallback plan = %+v", plan.Slides[0].Visual)
	}
	input, result, err := Compile(pillarsSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.Slides[0].Pattern != nil {
		t.Fatal("partial house should fall back")
	}
	var joined string
	for _, c := range input.Slides[0].Content {
		if c.BulletsValue != nil {
			joined = strings.Join(*c.BulletsValue, " | ")
		}
	}
	for _, want := range []string{"Trusted platform", "Trust", "Resilience", "Transparent pricing", "Speed", "Scale"} {
		if !strings.Contains(joined, want) {
			t.Errorf("fallback lost %q: %s", want, joined)
		}
	}
	if !hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
		t.Fatalf("missing degrade finding: %+v", result.Diagnostics)
	}
	assertPillarsDegradedFrom(t, result.Diagnostics, "strategy-house")
}

func TestPillarsMalformedItemHasSemanticPath(t *testing.T) {
	items := pillarItems()
	items[1].(map[string]any)["body"] = []any{"Weekly releases", 12}
	input, result, err := Compile(pillarsSpec(map[string]any{"pillars": items}), CompileOptions{})
	if err == nil || input != nil {
		t.Fatalf("malformed pillar compiled: %+v", input)
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Path == "slides[0].pillars[1].body[1]" && d.Severity == diagnostics.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing semantic path: %+v", result.Diagnostics)
	}
}

func TestPillarsOverBudgetKeepsFoundationAndBadges(t *testing.T) {
	items := pillarItems()
	items = append(items, map[string]any{"title": "Fourth", "body": []any{"Detail"}}, map[string]any{"title": "Fifth", "body": []any{"Detail"}}, map[string]any{"title": "Sixth", "body": []any{"Detail"}})
	body := map[string]any{"pillars": items, "objective": "Trusted platform", "foundation": "People and data", "roof_badges": []any{"Vision"}, "takeaway": "Six themes guide delivery."}
	input, result, err := Compile(pillarsSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.Slides[0].Pattern != nil {
		t.Fatal("six pillars should fall back")
	}
	var joined string
	for _, c := range input.Slides[0].Content {
		if c.BulletsValue != nil {
			joined = strings.Join(*c.BulletsValue, " | ")
		}
	}
	for _, want := range []string{"Trusted platform", "People and data", "Vision", "Sixth"} {
		if !strings.Contains(joined, want) {
			t.Errorf("fallback lost %q: %s", want, joined)
		}
	}
	if !hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
		t.Fatalf("missing degrade finding: %+v", result.Diagnostics)
	}
	assertPillarsDegradedFrom(t, result.Diagnostics, "strategy-house")
}

func TestPillarsDiscoverySchema(t *testing.T) {
	ex := KindExample(KindPillars)
	if ex == nil || ex["kind"] != "pillars" {
		t.Fatalf("example = %+v", ex)
	}
	variant := KindItemSchema(KindPillars)
	if variant["additionalProperties"] != false {
		t.Fatal("open pillars schema")
	}
	props := variant["properties"].(map[string]any)
	items := props["pillars"].(map[string]any)["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Fatal("open pillar item schema")
	}
	body := items["properties"].(map[string]any)["body"].(map[string]any)
	if body["type"] != "array" {
		t.Fatalf("body schema = %+v", body)
	}
}

func TestPillarsExplicitContentLayoutUsesFallback(t *testing.T) {
	body := map[string]any{"pillars": pillarItems(), "layout": "content", "takeaway": "Trust needs three capabilities."}
	plan := Normalize(pillarsSpec(body))
	if plan.Slides[0].Visual.Pattern != "" || plan.Slides[0].Visual.Layout != "content" {
		t.Fatalf("override plan = %+v", plan.Slides[0].Visual)
	}
	input, result, err := Compile(pillarsSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v, %+v", err, result.Diagnostics)
	}
	if input.Slides[0].Pattern != nil || input.Slides[0].LayoutID != "content" {
		t.Fatalf("override compiled = %+v", input.Slides[0])
	}
}

// TestPillarsHouseLevels pins go-slide-creator-bjxb9 for the pillars kind: a
// listed foundation and a beam compile to the strategy-house's levels, and
// every level and cell maps back to its DeckSpec path.
func TestPillarsHouseLevels(t *testing.T) {
	items := append(pillarItems(), map[string]any{"title": "Reach", "body": []any{"Partner network"}})
	body := map[string]any{
		"title": "Four pillars on one platform", "pillars": items, "objective": "Trusted platform",
		"beam":       "One operating model",
		"foundation": []any{"Shared data platform", []any{"People", "Data", "Controls"}},
		"takeaway":   "Four pillars rest on one platform and three enablers.",
	}
	plan := Normalize(pillarsSpec(body))
	if got := plan.Slides[0].Visual.Pattern; got != "strategy-house" {
		t.Fatalf("plan pattern = %q", got)
	}
	input, result, err := Compile(pillarsSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
	}
	if hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
		t.Fatalf("unexpected degradation: %+v", result.Diagnostics)
	}
	p := input.Slides[0].Pattern
	pat, _ := patterns.Default().Get(p.Name)
	values := pat.NewValues()
	if err := json.Unmarshal(p.Values, values); err != nil {
		t.Fatal(err)
	}
	if err := pat.Validate(values, nil, nil); err != nil {
		t.Fatalf("pattern values: %v", err)
	}
	house := values.(*patterns.StrategyHouseValues)
	if house.Beam != "One operating model" || len(house.Pillars) != 4 || len(house.FoundationLayers) != 2 || len(house.FoundationLayers[1]) != 3 {
		t.Fatalf("house = %+v", house)
	}
	for raw, want := range map[string]string{
		"slides[0].pattern.values.beam":             "slides[0].beam",
		"slides[0].pattern.values.foundation":       "slides[0].foundation",
		"slides[0].pattern.values.foundation[0]":    "slides[0].foundation[0]",
		"slides[0].pattern.values.foundation[1]":    "slides[0].foundation[1]",
		"slides[0].pattern.values.foundation[1][2]": "slides[0].foundation[1][2]",
		"slides[0].pattern.values.pillars[3].title": "slides[0].pillars",
	} {
		if got, _, ok := result.SourceMap.ResolveSemantic(raw); !ok || got != want {
			t.Errorf("source path %s -> %q (mapped %t), want %q", raw, got, ok, want)
		}
	}
}

// TestPillarsHouseLevelProblems: a malformed level is a typed error at its
// path; more levels or cells than a house keeps readable degrade to bullets
// that keep every enabler.
func TestPillarsHouseLevelProblems(t *testing.T) {
	house := func(foundation any) map[string]any {
		return map[string]any{"pillars": pillarItems(), "objective": "Trusted platform", "foundation": foundation, "takeaway": "Three pillars carry the plan."}
	}
	for _, tc := range []struct {
		name       string
		foundation any
		path       string
	}{
		{"object level", []any{map[string]any{"label": "People"}}, "slides[0].foundation[0]"},
		{"number cell", []any{[]any{"People", 3}}, "slides[0].foundation[0][1]"},
		{"number foundation", 7, "slides[0].foundation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, result, err := Compile(pillarsSpec(house(tc.foundation)), CompileOptions{})
			if err == nil || input != nil {
				t.Fatalf("malformed foundation compiled: %+v", input)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.Path == tc.path && d.Severity == diagnostics.SeverityError {
					found = true
				}
			}
			if !found {
				t.Fatalf("no error at %s: %+v", tc.path, result.Diagnostics)
			}
		})
	}

	tooMany := house([]any{"One", "Two", "Three", []any{"People", "Data"}})
	input, result, err := Compile(pillarsSpec(tooMany), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.Slides[0].Pattern != nil || !hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
		t.Fatalf("four foundation levels should degrade: %+v", result.Diagnostics)
	}
	var joined string
	for _, c := range input.Slides[0].Content {
		if c.BulletsValue != nil {
			joined = strings.Join(*c.BulletsValue, " | ")
		}
	}
	for _, want := range []string{"Foundation: One", "Foundation: Three", "Foundation: People · Data"} {
		if !strings.Contains(joined, want) {
			t.Errorf("fallback lost %q: %s", want, joined)
		}
	}
}

// TestPillarsExampleIsNotTheDefaultSilhouette pins go-slide-creator-qad87: the
// kind example shows a pillar count and levels taken from content — not three
// equal pillars over one joined string — and compiles clean.
func TestPillarsExampleIsNotTheDefaultSilhouette(t *testing.T) {
	ex := KindExample(KindPillars)
	pillars := ex["pillars"].([]any)
	if len(pillars) == 3 {
		t.Error("the example still has three pillars")
	}
	counts := map[int]bool{}
	for _, p := range pillars {
		counts[len(p.(map[string]any)["body"].([]any))] = true
	}
	if len(counts) < 2 {
		t.Error("every example pillar has the same number of bullets")
	}
	if _, isString := ex["foundation"].(string); isString {
		t.Error("the example foundation is one string")
	}
	body := map[string]any{}
	for k, v := range ex {
		if k != "kind" {
			body[k] = v
		}
	}
	input, result, err := Compile(pillarsSpec(body), CompileOptions{})
	if err != nil || input.Slides[0].Pattern == nil || input.Slides[0].Pattern.Name != "strategy-house" {
		t.Fatalf("example does not compile to a house: %v %+v", err, result.Diagnostics)
	}
	fields := kindPayloadFields[KindPillars]
	for field, want := range map[string]string{"pillars": "one pillar per independent theme", "foundation": "one level per kind of enabler"} {
		if !strings.Contains(fields[field].desc, want) {
			t.Errorf("%s description does not say how to choose the shape: %q", field, fields[field].desc)
		}
	}
}

// assertPillarsDegradedFrom checks the pattern a degraded pillars slide names
// as the one whose budget it missed.
func assertPillarsDegradedFrom(t *testing.T, diags []diagnostics.Diagnostic, want string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == diagnostics.CodeSemanticPatternDegraded && d.Fix != nil {
			if got := d.Fix.Params["from"]; got != want {
				t.Errorf("degraded from = %v, want %s", got, want)
			}
			return
		}
	}
	t.Errorf("no degrade finding with a fix: %+v", diags)
}

// TestPillarsDegradedPanelsNameThePanels: pillars with no house frame that
// miss the panels' range name stylish-panels, not both visuals
// (go-slide-creator-vag44).
func TestPillarsDegradedPanelsNameThePanels(t *testing.T) {
	items := append(pillarItems(), map[string]any{"title": "Fourth", "body": []any{"Detail"}}, map[string]any{"title": "Fifth", "body": []any{"Detail"}}, map[string]any{"title": "Sixth", "body": []any{"Detail"}})
	_, result, err := Compile(pillarsSpec(map[string]any{"pillars": items, "takeaway": "Six themes guide delivery."}), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	assertPillarsDegradedFrom(t, result.Diagnostics, "stylish-panels")
}
