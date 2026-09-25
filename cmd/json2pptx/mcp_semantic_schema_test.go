package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-h8o7: the MCP input schema for spec must carry the real
// DeckSpec schema — slides[].items.oneOf with one closed variant per kind —
// instead of an empty object.
//
// go-slide-creator-uhaq: it carries it ONCE. The same 17KB was embedded in all
// four spec tools, twice in the core listing; it now lives on
// validate_deck_spec, the tool the workflow says to call before rendering,
// while the others declare the outline and point at list_slide_kinds. The
// payload contract is unchanged — an unknown field is still rejected by the
// compiler, whichever tool receives it (asserted by
// TestSpecOutlineToolsStillRejectUnknownFields).
func TestSemanticMCP_SpecInputSchemaHasPerKindOneOf(t *testing.T) {
	kinds := semantic.AllSlideKinds()
	spec := toolSpecProperty(t, mcpValidateDeckSpecTool().InputSchema.Properties)
	const tool = "validate_deck_spec"

	if !reflect.DeepEqual(spec["type"], []string{"object", "string"}) {
		t.Errorf("%s: spec.type = %v, want object|string", tool, spec["type"])
	}
	if spec["additionalProperties"] != false {
		t.Errorf("%s: spec must be closed (additionalProperties:false)", tool)
	}
	props, _ := spec["properties"].(map[string]any)
	slides, _ := props["slides"].(map[string]any)
	items, _ := slides["items"].(map[string]any)
	defs, _ := spec["$defs"].(map[string]any)
	if ref, _ := items["$ref"].(string); !strings.HasSuffix(ref, "/$defs/S") {
		t.Fatalf("%s: slides.items ref = %q, want compact SlideSpec definition", tool, ref)
	}
	slideSpec, _ := defs["S"].(map[string]any)
	if slideSpec["unevaluatedProperties"] != false {
		t.Fatalf("%s: SlideSpec union must set unevaluatedProperties:false", tool)
	}
	oneOf, _ := slideSpec["oneOf"].([]any)
	if len(oneOf) != len(kinds) {
		t.Fatalf("%s: slides.items.oneOf has %d variants, want %d (one per kind)", tool, len(oneOf), len(kinds))
	}
	seen := map[string]bool{}
	for _, v := range oneOf {
		variantRef, _ := v.(map[string]any)["$ref"].(string)
		name := variantRef[strings.LastIndex(variantRef, "/")+1:]
		variant, _ := defs[name].(map[string]any)
		if variant == nil {
			t.Fatalf("%s: variant ref %q does not resolve", tool, variantRef)
		}
		vp, _ := variant["properties"].(map[string]any)
		kind, _ := vp["kind"].(map[string]any)
		if c, ok := kind["const"].(string); ok {
			seen[c] = true
		}
	}
	for _, k := range kinds {
		if !seen[string(k)] {
			t.Errorf("%s: no oneOf variant pins kind %q", tool, k)
		}
	}
}

// TestSpecOutlineIsCarriedByTheOtherSpecTools pins the other half: the three
// tools that no longer embed the full schema still declare the SHAPE — meta,
// slides, and a kind from the registered enum — and still point at where the
// per-kind contract lives.
func TestSpecOutlineIsCarriedByTheOtherSpecTools(t *testing.T) {
	kinds := semantic.AllSlideKinds()
	for _, tool := range []struct {
		name string
		def  mcp.Tool
	}{
		{"render_deck_spec", mcpRenderDeckSpecTool()},
		{"compile_deck_spec", mcpCompileDeckSpecTool()},
		{"explain_deck_spec", mcpExplainDeckSpecTool()},
	} {
		spec := toolSpecProperty(t, tool.def.InputSchema.Properties)
		if !reflect.DeepEqual(spec["type"], []string{"object", "string"}) {
			t.Errorf("%s: spec.type = %v, want object|string", tool.name, spec["type"])
		}
		props, _ := spec["properties"].(map[string]any)
		slides, _ := props["slides"].(map[string]any)
		if slides == nil {
			t.Fatalf("%s: outline does not declare slides", tool.name)
		}
		items, _ := slides["items"].(map[string]any)
		itemProps, _ := items["properties"].(map[string]any)
		kind, _ := itemProps["kind"].(map[string]any)
		enum, _ := kind["enum"].([]any)
		if len(enum) != len(kinds) {
			t.Errorf("%s: kind enum has %d values, want %d", tool.name, len(enum), len(kinds))
		}
		if _, hasOneOf := items["oneOf"]; hasOneOf {
			t.Errorf("%s: still embeds the per-kind oneOf; it belongs on validate_deck_spec only", tool.name)
		}
		desc, _ := spec["description"].(string)
		for _, want := range []string{"list_slide_kinds", "validate_deck_spec", "SEMANTIC_UNKNOWN_FIELD"} {
			if !strings.Contains(desc, want) {
				t.Errorf("%s: spec description does not mention %q: %q", tool.name, want, desc)
			}
		}
	}
}

