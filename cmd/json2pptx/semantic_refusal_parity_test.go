package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// archOnModernSpec is list_slide_kinds' architecture example pinned to the
// modern template, the review's reproduction: its first tier's items render at
// 11.04pt against a 12pt card-body floor (go-slide-creator-b7qqg.3/.4).
const archOnModernSpec = `{"meta":{"template":"modern","title":"Architecture validation mismatch"},"slides":[{"kind":"architecture","rails":["Security & compliance","Cost governance"],"takeaway":"Every tier ships independently; the rails are owned centrally.","tiers":[{"items":["Web console","Mobile approvals","Partner portal"],"label":"Experience"},{"items":["Orders","Pricing","Fulfilment","Identity"],"label":"Services"},{"description":"Event stream, warehouse, feature store","label":"Data"},{"description":"Kubernetes, observability, secrets","label":"Platform"}],"title":"Four tiers, two concerns that cut across them"}]}`

// wantArchPatch is the source-preserving fix both surfaces must hand back: the
// kind's native-bullets composition keeps every tier item.
var wantArchPatch = []any{map[string]any{"op": "add", "path": "/slides/0/layout", "value": "content"}}

func refusalTestConfig(t *testing.T) *mcpConfig {
	t.Helper()
	mc := semanticTestConfig(t)
	mc.deckHandles = newDeckHandleStore(deckHandleTTL)
	return mc
}

func decodeSpecObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var spec map[string]any
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

func patchOf(t *testing.T, call any) []any {
	t.Helper()
	b, _ := json.Marshal(call)
	var c struct {
		Tool         string `json:"tool"`
		ArgsTemplate struct {
			DeckID string `json:"deck_id"`
			Patch  []any  `json:"patch"`
		} `json:"args_template"`
	}
	if err := json.Unmarshal(b, &c); err != nil || c.Tool != "validate_deck_spec" || c.ArgsTemplate.DeckID == "" {
		t.Fatalf("next_tool_call is not a validate_deck_spec patch on a deck_id: %s", b)
	}
	return c.ArgsTemplate.Patch
}

// go-slide-creator-b7qqg.3: validate must refuse what render refuses, at the
// authored field.
func TestValidateDeckSpecRefusesUnreadableGridTextLikeRender(t *testing.T) {
	ctx := context.Background()
	mc := refusalTestConfig(t)
	res, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": decodeSpecObject(t, archOnModernSpec)}))
	if err != nil {
		t.Fatal(err)
	}
	var env deckSpecEnvelopeResponse
	structuredInto(t, res.StructuredContent, &env)
	if env.OK {
		t.Fatalf("validate approved a deck render refuses: %+v", env.Findings)
	}
	var found *diagnostics.Finding
	for i := range env.Findings {
		if env.Findings[i].Code == "INPUT.TEXT_BELOW_READABLE_MIN" {
			found = &env.Findings[i]
		}
	}
	if found == nil || found.Severity != diagnostics.SeverityError {
		t.Fatalf("want an error-severity INPUT.TEXT_BELOW_READABLE_MIN, got %+v", env.Findings)
	}
	if got := found.Evidence["path"]; got != "slides[0].tiers[0].items" {
		t.Errorf("evidence.path = %v, want the authored field slides[0].tiers[0].items", got)
	}
	if found.Evidence["measured"] == nil || found.Evidence["allowed"] == nil {
		t.Errorf("finding lacks measured/allowed evidence: %+v", found.Evidence)
	}
	if _, leaked := found.Evidence[compositionPatchDetail]; leaked {
		t.Error("internal composition_patch detail leaked into evidence")
	}
	if patch := patchOf(t, found.NextToolCall); !reflect.DeepEqual(patch, wantArchPatch) {
		t.Errorf("patch = %v, want %v", patch, wantArchPatch)
	}
}

