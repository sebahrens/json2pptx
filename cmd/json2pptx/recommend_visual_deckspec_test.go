package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/svggen"
)

// recipeArgs round-trips a candidate's next_tool_call args through JSON, as
// an agent receives and resends them.
func recipeArgs(t *testing.T, c patterns.VisualCandidate) map[string]any {
	t.Helper()
	if c.NextToolCall == nil {
		t.Fatalf("%s %q has no next_tool_call", c.Category, c.Name)
	}
	raw, err := json.Marshal(c.NextToolCall.ArgsTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	return args
}

// compileRecipe compiles a candidate's recipe spec and returns its one slide.
func compileRecipe(t *testing.T, c patterns.VisualCandidate) (pattern string, raw string) {
	t.Helper()
	specJSON, err := json.Marshal(recipeArgs(t, c)["spec"])
	if err != nil {
		t.Fatal(err)
	}
	spec, diags := semantic.ParseJSON(specJSON)
	if spec == nil {
		t.Fatalf("%s: recipe spec does not parse: %+v", c.Name, diags)
	}
	deck, _, err := semantic.Compile(spec, semantic.CompileOptions{})
	if err != nil || deck == nil || len(deck.Slides) != 1 {
		t.Fatalf("%s: recipe spec does not compile to one slide: %v", c.Name, err)
	}
	out, _ := json.Marshal(deck.Slides[0])
	if p := deck.Slides[0].Pattern; p != nil {
		pattern = p.Name
	}
	return pattern, string(out)
}

func assertRecipeRenders(t *testing.T, mc *mcpConfig, c patterns.VisualCandidate) {
	t.Helper()
	verdict := renderRecipe(t, mc, c)
	if !verdict.Success {
		t.Fatalf("%s %q: recipe did not render: error=%q diags=%+v", c.Category, c.Name, verdict.Error, verdict.Diagnostics)
	}
	for _, d := range verdict.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("%s %q: recipe raised error diagnostic %+v", c.Category, c.Name, d)
		}
	}
}

