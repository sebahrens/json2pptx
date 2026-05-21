package main

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
)

// TestMCPArgErrors_EnvelopeShape exercises every MCP handler that takes a
// required argument with empty args (missing-required) and with a wrong-type
// value (when the type contract is clear). The contract under test is the
// arg-validation envelope: every response must be an error with a
// machine-readable `path` field on the first diagnostic, and at least one of
// `expected_type` or `next_tool_call` populated so an agent can self-correct
// without re-reading the schema.
//
// Tools that take no required arguments are skipped — they have nothing to
// arg-validate. The list of tools below tracks registerMCPTools in mcp.go; if
// a new tool is added with required args, add it here.
func TestMCPArgErrors_EnvelopeShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()

	type missingCase struct {
		// name labels the subtest.
		name string
		// handler is the tool handler under test.
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		// args is the request payload. Use map{} to trigger missing-required;
		// supply a value of the wrong type to trigger an invalid-type error.
		args map[string]any
		// wantPath is the JSON path the diagnostic must surface.
		wantPath string
	}

	// Wrap method handlers so they match the plain function signature.
	wrap := func(fn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return fn
	}

	cases := []missingCase{
		// generate_presentation — required: presentation (object)
		{name: "generate_presentation/missing", handler: wrap(mc.handleGenerate), args: map[string]any{}, wantPath: "presentation"},
		{name: "generate_presentation/wrong_type", handler: wrap(mc.handleGenerate), args: map[string]any{"presentation": "not-an-object"}, wantPath: "presentation"},

		// validate_input — required: presentation (object)
		{name: "validate_input/missing", handler: wrap(mc.handleValidate), args: map[string]any{}, wantPath: "presentation"},
		{name: "validate_input/wrong_type", handler: wrap(mc.handleValidate), args: map[string]any{"presentation": "not-an-object"}, wantPath: "presentation"},

		// recommend_pattern — required: intent (string)
		{name: "recommend_pattern/missing", handler: wrap(mc.handleRecommendPattern), args: map[string]any{}, wantPath: "intent"},

		// recommend_visual — required: intent (string)
		{name: "recommend_visual/missing", handler: wrap(mc.handleRecommendVisual), args: map[string]any{}, wantPath: "intent"},

		// show_pattern — required: name (string)
		{name: "show_pattern/missing", handler: handleShowPattern, args: map[string]any{}, wantPath: "name"},

		// validate_pattern — required: name (string), values (object)
		{name: "validate_pattern/missing_name", handler: handleValidatePattern, args: map[string]any{}, wantPath: "name"},
		{name: "validate_pattern/missing_values", handler: handleValidatePattern, args: map[string]any{"name": "kpi-3up"}, wantPath: "values"},

		// expand_pattern — required: name (string), values (object)
		{name: "expand_pattern/missing_name", handler: wrap(mc.handleExpandPattern), args: map[string]any{}, wantPath: "name"},
		{name: "expand_pattern/missing_values", handler: wrap(mc.handleExpandPattern), args: map[string]any{"name": "kpi-3up"}, wantPath: "values"},

		// expand_patterns — required: names (array)
		{name: "expand_patterns/missing", handler: wrap(mc.handleExpandPatterns), args: map[string]any{}, wantPath: "names"},

		// repair_slide — required: presentation (object), fixes (array)
		{name: "repair_slide/missing_presentation", handler: wrap(mc.handleRepairSlide), args: map[string]any{}, wantPath: "presentation"},

		// repair_slides_batch — required: presentation (object), fixes (array)
		{name: "repair_slides_batch/missing", handler: wrap(mc.handleRepairSlidesBatch), args: map[string]any{}, wantPath: "presentation"},

		// propose_repairs — required: presentation (object), findings (array)
		{name: "propose_repairs/missing", handler: wrap(mc.handleProposeRepairs), args: map[string]any{}, wantPath: "presentation"},

		// preview_presentation_plan — required: presentation (object)
		{name: "preview_presentation_plan/missing", handler: wrap(mc.handlePreviewPlan), args: map[string]any{}, wantPath: "presentation"},

		// preview_slide_wireframe — required: presentation (object), slide_index (int)
		{name: "preview_slide_wireframe/missing", handler: wrap(mc.handlePreviewSlideWireframe), args: map[string]any{}, wantPath: "presentation"},

		// score_deck — required: presentation (object)
		{name: "score_deck/missing", handler: wrap(mc.handleScoreDeck), args: map[string]any{}, wantPath: "presentation"},

		// score_candidates — required: presentation, candidates
		{name: "score_candidates/missing", handler: wrap(mc.handleScoreCandidates), args: map[string]any{}, wantPath: "presentation"},

		// inspect_slide_images — required: slide_images (array)
		{name: "inspect_slide_images/missing", handler: wrap(mc.handleInspectSlideImages), args: map[string]any{}, wantPath: "slide_images"},

		// analyze_deck_rhythm — required: presentation (object)
		{name: "analyze_deck_rhythm/missing", handler: handleAnalyzeDeckRhythm, args: map[string]any{}, wantPath: "presentation"},

		// plan_deck — required: brief (string)
		{name: "plan_deck/missing", handler: wrap(mc.handlePlanDeck), args: map[string]any{}, wantPath: "brief"},

		// describe_finding — required: code (string)
		{name: "describe_finding/missing", handler: handleDescribeFinding, args: map[string]any{}, wantPath: "code"},

		// resolve_theme — required: template_name (string)
		{name: "resolve_theme/missing", handler: wrap(mc.handleResolveTheme), args: map[string]any{}, wantPath: "template_name"},

		// render_slide_image — required: pptx_path (string)
		{name: "render_slide_image/missing", handler: wrap(mc.handleRenderSlideImage), args: map[string]any{}, wantPath: "pptx_path"},

		// render_deck_thumbnails — required: pptx_path (string)
		{name: "render_deck_thumbnails/missing", handler: wrap(mc.handleRenderDeckThumbnails), args: map[string]any{}, wantPath: "pptx_path"},

		// render_slide_image_from_json — required: slide (object), template (string)
		{name: "render_slide_image_from_json/missing_slide", handler: wrap(mc.handleRenderSlideImageFromJSON), args: map[string]any{}, wantPath: "slide"},
		{name: "render_slide_image_from_json/missing_template", handler: wrap(mc.handleRenderSlideImageFromJSON), args: map[string]any{"slide": map[string]any{"layout_id": "title"}}, wantPath: "template"},

		// read_presentation — required: pptx_path (string)
		{name: "read_presentation/missing", handler: handleReadPresentation, args: map[string]any{}, wantPath: "pptx_path"},

		// validate_output — required: path (string)
		{name: "validate_output/missing", handler: handleValidateOutput, args: map[string]any{}, wantPath: "path"},

		// list_template_settings — required: template_name (string)
		{name: "list_template_settings/missing", handler: wrap(mc.handleListTemplateSettings), args: map[string]any{}, wantPath: "template_name"},

		// preview_icon — required: icon (object)
		{name: "preview_icon/missing", handler: wrap(mc.handlePreviewIcon), args: map[string]any{}, wantPath: "icon"},

		// audit_palette — required: pptx_path (string)
		{name: "audit_palette/missing", handler: handleAuditPalette, args: map[string]any{}, wantPath: "pptx_path"},

		// examine_template — required: template_name (string)
		{name: "examine_template/missing", handler: wrap(mc.handleExamineTemplate), args: map[string]any{}, wantPath: "template_name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.handler(ctx, makeRequest(tc.args))
			if err != nil {
				t.Fatalf("handler returned go error: %v", err)
			}
			// Read the FindingEnvelope wire shape directly: the offending JSON
			// path and the expected type live in findings[0].evidence; the
			// recovery hint is findings[0].next_tool_call.
			fe := parseMCPFindingEnvelope(t, result)
			if len(fe.Findings) == 0 {
				t.Fatalf("expected at least one finding, got empty envelope")
			}
			f := fe.Findings[0]
			path, _ := f.Evidence["path"].(string)
			if path == "" {
				t.Errorf("expected evidence.path on finding, got empty (code=%q, message=%q)", f.Code, f.Message)
			}
			if tc.wantPath != "" && path != tc.wantPath {
				// A few handlers normalize the path differently (e.g. presentation.slides);
				// accept any path that begins with the expected root path.
				if len(path) < len(tc.wantPath) || path[:len(tc.wantPath)] != tc.wantPath {
					t.Errorf("expected path=%q (or prefix), got %q", tc.wantPath, path)
				}
			}
			// Every arg-validation finding must surface at least one of:
			// evidence.expected_type or next_tool_call. Both are agent-actionable;
			// one alone is enough for a recovery path.
			expectedType, _ := f.Evidence["expected_type"].(string)
			if expectedType == "" && f.NextToolCall == nil {
				t.Errorf("expected finding to carry evidence.expected_type or next_tool_call, got neither (code=%q, message=%q)", f.Code, f.Message)
			}
		})
	}
}

