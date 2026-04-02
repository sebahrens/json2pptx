package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShapeGridCrossTemplate runs all shape_grid integration fixtures across all
// available templates, verifying that each produces a valid PPTX file.
// This is Phase C of the shape grid stress test plan (spec 30).
func TestShapeGridCrossTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-template matrix test in short mode")
	}

	// Locate project root (cmd/json2pptx -> project root)
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	fixturesDir := filepath.Join(projectRoot, "tests", "integration", "json_fixtures")
	templatesDir := filepath.Join(projectRoot, "templates")

	// Find all shape_grid fixtures (prefixed with _sg_ or the original 47_)
	matches, err := filepath.Glob(filepath.Join(fixturesDir, "*_sg_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Also include the original shape_grid fixture
	original := filepath.Join(fixturesDir, "47_shape_grid.json")
	if _, err := os.Stat(original); err == nil {
		matches = append(matches, original)
	}

	if len(matches) == 0 {
		t.Fatal("no shape_grid fixtures found")
	}

	// Available templates
	templates := []string{"midnight-blue", "forest-green", "warm-coral", "modern-template"}

	// Verify templates exist
	for _, tmpl := range templates {
		tmplPath := filepath.Join(templatesDir, tmpl+".pptx")
		if _, err := os.Stat(tmplPath); err != nil {
			t.Fatalf("template not found: %s", tmplPath)
		}
	}

	// Create temp output directory
	outputDir := t.TempDir()

	for _, fixture := range matches {
		fixtureName := strings.TrimSuffix(filepath.Base(fixture), ".json")

		for _, tmpl := range templates {
			tmpl := tmpl // capture
			testName := fixtureName + "/" + tmpl
			t.Run(testName, func(t *testing.T) {
				t.Parallel()

				// Read and modify the fixture to use the target template
				data, err := os.ReadFile(fixture)
				if err != nil {
					t.Fatal(err)
				}

				var input map[string]interface{}
				if err := json.Unmarshal(data, &input); err != nil {
					t.Fatal(err)
				}
				input["template"] = tmpl
				outputFilename := fixtureName + "_" + tmpl + ".pptx"
				input["output_filename"] = outputFilename

				modifiedJSON, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}

				// Write modified fixture to temp file
				tmpJSON := filepath.Join(outputDir, fixtureName+"_"+tmpl+".json")
				if err := os.WriteFile(tmpJSON, modifiedJSON, 0644); err != nil {
					t.Fatal(err)
				}

				// Run through JSON mode (no JSON output file needed)
				jsonResultPath := filepath.Join(outputDir, fixtureName+"_"+tmpl+".result.json")
				err = runJSONMode(tmpJSON, jsonResultPath, templatesDir, outputDir, "", false, false, "")
				if err != nil {
					t.Fatalf("runJSONMode failed: %v", err)
				}

				// Validate output PPTX
				pptxPath := filepath.Join(outputDir, outputFilename)
				validatePPTX(t, pptxPath)
			})
		}
	}
}

// validatePPTX checks that a file is a valid PPTX (ZIP archive with required entries).
func validatePPTX(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PPTX file not found: %s", path)
	}
	if info.Size() < 10*1024 {
		t.Errorf("PPTX file too small (%d bytes): %s", info.Size(), path)
	}

	// Open as ZIP
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("not a valid ZIP/PPTX: %s: %v", path, err)
	}
	defer reader.Close()

	// Check required entries
	required := []string{"[Content_Types].xml", "ppt/presentation.xml"}
	found := make(map[string]bool)
	hasSlide := false
	for _, f := range reader.File {
		found[f.Name] = true
		if strings.HasPrefix(f.Name, "ppt/slides/slide") {
			hasSlide = true
		}
	}

	for _, req := range required {
		if !found[req] {
			t.Errorf("missing required entry: %s", req)
		}
	}
	if !hasSlide {
		t.Error("no slides found in PPTX")
	}
}
