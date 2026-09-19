package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// go-slide-creator-ccqn: every tool shipped mcp-go's NewTool() defaults —
// readOnlyHint:false, destructiveHint:true, idempotentHint:false,
// openWorldHint:true, no title — so get_started, get_capabilities,
// list_templates and every validate_* were advertised as destructive,
// non-idempotent and open-world. Hosts that gate destructive tools prompt on
// every discovery call; planners that avoid them avoid the tools the workflow
// starts with.
func TestToolAnnotationsMatchClassification(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	_, tools := listToolsOverWire(t, s)
	if len(tools) < 40 {
		t.Fatalf("expected the full catalog, got %d tools", len(tools))
	}

	classifications := toolClassifications()
	for _, tool := range tools {
		c, ok := classifications[tool.Name]
		if !ok {
			t.Errorf("%s has no classification, so its annotations cannot be derived", tool.Name)
			continue
		}
		a := tool.Annotations
		if a.Title == "" {
			t.Errorf("%s has no title annotation", tool.Name)
		}
		wantReadOnly := !c.MutatesState && (!c.WritesFiles || c.CacheWritesOnly)
		if got := boolValue(a.ReadOnlyHint); got != wantReadOnly {
			t.Errorf("%s readOnlyHint = %v, want %v (mutates=%v writes=%v cache_only=%v)",
				tool.Name, got, wantReadOnly, c.MutatesState, c.WritesFiles, c.CacheWritesOnly)
		}
		if got := boolValue(a.DestructiveHint); got != c.MutatesState {
			t.Errorf("%s destructiveHint = %v, want %v", tool.Name, got, c.MutatesState)
		}
		if got := boolValue(a.IdempotentHint); got != !c.MutatesState {
			t.Errorf("%s idempotentHint = %v, want %v", tool.Name, got, !c.MutatesState)
		}
		if got := boolValue(a.OpenWorldHint); got != c.APIKeyDependency {
			t.Errorf("%s openWorldHint = %v, want %v (api_key=%v)", tool.Name, got, c.APIKeyDependency, c.APIKeyDependency)
		}
	}
}

// The discovery and validation tools an agent calls first must not be
// advertised as destructive.
func TestDiscoveryToolsAreReadOnly(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	_, tools := listToolsOverWire(t, s)
	byName := map[string]mcp.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	for _, name := range []string{
		"get_started", "get_capabilities", "list_templates", "list_patterns",
		"show_pattern", "describe_finding", "list_slide_kinds", "validate_input",
		"validate_deck_spec", "validate_presentation_output", "score_deck",
		"analyze_deck_rhythm", "recommend_visual", "preview_presentation_plan",
		"plan_deck", "expand_pattern",
	} {
		tool, ok := byName[name]
		if !ok {
			t.Errorf("%s is not registered", name)
			continue
		}
		if !boolValue(tool.Annotations.ReadOnlyHint) {
			t.Errorf("%s is not advertised read-only; a host that gates destructive tools will prompt for it", name)
		}
		if boolValue(tool.Annotations.DestructiveHint) {
			t.Errorf("%s is advertised destructive", name)
		}
	}

	// Only the settings writers are destructive.
	for _, tool := range tools {
		if !boolValue(tool.Annotations.DestructiveHint) {
			continue
		}
		switch tool.Name {
		case "register_template_setting", "delete_template_setting":
		default:
			t.Errorf("%s is advertised destructive, but only the template-settings writers mutate server state", tool.Name)
		}
	}
}

// The annotations must survive serialization to the client.
func TestToolAnnotationsSerialize(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore)
	raw, _ := listToolsOverWire(t, s)
	var env struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	for _, tool := range env.Result.Tools {
		ann, ok := tool["annotations"].(map[string]any)
		if !ok {
			t.Fatalf("%v carries no annotations object", tool["name"])
		}
		if _, ok := ann["title"].(string); !ok {
			t.Errorf("%v has no title in the serialized annotations", tool["name"])
		}
	}
	_ = context.Background()
}

func boolValue(p *bool) bool {
	return p != nil && *p
}
