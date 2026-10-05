package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// flowBandGrid is a slide's own block of two content-sized rows — four filled
// step shapes over four lines of description — in a 400pt-tall, 900pt-wide
// area: the grid a value-chain expands to.
func flowBandGrid(stepPt, descPt float64) *Grid {
	step, _ := json.Marshal(map[string]any{"content": "Step", "size": 14, "bold": true})
	desc, _ := json.Marshal(map[string]any{"content": "What happens here", "size": 14})
	row := func(h float64, text json.RawMessage, fill string) Row {
		cells := make([]Cell, 4)
		for i := range cells {
			cells[i] = Cell{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(fill), Text: text}}
		}
		return Row{MaxHeight: h, Cells: cells}
	}
	return &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: PtToEMU(100), CX: PtToEMU(900), CY: PtToEMU(400)},
		Columns: []float64{25, 25, 25, 25},
		RowGap:  4,
		VAlign:  VAlignAuto,
		Compose: true,
		Rows:    []Row{row(stepPt, step, `"accent1"`), row(descPt, desc, `"none"`)},
	}
}

// A sparse block marked ComposeBand has its rows grown until the block, set
// at the optical centre, leaves 28% of the area under it — clear of the lower
// third a slide may not leave empty (go-slide-creator-kgfs1). A block that
// would need more than 1.6x keeps its height: it is a strip and is reported
// as one, not stretched into slabs.
func TestComposeBandScalesASparseBlockClearOfTheLowerThird(t *testing.T) {
	bandUnder := func(g *Grid) (blockPt, underPct float64) {
		_, top, bottom := resolvedBlock(t, g)
		end := g.Bounds.Y + g.Bounds.CY
		return float64(bottom-top) / 12700, 100 * float64(end-bottom) / float64(g.Bounds.CY)
	}

	// 50 + 4 + 80 = 134pt of 400pt: 33.5% of the area, 36.6% under it.
	plain := flowBandGrid(50, 80)
	plainBlock, plainUnder := bandUnder(plain)
	if plainUnder < 100.0/3 {
		t.Fatalf("setup: the unscaled block leaves %.1f%% under it, want a third or more", plainUnder)
	}
	scaled := flowBandGrid(50, 80)
	scaled.ComposeBand = true
	block, under := bandUnder(scaled)
	if under < 27 || under > 29 {
		t.Errorf("band-scaled block leaves %.1f%% of the area under it, want about 28%%", under)
	}
	if k := block / plainBlock; k <= 1 || k > 1.6 {
		t.Errorf("rows grew %.2fx (%.0fpt -> %.0fpt), want over 1x and at most 1.6x", k, plainBlock, block)
	}

	// 30 + 4 + 40 = 74pt: 18.5% of the area. Reaching 28% needs 2.5x.
	thin := flowBandGrid(30, 40)
	thinBlock, _ := bandUnder(thin)
	thinScaled := flowBandGrid(30, 40)
	thinScaled.ComposeBand = true
	if got, under := bandUnder(thinScaled); got > thinBlock*1.36 || under < 100.0/3 {
		t.Errorf("a block too thin for 1.6x grew from %.0fpt to %.0fpt (%.1f%% under it): want it left a strip", thinBlock, got, under)
	}

	// The same thin block may grow further when its text boxes are allowed
	// to become squares: 225pt-wide boxes hold a far taller row.
	square := flowBandGrid(30, 40)
	square.ComposeBand, square.ComposeBandSquare = true, true
	if _, under := bandUnder(square); under < 27 || under > 29 {
		t.Errorf("band-scaled block of boxes that may be square leaves %.1f%% under it, want about 28%%", under)
	}

	// A block already over the target is not touched, and neither is a grid
	// that did not ask.
	tall := flowBandGrid(90, 110)
	tallBlock, _ := bandUnder(tall)
	tallScaled := flowBandGrid(90, 110)
	tallScaled.ComposeBand = true
	if got, _ := bandUnder(tallScaled); got < tallBlock-1 || got > tallBlock+1 {
		t.Errorf("a block that already clears the lower third went from %.0fpt to %.0fpt", tallBlock, got)
	}
}
