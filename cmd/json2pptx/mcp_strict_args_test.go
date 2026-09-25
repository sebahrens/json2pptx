package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// callToolViaServer sends a tools/call JSON-RPC message through the full
// server stack (middlewares included) and returns the tool result.
func callToolViaServer(t *testing.T, s *server.MCPServer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	msg, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	out := s.HandleMessage(context.Background(), msg)
	resp, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("%s: expected JSON-RPC response, got %T: %+v", name, out, out)
	}
	res, ok := resp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("%s: expected *CallToolResult, got %T", name, resp.Result)
	}
	return res
}

func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	if res.StructuredContent != nil {
		if raw, err := json.Marshal(res.StructuredContent); err == nil {
			b.Write(raw)
		}
	}
	return b.String()
}

func strictArgsTestServer(t *testing.T) *server.MCPServer {
	t.Helper()
	return newMCPServer(&mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()})
}

// plan_deck{slide_count} is now ACCEPTED and rewritten to slide_budget
// (go-slide-creator-r1m3). The did_you_mean error it used to return was good,
// but the cheapest fix is not to need it. A genuinely unknown argument is still
// rejected — see TestStrictArgs_AllToolsRejectBogusArgument.
func TestStrictArgs_PlanDeckAcceptsSlideCount(t *testing.T) {
	s := strictArgsTestServer(t)
	res := callToolViaServer(t, s, "plan_deck", map[string]any{"brief": "Series B pitch", "slide_count": 8})
	if res.IsError && strings.Contains(resultText(res), "UNKNOWN_PARAMETER") {
		t.Fatalf("plan_deck rejected the sibling name slide_count: %s", resultText(res))
	}
}

// Parity: every registered tool rejects a bogus argument before its handler
// runs, so strictness cannot silently regress for a newly added tool.
func TestStrictArgs_AllToolsRejectBogusArgument(t *testing.T) {
	s := strictArgsTestServer(t)
	tools := s.ListTools()
	if len(tools) < 40 {
		t.Fatalf("expected the full tool catalog, got %d tools", len(tools))
	}
	for name := range tools {
		res := callToolViaServer(t, s, name, map[string]any{"zz_bogus_argument": 1})
		if !res.IsError {
			t.Errorf("%s accepted an unknown argument: %s", name, resultText(res))
			continue
		}
		if text := resultText(res); !strings.Contains(text, "UNKNOWN_PARAMETER") || !strings.Contains(text, "zz_bogus_argument") {
			t.Errorf("%s: error does not name UNKNOWN_PARAMETER / the argument: %s", name, text)
		}
	}
}

// Declared arguments still pass through to the handler.
func TestStrictArgs_KnownArgumentsPassThrough(t *testing.T) {
	s := strictArgsTestServer(t)
	res := callToolViaServer(t, s, "get_started", map[string]any{"task": "brief"})
	if res.IsError {
		t.Fatalf("get_started{task} must succeed, got %s", resultText(res))
	}
	res = callToolViaServer(t, s, "list_slide_kinds", map[string]any{})
	if res.IsError {
		t.Fatalf("list_slide_kinds{} must succeed, got %s", resultText(res))
	}
}

func TestStrictArgs_LegacyAliasAccepted(t *testing.T) {
	if res := unknownArgumentsError(mcpMakeDeckTool(), map[string]any{"outline": "x", "max_passes": 2}); res != nil {
		t.Fatalf("make_deck max_passes legacy alias must be accepted, got %s", resultText(res))
	}
	if res := invalidArgumentValuesError(mcpMakeDeckTool(), map[string]any{"max_passes": "2"}); res == nil {
		t.Fatal("make_deck max_passes legacy alias must be type-checked")
	}
}

