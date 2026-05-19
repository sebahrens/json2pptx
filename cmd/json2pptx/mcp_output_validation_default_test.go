package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
)

// TestMCPGenerate_StrictOutputValidationIsDefault verifies that omitting the
// output_validation parameter from a generate_presentation call still produces
// a successful result for a known-valid deck. The default switched from "off"
// to "strict" as part of the 'zero needs repair' guarantee
// (go-slide-creator-0myv) — a regression here would silently re-enable the
// pre-guarantee behavior where agents were free to ship structurally broken
// decks. This test pins down the new contract.
func TestMCPGenerate_StrictOutputValidationIsDefault(t *testing.T) {
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
				"text_value": "Strict default test"
			}]
		}]
	}`

	// Intentionally omit output_validation — exercising the default path.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("strict default rejected a valid deck — engine produced findings that broke strict mode: %+v", result.StructuredContent)
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}
	var success bool
	if err := json.Unmarshal(raw["success"], &success); err != nil || !success {
		t.Fatalf("expected success=true, got raw=%s err=%v", string(b), err)
	}
}

// TestMCPOutputValidationError_EnvelopeShape pins the structured-error contract
// that agents depend on when strict output validation rejects a generated
// deck. The shape is: {summary, findings[], next_tool_call:{tool:"repair_slide",
// args_template:{slide_index, fixes:[]}}}. This is the machine-readable hook
// that lets agents chain a fix without re-deriving the protocol from prose.
func TestMCPOutputValidationError_EnvelopeShape(t *testing.T) {
	report := &pptx.Report{
		Findings: []pptx.Finding{
			{
				Code:       "OOXML_INVALID_COLOR",
				Severity:   pptx.SeverityBlocking,
				Path:       "ppt/slides/slide3.xml",
				Message:    "invalid color value ZZZZZZ",
				Phase:      "ooxml",
				Validator:  "ooxml_content",
				SlideIndex: 2,
				SourcePath: "/slides/2",
				Scope:      pptx.RepairScopeSource,
			},
		},
	}

	result := mcpOutputValidationError(report)
	if result == nil {
		t.Fatal("mcpOutputValidationError returned nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for blocking-finding envelope")
	}
	if result.StructuredContent == nil {
		t.Fatal("expected StructuredContent to be populated for programmatic agents")
	}

	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("envelope is not a JSON object: %v", err)
	}

	for _, field := range []string{"summary", "findings", "next_tool_call"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("envelope missing required field %q (raw=%s)", field, string(b))
		}
	}

	var findings []pptx.Finding
	if err := json.Unmarshal(raw["findings"], &findings); err != nil {
		t.Fatalf("findings is not an array: %v", err)
	}
	if len(findings) != 1 || findings[0].Code != "OOXML_INVALID_COLOR" {
		t.Errorf("findings did not roundtrip: %+v", findings)
	}

	var next struct {
		Tool         string         `json:"tool"`
		ArgsTemplate map[string]any `json:"args_template"`
	}
	if err := json.Unmarshal(raw["next_tool_call"], &next); err != nil {
		t.Fatalf("next_tool_call is not an object: %v", err)
	}
	if next.Tool != "repair_slide" {
		t.Errorf("next_tool_call.tool = %q, want %q", next.Tool, "repair_slide")
	}
	if got, _ := next.ArgsTemplate["slide_index"].(float64); int(got) != 2 {
		t.Errorf("args_template.slide_index = %v, want 2", next.ArgsTemplate["slide_index"])
	}
	fixes, ok := next.ArgsTemplate["fixes"].([]any)
	if !ok {
		t.Errorf("args_template.fixes is not an array, got %T", next.ArgsTemplate["fixes"])
	} else if len(fixes) != 0 {
		t.Errorf("args_template.fixes = %v, want empty array", fixes)
	}
}

// TestMCPOutputValidationError_MultiSlideSentinel verifies that when blocking
// findings span more than one source slide, the next_tool_call.args_template
// encodes slide_index=-1. Agents are expected to fill in the slide_index from
// each finding's slide_index field; the sentinel is the explicit hand-off.
func TestMCPOutputValidationError_MultiSlideSentinel(t *testing.T) {
	report := &pptx.Report{
		Findings: []pptx.Finding{
			{
				Code:       "OOXML_INVALID_COLOR",
				Severity:   pptx.SeverityBlocking,
				SlideIndex: 1,
				Scope:      pptx.RepairScopeSource,
			},
			{
				Code:       "OOXML_DUPLICATE_ID",
				Severity:   pptx.SeverityBlocking,
				SlideIndex: 4,
				Scope:      pptx.RepairScopeSource,
			},
		},
	}

	result := mcpOutputValidationError(report)
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("envelope is not a JSON object: %v", err)
	}
	var next struct {
		ArgsTemplate map[string]any `json:"args_template"`
	}
	if err := json.Unmarshal(raw["next_tool_call"], &next); err != nil {
		t.Fatalf("next_tool_call missing or malformed: %v", err)
	}
	got, _ := next.ArgsTemplate["slide_index"].(float64)
	if int(got) != -1 {
		t.Errorf("multi-slide envelope: slide_index = %v, want -1", next.ArgsTemplate["slide_index"])
	}
}
