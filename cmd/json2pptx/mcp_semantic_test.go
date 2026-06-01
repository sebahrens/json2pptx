package main

import (
	"context"
	"encoding/json"
	"os"
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
	res, err := handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": validSemanticSpec}))
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
	res2, err := handleValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": invalidSemanticSpec}))
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

	res, err := handleExplainDeckSpec(ctx, makeRequest(map[string]any{"spec": validSemanticSpec}))
	if err != nil {
		t.Fatalf("explain_deck_spec returned go error: %v", err)
	}
	if res.IsError {
		t.Fatal("explain on a parseable spec must not be a tool error")
	}
	var explain struct {
		Template string `json:"template"`
		Slides   []struct {
			Index int    `json:"index"`
			Kind  string `json:"kind"`
		} `json:"slides"`
	}
	structuredInto(t, res.StructuredContent, &explain)
	if len(explain.Slides) != 2 {
		t.Errorf("explain returned %d slides, want 2", len(explain.Slides))
	}
	if len(explain.Slides) > 0 && explain.Slides[0].Kind != "title" {
		t.Errorf("first slide kind = %q, want title", explain.Slides[0].Kind)
	}

	// Unparseable spec → structured tool error.
	res2, err := handleExplainDeckSpec(ctx, makeRequest(map[string]any{"spec": "meta: [this: is: not: valid"}))
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
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, res2.StructuredContent, &kinds)
	if len(kinds.SlideKinds) == 0 {
		t.Fatal("list_slide_kinds returned no kinds")
	}
	var sawKPI bool
	for _, k := range kinds.SlideKinds {
		if k.Kind == "kpi_snapshot" {
			sawKPI = true
			if len(k.RequiredFields) == 0 {
				t.Error("kpi_snapshot should list required_fields")
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
			handler:  handleValidateDeckSpec,
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
