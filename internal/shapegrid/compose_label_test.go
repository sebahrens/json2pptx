package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// A short label that sat on one line is not wrapped by the step: a KPI
// caption broken as "Logo churn (SMB-" / "weighted)" at the stepped size reads
// worse than the caption whole at its own size (go-slide-creator-wwmod). A
// sentence may take another line.
func TestComposeKeepsShortLabelsOnOneLine(t *testing.T) {
	text := func(s string, size float64) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"content": s, "size": size})
		return raw
	}
	grid := func(body string) *Grid {
		g := &Grid{
			Bounds:  pptx.RectEmu{X: 0, Y: 1000000, CX: PtToEMU(880), CY: PtToEMU(300)},
			Columns: []float64{25, 25, 25, 25},
			VAlign:  VAlignAuto,
			Compose: true,
			RowGap:  8,
		}
		row := Row{MaxHeight: 80}
		for i := 0; i < 4; i++ {
			row.Cells = append(row.Cells, Cell{Shape: &ShapeSpec{Geometry: "rect", Text: text(body, 14)}})
		}
		g.Rows = []Row{row}
		return g
	}
	// Three words: one line at 14pt in a 220pt column, two at 18pt.
	res, _, _ := resolvedBlock(t, grid("Logo churn (SMB-weighted)"))
	if got := sizesOf(res); got != "14,14,14,14" {
		t.Errorf("label sizes = %s, want the 14pt label kept (it would wrap at 18pt)", got)
	}
	if !res.Composed {
		t.Error("the block keeps its sizes but is still placed")
	}
	// A short label that still fits takes the step.
	res, _, _ = resolvedBlock(t, grid("Logo churn"))
	if got := sizesOf(res); got != "18,18,18,18" {
		t.Errorf("short label sizes = %s, want the step to 18pt", got)
	}
	// A sentence may wrap onto another line.
	res, _, _ = resolvedBlock(t, grid("Logo churn weighted by customer segment"))
	if got := sizesOf(res); got != "18,18,18,18" {
		t.Errorf("sentence sizes = %s, want the step to 18pt", got)
	}
}
