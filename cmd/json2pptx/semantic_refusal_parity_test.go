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
// modern template, the review's reproduction: its first tier's items render
// under the 12pt card-body floor (go-slide-creator-b7qqg.3/.4). It carries a
// source line since go-slide-creator-me53q: a one-line takeaway reserves 17pt
// less than it did, and without the source line the four tiers fit modern.
// Since go-slide-creator-6h1fy a tier's items are drawn as component blocks,
// which the example fits; the reproduction keeps the joined detail line by
// giving the first tier an item past the 40-character component budget and
// the second a description, so no tier has components.
const archOnModernSpec = `{"meta":{"template":"modern","title":"Architecture validation mismatch"},"slides":[{"kind":"architecture","rails":["Security & compliance","Cost governance"],"source":"Platform architecture review","takeaway":"Every tier ships independently; the rails are owned centrally.","tiers":[{"items":["Web console","Mobile approvals","Partner portal for resellers and distributors"],"label":"Experience"},{"description":"Orders, pricing, fulfilment, identity","label":"Services"},{"description":"Event stream, warehouse, feature store","label":"Data"},{"description":"Kubernetes, observability, secrets","label":"Platform"}],"title":"Four tiers, two concerns that cut across them"}]}`

// wantArchPatch is the fix both surfaces must hand back, tried on the spec
// before it is offered: the smallest cut that clears the refused tier — its
// longest item (go-slide-creator-4mmvb). The source-preserving alternative,
// the kind's native-bullets composition, is named beside it.
var wantArchPatch = []any{map[string]any{"op": "remove", "path": "/slides/0/tiers/0/items/2"}}

