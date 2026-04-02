package main

import (
	"os"
	"path/filepath"
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

	result := validateJSONFile(path, "./templates")
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

	result := validateJSONFile(path, "./templates")
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

	result := validateJSONFile(path, "./templates")
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

	result := validateJSONFile(path, "./templates")
	// Bad fill color is a warning, not an error — should still be valid
	if !result.Valid {
		t.Fatalf("expected valid (bad fill is a warning), got errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected a warning for bad fill color format")
	}
}
