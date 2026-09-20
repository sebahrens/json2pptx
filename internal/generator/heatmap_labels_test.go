package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-3rkpt: a dense heatmap drew every label at full length into
// a box one cell tall, so a 14x14 grid of long labels rendered them on top of
// each other and clipped the column headers, with nothing reporting it.

// bigBounds is a full content area on a 16:9 slide.
var heatmapTestBounds = types.BoundingBox{X: 0, Y: 0, Width: 10515600, Height: 3913340}

func heatmapData(rows, cols int, rowLabel, colLabel string) heatmapParsedData {
	p := heatmapParsedData{}
	for i := 0; i < rows; i++ {
		p.rowLabels = append(p.rowLabels, rowLabel)
		row := make([]float64, cols)
		p.values = append(p.values, row)
	}
	for i := 0; i < cols; i++ {
		p.colLabels = append(p.colLabels, colLabel)
	}
	return p
}

// A sparse heatmap whose labels already fit must be left exactly alone — the
// bead's "a 5x4 heatmap is unchanged and draws nothing".
func TestHeatmapSparseLabelsUntouched(t *testing.T) {
	p := heatmapData(5, 4, "Workstream A", "Q1")
	before := append([]string(nil), p.rowLabels...)
	if n := fitHeatmapLabels(&p, heatmapTestBounds); n != 0 {
		t.Errorf("shortened %d labels on a 5x4; nothing should have been cut", n)
	}
	for i := range before {
		if p.rowLabels[i] != before[i] {
			t.Errorf("row label %d changed from %q to %q", i, before[i], p.rowLabels[i])
		}
	}
	if heatmapLabelFinding(5, 4, 0, "p") != nil {
		t.Error("a heatmap that fits must emit no finding")
	}
}

// A dense heatmap with long labels shortens them rather than overlapping.
func TestHeatmapDenseLabelsAreShortened(t *testing.T) {
	const rowLabel = "Geschaeftsbereich A Wirtschaftlichkeitsberechnung"
	const colLabel = "Lieferantenrahmenvertraege 1"
	p := heatmapData(14, 14, rowLabel, colLabel)
	n := fitHeatmapLabels(&p, heatmapTestBounds)
	if n == 0 {
		t.Fatal("a 14x14 grid of long labels shortened nothing")
	}
	// The invariant that stops the overlap: after fitting, EVERY label fits the
	// box it will be drawn into. Whether a given label needed cutting depends
	// on the placeholder, so assert the postcondition rather than the count.
	assertHeatmapLabelsFit(t, p, heatmapTestBounds)
	for i, got := range p.colLabels {
		if got == colLabel {
			continue // this one happened to fit
		}
		if !strings.HasSuffix(got, heatmapLabelEllipsis) {
			t.Errorf("column label %d = %q; a shortened label should say so with an ellipsis", i, got)
		}
	}
}

// A shorter placeholder is where the row labels themselves stop fitting — the
// case the report hit, where each row is barely a line tall.
func TestHeatmapShortPlaceholderShortensRowLabels(t *testing.T) {
	tight := types.BoundingBox{X: 0, Y: 0, Width: 10515600, Height: 2200000}
	p := heatmapData(14, 14, "Geschaeftsbereich A Wirtschaftlichkeitsberechnung", "Q1 2025")
	if n := fitHeatmapLabels(&p, tight); n == 0 {
		t.Fatal("nothing was shortened in a 14-row grid 2.2M EMU tall")
	}
	assertHeatmapLabelsFit(t, p, tight)
	for i, got := range p.rowLabels {
		if !strings.HasSuffix(got, heatmapLabelEllipsis) {
			t.Errorf("row label %d = %q; it cannot fit a %d EMU row at full length", i, got, heatmapGeometryFor(tight, 14, 14, true, true).cellH)
		}
	}
}

// assertHeatmapLabelsFit checks every label against the box it is drawn into.
func assertHeatmapLabelsFit(t *testing.T, p heatmapParsedData, bounds types.BoundingBox) {
	t.Helper()
	rows, cols := len(p.values), 0
	if rows > 0 {
		cols = len(p.values[0])
	}
	g := heatmapGeometryFor(bounds, rows, cols, len(p.rowLabels) > 0, len(p.colLabels) > 0)
	for i, got := range p.rowLabels {
		if h, budget := heatmapLabelHeight(got, g.rowLabelW-heatmapGap), heatmapLineBudget(g.rowLabelW-heatmapGap, g.cellH); h > budget {
			t.Errorf("row label %d (%q) wraps to %d EMU against a %d EMU budget — it will overlap its neighbour", i, got, h, budget)
		}
	}
	for i, got := range p.colLabels {
		if h, budget := heatmapLabelHeight(got, g.cellW), heatmapLineBudget(g.cellW, g.colLabelH); h > budget {
			t.Errorf("column label %d (%q) wraps to %d EMU against a %d EMU budget — it will clip", i, got, h, budget)
		}
	}
}

// The finding is the other half: the render is legible but the labels no longer
// distinguish the rows, so the author has to be told.
func TestHeatmapLabelFindingNamesTheGrid(t *testing.T) {
	f := heatmapLabelFinding(14, 14, 28, "/slides/0/content/body")
	if f == nil {
		t.Fatal("no finding for 28 shortened labels")
	}
	for _, want := range []string{"14x14", "196 cells", "28 label"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message %q does not mention %q", f.Message, want)
		}
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review — the deck still renders", f.Action)
	}
	if f.Fix == nil || f.Fix.Params["cells"] != 196 {
		t.Errorf("fix params should carry the cell count, got %+v", f.Fix)
	}
}

// Registration measures against the same geometry the builder draws with, so a
// label fitted at one and drawn at the other cannot drift.
func TestHeatmapGeometryMatchesTheDrawnGrid(t *testing.T) {
	g := heatmapGeometryFor(heatmapTestBounds, 14, 14, true, true)
	if g.rowLabelW != heatmapRowLabelWidth || g.colLabelH != heatmapColLabelHeight {
		t.Errorf("label bands = %d x %d, want the reserved constants", g.rowLabelW, g.colLabelH)
	}
	wantCellW := (heatmapTestBounds.Width - heatmapRowLabelWidth - 13*heatmapGap) / 14
	wantCellH := (heatmapTestBounds.Height - heatmapColLabelHeight - 13*heatmapGap) / 14
	if g.cellW != wantCellW || g.cellH != wantCellH {
		t.Errorf("cells = %dx%d, want %dx%d", g.cellW, g.cellH, wantCellW, wantCellH)
	}
	// No labels reserves no band.
	if bare := heatmapGeometryFor(heatmapTestBounds, 4, 4, false, false); bare.rowLabelW != 0 || bare.colLabelH != 0 {
		t.Errorf("an unlabelled heatmap reserved %d x %d", bare.rowLabelW, bare.colLabelH)
	}
}

// A box too small for even one character keeps the label: replacing it with a
// lone ellipsis tells the reader less than the overflow did, and the finding
// already carries the signal.
func TestHeatmapImpossibleBoxKeepsTheLabel(t *testing.T) {
	got, cut := fitLabelToBox("Workstream A", 1000, 1000)
	if cut || got != "Workstream A" {
		t.Errorf("fitLabelToBox on an impossible box = (%q, %v), want the label untouched", got, cut)
	}
}
