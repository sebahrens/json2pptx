package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func TestMCPSuccessResult_StructuredContent(t *testing.T) {
	type payload struct {
		Success bool   `json:"success"`
		Path    string `json:"path"`
	}
	data := payload{Success: true, Path: "/tmp/out.pptx"}

	result, err := MCPSuccessResult(context.Background(), data)
	if err != nil {
		t.Fatalf("MCPSuccessResult error: %v", err)
	}

	// StructuredContent should be the data itself.
	sc, ok := result.StructuredContent.(payload)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want payload", result.StructuredContent)
	}
	if !sc.Success || sc.Path != "/tmp/out.pptx" {
		t.Errorf("StructuredContent = %+v, want {true /tmp/out.pptx}", sc)
	}

	// Should not be an error.
	if result.IsError {
		t.Error("IsError = true, want false")
	}

	// Text fallback should be present.
	if len(result.Content) == 0 {
		t.Fatal("Content is empty, want text fallback")
	}

	// Text should be valid JSON containing the data.
	textContent := result.Content[0]
	tc, ok := textContent.(interface{ GetText() string })
	if !ok {
		// Try raw field access via JSON round-trip.
		b, _ := json.Marshal(textContent)
		if !strings.Contains(string(b), "/tmp/out.pptx") {
			t.Errorf("text fallback does not contain expected data: %s", b)
		}
	} else if !strings.Contains(tc.GetText(), "/tmp/out.pptx") {
		t.Errorf("text fallback = %q, want to contain /tmp/out.pptx", tc.GetText())
	}
}

func TestMCPSuccessResult_IndentedByDefault(t *testing.T) {
	data := map[string]string{"key": "value"}

	result, err := MCPSuccessResult(context.Background(), data)
	if err != nil {
		t.Fatalf("MCPSuccessResult error: %v", err)
	}

	// Default (non-compact) should produce indented JSON.
	b, _ := json.Marshal(result.Content[0])
	text := string(b)
	if !strings.Contains(text, "\\n") {
		t.Errorf("expected indented JSON in fallback text, got: %s", text)
	}
}

func TestMCPDiagnosticsError(t *testing.T) {
	ds := []diagnostics.Diagnostic{
		{
			Code:     "required",
			Message:  "template is required",
			Severity: diagnostics.SeverityError,
		},
		{
			Code:     "min_items",
			Message:  "at least one slide is required",
			Path:     "slides",
			Severity: diagnostics.SeverityError,
		},
	}

	result := MCPDiagnosticsError(ds)

	// Must be an error result.
	if !result.IsError {
		t.Error("IsError = false, want true")
	}

	// StructuredContent should be the shared FindingEnvelope.
	envelope, ok := result.StructuredContent.(diagnostics.FindingEnvelope)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want diagnostics.FindingEnvelope", result.StructuredContent)
	}

	if envelope.SchemaVersion != diagnostics.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, diagnostics.SchemaVersion)
	}
	if envelope.Tool != diagnostics.DefaultTool {
		t.Errorf("Tool = %q, want %q", envelope.Tool, diagnostics.DefaultTool)
	}
	if envelope.Subcommand == "" {
		t.Error("Subcommand is empty, want a generic surface identifier")
	}
	if envelope.OK {
		t.Error("OK = true, want false for error-severity findings")
	}
	if len(envelope.Findings) != 2 {
		t.Errorf("Findings count = %d, want 2", len(envelope.Findings))
	}
	if envelope.Summary != "2 errors" {
		t.Errorf("Summary = %q, want %q", envelope.Summary, "2 errors")
	}

	// Codes are namespaced on the wire.
	if len(envelope.Findings) > 0 && !strings.Contains(envelope.Findings[0].Code, ".") {
		t.Errorf("Findings[0].Code = %q, want namespaced", envelope.Findings[0].Code)
	}

	// Text fallback should be present.
	if len(result.Content) == 0 {
		t.Fatal("Content is empty, want text fallback")
	}

	// Text fallback should contain the findings as JSON.
	b, _ := json.Marshal(result.Content[0])
	text := string(b)
	if !strings.Contains(text, "required") {
		t.Errorf("text fallback missing 'required': %s", text)
	}
}

func TestMCPDiagnosticsError_SingleWarning(t *testing.T) {
	ds := []diagnostics.Diagnostic{
		{
			Code:     "unknown_key",
			Message:  "unknown field 'colour'",
			Path:     "slides[0].colour",
			Severity: diagnostics.SeverityWarning,
		},
	}

	result := MCPDiagnosticsError(ds)

	if !result.IsError {
		t.Error("IsError = false, want true")
	}

	envelope := result.StructuredContent.(diagnostics.FindingEnvelope)
	if envelope.Summary != "1 warning" {
		t.Errorf("Summary = %q, want %q", envelope.Summary, "1 warning")
	}
	// No error-severity finding, so the envelope reports OK=true even though the
	// transport result is flagged IsError.
	if !envelope.OK {
		t.Error("OK = false, want true for a warning-only envelope")
	}
}

func TestMCPSimpleError(t *testing.T) {
	result := MCPSimpleError("INVALID_JSON", "invalid JSON: unexpected EOF")

	if !result.IsError {
		t.Error("IsError = false, want true")
	}

	envelope, ok := result.StructuredContent.(diagnostics.FindingEnvelope)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want diagnostics.FindingEnvelope", result.StructuredContent)
	}

	if len(envelope.Findings) != 1 {
		t.Fatalf("Findings count = %d, want 1", len(envelope.Findings))
	}

	f := envelope.Findings[0]
	if f.Code != "INPUT.INVALID_JSON" {
		t.Errorf("Code = %q, want INPUT.INVALID_JSON", f.Code)
	}
	if f.Category != diagnostics.NamespaceInput {
		t.Errorf("Category = %q, want %q", f.Category, diagnostics.NamespaceInput)
	}
	if f.Message != "invalid JSON: unexpected EOF" {
		t.Errorf("Message = %q", f.Message)
	}
	if f.Severity != diagnostics.SeverityError {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
}

func TestMCPDiagnosticsError_WithFix(t *testing.T) {
	ds := []diagnostics.Diagnostic{
		{
			Code:     "required",
			Message:  "template is required",
			Severity: diagnostics.SeverityError,
			Fix: &diagnostics.Fix{
				Kind:   "provide_value",
				Params: map[string]any{"field": "template"},
			},
		},
	}

	result := MCPDiagnosticsError(ds)

	envelope := result.StructuredContent.(diagnostics.FindingEnvelope)
	rem := envelope.Findings[0].Remediation
	if rem == nil || rem.Primary == nil {
		t.Fatal("Remediation.Primary is nil, want non-nil")
	}
	// "provide_value" is not a member of the action vocabulary, so it maps to
	// replace_value while the original kind is preserved in params.
	if rem.Primary.Action != diagnostics.ActionReplaceValue {
		t.Errorf("Remediation.Primary.Action = %q, want %q", rem.Primary.Action, diagnostics.ActionReplaceValue)
	}
	if got, _ := rem.Primary.Params["kind"].(string); got != "provide_value" {
		t.Errorf("Remediation.Primary.Params[kind] = %v, want provide_value", rem.Primary.Params["kind"])
	}

	// Verify JSON round-trip preserves the original kind.
	b, _ := json.Marshal(envelope)
	if !strings.Contains(string(b), "provide_value") {
		t.Errorf("JSON missing original fix kind: %s", b)
	}
}
