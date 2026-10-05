package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// TestBundledSemanticExamplesValidateClean is the regression guard for the
// chart_insight field report (go-slide-creator-6wss.2): every bundled positive
// semantic example under examples/semantic/ must validate with no error-severity
// findings at the default strictness, so agents copying an official example get
// a clean signal. The canonical qbr example — which uses the documented
// chart.data.series chart payload shape — must additionally pass strict
// validation, where advisory findings are promoted to errors.
func TestBundledSemanticExamplesValidateClean(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "semantic")
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("no semantic examples found under %s", dir)
	}
	// The wiki playbooks (docs/wiki/) are complete decks agents copy; they are
	// held to the same bar.
	playbooks, err := filepath.Glob(filepath.Join(dir, "playbooks", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(playbooks) == 0 {
		t.Fatalf("no playbook examples found under %s", filepath.Join(dir, "playbooks"))
	}
	matches = append(matches, playbooks...)

	for _, path := range matches {
		name := filepath.Base(path)
		// Negative fixtures (if any are bundled) intentionally carry errors.
		if strings.Contains(name, "invalid") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if ds := Check(name, data, StrictnessWarn); diagnostics.HasErrors(ds) {
				t.Fatalf("%s produced error findings at default strictness: %v", name, ds)
			}
			if name == "qbr.yaml" {
				if ds := Check(name, data, StrictnessStrict); diagnostics.HasErrors(ds) {
					t.Fatalf("qbr.yaml must pass strict validation (chart.data.series), got: %v", ds)
				}
			}
		})
	}
}

// TestDocSemanticSnippetsValidateClean guards the semantic quick-start YAML
// blocks embedded in README.md and docs/SEMANTIC_COMPILER.md (go-slide-creator-
// csclk.57): agents copy these verbatim, so every ```yaml block that declares
// top-level `meta:` + `slides:` must validate without error findings.
func TestDocSemanticSnippetsValidateClean(t *testing.T) {
	docs := []string{"README.md", filepath.Join("docs", "SEMANTIC_COMPILER.md")}
	// Every wiki page: agents copy their YAML blocks too.
	wiki, err := filepath.Glob(filepath.Join("..", "..", "docs", "wiki", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range wiki {
		docs = append(docs, filepath.Join("docs", "wiki", filepath.Base(page)))
	}
	for _, doc := range docs {
		data, err := os.ReadFile(filepath.Join("..", "..", doc))
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for i, block := range strings.Split(string(data), "```yaml\n")[1:] {
			end := strings.Index(block, "\n```")
			if end < 0 {
				continue
			}
			snippet := block[:end+1]
			if !strings.HasPrefix(snippet, "meta:") || !strings.Contains(snippet, "\nslides:") {
				continue
			}
			found++
			if ds := Check("snippet.yaml", []byte(snippet), StrictnessWarn); diagnostics.HasErrors(ds) {
				t.Errorf("%s yaml block %d produced error findings: %v", doc, i, ds)
			}
		}
		// A wiki page may be prose only (the hub, the journey); the two
		// canonical docs must keep their quick-start snippet.
		if found == 0 && !strings.HasPrefix(doc, filepath.Join("docs", "wiki")) {
			t.Errorf("%s: no semantic yaml snippet found", doc)
		}
	}
}