// TestRecommendVisualEveryPatternIsAuthorable is the acceptance test for
// go-slide-creator-x97m6: every registered pattern, ranked as a candidate,
// carries a data contract and a runnable next_tool_call whose example values
// validate. A pattern a DeckSpec kind compiles to is authored as that kind
// (and its recipe really compiles to the pattern); each of the raw-only
// patterns says so and carries its contract — keys, item counts, character
// budgets — inline.
func TestRecommendVisualEveryPatternIsAuthorable(t *testing.T) {
	mc := semanticTestConfig(t)
	reg := patterns.Default()
	rawOnly := 0
	for _, pat := range reg.List() {
		name := pat.Name()
		t.Run(name, func(t *testing.T) {
			rec := patterns.RecommendVisualResult{Candidates: []patterns.VisualCandidate{{Category: patterns.VisualCategoryPattern, Name: name}}}
			attachVisualRecipes(&rec, "midnight-blue")
			c := rec.Candidates[0]
			if c.DataContract == nil || c.NextToolCall == nil {
				t.Fatalf("%s: data_contract=%v next_tool_call=%v", name, c.DataContract, c.NextToolCall)
			}
			if c.NextToolCall.Tool != "render_deck_spec" || !toolInProfile(t, toolProfileDeckSpec, c.NextToolCall.Tool) {
				t.Errorf("%s: next_tool_call.tool = %q, want a default-profile render_deck_spec", name, c.NextToolCall.Tool)
			}
			kind, known := semantic.PatternReach(name)
			if !known {
				t.Fatalf("%s is missing from the reachability table", name)
			}
			compiled, _ := compileRecipe(t, c)
			if compiled != name {
				t.Errorf("%s: recipe compiles to pattern %q", name, compiled)
			}
			if kind != "" {
				// The kind is the one whose recipe compiles to the pattern
				// (checked above); for a pattern several kinds reach it is the
				// one that states the content (process for numbered steps,
				// pillars for a panel row).
				if c.DeckSpec == nil || !semantic.SlideKind(c.DeckSpec.Kind).Valid() {
					t.Fatalf("%s: deckspec = %+v, want a kind (the reach table names %s)", name, c.DeckSpec, kind)
				}
				kind = semantic.SlideKind(c.DeckSpec.Kind)
				if c.DataContract.Form != patterns.ContractFormKind || c.DataContract.FieldPath != "slides[0]" {
					t.Errorf("%s: contract form %q at %q, want the kind slide", name, c.DataContract.Form, c.DataContract.FieldPath)
				}
				if len(c.DeckSpec.Fields.Required) == 0 || c.DeckSpec.ExamplePath == "" {
					t.Errorf("%s: deckspec form lacks fields / example_path: %+v", name, c.DeckSpec)
				}
				slides := recipeArgs(t, c)["spec"].(map[string]any)["slides"].([]any)
				if got := slides[0].(map[string]any)["kind"]; got != string(kind) {
					t.Errorf("%s: example slide kind = %v, want %s", name, got, kind)
				}
			} else {
				rawOnly++
				if c.DeckSpec != nil {
					t.Errorf("%s: raw-only pattern carries a deckspec form %+v", name, c.DeckSpec)
				}
				dc := c.DataContract
				if dc.Form != patterns.ContractFormRaw || !strings.HasPrefix(dc.Description, "Raw-only") {
					t.Errorf("%s: contract does not say raw-only: form %q, %q", name, dc.Form, dc.Description)
				}
				if dc.FieldPath != "slides[0].slide.pattern.values" || len(dc.RequiredKeys)+len(dc.OptionalKeys) == 0 {
					t.Errorf("%s: contract lacks keys or field_path: %+v", name, dc)
				}
				if len(dc.Limits) == 0 {
					t.Errorf("%s: contract carries no item counts or character budgets", name)
				}
			}
			assertRecipeValidates(t, mc, c)
			if !testing.Short() {
				assertRecipeRenders(t, mc, c)
			}
		})
	}
	if want := len(semantic.UnreachablePatterns()); rawOnly != want {
		t.Errorf("%d raw-only patterns carried an inline contract, want all %d", rawOnly, want)
	}
}

// TestRecommendVisualChartsAndKindDiagramsUseDeckSpecForm covers
// go-slide-creator-3ujfq: a chart is authored as chart_insight with the chart
// type it was asked for, and a diagram a kind draws (org, framework) as that
// kind.
func TestRecommendVisualChartsAndKindDiagramsUseDeckSpecForm(t *testing.T) {
	mc := semanticTestConfig(t)
	for _, cc := range svggen.ChartCapabilities() {
		if cc.Status != "ready" {
			continue
		}
		rec := patterns.RecommendVisualResult{Candidates: []patterns.VisualCandidate{{Category: patterns.VisualCategoryChart, Name: cc.Type}}}
		attachVisualRecipes(&rec, "midnight-blue")
		c := rec.Candidates[0]
		if chartInsightCannotDraw[cc.Type] {
			// chart_insight reads {categories, series}; these types take
			// another data shape and stay a raw chart slide.
			if c.DeckSpec != nil || c.DataContract == nil || c.DataContract.Form != patterns.ContractFormRaw {
				t.Errorf("chart %s: deckspec %+v contract %+v, want the raw form", cc.Type, c.DeckSpec, c.DataContract)
			}
			assertRecipeValidates(t, mc, c)
			continue
		}
		if c.DeckSpec == nil || c.DeckSpec.Kind != "chart_insight" {
			t.Fatalf("chart %s: deckspec = %+v, want chart_insight", cc.Type, c.DeckSpec)
		}
		if res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(recipeArgs(t, c))); err != nil || strings.Contains(resultText(res), "DEGRADED") {
			t.Errorf("chart %s: the chart_insight recipe degrades instead of drawing the chart: %v %s", cc.Type, err, resultText(res))
		}
		slide := recipeArgs(t, c)["spec"].(map[string]any)["slides"].([]any)[0].(map[string]any)
		if got := slide["chart"].(map[string]any)["type"]; got != cc.Type {
			t.Errorf("chart %s: recipe chart.type = %v", cc.Type, got)
		}
		if c.DataContract.FieldPath != "slides[0].chart.data" || len(c.DataContract.Description) == 0 {
			t.Errorf("chart %s: contract = %+v", cc.Type, c.DataContract)
		}
		assertRecipeValidates(t, mc, c)
	}
	for diagram, kind := range map[string]string{"org_chart": "org", "swot": "framework", "porters_five_forces": "framework"} {
		rec := patterns.RecommendVisualResult{Candidates: []patterns.VisualCandidate{{Category: patterns.VisualCategoryDiagram, Name: diagram}}}
		attachVisualRecipes(&rec, "midnight-blue")
		c := rec.Candidates[0]
		if c.DeckSpec == nil || c.DeckSpec.Kind != kind || c.DeckSpec.DiffersFromRaw == "" {
			t.Errorf("diagram %s: deckspec = %+v, want kind %s with differs_from_raw", diagram, c.DeckSpec, kind)
		}
		assertRecipeValidates(t, mc, c)
	}
}

