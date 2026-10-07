package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// go-slide-creator-7ophx: peers in sibling nested grids — the labels of a
// ranked bar list, one sub-grid per bar — grow together, and the preflight
// walkers read them at the sizes generation writes.
func TestNestedPeersShareOneSizeInGenerationAndPreflight(t *testing.T) {
	grid, _, err := expandPattern(&PatternInput{
		Name: "horizontal-bar-with-callouts",
		Values: json.RawMessage(`{"unit": "%", "bars": [
			{"label": "Vendor A", "value": 87, "callout": "Best coverage"},
			{"label": "Vendor B across every region", "value": 64, "callout": "Strong in EMEA only"},
			{"label": "Vendor C", "value": 51, "callout": "Cheapest, thin support"},
			{"label": "Vendor D", "value": 38, "callout": "New entrant"}]}`),
	}, patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	applyGridTypeScale(grid, "presentation")
	bounds := pptx.RectEmu{X: 457200, Y: 1371600, CX: 11277600, CY: 4572000}
	const w, h = int64(12192000), int64(6858000)

	labelSizes := func(cells []shapegrid.ResolvedCell) map[string]int {
		sizes := map[string]int{}
		for _, c := range cells {
			if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
				continue
			}
			tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
			if err != nil || tb == nil || len(tb.Paragraphs) == 0 || len(tb.Paragraphs[0].Runs) == 0 {
				continue
			}
			if text := tb.Paragraphs[0].Runs[0].Text; strings.HasPrefix(text, "Vendor ") {
				sizes[text] = tb.Paragraphs[0].Runs[0].FontSize
			}
		}
		return sizes
	}

	rendered, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &bounds, nil, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	generated := labelSizes(rendered.Cells)
	if len(generated) != 4 {
		t.Fatalf("expected the four bar labels among the rendered cells, got %v", generated)
	}
	first := generated["Vendor A"]
	for label, size := range generated {
		if size != first {
			t.Errorf("bar labels are written at several sizes: %q at %d, \"Vendor A\" at %d (%v)", label, size, first, generated)
		}
	}
	if first <= 1200 {
		t.Errorf("the labels no longer carry a compact pin and should grow under type_scale presentation, got %d", first)
	}

	structural := resolveGridForStructural(grid, &bounds, nil, w, h)
	if structural == nil {
		t.Fatal("preflight could not resolve the grid")
	}
	var walked []shapegrid.ResolvedCell
	for _, c := range readabilityGridCells(grid, structural, "", w, h, 0) {
		walked = append(walked, c.cell)
	}
	preflight := labelSizes(walked)
	for label, size := range generated {
		if preflight[label] != size {
			t.Errorf("%q: preflight reads %d, generation writes %d", label, preflight[label], size)
		}
	}
}

// go-slide-creator-fmmec: the envelope callout is the takeaway band and spans
// every column of its host grid. Counting the first row's cells stopped it at
// the first segment of a horizontal compose envelope.
func TestComposeCalloutBandSpansTheWholeGrid(t *testing.T) {
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{SizePct: 50, Pattern: PatternInput{Name: "kpi-2up", Values: json.RawMessage(`[{"value": "+8%", "label": "Average price"}, {"value": "94%", "label": "Net retention"}]`)}},
			{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "B", "label": "Second"}`)}},
		},
		Callout: &patterns.PatternCallout{Text: "Raise list price at renewal, not mid-term."},
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	last := grid.Rows[len(grid.Rows)-1]
	// The default callout is the accent bar beside unfilled text.
	bar, text := calloutBandCells(t, last)
	if string(bar.Shape.Fill) != `"accent1"` || string(text.Shape.Fill) != `"none"` {
		t.Errorf("callout band = bar %s / text fill %s, want accent1 bar, no fill", bar.Shape.Fill, text.Shape.Fill)
	}
	// The band row was appended after the count was taken, so it holds no
	// more columns than the rows above it.
	if cols := inferColumnCount(grid); cols < 2 || last.Cells[0].ColSpan != cols {
		t.Errorf("callout band spans %d of the grid's %d columns", last.Cells[0].ColSpan, cols)
	}
}
