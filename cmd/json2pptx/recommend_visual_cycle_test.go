package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// The discovery journey of the circular family (go-slide-creator-53v5u,
// go-slide-creator-9d1mp): an agent that has never heard of these layouts
// reaches a working render_deck_spec call from recommend_visual,
// list_slide_kinds and the findings of a payload that does not fit.

// recommend_visual for each family intent leads with the family's pattern,
// names the cycle kind and hands a render_deck_spec call whose slide carries
// the matching style.
func TestRecommendVisualCycleIntentsReturnTheKindRecipe(t *testing.T) {
	mc := refusalTestConfig(t)
	cases := []struct {
		intent, pattern string
		style           any // nil: the default ring
	}{
		{"flywheel", "cycle-ring", nil},
		{"customer lifecycle", "cycle-ring", nil},
		{"hub and spoke operating model", "radial-hub", "radial"},
		{"devops loop", "cycle-figure-eight", "figure_eight"},
		{"onion model of security layers", "concentric-rings", "concentric"},
		{"onboarding then recurring service cycle", "cycle-intake", "intake"},
		{"six-phase continuous improvement loop fed by a two-step onboarding intake", "cycle-intake", "intake"},
	}
	for _, tc := range cases {
		t.Run(tc.intent, func(t *testing.T) {
			res := mustCall(t, mc.handleRecommendVisual, map[string]any{"intent": tc.intent, "template": "midnight-blue"})
			var rec struct {
				Candidates []struct {
					Category     string         `json:"category"`
					Name         string         `json:"name"`
					DataContract map[string]any `json:"data_contract"`
					DeckSpec     *struct {
						Kind string `json:"kind"`
					} `json:"deckspec"`
					NextToolCall *struct {
						Tool         string         `json:"tool"`
						ArgsTemplate map[string]any `json:"args_template"`
					} `json:"next_tool_call"`
				} `json:"candidates"`
			}
			structuredInto(t, res.StructuredContent, &rec)
			if len(rec.Candidates) == 0 {
				t.Fatal("no candidates")
			}
			top := rec.Candidates[0]
			if top.Name != tc.pattern {
				t.Fatalf("first candidate is %s %s, want %s", top.Category, top.Name, tc.pattern)
			}
			if top.DeckSpec == nil || top.DeckSpec.Kind != "cycle" {
				t.Fatalf("deckspec = %+v, want kind cycle", top.DeckSpec)
			}
			if form, _ := top.DataContract["form"].(string); form != patterns.ContractFormKind {
				t.Errorf("data_contract.form = %q, want the kind form (not raw-only)", form)
			}
			if top.NextToolCall == nil || top.NextToolCall.Tool != "render_deck_spec" {
				t.Fatalf("next_tool_call = %+v, want render_deck_spec", top.NextToolCall)
			}
			spec, _ := top.NextToolCall.ArgsTemplate["spec"].(map[string]any)
			list, _ := spec["slides"].([]any)
			if len(list) != 1 {
				t.Fatalf("recipe spec = %v", spec)
			}
			slide, _ := list[0].(map[string]any)
			if slide["kind"] != "cycle" || slide["style"] != tc.style {
				t.Errorf("recipe slide kind %v style %v, want cycle / %v", slide["kind"], slide["style"], tc.style)
			}
		})
	}
}

