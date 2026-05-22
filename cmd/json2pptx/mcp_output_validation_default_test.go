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
// that agents depend on when strict output validation rejects a generated deck.
// The shape is: {summary, findings[], repairable:false, repair_unavailable_reason,
// next_tool_call:{tool:"describe_finding", args_template:{code}}}. The recovery
// hint points at describe_finding — a directly-executable call — instead of
// repair_slide with an empty fixes array, which repair_slide rejects
// (go-slide-creator-gy8j). The full finding context (code/scope/source_path/
// slide_index) is preserved so an agent can map a finding to a repair_slide
// directive itself.
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

	for _, field := range []string{"summary", "findings", "repairable", "next_tool_call"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("envelope missing required field %q (raw=%s)", field, string(b))
		}
	}

	var repairable bool
	if err := json.Unmarshal(raw["repairable"], &repairable); err != nil {
		t.Fatalf("repairable is not a bool: %v", err)
	}
	if repairable {
		t.Error("repairable should be false for output-validation findings")
	}
	if _, ok := raw["repair_unavailable_reason"]; !ok {
		t.Errorf("envelope missing repair_unavailable_reason when repairable=false (raw=%s)", string(b))
	}

	var findings []pptx.Finding
	if err := json.Unmarshal(raw["findings"], &findings); err != nil {
		t.Fatalf("findings is not an array: %v", err)
	}
	if len(findings) != 1 || findings[0].Code != "OOXML_INVALID_COLOR" {
		t.Errorf("findings did not roundtrip: %+v", findings)
	}
	// Slide/path/code context must survive so agents can target a repair.
	if findings[0].SlideIndex != 2 || findings[0].SourcePath != "/slides/2" || findings[0].Scope != pptx.RepairScopeSource {
		t.Errorf("finding lost slide/path/scope context: %+v", findings[0])
	}

	var next struct {
		Tool         string         `json:"tool"`
		ArgsTemplate map[string]any `json:"args_template"`
	}
	if err := json.Unmarshal(raw["next_tool_call"], &next); err != nil {
		t.Fatalf("next_tool_call is not an object: %v", err)
	}
	if next.Tool != "describe_finding" {
		t.Errorf("next_tool_call.tool = %q, want %q", next.Tool, "describe_finding")
	}
	if next.ArgsTemplate["fixes"] != nil {
		t.Errorf("next_tool_call must not advertise repair_slide fixes, got %v", next.ArgsTemplate["fixes"])
	}
	code, ok := next.ArgsTemplate["code"].(string)
	if !ok || code == "" {
		t.Errorf("args_template.code = %v, want a non-empty string", next.ArgsTemplate["code"])
	}
}

// TestMCPOutputValidationError_NextCallIsExecutable is the contract test the
// task (go-slide-creator-gy8j) requires: every emitted next_tool_call must be
// directly executable. It builds the envelope from a multi-slide report with a
// mix of describable and non-describable codes, then actually invokes the
// suggested describe_finding call and asserts it does NOT return an error
// result — i.e. the args satisfy describe_finding's schema and the code
// resolves to a real description.
func TestMCPOutputValidationError_NextCallIsExecutable(t *testing.T) {
	reports := map[string]*pptx.Report{
		"specific-code-not-describable": {
			Findings: []pptx.Finding{
				{Code: "OOXML_INVALID_COLOR", Severity: pptx.SeverityBlocking, SlideIndex: 1, Scope: pptx.RepairScopeSource},
				{Code: "OOXML_DUPLICATE_ID", Severity: pptx.SeverityBlocking, SlideIndex: 4, Scope: pptx.RepairScopeGenerator},
			},
		},
		"umbrella-fallback-no-codes": {
			Findings: []pptx.Finding{
				{Code: "", Severity: pptx.SeverityBlocking, SlideIndex: -1},
			},
		},
	}

	for name, report := range reports {
		t.Run(name, func(t *testing.T) {
			result := mcpOutputValidationError(report)
			b, _ := json.Marshal(result.StructuredContent)
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(b, &raw); err != nil {
				t.Fatalf("envelope is not a JSON object: %v", err)
			}
			var next struct {
				Tool         string         `json:"tool"`
				ArgsTemplate map[string]any `json:"args_template"`
			}
			if err := json.Unmarshal(raw["next_tool_call"], &next); err != nil {
				t.Fatalf("next_tool_call missing or malformed: %v", err)
			}
			if next.Tool != "describe_finding" {
				t.Fatalf("next_tool_call.tool = %q, want describe_finding", next.Tool)
			}

			// Execute the suggested call verbatim.
			execResult, err := handleDescribeFinding(context.Background(), makeRequest(next.ArgsTemplate))
			if err != nil {
				t.Fatalf("executing suggested describe_finding returned a transport error: %v", err)
			}
			if execResult.IsError {
				t.Fatalf("suggested describe_finding call was rejected — recovery hint is not executable: %+v", execResult.StructuredContent)
			}
		})
	}
}
