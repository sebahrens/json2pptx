package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
)

// semanticTestConfig builds an mcpConfig wired at the bundled templates and a
// per-test output dir, matching the other MCP handler tests.
func semanticTestConfig(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

// structuredInto re-encodes a tool result's StructuredContent into v.
func structuredInto(t *testing.T, sc any, v any) {
	t.Helper()
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal StructuredContent into %T: %v", v, err)
	}
}

// TestSemanticMCP_ValidateDeckSpec exercises validate_deck_spec on a clean spec
// (passed as a string) and a dirty spec, asserting the shared finding envelope
// shape and the ok flag.
func TestSemanticMCP_ValidateDeckSpec(t *testing.T) {
	ctx := context.Background()

	// Clean spec → ok=true.
	res, err := testValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": validSemanticSpec}))
	if err != nil {
		t.Fatalf("validate_deck_spec returned go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("validate_deck_spec on a clean spec must not be a tool error")
	}
	var env diagnostics.FindingEnvelope
	structuredInto(t, res.StructuredContent, &env)
	if env.Subcommand != "validate_deck_spec" {
		t.Errorf("envelope.subcommand = %q, want validate_deck_spec", env.Subcommand)
	}
	if !env.OK {
		t.Errorf("clean spec should validate ok=true, got findings: %+v", env.Findings)
	}
	if len(env.InputSHA256) != 64 {
		t.Errorf("envelope.input_sha256 = %q, want 64-hex digest", env.InputSHA256)
	}

	// Dirty spec → ok=false with at least one error-severity finding.
	res2, err := testValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": invalidSemanticSpec}))
	if err != nil {
		t.Fatalf("validate_deck_spec(dirty) returned go error: %v", err)
	}
	var env2 diagnostics.FindingEnvelope
	structuredInto(t, res2.StructuredContent, &env2)
	if env2.OK {
		t.Error("invalid spec should validate ok=false")
	}
	if len(env2.Findings) == 0 {
		t.Error("invalid spec should surface findings")
	}
}

func TestSemanticMCP_ValidateTemplateMatchesRenderAndHandle(t *testing.T) {
	mc := handleTestConfig(t)
	spec := strings.Replace(validSemanticSpec, "  template: midnight-blue\n", "  archetype: strategy_proposal\n", 1)
	validated := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"spec": spec, "template": "midnight-blue",
	}))
	if !validated.OK {
		t.Fatalf("selected-template validation failed: %+v", validated.Findings)
	}
	handle, ok := mc.deckHandles.Load(validated.DeckID)
	if !ok || handle.Template != "midnight-blue" {
		t.Fatalf("validated deck handle template = %+v, want midnight-blue", handle)
	}
	patched := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"deck_id": validated.DeckID,
		"patch":   []any{map[string]any{"op": "replace", "path": "/slides/0/title", "value": "Revised Review"}},
	}))
	if !patched.OK || patched.DeckID != validated.DeckID {
		t.Fatalf("patched validation lost the deck handle: %+v", patched)
	}
	if handle, ok = mc.deckHandles.Load(patched.DeckID); !ok || handle.Template != "midnight-blue" {
		t.Fatalf("patched validation lost selected template: %+v", handle)
	}
	var render renderDeckSpecResponse
	structuredInto(t, mustCall(t, mc.handleRenderDeckSpec, map[string]any{"deck_id": patched.DeckID}).StructuredContent, &render)
	if !render.Success || render.Template != "midnight-blue" || render.Explanation == nil || render.Explanation.Template != "midnight-blue" {
		t.Fatalf("render template/explanation drifted from validation: %+v", render)
	}

	pinned := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"spec": validSemanticSpec, "template": "warm-coral",
	}))
	if got, ok := mc.deckHandles.Load(pinned.DeckID); !ok || got.Template != "midnight-blue" {
		t.Errorf("spec-pinned template should win: %+v", got)
	}
	bad := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": 42})
	if !bad.IsError {
		t.Error("validate_deck_spec accepted a non-string template argument")
	}
	missing := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"spec": spec, "template": "definitely-not-a-template",
	}))
	if missing.OK {
		t.Fatalf("validate_deck_spec accepted a missing selected template: %+v", missing.Findings)
	}
	foundMissing := false
	for _, finding := range missing.Findings {
		foundMissing = foundMissing || strings.HasSuffix(finding.Code, string(diagnostics.CodeTemplateNotFound))
	}
	if !foundMissing {
		t.Errorf("missing selected template was not diagnosed: %+v", missing.Findings)
	}
}

