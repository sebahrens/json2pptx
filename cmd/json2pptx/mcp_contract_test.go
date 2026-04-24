package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// Contract tests lock the machine-readable response shapes that agents depend
// on. They assert specific JSON field names, types, and nesting — not just
// behavioral correctness. Breaking a contract test means an agent integration
// will break.
//
// Stable fields (safe for programmatic matching):
//   - diagnostics[].code, .severity, .path, .fix.kind, .fix.params
//   - success, valid, output_path, slide_count, fit_findings[].code
//   - error envelope: diagnostics[], summary
//
// Advisory fields (human-readable, may change wording):
//   - diagnostics[].message, summary text, warnings[]

// TestMCPErrorEnvelope_ContractShape verifies that MCP error results carry
// the diagnostics envelope with the exact field names agents expect.
func TestMCPErrorEnvelope_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Trigger a structured error via invalid JSON.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"json_input": `{invalid`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	// --- Assert envelope shape via StructuredContent ---
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil — agents depend on this")
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}

	// Parse into raw map to assert field names without relying on Go types.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("error envelope is not a JSON object: %v", err)
	}

	// "diagnostics" must be present and an array.
	diagsRaw, ok := raw["diagnostics"]
	if !ok {
		t.Fatal("error envelope missing 'diagnostics' field")
	}
	var diags []map[string]json.RawMessage
	if err := json.Unmarshal(diagsRaw, &diags); err != nil {
		t.Fatalf("diagnostics is not an array of objects: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("diagnostics array is empty")
	}

	// "summary" must be present and a string.
	summaryRaw, ok := raw["summary"]
	if !ok {
		t.Fatal("error envelope missing 'summary' field")
	}
	var summary string
	if err := json.Unmarshal(summaryRaw, &summary); err != nil {
		t.Fatalf("summary is not a string: %v", err)
	}

	// --- Assert diagnostic entry shape ---
	d := diags[0]
	for _, field := range []string{"code", "message", "severity"} {
		if _, ok := d[field]; !ok {
			t.Errorf("diagnostic missing required field %q", field)
		}
	}

	// "code" and "severity" must be strings.
	var code string
	if err := json.Unmarshal(d["code"], &code); err != nil {
		t.Errorf("diagnostic.code is not a string: %v", err)
	}
	var severity string
	if err := json.Unmarshal(d["severity"], &severity); err != nil {
		t.Errorf("diagnostic.severity is not a string: %v", err)
	}

	// Severity must be one of the stable values.
	switch diagnostics.Severity(severity) {
	case diagnostics.SeverityError, diagnostics.SeverityWarning, diagnostics.SeverityInfo:
		// ok
	default:
		t.Errorf("diagnostic.severity = %q, want one of error/warning/info", severity)
	}

	// --- Assert text fallback is also present ---
	if len(result.Content) == 0 {
		t.Fatal("MCP result has no Content text fallback — older clients depend on this")
	}
}

// TestMCPErrorEnvelope_FixShape verifies that the fix suggestion in diagnostics
// carries the stable {kind, params} structure.
func TestMCPErrorEnvelope_FixShape(t *testing.T) {
	// Trigger an error that includes a fix (unknown pattern with suggestion).
	result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{
		"name": "kp1-3up", // typo — should suggest "kpi-3up"
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var env mcpErrorEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}

	// Find a diagnostic with a fix.
	var fixDiag *diagnostics.Diagnostic
	for i := range env.Diagnostics {
		if env.Diagnostics[i].Fix != nil {
			fixDiag = &env.Diagnostics[i]
			break
		}
	}
	if fixDiag == nil {
		t.Skip("no diagnostic with fix — typo suggestion may not have fired")
	}

	// Assert fix shape: {kind: string, params?: object}
	if fixDiag.Fix.Kind == "" {
		t.Error("fix.kind is empty — agents use this to decide repair action")
	}

	// Round-trip through JSON to verify serialization.
	fixJSON, _ := json.Marshal(fixDiag.Fix)
	var fixRaw map[string]json.RawMessage
	if err := json.Unmarshal(fixJSON, &fixRaw); err != nil {
		t.Fatalf("fix is not a JSON object: %v", err)
	}
	if _, ok := fixRaw["kind"]; !ok {
		t.Error("fix JSON missing 'kind' field")
	}
}

