package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// mcpErrorEnvelope is the legacy {Diagnostics, Summary} view of an MCP error
// result. The wire shape is now diagnostics.FindingEnvelope (see
// api/mcp_result.go); parseMCPError and structuredErrorEnvelope reconstruct this
// view from the envelope so the broad set of behavioral tests can keep asserting
// against un-namespaced codes, JSON paths, and the agent-recovery fields
// (expected_type, next_tool_call, example_value). Wire-shape contract tests
// assert the FindingEnvelope directly via parseMCPFindingEnvelope.
type mcpErrorEnvelope struct {
	Diagnostics []diagnostics.Diagnostic
	Summary     string
}

// parseMCPFindingEnvelope parses the FindingEnvelope wire shape from the text
// fallback of an IsError MCP result.
func parseMCPFindingEnvelope(t *testing.T, result *mcp.CallToolResult) diagnostics.FindingEnvelope {
	t.Helper()
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	text := result.Content[0].(mcp.TextContent).Text
	var fe diagnostics.FindingEnvelope
	if err := json.Unmarshal([]byte(text), &fe); err != nil {
		t.Fatalf("failed to parse finding envelope: %v\nraw: %s", err, text)
	}
	return fe
}

// parseMCPError extracts the reconstructed legacy diagnostics view from an
// IsError MCP result's text fallback.
func parseMCPError(t *testing.T, result *mcp.CallToolResult) mcpErrorEnvelope {
	t.Helper()
	return reconstructErrorEnvelope(parseMCPFindingEnvelope(t, result))
}

// structuredErrorEnvelope reconstructs the legacy diagnostics view from the
// StructuredContent (a diagnostics.FindingEnvelope) of an IsError MCP result.
func structuredErrorEnvelope(t *testing.T, result *mcp.CallToolResult) mcpErrorEnvelope {
	t.Helper()
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil, want non-nil")
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("failed to marshal StructuredContent: %v", err)
	}
	var fe diagnostics.FindingEnvelope
	if err := json.Unmarshal(b, &fe); err != nil {
		t.Fatalf("StructuredContent is not a FindingEnvelope: %v", err)
	}
	return reconstructErrorEnvelope(fe)
}

// reconstructErrorEnvelope adapts a FindingEnvelope back into the legacy
// {Diagnostics, Summary} view: codes are de-namespaced, the evidence map is
// surfaced as Details (with path / expected_type also lifted onto their dedicated
// fields), and the agent-recovery fields (next_tool_call, example_value) plus the
// primary remediation are carried across.
func reconstructErrorEnvelope(fe diagnostics.FindingEnvelope) mcpErrorEnvelope {
	env := mcpErrorEnvelope{Summary: fe.Summary}
	for _, f := range fe.Findings {
		d := diagnostics.Diagnostic{
			Code:         legacyFindingCode(f.Code),
			Message:      f.Message,
			Severity:     f.Severity,
			NextToolCall: f.NextToolCall,
			ExampleValue: f.ExampleValue,
		}
		if len(f.Evidence) > 0 {
			d.Details = f.Evidence
		}
		if p, ok := f.Evidence["path"].(string); ok {
			d.Path = p
		}
		if et, ok := f.Evidence["expected_type"].(string); ok {
			d.ExpectedType = et
		}
		if f.Remediation != nil && f.Remediation.Primary != nil {
			d.Fix = &diagnostics.Fix{
				Kind:   f.Remediation.Primary.Action,
				Params: f.Remediation.Primary.Params,
			}
		}
		env.Diagnostics = append(env.Diagnostics, d)
	}
	return env
}

// legacyDiagsFromWire parses the FindingEnvelope text fallback of an MCP error
// result and reconstructs the legacy map-shaped diagnostics that the asset / URL
// parity tests assert against: each finding becomes a map with a de-namespaced
// "code", its "path" lifted from evidence, the full evidence map under
// "details", plus "message" and "severity".
func legacyDiagsFromWire(t *testing.T, text string) []map[string]any {
	t.Helper()
	var fe diagnostics.FindingEnvelope
	if err := json.Unmarshal([]byte(text), &fe); err != nil {
		t.Fatalf("failed to parse finding envelope: %v\nraw: %s", err, text)
	}
	out := make([]map[string]any, 0, len(fe.Findings))
	for _, f := range fe.Findings {
		d := map[string]any{
			"code":     legacyFindingCode(f.Code),
			"message":  f.Message,
			"severity": string(f.Severity),
		}
		if p, ok := f.Evidence["path"].(string); ok {
			d["path"] = p
		}
		if len(f.Evidence) > 0 {
			d["details"] = map[string]any(f.Evidence)
		}
		out = append(out, d)
	}
	return out
}