const wantArchAlternative = `set /slides/0/layout to "content"`

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
	if found.Path == nil || *found.Path != "/slides/0/tiers/0/items" {
		t.Errorf("path = %v, want the authored field /slides/0/tiers/0/items", found.Path)
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
	if !found.PatchVerified || !strings.Contains(found.Message, "verified fix") || !strings.Contains(found.Message, wantArchAlternative) {
		t.Errorf("the patch is not marked verified, or the finding does not name the layout that keeps every word: verified=%v %s", found.PatchVerified, found.Message)
	}
	if found.Remediation == nil || found.Remediation.Primary.Action != diagnostics.ActionApplyPatch {
		t.Errorf("remediation = %+v, want apply_patch", found.Remediation)
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
	if !strings.Contains(d.RawPath, "/rendered_shapes/") && !strings.Contains(d.RawPath, "/pattern/rows/") {
		t.Errorf("raw_path = %q, want the generated locator kept as evidence", d.RawPath)
	}
	measured, _ := d.Evidence["measured"].(map[string]any)
	allowed, _ := d.Evidence["allowed"].(map[string]any)
	// 8.88pt since the slide takeaway is a band 12pt taller than the bar was
	// (go-slide-creator-fmmec); 10.08pt before.
	if measured["font_pt"] != 8.88 || allowed["min_font_pt"] != 12.0 {
		t.Errorf("evidence = %+v, want measured 8.88pt against allowed 12pt", d.Evidence)
	}
	if d.Evidence["text"] != "Web console, Mobile approvals, Partner portal for resellers and distributors" {
		t.Errorf("evidence.text = %v", d.Evidence["text"])
	}
	patch := patchOf(t, d.NextToolCall)
	if !reflect.DeepEqual(patch, wantArchPatch) {
		t.Fatalf("patch = %v, want %v", patch, wantArchPatch)
	}
	if !reflect.DeepEqual(render.NextToolCall, d.NextToolCall) {
		t.Errorf("top-level next_tool_call = %+v, want the refusal's patch", render.NextToolCall)
	}
	if !d.patchVerified || !strings.Contains(d.Message, wantArchAlternative) {
		t.Errorf("the refusal's patch is not marked verified, or the alternative is not named: verified=%v %s", d.patchVerified, d.Message)
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
// readable floors for every kind's example on every shipped template. Since
// go-slide-creator-3rn3s they agree on every finding: validate reports the
// render's own diagnostics, so the two sets are compared whole.
func TestDeckSpecReadabilityVerdictParityAllKindsAllTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("renders every kind on every template")
	}
	mc := refusalTestConfig(t)
	for _, tpl := range shippedTemplateNames(t) {
		for _, k := range semantic.AllSlideKinds() {
			spec := map[string]any{
				"meta":   map[string]any{"template": tpl, "title": "Parity"},
				"slides": []any{semantic.KindExample(k)},
			}
			label := tpl + "/" + string(k)
			v := deckSpecVerdicts(t, mc, map[string]any{"spec": spec})
			assertFindingParity(t, label, v)
			if v.RenderEnvelope != nil {
				t.Errorf("%s: render answered with an error envelope: %+v", label, v.RenderEnvelope.Findings)
				continue
			}
			for _, d := range v.Render.Diagnostics {
				if !d.Blocking {
					continue
				}
				// A blocking finding says where it is and what to do: a patch, or
				// the edit and its budget.
				if d.SemanticPath == "" || (d.NextToolCall == nil && d.RecommendedEdit == nil) {
					t.Errorf("%s: blocking diagnostic not source-addressed: %+v", label, d)
				}
			}
		}
	}
}

// regionsTimelineOnModernSpec is the first regions kind example with its
// right-hand stack at 75/25, which leaves the dated timeline under its heading
// too short on the modern template even trimmed: generation writes the
// milestone labels shrunk and refuses. (The example's own 40/60 stack reads
// since the dots timeline trims its rows, go-slide-creator-wj8uz.)
const regionsTimelineOnModernSpec = `{"meta":{"template":"modern","title":"Parity"},"slides":[{"kind":"regions","title":"Revenue growth funds the launch at a 32% gross margin","arrangement":"main_left","regions":[{"kind":"chart","size_pct":65,"heading":"Quarterly revenue","unit":"€m","chart":{"type":"line_chart","data":{"categories":["Q1","Q2","Q3","Q4"],"series":[{"name":"Revenue","values":[12,14,17,21]}]}}},{"kind":"stat","size_pct":75,"value":"32%","label":"Gross margin, Q4"},{"kind":"timeline","size_pct":25,"heading":"Launch plan","milestones":[{"label":"Design","date":"Oct"},{"label":"Pilot","date":"Nov"},{"label":"Rollout","date":"Dec"}]}],"source":"Finance ledger, FY26","takeaway":"Revenue nearly doubled in a year; the margin pays for the rollout."}]}`

// go-slide-creator-fn2ka: text written by a pattern nested in a grid cell (a
// regions slide's timeline) is measured by validate too. The readability check
// skipped nested patterns, so validate approved a slide render refused.
func TestDeckSpecNestedPatternReadabilityParity(t *testing.T) {
	ctx := context.Background()
	mc := refusalTestConfig(t)
	spec := decodeSpecObject(t, regionsTimelineOnModernSpec)

	vres, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatal(err)
	}
	var env deckSpecEnvelopeResponse
	structuredInto(t, vres.StructuredContent, &env)
	var refusal *diagnostics.Finding
	for i := range env.Findings {
		if env.Findings[i].Code == "INPUT.TEXT_BELOW_READABLE_MIN" && env.Findings[i].Severity == diagnostics.SeverityError {
			refusal = &env.Findings[i]
			break
		}
	}
	if env.OK || refusal == nil {
		t.Fatalf("validate must refuse the unreadable nested timeline: %+v", env.Findings)
	}
	if refusal.Path == nil || *refusal.Path != "/slides/0/regions/2" {
		t.Errorf("refusal path = %v, want the timeline region /slides/0/regions/2", refusal.Path)
	}

	rres, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatal(err)
	}
	var render renderDeckSpecResponse
	structuredInto(t, rres.StructuredContent, &render)
	if render.OK {
		t.Fatal("render must refuse the slide validate refused")
	}
}
