package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Documented counts drift (go-slide-creator-zi17).
//
// README said "51 MCP tools" and "4 bundled templates" while the wire showed 52
// and 9, and SKILL.md enumerated a core profile that had since gained
// submit_visual_review — the tool the no-vision completion path depends on. A
// test that only forbids hardcoded numbers does not catch an enumerated list
// going stale, so these read the documents and compare them with the registry.

// readRepoFile reads a file relative to the repository root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// coreProfileTools lists the core profile's tool names, sorted.
func coreProfileTools() []string {
	set := coreToolSet()
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// skillCoreListRE captures the backticked names SKILL.md enumerates for the
// core profile, and the count it claims.
var skillCoreListRE = regexp.MustCompile(`tools/list` + "`" + ` advertises only the (\d+) core tools \(([^)]*)\)`)

// TestSkillCoreToolListMatchesTheProfile: the enumerated list and its count
// must equal what the server actually advertises.
func TestSkillCoreToolListMatchesTheProfile(t *testing.T) {
	skill := readRepoFile(t, filepath.Join("skills", "generate-deck", "SKILL.md"))
	m := skillCoreListRE.FindStringSubmatch(skill)
	if m == nil {
		t.Fatal("SKILL.md no longer enumerates the core profile in the expected form; update this test with it")
	}
	claimed, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("parse claimed count %q: %v", m[1], err)
	}

	documented := map[string]bool{}
	for _, tok := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(m[2], -1) {
		documented[tok[1]] = true
	}

	want := coreProfileTools()
	if claimed != len(want) {
		t.Errorf("SKILL.md says %d core tools; the profile has %d", claimed, len(want))
	}
	if len(documented) != len(want) {
		t.Errorf("SKILL.md enumerates %d names but claims %d and the profile has %d", len(documented), claimed, len(want))
	}
	for _, name := range want {
		if !documented[name] {
			t.Errorf("core tool %q is missing from SKILL.md's list", name)
		}
		delete(documented, name)
	}
	for name := range documented {
		t.Errorf("SKILL.md lists %q as core, but the profile does not", name)
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

// TestBundledTemplatesAreAllDocumented: every .pptx in templates/ is named
// where the docs list the bundled set, so an agent cannot be told a template
// does not exist.
func TestBundledTemplatesAreAllDocumented(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}
	claude := readRepoFile(t, "CLAUDE.md")
	readme := readRepoFile(t, "README.md")
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".pptx" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".pptx")
		if !strings.Contains(claude, "`"+name+"`") {
			t.Errorf("template %q ships but CLAUDE.md does not name it", name)
		}
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("template %q ships but README.md does not name it", name)
		}
	}
}
