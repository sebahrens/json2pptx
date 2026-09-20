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