func toolSpecProperty(t *testing.T, props map[string]any) map[string]any {
	t.Helper()
	spec, ok := props["spec"].(map[string]any)
	if !ok {
		t.Fatalf("tool has no spec property: %v", props)
	}
	return spec
}

// Unknown fields in a kpi payload surface over MCP as SEMANTIC_UNKNOWN_FIELD
// naming the dropped path.
func TestSemanticMCP_UnknownKPIFieldDiagnostic(t *testing.T) {
	spec := map[string]any{
		"meta": map[string]any{"title": "Deck"},
		"slides": []any{map[string]any{
			"kind": "kpi_snapshot", "title": "KPIs", "takeaway": "Up.",
			"kpis": []any{
				map[string]any{"value": "$4M", "label": "ARR"},
				map[string]any{"value": "118%", "lable": "NRR"},
			},
		}},
	}
	res, err := testValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
	if err != nil || res.IsError {
		t.Fatalf("validate_deck_spec failed: %v %v", err, res)
	}
	var env diagnostics.FindingEnvelope
	structuredInto(t, res.StructuredContent, &env)
	if env.OK {
		t.Error("validate_deck_spec must not return ok=true when a field's content is dropped")
	}
	found := false
	for _, f := range env.Findings {
		if f.Evidence["path"] == "slides[0].kpis[1].lable" && strings.HasSuffix(f.Code, diagnostics.CodeSemanticUnknownField) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a finding at slides[0].kpis[1].lable, got %+v", env.Findings)
	}
}

// Every list_slide_kinds example must validate clean through validate_deck_spec.
func TestSemanticMCP_ListSlideKindsExamplesValidate(t *testing.T) {
	ctx := context.Background()
	res, err := handleListSlideKinds(ctx, makeRequest(map[string]any{"fields": []any{"item_schema"}}))
	if err != nil || res.IsError {
		t.Fatalf("list_slide_kinds failed: %v", err)
	}
	var out struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, res.StructuredContent, &out)
	if len(out.SlideKinds) != len(semantic.AllSlideKinds()) {
		t.Fatalf("got %d kinds, want %d", len(out.SlideKinds), len(semantic.AllSlideKinds()))
	}
	for _, k := range out.SlideKinds {
		if k.Example == nil || k.ItemSchema == nil {
			t.Errorf("%s: missing example or item_schema", k.Kind)
			continue
		}
		if k.ItemSchema["additionalProperties"] != false {
			t.Errorf("%s: item_schema must be closed", k.Kind)
		}
		spec := map[string]any{
			"meta":   map[string]any{"title": "Example deck"},
			"slides": []any{k.Example},
		}
		vres, err := testValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec, "strict": "strict"}))
		if err != nil || vres.IsError {
			t.Fatalf("%s: validate_deck_spec failed: %v", k.Kind, err)
		}
		var env diagnostics.FindingEnvelope
		structuredInto(t, vres.StructuredContent, &env)
		if !env.OK || len(env.Findings) != 0 {
			t.Errorf("%s: example does not validate clean: %+v", k.Kind, env.Findings)
		}
	}
}

