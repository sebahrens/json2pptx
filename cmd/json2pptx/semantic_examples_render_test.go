package main

import (
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBundledSemanticExamplesRenderCorpus follows semantic validate through
// compilation, scratch rendering and its error-severity findings, not Check alone.
func TestBundledSemanticExamplesRenderCorpus(t *testing.T) {
	root := filepath.Join(testutil.RepoRoot(), "examples", "semantic")
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".yaml") || strings.Contains(entry.Name(), "invalid") {
			return nil
		}
		count++
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var evalErr error
			eval, ds := evaluateSpecFindings(path, data, semantic.StrictnessWarn, func(spec *semantic.DeckSpec, _ []byte) specEvaluation {
				if spec.Meta.Template == "p-style" {
					if _, err := os.Stat(filepath.Join(testutil.TemplatesDir(), "p-style.pptx")); os.IsNotExist(err) {
						t.Skip("optional p-style example requires local template")
					}
				}
				fallback := ""
				if spec.Meta.Template == "" {
					fallback = "midnight-blue"
				}
				e, err := evaluateSpecCLI(path, spec, semantic.StrictnessWarn, fallback, testutil.TemplatesDir())
				evalErr = err
				return e
			})
			if evalErr != nil {
				t.Fatal(evalErr)
			}
			if !eval.Evaluated || eval.CompileFailed {
				t.Errorf("example did not reach scratch-render evaluation: %+v", eval)
			}
			if diagnostics.HasErrors(ds) {
				t.Errorf("semantic validate render-stage findings: %+v", ds)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no semantic examples discovered")
	}
}
