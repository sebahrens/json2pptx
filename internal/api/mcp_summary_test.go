package api

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestModernMCPTextSummaryIsBoundedAndActionable(t *testing.T) {
	withMode(t, TextFallbackAuto)
	ctx := ctxForSession("modern-summary")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })
	payload := map[string]any{
		"ok": false, "summary": "3 problems", "pptx_path": "/tmp/deck.pptx",
		"large_payload":  strings.Repeat("x", 200_000),
		"next_tool_call": map[string]any{"tool": "repair_slide", "args_template": map[string]any{"slide_index": 0}},
		"findings": []map[string]any{
			{"code": "INPUT.title_wraps", "message": "shorten title", "path": "/slides/0/title"},
			{"code": "INPUT.body_too_long", "message": "split body", "path": "/slides/1/body"},
			{"code": "INPUT.empty_cell", "message": "supply text", "path": "/slides/2/body"},
			{"code": "INPUT.fourth", "message": "should not appear"},
		},
	}
	result, err := MCPSuccessResult(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d, want one text synopsis", len(result.Content))
	}
	text := result.Content[0].(mcp.TextContent).Text
	if len(text) > maxMCPTextSummaryBytes {
		t.Fatalf("synopsis has %d bytes, want at most %d", len(text), maxMCPTextSummaryBytes)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatalf("synopsis is not JSON: %v", err)
	}
	if summary["ok"] != false || summary["summary"] != "3 problems" || summary["pptx_path"] != "/tmp/deck.pptx" {
		t.Errorf("synopsis lost status/path: %v", summary)
	}
	if got := summary["findings_count"]; got != float64(4) {
		t.Errorf("findings_count = %v, want 4", got)
	}
	findings, ok := summary["findings"].([]any)
	if !ok || len(findings) != 3 {
		t.Fatalf("first findings = %v, want three", summary["findings"])
	}
	if !strings.Contains(text, "repair_slide") || strings.Contains(text, "INPUT.fourth") || strings.Contains(text, strings.Repeat("x", 100)) {
		t.Errorf("synopsis omitted action or duplicated large data: %s", text)
	}
	if result.StructuredContent == nil {
		t.Fatal("full structuredContent is missing")
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(wire, &encoded); err != nil {
		t.Fatal(err)
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(encoded["content"], &blocks); err != nil || len(blocks) != 1 {
		t.Errorf("wire content is not one text block: %s", wire)
	}
	if _, ok := encoded["structuredContent"]; !ok {
		t.Errorf("wire structuredContent missing: %s", wire)
	}
}

func TestModernMCPResultForPreservesErrorFlag(t *testing.T) {
	withMode(t, TextFallbackAuto)
	ctx := ctxForSession("modern-is-error")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })
	for _, tc := range []struct {
		name    string
		payload any
		isError bool
	}{
		{"failed ok", map[string]any{"ok": false, "error": "template missing"}, true},
		{"failed success", map[string]any{"success": false, "error": "compile failed"}, true},
		{"successful", map[string]any{"success": true, "pptx_path": "/tmp/out.pptx"}, false},
		{"assessment", map[string]any{"valid": false}, false},
		{"non-object", []string{"a", "b"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := MCPResultFor(ctx, tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError != tc.isError {
				t.Errorf("IsError = %v, want %v", result.IsError, tc.isError)
			}
			if len(result.Content) != 1 {
				t.Errorf("content blocks = %d, want one", len(result.Content))
			}
		})
	}
}

