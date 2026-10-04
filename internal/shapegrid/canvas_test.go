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
	grid := func(label string) *Grid {
		return &Grid{
			Bounds:      pptx.RectEmu{CX: PtToEMU(400), CY: PtToEMU(300)},
			Columns:     []float64{30, 70},
			VAlign:      VAlignAuto,
			CanvasScale: 1.1,
			Rows: []Row{{MaxHeight: 60, Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect", Text: text(label, 14)}},
				{Shape: &ShapeSpec{Geometry: "rect", Text: text("A sentence that wraps where it likes", 12)}},
			}}},
		}
	}
	// 30% of 400pt less the column gap and the text margins leaves about 89pt:
	// "Scope" holds its line at 14pt and at 15pt, "Accountability" only
	// at 14pt.
	if res, _, _ := resolvedBlock(t, grid("Scope")); sizesOf(res) != "15,13" {
		t.Errorf("sizes = %s, want both levels scaled (15,13)", sizesOf(res))
	}
	res, _, _ := resolvedBlock(t, grid("Accountability"))
	if got := sizesOf(res); got != "14,13" {
		t.Errorf("sizes = %s, want the label level kept at 14 and the body at 13", got)
	}

	// Text pinned at a size nothing can scale: the design stands, rows included.
	pinned := grid("Accountability")
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
