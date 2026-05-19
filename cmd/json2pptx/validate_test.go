package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateJSONFile_ShapeGridValid(t *testing.T) {
	input := `{
  "template": "modern-template",
  "slides": [{
    "layout_id": "someLayout",
    "slide_type": "content",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}],
    "shape_grid": {
      "columns": 2,
      "rows": [{"cells": [
        {"shape": {"geometry": "roundRect", "fill": "#4472C4", "text": "A"}},
        {"shape": {"geometry": "roundRect", "fill": "#ED7D31", "text": "B"}}
      ]}]
    }
  }]
}`
	path := filepath.Join(t.TempDir(), "valid.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, "./templates", false)
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if result.ShapeCount != 2 {
		t.Errorf("expected ShapeCount=2, got %d", result.ShapeCount)
	}
}

func TestValidateJSONFile_ShapeGridInvalidGeometry(t *testing.T) {
	input := `{
  "template": "modern-template",
  "slides": [{
    "layout_id": "someLayout",
    "slide_type": "content",
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}],
    "shape_grid": {
      "columns": 1,
      "rows": [{"cells": [
        {"shape": {"geometry": "notARealGeometry", "fill": "#FF0000", "text": "X"}}
      ]}]
    }
  }]
}`
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(path, "./templates", false)
	if result.Valid {
		t.Fatal("expected invalid due to unknown geometry, but got valid")
	}
	found := false
	for _, e := range result.Errors {
		if len(e) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one error for unknown geometry")
	}
}

func TestValidateJSONFile_ShapeGridEmptyRows(t *testing.T) {
	input := `{
  "template": "modern-template",
  "slides": [{
    "layout_id": "someLayout",
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

	result := validateJSONFile(path, "./templates", false)
	if result.Valid {
		t.Fatal("expected invalid due to empty rows, but got valid")
	}
}

func TestValidateJSONFile_ShapeGridBadFillColor(t *testing.T) {
	input := `{
  "template": "modern-template",
  "slides": [{
    "layout_id": "someLayout",
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

	result := validateJSONFile(path, "./templates", false)
	// Bad fill color is a warning, not an error — should still be valid
	if !result.Valid {
		t.Fatalf("expected valid (bad fill is a warning), got errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected a warning for bad fill color format")
	}
}

// TestValidateJSONFile_SlideTypeAlternativeToLayoutID is a regression test for
// go-slide-creator-p13e. The validator previously rejected slides that used
// slide_type instead of layout_id, but the generator accepts slide_type and
// auto-selects a layout via heuristic. The validator must mirror that
// behaviour: a slide with slide_type but no layout_id is valid.
func TestValidateJSONFile_SlideTypeAlternativeToLayoutID(t *testing.T) {
	input := `{
  "template": "modern-template",
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

	result := validateJSONFile(path, "./templates", false)
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
  "template": "modern-template",
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

	result := validateJSONFile(path, "./templates", false)
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