// list_slide_kinds answers for the kind: the catalogue line names what it
// draws, the named kind returns its example and the phases' aliases, and its
// preview — asked for alone — is one recipe per style, each compiling to that
// style's pattern.
func TestListSlideKindsCycle(t *testing.T) {
	mc := refusalTestConfig(t)
	type row struct {
		Kind            string              `json:"kind"`
		Summary         string              `json:"summary"`
		RequiredAliases map[string][]string `json:"required_aliases"`
		Example         map[string]any      `json:"example"`
		ItemSchema      map[string]any      `json:"item_schema"`
	}
	var listing struct {
		SlideKinds []row `json:"slide_kinds"`
	}
	find := func(args map[string]any) row {
		structuredInto(t, mustCall(t, mc.handleListSlideKinds, args).StructuredContent, &listing)
		for _, r := range listing.SlideKinds {
			if r.Kind == "cycle" {
				return r
			}
		}
		t.Fatalf("list_slide_kinds %v has no cycle row", args)
		return row{}
	}

	catalogue := find(map[string]any{})
	for _, word := range []string{"loop", "hub", "nested rings"} {
		if !strings.Contains(catalogue.Summary, word) {
			t.Errorf("the catalogue line does not say %q: %s", word, catalogue.Summary)
		}
	}
	if got := catalogue.RequiredAliases["phases"]; len(got) != 2 || got[0] != "steps" || got[1] != "items" {
		t.Errorf("required_aliases = %v, want phases: steps, items", catalogue.RequiredAliases)
	}

	named := find(map[string]any{"kinds": []any{"cycle"}})
	if named.Example["kind"] != "cycle" {
		t.Errorf("example = %v", named.Example)
	}
	for _, style := range slides.CycleStyles {
		if !strings.Contains(named.Summary, style) {
			t.Errorf("the kind's summary does not name style %s: %s", style, named.Summary)
		}
	}
	schema := find(map[string]any{"kinds": []any{"cycle"}, "fields": []any{"item_schema"}})
	props, _ := schema.ItemSchema["properties"].(map[string]any)
	style, _ := props["style"].(map[string]any)
	if enum, _ := style["enum"].([]any); len(enum) != len(slides.CycleStyles) {
		t.Errorf("style enum = %v, want %v", style["enum"], slides.CycleStyles)
	}
	for _, alias := range []string{"steps", "items"} {
		if _, own := props[alias]; own {
			t.Errorf("the compact schema lists alias %s as a field of its own", alias)
		}
	}

	reqs := kindPreviewRequests([]string{"cycle"}, "midnight-blue")
	if len(reqs) != len(slides.CycleStyles) || len(reqs) > maxStylePreviewImages {
		t.Fatalf("%d preview recipes for %d styles (ceiling %d)", len(reqs), len(slides.CycleStyles), maxStylePreviewImages)
	}
	wantPattern := map[string]string{}
	for pattern, s := range cycleStyleOfPattern {
		wantPattern[s] = pattern
	}
	for i, req := range reqs {
		style := slides.CycleStyles[i]
		raw, err := json.Marshal(req.Spec)
		if err != nil {
			t.Fatal(err)
		}
		spec, diags := semantic.ParseJSON(raw)
		if spec == nil {
			t.Fatalf("%s: %v", req.Name, diags)
		}
		if findings := semantic.Validate(spec, semantic.StrictnessStrict); len(findings) > 0 {
			t.Errorf("%s: the example does not validate clean under strict: %+v", req.Name, findings)
		}
		deck, _, err := semantic.Compile(spec, semantic.CompileOptions{})
		if err != nil || len(deck.Slides) != 1 || deck.Slides[0].Pattern == nil {
			t.Fatalf("%s: does not compile to one pattern slide: %v", req.Name, err)
		}
		if got := deck.Slides[0].Pattern.Name; got != wantPattern[style] || !strings.HasSuffix(req.Name, style) {
			t.Errorf("%s compiles to %s, want %s", req.Name, got, wantPattern[style])
		}
	}
	// Beside another kind the preview stays one image per kind.
	if reqs := kindPreviewRequests([]string{"cycle", "process"}, ""); len(reqs) != 2 {
		t.Errorf("two kinds gave %d previews", len(reqs))
	}
}

// cycleJourneySpec is a three-slide deck around one cycle slide.
func cycleJourneySpec(cycle map[string]any) map[string]any {
	return map[string]any{
		"meta": map[string]any{"title": "Operating rhythm", "template": "midnight-blue", "source": "Illustrative"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Operating rhythm", "subtitle": "Operations review, October 2026"},
			cycle,
		},
	}
}

// validate_deck_spec and render_deck_spec say the same thing about a loop that
// does not fit, at the authored field, with where the content belongs; and
// say nothing about one that does.
func TestCycleFindingsAgreeBetweenValidateAndRender(t *testing.T) {
	mc := refusalTestConfig(t)
	// both runs the spec through validate and render, holds them to the same
	// findings (code, path, severity) and returns validate's semantic ones.
	both := func(t *testing.T, cycle map[string]any) map[string]string {
		t.Helper()
		v := deckSpecVerdicts(t, mc, map[string]any{"spec": cycleJourneySpec(cycle)})
		for _, problem := range findingParityProblems(t, t.Name(), v) {
			t.Error(problem)
		}
		out := map[string]string{}
		for _, f := range v.Validate.Findings {
			if strings.Contains(f.Code, "SEMANTIC_") && f.Path != nil {
				out[*f.Path] = f.Code + ": " + f.Message
			}
		}
		return out
	}
	nine := []any{"Plan", "Source", "Build", "Test", "Release", "Deploy", "Operate", "Monitor", "Learn"}

	t.Run("nine phases", func(t *testing.T) {
		got := both(t, map[string]any{"kind": "cycle", "title": "Nine phases run the service", "phases": nine, "takeaway": "Nine is one too many."})
		msg := got["/slides/1/phases"]
		for _, want := range []string{"SEMANTIC_PATTERN_DEGRADED", "4–8", "found 9", "two cycle slides"} {
			if !strings.Contains(msg, want) {
				t.Errorf("the finding at /slides/1/phases lacks %q: %q (all: %v)", want, msg, got)
			}
		}
	})
	t.Run("figure eight in a half-width region", func(t *testing.T) {
		got := both(t, map[string]any{
			"kind": "regions", "title": "Build and run beside the numbers", "takeaway": "The loop is the story.",
			"regions": []any{
				map[string]any{"kind": "cycle", "style": "figure_eight", "phases": nine[:8]},
				map[string]any{"kind": "text", "bullets": []any{"Release weekly", "Operate daily"}},
			},
		})
		msg := got["/slides/1/regions/0/style"]
		for _, want := range []string{"SEMANTIC_DENSITY", "full slide width", "style ring", "arrangement rows"} {
			if !strings.Contains(msg, want) {
				t.Errorf("the finding at /slides/1/regions/0/style lacks %q: %q (all: %v)", want, msg, got)
			}
		}
	})
	t.Run("a style's field on another style", func(t *testing.T) {
		got := both(t, map[string]any{"kind": "cycle", "title": "A ring has no lobes", "phases": nine[:4], "left_label": "Build", "takeaway": "One loop."})
		if msg := got["/slides/1/left_label"]; !strings.Contains(msg, "SEMANTIC_UNKNOWN_FIELD") || !strings.Contains(msg, "style: figure_eight") {
			t.Errorf("left_label on a ring: %q (all: %v)", msg, got)
		}
	})
	for _, style := range slides.CycleStyles {
		t.Run("clean "+style, func(t *testing.T) {
			if got := both(t, semantic.CycleStyleExample(style)); len(got) != 0 {
				t.Errorf("the %s example raises %v", style, got)
			}
		})
	}
}

