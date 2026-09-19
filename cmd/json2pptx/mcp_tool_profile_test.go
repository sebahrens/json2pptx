package main

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/template"
)

// listToolsOverWire drives initialize + tools/list through the real JSON-RPC
// handler (so tool filters apply) and returns the raw result bytes.
func listToolsOverWire(t *testing.T, s *server.MCPServer) ([]byte, []mcp.Tool) {
	t.Helper()
	ctx := context.Background()
	s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	resp := s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	return raw, env.Result.Tools
}

func profileTestConfig(t *testing.T) *mcpConfig {
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

// TestCoreToolProfileBudget is the go-slide-creator-vdxa acceptance test: the
// default core profile advertises <= coreToolLimit tools and a marshalled
// tools/list under coreToolListByteBudget.
func TestCoreToolProfileBudget(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore)
	raw, tools := listToolsOverWire(t, s)
	if len(tools) > coreToolLimit {
		t.Errorf("core profile advertises %d tools, want <= %d", len(tools), coreToolLimit)
	}
	if len(raw) >= coreToolListByteBudget {
		t.Errorf("core tools/list is %d bytes, want < %d", len(raw), coreToolListByteBudget)
	}
	t.Logf("core profile: %d tools, %d bytes", len(tools), len(raw))
	if len(tools) != len(coreToolNames) {
		t.Errorf("core profile advertised %d tools but coreToolNames has %d — a core name is not registered", len(tools), len(coreToolNames))
	}
	for _, tool := range tools {
		if tool.RawOutputSchema != nil || tool.OutputSchema.Type != "" {
			t.Errorf("core tool %q still advertises an outputSchema", tool.Name)
		}
	}
}

// TestCoreToolProfile_RequiredTools pins the tools every workflow in SKILL.md
// depends on.
func TestCoreToolProfile_RequiredTools(t *testing.T) {
	core := coreToolSet()
	for _, name := range []string{
		"render_deck_spec", "validate_deck_spec", "list_slide_kinds", "get_started",
		"render_deck_thumbnails", "render_slide_image", "generate_presentation",
		"validate_input", "repair_slide", "score_deck", "list_patterns", "show_pattern",
		"expand_pattern", "recommend_visual", "plan_deck", "list_templates",
	} {
		if !core[name] {
			t.Errorf("core profile missing required tool %q", name)
		}
	}
}

// TestAllToolProfile_FullSurface verifies --tools=all advertises every
// registered tool with its outputSchema intact.
func TestAllToolProfile_FullSurface(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	_, tools := listToolsOverWire(t, s)
	got := make([]string, 0, len(tools))
	for _, tool := range tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	want := mcpToolNames()
	if len(got) != len(want) {
		t.Fatalf("all profile advertises %d tools, catalog has %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("all profile tool[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(s.ListTools()) != len(want) {
		t.Fatalf("registered %d tools, want %d", len(s.ListTools()), len(want))
	}
}

// TestCoreToolProfile_HiddenToolsStillCallable: the profile only filters
// tools/list; a non-core tool invoked by name still dispatches.
func TestCoreToolProfile_HiddenToolsStillCallable(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore)
	if coreToolSet()["list_deck_archetypes"] {
		t.Skip("list_deck_archetypes promoted to core; pick another hidden tool")
	}
	ctx := context.Background()
	s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	resp := s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_deck_archetypes","arguments":{}}}`))
	raw, _ := json.Marshal(resp)
	var env struct {
		Result *mcp.CallToolResult `json:"result"`
		Error  any                 `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.Error != nil || env.Result == nil || env.Result.IsError {
		t.Fatalf("hidden tool call failed: %s", raw)
	}
}

func TestParseToolProfile(t *testing.T) {
	for in, want := range map[string]string{"": "core", "core": "core", "ALL": "all", " all ": "all"} {
		got, err := parseToolProfile(in)
		if err != nil || got != want {
			t.Errorf("parseToolProfile(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := parseToolProfile("everything"); err == nil {
		t.Error("expected error for unknown profile")
	}
	t.Setenv(toolProfileEnv, "all")
	if got, _ := resolveToolProfile("core", false); got != "all" {
		t.Errorf("env should apply when flag unset, got %q", got)
	}
	if got, _ := resolveToolProfile("core", true); got != "core" {
		t.Errorf("explicit flag should win over env, got %q", got)
	}
}

func TestCapabilitiesInCoreProfileFlag(t *testing.T) {
	core := coreToolSet()
	var n int
	for _, e := range mcpToolCatalog() {
		if e.InCoreProfile != core[e.Name] {
			t.Errorf("%s: in_core_profile=%v, want %v", e.Name, e.InCoreProfile, core[e.Name])
		}
		if e.InCoreProfile {
			n++
		}
	}
	if n != len(coreToolNames) {
		t.Errorf("%d catalog entries flagged in_core_profile, want %d", n, len(coreToolNames))
	}
}