// TestRecommendVisualLayoutCandidatesAreAuthorable: placeholder-layout and
// raw_shape_grid candidates carry a contract and a recipe that validates and
// renders (go-slide-creator-x97m6).
func TestRecommendVisualLayoutCandidatesAreAuthorable(t *testing.T) {
	mc := semanticTestConfig(t)
	wantKind := map[string]string{"title": "title", "section": "section", "table": "table", "image": "image_case"}
	cands := []patterns.VisualCandidate{{Category: patterns.VisualCategoryShapeGrid, Name: "raw_shape_grid"}}
	for _, name := range []string{"title", "section", "content", "two-column", "image", "blank", "table"} {
		cands = append(cands, patterns.VisualCandidate{Category: patterns.VisualCategoryPlaceholder, Name: name})
	}
	rec := patterns.RecommendVisualResult{Candidates: cands}
	attachVisualRecipes(&rec, "midnight-blue")
	for _, c := range rec.Candidates {
		if c.DataContract == nil || c.NextToolCall == nil || c.DataContract.Form == "" {
			t.Errorf("%s %q: data_contract=%+v next_tool_call=%v", c.Category, c.Name, c.DataContract, c.NextToolCall)
			continue
		}
		if kind := wantKind[c.Name]; kind != "" && (c.DeckSpec == nil || c.DeckSpec.Kind != kind) {
			t.Errorf("layout %q: deckspec = %+v, want kind %s", c.Name, c.DeckSpec, kind)
		}
		assertRecipeValidates(t, mc, c)
		if !testing.Short() {
			assertRecipeRenders(t, mc, c)
		}
	}
}

