package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func valignGrid(align VerticalAlign, rows ...Row) *Grid {
	return &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 1000000, CX: 1270000, CY: 1270000}, // 100pt tall
		Columns: []float64{100},
		Rows:    rows,
		RowGap:  0.0001, // effectively zero (0 means default 8pt)
		VAlign:  align,
	}
}

func shapeRow(maxPt float64) Row {
	return Row{MaxHeight: maxPt, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}}}}
}

func TestResolve_VAlignCentersCappedRow(t *testing.T) {
	for _, tc := range []struct {
		align VerticalAlign
		wantY int64
		wantH int64
	}{
		{VAlignStretch, 1000000, 1270000}, // legacy: capped row re-stretched to fill
		{VAlignTop, 1000000, 508000},
		{VAlignCenter, 1000000 + (1270000-508000)/2, 508000},
		{VAlignBottom, 1000000 + 1270000 - 508000, 508000},
	} {
		res, err := Resolve(valignGrid(tc.align, shapeRow(40)), pptx.NewShapeIDAllocator(nil))
		if err != nil {
			t.Fatal(err)
		}
		b := res.Cells[0].Bounds
		if abs64(b.Y-tc.wantY) > 2 || abs64(b.CY-tc.wantH) > 2 {
			t.Errorf("%q: got y=%d h=%d, want y=%d h=%d", tc.align, b.Y, b.CY, tc.wantY, tc.wantH)
		}
	}
}

// Grids without a max_height-capped row keep the legacy stretch even when an
// alignment is requested (auto / proportional rows rely on normalisation).
func TestResolve_VAlignWithoutCapStretches(t *testing.T) {
	g := valignGrid(VAlignCenter,
		Row{Height: 20, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
		Row{Height: 20, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
	)
	res, err := Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	last := res.Cells[len(res.Cells)-1].Bounds
	if got := last.Y + last.CY; abs64(got-(1000000+1270000)) > 2 {
		t.Errorf("uncapped rows must fill the bounds, bottom=%d", got)
	}
}

func TestParseVerticalAlign(t *testing.T) {
	for in, want := range map[string]VerticalAlign{"": VAlignStretch, "stretch": VAlignStretch, "top": VAlignTop, "center": VAlignCenter, "middle": VAlignCenter, "bottom": VAlignBottom} {
		got, ok := ParseVerticalAlign(in)
		if !ok || got != want {
			t.Errorf("ParseVerticalAlign(%q) = %q,%v", in, got, ok)
		}
	}
	if _, ok := ParseVerticalAlign("sideways"); ok {
		t.Error("unknown value must not parse")
	}
}

func TestLayoutRowsEMU_Overfull(t *testing.T) {
	h, off := layoutRowsEMU([]float64{60, 60}, 1200, VAlignCenter)
	if off != 0 || h[0]+h[1] != 1200 {
		t.Errorf("overfull rows must be scaled to fit: %v off=%d", h, off)
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
