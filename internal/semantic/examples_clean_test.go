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