// requireDiagCode asserts at least one diagnostic has the given code.
func requireDiagCode(t *testing.T, diags []diagnostics.Diagnostic, code string) diagnostics.Diagnostic {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			return d
		}
	}
	codes := make([]string, len(diags))
	for i, d := range diags {
		codes[i] = d.Code
	}
	t.Fatalf("expected diagnostic with code %q, got codes: %v", code, codes)
	return diagnostics.Diagnostic{} // unreachable
}

// --- Generate tool typed diagnostics ---

func TestHandleGenerate_NonObjectPresentation(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Passing a string where an object is expected — handler should reject.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": "not-an-object",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
}

func TestHandleGenerate_MissingRequired(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "REQUIRED")
	// Should have diagnostics for both template and slides.
	count := 0
	for _, d := range env.Diagnostics {
		if d.Code == "REQUIRED" {
			count++
		}
	}
	if count < 2 {
		t.Errorf("expected at least 2 REQUIRED diagnostics, got %d", count)
	}
}

func TestHandleGenerate_UnknownKey_DefaultWarning(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// "tmplate" is a typo for "template". By default, unknown keys are warnings
	// and generation proceeds.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(`{"template":"midnight-blue","tmplate":"typo","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected IsError=false (unknown keys are warnings by default)")
	}
	// The warning should appear in the output warnings.
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "tmplate") {
		t.Errorf("expected unknown key 'tmplate' in warnings, got: %s", text)
	}
}

func TestHandleGenerate_UnknownKey_StrictError(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// With strict_unknown_keys=true, unknown keys are errors.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation":        mustParseJSON(`{"template":"midnight-blue","tmplate":"typo","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`),
		"strict_unknown_keys": true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "unknown_key")
}

func TestHandleGenerate_UnknownEnum(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(`{"template":"midnight-blue","slides":[{"layout_id":"slideLayout2","transition":"BOGUS","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "UNKNOWN_ENUM")
}

func TestHandleGenerate_MissingParam(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
}

// --- Validate tool typed diagnostics ---

func TestHandleValidate_NonObjectPresentation(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": "not-an-object",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
}

func TestHandleValidate_StructuredDiagnostics(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Unknown key + missing template → IsError=true with diagnostics envelope
	// (same shape as generate_presentation errors).
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(`{"tmplate":"typo","slides":[]}`),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("validate with errors should return IsError=true")
	}

	// Parse the error envelope — same shape as generate_presentation.
	env := parseMCPError(t, result)
	if len(env.Diagnostics) == 0 {
		t.Fatal("expected non-empty diagnostics array")
	}
	if env.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

// --- ValidatePattern typed diagnostics ---

func TestHandleValidatePattern_MissingParam(t *testing.T) {
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
}

func TestHandleValidatePattern_UnknownPattern(t *testing.T) {
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":   "nonexistent-pattern",
		"values": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	d := requireDiagCode(t, env.Diagnostics, "UNKNOWN_PATTERN")
	if d.Path != "name" {
		t.Errorf("expected path=name, got %q", d.Path)
	}
}

func TestHandleValidatePattern_InvalidCellOverrideKey(t *testing.T) {
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name": "kpi-3up",
		"values": []any{
			map[string]any{"big": "A", "small": "a"},
			map[string]any{"big": "B", "small": "b"},
			map[string]any{"big": "C", "small": "c"},
		},
		"cell_overrides": map[string]any{"abc": map[string]any{}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	d := requireDiagCode(t, env.Diagnostics, "INVALID_KEY")
	if !strings.Contains(d.Path, "cell_overrides") {
		t.Errorf("expected path to contain cell_overrides, got %q", d.Path)
	}
}

// --- ExpandPattern typed diagnostics ---

func TestHandleExpandPattern_MissingParam(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
}

func TestHandleExpandPattern_UnknownPattern(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{
		"name":   "does-not-exist",
		"values": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	d := requireDiagCode(t, env.Diagnostics, "UNKNOWN_PATTERN")
	if d.Fix == nil {
		t.Error("expected fix suggestion for unknown pattern")
	}
}

// --- strictUnmarshalJSON unit tests ---

func TestStrictUnmarshalJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		var m map[string]any
		if err := strictUnmarshalJSON([]byte(`{"a": 1}`), &m); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("trailing data rejected", func(t *testing.T) {
		var m map[string]any
		err := strictUnmarshalJSON([]byte(`{"a": 1} {"b": 2}`), &m)
		if err == nil {
			t.Fatal("expected error for trailing data")
		}
		if !strings.Contains(err.Error(), "trailing") {
			t.Errorf("expected 'trailing' in error, got: %v", err)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		var m map[string]any
		err := strictUnmarshalJSON([]byte(`{not valid}`), &m)
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	t.Run("whitespace after value is fine", func(t *testing.T) {
		var m map[string]any
		if err := strictUnmarshalJSON([]byte(`{"a": 1}   `), &m); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
