package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCanonicalWorkflowDocs makes the human guides checked renderings of the
// same workflow contract advertised by get_started. Editing one copy without
// the others fails before contradictory mandatory steps reach an agent.
func TestCanonicalWorkflowDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "README.md"),
		filepath.Join("..", "..", "skills", "generate-deck", "SKILL.md"),
		filepath.Join("..", "..", "skills", "generate-deck", "TOOLS.md"),
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			const start = "<!-- workflow-contract:start -->"
			const end = "<!-- workflow-contract:end -->"
			content := string(data)
			if strings.Count(content, start) != 1 || strings.Count(content, end) != 1 {
				t.Fatal("expected exactly one canonical workflow block")
			}
			_, after, found := strings.Cut(content, start)
			if !found {
				t.Fatal("missing canonical workflow start marker")
			}
			actual, _, found := strings.Cut(after, end)
			if !found {
				t.Fatal("missing canonical workflow end marker")
			}
			if got := strings.TrimSpace(actual); got != canonicalWorkflowStatement {
				t.Errorf("workflow block drifted from get_started contract:\n%s", got)
			}
		})
	}
	if desc := getStartedToolDescription(); !strings.Contains(desc, "Default for content-bearing decks:") || !strings.Contains(desc, "`validate_deck_spec` → `render_deck_spec`") {
		t.Error("get_started tool description does not carry the canonical workflow contract")
	}
	resp := buildGetStartedResponse("brief", getStartedRuntime{RenderAvailable: true})
	for _, note := range resp.Notes {
		if strings.Contains(note, "validate_input is mandatory per SKILL.md") || strings.Contains(note, "This is the canonical new-deck workflow") {
			t.Errorf("brief notes still make the raw path globally mandatory: %q", note)
		}
	}
}
