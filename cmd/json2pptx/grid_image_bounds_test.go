package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestImageGridExplicitBounds(t *testing.T) {
	zone := &shapegrid.ContentZone{TitleBottom: 1000000, FooterTop: 8000000, LeftMargin: 1000000, RightEdge: 15000000, SlideWidth: 16000000, SlideHeight: 9000000, SideDecor: true}
	picture := &GridCellInput{Image: &GridImageInput{Path: "slide.svg"}}
	for _, tc := range []struct {
		name                     string
		cells                    []*GridCellInput
		relative, implicit, full bool
	}{
		{"image", []*GridCellInput{picture}, false, false, true},
		{"images", []*GridCellInput{picture, picture}, false, false, true},
		{"nil cell", []*GridCellInput{nil, picture}, false, false, true},
		{"mixed", []*GridCellInput{picture, {Shape: &ShapeSpecInput{Geometry: "rect"}}}, false, false, false},
		{"empty", nil, false, false, false},
		{"spacer", []*GridCellInput{{}}, false, false, false},
		{"content relative", []*GridCellInput{picture}, true, false, false},
		{"default bounds", []*GridCellInput{picture}, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := &ShapeGridInput{Bounds: &GridBoundsInput{Width: 100, Height: 100}, BoundsRelativeToContentArea: tc.relative, Rows: []GridRowInput{{Cells: tc.cells}}}
			if tc.implicit {
				input.Bounds = nil
			}
			got := resolveGridBounds(input, nil, zone, 16000000, 9000000)
			if tc.full {
				if want := (pptx.RectEmu{CX: 16000000, CY: 9000000}); got != want {
					t.Fatalf("bounds=%+v want %+v", got, want)
				}
				return
			}
			if got.X < zone.LeftMargin || got.Y < zone.TitleBottom || got.X+got.CX > zone.RightEdge || got.Y+got.CY > zone.FooterTop {
				t.Fatalf("ordinary/content-relative grid escaped zone: %+v", got)
			}
		})
	}
}
