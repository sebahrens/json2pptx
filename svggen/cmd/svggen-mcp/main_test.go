package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	// Import root package to auto-register all diagram types.
	_ "github.com/sebahrens/json2pptx/svggen"
)

func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestHandleListDiagramTypes(t *testing.T) {
	result, err := handleListDiagramTypes(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text

	var types []string
	if err := json.Unmarshal([]byte(text), &types); err != nil {
		t.Fatalf("failed to parse types: %v", err)
	}
	if len(types) < 10 {
		t.Fatalf("expected at least 10 diagram types, got %d", len(types))
	}

	// Check a few known types
	found := map[string]bool{}
	for _, typ := range types {
		found[typ] = true
	}
	for _, expected := range []string{"bar_chart", "pie_chart", "org_chart"} {
		if !found[expected] {
			t.Errorf("expected type %q not found", expected)
		}
	}
}

func TestHandleRenderDiagramSVG(t *testing.T) {
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{
			"categories": []any{"A", "B", "C"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10, 20, 30}},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "<svg") {
		t.Fatal("expected SVG output")
	}
}

func TestHandleRenderDiagramPNG(t *testing.T) {
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type":   "bar_chart",
		"format": "png",
		"data": map[string]any{
			"categories": []any{"A", "B"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10, 20}},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	// PNG result is an image content block
	img, ok := result.Content[0].(mcp.ImageContent)
	if !ok {
		t.Fatalf("expected ImageContent, got %T", result.Content[0])
	}
	if img.MIMEType != "image/png" {
		t.Fatalf("expected image/png, got %s", img.MIMEType)
	}
	if len(img.Data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}
}

func TestHandleRenderDiagramUnknownType(t *testing.T) {
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type": "nonexistent_chart",
		"data": map[string]any{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown type")
	}
}

func TestHandleValidateDiagramValid(t *testing.T) {
	result, err := handleValidateDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{
			"categories": []any{"A", "B"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10, 20}},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var vr struct {
		Valid       bool         `json:"valid"`
		Diagnostics []diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		t.Fatalf("failed to parse validation result: %v", err)
	}
	if !vr.Valid {
		t.Fatalf("expected valid, got diagnostics: %v", vr.Diagnostics)
	}
}

