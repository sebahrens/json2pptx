package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func peerTestGrid(typeScale string, texts ...string) *Grid {
	cells := make([]Cell, len(texts))
	for i, text := range texts {
		raw, _ := json.Marshal(map[string]any{"paragraphs": []map[string]any{{"content": text, "size": 12}}})
		cells[i] = Cell{Shape: &ShapeSpec{Geometry: "rect", Text: raw}}
	}
	cols := make([]float64, len(texts))
	for i := range cols {
		cols[i] = 100 / float64(len(texts))
	}
	return &Grid{
		Bounds:    pptx.RectEmu{CX: 600 * 12700, CY: 90 * 12700},
		Columns:   cols,
		Rows:      []Row{{Cells: cells}},
		TypeScale: typeScale,
		// Sizes are compared as grown, before the type-scale snap.
		KeepTextSizes: true,
	}
}

func peerTestSizes(t *testing.T, grid *Grid) []int {
	t.Helper()
	res, err := Resolve(grid, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	for _, c := range res.Cells {
		if c.Kind != CellKindShape || c.ShapeSpec == nil {
			continue
		}
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, firstRunSizeHP(tb))
	}
	return sizes
}

// Peers grow together or not at all (go-slide-creator-riyh7): grow-to-fill
// measured each shape on its own, so the two-word card of a row grew while its
// two-line neighbour stayed, and one row read as two sizes.
func TestPeersShareOneGrowth(t *testing.T) {
	long := "A sentence long enough to fill its card at the size it was given, so that it cannot grow any further without leaving the share of the box the policy allows it to take, however the row beside it is filled"

	even := peerTestSizes(t, peerTestGrid("presentation", "Short label here", "Another short one", "Third short label"))
	for _, s := range even {
		if s <= 1200 {
			t.Fatalf("a row of short peers must grow under type_scale presentation, got %v", even)
		}
	}
	if even[0] != even[1] || even[1] != even[2] {
		t.Errorf("even peers grew to several sizes: %v", even)
	}

	uneven := peerTestSizes(t, peerTestGrid("presentation", "Short label here", long, "Third short label"))
	if uneven[0] != uneven[1] || uneven[1] != uneven[2] {
		t.Errorf("peers are written at several sizes: %v — the short ones grew past the long one", uneven)
	}
	if uneven[0] >= even[0] {
		t.Errorf("the long peer must hold its group back: %v beside the even row's %v", uneven, even)
	}
}

// A shape its pattern pinned holds its peers at the size they were given, and
// a shape that is no peer (another geometry) grows on its own.
func TestPinnedPeerHoldsItsGroup(t *testing.T) {
	grid := peerTestGrid("presentation", "Short label here", "Another short one", "Third short label")
	grid.Rows[0].Cells[1].Shape.TypeScale = "compact"
	if sizes := peerTestSizes(t, grid); sizes[0] != 1200 || sizes[1] != 1200 || sizes[2] != 1200 {
		t.Errorf("a pinned peer must hold the row at 12pt, got %v", sizes)
	}

	grid = peerTestGrid("presentation", "Short label here", "Another short one", "Third short label")
	grid.Rows[0].Cells[1].Shape.TypeScale = "compact"
	grid.Rows[0].Cells[1].Shape.Geometry = "roundRect"
	if sizes := peerTestSizes(t, grid); sizes[0] <= 1200 || sizes[1] != 1200 || sizes[2] != sizes[0] {
		t.Errorf("a pinned shape of another geometry is no peer of the rects beside it, got %v", sizes)
	}
}

func TestPeerAligned(t *testing.T) {
	r := func(x, y, w, h int64) pptx.RectEmu {
		return pptx.RectEmu{X: x * 12700, Y: y * 12700, CX: w * 12700, CY: h * 12700}
	}
	cases := []struct {
		name string
		a, b pptx.RectEmu
		want bool
	}{
		{"cards of a row", r(0, 0, 100, 60), r(110, 0, 100, 60), true},
		{"cells of a column, content-sized", r(0, 0, 100, 40), r(0, 50, 100, 80), true},
		{"steps of a staircase share foot and width", r(0, 60, 100, 40), r(110, 20, 100, 80), true},
		{"ranked bars share left and height", r(0, 0, 300, 30), r(0, 40, 120, 30), true},
		{"same-size steps in different lanes", r(0, 0, 100, 40), r(230, 170, 100, 40), true},
		{"a heading over a narrower cell of another height", r(0, 0, 600, 30), r(0, 40, 100, 80), false},
		{"unrelated frames", r(0, 0, 100, 40), r(230, 170, 140, 70), false},
		{"same width, no shared edge", r(0, 0, 100, 40), r(230, 170, 100, 70), false},
	}
	for _, tc := range cases {
		if got := peerAligned(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: peerAligned = %t, want %t", tc.name, got, tc.want)
		}
	}
}