// go-slide-creator-b7qqg.4: the render refusal keeps its code, severity,
// authored path, measurement and an executable follow-up, and the follow-up
// renders.
func TestRenderDeckSpecRefusalIsSourceAddressedAndRepairable(t *testing.T) {
	ctx := context.Background()
	mc := refusalTestConfig(t)
	res, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"spec": decodeSpecObject(t, archOnModernSpec)}))
	if err != nil {
		t.Fatal(err)
	}
	var render renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &render)
	if render.OK || !res.IsError {
		t.Fatalf("render should refuse the unreadable tier: %+v", render)
	}
	var d *semanticDiagnostic
	for i := range render.Diagnostics {
		if render.Diagnostics[i].Code == "TEXT_BELOW_READABLE_MIN" {
			d = &render.Diagnostics[i]
		}
	}
	if d == nil {
		t.Fatalf("refusal left no diagnostic, only error %q", render.Error)
	}
	if d.Severity != "error" || d.Action != "refuse" {
		t.Errorf("severity/action = %s/%s, want error/refuse", d.Severity, d.Action)
	}
	if d.SemanticPath != "slides[0].tiers[0].items" {
		t.Errorf("semantic_path = %q, want slides[0].tiers[0].items", d.SemanticPath)
	}
	if !strings.Contains(d.RawPath, "/rendered_shapes/") {
		t.Errorf("raw_path = %q, want the generated locator kept as evidence", d.RawPath)
	}
	measured, _ := d.Evidence["measured"].(map[string]any)
	allowed, _ := d.Evidence["allowed"].(map[string]any)
	if measured["font_pt"] != 11.04 || allowed["min_font_pt"] != 12.0 {
		t.Errorf("evidence = %+v, want measured 11.04pt against allowed 12pt", d.Evidence)
	}
	if d.Evidence["text"] != "Web console, Mobile approvals, Partner portal" {
		t.Errorf("evidence.text = %v", d.Evidence["text"])
	}
	patch := patchOf(t, d.NextToolCall)
	if !reflect.DeepEqual(patch, wantArchPatch) {
		t.Fatalf("patch = %v, want %v", patch, wantArchPatch)
	}
	if !reflect.DeepEqual(render.NextToolCall, d.NextToolCall) {
		t.Errorf("top-level next_tool_call = %+v, want the refusal's patch", render.NextToolCall)
	}

	// The follow-up is executable: apply it, then render the stored deck.
	vres, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"deck_id": render.DeckID, "patch": patch}))
	if err != nil {
		t.Fatal(err)
	}
	var env deckSpecEnvelopeResponse
	structuredInto(t, vres.StructuredContent, &env)
	if !env.OK {
		t.Fatalf("patched deck still fails validation: %+v", env.Findings)
	}
	rres, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"deck_id": env.DeckID}))
	if err != nil {
		t.Fatal(err)
	}
	var fixed renderDeckSpecResponse
	structuredInto(t, rres.StructuredContent, &fixed)
	if !fixed.OK {
		t.Fatalf("patched deck did not render: %s %+v", fixed.Error, fixed.Diagnostics)
	}
}

// go-slide-creator-b7qqg.3 acceptance: validate and render agree on the
// readable floors for every kind's example on every shipped template.
func TestDeckSpecReadabilityVerdictParityAllKindsAllTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("renders every kind on every template")
	}
	ctx := context.Background()
	mc := refusalTestConfig(t)
	for _, tpl := range []string{"abstract", "blue-corporate", "business-template", "forest-green", "midnight-blue", "modern", "modern-template", "modern-yellow", "warm-coral"} {
		for _, k := range semantic.AllSlideKinds() {
			spec := map[string]any{
				"meta":   map[string]any{"template": tpl, "title": "Parity"},
				"slides": []any{semantic.KindExample(k)},
			}
			vres, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
			if err != nil {
				t.Fatal(err)
			}
			var env deckSpecEnvelopeResponse
			structuredInto(t, vres.StructuredContent, &env)
			validateRefuses := false
			for _, f := range env.Findings {
				if f.Code == "INPUT.TEXT_BELOW_READABLE_MIN" && f.Severity == diagnostics.SeverityError {
					validateRefuses = true
				}
			}
			rres, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
			if err != nil {
				t.Fatal(err)
			}
			var render renderDeckSpecResponse
			structuredInto(t, rres.StructuredContent, &render)
			renderRefuses := false
			for _, d := range render.Diagnostics {
				if d.Code == "TEXT_BELOW_READABLE_MIN" && d.Severity == "error" && !render.OK {
					renderRefuses = true
					if d.SemanticPath == "" || d.NextToolCall == nil {
						t.Errorf("%s/%s: render refusal not source-addressed: %+v", tpl, k, d)
					}
				}
			}
			if validateRefuses != renderRefuses {
				t.Errorf("%s/%s: validate refuses=%v but render refuses=%v (render error %q)", tpl, k, validateRefuses, renderRefuses, render.Error)
			}
		}
	}
}
