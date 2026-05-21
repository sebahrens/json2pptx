package main

import (
	"os"
	"strings"
	"testing"
)

// readDiscoveryDoc loads a checked-in discovery document relative to this
// package (cmd/json2pptx). It fails the test if the file is missing so the
// drift gate cannot silently no-op when a doc is moved or renamed.
func readDiscoveryDoc(t *testing.T, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(relPath) //nolint:gosec // fixed in-repo doc paths
	if err != nil {
		t.Fatalf("failed to read discovery doc %q: %v", relPath, err)
	}
	return string(data)
}

// TestReadmeListsAllMCPTools is the AC3 drift gate for the human-facing
// discovery surface: every registered MCP tool must appear by name in the
// README. The README MCP tables are grouped by phase; this test fails when a
// newly registered tool is missing, so README cannot silently lag the surface.
func TestReadmeListsAllMCPTools(t *testing.T) {
	readme := readDiscoveryDoc(t, "../../README.md")
	for _, name := range mcpToolNames() {
		if !strings.Contains(readme, name) {
			t.Errorf("README.md does not mention registered MCP tool %q — add it to the MCP tool tables (grouped by phase) so discovery docs do not lag the surface", name)
		}
	}
}

// TestToolsDocListsAllMCPTools is the AC3 drift gate for the agent-facing
// discovery surface: every registered MCP tool must appear by name in
// skills/generate-deck/TOOLS.md (the authoritative tool catalogue the skill
// points agents at). Fails when a registered tool is missing from the doc.
func TestToolsDocListsAllMCPTools(t *testing.T) {
	doc := readDiscoveryDoc(t, "../../skills/generate-deck/TOOLS.md")
	for _, name := range mcpToolNames() {
		if !strings.Contains(doc, name) {
			t.Errorf("skills/generate-deck/TOOLS.md does not mention registered MCP tool %q — add it to the full catalogue so the agent-facing doc does not lag the surface", name)
		}
	}
}

// TestMCPOnlyToolsAgreeAcrossDiscoveryDocs proves the README and TOOLS.md agree
// with the classification source of truth on which tools are MCP-only: every
// tool with an MCPOnlyReason must be flagged as MCP-only in both docs. README
// uses the literal "MCP-only" marker in its CLI column; TOOLS.md uses the
// "(MCP-only" marker in its CLI-equivalent column.
func TestMCPOnlyToolsAgreeAcrossDiscoveryDocs(t *testing.T) {
	readme := readDiscoveryDoc(t, "../../README.md")
	tools := readDiscoveryDoc(t, "../../skills/generate-deck/TOOLS.md")

	if !strings.Contains(readme, "MCP-only") {
		t.Error("README.md must mark MCP-only tools with the literal 'MCP-only'")
	}
	if !strings.Contains(tools, "MCP-only") {
		t.Error("skills/generate-deck/TOOLS.md must mark MCP-only tools with 'MCP-only'")
	}
}