func TestSemanticMCP_ValidateSelectedTemplateUsesRotatedAccent(t *testing.T) {
	mc := handleTestConfig(t)
	const spec = `{"meta":{"title":"Review","archetype":"strategy_proposal","accent_strategy":"section-keyed"},"slides":[{"kind":"title","title":"Review"},{"kind":"section","title":"Results"},{"kind":"kpi_snapshot","title":"Results at a glance","kpis":[{"value":"42%","label":"Growth"},{"value":"1.2M","label":"ARR"}]}]}`
	var forest deckSpecEnvelopeResponse
	structuredInto(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"spec": spec, "template": "forest-green",
	}).StructuredContent, &forest)
	var midnight deckSpecEnvelopeResponse
	structuredInto(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"spec": spec, "template": "midnight-blue",
	}).StructuredContent, &midnight)
	forestAccent := false
	for _, finding := range forest.Findings {
		if strings.Contains(finding.Message, "#FF8F00") && strings.HasSuffix(finding.Code, "contrast_predicted") {
			forestAccent = true
		}
	}
	if !forestAccent {
		t.Fatalf("forest-green validation did not predict contrast on rotated accent2 #FF8F00: %+v", forest.Findings)
	}
	for _, finding := range midnight.Findings {
		if strings.Contains(finding.Message, "#FF8F00") {
			t.Fatalf("midnight-blue validation leaked forest-green accent: %+v", finding)
		}
	}
}

func TestSemanticMCP_ValidateAndExplainRejectUnknownTemplate(t *testing.T) {
	ctx := context.Background()
	mc := semanticTestConfig(t)
	spec := `meta:
  title: Missing template
  template: definitely-not-a-template
slides:
  - kind: title
    title: Missing template
`
	validated, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatal(err)
	}
	var env diagnostics.FindingEnvelope
	structuredInto(t, validated.StructuredContent, &env)
	if env.OK {
		t.Fatalf("validate_deck_spec accepted an unknown template: %+v", env)
	}
	found := false
	for _, finding := range env.Findings {
		if strings.HasSuffix(finding.Code, string(diagnostics.CodeTemplateNotFound)) && finding.Evidence["path"] == "meta.template" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing TEMPLATE_NOT_FOUND at meta.template: %+v", env.Findings)
	}
	explained, err := mc.handleExplainDeckSpec(ctx, makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatal(err)
	}
	if !explained.IsError {
		t.Fatal("explain_deck_spec accepted an unknown template")
	}
}

