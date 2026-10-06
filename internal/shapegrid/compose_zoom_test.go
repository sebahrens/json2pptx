package shapegrid

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// headedColumnsGrid is a slide's own block of two content-sized rows — n bold
// 14pt headings over n 12pt bodies — in a 400pt-tall, 900pt-wide area: the
// grid an open column pattern (before-after, stylish-panels) expands to.
func headedColumnsGrid(n int, headPt, bodyPt float64, body string) *Grid {
	head, _ := json.Marshal(map[string]any{"content": "Heading", "size": 14, "bold": true})
	text, _ := json.Marshal(map[string]any{"content": body, "size": 12})
	row := func(h float64, raw json.RawMessage) Row {
		cells := make([]Cell, n)
		for i := range cells {
			cells[i] = Cell{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: raw}}
		}
		return Row{MaxHeight: h, Cells: cells}
	}
	cols := make([]float64, n)
	for i := range cols {
		cols[i] = 100 / float64(n)
	}
	return &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: PtToEMU(100), CX: PtToEMU(900), CY: PtToEMU(400)},
		Columns: cols,
		RowGap:  4,
		VAlign:  VAlignAuto,
		Compose: true,
		Rows:    []Row{row(headPt, head), row(bodyPt, text)},
	}
}

// A sparse full-slide exhibit (ComposeZoom) is scaled as a whole: its rows
// grow towards 70% of the area, at most 1.6x, and its type takes two steps of
// the zoom ladder — 12pt body to 18pt, the 14pt heading over it to 24pt —
// with every cell of a role at one size (go-slide-creator-cyyiy).
func TestComposeZoomScalesASparseExhibitAsAWhole(t *testing.T) {
	// 44 + 4 + 110 = 158pt of 400pt: 39.5% of the area.
	plain := headedColumnsGrid(3, 44, 110, "Three short words")
	plainRes, top, bottom := resolvedBlock(t, plain)
	plainBlock := float64(bottom-top) / 12700
	if got := sizesOf(plainRes); got != "18,18,18,14,14,14" {
		t.Fatalf("setup: the one-step policy sets %s, want headings 18 and bodies 14", got)
	}

	zoom := headedColumnsGrid(3, 44, 110, "Three short words")
	zoom.ComposeZoom = true
	res, top, bottom := resolvedBlock(t, zoom)
	if got := sizesOf(res); got != "24,24,24,18,18,18" {
		t.Errorf("zoomed sizes = %s, want headings 24 and bodies 18, one size per role", got)
	}
	block := float64(bottom-top) / 12700
	if k := block / 158; k < 1.55 || k > 1.61 {
		t.Errorf("rows grew %.2fx (158pt -> %.0fpt), want the 1.6x limit", k, block)
	}
	if block <= plainBlock {
		t.Errorf("zoomed block %.0fpt is no taller than the stepped block %.0fpt", block, plainBlock)
	}
	if share := block / 400; share > composeZoomFill+0.01 {
		t.Errorf("zoomed block takes %.0f%% of the area, want at most %.0f%%", 100*share, 100*composeZoomFill)
	}
	// Placed at the optical centre: more of the spare height under it.
	above := float64(top-zoom.Bounds.Y) / 12700
	below := float64(zoom.Bounds.Y+zoom.Bounds.CY-bottom) / 12700
	if above <= 0 || below <= above {
		t.Errorf("zoomed block has %.0fpt above and %.0fpt below, want it at the optical centre", above, below)
	}
}

// Type follows the rows and never outgrows what fits: body text that would
// wrap past its rows at two steps takes one, and text that fits no step
// keeps its size while the rows take at most 1.35x of air.
func TestComposeZoomStepsOnlyAsFarAsTheTextFits(t *testing.T) {
	long := strings.Repeat("A sentence that fills its column at the body size. ", 3)
	g := headedColumnsGrid(4, 44, 110, long)
	g.ComposeZoom = true
	res, top, bottom := resolvedBlock(t, g)
	got := sizesOf(res)
	if strings.Contains(got, "24") || strings.HasSuffix(got, ",18") {
		t.Errorf("sizes = %s: text that does not fit two steps must not take them", got)
	}
	if k := float64(bottom-top) / 12700 / 158; k > composeBandMaxScale+0.01 {
		t.Errorf("rows grew %.2fx, over the 1.6x limit", k)
	}
	for _, c := range res.Cells {
		if c.ShapeSpec != nil && strings.Contains(string(c.ShapeSpec.Text), "fontScale") {
			t.Errorf("zoomed text stores a shrink: %s", c.ShapeSpec.Text)
		}
	}
}

// A block of one text size never passes the 18pt lead step, a block already
// at the target keeps its rows, and a dense block or a grid that did not ask
// is not touched: short content is not turned into a billboard.
func TestComposeZoomCaps(t *testing.T) {
	label, _ := json.Marshal(map[string]any{"content": "Label", "size": 14, "bold": true})
	one := headedColumnsGrid(3, 44, 110, "x")
	for i := range one.Rows[1].Cells {
		one.Rows[1].Cells[i].Shape.Text = label
	}
	one.ComposeZoom = true
	res, _, _ := resolvedBlock(t, one)
	if got := sizesOf(res); got != "18,18,18,18,18,18" {
		t.Errorf("one-size block zoomed to %s, want every label at the 18pt lead step", got)
	}

	// 120 + 4 + 160 = 284pt: 71% of the area, at the target already.
	full := headedColumnsGrid(3, 120, 160, "Three short words")
	_, t0, b0 := resolvedBlock(t, full)
	fullZoom := headedColumnsGrid(3, 120, 160, "Three short words")
	fullZoom.ComposeZoom = true
	_, t1, b1 := resolvedBlock(t, fullZoom)
	if (b1 - t1) > (b0-t0)+PtToEMU(1) {
		t.Errorf("a block at the target grew from %dpt to %dpt", (b0-t0)/12700, (b1-t1)/12700)
	}

	// 150 + 4 + 180 = 334pt: 83.5% of the area is dense and keeps its sizes.
	dense := headedColumnsGrid(3, 150, 180, "Three short words")
	dense.ComposeZoom = true
	res, _, _ = resolvedBlock(t, dense)
	if got := sizesOf(res); got != "14,14,14,12,12,12" {
		t.Errorf("dense block sizes = %s, want the pattern's own 14 / 12", got)
	}

	// Inside a compose segment or a cell the grid is not the slide's block.
	nested := headedColumnsGrid(3, 44, 110, "Three short words")
	nested.Compose, nested.ComposeZoom = false, true
	res, _, _ = resolvedBlock(t, nested)
	if got := sizesOf(res); got != "14,14,14,12,12,12" {
		t.Errorf("nested grid sizes = %s, want the pattern's own 14 / 12", got)
	}
}
