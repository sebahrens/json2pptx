package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
	"github.com/sebahrens/json2pptx/internal/template"
)

// These tests exercise the cmd/json2pptx boundary: the unresolved-placeholder
// policy applied to validate_input / generate_presentation and its conversion to
// diagnostics. The JSON-walk scanner itself is unit tested in
// internal/policy/placeholder (including shape_grid, table, chart, and notes
// coverage).

func TestPlaceholderPolicyFromRequest(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"absent defaults to warn", map[string]any{}, "warn"},
		{"empty defaults to warn", map[string]any{"placeholder_policy": ""}, "warn"},
		{"off honored", map[string]any{"placeholder_policy": "off"}, "off"},
		{"warn honored", map[string]any{"placeholder_policy": "warn"}, "warn"},
		{"strict honored", map[string]any{"placeholder_policy": "strict"}, "strict"},
		{"unknown clamps to warn", map[string]any{"placeholder_policy": "bogus"}, "warn"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := placeholderPolicyFromRequest(makeRequest(tc.args)); got != tc.want {
				t.Errorf("placeholderPolicyFromRequest(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestPlaceholderDiagnostics_WarnVsStrict(t *testing.T) {
	violations := []placeholder.Violation{
		{Path: "slides[0].content[0].text_value", Value: "__FILL__", Token: placeholder.Token},
	}

	warnDiags := placeholderDiagnostics(violations, false)
	if len(warnDiags) != 1 {
		t.Fatalf("expected 1 warn diagnostic, got %d", len(warnDiags))
	}
	w := warnDiags[0]
	if w.Severity != diagnostics.SeverityWarning {
		t.Errorf("warn severity = %q, want warning", w.Severity)
	}
	if w.Code != placeholder.FindingCode {
		t.Errorf("code = %q, want %q", w.Code, placeholder.FindingCode)
	}
	if w.Path != violations[0].Path {
		t.Errorf("path = %q, want %q", w.Path, violations[0].Path)
	}
	if w.Fix == nil || w.Fix.Kind != "replace_placeholder" {
		t.Errorf("fix should be replace_placeholder, got %+v", w.Fix)
	}
	if w.NextToolCall == nil || w.NextToolCall.Tool != "validate_input" {
		t.Errorf("next_tool_call should point at validate_input, got %+v", w.NextToolCall)
	}
	if !strings.Contains(w.Message, placeholder.Token) {
		t.Errorf("warn message %q should name the token", w.Message)
	}

	strictDiags := placeholderDiagnostics(violations, true)
	if len(strictDiags) != 1 || strictDiags[0].Severity != diagnostics.SeverityError {
		t.Fatalf("expected 1 error diagnostic in strict mode, got %+v", strictDiags)
	}
}

func TestScanPlaceholderDiagnostics_OffSkips(t *testing.T) {
	input := map[string]any{"slides": []any{map[string]any{"x": "__FILL__"}}}
	diags, blocking := scanPlaceholderDiagnostics(input, "off")
	if diags != nil || blocking {
		t.Errorf("off policy should skip the scan, got diags=%+v blocking=%v", diags, blocking)
	}

	diags, blocking = scanPlaceholderDiagnostics(input, "warn")
	if len(diags) != 1 || blocking {
		t.Errorf("warn policy should produce 1 non-blocking diagnostic, got diags=%+v blocking=%v", diags, blocking)
	}

	diags, blocking = scanPlaceholderDiagnostics(input, "strict")
	if len(diags) != 1 || !blocking {
		t.Errorf("strict policy should produce 1 blocking diagnostic, got diags=%+v blocking=%v", diags, blocking)
	}
}

// deckWithPlaceholders is a structurally valid midnight-blue deck that still
// carries __FILL__ tokens in both a placeholder text value and the speaker
// notes — proving the scan reaches typed fields and notes through the handler.
const deckWithPlaceholders = `{
	"template": "midnight-blue",
	"slides": [{
		"layout_id": "title",
		"speaker_notes": "intro: __FILL__",
		"content": [{
			"placeholder_id": "title",
			"type": "text",
			"text_value": "__FILL__"
		}]
	}]
}`

func newPlaceholderTestConfig(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

func TestHandleValidate_PlaceholderWarnByDefault(t *testing.T) {
	mc := newPlaceholderTestConfig(t)
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckWithPlaceholders),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("warn-mode validate must not error: %+v", result.StructuredContent)
	}

	b, _ := json.Marshal(result.StructuredContent)
	var resp dryRunOutput
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !resp.Valid {
		t.Error("warn-mode placeholders must keep valid=true")
	}

	var paths []string
	for _, f := range resp.Findings.Findings {
		if strings.Contains(f.Code, placeholder.FindingCode) {
			if f.Severity != diagnostics.SeverityWarning {
				t.Errorf("placeholder finding severity = %q, want warning", f.Severity)
			}
			if p, ok := f.Evidence["path"].(string); ok {
				paths = append(paths, p)
			}
		}
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 placeholder findings (text + notes), got %d: %+v", len(paths), resp.Findings.Findings)
	}
	wantText, wantNotes := false, false
	for _, p := range paths {
		if strings.Contains(p, "text_value") {
			wantText = true
		}
		if strings.Contains(p, "speaker_notes") {
			wantNotes = true
		}
	}
	if !wantText || !wantNotes {
		t.Errorf("placeholder finding paths should cover text_value and speaker_notes, got %v", paths)
	}
}

func TestHandleValidate_PlaceholderStrictBlocks(t *testing.T) {
	mc := newPlaceholderTestConfig(t)
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation":       mustParseJSON(deckWithPlaceholders),
		"placeholder_policy": "strict",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("strict-mode unresolved placeholders must fail validation")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("error envelope is not a JSON object: %v", err)
	}
	if !strings.Contains(string(b), placeholder.FindingCode) {
		t.Errorf("strict error envelope should carry the %q code: %s", placeholder.FindingCode, string(b))
	}
}

func TestHandleValidate_PlaceholderPolicyOffSkips(t *testing.T) {
	mc := newPlaceholderTestConfig(t)
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation":       mustParseJSON(deckWithPlaceholders),
		"placeholder_policy": "off",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("off-mode validate must not error: %+v", result.StructuredContent)
	}
	b, _ := json.Marshal(result.StructuredContent)
	if strings.Contains(string(b), placeholder.FindingCode) {
		t.Errorf("off policy must not emit placeholder findings: %s", string(b))
	}
}

func TestHandleGenerate_PlaceholderStrictBlocks(t *testing.T) {
	mc := newPlaceholderTestConfig(t)
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation":       mustParseJSON(deckWithPlaceholders),
		"placeholder_policy": "strict",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("strict-mode generate must refuse a deck with unresolved placeholders")
	}
	b, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(b), placeholder.FindingCode) {
		t.Errorf("strict generate error envelope should carry the %q code: %s", placeholder.FindingCode, string(b))
	}
}

func TestHandleGenerate_PlaceholderWarnByDefault(t *testing.T) {
	mc := newPlaceholderTestConfig(t)
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckWithPlaceholders),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("warn-mode generate must still produce a deck: %+v", result.StructuredContent)
	}

	b, _ := json.Marshal(result.StructuredContent)
	var resp JSONOutput
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !resp.Success {
		t.Error("warn-mode generate should succeed")
	}
	found := false
	for _, w := range resp.Warnings {
		if strings.Contains(w, placeholder.Token) {
			found = true
		}
	}
	if !found {
		t.Errorf("warn-mode generate should surface the unresolved placeholder in warnings, got %v", resp.Warnings)
	}
}