func TestSemanticMCP_ListSlideKindsProjection(t *testing.T) {
	ctx := context.Background()
	compact, err := handleListSlideKinds(ctx, makeRequest(map[string]any{}))
	if err != nil || compact.IsError {
		t.Fatalf("compact list failed: %v, %+v", err, compact)
	}
	compactJSON, err := json.Marshal(compact.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(compactJSON), `"item_schema"`) || strings.Contains(string(compactJSON), `"compositions"`) {
		t.Fatal("default catalog includes opt-in detail fields")
	}
	var summary struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, compact.StructuredContent, &summary)
	if len(summary.SlideKinds) != len(semantic.AllSlideKinds()) {
		t.Fatalf("compact returned %d kinds, want %d", len(summary.SlideKinds), len(semantic.AllSlideKinds()))
	}
	for _, k := range summary.SlideKinds {
		if k.Kind == "" || k.Summary == "" || k.Example == nil {
			t.Fatalf("compact entry missing authoring context: %+v", k)
		}
	}

	full, err := handleListSlideKinds(ctx, makeRequest(map[string]any{
		"fields": []any{"item_schema", "compositions"},
	}))
	if err != nil || full.IsError {
		t.Fatalf("full list failed: %v, %+v", err, full)
	}
	fullJSON, err := json.Marshal(full.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if len(compactJSON)*2 >= len(fullJSON) {
		t.Fatalf("compact catalog too large: %d vs full %d bytes", len(compactJSON), len(fullJSON))
	}
	t.Logf("slide-kind structured payload: compact=%d bytes, full=%d bytes", len(compactJSON), len(fullJSON))

	selected, err := handleListSlideKinds(ctx, makeRequest(map[string]any{
		"kinds":  []any{"kpi_snapshot"},
		"fields": []any{"item_schema"},
	}))
	if err != nil || selected.IsError {
		t.Fatalf("filtered list failed: %v, %+v", err, selected)
	}
	var detail struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, selected.StructuredContent, &detail)
	if len(detail.SlideKinds) != 1 || detail.SlideKinds[0].Kind != "kpi_snapshot" || detail.SlideKinds[0].ItemSchema == nil || len(detail.SlideKinds[0].Compositions) != 0 {
		t.Fatalf("filtered projection is wrong: %+v", detail.SlideKinds)
	}
	compositionOnly, err := handleListSlideKinds(ctx, makeRequest(map[string]any{
		"kinds":  []any{"kpi_snapshot", "kpi_snapshot"},
		"fields": []any{"compositions"},
	}))
	if err != nil || compositionOnly.IsError {
		t.Fatalf("composition projection failed: %v, %+v", err, compositionOnly)
	}
	var compositions struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, compositionOnly.StructuredContent, &compositions)
	if len(compositions.SlideKinds) != 1 || compositions.SlideKinds[0].ItemSchema != nil || len(compositions.SlideKinds[0].Compositions) == 0 {
		t.Fatalf("composition projection is wrong: %+v", compositions.SlideKinds)
	}
	for _, args := range []map[string]any{
		{"kinds": []any{"not_a_kind"}},
		{"kinds": []any{42}},
		{"fields": []any{"bogus"}},
		{"fields": []any{42}},
		{"fields": "full"},
	} {
		result, err := handleListSlideKinds(ctx, makeRequest(args))
		if err != nil || !result.IsError {
			t.Fatalf("invalid list filter was accepted: args=%v err=%v result=%+v", args, err, result)
		}
		var findings diagnostics.FindingEnvelope
		structuredInto(t, result.StructuredContent, &findings)
		if len(findings.Findings) == 0 || findings.Findings[0].NextToolCall == nil || findings.Findings[0].NextToolCall.Tool != "list_slide_kinds" || len(findings.Findings[0].NextToolCall.ArgsTemplate) != 0 {
			t.Fatalf("invalid filter lacks a usable compact-catalog recovery call: %+v", findings.Findings)
		}
	}
}

