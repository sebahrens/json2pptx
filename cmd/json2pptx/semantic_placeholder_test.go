package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/svggen"
)

// Placeholder copy the product itself emits never passes the deterministic
// gate (go-slide-creator-327g6).

// placeholderOf returns the registered marker name a finding reports, or "".
func placeholderOf(f diagnostics.Finding) string {
	name, _ := f.Evidence[semantic.PlaceholderDetail].(string)
	return name
}

// assertOnlyPlaceholdersBlock checks a validate_deck_spec result of a spec the
// product handed out as scaffolding: it is refused, every blocking finding is
// a registered placeholder, and each placeholder is a blocking error at an
// authored path.
func assertOnlyPlaceholdersBlock(t *testing.T, label string, res *mcp.CallToolResult) {
	t.Helper()
	var env deckSpecEnvelopeResponse
	structuredInto(t, res.StructuredContent, &env)
	if res.IsError || env.OK {
		t.Fatalf("%s: a spec that still carries the product's placeholder copy must validate to ok:false, not fail or pass: isError=%v ok=%v\n%s", label, res.IsError, env.OK, resultText(res))
	}
	placeholders := 0
	for _, f := range env.Findings {
		name := placeholderOf(f)
		if f.Severity == diagnostics.SeverityError && name == "" {
			t.Errorf("%s: blocked by something other than its placeholders: %s at %v: %s", label, f.Code, f.Path, f.Message)
		}
		if name == "" {
			continue
		}
		placeholders++
		if f.Severity != diagnostics.SeverityError || f.Blocking == nil || !*f.Blocking || !strings.HasSuffix(f.Code, diagnostics.CodeSemanticWeakContent) {
			t.Errorf("%s: placeholder %s is not a blocking %s: %+v", label, name, diagnostics.CodeSemanticWeakContent, f)
		}
		if f.Path == nil || !strings.HasPrefix(*f.Path, "/") {
			t.Errorf("%s: placeholder %s has no authored path", label, name)
		}
	}
	if placeholders == 0 {
		t.Errorf("%s: no placeholder finding: %s", label, resultText(res))
	}
}

// recipeArgs round-trips a next_tool_call's arguments as an agent receives and
// resends them.
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

// fillPlaceholders replaces every registered placeholder in a decoded JSON
// value with real copy, returning how many it replaced.
func fillPlaceholders(v any) (any, int) {
	switch t := v.(type) {
	case string:
		if _, ok := placeholder.Detect(t); ok {
			return "Unit sales grew 12% in the north region", 1
		}
		return t, 0
	case map[string]any:
		n := 0
		for k, child := range t {
			next, c := fillPlaceholders(child)
			t[k] = next
			n += c
		}
		return t, n
	case []any:
		n := 0
		for i, child := range t {
			next, c := fillPlaceholders(child)
			t[i] = next
			n += c
		}
		return t, n
	}
	return v, 0
}

// assertRecipeIsBlockedAsExemplar renders a recipe verbatim and expects the
// exemplar-content blocker; with the placeholders written over, the recipe has
// no placeholder finding left.
func assertRecipeIsBlockedAsExemplar(t *testing.T, mc *mcpConfig, c patterns.VisualCandidate) {
	t.Helper()
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(recipeArgs(t, c)))
	if err != nil {
		t.Fatal(err)
	}
	var verdict renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &verdict)
	if !verdict.Success {
		t.Fatalf("the recipe did not render: %q %+v", verdict.Error, verdict.Diagnostics)
	}
	if verdict.DeterministicReady == nil || *verdict.DeterministicReady {
		t.Errorf("a recipe rendered verbatim is deterministic_ready: its title is %q", placeholder.RecipeActionTitle("…"))
	}
	if len(verdict.DeterministicBlockingReasons) == 0 || verdict.DeterministicBlockingReasons[0] != exemplarContentReason {
		t.Errorf("blocking reasons do not lead with %q: %v", exemplarContentReason, verdict.DeterministicBlockingReasons)
	}
	title := false
	for _, d := range verdict.Diagnostics {
		if isPlaceholderFinding(d) && d.Evidence[semantic.PlaceholderDetail] == placeholder.NameRecipeTitle {
			title = d.Code == diagnostics.CodeSemanticWeakContent && d.Blocking
		}
	}
	if !title {
		t.Errorf("no blocking %s for the recipe's title: %+v", diagnostics.CodeSemanticWeakContent, verdict.Diagnostics)
	}

	filled, n := fillPlaceholders(recipeArgs(t, c))
	if n == 0 {
		t.Fatal("the recipe carries no registered placeholder")
	}
	vres, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(filled.(map[string]any)))
	if err != nil {
		t.Fatal(err)
	}
	var env deckSpecEnvelopeResponse
	structuredInto(t, vres.StructuredContent, &env)
	for _, f := range env.Findings {
		if placeholderOf(f) != "" || strings.HasSuffix(f.Code, diagnostics.CodeSemanticWeakContent) {
			t.Errorf("a filled recipe still reports placeholder content at %v: %s", f.Path, f.Message)
		}
	}
}

