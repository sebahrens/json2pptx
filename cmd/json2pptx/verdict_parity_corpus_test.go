//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// TestVerdictParityCorpus is go-slide-creator-9k5fh over the example corpus:
// validate_input, generate --dry-run, generate and generate_presentation give
// one verdict for every examples/*.json, on the template the example names.
// An example is a deck the repository ships as working, so that verdict is
// "valid": a check that rejects one is a check generation does not make.
func TestVerdictParityCorpus(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "examples", "*.json"))
	if err != nil || len(examples) == 0 {
		t.Fatalf("find example corpus: %v (%d files)", err, len(examples))
	}
	mc := testMCPConfig(t)
	for _, examplePath := range examples {
		t.Run(strings.TrimSuffix(filepath.Base(examplePath), ".json"), func(t *testing.T) {
			verdict := assertOneVerdict(t, verdictSurfaces(t, mc, examplePath, true))
			if !verdict.Valid {
				t.Errorf("a shipped example is refused: %s", verdict)
			}
		})
	}
}
