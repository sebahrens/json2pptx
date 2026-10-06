package main

import (
	"path/filepath"
	"sort"
	"testing"
)

// TestShippedExamplesValidate is the CI validate sweep over every shipped
// example deck (go-slide-creator-csclk.68): examples/diagrams/donut_chart.json
// carried raw hex chart colors that constrained mode refuses, and nothing
// validated that directory. Users copy these files, so each must pass
// `json2pptx validate` without errors.
//
// The sweep is every deck, about a quarter of an hour of validation under
// -race. The short race run validates every third deck in name order — the
// concurrent validation -race is there to watch, on a third of the files — and
// the integration corpus job validates each one (go-slide-creator-efhg2).
func TestShippedExamplesValidate(t *testing.T) {
	// Read-only inputs; writes go only to t.TempDir() (go-slide-creator-s2s53).
	t.Parallel()
	var paths []string
	for _, pattern := range []string{"*.json", filepath.Join("diagrams", "*.json")} {
		matches, err := filepath.Glob(filepath.Join("..", "..", "examples", pattern))
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		t.Fatal("no example decks found")
	}
	sort.Strings(paths)
	for i, path := range paths {
		if testing.Short() && i%3 != 0 {
			continue
		}
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
			if !result.Valid {
				t.Fatalf("%s does not validate: %v", path, result.Errors)
			}
		})
	}
}
