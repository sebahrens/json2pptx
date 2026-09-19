package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// go-slide-creator-yomm. Overlays rode along in SlideSpec.RawShapeXML, which is
// inserted at the START of the spTree, while media pics go in later. A numbered
// badge placed squarely on the screenshot it annotates was therefore painted
// over by that screenshot and simply invisible — the numbered-callouts slide,
// one of the most requested product/demo slides, could not be built.

// overlayCalloutDeck is a screenshot with two numbered badges (one of them
// squarely on the image) and an arrow whose head lands on the image.
func overlayCalloutDeck(t *testing.T, imagePath string) []byte {
	t.Helper()
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{map[string]any{
			"slide_type": "blank",
			"shape_grid": map[string]any{
				"rows": []any{map[string]any{"cells": []any{
					map[string]any{"image": map[string]any{"path": imagePath}},
				}}},
			},
			"overlays": []any{
				map[string]any{"kind": "badge", "text": "1", "color": "accent2", "width": 6, "height": 9, "from": map[string]any{"x": 28, "y": 28}},
				map[string]any{"kind": "badge", "text": "2", "color": "accent2", "width": 6, "height": 9, "from": map[string]any{"x": 55, "y": 55}},
				map[string]any{"kind": "arrow", "color": "accent2", "width": 3, "from": map[string]any{"x": 75, "y": 35}, "to": map[string]any{"x": 62, "y": 58}},
			},
		}},
	}
	b, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// tinyPNG writes a 2x2 PNG and returns its path.
func tinyPNG(t *testing.T, dir string) string {
	t.Helper()
	// A minimal valid PNG (2x2, opaque).
	png := []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02,
		0x08, 0x02, 0x00, 0x00, 0x00, 0xfd, 0xd4, 0x9a, 0x73,
		0x00, 0x00, 0x00, 0x16, 'I', 'D', 'A', 'T',
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0xc0, 0xc0, 0xc0, 0x00,
		0xc4, 0x20, 0x8e, 0x01, 0x00, 0x1a, 0x0a, 0x03, 0x01, 0x4c, 0xf5, 0x94, 0x54,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
	}
	path := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestOverlaysPaintAboveMedia is the bead's VERIFY, asserted on the slide XML:
// the picture must appear BEFORE the overlay shapes, because later in the
// spTree means higher z-order.
func TestOverlaysPaintAboveMedia(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a deck")
	}
	dir := t.TempDir()
	deckPath := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(deckPath, overlayCalloutDeck(t, tinyPNG(t, dir)), 0o600); err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join("..", "..", "templates")
	resultPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(deckPath, resultPath, templatesDir, dir, "", false, false, "", "off", false, "off", "", false); err != nil {
		t.Fatalf("runJSONMode: %v", err)
	}

	slideXML := readSlideXML(t, filepath.Join(dir, "output.pptx"), "ppt/slides/slide1.xml")

	picIdx := strings.Index(slideXML, `name="Picture`)
	if picIdx < 0 {
		t.Fatalf("no picture in the slide:\n%s", slideXML)
	}
	for _, overlay := range []string{"Overlay badge 1", "Overlay badge 2", "Overlay arrow 3"} {
		idx := strings.Index(slideXML, `name="`+overlay+`"`)
		if idx < 0 {
			t.Errorf("%s is missing from the slide", overlay)
			continue
		}
		if idx < picIdx {
			t.Errorf("%s is emitted BEFORE the picture, so the picture paints over it", overlay)
		}
	}
}

// TestOverlaysStayAboveGridCells guards the other direction: shape_grid cell
// shapes must still go in first, so overlays land on top of them too.
func TestOverlaysStayAboveGridCells(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a deck")
	}
	dir := t.TempDir()
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{map[string]any{
			"slide_type": "blank",
			"shape_grid": map[string]any{
				"rows": []any{map[string]any{"cells": []any{
					map[string]any{"shape": map[string]any{"geometry": "rect", "fill": "accent1", "text": map[string]any{"content": "Cell"}}},
				}}},
			},
			"overlays": []any{
				map[string]any{"kind": "badge", "text": "1", "color": "accent2", "from": map[string]any{"x": 40, "y": 40}},
			},
		}},
	}
	b, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	deckPath := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(deckPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(deckPath, resultPath, filepath.Join("..", "..", "templates"), dir, "", false, false, "", "off", false, "off", "", false); err != nil {
		t.Fatalf("runJSONMode: %v", err)
	}

	slideXML := readSlideXML(t, filepath.Join(dir, "output.pptx"), "ppt/slides/slide1.xml")
	badgeIdx := strings.Index(slideXML, `name="Overlay badge 1"`)
	if badgeIdx < 0 {
		t.Fatal("overlay badge missing from the slide")
	}
	// Every shape_grid cell shape must come before the overlay.
	cellIdx := strings.Index(slideXML, "<a:t>Cell</a:t>")
	if cellIdx < 0 {
		t.Fatalf("grid cell missing from the slide:\n%s", slideXML)
	}
	if cellIdx > badgeIdx {
		t.Error("grid cell is emitted after the overlay — overlays must stay on top")
	}
}

// readSlideXML extracts one entry from a generated .pptx.
func readSlideXML(t *testing.T, pptxPath, entry string) string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open %s: %v", pptxPath, err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != entry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry, err)
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", entry, err)
		}
		return string(bytes.TrimSpace(data))
	}
	t.Fatalf("%s not found in %s", entry, pptxPath)
	return ""
}
