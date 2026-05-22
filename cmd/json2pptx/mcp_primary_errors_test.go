package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
)

// These tests pin the *actionable* envelope contract for the high-frequency
// runtime failures of primary MCP tools. Beyond IsError, every case must carry
// the machine-readable recovery fields an agent needs: the offending JSON path,
// an executable next_tool_call, and (where the contract is clear) expected_type,
// example_value, and contextual details — while keeping the human message
// concise.

func TestMCPPrimaryErrors_TemplateNotFound_Envelope(t *testing.T) {
	env := parseMCPError(t, mcpTemplateNotFoundError("nonexistent-template-xyz", "../../templates"))
	d := env.Diagnostics[0]

	if d.Code != diagnostics.CodeTemplateNotFound {
		t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeTemplateNotFound)
	}
	if d.Path != "template" {
		t.Errorf("path = %q, want %q", d.Path, "template")
	}
	if d.ExpectedType != "string" {
		t.Errorf("expected_type = %q, want string", d.ExpectedType)
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "list_templates" {
		t.Errorf("next_tool_call = %v, want tool=list_templates", d.NextToolCall)
	}
	if got, _ := d.Details["template_name"].(string); got != "nonexistent-template-xyz" {
		t.Errorf("details.template_name = %v, want the requested name", d.Details["template_name"])
	}
	if _, ok := d.Details["available_templates"]; !ok {
		t.Error("expected details.available_templates listing the bundled templates")
	}
	if d.ExampleValue == nil {
		t.Error("expected example_value (a valid template name), got nil")
	}
	if !strings.Contains(d.Message, "nonexistent-template-xyz") {
		t.Errorf("message %q should name the missing template", d.Message)
	}
}

