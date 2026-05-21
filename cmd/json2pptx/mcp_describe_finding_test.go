package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// callDescribeFinding invokes the describe_finding handler with the given code
// and returns the parsed success payload. Use describeFindingError when the
// test expects the IsError envelope.
func callDescribeFinding(t *testing.T, code string) patterns.FindingMeta {
	t.Helper()
	args := map[string]any{}
	if code != "" {
		args["code"] = code
	}
	result, err := handleDescribeFinding(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		t.Fatalf("handleDescribeFinding transport error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleDescribeFinding returned IsError for code %q: %v", code, result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var meta patterns.FindingMeta
	if err := json.Unmarshal([]byte(text), &meta); err != nil {
		t.Fatalf("failed to parse describe_finding response: %v", err)
	}
	return meta
}

func TestDescribeFinding_ReturnsMetadataForKnownCode(t *testing.T) {
	meta := callDescribeFinding(t, patterns.ErrCodePlaceholderOverflow)
	if meta.Code != patterns.ErrCodePlaceholderOverflow {
		t.Errorf("code = %q, want %q", meta.Code, patterns.ErrCodePlaceholderOverflow)
	}
	if meta.Summary == "" {
		t.Error("summary is empty")
	}
	if meta.Severity != "shrink_or_split" {
		t.Errorf("severity = %q, want shrink_or_split", meta.Severity)
	}
	if len(meta.RemediationSteps) == 0 {
		t.Error("remediation_steps is empty")
	}
}

func TestDescribeFinding_MissingCodeReturnsError(t *testing.T) {
	result, err := handleDescribeFinding(context.Background(), mcpRequestWithArgs(map[string]any{}))
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError envelope when code parameter is missing")
	}
}

func TestDescribeFinding_UnknownCodeReturnsAllowedList(t *testing.T) {
	result, err := handleDescribeFinding(context.Background(), mcpRequestWithArgs(map[string]any{
		"code": "totally_made_up_code",
	}))
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError envelope for unknown code")
	}
	// Structured content should expose the FindingEnvelope with a remediation
	// whose params.allowed enumerates the vocabulary. The source fix kind
	// "use_one_of" is not a member of the action vocabulary, so it maps to the
	// replace_value action while the original kind is preserved in params.
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil — agents depend on it to discover allowed codes")
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var fe diagnostics.FindingEnvelope
	if err := json.Unmarshal(b, &fe); err != nil {
		t.Fatalf("parse finding envelope: %v", err)
	}
	if len(fe.Findings) == 0 {
		t.Fatal("finding envelope has no findings entries")
	}
	f := fe.Findings[0]
	if f.Code != "INPUT.UNKNOWN_FINDING_CODE" {
		t.Errorf("finding.code = %q, want INPUT.UNKNOWN_FINDING_CODE", f.Code)
	}
	if f.Remediation == nil || f.Remediation.Primary == nil {
		t.Fatal("finding has no primary remediation")
	}
	primary := f.Remediation.Primary
	if primary.Action != diagnostics.ActionReplaceValue {
		t.Errorf("remediation.primary.action = %q, want %q", primary.Action, diagnostics.ActionReplaceValue)
	}
	if got, _ := primary.Params["kind"].(string); got != "use_one_of" {
		t.Errorf("remediation.primary.params[kind] = %v, want use_one_of", primary.Params["kind"])
	}
	allowed, ok := primary.Params["allowed"].([]any)
	if !ok {
		t.Fatalf("remediation.primary.params.allowed is not an array: %T", primary.Params["allowed"])
	}
	if len(allowed) == 0 {
		t.Fatal("remediation.primary.params.allowed must list at least one known code")
	}
}

func TestDescribeFinding_CoversEverySentinelCode(t *testing.T) {
	// Asserts the tool returns a populated payload for every code in
	// AllFitFindingCodes(). This is the runtime mirror of the registry
	// drift test in internal/patterns/finding_meta_test.go.
	for _, code := range patterns.AllFitFindingCodes() {
		code := code
		t.Run(code, func(t *testing.T) {
			meta := callDescribeFinding(t, code)
			if meta.Code != code {
				t.Errorf("code = %q, want %q", meta.Code, code)
			}
			if meta.Summary == "" {
				t.Errorf("code %q returned empty summary", code)
			}
		})
	}
}
