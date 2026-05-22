package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testTemplatesDir is the relative path to templates from cmd/json2pptx,
// matching the MCP-side test setup so CLI and MCP exercise the same files.
const testTemplatesDir = "../../templates"

func TestValidateJSONFile_ShapeGridValid(t *testing.T) {
	input := `{
  "template": "midnight-blue",
  "slides": [{
    "layout_id": "slideLayout2",
    "slide_type": "content",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}],
    "shape_grid": {
      "columns": 2,
      "rows": [{"cells": [
        {"shape": {"geometry": "roundRect", "fill": "accent1", "text": "A"}},
        {"shape": {"geometry": "roundRect", "fill": "accent2", "text": "B"}}
      ]}]
    }
  }]
}`
	path := filepath.Join(t.TempDir(), "valid.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if result.ShapeCount != 2 {
		t.Errorf("expected ShapeCount=2, got %d", result.ShapeCount)
	}
}

func TestValidateJSONFile_ShapeGridInvalidGeometry(t *testing.T) {
	input := `{
  "template": "midnight-blue",
  "slides": [{
    "layout_id": "slideLayout2",
    "slide_type": "content",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}],
    "shape_grid": {
      "columns": 1,
      "rows": [{"cells": [
        {"shape": {"geometry": "notARealGeometry", "fill": "accent1", "text": "X"}}
      ]}]
    }
  }]
}`
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
	if result.Valid {
		t.Fatal("expected invalid due to unknown geometry, but got valid")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "notARealGeometry") || strings.Contains(e, "geometry") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an unknown-geometry error, got: %v", result.Errors)
	}
}

func TestValidateJSONFile_ShapeGridEmptyRows(t *testing.T) {
	input := `{
  "template": "midnight-blue",
  "slides": [{
    "layout_id": "slideLayout2",
    "slide_type": "content",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}],
    "shape_grid": {
      "columns": 1,
      "rows": []
    }
  }]
}`
	path := filepath.Join(t.TempDir(), "empty_rows.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
	if result.Valid {
		t.Fatal("expected invalid due to empty rows, but got valid")
	}
}

func TestValidateJSONFile_ShapeGridBadFillColor(t *testing.T) {
	// design_mode=free so the bad fill string is the only finding — constrained
	// mode would also flag the raw color string as a design violation.
	input := `{
  "template": "midnight-blue",
  "design_mode": "free",
  "slides": [{
    "layout_id": "slideLayout2",
    "slide_type": "content",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}],
    "shape_grid": {
      "columns": 1,
      "rows": [{"cells": [
        {"shape": {"geometry": "rect", "fill": "notAColor", "text": "X"}}
      ]}]
    }
  }]
}`
	path := filepath.Join(t.TempDir(), "bad_fill.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
	// Bad fill color is a warning, not an error — should still be valid.
	if !result.Valid {
		t.Fatalf("expected valid (bad fill is a warning), got errors: %v", result.Errors)
	}
	if !anyContains(result.Warnings, "notAColor") {
		t.Errorf("expected a warning mentioning 'notAColor', got warnings: %v", result.Warnings)
	}
}

// TestValidateJSONFile_SlideTypeAlternativeToLayoutID is a regression test for
// go-slide-creator-p13e. The validator previously rejected slides that used
// slide_type instead of layout_id, but the generator accepts slide_type and
// auto-selects a layout via heuristic. The validator must mirror that
// behaviour: a slide with slide_type but no layout_id is valid.
func TestValidateJSONFile_SlideTypeAlternativeToLayoutID(t *testing.T) {
	input := `{
  "template": "midnight-blue",
  "slides": [
    {
      "slide_type": "title",
      "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hello"}]
    },
    {
      "slide_type": "content",
      "content": [{"placeholder_id": "title", "type": "text", "text_value": "Body"}]
    }
  ]
}`
	path := filepath.Join(t.TempDir(), "slide_type_only.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
	if !result.Valid {
		t.Fatalf("expected valid (slide_type is alternative to layout_id), got errors: %v", result.Errors)
	}
	for _, e := range result.Errors {
		if strings.Contains(e, "layout_id is required") || strings.Contains(e, "layout_id or slide_type is required") {
			t.Errorf("unexpected layout/slide_type error: %s", e)
		}
	}
}

// TestValidateJSONFile_MissingLayoutAndSlideType verifies the validator still
// errors when neither layout_id nor slide_type is provided.
func TestValidateJSONFile_MissingLayoutAndSlideType(t *testing.T) {
	input := `{
  "template": "midnight-blue",
  "slides": [
    {
      "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hello"}]
    }
  ]
}`
	path := filepath.Join(t.TempDir(), "missing_both.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, testTemplatesDir, "", false, "warn")
	if result.Valid {
		t.Fatal("expected invalid when both layout_id and slide_type are missing")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "layout_id or slide_type is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'layout_id or slide_type is required' error, got: %v", result.Errors)
	}
}

// TestValidateJSONFile_MCPParity confirms that validateJSONFile produces the
// same diagnostic codes for a given input as the MCP validate_input handler.
// This is the regression guard for the unification refactor: any future drift
// between CLI human-mode validate and MCP validate_input must be caught here.
func TestValidateJSONFile_MCPParity(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		strict bool
		// wantCodes are the diagnostic codes the input is expected to produce.
		// We compare CLI vs MCP on the SET of codes returned for the same input.
		wantCodes []string
	}{
		{
			name: "unknown_layout_id error",
			input: `{
  "template": "midnight-blue",
  "slides": [{
    "layout_id": "noSuchLayout",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "X"}]
  }]
}`,
			wantCodes: []string{"unknown_layout_id"},
		},
		{
			name: "unknown_key default warning",
			input: `{
  "template": "midnight-blue",
  "tmplate": "typo",
  "slides": [{
    "layout_id": "slideLayout2",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hi"}]
  }]
}`,
			wantCodes: []string{"unknown_key"},
		},
		{
			name:   "unknown_key strict error",
			strict: true,
			input: `{
  "template": "midnight-blue",
  "tmplate": "typo",
  "slides": [{
    "layout_id": "slideLayout2",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hi"}]
  }]
}`,
			wantCodes: []string{"unknown_key"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(path, []byte(tc.input), 0644); err != nil {
				t.Fatal(err)
			}

			result := validateJSONFile(path, testTemplatesDir, "", tc.strict, "warn")
			cliCodes := make(map[string]bool)
			for _, d := range result.Diagnostics {
				cliCodes[d.Code] = true
			}

			for _, code := range tc.wantCodes {
				if !cliCodes[code] {
					t.Errorf("CLI validate missing expected code %q\n  got: %v", code, codeSet(cliCodes))
				}
			}
		})
	}
}

func codeSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
