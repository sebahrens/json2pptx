package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// plan_deck{slide_count} must be rejected, naming the real slide_budget arg.
func TestStrictArgs_PlanDeckSlideCountSuggestsSlideBudget(t *testing.T) {
	s := strictArgsTestServer(t)
	res := callToolViaServer(t, s, "plan_deck", map[string]any{"brief": "Series B pitch", "slide_count": 8})
	if !res.IsError {
		t.Fatalf("plan_deck with slide_count must be rejected, got %s", resultText(res))
	}
	text := resultText(res)
	for _, want := range []string{"UNKNOWN_PARAMETER", "slide_count", "slide_budget", "did_you_mean"} {
		if !strings.Contains(text, want) {
			t.Errorf("error must mention %q, got %s", want, text)
		}
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