// The schema embedded in validate_deck_spec accepts every style's example and
// a cycle region, and still refuses a field no region has: the region union
// states its shared fields once and stays closed.
func TestCompactSchemaAcceptsCycleSlidesAndRegions(t *testing.T) {
	root := map[string]any{"type": "object", "properties": map[string]any{"spec": semantic.CompactSchemaAt("#/properties/spec")}}
	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	validator := compileToolInputSchema(t, "embedded DeckSpec", raw)
	validate := func(slide any) error {
		return validator.Validate(roundTripJSON(t, map[string]any{"spec": map[string]any{"slides": []any{slide}}}))
	}
	for _, style := range slides.CycleStyles {
		if err := validate(semantic.CycleStyleExample(style)); err != nil {
			t.Errorf("style %s example: %v", style, err)
		}
	}
	regions := func(region map[string]any) map[string]any {
		return map[string]any{"kind": "regions", "title": "Loop beside the story", "regions": []any{
			region, map[string]any{"kind": "text", "heading": "Why", "body": "It compounds.", "size_pct": 40, "source": "Ops"},
		}}
	}
	loop := map[string]any{"kind": "cycle", "size_pct": 60, "heading": "Monthly", "phases": []any{"Plan", map[string]any{"label": "Do", "highlight": true}, "Check", "Act"}, "center": "Loop"}
	if err := validate(regions(loop)); err != nil {
		t.Errorf("cycle region: %v", err)
	}
	for name, bad := range map[string]map[string]any{
		"a field no region has":       {"kind": "cycle", "phases": []any{"a", "b", "c"}, "rails": []any{"x"}},
		"a slide-only alias":          {"kind": "cycle", "steps": []any{"a", "b", "c"}},
		"another region kind's field": {"kind": "stat", "value": "32%", "phases": []any{"a", "b", "c"}},
		"a share off the range":       {"kind": "cycle", "phases": []any{"a", "b", "c"}, "size_pct": 5},
		"a kind no region has":        {"kind": "roadmap", "phases": []any{"a", "b", "c"}},
		"no kind at all":              {"phases": []any{"a", "b", "c"}},
	} {
		if err := validate(regions(bad)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The family stays discoverable on the raw path too: skill-info lists every
// circular pattern with its own use_when.
func TestSkillInfoListsTheCircularFamily(t *testing.T) {
	compact, _ := buildPatternEntries("compact")
	useWhen := map[string]string{}
	for _, c := range compact {
		useWhen[c.Name] = c.UseWhen
	}
	reg := patterns.Default()
	for pattern := range cycleStyleOfPattern {
		p, ok := reg.Get(pattern)
		if !ok {
			t.Fatalf("%s is not registered", pattern)
		}
		if useWhen[pattern] == "" || useWhen[pattern] != p.UseWhen() {
			t.Errorf("skill-info %s use_when = %q, want the pattern's own", pattern, useWhen[pattern])
		}
		if kind, _ := semantic.PatternReach(pattern); kind != semantic.KindCycle {
			t.Errorf("%s reaches kind %q, want cycle", pattern, kind)
		}
	}
	if len(cycleStyleOfPattern) != len(slides.CycleStyles) {
		t.Errorf("%d patterns for %d styles", len(cycleStyleOfPattern), len(slides.CycleStyles))
	}
}
