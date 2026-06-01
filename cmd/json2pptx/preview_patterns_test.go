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

	got := pickPreviewLayout(layouts, nil)
	if got != "slideLayout2" {
		t.Errorf("expected slideLayout2 (fewest placeholders), got %s", got)
	}
}

func TestPickPreviewLayoutEmpty(t *testing.T) {
	got := pickPreviewLayout(nil, nil)
	if got != "slideLayout1" {
		t.Errorf("expected default slideLayout1, got %s", got)
	}
}

// TestPickPreviewLayoutSkipsSynthesized is the regression test for
// J2P-PREVIEW-007: a synthesized layout (slideLayout5) with the fewest
// placeholders must not be selected, because it is absent from the physical
// template package that preview generation renders against.
func TestPickPreviewLayoutSkipsSynthesized(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "slideLayout1", Placeholders: make([]types.PlaceholderInfo, 3)},
		{ID: "slideLayout2", Placeholders: make([]types.PlaceholderInfo, 2)},
		{ID: "slideLayout3", Placeholders: make([]types.PlaceholderInfo, 4)},
		{ID: "slideLayout4", Placeholders: make([]types.PlaceholderInfo, 4)},
		// Synthesized: fewest placeholders, so the old logic would pick it.
		{ID: "slideLayout5", Placeholders: make([]types.PlaceholderInfo, 1)},
	}
	synthesis := &types.SynthesisManifest{
		SyntheticFiles: map[string][]byte{
			"ppt/slideLayouts/slideLayout5.xml":            []byte("<xml/>"),
			"ppt/slideLayouts/_rels/slideLayout5.xml.rels": []byte("<rels/>"),
		},
	}

	got := pickPreviewLayout(layouts, synthesis)
	if got == "slideLayout5" {
		t.Fatalf("pickPreviewLayout returned synthesized-only layout %q; must pick a physical layout", got)
	}
	if got != "slideLayout2" {
		t.Errorf("expected slideLayout2 (fewest placeholders among physical layouts), got %s", got)
	}
}

// TestPickPreviewLayoutAllSynthesized covers the degenerate case where every
// known layout is synthesized: fall back to the canonical first layout.
func TestPickPreviewLayoutAllSynthesized(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "slideLayout5", Placeholders: make([]types.PlaceholderInfo, 1)},
		{ID: "slideLayout6", Placeholders: make([]types.PlaceholderInfo, 2)},
	}
	synthesis := &types.SynthesisManifest{
		SyntheticFiles: map[string][]byte{
			"ppt/slideLayouts/slideLayout5.xml": []byte("<xml/>"),
			"ppt/slideLayouts/slideLayout6.xml": []byte("<xml/>"),
		},
	}

	got := pickPreviewLayout(layouts, synthesis)
	if got != "slideLayout1" {
		t.Errorf("expected fallback slideLayout1 when all layouts are synthesized, got %s", got)
	}
}

func TestSyntheticLayoutIDs(t *testing.T) {
	if ids := syntheticLayoutIDs(nil); len(ids) != 0 {
		t.Errorf("expected empty set for nil manifest, got %v", ids)
	}

	synthesis := &types.SynthesisManifest{
		SyntheticFiles: map[string][]byte{
			"ppt/slideLayouts/slideLayout5.xml":            []byte("<xml/>"),
			"ppt/slideLayouts/_rels/slideLayout5.xml.rels": []byte("<rels/>"),
			"ppt/slideLayouts/slideLayout7.xml":            []byte("<xml/>"),
		},
	}
	ids := syntheticLayoutIDs(synthesis)
	if !ids["slideLayout5"] || !ids["slideLayout7"] {
		t.Errorf("expected slideLayout5 and slideLayout7 in set, got %v", ids)
	}
	if len(ids) != 2 {
		t.Errorf("expected exactly 2 synthetic IDs (.rels ignored), got %d: %v", len(ids), ids)
	}
}