// TestRecipesVerbatimAreBlockedAsExemplarContent is the go-slide-creator-327g6
// acceptance test: every recommend_visual recipe — each chart type, each
// diagram type, and the compose recipes — rendered exactly as handed out, hits
// the exemplar-content blocker. Short mode renders one recipe of each family.
func TestRecipesVerbatimAreBlockedAsExemplarContent(t *testing.T) {
	mc := semanticTestConfig(t)
	type entry struct {
		cat  patterns.VisualCategory
		name string
	}
	var types []entry
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			types = append(types, entry{patterns.VisualCategoryChart, c.Type})
		}
	}
	for _, d := range svggen.DiagramCapabilitiesReady() {
		types = append(types, entry{patterns.VisualCategoryDiagram, d.Type})
	}
	if testing.Short() {
		types = []entry{{patterns.VisualCategoryChart, "bar"}, {patterns.VisualCategoryDiagram, "swot"}}
	}
	for _, e := range types {
		t.Run(string(e.cat)+"/"+e.name, func(t *testing.T) {
			rec := patterns.RecommendVisualResult{Candidates: []patterns.VisualCandidate{{Category: e.cat, Name: e.name}}}
			attachVisualRecipes(&rec, "midnight-blue")
			assertRecipeIsBlockedAsExemplar(t, mc, rec.Candidates[0])
		})
	}
	cases := composeRecipeCases
	if testing.Short() {
		cases = cases[:1]
	}
	for _, tc := range cases {
		t.Run("compose/"+tc.id, func(t *testing.T) {
			args := map[string]any{"template": "midnight-blue"}
			for k, v := range tc.args {
				args[k] = v
			}
			assertRecipeIsBlockedAsExemplar(t, mc, recommendVisualFor(t, mc, args).Candidates[0])
		})
	}
}

// A plan_deck draft is scaffolding too: validated unmodified it is refused, and
// every __FILL__ it carries — meta.date included — is a blocking finding.
func TestPlanDraftIsBlockedOnEveryFillToken(t *testing.T) {
	mc := refusalTestConfig(t)
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{
		"brief":        "Board update on SMB churn. Churn rose to 4.2% in Q2, NRR fell to 108%, and the EU expansion slipped a quarter. We propose a dedicated success pod.",
		"slide_budget": float64(9), "format": "deckspec",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("plan_deck failed: %v %s", err, textContent(res))
	}
	var plan deckplan.DeckSpecPlan
	if err := json.Unmarshal([]byte(textContent(res)), &plan); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(plan.DeckSpec)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	collectFillPaths(doc, "", want)
	if !want["/meta/date"] {
		t.Fatalf("the draft no longer carries __FILL__ at /meta/date; the test needs another meta field: %s", raw)
	}

	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": doc, "template": "midnight-blue"}))
	if env.OK {
		t.Fatal("an unmodified plan draft validates ok")
	}
	for _, f := range env.Findings {
		if placeholderOf(f) != placeholder.NameFillToken {
			continue
		}
		if f.Severity != diagnostics.SeverityError {
			t.Errorf("__FILL__ at %v is %s, not a blocking error", f.Path, f.Severity)
		}
		for _, p := range pathsOf(f) {
			delete(want, p)
		}
	}
	for p := range want {
		t.Errorf("__FILL__ at %s is not reported", p)
	}
}

// collectFillPaths records the pointer of every string that carries __FILL__.
func collectFillPaths(v any, pointer string, out map[string]bool) {
	switch t := v.(type) {
	case string:
		if placeholder.Contains(t) {
			out[pointer] = true
		}
	case map[string]any:
		for k, child := range t {
			collectFillPaths(child, pointer+"/"+k, out)
		}
	case []any:
		for i, child := range t {
			collectFillPaths(child, pointer+"/"+strconv.Itoa(i), out)
		}
	}
}

// Every registered marker is detected in any text field, and a suggested
// patch value applied verbatim is one of them.
func TestProductPlaceholdersAreDetectedInAnyField(t *testing.T) {
	mc := refusalTestConfig(t)
	samples := map[string]string{
		placeholder.NameFillToken:          "Q3 __FILL__ results",
		placeholder.NameRecipeTitle:        placeholder.RecipeActionTitle("bar chart"),
		placeholder.NameRecipeAltText:      placeholder.RecipeAltText("bar chart"),
		placeholder.NameRecipeSampleSource: placeholder.RecipeSampleSource,
		placeholder.NameArgumentHint:       rewriteHint("INPUT.SEMANTIC_DENSITY", map[string]any{"max_chars": 40}),
	}
	for _, m := range placeholder.Registered() {
		text, ok := samples[m.Name]
		if !ok {
			t.Errorf("registered marker %s has no sample here: add one, so its detection is tested", m.Name)
			continue
		}
		for _, field := range []string{"/meta/subtitle", "/meta/date", "/meta/source", "/slides/1/title", "/slides/1/kpis/0/label", "/slides/1/takeaway", "/slides/1/notes"} {
			spec := decodeSpecObject(t, `{"meta":{"title":"Churn review","subtitle":"Board, October","date":"October 2026","source":"Company data"},"slides":[
			 {"kind":"title","title":"Churn rose to 4.2% in the second quarter","subtitle":"Board, October"},
			 {"kind":"kpi_snapshot","title":"Churn rose while net retention fell to 108%","kpis":[{"value":"4.2%","label":"Churn"},{"value":"108%","label":"NRR"}],"takeaway":"Retention needs a dedicated pod.","notes":"Speaker notes."}]}`)
			agent := &journeyAgent{t: t, spec: spec}
			agent.set(field, text)
			env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "midnight-blue"}))
			found := false
			for _, f := range env.Findings {
				if placeholderOf(f) == m.Name && f.Path != nil && *f.Path == field && f.Severity == diagnostics.SeverityError {
					found = true
				}
			}
			if !found || env.OK {
				t.Errorf("%s in %s is not a blocking finding (ok=%v): %+v", m.Name, field, env.OK, env.Findings)
			}
		}
	}
}