// End-to-end: the missing-template path through generate_presentation must
// surface the same actionable envelope, not a bare code/message.
func TestMCPPrimaryErrors_TemplateNotFound_ViaGenerate(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(`{"template":"nonexistent-template-xyz","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, result)
	d := env.Diagnostics[0]
	if d.Code != diagnostics.CodeTemplateNotFound {
		t.Fatalf("code = %q, want %q", d.Code, diagnostics.CodeTemplateNotFound)
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "list_templates" {
		t.Errorf("next_tool_call = %v, want tool=list_templates", d.NextToolCall)
	}
	if d.Path != "template" {
		t.Errorf("path = %q, want template", d.Path)
	}
}

func TestMCPPrimaryErrors_ReadPresentation_FileNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.pptx")
	res, err := handleReadPresentation(context.Background(), makeRequest(map[string]any{
		"pptx_path": missing,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, res)
	d := env.Diagnostics[0]
	if d.Code != diagnostics.CodeFileNotFound {
		t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeFileNotFound)
	}
	if d.Path != "pptx_path" {
		t.Errorf("path = %q, want pptx_path", d.Path)
	}
	if d.ExpectedType != "string" {
		t.Errorf("expected_type = %q, want string", d.ExpectedType)
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "read_presentation" {
		t.Errorf("next_tool_call = %v, want tool=read_presentation", d.NextToolCall)
	}
	if got, _ := d.Details["file_path"].(string); got != missing {
		t.Errorf("details.file_path = %v, want %q", d.Details["file_path"], missing)
	}
}

func TestMCPPrimaryErrors_ReadPresentation_ReadFailed(t *testing.T) {
	// A file that exists but is not a valid PPTX (OPC/zip) must report
	// READ_FAILED with a next hop at the structural validator.
	junk := filepath.Join(t.TempDir(), "corrupt.pptx")
	if err := os.WriteFile(junk, []byte("this is not a zip"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := handleReadPresentation(context.Background(), makeRequest(map[string]any{
		"pptx_path": junk,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, res)
	d := env.Diagnostics[0]
	if d.Code != diagnostics.CodeReadFailed {
		t.Fatalf("code = %q, want %q", d.Code, diagnostics.CodeReadFailed)
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "validate_presentation_output" {
		t.Errorf("next_tool_call = %v, want tool=validate_presentation_output", d.NextToolCall)
	}
	if got, _ := d.NextToolCall.ArgsTemplate["path"].(string); got != junk {
		t.Errorf("next_tool_call args.path = %v, want %q", d.NextToolCall.ArgsTemplate["path"], junk)
	}
}

func TestMCPPrimaryErrors_InvalidSlideIndex_Envelope(t *testing.T) {
	env := parseMCPError(t, mcpInvalidSlideIndexError(7, 3))
	d := env.Diagnostics[0]
	if d.Code != diagnostics.CodeInvalidSlideIndex {
		t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeInvalidSlideIndex)
	}
	if d.Path != "slide_index" {
		t.Errorf("path = %q, want slide_index", d.Path)
	}
	if d.ExpectedType != "integer" {
		t.Errorf("expected_type = %q, want integer", d.ExpectedType)
	}
	// slide_count is carried as a JSON number after the round-trip.
	if got, _ := d.Details["slide_count"].(float64); int(got) != 3 {
		t.Errorf("details.slide_count = %v, want 3", d.Details["slide_count"])
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "read_presentation" {
		t.Errorf("next_tool_call = %v, want tool=read_presentation", d.NextToolCall)
	}
	if !strings.Contains(d.Message, "out of range") {
		t.Errorf("message %q should explain the range error", d.Message)
	}
}

func TestMCPPrimaryErrors_ValidateOutput_FileNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.pptx")
	res, err := handleValidateOutput(context.Background(), makeRequest(map[string]any{
		"path": missing,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := parseMCPError(t, res)
	d := env.Diagnostics[0]
	if d.Code != diagnostics.CodeFileNotFound {
		t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeFileNotFound)
	}
	if d.Path != "path" {
		t.Errorf("path = %q, want path", d.Path)
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "validate_presentation_output" {
		t.Errorf("next_tool_call = %v, want tool=validate_presentation_output", d.NextToolCall)
	}
	if got, _ := d.Details["file_path"].(string); got != missing {
		t.Errorf("details.file_path = %v, want %q", d.Details["file_path"], missing)
	}
}

func TestMCPPrimaryErrors_ValidateOutput_InvalidPath(t *testing.T) {
	cases := map[string]string{
		"wrong_extension": "/tmp/secrets.txt",
		"traversal":       "../../etc/passwd.pptx",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := handleValidateOutput(context.Background(), makeRequest(map[string]any{
				"path": path,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			env := parseMCPError(t, res)
			d := env.Diagnostics[0]
			if d.Code != diagnostics.CodeInvalidPath {
				t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeInvalidPath)
			}
			if d.Path != "path" {
				t.Errorf("path = %q, want path", d.Path)
			}
			if d.NextToolCall == nil || d.NextToolCall.Tool != "validate_presentation_output" {
				t.Errorf("next_tool_call = %v, want tool=validate_presentation_output", d.NextToolCall)
			}
		})
	}
}

func TestMCPPrimaryErrors_ValidationFailed_Envelope(t *testing.T) {
	env := parseMCPError(t, mcpValidationFailedError("/tmp/out/deck.pptx", errors.New("zip: not a valid archive")))
	d := env.Diagnostics[0]
	if d.Code != diagnostics.CodeValidationFailed {
		t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeValidationFailed)
	}
	if d.Path != "path" {
		t.Errorf("path = %q, want path", d.Path)
	}
	if d.NextToolCall == nil || d.NextToolCall.Tool != "read_presentation" {
		t.Errorf("next_tool_call = %v, want tool=read_presentation", d.NextToolCall)
	}
	if got, _ := d.NextToolCall.ArgsTemplate["pptx_path"].(string); got != "/tmp/out/deck.pptx" {
		t.Errorf("next_tool_call args.pptx_path = %v, want the file path", d.NextToolCall.ArgsTemplate["pptx_path"])
	}
	if !strings.Contains(d.Message, "not a valid archive") {
		t.Errorf("message %q should carry the underlying cause", d.Message)
	}
}