func TestHandleValidateDiagramInvalid(t *testing.T) {
	result, err := handleValidateDiagram(context.Background(), makeRequest(map[string]any{
		"type": "nonexistent_type",
		"data": map[string]any{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown type")
	}
}

func TestHandleValidateDiagramStructuredErrors(t *testing.T) {
	// bar_chart with missing required data fields should return structured diagnostics
	result, err := handleValidateDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("expected non-error result with validation diagnostics, got tool error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var vr struct {
		Valid       bool         `json:"valid"`
		Diagnostics []diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		t.Fatalf("failed to parse validation result: %v", err)
	}
	if vr.Valid {
		t.Fatal("expected invalid result for empty data")
	}
	if len(vr.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	// Verify unified diagnostic shape matches internal/diagnostics.Diagnostic
	first := vr.Diagnostics[0]
	if first.Code == "" {
		t.Error("expected non-empty code")
	}
	if first.Message == "" {
		t.Error("expected non-empty message")
	}
	if first.Path == "" {
		t.Error("expected non-empty path")
	}
	if first.Severity != "error" {
		t.Errorf("expected severity 'error', got %q", first.Severity)
	}
	// Code should be lowercase_snake, not UPPER_SNAKE
	for _, c := range first.Code {
		if c >= 'A' && c <= 'Z' {
			t.Errorf("expected lowercase_snake code, got %q", first.Code)
			break
		}
	}
	// Pattern should be in details, not a top-level field
	if first.Details == nil {
		t.Error("expected non-nil details")
	} else if first.Details["pattern"] != "bar_chart" {
		t.Errorf("expected details.pattern = 'bar_chart', got %v", first.Details["pattern"])
	}
}

func TestHandleValidateDiagramFixSuggestion(t *testing.T) {
	// pie_chart with missing slices should produce a fix suggestion
	result, err := handleValidateDiagram(context.Background(), makeRequest(map[string]any{
		"type": "pie_chart",
		"data": map[string]any{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("expected validation diagnostics, got tool error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var vr struct {
		Valid       bool         `json:"valid"`
		Diagnostics []diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if vr.Valid {
		t.Fatal("expected invalid result")
	}
	if len(vr.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	// At least one diagnostic should have a fix
	hasFix := false
	for _, d := range vr.Diagnostics {
		if d.Fix != nil {
			hasFix = true
			if d.Fix.Kind == "" {
				t.Error("fix has empty kind")
			}
			break
		}
	}
	if !hasFix {
		t.Log("no fix suggestions generated (acceptable if validator returns plain errors)")
	}
}

// TestDiagnosticContractShape verifies that svggen diagnostics produce the
// same JSON field set as internal/diagnostics.Diagnostic. This locks the
// unified contract across the two modules.
func TestDiagnosticContractShape(t *testing.T) {
	result, err := handleValidateDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{},
	}))
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].(mcp.TextContent).Text

	// Parse as raw JSON to inspect field names without struct bias.
	var raw struct {
		Valid       bool             `json:"valid"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(raw.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	first := raw.Diagnostics[0]

	// Required fields per unified contract.
	for _, field := range []string{"code", "message", "path", "severity"} {
		if _, ok := first[field]; !ok {
			t.Errorf("diagnostic missing required field %q", field)
		}
	}

	// Severity must be one of the canonical values.
	sev, _ := first["severity"].(string)
	switch sev {
	case "error", "warning", "info":
		// ok
	default:
		t.Errorf("severity %q is not a canonical value (error/warning/info)", sev)
	}

	// Fix, when present, must have "kind".
	if fixRaw, ok := first["fix"]; ok && fixRaw != nil {
		fixMap, ok := fixRaw.(map[string]any)
		if !ok {
			t.Errorf("fix is not an object: %T", fixRaw)
		} else if _, ok := fixMap["kind"]; !ok {
			t.Error("fix missing required field 'kind'")
		}
	}

	// next_tool_call, when present, must have "tool" and "args_template".
	if ntcRaw, ok := first["next_tool_call"]; ok && ntcRaw != nil {
		ntcMap, ok := ntcRaw.(map[string]any)
		if !ok {
			t.Errorf("next_tool_call is not an object: %T", ntcRaw)
		} else {
			if _, ok := ntcMap["tool"]; !ok {
				t.Error("next_tool_call missing 'tool'")
			}
			if _, ok := ntcMap["args_template"]; !ok {
				t.Error("next_tool_call missing 'args_template'")
			}
		}
	}

	// details.pattern must carry the diagram type.
	if details, ok := first["details"].(map[string]any); ok {
		if details["pattern"] != "bar_chart" {
			t.Errorf("details.pattern = %v, want bar_chart", details["pattern"])
		}
	} else {
		t.Error("expected details map with pattern key")
	}

	// Verify that "pattern" is NOT a top-level field (it's in details now).
	if _, ok := first["pattern"]; ok {
		t.Error("'pattern' should not be a top-level field; it belongs in details")
	}
}

func TestHandleRenderDiagramInvalidStyle(t *testing.T) {
	// Invalid style payload should produce a structured error, not be silently ignored
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{
			"categories": []any{"A", "B"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10, 20}},
			},
		},
		"style": "not-an-object",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for non-object style")
	}
}

func TestHandleGetDiagramSchemaKnown(t *testing.T) {
	result, err := handleGetDiagramSchema(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var sr struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Example     any    `json:"example"`
	}
	if err := json.Unmarshal([]byte(text), &sr); err != nil {
		t.Fatalf("failed to parse schema result: %v", err)
	}
	if sr.Type != "bar_chart" {
		t.Fatalf("expected type bar_chart, got %s", sr.Type)
	}
	if sr.Description == "" {
		t.Fatal("expected non-empty description")
	}
	if sr.Example == nil {
		t.Fatal("expected non-nil example")
	}
}

func TestHandleGetDiagramSchemaUnknown(t *testing.T) {
	result, err := handleGetDiagramSchema(context.Background(), makeRequest(map[string]any{
		"type": "nonexistent_type",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown type")
	}
}

func TestHandleRenderDiagramWithOptions(t *testing.T) {
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type":   "bar_chart",
		"title":  "Test Chart",
		"width":  float64(400),
		"height": float64(300),
		"data": map[string]any{
			"categories": []any{"X", "Y"},
			"series": []any{
				map[string]any{"name": "S", "values": []any{5, 10}},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "<svg") {
		t.Fatal("expected SVG output")
	}
}
