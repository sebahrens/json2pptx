package main

import (
	"path/filepath"
	"testing"
)

// TestShippedExamplesValidate is the CI validate sweep over every shipped
// example deck (go-slide-creator-csclk.68): examples/diagrams/donut_chart.json
// carried raw hex chart colors that constrained mode refuses, and nothing
// validated that directory. Users copy these files, so each must pass
// `json2pptx validate` without errors.
func TestShippedExamplesValidate(t *testing.T) {
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
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
			if !result.Valid {
				t.Fatalf("%s does not validate: %v", path, result.Errors)
			}
		})
	}
}