// TestArgErrorHelpers_ProduceEnvelope is a direct unit test for the shared
// helpers in mcp_errors.go. It guards against silent regressions in the
// envelope shape (path / expected_type / example_value / next_tool_call)
// independent of the per-tool wiring.
func TestArgErrorHelpers_ProduceEnvelope(t *testing.T) {
	t.Run("argMissing_defaults_to_retry_next_call", func(t *testing.T) {
		result := argMissing("repair_slide", "fixes", "array", []any{map[string]any{"kind": "reduce_text"}}, nil)
		env := parseMCPError(t, result)
		d := env.Diagnostics[0]
		if d.Code != "MISSING_PARAMETER" {
			t.Errorf("expected code=MISSING_PARAMETER, got %q", d.Code)
		}
		if d.Path != "fixes" {
			t.Errorf("expected path=fixes, got %q", d.Path)
		}
		if d.ExpectedType != "array" {
			t.Errorf("expected expected_type=array, got %q", d.ExpectedType)
		}
		if d.ExampleValue == nil {
			t.Error("expected example_value, got nil")
		}
		if d.NextToolCall == nil || d.NextToolCall.Tool != "repair_slide" {
			t.Errorf("expected next_tool_call.tool=repair_slide, got %v", d.NextToolCall)
		}
	})

	t.Run("argInvalidJSON_defaults_to_get_input_schema", func(t *testing.T) {
		result := argInvalidJSON("presentation", "invalid: bad token", "object", nil, nil)
		env := parseMCPError(t, result)
		d := env.Diagnostics[0]
		if d.Code != "INVALID_JSON" {
			t.Errorf("expected code=INVALID_JSON, got %q", d.Code)
		}
		if d.Path != "presentation" {
			t.Errorf("expected path=presentation, got %q", d.Path)
		}
		if d.ExpectedType != "object" {
			t.Errorf("expected expected_type=object, got %q", d.ExpectedType)
		}
		if d.NextToolCall == nil || d.NextToolCall.Tool != "get_input_schema" {
			t.Errorf("expected next_tool_call.tool=get_input_schema, got %v", d.NextToolCall)
		}
	})

	t.Run("argInvalidValue_custom_code_and_example", func(t *testing.T) {
		result := argInvalidValue("validate_pattern", "INVALID_KEY", "cell_overrides.foo", "key must be an integer", "integer", 0, nil)
		env := parseMCPError(t, result)
		d := env.Diagnostics[0]
		if d.Code != "INVALID_KEY" {
			t.Errorf("expected code=INVALID_KEY, got %q", d.Code)
		}
		if d.Path != "cell_overrides.foo" {
			t.Errorf("expected path=cell_overrides.foo, got %q", d.Path)
		}
		if d.ExpectedType != "integer" {
			t.Errorf("expected expected_type=integer, got %q", d.ExpectedType)
		}
		// Severity must be error so MCP marks the response as IsError.
		if d.Severity != diagnostics.SeverityError {
			t.Errorf("expected severity=error, got %q", d.Severity)
		}
		if d.NextToolCall == nil || d.NextToolCall.Tool != "validate_pattern" {
			t.Errorf("expected next_tool_call.tool=validate_pattern, got %v", d.NextToolCall)
		}
	})
}