// TestRecommendVisualSameSlideIntentReturnsRegions covers
// go-slide-creator-3ujfq: the chart + KPI + timeline brief returns a regions
// candidate first, with the line chart it asked for, and it renders.
func TestRecommendVisualSameSlideIntentReturnsRegions(t *testing.T) {
	mc := semanticTestConfig(t)
	rec := recommendVisualFor(t, mc, map[string]any{
		"intent":   "a revenue line chart with a headline KPI and a 3-milestone timeline on the same slide",
		"template": "midnight-blue",
	})
	top := rec.Candidates[0]
	if top.Category != patterns.VisualCategoryKind || top.Name != "regions" || top.DeckSpec == nil || top.DeckSpec.Kind != "regions" {
		t.Fatalf("top = %s %q deckspec %+v, want the regions kind", top.Category, top.Name, top.DeckSpec)
	}
	slide := recipeArgs(t, top)["spec"].(map[string]any)["slides"].([]any)[0].(map[string]any)
	if slide["kind"] != "regions" || slide["arrangement"] != "main_left" {
		t.Fatalf("recipe slide = kind %v arrangement %v", slide["kind"], slide["arrangement"])
	}
	regions := slide["regions"].([]any)
	var kinds []string
	for _, r := range regions {
		kinds = append(kinds, r.(map[string]any)["kind"].(string))
	}
	if strings.Join(kinds, ",") != "chart,stat,timeline" {
		t.Errorf("region kinds = %v, want chart, stat, timeline", kinds)
	}
	chart := regions[0].(map[string]any)["chart"].(map[string]any)
	if chart["type"] != "line" {
		t.Errorf("chart region type = %v, want the line chart the intent asked for", chart["type"])
	}
	if n := len(regions[2].(map[string]any)["milestones"].([]any)); n != 3 {
		t.Errorf("timeline region has %d milestones, want 3", n)
	}
	// The raw composition stays as the second form, with the same line chart.
	second := rec.Candidates[1]
	if second.Category != patterns.VisualCategoryCompose || !strings.Contains(second.Name, "chart:line") {
		t.Errorf("second = %s %q, want the compose form with chart:line", second.Category, second.Name)
	}
	if top.DiffersBy == "" || second.DiffersBy == "" {
		t.Errorf("regions / compose near tie carries no differs_by: %q / %q", top.DiffersBy, second.DiffersBy)
	}
	assertRecipeValidates(t, mc, top)
	if !testing.Short() {
		assertRecipeRenders(t, mc, top)
		assertDeckHasVisual(t, renderRecipe(t, mc, top).PptxPath)
	}
}

// TestRecommendVisualHonoursRequestedChartType: the chart type named in the
// intent is the one returned (line stays line).
func TestRecommendVisualHonoursRequestedChartType(t *testing.T) {
	mc := semanticTestConfig(t)
	for intent, want := range map[string]string{
		"a line chart of weekly active users":      "line",
		"a bar chart of headcount by department":   "bar",
		"revenue as a stacked bar chart by region": "stacked_bar",
		"a pie chart of the budget split":          "pie",
	} {
		rec := recommendVisualFor(t, mc, map[string]any{"intent": intent})
		if top := rec.Candidates[0]; top.Category != patterns.VisualCategoryChart || top.Name != want {
			t.Errorf("%q: top = %s %q, want chart %s", intent, top.Category, top.Name, want)
		}
	}
}

// TestRecommendVisualCrossReferencesForms covers go-slide-creator-7sqof: the
// kind, pattern and diagram forms of one visual point at each other and say
// what differs; aliased kind fields name their canonical field.
func TestRecommendVisualCrossReferencesForms(t *testing.T) {
	mc := semanticTestConfig(t)
	rec := recommendVisualFor(t, mc, map[string]any{"intent": "a 2x2 prioritisation matrix"})
	var pat, dia *patterns.VisualCandidate
	for i := range rec.Candidates {
		switch c := &rec.Candidates[i]; {
		case c.Category == patterns.VisualCategoryPattern && c.Name == "matrix-2x2":
			pat = c
		case c.Category == patterns.VisualCategoryDiagram && c.Name == "matrix_2x2":
			dia = c
		}
	}
	if pat == nil || dia == nil {
		t.Fatalf("both 2x2 forms should be ranked: %+v", rec.Candidates)
	}
	hasRef := func(refs []patterns.VisualFormRef, form, name string) bool {
		for _, r := range refs {
			if r.Form == form && r.Name == name && r.Differs != "" {
				return true
			}
		}
		return false
	}
	if pat.DeckSpec == nil || pat.DeckSpec.Kind != "matrix_2x2" || !strings.Contains(pat.DeckSpec.DiffersFromRaw, "clockwise") {
		t.Errorf("pattern matrix-2x2: deckspec = %+v, want kind matrix_2x2 with the quadrant-order difference", pat.DeckSpec)
	}
	if !hasRef(pat.AlsoAs, "diagram", "matrix_2x2") {
		t.Errorf("pattern matrix-2x2 does not cross-reference the diagram: %+v", pat.AlsoAs)
	}
	if !hasRef(dia.AlsoAs, "pattern", "matrix-2x2") || !hasRef(dia.AlsoAs, "kind", "matrix_2x2") {
		t.Errorf("diagram matrix_2x2 does not cross-reference the pattern and the kind: %+v", dia.AlsoAs)
	}
	// Aliased fields mark the canonical name.
	if got := pat.DeckSpec.Aliases["x_axis_label"]; got != "x_axis" {
		t.Errorf("matrix_2x2 aliases = %v, want x_axis_label → x_axis", pat.DeckSpec.Aliases)
	}
	for kind, want := range map[semantic.SlideKind][2]string{
		semantic.KindKPISnapshot:  {"metrics", "kpis"},
		semantic.KindTeam:         {"people", "members"},
		semantic.KindOptionMatrix: {"rows", "options"},
	} {
		form := deckSpecFormFor(kind, "")
		if form == nil || form.Aliases[want[0]] != want[1] {
			t.Errorf("%s: aliases = %v, want %s → %s", kind, form, want[0], want[1])
		}
		for alias, canonical := range form.Aliases {
			if _, isAlias := form.Aliases[canonical]; isAlias {
				t.Errorf("%s: alias %q points at %q, itself an alias", kind, alias, canonical)
			}
		}
	}
}

