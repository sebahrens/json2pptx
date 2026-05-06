package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestFindPatternPreviewPNGs(t *testing.T) {
	// Create temp dir structure simulating assets/pattern-previews
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	previewsDir := filepath.Join(tmpDir, "assets", "pattern-previews", "midnight-blue")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(previewsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create fake PNG files
	if err := os.WriteFile(filepath.Join(previewsDir, "card-grid.png"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test: should find the PNG
	paths := findPatternPreviewPNGs(templatesDir, "card-grid")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if filepath.Base(paths[0]) != "card-grid.png" {
		t.Errorf("unexpected path: %s", paths[0])
	}

	// Test: should not find non-existent pattern
	paths = findPatternPreviewPNGs(templatesDir, "nonexistent")
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for nonexistent pattern, got %d", len(paths))
	}
}

func TestPickPreviewLayout(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "slideLayout1", Placeholders: make([]types.PlaceholderInfo, 3)},
		{ID: "slideLayout2", Placeholders: make([]types.PlaceholderInfo, 1)},
		{ID: "slideLayout3", Placeholders: make([]types.PlaceholderInfo, 5)},
	}

	got := pickPreviewLayout(layouts)
	if got != "slideLayout2" {
		t.Errorf("expected slideLayout2 (fewest placeholders), got %s", got)
	}
}

func TestPickPreviewLayoutEmpty(t *testing.T) {
	got := pickPreviewLayout(nil)
	if got != "slideLayout1" {
		t.Errorf("expected default slideLayout1, got %s", got)
	}
}