func TestSemanticMCP_ParseFailuresReturnFindingsWithoutHandles(t *testing.T) {
	mc := handleTestConfig(t)
	for _, spec := range []string{
		"meta: [unterminated",
		"meta:\n  title: Review\nslides:\n  - kind: not_a_kind\n    title: Bad slide\n",
	} {
		validated := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec})
		var response deckSpecEnvelopeResponse
		structuredInto(t, validated.StructuredContent, &response)
		if response.OK || response.DeckID != "" {
			t.Fatalf("invalid spec got a successful validation or deck handle: %+v", response)
		}
		if strings.Contains(spec, "not_a_kind") {
			found := false
			for _, finding := range response.Findings {
				if strings.HasSuffix(finding.Code, diagnostics.CodeSemanticUnknownKind) {
					found = true
					if finding.NextToolCall == nil || finding.NextToolCall.Tool != "list_slide_kinds" {
						t.Fatalf("validation unknown-kind finding lacks discovery call: %+v", finding)
					}
				}
			}
			if !found {
				t.Fatalf("validation lost SEMANTIC_UNKNOWN_KIND: %+v", response.Findings)
			}
		}
		caseName := "invalid_yaml"
		if strings.Contains(spec, "not_a_kind") {
			caseName = "unknown_kind"
		}
		for _, producer := range []struct {
			name string
			fn   func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		}{
			{"render_deck_spec", mc.handleRenderDeckSpec},
			{"compile_deck_spec", handleCompileDeckSpec},
		} {
			t.Run(producer.name+"/"+caseName, func(t *testing.T) {
				result := mustCall(t, producer.fn, map[string]any{"spec": spec})
				if !result.IsError {
					t.Fatal("parse failure must be an MCP error")
				}
				var envelope diagnostics.FindingEnvelope
				structuredInto(t, result.StructuredContent, &envelope)
				if envelope.OK || len(envelope.Findings) == 0 {
					t.Fatalf("parse failure has no actionable finding envelope: %+v", envelope)
				}
				if caseName == "invalid_yaml" && !strings.HasSuffix(envelope.Findings[0].Code, diagnostics.CodeInvalidJSON) {
					t.Fatalf("invalid YAML finding code = %q, want INVALID_JSON", envelope.Findings[0].Code)
				}
				if strings.Contains(spec, "not_a_kind") {
					found := false
					for _, finding := range envelope.Findings {
						if !strings.HasSuffix(finding.Code, diagnostics.CodeSemanticUnknownKind) {
							continue
						}
						found = true
						if finding.Evidence["path"] != "slides[0].kind" || finding.NextToolCall == nil || finding.NextToolCall.Tool != "list_slide_kinds" {
							t.Fatalf("unknown kind finding lacks source path or recovery call: %+v", finding)
						}
						if available, ok := finding.Evidence["available"].([]any); !ok || len(available) == 0 {
							t.Fatalf("unknown kind finding lacks available kinds: %+v", finding)
						}
					}
					if !found {
						t.Fatalf("missing SEMANTIC_UNKNOWN_KIND finding: %+v", envelope.Findings)
					}
				}
			})
		}
	}
}

func TestSemanticMCP_InvalidPatchDoesNotReplaceStoredDeck(t *testing.T) {
	mc := handleTestConfig(t)
	var validated deckSpecEnvelopeResponse
	structuredInto(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": validSemanticSpec}).StructuredContent, &validated)
	if validated.DeckID == "" {
		t.Fatal("valid spec did not get a deck handle")
	}
	before, ok := mc.deckHandles.Load(validated.DeckID)
	if !ok {
		t.Fatal("new deck handle is not loadable")
	}
	result := mustCall(t, mc.handleRenderDeckSpec, map[string]any{
		"deck_id": validated.DeckID,
		"patch":   []any{map[string]any{"op": "replace", "path": "/slides/0/kind", "value": "not_a_kind"}},
	})
	if !result.IsError {
		t.Fatal("invalid patch was rendered")
	}
	after, ok := mc.deckHandles.Load(validated.DeckID)
	if !ok {
		t.Fatal("failed render removed the stored valid deck")
	}
	if string(after.Spec) != string(before.Spec) {
		t.Fatalf("failed render corrupted the stored valid deck: before=%q after=%q", before.Spec, after.Spec)
	}
}

func TestSemanticMCP_MissingTemplateHasAvailableNamesAndSuggestion(t *testing.T) {
	mc := semanticTestConfig(t)
	spec := strings.Replace(validSemanticSpec, "  template: midnight-blue\n", "", 1)
	for _, call := range []struct {
		name string
		fn   func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"validate", mc.handleValidateDeckSpec},
		{"render", mc.handleRenderDeckSpec},
	} {
		t.Run(call.name, func(t *testing.T) {
			result := mustCall(t, call.fn, map[string]any{"spec": spec, "template": "midnight-blu"})
			var envelope diagnostics.FindingEnvelope
			structuredInto(t, result.StructuredContent, &envelope)
			if envelope.OK {
				t.Fatalf("missing template was accepted: %+v", envelope)
			}
			found := false
			for _, finding := range envelope.Findings {
				if !strings.HasSuffix(finding.Code, diagnostics.CodeTemplateNotFound) {
					continue
				}
				found = true
				if finding.NextToolCall == nil || finding.NextToolCall.Tool != "list_templates" {
					t.Fatalf("missing template finding has no discovery call: %+v", finding)
				}
				if available, ok := finding.Evidence["available_templates"].([]any); !ok || len(available) == 0 {
					t.Fatalf("missing template finding has no available list: %+v", finding)
				}
				if available, ok := finding.Evidence["available"].([]any); !ok || len(available) == 0 {
					t.Fatalf("missing template finding has no canonical available list: %+v", finding)
				}
				if finding.Remediation == nil || finding.Remediation.Primary == nil || finding.Remediation.Primary.Params["did_you_mean"] != "midnight-blue" {
					t.Fatalf("missing template finding has no typo suggestion: %+v", finding)
				}
			}
			if !found {
				t.Fatalf("missing TEMPLATE_NOT_FOUND finding: %+v", envelope.Findings)
			}
		})
	}
}

