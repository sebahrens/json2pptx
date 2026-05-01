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

// mcpErrorEnvelope mirrors the internal type in api/mcp_result.go.
type mcpErrorEnvelope struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	Summary     string                   `json:"summary"`
}

// parseMCPError extracts diagnostics from an IsError MCP result.
func parseMCPError(t *testing.T, result *mcp.CallToolResult) mcpErrorEnvelope {
	t.Helper()
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	text := result.Content[0].(mcp.TextContent).Text
	var env mcpErrorEnvelope
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("failed to parse error envelope: %v\nraw: %s", err, text)
	}
	return env
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

func TestHandleGenerate_MalformedJSON(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"json_input": `{"template":"x" GARBAGE`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
}

func TestHandleGenerate_TrailingJSON(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Valid JSON followed by extra data — should be rejected.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"json_input": `{"template":"midnight-blue","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]} {"extra": true}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	d := requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
	if !strings.Contains(d.Message, "trailing") {
		t.Errorf("expected message to mention trailing data, got: %s", d.Message)
	}
}

func TestHandleGenerate_MissingRequired(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"json_input": `{}`,
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
		"json_input": `{"template":"midnight-blue","tmplate":"typo","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`,
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
		"json_input":          `{"template":"midnight-blue","tmplate":"typo","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`,
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
		"json_input": `{"template":"midnight-blue","slides":[{"layout_id":"slideLayout2","transition":"BOGUS","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "unknown_enum")
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

func TestHandleValidate_MalformedJSON(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"json_input": `{not valid json`,
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

	// Unknown key + missing template → structured diagnostics in output.
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"json_input": `{"tmplate":"typo","slides":[]}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("validate should not be IsError — it returns structured output")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp dryRunOutput
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	if len(resp.Diagnostics) == 0 {
		t.Fatal("expected non-empty diagnostics array")
	}
	// Should also have backfilled string errors.
	if len(resp.Errors) == 0 {
		t.Error("expected backfilled errors for backward compat")
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
		"values": `{}`,
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

func TestHandleValidatePattern_InvalidValuesJSON(t *testing.T) {
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":   "kpi-3up",
		"values": `{not valid}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	d := requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
	if d.Path != "values" {
		t.Errorf("expected path=values, got %q", d.Path)
	}
}

func TestHandleValidatePattern_InvalidCellOverrideKey(t *testing.T) {
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":           "kpi-3up",
		"values":         `[{"big":"A","small":"a"},{"big":"B","small":"b"},{"big":"C","small":"c"}]`,
		"cell_overrides": `{"abc": {}}`,
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
		"values": `{}`,
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