func TestModernGetStartedSummaryNamesNextTools(t *testing.T) {
	summary, err := compactMCPTextSummary(map[string]any{
		"task": "brief",
		"fast_path": map[string]any{"steps": []map[string]any{
			{"tool": "list_slide_kinds"}, {"tool": "validate_deck_spec"},
			{"tool": "render_deck_spec"}, {"tool": "render_deck_thumbnails"},
		}},
		"quality_workflow": strings.Repeat("long instructions", 100),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "validate_deck_spec") || !strings.Contains(string(summary), "render_deck_thumbnails") || len(summary) > maxMCPTextSummaryBytes {
		t.Errorf("get_started synopsis is not actionable: %s", summary)
	}
}

func TestModernMCPTextSummaryHandlesScalarAndArray(t *testing.T) {
	for _, payload := range []any{nil, "done", []int{1, 2, 3}} {
		text, err := compactMCPTextSummary(payload)
		if err != nil {
			t.Fatal(err)
		}
		if len(text) == 0 || len(text) > maxMCPTextSummaryBytes || !json.Valid(text) {
			t.Errorf("payload %T yielded invalid synopsis %s", payload, text)
		}
	}
}

func TestModernMCPTextSummaryBoundsUnicodeWithoutLosingPath(t *testing.T) {
	path := "/tmp/" + strings.Repeat("a", 180) + ".pptx"
	text, err := compactMCPTextSummary(map[string]any{
		"success": false, "error": strings.Repeat("界", 1000), "pptx_path": path,
		"findings": []map[string]any{{"code": "INPUT.title_wraps", "message": strings.Repeat("界", 1000)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(text) > maxMCPTextSummaryBytes || !json.Valid(text) {
		t.Fatalf("unicode synopsis has %d bytes or invalid JSON: %s", len(text), text)
	}
	var summary map[string]any
	if err := json.Unmarshal(text, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["success"] != false || summary["pptx_path"] != path || summary["findings_count"] != float64(1) {
		t.Errorf("unicode synopsis lost status, path or finding count: %v", summary)
	}
}

func TestModernDiscoverySummaryNamesKinds(t *testing.T) {
	text, err := compactMCPTextSummary(map[string]any{
		"slide_kinds": []map[string]any{
			{"kind": "title", "item_schema": map[string]any{"large": strings.Repeat("x", 4000)}},
			{"kind": "matrix", "item_schema": map[string]any{"large": strings.Repeat("x", 4000)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(text) > maxMCPTextSummaryBytes || !strings.Contains(string(text), `"slide_kinds_count":2`) || !strings.Contains(string(text), `"title"`) || !strings.Contains(string(text), `"matrix"`) || strings.Contains(string(text), strings.Repeat("x", 100)) {
		t.Errorf("discovery synopsis = %s", text)
	}
}

func TestModernMCPTextSummaryOversizeFallbackKeepsErrorClue(t *testing.T) {
	long := strings.Repeat("x", 600)
	payload := map[string]any{
		"ok": false, "error": long, "summary": long, "message": long,
		"pptx_path": "/tmp/" + long, "path": "/tmp/" + long, "output_filename": long,
		"task": long, "next_tool_call": map[string]any{"tool": "repair_slide", "args_template": map[string]any{"path": "/slides/0/title"}},
		"findings":    []map[string]any{{"code": "INPUT.title_wraps", "message": long}},
		"diagnostics": []map[string]any{{"code": "INPUT.other", "message": long}},
	}
	text, err := compactMCPTextSummary(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(text) > maxMCPTextSummaryBytes || !json.Valid(text) {
		t.Fatalf("oversized synopsis has %d bytes or invalid JSON: %s", len(text), text)
	}
	var summary map[string]any
	if err := json.Unmarshal(text, &summary); err != nil {
		t.Fatal(err)
	}
	if _, ok := summary["first_finding"]; !ok {
		t.Errorf("oversize case did not exercise the minimal fallback: %s", text)
	}
	if summary["ok"] != false || !strings.Contains(string(text), "INPUT.title_wraps") || !strings.Contains(string(text), "repair_slide") {
		t.Errorf("oversize fallback lost failure recovery clues: %s", text)
	}
}

func TestModernMCPResultRejectsUnserializablePayload(t *testing.T) {
	withMode(t, TextFallbackAuto)
	ctx := ctxForSession("modern-bad-json")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })
	if result, err := MCPResultFor(ctx, map[string]any{"success": true, "value": math.NaN()}); err == nil || result != nil {
		t.Errorf("unserializable payload yielded result=%v err=%v, want nil result and an error", result, err)
	}
}

func TestModernMCPGenericSummaryKeepsBoundedValues(t *testing.T) {
	data := map[string]any{}
	for _, key := range []string{"a", "b", "c", "d", "e", "f"} {
		data[key] = strings.Repeat(key, 600)
	}
	text, err := compactMCPTextSummary(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(text) > maxMCPTextSummaryBytes || !strings.Contains(string(text), `"a":"`) || strings.Contains(string(text), `"e":"`) {
		t.Errorf("generic summary does not retain bounded first values: %s", text)
	}
}

func TestModernMCPTextSummaryBoundsEscapedJSON(t *testing.T) {
	escaped := strings.Repeat("\"<\\", 600)
	text, err := compactMCPTextSummary(map[string]any{
		"ok": false, "error": escaped, "summary": escaped,
		"pptx_path": escaped, "path": escaped, "output_filename": escaped,
		"findings":       []map[string]any{{"code": escaped, "message": escaped}},
		"next_tool_call": map[string]any{"tool": "repair_slide"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(text) > maxMCPTextSummaryBytes || !json.Valid(text) {
		t.Fatalf("escaped synopsis has %d bytes or invalid JSON: %s", len(text), text)
	}
	var summary map[string]any
	if err := json.Unmarshal(text, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["ok"] != false || summary["error"] == nil {
		t.Errorf("escaped synopsis lost failure status or error: %s", text)
	}
}
