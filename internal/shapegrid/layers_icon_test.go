package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func discLayer(x, y, w, h float64, text string, icon *IconSpec) Layer {
	l := Layer{Frame: LayerFrame{X: x, Y: y, W: w, H: h}, Shape: &ShapeSpec{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`)}, Icon: icon}
	if text != "" {
		l.Shape.Text = json.RawMessage(`{"content":"` + text + `","size":12}`)
	}
	return l
}

func layerCells(res *ResolveResult) []ResolvedCell {
	var out []ResolvedCell
	for _, c := range res.Cells {
		if c.Layer {
			out = append(out, c)
		}
	}
	return out
}

func centre(r pptx.RectEmu) (int64, int64) { return r.X + r.CX/2, r.Y + r.CY/2 }

// An icon on a textless layer is centred in the layer's frame, a square of
// the icon's scale (default 60%) of the frame's shorter side.
func TestLayerIconIsCentredInTheLayerFrame(t *testing.T) {
	grid := layeredGrid([]float64{100}, Cell{Fit: FitContain, Layers: []Layer{
		discLayer(0, 0, 0.5, 0.5, "", &IconSpec{Name: "shield"}),
		discLayer(0.5, 0.25, 0.4, 0.2, "", &IconSpec{Name: "shield", Scale: 0.5}),
		discLayer(0.5, 0.6, 0.3, 0.3, "", nil),
	}})
	layers := layerCells(mustResolve(t, grid))
	if len(layers) != 3 {
		t.Fatalf("resolved %d layers, want 3", len(layers))
	}
	for i, want := range []float64{0.6, 0.5} {
		l := layers[i]
		if l.IconSpec == nil || l.Kind != CellKindShape {
			t.Fatalf("layer %d: icon not carried on the layer's shape entry: %+v", i, l)
		}
		if l.IconBounds.CX != l.IconBounds.CY {
			t.Errorf("layer %d: icon is %d x %d EMU, not a square", i, l.IconBounds.CX, l.IconBounds.CY)
		}
		side := min(l.Bounds.CX, l.Bounds.CY)
		if got := int64(float64(side) * want); l.IconBounds.CX != got {
			t.Errorf("layer %d: icon side = %d, want %.0f%% of the frame's shorter side (%d)", i, l.IconBounds.CX, want*100, got)
		}
		ix, iy := centre(l.IconBounds)
		fx, fy := centre(l.Bounds)
		if d := ix - fx; d < -1 || d > 1 {
			t.Errorf("layer %d: icon centre x is %d EMU off the frame's", i, d)
		}
		if d := iy - fy; d < -1 || d > 1 {
			t.Errorf("layer %d: icon centre y is %d EMU off the frame's", i, d)
		}
		if l.TextInsets != [4]int64{} {
			t.Errorf("layer %d: a centred icon reserved text insets %v", i, l.TextInsets)
		}
	}
	if l := layers[2]; l.IconSpec != nil || l.IconBounds != (pptx.RectEmu{}) {
		t.Errorf("a layer without an icon resolved one: %+v", l)
	}
}

// With text, a layer's icon is laid out as a cell's is: beside the text on a
// wide shape, above it otherwise, and the text gets the matching extra inset.
func TestLayerIconReservesItsTextInset(t *testing.T) {
	grid := layeredGrid([]float64{100}, Cell{Layers: []Layer{
		discLayer(0, 0, 0.5, 1, "Platform", &IconSpec{Name: "shield"}),
		discLayer(0.5, 0, 0.1, 1, "Risk", &IconSpec{Name: "shield"}),
	}})
	layers := layerCells(mustResolve(t, grid))
	wide, tall := layers[0], layers[1]
	if wide.TextInsets[0] <= 0 || wide.TextInsets[1] != 0 {
		t.Errorf("wide layer: text insets = %v, want a left inset only", wide.TextInsets)
	}
	if wide.IconBounds.X < wide.Bounds.X || wide.IconBounds.X+wide.IconBounds.CX > wide.Bounds.X+wide.TextInsets[0] {
		t.Errorf("wide layer: icon %+v is not inside the inset it reserved (%d)", wide.IconBounds, wide.TextInsets[0])
	}
	if tall.TextInsets[1] <= 0 || tall.TextInsets[0] != 0 {
		t.Errorf("tall layer: text insets = %v, want a top inset only", tall.TextInsets)
	}
	// The same shape and icon as a cell of the layer's size resolve alike.
	asCell := mustResolve(t, &Grid{Bounds: wide.Bounds, Columns: []float64{100}, Rows: []Row{{Cells: []Cell{{
		Shape: &ShapeSpec{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`), Text: json.RawMessage(`{"content":"Platform","size":12}`)},
		Icon:  &IconSpec{Name: "shield"},
	}}}}}).Cells[0]
	if asCell.IconBounds != wide.IconBounds || asCell.TextInsets != wide.TextInsets {
		t.Errorf("layer icon layout %+v / %v differs from a cell's %+v / %v", wide.IconBounds, wide.TextInsets, asCell.IconBounds, asCell.TextInsets)
	}
}