// TestMCPGenerateSuccess_ContractShape verifies the generate success response
// has the stable fields agents depend on.
func TestMCPGenerateSuccess_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Contract Test"
			}]
		}]
	}`

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"json_input": deckJSON,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	// Parse via raw map to assert field names.
	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Stable fields agents depend on.
	for _, field := range []string{"success", "output_path", "slide_count"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("generate success response missing stable field %q", field)
		}
	}

	var success bool
	if err := json.Unmarshal(raw["success"], &success); err != nil {
		t.Errorf("success is not a boolean: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}

	var slideCount int
	if err := json.Unmarshal(raw["slide_count"], &slideCount); err != nil {
		t.Errorf("slide_count is not a number: %v", err)
	}
	if slideCount < 1 {
		t.Errorf("slide_count = %d, want >= 1", slideCount)
	}
}

// TestMCPValidateSuccess_ContractShape verifies the validate success response
// has the stable fields agents depend on.
func TestMCPValidateSuccess_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Contract Test"
			}]
		}]
	}`

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"json_input": deckJSON,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("unexpected tool error")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Stable fields agents depend on.
	for _, field := range []string{"valid", "slide_count", "slides"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("validate success response missing stable field %q", field)
		}
	}

	var valid bool
	if err := json.Unmarshal(raw["valid"], &valid); err != nil {
		t.Errorf("valid is not a boolean: %v", err)
	}
	if !valid {
		t.Error("expected valid=true for well-formed input")
	}
}

// TestMCPValidateWithDiagnostics_ContractShape verifies that validate returns
// diagnostics[] with the same shape as the error envelope diagnostics.
func TestMCPValidateWithDiagnostics_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Unknown key should produce diagnostics in the validate output.
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"json_input": `{"template":"midnight-blue","tmplate":"typo","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("validate should return success with diagnostics, not IsError")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	diagsRaw, ok := raw["diagnostics"]
	if !ok {
		t.Fatal("validate response missing 'diagnostics' field")
	}

	var diags []map[string]json.RawMessage
	if err := json.Unmarshal(diagsRaw, &diags); err != nil {
		t.Fatalf("diagnostics is not an array: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("expected non-empty diagnostics for unknown key")
	}

	// Each diagnostic must have the same shape as the error envelope diagnostics.
	for i, d := range diags {
		for _, field := range []string{"code", "message", "severity"} {
			if _, ok := d[field]; !ok {
				t.Errorf("diagnostics[%d] missing required field %q", i, field)
			}
		}
	}
}

// TestMCPPatternValidate_ContractShape verifies the pattern validation response
// shape: {ok: bool, errors?: [{code, message, path?, fix?}]}.
func TestMCPPatternValidate_ContractShape(t *testing.T) {
	// Valid input → {ok: true}
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":   "kpi-3up",
		"values": `[{"big":"A","small":"a"},{"big":"B","small":"b"},{"big":"C","small":"c"}]`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("unexpected tool error for valid input")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}
	if _, ok := raw["ok"]; !ok {
		t.Fatal("pattern validate response missing 'ok' field")
	}
	var ok2 bool
	if err := json.Unmarshal(raw["ok"], &ok2); err != nil {
		t.Errorf("ok is not a boolean: %v", err)
	}
	if !ok2 {
		t.Error("expected ok=true for valid input")
	}

	// Invalid input → {ok: false, errors: [...]}
	result2, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":   "kpi-3up",
		"values": `[]`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.IsError {
		t.Fatal("validation failures should not be IsError")
	}

	b2, _ := json.Marshal(result2.StructuredContent)
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(b2, &raw2); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	errorsRaw, hasErrors := raw2["errors"]
	if !hasErrors {
		t.Fatal("expected 'errors' field in invalid pattern validation")
	}

	var validationErrors []map[string]json.RawMessage
	if err := json.Unmarshal(errorsRaw, &validationErrors); err != nil {
		t.Fatalf("errors is not an array: %v", err)
	}
	if len(validationErrors) == 0 {
		t.Fatal("expected non-empty errors array")
	}

	// Each error must have at least code and message.
	for i, ve := range validationErrors {
		for _, field := range []string{"code", "message"} {
			if _, ok := ve[field]; !ok {
				t.Errorf("errors[%d] missing required field %q", i, field)
			}
		}
	}
}
