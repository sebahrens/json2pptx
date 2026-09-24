package patterns

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Keep both human-facing catalogs aligned with the live registry. A missing
// row makes a registered pattern invisible to authors reading the guides.
func TestPatternDocumentationMatchesRegistry(t *testing.T) {
	want := make([]string, 0)
	for _, pattern := range Default().List() {
		want = append(want, pattern.Name())
	}
	slices.Sort(want)
	for _, tc := range []struct {
		name, path, heading string
	}{
		{"CLAUDE.md", filepath.Join("..", "..", "CLAUDE.md"), "### Named Patterns (registered in `internal/patterns/`)"},
		{"PATTERNS.md", filepath.Join("..", "..", "skills", "generate-deck", "PATTERNS.md"), "### Registered Pattern Index"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			got := patternCatalogNames(t, string(data), tc.heading)
			if !slices.Equal(got, want) {
				t.Errorf("documented patterns differ from registry\nmissing from docs: %v\nnot registered: %v", patternNameDifference(want, got), patternNameDifference(got, want))
			}
		})
	}
}

func patternCatalogNames(t *testing.T, doc, heading string) []string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start := slices.Index(lines, heading)
	if start < 0 {
		t.Fatalf("missing pattern catalog heading %q", heading)
	}
	var names []string
	seen := map[string]bool{}
	inTable := false
	for _, line := range lines[start+1:] {
		if !inTable {
			if strings.HasPrefix(line, "| Pattern |") {
				inTable = true
			}
			continue
		}
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			t.Fatalf("malformed pattern catalog row %q", line)
		}
		name := strings.TrimSpace(cells[1])
		if strings.HasPrefix(name, "---") {
			continue
		}
		if !strings.HasPrefix(name, "`") || !strings.HasSuffix(name, "`") {
			t.Fatalf("pattern catalog row has no backticked name: %q", line)
		}
		name = strings.Trim(name, "`")
		if seen[name] {
			t.Errorf("duplicate pattern %q in catalog", name)
		}
		seen[name] = true
		names = append(names, name)
	}
	if !inTable || len(names) == 0 {
		t.Fatal("pattern catalog table is empty or missing")
	}
	slices.Sort(names)
	return names
}

func patternNameDifference(a, b []string) []string {
	var diff []string
	for _, name := range a {
		if !slices.Contains(b, name) {
			diff = append(diff, name)
		}
	}
	return diff
}
