package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
)

// Tests for --strict-unknown-keys parity between CLI generate/validate/preview
// and the corresponding MCP tools.
//
// Default (strict=false): unknown JSON keys are warnings, the operation proceeds.
// Strict (strict=true):   unknown JSON keys are errors, the operation aborts.

// inputWithTypo is a minimal-but-valid PresentationInput with an unknown key
// ("tmplate" — a deliberate typo for "template") at the root.
const inputWithTypo = `{
  "template": "midnight-blue",
  "tmplate": "typo-here",
  "slides": [{
    "layout_id": "slideLayout2",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hi"}]
  }]
}`

func writeTempJSON(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// --- validateJSONFile -------------------------------------------------------

func TestValidateJSONFile_UnknownKey_DefaultWarning(t *testing.T) {
	path := writeTempJSON(t, inputWithTypo)
	result := validateJSONFile(path, "../../templates", false, "warn")
	if !result.Valid {
		t.Fatalf("expected Valid=true with default mode, got errors: %v", result.Errors)
	}
	if !anyContains(result.Warnings, "tmplate") {
		t.Errorf("expected 'tmplate' in warnings, got: %v", result.Warnings)
	}
}

func TestValidateJSONFile_UnknownKey_StrictError(t *testing.T) {
	path := writeTempJSON(t, inputWithTypo)
	result := validateJSONFile(path, "../../templates", true, "warn")
	if result.Valid {
		t.Fatalf("expected Valid=false with strict=true")
	}
	if !anyContains(result.Errors, "tmplate") {
		t.Errorf("expected 'tmplate' in errors, got: %v", result.Errors)
	}
	if anyContains(result.Warnings, "tmplate") {
		t.Errorf("unknown key should be in errors, not warnings: %v", result.Warnings)
	}
}

// --- runJSONDryRun ----------------------------------------------------------

func TestRunJSONDryRun_UnknownKey_DefaultWarning(t *testing.T) {
	path := writeTempJSON(t, inputWithTypo)
	output := captureDryRun(t, func() error {
		return runJSONDryRun(path, "../../templates", "", "", false)
	})
	warns := findingMessages(output.Findings, diagnostics.SeverityWarning)
	if !output.Valid {
		t.Fatalf("expected Valid=true with default mode, got errors: %v", findingMessages(output.Findings, diagnostics.SeverityError))
	}
	if !anyContains(warns, "tmplate") {
		t.Errorf("expected 'tmplate' in warnings, got: %v", warns)
	}
}

func TestRunJSONDryRun_UnknownKey_StrictError(t *testing.T) {
	path := writeTempJSON(t, inputWithTypo)
	output := captureDryRun(t, func() error {
		return runJSONDryRun(path, "../../templates", "", "", true)
	})
	if output.Valid {
		t.Fatalf("expected Valid=false with strict=true")
	}
	errs := findingMessages(output.Findings, diagnostics.SeverityError)
	if !anyContains(errs, "tmplate") {
		t.Errorf("expected 'tmplate' in errors, got: %v", errs)
	}
}

// --- parseJSONInput (used by runJSONMode) -----------------------------------

func TestParseJSONInput_UnknownKey_DefaultWarning(t *testing.T) {
	path := writeTempJSON(t, inputWithTypo)
	_, warnings, err := parseJSONInput(path, "", "", false)
	if err != nil {
		t.Fatalf("unexpected error in default mode: %v", err)
	}
	if !anyContains(warnings, "tmplate") {
		t.Errorf("expected 'tmplate' in warnings, got: %v", warnings)
	}
}

func TestParseJSONInput_UnknownKey_StrictError(t *testing.T) {
	path := writeTempJSON(t, inputWithTypo)
	_, _, err := parseJSONInput(path, "", "", true)
	if err == nil {
		t.Fatalf("expected error in strict mode")
	}
	if !strings.Contains(err.Error(), "tmplate") {
		t.Errorf("expected 'tmplate' in error message, got: %v", err)
	}
}

// --- handlePreviewPlan (MCP) ------------------------------------------------

func TestHandlePreviewPlan_UnknownKey_StrictError(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	var pres any
	if err := json.Unmarshal([]byte(inputWithTypo), &pres); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result, err := mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
		"presentation":        pres,
		"strict_unknown_keys": true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true with strict_unknown_keys=true")
	}
	env := parseMCPError(t, result)
	requireDiagCode(t, env.Diagnostics, "unknown_key")
}

func TestHandlePreviewPlan_UnknownKey_DefaultWarning(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	var pres any
	if err := json.Unmarshal([]byte(inputWithTypo), &pres); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result, err := mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
		"presentation": pres,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected IsError=false in default mode")
	}
}

// --- helpers ----------------------------------------------------------------

func anyContains(items []string, needle string) bool {
	for _, s := range items {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// captureDryRun reroutes stdout while invoking fn, parses the JSON written by
// writeDryRunOutput, and returns the parsed dryRunOutput so tests can assert
// the structured fields directly (instead of grepping stdout text).
func captureDryRun(t *testing.T, fn func() error) dryRunOutput {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	_ = fn() // errors are surfaced via output.Errors / output.Valid
	_ = w.Close()

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, readErr := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	var output dryRunOutput
	if err := json.Unmarshal(buf, &output); err != nil {
		t.Fatalf("dry-run output not JSON: %v\nraw: %s", err, string(buf))
	}
	return output
}
