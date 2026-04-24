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

	// StructuredContent should be the error envelope.
	envelope, ok := result.StructuredContent.(mcpErrorEnvelope)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want mcpErrorEnvelope", result.StructuredContent)
	}

	if len(envelope.Diagnostics) != 2 {
		t.Errorf("Diagnostics count = %d, want 2", len(envelope.Diagnostics))
	}
	if envelope.Summary != "2 errors" {
		t.Errorf("Summary = %q, want %q", envelope.Summary, "2 errors")
	}

	// Text fallback should be present.
	if len(result.Content) == 0 {
		t.Fatal("Content is empty, want text fallback")
	}

	// Text fallback should contain the diagnostics as JSON.
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

	envelope := result.StructuredContent.(mcpErrorEnvelope)
	if envelope.Summary != "1 warning" {
		t.Errorf("Summary = %q, want %q", envelope.Summary, "1 warning")
	}
}

func TestMCPSimpleError(t *testing.T) {
	result := MCPSimpleError("INVALID_JSON", "invalid JSON: unexpected EOF")

	if !result.IsError {
		t.Error("IsError = false, want true")
	}

	envelope, ok := result.StructuredContent.(mcpErrorEnvelope)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want mcpErrorEnvelope", result.StructuredContent)
	}

	if len(envelope.Diagnostics) != 1 {
		t.Fatalf("Diagnostics count = %d, want 1", len(envelope.Diagnostics))
	}

	d := envelope.Diagnostics[0]
	if d.Code != "INVALID_JSON" {
		t.Errorf("Code = %q, want INVALID_JSON", d.Code)
	}
	if d.Message != "invalid JSON: unexpected EOF" {
		t.Errorf("Message = %q", d.Message)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("Severity = %q, want error", d.Severity)
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

	envelope := result.StructuredContent.(mcpErrorEnvelope)
	if envelope.Diagnostics[0].Fix == nil {
		t.Error("Fix is nil, want non-nil")
	}
	if envelope.Diagnostics[0].Fix.Kind != "provide_value" {
		t.Errorf("Fix.Kind = %q, want provide_value", envelope.Diagnostics[0].Fix.Kind)
	}

	// Verify JSON round-trip preserves fix.
	b, _ := json.Marshal(envelope)
	if !strings.Contains(string(b), "provide_value") {
		t.Errorf("JSON missing fix kind: %s", b)
	}
}
