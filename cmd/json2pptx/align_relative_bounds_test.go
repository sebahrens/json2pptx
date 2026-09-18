package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestAlignRelativeBounds: height-capped, top-anchored pattern bounds are
// placed inside the content area (which callers derive from the chrome-aware
// zone, so a takeaway band shrinks the area the block is centred in).
func TestAlignRelativeBounds(t *testing.T) {
	content := pptx.RectEmu{X: 0, Y: 1000, CX: 10000, CY: 8000}
	box := pptx.RectEmu{X: 0, Y: 1000, CX: 10000, CY: 2000}
	cases := []struct {
		align string
		y     float64
		want  int64
	}{
		{"center", 0, 1000 + 3000},
		{"bottom", 0, 1000 + 6000},
		{"top", 0, 1000},
		{"", 0, 1000},
		{"center", 10, 1000}, // authored y offset is kept
	}
	for _, tc := range cases {
		in := &ShapeGridInput{VerticalAlign: tc.align, Bounds: &GridBoundsInput{Y: tc.y, Width: 100, Height: 25}}
		if got := alignRelativeBounds(box, content, in).Y; got != tc.want {
			t.Errorf("align=%q y=%v: got Y=%d want %d", tc.align, tc.y, got, tc.want)
		}
	}
	// A box as tall as the area is left alone.
	in := &ShapeGridInput{VerticalAlign: "center", Bounds: &GridBoundsInput{Width: 100, Height: 100}}
	if got := alignRelativeBounds(content, content, in); got != content {
		t.Errorf("full-height box must not move: %+v", got)
	}
}