// TestRecommendVisualChartTypeSpellings covers go-slide-creator-7sqof: a chart
// type is accepted as "bar" and as "bar_chart" by recommend_visual and by the
// renderer, and responses emit the canonical short name.
func TestRecommendVisualChartTypeSpellings(t *testing.T) {
	mc := semanticTestConfig(t)
	rec := recommendVisualFor(t, mc, map[string]any{
		"intent":     "compare revenue across 5 regions",
		"candidates": []any{"bar_chart", "line_chart", "stacked_bar_chart", "compose:chart:line_chart+stat-hero"},
	})
	got := map[string]patterns.VisualCategory{}
	for _, c := range rec.Candidates {
		got[c.Name] = c.Category
	}
	for _, name := range []string{"bar", "line", "stacked_bar"} {
		if got[name] != patterns.VisualCategoryChart {
			t.Errorf("candidate %q not resolved as a chart: %v", name, got)
		}
	}
	if got["compose:chart:line+stat-hero"] != patterns.VisualCategoryCompose {
		t.Errorf("compose name with a _chart spelling not canonicalised: %v", got)
	}
	for _, cc := range svggen.ChartCapabilities() {
		for _, spelling := range []string{cc.Type, cc.Type + "_chart"} {
			canon, ok := patterns.CanonicalChartType(spelling)
			if !ok || canon != cc.Type {
				t.Errorf("CanonicalChartType(%q) = %q, %v; want %q", spelling, canon, ok, cc.Type)
			}
			if svggen.DefaultRegistry().Get(spelling) == nil {
				t.Errorf("the renderer does not accept chart type %q", spelling)
			}
		}
	}
	if _, ok := patterns.CanonicalChartType("org_chart"); ok {
		t.Error("org_chart is a diagram, not a chart type")
	}
	// The kind examples list_slide_kinds publishes write the canonical name.
	for _, kind := range []semantic.SlideKind{semantic.KindChartInsight, semantic.KindRegions} {
		raw, _ := json.Marshal(semantic.KindExample(kind))
		if strings.Contains(string(raw), `_chart"`) {
			t.Errorf("%s example writes a _chart spelling: %s", kind, raw)
		}
	}
}