func TestSuggestArgName(t *testing.T) {
	accepted := []string{"audience", "brief", "must_include", "slide_budget", "template"}
	cases := map[string]string{
		"slide_count": "slide_budget",
		"templte":     "template",
		"breif":       "brief",
		"xyz":         "",
	}
	for in, want := range cases {
		if got := suggestArgName(in, accepted); got != want {
			t.Errorf("suggestArgName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := suggestArgName("anything", nil); got != "" {
		t.Errorf("no accepted names should yield no suggestion, got %q", got)
	}
}

func TestStrictArgs_RawInputSchemaProperties(t *testing.T) {
	tool := mcp.NewToolWithRawSchema("raw_tool", "d", json.RawMessage(`{"type":"object","properties":{"alpha":{"type":"string"}}}`))
	if res := unknownArgumentsError(tool, map[string]any{"alpha": "x"}); res != nil {
		t.Fatalf("declared raw-schema arg rejected: %s", resultText(res))
	}
	res := unknownArgumentsError(tool, map[string]any{"alpah": "x", "beta": 1})
	if res == nil {
		t.Fatal("unknown raw-schema args must be rejected")
	}
	if text := resultText(res); !strings.Contains(text, `did you mean \"alpha\"`) && !strings.Contains(text, fmt.Sprintf("did you mean %q", "alpha")) {
		t.Errorf("expected did-you-mean alpha, got %s", text)
	}
}

func TestStrictArgs_RejectsWrongTypedAndEnumArguments(t *testing.T) {
	s := strictArgsTestServer(t)
	tests := []struct {
		name, tool, path, expected string
		args                       map[string]any
	}{
		{"slide index", "render_slide_image", "slide_index", "number", map[string]any{"pptx_path": "/missing.pptx", "slide_index": "3"}},
		{"max slides", "render_deck_thumbnails", "max_slides", "number", map[string]any{"pptx_path": "/missing.pptx", "max_slides": "2"}},
		{"fit report", "generate_presentation", "fit_report", "boolean", map[string]any{"presentation": map[string]any{}, "fit_report": "true"}},
		{"strict fit enum", "generate_presentation", "strict_fit", "one of", map[string]any{"presentation": map[string]any{}, "strict_fit": "strictly"}},
		{"null value", "render_deck_thumbnails", "max_slides", "number", map[string]any{"pptx_path": "/missing.pptx", "max_slides": nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := callToolViaServer(t, s, tt.tool, tt.args)
			got := resultText(res)
			if !res.IsError || !strings.Contains(got, "INVALID_PARAMETER") || !strings.Contains(got, tt.path) || !strings.Contains(got, tt.expected) {
				t.Fatalf("expected typed argument error for %s: %s", tt.path, got)
			}
		})
	}
}

func TestStrictArgs_RawSchemaTypeAndEnum(t *testing.T) {
	tool := mcp.NewToolWithRawSchema("raw_tool", "d", json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string","enum":["a","b"]},"count":{"type":"integer"}}}`))
	for _, tt := range []struct {
		args map[string]any
		path string
	}{
		{map[string]any{"mode": "c"}, "mode"},
		{map[string]any{"count": 1.5}, "count"},
		{map[string]any{"count": "1"}, "count"},
	} {
		res := invalidArgumentValuesError(tool, tt.args)
		if res == nil || !res.IsError || !strings.Contains(resultText(res), tt.path) {
			t.Fatalf("expected invalid %s: %v", tt.path, res)
		}
	}
	if res := invalidArgumentValuesError(tool, map[string]any{"mode": "a", "count": float64(2)}); res != nil {
		t.Fatalf("valid raw-schema values rejected: %s", resultText(res))
	}
}

func TestStrictArgs_SemanticSpecStringCompatibility(t *testing.T) {
	for _, tool := range []mcp.Tool{mcpValidateDeckSpecTool(), mcpCompileDeckSpecTool(), mcpRenderDeckSpecTool(), mcpExplainDeckSpecTool()} {
		property, ok := tool.InputSchema.Properties["spec"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no spec property schema", tool.Name)
		}
		if got, ok := property["type"].([]string); !ok || !reflect.DeepEqual(got, []string{"object", "string"}) {
			t.Errorf("%s spec type = %v, want object|string", tool.Name, property["type"])
		}
		wire, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("marshal %s: %v", tool.Name, err)
		}
		var published struct {
			InputSchema struct {
				Properties map[string]map[string]any `json:"properties"`
			} `json:"inputSchema"`
		}
		if err := json.Unmarshal(wire, &published); err != nil {
			t.Fatalf("decode %s published schema: %v", tool.Name, err)
		}
		if got := published.InputSchema.Properties["spec"]["type"]; !reflect.DeepEqual(got, []any{"object", "string"}) {
			t.Errorf("%s published spec type = %v, want object|string", tool.Name, got)
		}
		if res := invalidArgumentValuesError(tool, map[string]any{"spec": "meta:\n  title: Test"}); res != nil {
			t.Errorf("%s rejected accepted YAML string: %s", tool.Name, resultText(res))
		}
		if res := invalidArgumentValuesError(tool, map[string]any{"spec": map[string]any{"meta": map[string]any{}}}); res != nil {
			t.Errorf("%s rejected spec object: %s", tool.Name, resultText(res))
		}
		if res := invalidArgumentValuesError(tool, map[string]any{"spec": true}); res == nil {
			t.Errorf("%s accepted boolean spec", tool.Name)
		}
	}
}

// TestStrictArgs_SynonymSuggestion is go-slide-creator-dwkkf: the server's own
// instructions promise a did_you_mean, but show_pattern{pattern: "agenda"} got
// only "Accepted arguments: name" — "pattern" and "name" share neither spelling
// nor tokens, so neither existing rule fired.
func TestStrictArgs_SynonymSuggestion(t *testing.T) {
	res := unknownArgumentsError(mcpShowPatternTool(), map[string]any{"pattern": "agenda"})
	if res == nil {
		t.Fatal("an unknown argument must be rejected")
	}
	text := resultText(res)
	if !strings.Contains(text, "name") || !strings.Contains(text, "did you mean") {
		t.Errorf("expected a did-you-mean naming %q, got %s", "name", text)
	}
	if !strings.Contains(text, "did_you_mean") {
		t.Errorf("the fix must carry did_you_mean for a machine reader: %s", text)
	}
}

// TestSuggestArgName_SynonymsOnlyWhenAccepted: a synonym is offered only when
// the tool really takes the target, and never invents one.
func TestSuggestArgName_SynonymsOnlyWhenAccepted(t *testing.T) {
	if got := suggestArgName("pattern", []string{"name", "fields"}); got != "name" {
		t.Errorf("suggestArgName(pattern) = %q, want name", got)
	}
	if got := suggestArgName("pattern", []string{"brief", "audience"}); got != "" {
		t.Errorf("a synonym whose target the tool does not accept must not be suggested, got %q", got)
	}
	if got := suggestArgName("prompt", []string{"brief", "slide_budget"}); got != "brief" {
		t.Errorf("suggestArgName(prompt) = %q, want brief", got)
	}
	// Spelling and token rules still win before the table is consulted.
	if got := suggestArgName("slide_count", []string{"slide_budget", "brief"}); got != "slide_budget" {
		t.Errorf("token overlap should still decide: got %q", got)
	}
}