// TestSpecOutlineToolsStillRejectUnknownFields is the guarantee the outline had
// to preserve (go-slide-creator-uhaq): dropping the per-kind schema from a
// tool's inputSchema does NOT loosen what it accepts. The check was never the
// input schema's — it is the compiler's — so every spec tool still reports an
// unknown payload field.
func TestSpecOutlineToolsStillRejectUnknownFields(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	spec := map[string]any{
		"meta": map[string]any{"title": "T", "template": "midnight-blue"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "T"},
			map[string]any{"kind": "kpi_snapshot", "title": "N",
				"kpis":           []any{map[string]any{"value": "1", "label": "a"}, map[string]any{"value": "2", "label": "b"}},
				"nonsense_field": "x"},
		},
	}
	// explain_deck_spec is not here on purpose: it is a projection of the plan,
	// not a validator, and never reported payload findings. render_deck_spec is
	// covered by TestSpecOutlineRenderRejectsUnknownFields, which needs a
	// template and so does not run in -short.
	for _, tc := range []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"validate_deck_spec", mc.handleValidateDeckSpec},
		{"compile_deck_spec", handleCompileDeckSpec},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.handler(context.Background(), makeRequest(map[string]any{"spec": spec}))
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !strings.Contains(resultText(res), "SEMANTIC_UNKNOWN_FIELD") {
				t.Errorf("%s did not report the unknown field:\n%s", tc.name, resultText(res))
			}
			var verdict struct {
				OK bool `json:"ok"`
			}
			structuredInto(t, res.StructuredContent, &verdict)
			if verdict.OK {
				t.Errorf("%s accepted a spec whose field content was dropped", tc.name)
			}
		})
	}
}

// TestSpecOutlineRenderRejectsUnknownFields covers the tool the outline change
// actually moved: render_deck_spec no longer embeds the per-kind schema, and
// still refuses to render a deck with an unknown payload field.
func TestSpecOutlineRenderRejectsUnknownFields(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	spec := map[string]any{
		"meta": map[string]any{"title": "T", "template": "midnight-blue"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "T"},
			map[string]any{"kind": "kpi_snapshot", "title": "N",
				"kpis":           []any{map[string]any{"value": "1", "label": "a"}, map[string]any{"value": "2", "label": "b"}},
				"nonsense_field": "x"},
		},
	}
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatalf("render_deck_spec: %v", err)
	}
	if !strings.Contains(resultText(res), "SEMANTIC_UNKNOWN_FIELD") {
		t.Errorf("render_deck_spec did not report the unknown field:\n%s", resultText(res))
	}
	var verdict renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &verdict)
	if verdict.Success || (verdict.Publishable != nil && *verdict.Publishable) {
		t.Errorf("render_deck_spec accepted dropped content: %+v", verdict)
	}
}

func TestRenderDeckSpecOutputSchemaSeparatesReadinessAndPublication(t *testing.T) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(outputSchemaRenderDeckSpec, &schema); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		"success", "deterministic_ready", "publishable",
		"manual_review_required", "blocking_reasons",
		"deterministic_blocking_reasons",
	} {
		if len(schema.Properties[field]) == 0 {
			t.Errorf("render_deck_spec output schema omits %q", field)
		}
	}
}

func TestRenderDeckSpecFreshArtifactNeedsVisualReview(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	spec := map[string]any{
		"meta": map[string]any{"title": "Publication status check", "template": "midnight-blue"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Publication status check"},
			map[string]any{"kind": "decision", "title": "Recommendation", "recommendation": "Proceed",
				"options": []any{
					map[string]any{"label": "Proceed", "detail": "Fund the next phase", "recommended": true},
					map[string]any{"label": "Wait", "detail": "Defer the decision"},
				},
			},
		},
	}
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatal(err)
	}
	var verdict renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &verdict)
	if !verdict.Success || verdict.DeterministicReady == nil || !*verdict.DeterministicReady ||
		verdict.Publishable == nil || *verdict.Publishable ||
		verdict.ManualReviewRequired == nil || !*verdict.ManualReviewRequired ||
		len(verdict.BlockingReasons) == 0 {
		t.Fatalf("fresh semantic render status = %+v", verdict)
	}
	if verdict.Quality == nil || verdict.Quality.Evidence == nil ||
		verdict.Quality.Evidence.VisuallyInspected || verdict.Quality.Evidence.Approved {
		t.Fatalf("fresh render claimed visual evidence: %+v", verdict.Quality)
	}
}
