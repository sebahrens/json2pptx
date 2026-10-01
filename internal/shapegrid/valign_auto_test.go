package shapegrid

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-e17xy: "auto" hangs a block from the template's body line
// whatever its fill; without a body line it keeps the 60% fill rule.
func TestVAlignAutoResolution(t *testing.T) {
	const avail = 1000 * 12700
	cases := []struct {
		name    string
		used    int64
		anchorY int64
		want    VerticalAlign
	}{
		{"short block, no body line", 400 * 12700, 0, VAlignTop},
		{"full block, no body line", 800 * 12700, 0, VAlignCenter},
		{"short block, body line", 400 * 12700, 5 * 12700, VAlignTop},
		{"full block, body line", 800 * 12700, 5 * 12700, VAlignTop},
	}
	for _, tc := range cases {
		if got := VAlignAuto.AnchoredAuto(tc.anchorY).ResolveAuto(tc.used, avail); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	for _, a := range []VerticalAlign{VAlignStretch, VAlignTop, VAlignCenter, VAlignBottom} {
		if got := a.AnchoredAuto(12700); got != a {
			t.Errorf("AnchoredAuto changed explicit %q to %q", a, got)
		}
	}
}

// A capped block in a grid with a body line hangs from it even when it fills
// most of the bounds.
func TestResolveAutoFullBlockHangsFromAnchor(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 1000 * 12700, CX: 9000 * 12700, CY: 400 * 12700},
		Columns: []float64{100},
		Rows:    []Row{{MaxHeight: 300, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}}},
		VAlign:  VAlignAuto,
		AnchorY: 1010 * 12700,
	}
	res, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cells) == 0 {
		t.Fatal("no cells")
	}
	if y := res.Cells[0].Bounds.Y; y != 1010*12700 {
		t.Errorf("block top = %.1fpt, want the body line at 1010pt", float64(y)/12700)
	}
}
