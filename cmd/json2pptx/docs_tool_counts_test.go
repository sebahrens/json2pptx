package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Documented counts drift (go-slide-creator-zi17).
//
// README said "51 MCP tools" and "4 bundled templates" while the wire showed 52
// and 9, and SKILL.md enumerated a core profile that had since gained
// submit_visual_review. The skill now routes discovery to the live registry
// instead of copying its changing names.

// readRepoFile reads a file relative to the repository root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestSkillCoreProfileUsesRuntimeDiscovery prevents a static core profile
// list from drifting away from the server's live registry.
func TestSkillCoreProfileUsesRuntimeDiscovery(t *testing.T) {
	skill := readRepoFile(t, filepath.Join("skills", "generate-deck", "SKILL.md"))
	if len(coreToolSet()) == 0 {
		t.Fatal("core profile is empty")
	}
	for _, want := range []string{"get_capabilities().mcp_tools_available", "--tools all", "tools/list"} {
		if !strings.Contains(skill, want) {
			t.Errorf("SKILL.md must route core-profile discovery through %q", want)
		}
	}
	if strings.Contains(skill, "core tools (") {
		t.Error("SKILL.md reintroduced a static core-tool list")
	}
}

// TestDocsDoNotHardcodeToolOrTemplateCounts: a number in prose goes stale the
// next time a tool or template is added. The docs point at the live source
// instead.
func TestDocsDoNotHardcodeToolOrTemplateCounts(t *testing.T) {
	stale := []struct{ file, phrase string }{
		{"README.md", "51 MCP tools"},
		{"README.md", "4 bundled templates"},
		{filepath.Join("skills", "generate-deck", "TOOLS.md"), "~20 tools"},
		{"CLAUDE.md", "4 bundled templates"},
	}
	for _, s := range stale {
		if strings.Contains(readRepoFile(t, s.file), s.phrase) {
			t.Errorf("%s still says %q; point at the live source instead", s.file, s.phrase)
		}
	}
}

// TestBundledTemplatesAreAllDocumented: every embedded built-in is named
// where the docs list the bundled set, so an agent cannot be told a template
// does not exist.
func TestBundledTemplatesAreAllDocumented(t *testing.T) {
	claude := readRepoFile(t, "CLAUDE.md")
	readme := readRepoFile(t, "README.md")
	for _, name := range testutil.AllBuiltinTemplateNames() {
		if !strings.Contains(claude, "`"+name+"`") {
			t.Errorf("template %q ships but CLAUDE.md does not name it", name)
		}
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("template %q ships but README.md does not name it", name)
		}
	}
}