// TestSemanticMCP_CompileDeckSpec verifies compact-by-default output and the
// include_compiled_json escape hatch.
func TestSemanticMCP_CompileDeckSpec(t *testing.T) {
	ctx := context.Background()

	// Compact (default): no compiled_json.
	res, err := handleCompileDeckSpec(ctx, makeRequest(map[string]any{"spec": validSemanticSpec}))
	if err != nil {
		t.Fatalf("compile_deck_spec returned go error: %v", err)
	}
	var compact compileDeckSpecResponse
	structuredInto(t, res.StructuredContent, &compact)
	if !compact.OK {
		t.Fatalf("clean spec should compile ok=true, error=%q diags=%+v", compact.Error, compact.Diagnostics)
	}
	if compact.SlideCount != 2 {
		t.Errorf("slide_count = %d, want 2", compact.SlideCount)
	}
	if compact.Template != "midnight-blue" {
		t.Errorf("template = %q, want midnight-blue", compact.Template)
	}
	if len(compact.CompiledJSON) != 0 {
		t.Error("compiled_json must be omitted unless include_compiled_json=true")
	}

	// Full: compiled_json present and is a valid PresentationInput-shaped object.
	res2, err := handleCompileDeckSpec(ctx, makeRequest(map[string]any{
		"spec":                  validSemanticSpec,
		"include_compiled_json": true,
	}))
	if err != nil {
		t.Fatalf("compile_deck_spec(include) returned go error: %v", err)
	}
	var full compileDeckSpecResponse
	structuredInto(t, res2.StructuredContent, &full)
	if len(full.CompiledJSON) == 0 {
		t.Fatal("compiled_json must be present when include_compiled_json=true")
	}
	var compiled map[string]any
	if err := json.Unmarshal(full.CompiledJSON, &compiled); err != nil {
		t.Fatalf("compiled_json is not a JSON object: %v", err)
	}
	if _, ok := compiled["slides"]; !ok {
		t.Error("compiled_json missing slides[] — not a PresentationInput")
	}
}

// TestSemanticMCP_RenderDeckSpec exercises the one-call render path and asserts
// the artifact, quality summary, and explanation summary are returned.
func TestSemanticMCP_RenderDeckSpec(t *testing.T) {
	ctx := context.Background()
	mc := semanticTestConfig(t)

	res, err := mc.handleRenderDeckSpec(ctx, makeRequest(map[string]any{"spec": validSemanticSpec}))
	if err != nil {
		t.Fatalf("render_deck_spec returned go error: %v", err)
	}
	var render renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &render)
	if !render.Success || !render.OK {
		t.Fatalf("render should succeed; error=%q diags=%+v", render.Error, render.Diagnostics)
	}
	if render.PptxPath == "" {
		t.Error("render response missing pptx_path")
	} else if _, err := os.Stat(render.PptxPath); err != nil {
		t.Errorf("pptx_path %q does not exist on disk: %v", render.PptxPath, err)
	}
	if render.SlideCount != 2 {
		t.Errorf("slide_count = %d, want 2", render.SlideCount)
	}
	if render.Quality == nil {
		t.Error("render response missing quality_summary")
	}
	if render.Explanation == nil {
		t.Fatal("render response missing explanation_summary")
	}
	if len(render.Explanation.Slides) != 2 {
		t.Errorf("explanation_summary has %d slides, want 2", len(render.Explanation.Slides))
	}
}

