package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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
	// Structured content should expose the diagnostics envelope with a
	// use_one_of fix whose params.allowed enumerates the vocabulary.
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil — agents depend on it to discover allowed codes")
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var env struct {
		Diagnostics []struct {
			Code string `json:"code"`
			Fix  struct {
				Kind   string         `json:"kind"`
				Params map[string]any `json:"params"`
			} `json:"fix"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("parse error envelope: %v", err)
	}
	if len(env.Diagnostics) == 0 {
		t.Fatal("error envelope has no diagnostics entries")
	}
	d := env.Diagnostics[0]
	if d.Code != "UNKNOWN_FINDING_CODE" {
		t.Errorf("diagnostic.code = %q, want UNKNOWN_FINDING_CODE", d.Code)
	}
	if d.Fix.Kind != "use_one_of" {
		t.Errorf("fix.kind = %q, want use_one_of", d.Fix.Kind)
	}
	allowed, ok := d.Fix.Params["allowed"].([]any)
	if !ok {
		t.Fatalf("fix.params.allowed is not an array: %T", d.Fix.Params["allowed"])
	}
	if len(allowed) == 0 {
		t.Fatal("fix.params.allowed must list at least one known code")
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
