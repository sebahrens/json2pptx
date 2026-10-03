package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// The contrast prediction reads the shapes generation writes, not the authored
// ones. Resolution changes text sizes — here the composition policy steps a
// sparse block's 14pt labels to 18pt — and 18pt is large text, held to WCAG's
// 3:1 instead of 4.5:1. Compiled from the authored spec, validate predicted a
// swap generation never made (TestContrastParityCorpus, varied-pitch-deck
// slide 7) and missed one it did (patterns-smoke slide 6).
func TestContrastPredictionCompilesResolvedGridShapes(t *testing.T) {
	const slideW, slideH = int64(12192000), int64(6858000)
	var grid ShapeGridInput
	if err := json.Unmarshal([]byte(`{
		"columns": 3, "vertical_align": "auto",
		"rows": [{"max_height": 60, "cells": [
			{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "North", "size": 14, "color": "accent1"}}},
			{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "South", "size": 14, "color": "accent1"}}},
			{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "West", "size": 14, "color": "accent1"}}}
		]}]
	}`), &grid); err != nil {
		t.Fatal(err)
	}
	zone := &shapegrid.ContentZone{
		TitleBottom: 1300000, FooterTop: 6300000, LeftMargin: 600000, RightEdge: 11600000,
		SlideWidth: slideW, SlideHeight: slideH,
	}

	rendered, err := resolveShapeGrid(&grid, pptx.NewShapeIDAllocator(nil), nil, zone, slideW, slideH, nil)
	if err != nil {
		t.Fatal(err)
	}
	predicted := compiledGridContrastCells(&grid, "/slides/0/shape_grid", nil, zone, slideW, slideH, 0)
	if len(predicted) != 3 {
		t.Fatalf("predicted %d text shapes, want 3", len(predicted))
	}
	for i, cell := range predicted {
		found := false
		for _, shape := range rendered.Shapes {
			if bytes.Equal(shape, cell.xml) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("predicted shape %d (%s) is not a shape generation writes:\n%s", i, cell.path, cell.xml)
		}
		// Guard the premise: the block is composed, so its type is stepped.
		if !bytes.Contains(cell.xml, []byte(`sz="1800"`)) || bytes.Contains(cell.xml, []byte(`sz="1400"`)) {
			t.Errorf("predicted shape %d keeps its authored 14pt; the composed block renders at 18pt:\n%s", i, cell.xml)
		}
	}
}