// TestSemanticMCP_ExplainDeckSpec asserts the explain projection and the
// parse-error path.
func TestSemanticMCP_ExplainDeckSpec(t *testing.T) {
	ctx := context.Background()

	res, err := testExplainDeckSpec(ctx, makeRequest(map[string]any{"spec": validSemanticSpec}))
	if err != nil {
		t.Fatalf("explain_deck_spec returned go error: %v", err)
	}
	if res.IsError {
		t.Fatal("explain on a parseable spec must not be a tool error")
	}
	var explain struct {
		Template string `json:"template"`
		Slides   []struct {
			Index        int    `json:"index"`
			Kind         string `json:"kind"`
			Alternatives []struct {
				Layout string `json:"layout"`
				Reason string `json:"reason"`
			} `json:"alternatives"`
		} `json:"slides"`
	}
	structuredInto(t, res.StructuredContent, &explain)
	if len(explain.Slides) != 2 {
		t.Errorf("explain returned %d slides, want 2", len(explain.Slides))
	}
	if len(explain.Slides) > 0 && explain.Slides[0].Kind != "title" {
		t.Errorf("first slide kind = %q, want title", explain.Slides[0].Kind)
	}
	if len(explain.Slides) > 0 {
		alts := explain.Slides[0].Alternatives
		if len(alts) == 0 || alts[0].Layout != "title" || alts[0].Reason == "" {
			t.Errorf("title slide must explain its supported composition: %+v", alts)
		}
	}

	// Unparseable spec → structured tool error.
	res2, err := testExplainDeckSpec(ctx, makeRequest(map[string]any{"spec": "meta: [this: is: not: valid"}))
	if err != nil {
		t.Fatalf("explain_deck_spec(bad) returned go error: %v", err)
	}
	if !res2.IsError {
		t.Error("explain on an unparseable spec must be a tool error")
	}
}

// TestSemanticMCP_ListArchetypesAndKinds asserts the discovery tools return the
// registries.
func TestSemanticMCP_ListArchetypesAndKinds(t *testing.T) {
	ctx := context.Background()

	res, err := handleListDeckArchetypes(ctx, makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("list_deck_archetypes returned go error: %v", err)
	}
	var arch struct {
		Archetypes []archetypeListEntry `json:"archetypes"`
	}
	structuredInto(t, res.StructuredContent, &arch)
	if len(arch.Archetypes) == 0 {
		t.Fatal("list_deck_archetypes returned no archetypes")
	}
	var sawStrategy bool
	for _, a := range arch.Archetypes {
		if a.Archetype == "strategy_proposal" {
			sawStrategy = true
			if a.DefaultTemplate == "" {
				t.Error("strategy_proposal should carry a default_template")
			}
		}
	}
	if !sawStrategy {
		t.Error("expected strategy_proposal in archetypes")
	}

	res2, err := handleListSlideKinds(ctx, makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("list_slide_kinds returned go error: %v", err)
	}
	var kinds struct {
		SlideKinds     []slideKindListEntry `json:"slide_kinds"`
		TakeawayBudget struct {
			FontPt   float64 `json:"font_pt"`
			MaxLines int     `json:"max_lines"`
			Note     string  `json:"note"`
		} `json:"takeaway_budget"`
	}
	structuredInto(t, res2.StructuredContent, &kinds)
	if len(kinds.SlideKinds) == 0 {
		t.Fatal("list_slide_kinds returned no kinds")
	}
	if kinds.TakeawayBudget.FontPt != 14 || kinds.TakeawayBudget.MaxLines != 1 || !strings.Contains(kinds.TakeawayBudget.Note, "BODY_TOO_LONG") {
		t.Errorf("list_slide_kinds lacks the measured takeaway budget: %+v", kinds.TakeawayBudget)
	}
	var sawKPI bool
	for _, k := range kinds.SlideKinds {
		if k.Kind == "kpi_snapshot" {
			sawKPI = true
			if len(k.RequiredFields) == 0 {
				t.Error("kpi_snapshot should list required_fields")
			}
			// The "metrics" alias for "kpis" must be discoverable, so an agent
			// knows a metrics-only spec is accepted (go-slide-creator-i2p4).
			if got := k.RequiredAliases["kpis"]; len(got) == 0 || got[0] != "metrics" {
				t.Errorf("kpi_snapshot should expose required_aliases kpis->[metrics], got %v", k.RequiredAliases)
			}
		}
	}
	if !sawKPI {
		t.Error("expected kpi_snapshot in slide_kinds")
	}
}

