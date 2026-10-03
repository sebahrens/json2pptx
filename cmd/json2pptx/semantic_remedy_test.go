package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// findingAt returns the finding of a code at a path.
func findingAt(t *testing.T, env deckSpecEnvelopeResponse, code, path string) diagnostics.Finding {
	t.Helper()
	for _, f := range env.Findings {
		if strings.HasSuffix(f.Code, code) && f.Path != nil && *f.Path == path {
			return f
		}
	}
	t.Fatalf("no %s at %s: %+v", code, path, env.Findings)
	return diagnostics.Finding{}
}

// go-slide-creator-4mmvb: an option matrix was refused with the option's name
// quoted as the text to shorten; the cause was the detail line on the
// recommended row. The finding now names the removal that clears it, tried on
// the spec, and quotes no symptom's text as the cause.
func TestFitFindingNamesTheCutThatClearsIt(t *testing.T) {
	mc := refusalTestConfig(t)
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": decodeSpecObject(t, optionMatrixTightSpec)}))
	f := findingAt(t, env, "BODY_TOO_LONG", "/slides/1")
	want := []any{map[string]any{"op": "remove", "path": "/slides/1/options/0/detail"}}
	if got := patchOf(t, f.NextToolCall); !f.PatchVerified || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("patch %v verified=%v, want the verified removal of the recommended row's detail", got, f.PatchVerified)
	}
	if !strings.Contains(f.Message, "verified fix: removing /slides/1/options/0/detail (37 characters) clears this") {
		t.Errorf("the message does not name the cut: %s", f.Message)
	}
	if text, quoted := f.Evidence["text"]; quoted {
		t.Errorf("the finding quotes one symptom's text as if it were the cause: %v", text)
	}
	for _, raw := range []string{"show_legend", "highlight_label"} {
		if strings.Contains(f.Message, raw) {
			t.Errorf("the advice names %s, which does not shorten an option_matrix: %s", raw, f.Message)
		}
	}
	// The render refusal says the same.
	render := renderDeckSpecCall(t, mc, map[string]any{"spec": decodeSpecObject(t, optionMatrixTightSpec)})
	seen := false
	for _, d := range render.Diagnostics {
		if d.Code == "BODY_TOO_LONG" {
			seen = true
			if got := patchOf(t, d.NextToolCall); !d.patchVerified || fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("render: patch %v verified=%v", got, d.patchVerified)
			}
		}
	}
	if !seen {
		t.Errorf("render reports no BODY_TOO_LONG: %+v", render.Diagnostics)
	}

	// When the limit is the number of rows, the finding says so and gives the
	// count that fits: five one-line points on the shortest content area.
	summary := map[string]any{
		"meta": map[string]any{"title": "Counts", "source": "Illustrative"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Counts every kind documents", "subtitle": "October 2026"},
			kindAtCount(t, semantic.KindExecutiveSummary, "points", 5),
		},
	}
	counted := findingAt(t, deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": summary, "template": "modern"})), "BODY_TOO_LONG", "/slides/1")
	if !counted.PatchVerified || !strings.Contains(counted.Message, "the limit here is the number of points: 4 fit") {
		t.Errorf("the finding does not say how many points fit: verified=%v %s", counted.PatchVerified, counted.Message)
	}
}

// go-slide-creator-vg73u: five options with a detail each were refused on
// modern-template with the instruction to shorten "RECOMMENDED", a badge the
// layout writes. The finding says whose text it is and what controls it.
func TestRefusalNamesTextTheLayoutWrote(t *testing.T) {
	mc := refusalTestConfig(t)
	options := []any{}
	for i := 1; i <= 5; i++ {
		options = append(options, map[string]any{"label": fmt.Sprintf("Option %d", i), "detail": "Short detail.", "recommended": i == 2})
	}
	spec := map[string]any{"meta": map[string]any{"title": "Probe"}, "slides": []any{map[string]any{
		"kind": "decision", "title": "We recommend option two for the programme", "options": options, "recommendation": "Take option two.",
	}}}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "modern-template"}))
	if env.OK {
		t.Skip("this template build holds five options with details; nothing is refused")
	}
	named := false
	for _, f := range env.Findings {
		if strings.Contains(f.Message, "shorten the text") {
			t.Errorf("the finding asks to shorten text without saying whose: %s", f.Message)
		}
		if strings.Contains(f.Message, `"RECOMMENDED", a label the layout writes, not copy from the spec (it marks the option with recommended: true)`) {
			named = true
			if !f.PatchVerified {
				t.Errorf("no verified patch beside the layout-label finding: %+v", f.NextToolCall)
			}
		}
	}
	if !named {
		t.Errorf("the refusal does not say that RECOMMENDED is the layout's own label: %+v", env.Findings)
	}
}