// recommendVisualEvalCases reads the tracked evaluation set the ranking test
// in internal/patterns pins.
func recommendVisualEvalCases(t *testing.T) []struct {
	Intent string         `json:"intent"`
	Hints  map[string]any `json:"hints,omitempty"`
	Expect []string       `json:"expect"`
	Must   bool           `json:"must,omitempty"`
} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "patterns", "testdata", "recommend_visual_eval.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Intent string         `json:"intent"`
			Hints  map[string]any `json:"hints,omitempty"`
			Expect []string       `json:"expect"`
			Must   bool           `json:"must,omitempty"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Cases
}

// recommendVisualEvalTemplates are the templates the evaluation renders on:
// two tracked ones, plus the local p-style template when it is present.
func recommendVisualEvalTemplates() []string {
	out := []string{"midnight-blue", "forest-green"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		out = append(out, "p-style")
	}
	return out
}

// TestRecommendVisualEvalTopCandidateRenders renders the top candidate's
// next_tool_call for the evaluation intents on two templates (and p-style when
// present): what recommend_visual ranks first can be rendered from its answer
// alone (go-slide-creator-x97m6). Short mode renders the "must" intents; the
// full run renders every intent.
func TestRecommendVisualEvalTopCandidateRenders(t *testing.T) {
	mc := semanticTestConfig(t)
	for _, tmpl := range recommendVisualEvalTemplates() {
		for _, tc := range recommendVisualEvalCases(t) {
			if testing.Short() && !tc.Must {
				continue
			}
			args := map[string]any{"intent": tc.Intent, "template": tmpl}
			if tc.Hints != nil {
				args["content_hints"] = tc.Hints
			}
			rec := recommendVisualFor(t, mc, args)
			if len(rec.Candidates) == 0 {
				t.Errorf("%s / %q: no candidates", tmpl, tc.Intent)
				continue
			}
			top := rec.Candidates[0]
			// With template context a candidate the template hosts badly is
			// demoted, so the top can be any accepted name's neighbour; the
			// ranking itself is pinned template-free in internal/patterns.
			if top.NextToolCall == nil || top.DataContract == nil {
				t.Errorf("%s / %q: top %s %q has no recipe (next_tool_call=%v, data_contract=%v)", tmpl, tc.Intent, top.Category, top.Name, top.NextToolCall, top.DataContract)
				continue
			}
			spec := recipeArgs(t, top)["spec"].(map[string]any)
			if got := spec["meta"].(map[string]any)["template"]; got != tmpl {
				t.Errorf("%s / %q: recipe names template %v", tmpl, tc.Intent, got)
			}
			verdict := renderRecipe(t, mc, top)
			if !verdict.Success {
				t.Errorf("%s / %q: top %s %q did not render: error=%q diags=%+v", tmpl, tc.Intent, top.Category, top.Name, verdict.Error, verdict.Diagnostics)
				continue
			}
			for _, d := range verdict.Diagnostics {
				if d.Severity == "error" {
					t.Errorf("%s / %q: top %s %q raised error diagnostic %s: %s", tmpl, tc.Intent, top.Category, top.Name, d.Code, d.Message)
				}
			}
		}
	}
}

// recommendVisualMaxResponseBytes caps one recommend_visual response for the
// evaluation intents, as compact JSON. The agent-journey review measured
// 1.5–10.7 KB per call before pattern and layout candidates carried a
// contract and a recipe; five fully authorable candidates fit in 16 KB.
const recommendVisualMaxResponseBytes = 16 * 1024

// TestRecommendVisualResponseSizeStaysInCheck: every candidate is authorable
// from the response, and the response is still one an agent can afford.
func TestRecommendVisualResponseSizeStaysInCheck(t *testing.T) {
	mc := semanticTestConfig(t)
	total, n, largest := 0, 0, 0
	for _, tc := range recommendVisualEvalCases(t) {
		args := map[string]any{"intent": tc.Intent, "template": "midnight-blue"}
		if tc.Hints != nil {
			args["content_hints"] = tc.Hints
		}
		res, err := mc.handleRecommendVisual(context.Background(), makeRequest(args))
		if err != nil || res.IsError {
			t.Fatalf("%q: %v %s", tc.Intent, err, resultText(res))
		}
		raw, _ := json.Marshal(res.StructuredContent)
		if len(raw) > recommendVisualMaxResponseBytes {
			t.Errorf("%q: response is %d bytes, over the %d budget", tc.Intent, len(raw), recommendVisualMaxResponseBytes)
		}
		total += len(raw)
		n++
		if len(raw) > largest {
			largest = len(raw)
		}
	}
	t.Logf("recommend_visual response size over %d intents: mean %d B, largest %d B", n, total/n, largest)
}