// TestSemanticMCP_SpecAsObject proves the spec argument is accepted as a JSON
// object (the natural MCP authoring path), not only as a YAML/JSON string.
func TestSemanticMCP_SpecAsObject(t *testing.T) {
	ctx := context.Background()

	specObj := map[string]any{
		"meta": map[string]any{
			"title":    "Object Spec",
			"template": "midnight-blue",
		},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Object Spec"},
			map[string]any{"kind": "closing", "title": "Thanks"},
		},
	}

	res, err := handleCompileDeckSpec(ctx, makeRequest(map[string]any{"spec": specObj}))
	if err != nil {
		t.Fatalf("compile_deck_spec(object) returned go error: %v", err)
	}
	var compact compileDeckSpecResponse
	structuredInto(t, res.StructuredContent, &compact)
	if !compact.OK {
		t.Fatalf("object spec should compile ok=true, error=%q diags=%+v", compact.Error, compact.Diagnostics)
	}
	if compact.SlideCount != 2 {
		t.Errorf("slide_count = %d, want 2", compact.SlideCount)
	}
}

// TestSemanticMCP_WrongTypedEnumArgs asserts that semantic MCP tools fail fast
// on optional enum/string/bool arguments supplied with the wrong JSON type
// instead of silently treating the wrong-typed value as absent and defaulting.
// A present-but-wrong-type `strict`, `template`, `output_validation`, or
// `include_compiled_json` must yield a structured INVALID_PARAMETER finding
// pointing at the offending path — not a quiet warn-mode / non-strict run.
func TestSemanticMCP_WrongTypedEnumArgs(t *testing.T) {
	ctx := context.Background()
	mc := semanticTestConfig(t)

	cases := []struct {
		name     string
		handler  func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args     map[string]any
		wantPath string
	}{
		{
			name:     "validate/strict_bool",
			handler:  testValidateDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "strict": true},
			wantPath: "strict",
		},
		{
			name:     "compile/strict_number",
			handler:  handleCompileDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "strict": 1},
			wantPath: "strict",
		},
		{
			name:     "compile/template_number",
			handler:  handleCompileDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "template": 42},
			wantPath: "template",
		},
		{
			name:     "compile/include_compiled_json_string",
			handler:  handleCompileDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "include_compiled_json": "true"},
			wantPath: "include_compiled_json",
		},
		{
			name:     "render/strict_bool",
			handler:  mc.handleRenderDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "strict": true},
			wantPath: "strict",
		},
		{
			name:     "render/template_bool",
			handler:  mc.handleRenderDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "template": true},
			wantPath: "template",
		},
		{
			name:     "render/output_validation_bool",
			handler:  mc.handleRenderDeckSpec,
			args:     map[string]any{"spec": validSemanticSpec, "output_validation": true},
			wantPath: "output_validation",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.handler(ctx, makeRequest(tc.args))
			if err != nil {
				t.Fatalf("handler returned go error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("wrong-typed %s must be a tool error, not a silent default", tc.wantPath)
			}
			env := parseMCPError(t, res)
			if len(env.Diagnostics) == 0 {
				t.Fatal("expected at least one diagnostic")
			}
			d := env.Diagnostics[0]
			if d.Code != "INVALID_PARAMETER" {
				t.Errorf("code = %q, want INVALID_PARAMETER", d.Code)
			}
			if d.Path != tc.wantPath {
				t.Errorf("path = %q, want %q", d.Path, tc.wantPath)
			}
		})
	}
}

// testValidateDeckSpec calls the handler with a config pointed at the bundled
// templates: validate_deck_spec now compiles the spec and runs the shared fit
// collectors against the compiled deck, which needs the template geometry
// (go-slide-creator-05wn).
func testValidateDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	return mc.handleValidateDeckSpec(ctx, request)
}

// testExplainDeckSpec calls explain_deck_spec on a throwaway config, mirroring
// testValidateDeckSpec. The handler took a config receiver when deck handles
// landed (go-slide-creator-voxp).
func testExplainDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	return mc.handleExplainDeckSpec(ctx, request)
}