// A collapsed finding's remediation carries every item's budget, in the order
// of paths; it used to carry the first item's alone (go-slide-creator-micna).
func TestCollapsedFindingCarriesEveryItemsBudget(t *testing.T) {
	mc := refusalTestConfig(t)
	draft := twelveFlawDraft(t)
	slides := draft["slides"].([]any)
	spec := map[string]any{
		"meta":   map[string]any{"title": "Cloud cost programme", "source": "Billing exports"},
		"slides": []any{slides[0], slides[8]},
	}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "midnight-blue"}))
	steps := findingAt(t, env, diagnostics.CodeSemanticPatternDegraded, "/slides/1/steps/0/description")
	if steps.Remediation == nil || steps.Remediation.Primary.Action != diagnostics.ActionShortenText {
		t.Fatalf("remediation = %+v, want shorten_text", steps.Remediation)
	}
	budgets, _ := steps.Remediation.Primary.Params["max_chars"].([]any)
	if len(budgets) != len(steps.Paths) || len(budgets) != 8 {
		t.Fatalf("max_chars = %v for %d paths, want one budget per step", steps.Remediation.Primary.Params["max_chars"], len(steps.Paths))
	}
	// The patch rewrites each step to its own budget.
	ops := patchOf(t, steps.NextToolCall)
	if len(ops) != 8 {
		t.Fatalf("patch = %v", ops)
	}
	for i, raw := range ops {
		op := raw.(map[string]any)
		if op["path"] != steps.Paths[i] || !strings.Contains(op["value"].(string), fmt.Sprintf("at most %v characters", budgets[i])) {
			t.Errorf("op %d = %v, want a rewrite of %s to %v characters", i, op, steps.Paths[i], budgets[i])
		}
	}
	// Beside the budget, only the pattern it belongs to (params.from).
	for key := range steps.Remediation.Primary.Params {
		if key != "max_chars" && key != "from" {
			t.Errorf("the remediation carries more than the budget and its pattern: %v", steps.Remediation.Primary.Params)
		}
	}
	for _, f := range env.Findings {
		if f.Remediation == nil || f.Remediation.Primary == nil {
			continue
		}
		for key := range f.Remediation.Primary.Params {
			if !remedyParamNames[key] {
				t.Errorf("%s: remediation param %q is not a budget or a choice", f.Code, key)
			}
		}
	}
}

// compile_deck_spec answered ok:true beside a blocking diagnostic for
// placeholder copy the product itself emitted (go-slide-creator-327g6): a
// spec that compiles and is not a deck.
func TestCompileDeckSpecIsNotOKBesideABlockingDiagnostic(t *testing.T) {
	spec := map[string]any{"meta": map[string]any{"title": "Draft", "template": "midnight-blue"}, "slides": []any{
		map[string]any{"kind": "title", "title": "Q3 __FILL__ results", "subtitle": "Board, October 2026"},
	}}
	res := mustCall(t, handleCompileDeckSpec, map[string]any{"spec": spec, "include_compiled_json": true})
	var out compileDeckSpecResponse
	structuredInto(t, res.StructuredContent, &out)
	if out.OK || !strings.Contains(out.Error, "exemplar_content") || len(out.CompiledJSON) != 0 {
		t.Errorf("ok=%v error=%q compiled_json=%d bytes, want a refusal that names the placeholder copy", out.OK, out.Error, len(out.CompiledJSON))
	}
	blocking := false
	for _, d := range out.Diagnostics {
		blocking = blocking || d.Blocking
	}
	if !blocking {
		t.Errorf("no blocking diagnostic: %+v", out.Diagnostics)
	}

	clean := mustCall(t, handleCompileDeckSpec, map[string]any{"spec": validSemanticSpec})
	var ok compileDeckSpecResponse
	structuredInto(t, clean.StructuredContent, &ok)
	if !ok.OK || ok.Error != "" {
		t.Errorf("a clean spec no longer compiles ok: %+v", ok)
	}
}
