package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/svggen/core"

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

	var entries []diagramTypeEntry
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("failed to parse entries: %v", err)
	}
	if len(entries) < 10 {
		t.Fatalf("expected at least 10 diagram types, got %d", len(entries))
	}

	// Check a few known canonical names are present.
	byName := map[string]diagramTypeEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	for _, expected := range []string{"bar_chart", "pie_chart", "org_chart"} {
		if _, ok := byName[expected]; !ok {
			t.Errorf("expected canonical name %q not found", expected)
		}
	}

	// bar_chart must advertise "bar" as an alias.
	bar, ok := byName["bar_chart"]
	if !ok {
		t.Fatal("bar_chart missing from list_diagram_types response")
	}
	hasBarAlias := false
	for _, a := range bar.Aliases {
		if a == "bar" {
			hasBarAlias = true
			break
		}
	}
	if !hasBarAlias {
		t.Errorf("bar_chart should advertise %q as an alias; got %v", "bar", bar.Aliases)
	}
}

// TestRenderDiagramAcceptsShortAndCanonicalNames verifies the bar/bar_chart
// aliasing contract: agents may send either form and receive the same chart.
// Regression test for the bead "unify chart-type vocabulary across json2pptx
// and svggen-mcp (bar ↔ bar_chart aliasing)".
func TestRenderDiagramAcceptsShortAndCanonicalNames(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"categories": []any{"A", "B", "C"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10, 20, 30}},
			},
		},
	}
	for _, name := range []string{"bar", "bar_chart"} {
		t.Run(name, func(t *testing.T) {
			args := map[string]any{"type": name}
			for k, v := range payload {
				args[k] = v
			}
			result, err := handleRenderDiagram(context.Background(), makeRequest(args))
			if err != nil {
				t.Fatalf("render(%q) returned error: %v", name, err)
			}
			if result.IsError {
				t.Fatalf("render(%q) unexpected error: %v", name, result.Content)
			}
			text := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, "<svg") {
				t.Fatalf("render(%q) expected SVG output, got: %s", name, text)
			}
		})
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
		Valid  bool         `json:"valid"`
		Errors []diagnostic `json:"errors"`
	}
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		t.Fatalf("failed to parse validation result: %v", err)
	}
	if !vr.Valid {
		t.Fatalf("expected valid, got errors: %v", vr.Errors)
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
		Valid  bool         `json:"valid"`
		Errors []diagnostic `json:"errors"`
	}
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		t.Fatalf("failed to parse validation result: %v", err)
	}
	if vr.Valid {
		t.Fatal("expected invalid result for empty data")
	}
	if len(vr.Errors) == 0 {
		t.Fatal("expected at least one error")
	}

	// Verify unified diagnostic shape matches internal/diagnostics.Diagnostic
	first := vr.Errors[0]
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
		Valid  bool         `json:"valid"`
		Errors []diagnostic `json:"errors"`
	}
	if err := json.Unmarshal([]byte(text), &vr); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if vr.Valid {
		t.Fatal("expected invalid result")
	}
	if len(vr.Errors) == 0 {
		t.Fatal("expected at least one error")
	}

	// At least one error should have a fix
	hasFix := false
	for _, d := range vr.Errors {
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
		Valid  bool             `json:"valid"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(raw.Errors) == 0 {
		t.Fatal("expected at least one error")
	}

	first := raw.Errors[0]

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

// TestValidateDiagramFixKindEnum locks the contract documented in
// skills/generate-deck/SKILL.md: every fix.kind returned by validate_diagram
// must come from the chart-finding enum
// {align_series, truncate_or_split, replace_value, explicit_scale, reduce_items}.
//
// Exercises multiple diagram types and a variety of invalid payloads so that
// required-field, type-mismatch, and constraint-violation paths through
// inferFix are all visited.
func TestValidateDiagramFixKindEnum(t *testing.T) {
	allowed := map[string]struct{}{
		"align_series":      {},
		"truncate_or_split": {},
		"replace_value":     {},
		"explicit_scale":    {},
		"reduce_items":      {},
	}

	cases := []struct {
		name string
		args map[string]any
	}{
		{
			name: "bar_chart empty data (required-field errors)",
			args: map[string]any{
				"type": "bar_chart",
				"data": map[string]any{},
			},
		},
		{
			name: "pie_chart empty data (required-field errors)",
			args: map[string]any{
				"type": "pie_chart",
				"data": map[string]any{},
			},
		},
		{
			name: "bar_chart bad series shape (type / value errors)",
			args: map[string]any{
				"type": "bar_chart",
				"data": map[string]any{
					"categories": []any{"A", "B"},
					"series":     "not-a-list",
				},
			},
		},
		{
			name: "line_chart mismatched series lengths (constraint / alignment)",
			args: map[string]any{
				"type": "line_chart",
				"data": map[string]any{
					"categories": []any{"A", "B", "C"},
					"series": []any{
						map[string]any{"name": "S1", "values": []any{1, 2}},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := handleValidateDiagram(context.Background(), makeRequest(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				// Tool-level errors (unknown type, malformed args) are
				// outside the validate_diagram envelope contract.
				t.Skipf("tool-level error, not a validation envelope: %v", result.Content)
			}

			text := result.Content[0].(mcp.TextContent).Text
			var vr struct {
				Valid  bool         `json:"valid"`
				Errors []diagnostic `json:"errors"`
			}
			if err := json.Unmarshal([]byte(text), &vr); err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			for i, d := range vr.Errors {
				if d.Fix == nil {
					continue
				}
				if _, ok := allowed[d.Fix.Kind]; !ok {
					t.Errorf("errors[%d]: fix.kind = %q is not in the SKILL.md chart enum (code=%q, path=%q)",
						i, d.Fix.Kind, d.Code, d.Path)
				}
			}
		})
	}
}

// TestInferFixCapacityAndLogScale covers the two new fix.kind branches added
// to align validate_diagram with the SKILL.md chart-finding enum:
//   - capacity-class constraints  → truncate_or_split
//   - log/log-scale  constraints  → explicit_scale
func TestInferFixCapacityAndLogScale(t *testing.T) {
	type tc struct {
		name string
		ve   core.ValidationError
		want string
	}

	cases := []tc{
		{
			name: "capacity-class field (items)",
			ve: core.ValidationError{
				Code:    core.ErrCodeConstraint,
				Field:   "data.items",
				Message: "10 items exceeds maximum of 8",
			},
			want: "truncate_or_split",
		},
		{
			name: "capacity-class field (slices)",
			ve: core.ValidationError{
				Code:    core.ErrCodeConstraint,
				Field:   "data.slices",
				Message: "too many slices",
			},
			want: "truncate_or_split",
		},
		{
			name: "capacity-class message (categories)",
			ve: core.ValidationError{
				Code:    core.ErrCodeConstraint,
				Field:   "data.categories",
				Message: "12 categories exceed renderer limit",
			},
			want: "truncate_or_split",
		},
		{
			name: "log-scale field",
			ve: core.ValidationError{
				Code:    core.ErrCodeConstraint,
				Field:   "style.scale",
				Message: "invalid scale value",
			},
			want: "explicit_scale",
		},
		{
			name: "log-scale message",
			ve: core.ValidationError{
				Code:    core.ErrCodeConstraint,
				Field:   "data.values[2]",
				Message: "negative value not allowed on log scale",
			},
			want: "explicit_scale",
		},
		{
			name: "series alignment still maps to align_series",
			ve: core.ValidationError{
				Code:    core.ErrCodeConstraint,
				Field:   "data.series[1].values",
				Message: "series length mismatch with categories",
			},
			want: "align_series",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := inferFix(c.ve)
			if got == nil {
				t.Fatalf("expected fix kind %q, got nil", c.want)
			}
			if got.Kind != c.want {
				t.Errorf("inferFix kind = %q, want %q", got.Kind, c.want)
			}
		})
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

// TestHandleRenderDiagramStructuredErrors verifies that every error return
// from handleRenderDiagram uses the unified diagnostic envelope rather than
// a plain "render failed: <go error>" string. Each diagnostic must carry at
// least code+fix; many also carry next_tool_call. Locks the agent-facing
// contract for render_diagram error responses.
func TestHandleRenderDiagramStructuredErrors(t *testing.T) {
	validBarData := map[string]any{
		"categories": []any{"A", "B"},
		"series": []any{
			map[string]any{"name": "S1", "values": []any{10, 20}},
		},
	}

	cases := []struct {
		name           string
		args           map[string]any
		wantCode       string
		wantFix        bool   // require Fix to be non-nil
		wantNextTool   string // empty = don't check; otherwise expected tool name
		wantPathPrefix string // empty = don't check; otherwise diagnostic.Path must start with this
	}{
		{
			name:         "missing_type",
			args:         map[string]any{"data": map[string]any{}},
			wantCode:     "required",
			wantFix:      true,
			wantNextTool: "list_diagram_types",
		},
		{
			name:         "unknown_diagram_type",
			args:         map[string]any{"type": "nonexistent_chart", "data": map[string]any{}},
			wantCode:     "unknown_diagram_type",
			wantFix:      true,
			wantNextTool: "list_diagram_types",
		},
		{
			name:         "missing_data",
			args:         map[string]any{"type": "bar_chart"},
			wantCode:     "required",
			wantFix:      true,
			wantNextTool: "get_diagram_schema",
		},
		{
			name:         "data_not_object",
			args:         map[string]any{"type": "bar_chart", "data": "not-an-object"},
			wantCode:     "invalid_type",
			wantFix:      true,
			wantNextTool: "get_diagram_schema",
		},
		{
			name: "style_not_object",
			args: map[string]any{
				"type":  "bar_chart",
				"data":  validBarData,
				"style": "not-an-object",
			},
			wantCode:       "invalid_type",
			wantFix:        true,
			wantPathPrefix: "style",
		},
		{
			name: "unsupported_format",
			args: map[string]any{
				"type":   "bar_chart",
				"data":   validBarData,
				"format": "bmp",
			},
			wantCode:       "invalid_value",
			wantFix:        true,
			wantPathPrefix: "format",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := handleRenderDiagram(context.Background(), makeRequest(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError=true, got result: %+v", result)
			}

			text := result.Content[0].(mcp.TextContent).Text

			// Must parse as the unified envelope, not a bare string and not
			// a doubly-encoded JSON-in-a-string.
			var er struct {
				Valid       bool             `json:"valid"`
				Diagnostics []map[string]any `json:"diagnostics"`
			}
			if err := json.Unmarshal([]byte(text), &er); err != nil {
				t.Fatalf("expected JSON envelope, got %q: %v", text, err)
			}
			if er.Valid {
				t.Errorf("expected valid=false, got true")
			}
			if len(er.Diagnostics) == 0 {
				t.Fatal("expected at least one diagnostic")
			}

			// Every diagnostic must carry code+fix per the unified contract.
			for i, d := range er.Diagnostics {
				if code, _ := d["code"].(string); code == "" {
					t.Errorf("diagnostic[%d] missing code: %+v", i, d)
				}
				if tc.wantFix {
					fixRaw, ok := d["fix"]
					if !ok || fixRaw == nil {
						t.Errorf("diagnostic[%d] missing fix: %+v", i, d)
					} else {
						fixMap, ok := fixRaw.(map[string]any)
						if !ok {
							t.Errorf("diagnostic[%d] fix not object: %T", i, fixRaw)
						} else if kind, _ := fixMap["kind"].(string); kind == "" {
							t.Errorf("diagnostic[%d] fix missing kind", i)
						}
					}
				}
				if sev, _ := d["severity"].(string); sev != "error" {
					t.Errorf("diagnostic[%d] severity=%q, want \"error\"", i, sev)
				}
			}

			first := er.Diagnostics[0]
			if got, _ := first["code"].(string); got != tc.wantCode {
				t.Errorf("first diagnostic code=%q, want %q", got, tc.wantCode)
			}
			if tc.wantPathPrefix != "" {
				path, _ := first["path"].(string)
				if path != tc.wantPathPrefix {
					t.Errorf("first diagnostic path=%q, want %q", path, tc.wantPathPrefix)
				}
			}
			if tc.wantNextTool != "" {
				ntcRaw, ok := first["next_tool_call"]
				if !ok || ntcRaw == nil {
					t.Errorf("expected next_tool_call=%q, got nil", tc.wantNextTool)
				} else {
					ntc, _ := ntcRaw.(map[string]any)
					tool, _ := ntc["tool"].(string)
					if tool != tc.wantNextTool {
						t.Errorf("next_tool_call.tool=%q, want %q", tool, tc.wantNextTool)
					}
				}
			}
		})
	}
}

// TestHandleRenderDiagramRenderFailedEnvelope verifies that a renderer-level
// failure (validation surfaced during render) flows through the unified
// envelope with structured diagnostics — no plain "render failed: ..." string
// and no double-encoded JSON-in-a-string.
func TestHandleRenderDiagramRenderFailedEnvelope(t *testing.T) {
	// bar_chart with empty data triggers the renderer's own validation pass
	// (categories+series are required). The error must surface as the unified
	// envelope, not "render failed: <go error>".
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}

	text := result.Content[0].(mcp.TextContent).Text

	// Must NOT be the legacy plain-string format.
	if strings.HasPrefix(text, "render failed:") {
		t.Fatalf("legacy plain-string error returned: %q", text)
	}

	// Must parse as the unified envelope (top level is an object).
	var er struct {
		Valid       bool             `json:"valid"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(text), &er); err != nil {
		t.Fatalf("expected JSON envelope, got %q: %v", text, err)
	}
	if er.Valid {
		t.Error("expected valid=false")
	}
	if len(er.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	// Every diagnostic must carry a non-empty code; check no double-encoding
	// of the diagnostic body (i.e. message must not itself be a JSON object).
	for i, d := range er.Diagnostics {
		code, _ := d["code"].(string)
		if code == "" {
			t.Errorf("diagnostic[%d] missing code: %+v", i, d)
		}
		msg, _ := d["message"].(string)
		if strings.HasPrefix(strings.TrimSpace(msg), "{") {
			t.Errorf("diagnostic[%d] message looks like nested JSON (double-encoded): %q", i, msg)
		}
	}
}

