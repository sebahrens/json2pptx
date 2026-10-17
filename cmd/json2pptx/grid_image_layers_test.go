package main

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestDefaultFitImageTransformsAssociatedLayers(t *testing.T) {
	input := []GridRowInput{{Cells: []*GridCellInput{{Image: &GridImageInput{Path: "hero.png"}, Layers: []jsonschema.LayerInput{{Frame: jsonschema.LayerFrameInput{X: 0, Y: .55, W: 1, H: .45}, Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"dk1"`)}}, {Frame: jsonschema.LayerFrameInput{X: .1, Y: .6, W: .8, H: .3}, Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}}}}}}}
	rows := convertGridRows(input)
	alloc := pptx.NewShapeIDAllocator(nil)
	res, err := shapegrid.Resolve(&shapegrid.Grid{Bounds: pptx.RectEmu{CX: 12000000, CY: 6000000}, Columns: []float64{100}, Rows: rows}, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cells) != 3 {
		t.Fatalf("resolved cells %d", len(res.Cells))
	}
	fillDefaultFitImages(res, defaultFitImageSpecs(input, rows))
	host := res.Cells[0]
	if host.Bounds != host.CellBounds {
		t.Fatal("base picture did not fill cell")
	}
	want := pptx.RectEmu{X: host.Bounds.X, Y: host.Bounds.Y + host.Bounds.CY*55/100, CX: host.Bounds.CX, CY: host.Bounds.CY * 45 / 100}
	if res.Cells[1].Bounds != want {
		t.Fatalf("scrim bounds %+v want %+v", res.Cells[1].Bounds, want)
	}
	output, err := generateGridOutput(res, alloc, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(output.ImageInserts) != 1 || !output.ImageInserts[0].BehindShapes {
		t.Fatal("base picture not marked behind its native layers")
	}
}

func TestImageHeroURLIsResolvedBeforeNativeExpansion(t *testing.T) {
	slides := []SlideInput{{Pattern: &PatternInput{Name: "image-hero", Values: json.RawMessage(`{"image":{"url":"https://example.com/hero.png"},"headline":"A readable image headline"}`)}}}
	if !hasURLReferences(slides) {
		t.Fatal("image hero URL was not discovered")
	}
	if ds := resolveURLs(slides, fakeURLResolver{path: "/cache/hero.png"}); len(ds) != 0 {
		t.Fatalf("URL diagnostics %+v", ds)
	}
	raw := string(slides[0].Pattern.Values)
	if !strings.Contains(raw, "/cache/hero.png") || strings.Contains(raw, "example.com") {
		t.Fatalf("URL did not resolve in typed values: %s", raw)
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, _, err := expandPattern(slides[0].Pattern, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if grid.Rows[0].Cells[0].Image.Path != "/cache/hero.png" || len(grid.Rows[0].Cells[0].Layers) != 2 {
		t.Fatal("native image/scrim/headline lost")
	}
	if grid.Bounds == nil || grid.Bounds.Width != 100 || grid.BoundsRelativeToContentArea {
		t.Fatal("image hero did not preserve full-slide bounds")
	}
}

func TestRawImageHeroRoutesAttributionAndConclusionInsideScrim(t *testing.T) {
	for _, deckSource := range []string{"", "Deck research"} {
		input := &PresentationInput{Source: deckSource, Slides: []SlideInput{{Source: "Slide research", Takeaway: "The next step is funded", Pattern: &PatternInput{Name: "image-hero", Values: json.RawMessage(`{"image":{"path":"hero.png"},"headline":"A platform for growth","source":"Photo credit"}`)}}}}
		applyDefaults(input)
		s := input.Slides[0]
		if s.Source != "" || s.Takeaway != "" {
			t.Fatal("outside photo bands remain")
		}
		var v map[string]any
		_ = json.Unmarshal(s.Pattern.Values, &v)
		if !strings.Contains(v["source"].(string), "Photo credit") || !strings.Contains(v["source"].(string), "Slide research") || v["takeaway"] != "The next step is funded" {
			t.Fatalf("content lost %+v", v)
		}
		before := string(s.Pattern.Values)
		applyDefaults(input)
		if string(input.Slides[0].Pattern.Values) != before {
			t.Fatal("routing is not idempotent")
		}
	}
	input := &PresentationInput{Source: "Deck research", Slides: []SlideInput{{Pattern: &PatternInput{Name: "image-hero", Values: json.RawMessage(`{"image":{"path":"hero.png"},"headline":"A platform for growth"}`)}}}}
	applyDefaults(input)
	if !strings.Contains(string(input.Slides[0].Pattern.Values), "Deck research") {
		t.Fatal("deck attribution missing from scrim")
	}
}
