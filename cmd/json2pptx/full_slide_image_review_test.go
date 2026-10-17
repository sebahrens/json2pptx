package main

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"strings"
	"testing"
)

func TestFullSlideImageCompositionOwnsChromeButKeepsOverlayChecks(t *testing.T) {
	input := geomSlides(t, `[{"shape_grid":{"bounds":{"x":0,"y":0,"width":100,"height":100},"columns":1,"rows":[{"min_height":500,"max_height":500,"cells":[{"image":{"path":"hero.png"},"layers":[{"frame":{"x":0,"y":0.5,"w":1,"h":0.5},"shape":{"geometry":"rect","text":"Headline","fill":"dk1"}}]}]}]}}]`)
	grid := input.Slides[0].ShapeGrid
	geom := GridGeometry{Zone: &shapegrid.ContentZone{TitleBottom: 1000000, BodyTop: 1000000, FooterTop: 6000000, SlideWidth: 12192000, SlideHeight: 6858000}}
	fs := checkShapeGridStructural(grid, 0, 12192000, 6858000, nil, geom, true)
	for _, f := range fs {
		if f.Code == patterns.ErrCodeTitleCollision || f.Code == patterns.ErrCodeFooterCollision {
			t.Fatalf("intentional fullslide image flagged: %+v", f)
		}
	}
	raw, _ := json.Marshal(map[string]any{"content": strings.Repeat("W", 400), "size": 28})
	grid.Rows[0].Cells[0].Layers[0].Shape.Text = raw
	if fs := findingsByCode(collectGeometryFindings(input, nil, 12192000, 6858000, nil), patterns.ErrCodeTextExceedsShape); len(fs) == 0 {
		t.Fatalf("fullslide image lost text geometry guard: %+v", collectGeometryFindings(input, nil, 12192000, 6858000, nil))
	}
	grid.Bounds.Width = 100
	grid.Bounds.Height = 50
	if fullSlideImageGrid(grid) {
		t.Fatal("partial image exempted from chrome checks")
	}
}
