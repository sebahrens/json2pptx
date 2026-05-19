package main

// Drift test for skills/generate-deck/.
//
// The skill is the agent-facing contract: it tells deck-generating agents
// which MCP tools exist and which repair-fix kinds repair_slide accepts. When
// the engine ships a new tool or fix kind but the skill is not updated, agents
// silently keep using the old surface. This test makes that drift loud.
//
// The skill was split out of a 1000+ line SKILL.md into focused sub-files
// (WORKFLOW.md, FINDINGS.md, RULES.md, PATTERNS.md). Drift is enforced against
// the concatenated text of every .md file in the skill directory — any of them
// counts as "documented".
//
// What is enforced:
//   - Every tool name returned by mcpToolNames() (the source of truth for
//     get_capabilities().mcp_tools_available) appears at least once in the
//     skill bundle.
//   - Every fix kind returned by repairFixKinds() (the source of truth for
//     get_capabilities().vocabularies.repair_fix_kinds and the
//     applyRepairFix switch in mcp_repair.go) appears at least once in the
//     skill bundle.
//   - The TEMPLATE_GUIDE.md cross-reference resolves to a real file.
//
// What is NOT enforced:
//   - Where a tool or kind is documented (no file or table-position requirement).
//   - Editorial voice / formatting — only presence.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// skillMdPaths returns the canonical SKILL.md locations to enforce drift
// against. The first is the repo-canonical authoritative copy; the second is
// the Claude Code-mounted copy that agents actually load. Both must stay in
// sync.
func skillMdPaths(t *testing.T) []string {
	t.Helper()
	// Resolve from the test's working directory (cmd/json2pptx) up to the
	// repo root.
	repoRoot := filepath.Join("..", "..")
	return []string{
		filepath.Join(repoRoot, "skills", "generate-deck", "SKILL.md"),
		filepath.Join(repoRoot, ".claude", "skills", "generate-deck", "SKILL.md"),
	}
}

// readSkillBundle returns the concatenated text of every .md file in the same
// directory as skillPath. Drift checks scan the whole bundle so MCP tool
// names and fix kinds may be documented in SKILL.md or any of its sibling
// sub-files (WORKFLOW.md, FINDINGS.md, RULES.md, PATTERNS.md).
func readSkillBundle(t *testing.T, skillPath string) string {
	t.Helper()
	dir := filepath.Dir(skillPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		mdFiles = append(mdFiles, filepath.Join(dir, e.Name()))
	}
	sort.Strings(mdFiles)
	var b strings.Builder
	for _, f := range mdFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestSkillMdMentionsEveryMCPTool(t *testing.T) {
	tools := mcpToolNames()
	if len(tools) == 0 {
		t.Fatal("mcpToolNames() returned no tools — capability catalog is empty")
	}

	for _, skillPath := range skillMdPaths(t) {
		skillPath := skillPath
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(skillPath)))+"/"+filepath.Base(filepath.Dir(skillPath)), func(t *testing.T) {
			if _, err := os.Stat(skillPath); os.IsNotExist(err) {
				t.Skipf("%s does not exist — skipping drift check", skillPath)
				return
			}
			body := readSkillBundle(t, skillPath)
			var missing []string
			for _, name := range tools {
				// Backtick-fenced is the canonical citation form; bare match
				// is also accepted so prose mentions count.
				if strings.Contains(body, "`"+name+"`") || strings.Contains(body, name) {
					continue
				}
				missing = append(missing, name)
			}
			if len(missing) > 0 {
				t.Errorf("%s bundle is missing %d MCP tool name(s): %v\n"+
					"Every tool registered in mcpToolCatalog() (cmd/json2pptx/mcp_capabilities.go) "+
					"must appear at least once in some .md file under the skill directory. "+
					"Update SKILL.md (or WORKFLOW.md / FINDINGS.md / RULES.md / PATTERNS.md) or remove the tool.",
					skillPath, len(missing), missing)
			}
		})
	}
}

func TestSkillMdMentionsEveryRepairFixKind(t *testing.T) {
	kinds := repairFixKinds()
	if len(kinds) == 0 {
		t.Fatal("repairFixKinds() returned no kinds — repair vocabulary is empty")
	}

	for _, skillPath := range skillMdPaths(t) {
		skillPath := skillPath
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(skillPath)))+"/"+filepath.Base(filepath.Dir(skillPath)), func(t *testing.T) {
			if _, err := os.Stat(skillPath); os.IsNotExist(err) {
				t.Skipf("%s does not exist — skipping drift check", skillPath)
				return
			}
			body := readSkillBundle(t, skillPath)
			var missing []string
			for _, kind := range kinds {
				if strings.Contains(body, "`"+kind+"`") || strings.Contains(body, kind) {
					continue
				}
				missing = append(missing, kind)
			}
			if len(missing) > 0 {
				t.Errorf("%s bundle is missing %d repair_slide fix kind(s): %v\n"+
					"Every case in applyRepairFix (cmd/json2pptx/mcp_repair.go) — also returned by "+
					"repairFixKinds() — must appear at least once in some .md file under the skill "+
					"directory (typically FINDINGS.md's repair_slide fix-kinds table). "+
					"Add the missing kind(s) or remove from applyRepairFix.",
					skillPath, len(missing), missing)
			}
		})
	}
}

func TestSkillMdTemplateGuideCrossRefResolves(t *testing.T) {
	for _, skillPath := range skillMdPaths(t) {
		skillPath := skillPath
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(skillPath)))+"/"+filepath.Base(filepath.Dir(skillPath)), func(t *testing.T) {
			if _, err := os.Stat(skillPath); os.IsNotExist(err) {
				t.Skipf("%s does not exist — skipping cross-ref check", skillPath)
				return
			}
			body := readSkillBundle(t, skillPath)

			// All TEMPLATE_GUIDE.md cross-refs must point to a real file.
			// The canonical form is ../template-deck/TEMPLATE_GUIDE.md
			// (relative to skills/generate-deck/<file>.md).
			const expected = "../template-deck/TEMPLATE_GUIDE.md"
			if !strings.Contains(body, expected) {
				t.Errorf("%s bundle does not mention TEMPLATE_GUIDE.md via the canonical relative path %q",
					skillPath, expected)
				return
			}
			// Resolve from SKILL.md's directory.
			resolved := filepath.Join(filepath.Dir(skillPath), expected)
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("cross-ref %q from %s does not resolve to a real file (looked at %s): %v",
					expected, skillPath, resolved, err)
			}
		})
	}
}
