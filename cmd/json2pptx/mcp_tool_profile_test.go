package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/patterns"
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
	allServer := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	allRaw, allTools := listToolsOverWire(t, allServer)
	t.Logf("all profile: %d tools, %d bytes", len(allTools), len(allRaw))
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
		"render_deck_thumbnails", "generate_presentation",
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
	var want []string
	for _, name := range mcpToolNames() {
		if _, folded := foldedTools[name]; !folded {
			want = append(want, name)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("all profile advertises %d tools, want the %d unfolded catalog tools", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("all profile tool[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(s.ListTools()) != len(mcpToolNames()) {
		t.Fatalf("registered %d tools, want %d", len(s.ListTools()), len(mcpToolNames()))
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
	for in, want := range map[string]string{"": "deckspec", "deckspec": "deckspec", "core": "core", "raw": "core", "ALL": "all", " all ": "all"} {
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

// TestDeckSpecToolProfileBudget is the go-slide-creator-355t7 acceptance test:
// the default deckspec profile advertises at most deckSpecToolLimit tools in a
// tools/list under deckSpecToolListByteBudget (~10K tokens), and
// validate_deck_spec carries the outline, not the closed per-kind schema.
func TestDeckSpecToolProfileBudget(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileDeckSpec)
	raw, tools := listToolsOverWire(t, s)
	if len(tools) > deckSpecToolLimit {
		t.Errorf("deckspec profile advertises %d tools, want <= %d", len(tools), deckSpecToolLimit)
	}
	if len(tools) != len(deckSpecToolNames) {
		t.Errorf("deckspec profile advertised %d tools but deckSpecToolNames has %d — a name is not registered", len(tools), len(deckSpecToolNames))
	}
	if len(raw) >= deckSpecToolListByteBudget {
		t.Errorf("deckspec tools/list is %d bytes, want < %d", len(raw), deckSpecToolListByteBudget)
	}
	t.Logf("deckspec profile: %d tools, %d bytes", len(tools), len(raw))
	core := coreToolSet()
	for _, tool := range tools {
		if !core[tool.Name] {
			t.Errorf("deckspec tool %q is not a core tool; the profiles must nest", tool.Name)
		}
		if tool.RawOutputSchema != nil || tool.OutputSchema.Type != "" {
			t.Errorf("deckspec tool %q still advertises an outputSchema", tool.Name)
		}
		if tool.Name != "validate_deck_spec" {
			continue
		}
		// Read the wire bytes: mcp.Tool's decoder keeps only the structured
		// schema fields and would drop oneOf.
		var wire struct {
			Result struct {
				Tools []struct {
					Name        string         `json:"name"`
					InputSchema map[string]any `json:"inputSchema"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("decode tools/list: %v", err)
		}
		var schema map[string]any
		for _, w := range wire.Result.Tools {
			if w.Name == tool.Name {
				schema = w.InputSchema
			}
		}
		if _, ok := schema["$defs"]; ok {
			t.Error("deckspec validate_deck_spec still carries the closed schema's $defs")
		}
		spec, _ := schema["properties"].(map[string]any)["spec"].(map[string]any)
		desc, _ := spec["description"].(string)
		if !strings.Contains(desc, "item_schema") || !strings.Contains(desc, "list_slide_kinds") {
			t.Errorf("deckspec validate_deck_spec.spec does not point at list_slide_kinds item_schema: %q", desc)
		}
		if _, ok := schema["oneOf"]; !ok {
			t.Error("deckspec validate_deck_spec lost the spec/deck_id choice")
		}
	}
}

// deckSpecContextMentions are the only places a deckspec tool may name a tool
// the profile hides: provenance of a shared argument, not a recommendation to
// call it.
var deckSpecContextMentions = map[string][]string{
	// pptx_path names where each render path reports its artifact
	// (TestMCPDescriptionsMatchRuntimeContracts pins it).
	"render_deck_thumbnails": {"generate_presentation"},
}

// TestDeckSpecToolDescriptionsNameOnlyAdvertisedTools is the deckspec twin of
// TestCoreToolDescriptionsNameOnlyCoreTools: a client model cannot call a tool
// absent from tools/list, so an advertised definition must not send it there.
func TestDeckSpecToolDescriptionsNameOnlyAdvertisedTools(t *testing.T) {
	withToolProfile(t, toolProfileDeckSpec)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileDeckSpec)
	_, advertised := listToolsOverWire(t, s)
	set := deckSpecToolSet()
	names := mcpToolNames()
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, tool := range advertised {
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		// Blank advertised names first so a hidden name that is a substring
		// of one (render_slide_image in render_slide_image_from_json) does
		// not match inside it.
		for _, name := range names {
			if set[name] {
				text = strings.ReplaceAll(text, name, "")
			}
		}
		allowed := toolSet(deckSpecContextMentions[tool.Name])
		for _, name := range names {
			if set[name] || !strings.Contains(text, name) {
				continue
			}
			text = strings.ReplaceAll(text, name, "")
			if !allowed[name] {
				t.Errorf("deckspec tool %s names hidden tool %s in its advertised definition", tool.Name, name)
			}
		}
	}
}

// TestFoldedToolsAreHiddenAliases pins go-slide-creator-fa3k8: a folded tool is
// advertised by no profile, stays callable by its old name, its successor is
// advertised, and a next_tool_call naming it is retargeted to the successor.
func TestFoldedToolsAreHiddenAliases(t *testing.T) {
	registered := toolSet(mcpToolNames())
	for _, profile := range []string{toolProfileDeckSpec, toolProfileCore, toolProfileAll} {
		withToolProfile(t, profile)
		s := newJSON2PPTXMCPServer(profileTestConfig(t), profile)
		_, tools := listToolsOverWire(t, s)
		advertised := map[string]bool{}
		for _, tool := range tools {
			advertised[tool.Name] = true
			if _, folded := foldedTools[tool.Name]; folded {
				t.Errorf("%s profile advertises folded tool %s", profile, tool.Name)
			}
		}
		for name, successor := range foldedTools {
			if !registered[name] {
				t.Errorf("folded tool %s is no longer registered; old clients calling it would break", name)
			}
			if profile == toolProfileDeckSpec {
				continue
			}
			if !advertised[successor] {
				t.Errorf("%s profile hides %s's successor %s", profile, name, successor)
			}
			sub := substituteUnadvertised(&patterns.ToolCallSuggestion{Tool: name, ArgsTemplate: map[string]any{"pptx_path": "/tmp/x.pptx", "slide_index": 2}})
			if sub == nil || sub.Tool != successor {
				t.Errorf("%s profile: next_tool_call %s retargets to %v, want %s", profile, name, sub, successor)
			}
		}
	}
}

// TestToolDescriptionsWithinCap enforces go-slide-creator-fa3k8's description
// cap across every advertised tool: long catalogues belong in describe tools
// (describe_finding, get_capabilities vocabularies), not in tools/list.
func TestToolDescriptionsWithinCap(t *testing.T) {
	const maxDescriptionBytes = 1536
	for _, profile := range []string{toolProfileDeckSpec, toolProfileCore, toolProfileAll} {
		withToolProfile(t, profile)
		s := newJSON2PPTXMCPServer(profileTestConfig(t), profile)
		_, tools := listToolsOverWire(t, s)
		for _, tool := range tools {
			if n := len(tool.Description); n > maxDescriptionBytes {
				t.Errorf("%s profile: %s description is %d bytes, cap %d", profile, tool.Name, n, maxDescriptionBytes)
			}
		}
	}
}
