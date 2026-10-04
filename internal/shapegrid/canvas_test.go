package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestCanvasScaleFor(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int64
		want float64
	}{
		{"standard 16:9", DefaultSlideWidthEMU, DefaultSlideHeightEMU, 1},
		{"13.33in rounding", 12188952, 6858000, 1},
		{"4:3 at the standard height", 9144000, 6858000, 1},
		{"unknown", 0, 0, 1},
		{"business-template 14.7 x 8.3in", 13442950, 7561263, 7561263.0 / float64(DefaultSlideHeightEMU)},
		{"outsized", 4 * DefaultSlideWidthEMU, 4 * DefaultSlideHeightEMU, canvasScaleMax},
	} {
		if got := CanvasScaleFor(tc.w, tc.h); got < tc.want-1e-9 || got > tc.want+1e-9 {
			t.Errorf("%s: scale = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// On a standard slide (scale 0 or 1) a grid resolves exactly as designed; on
// a larger one its point-sized rows, gaps and text follow the canvas
// (go-slide-creator-ttpae).
func TestCanvasScaleFollowsTheSlide(t *testing.T) {
	design, top, bottom := resolvedBlock(t, composeTestGrid(50, 5)) // dense: no type step
	for _, c := range []float64{0, 1} {
		g := composeTestGrid(50, 5)
		g.CanvasScale = c
		res, gotTop, gotBottom := resolvedBlock(t, g)
		if sizesOf(res) != sizesOf(design) || gotTop != top || gotBottom != bottom {
			t.Errorf("scale %v: sizes %s block %d..%d, want the design's %s %d..%d", c, sizesOf(res), gotTop, gotBottom, sizesOf(design), top, bottom)
		}
	}

	g := composeTestGrid(50, 5)
	g.Bounds.CY = PtToEMU(360)
	g.CanvasScale = 1.1
	res, top, bottom := resolvedBlock(t, g)
	if got := sizesOf(res); got != "15,15,13,13,13,13,13,13,13,13" {
		t.Errorf("sizes = %s, want the 14pt heading at 15 and the 12pt body at 13", got)
	}
	// Five 50pt rows and four 8pt gaps, all 1.1 times as tall.
	if h := float64(bottom-top) / 12700; h < 282*1.1-1 || h > 282*1.1+1 {
		t.Errorf("block height %.1fpt, want %.1fpt", h, 282*1.1)
	}
	if g.Rows[0].MaxHeight != 50 || g.CanvasScale != 1.1 {
		t.Error("scaling mutated the input grid")
	}
}

// A size level whose label would lose its line keeps its design size; the
// other levels still follow the canvas. A grid none of whose text can move is
// resolved as designed.
func TestCanvasScaleKeepsALevelThatWouldBreak(t *testing.T) {
	text := func(s string, size float64) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"content": s, "size": size})
		return raw
	}
	grid := func(label string, size float64) *Grid {
		return &Grid{
			Bounds:      pptx.RectEmu{CX: PtToEMU(400), CY: PtToEMU(300)},
			Columns:     []float64{30, 70},
			VAlign:      VAlignAuto,
			CanvasScale: 1.1,
			Rows: []Row{{MaxHeight: 60, Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect", Text: text(label, size)}},
				{Shape: &ShapeSpec{Geometry: "rect", Text: text("A sentence that wraps where it likes", 12)}},
			}}},
		}
	}
	// The label's line is about 104pt as designed and 111pt scaled (7% more).
	// A 14pt label becomes 15pt (7% more) and takes the share of its line it
	// took before, so "Accountability" at 83% of the line scales although it
	// is past the stand-in face's 80% token margin
	// (go-slide-creator-5x4w4). An 18pt label becomes 20pt (11% more):
	// "Accountable" takes more of its line than before and past the margin,
	// so its level stays.
	for _, label := range []string{"Scope", "Accountability"} {
		if res, _, _ := resolvedBlock(t, grid(label, 14)); sizesOf(res) != "15,13" {
			t.Errorf("%s: sizes = %s, want both levels scaled (15,13)", label, sizesOf(res))
		}
	}
	res, _, _ := resolvedBlock(t, grid("Accountable", 18))
	if got := sizesOf(res); got != "18,13" {
		t.Errorf("sizes = %s, want the label level kept at 18 and the body at 13", got)
	}

	// Text pinned at a size nothing can scale: the design stands, rows included.
	pinned := grid("Accountable", 18)
	pinned.Rows[0].Cells = pinned.Rows[0].Cells[:1]
	pinned.Columns = []float64{30, 70}
	design := *pinned
	design.CanvasScale = 0
	got, top, bottom := resolvedBlock(t, pinned)
	want, wantTop, wantBottom := resolvedBlock(t, &design)
	if sizesOf(got) != sizesOf(want) || top != wantTop || bottom != wantBottom {
		t.Errorf("unscalable grid: sizes %s block %d..%d, want the design's %s %d..%d", sizesOf(got), top, bottom, sizesOf(want), wantTop, wantBottom)
	}
}