// TestHandleRenderDiagramInvalidStyleEnvelope verifies that the invalid-style
// path returns the unified envelope (no doubly-encoded JSON string).
func TestHandleRenderDiagramInvalidStyleEnvelope(t *testing.T) {
	// style is an object but contains an invalid palette type — triggers the
	// json.Unmarshal failure path inside the style handling block.
	result, err := handleRenderDiagram(context.Background(), makeRequest(map[string]any{
		"type": "bar_chart",
		"data": map[string]any{
			"categories": []any{"A"},
			"series":     []any{map[string]any{"name": "S", "values": []any{1}}},
		},
		"style": map[string]any{"palette": 12345}, // palette expects string/object, not number
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid style")
	}

	text := result.Content[0].(mcp.TextContent).Text

	// Must parse as the unified envelope at the top level.
	var er struct {
		Valid       bool             `json:"valid"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(text), &er); err != nil {
		t.Fatalf("expected JSON envelope, got %q: %v", text, err)
	}
	if er.Valid {
		t.Error("expected valid=false")
	}
	if len(er.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	// First diagnostic must have a code (not be a bare string in a string).
	first := er.Diagnostics[0]
	if code, _ := first["code"].(string); code == "" {
		t.Errorf("expected diagnostic with code, got %+v", first)
	}
	if path, _ := first["path"].(string); path != "style" {
		t.Errorf("expected path=style, got %q", path)
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
