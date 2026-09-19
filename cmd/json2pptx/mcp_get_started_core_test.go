package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestGetStartedBriefToolsAreCore guards against get_started steering agents
// toward tools the default core profile does not advertise: MCP clients can
// only call tools they see in tools/list.
func TestGetStartedBriefToolsAreCore(t *testing.T) {
	core := coreToolSet()
	for _, task := range []string{"brief", "validate-only"} {
		resp := buildGetStartedResponse(task)
		if resp.FastPath != nil {
			if !core[resp.FastPath.Tool] {
				t.Errorf("task=%s fast_path tool %q not in core profile", task, resp.FastPath.Tool)
			}
		}
		for _, step := range resp.Sequence {
			if !core[step.Tool] {
				t.Errorf("task=%s sequence tool %q not in core profile", task, step.Tool)
			}
		}
	}
}

// TestCoreToolDescriptionsNameOnlyCoreTools guards the pre-call surface: a core
// tool's description (and its parameter descriptions) must not name a tool the
// core profile hides. Descriptions are the only workflow guidance an agent has
// before its first call, so a description built around an unadvertised tool
// sends it after something it can never call (go-slide-creator-mvny: repair_slide
// pointed at propose_repairs, list_templates at get_data_format_hints, and
// get_started's own description at auto_repair / make_deck / read_presentation).
func TestCoreToolDescriptionsNameOnlyCoreTools(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore)
	_, advertised := listToolsOverWire(t, s)

	hidden := hiddenToolNames(t)
	core := coreToolSet()
	for _, tool := range advertised {
		if !core[tool.Name] {
			t.Errorf("core tools/list advertises %q, which is not a core tool", tool.Name)
			continue
		}
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("marshal tool %s: %v", tool.Name, err)
		}
		text := string(raw)
		for _, name := range hidden {
			if strings.Contains(text, name) {
				t.Errorf("core tool %s names hidden tool %s in its advertised definition — an agent in this profile cannot call it", tool.Name, name)
			}
		}
	}
}

// hiddenToolNames returns every registered tool the core profile does not
// advertise, longest name first so a substring check reports the specific tool
// (get_data_format_hints) rather than a shorter name contained in it.
func hiddenToolNames(t *testing.T) []string {
	t.Helper()
	all := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	_, tools := listToolsOverWire(t, all)
	core := coreToolSet()
	var hidden []string
	for _, tool := range tools {
		if !core[tool.Name] {
			hidden = append(hidden, tool.Name)
		}
	}
	sort.Slice(hidden, func(i, j int) bool { return len(hidden[i]) > len(hidden[j]) })
	return hidden
}
