package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// loneRowGrid is a slide's own block of one content-sized row of three
// unpainted figure cells with hairline dividers, in a 400pt-tall area: the
// grid an open kpi-Nup strip expands to.
func loneRowGrid(rowPt float64) *Grid {
	text, _ := json.Marshal(map[string]any{
		"paragraphs": []map[string]any{
			{"content": "48%", "size": 48, "bold": true, "figure": true},
			{"content": "Share of revenue", "size": 18},
		},
		"vertical_align": "t",
	})
	cell := func(divider bool) Cell {
		c := Cell{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: text}}
		if divider {
			c.AccentBar = &AccentBarSpec{Position: "left", Color: "888888", Width: 0.75}
		}
		return c
	}
	return &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: PtToEMU(100), CX: PtToEMU(900), CY: PtToEMU(400)},
		Columns: []float64{100.0 / 3, 100.0 / 3, 100.0 / 3},
		VAlign:  VAlignAuto,
		Compose: true,
		Rows:    []Row{{MaxHeight: rowPt, Cells: []Cell{cell(false), cell(true), cell(true)}}},
	}
}

// A lone content-sized row marked ComposeGrow is grown into the free height:
// to half the area, at most 1.8x the height its content needs. The shapes
// keep that content height inside the taller row, so no text is stretched and
// the dividers span the band (go-slide-creator-i7yju).
func TestComposeGrowsALoneRow(t *testing.T) {
	const eps = 12700 // 1pt
	for _, tc := range []struct {
		name       string
		rowPt      float64
		wantCellPt float64
	}{
		{"capped by 1.8x its content", 100, 180},
		{"capped by half the area", 150, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plain := loneRowGrid(tc.rowPt)
			base, _, _ := resolvedBlock(t, plain)
			grown := loneRowGrid(tc.rowPt)
			grown.ComposeGrow = true
			res, top, bottom := resolvedBlock(t, grown)

			if got, want := bottom-top, PtToEMU(tc.wantCellPt); got < want-eps || got > want+eps {
				t.Errorf("grown row is %.0fpt tall, want %.0fpt", float64(got)/12700, tc.wantCellPt)
			}
			for i, c := range res.Cells {
				if d := c.Bounds.CY - base.Cells[i].Bounds.CY; d < -eps || d > eps {
					t.Errorf("cell %d: shape is %.0fpt tall, want its content height %.0fpt", i, float64(c.Bounds.CY)/12700, float64(base.Cells[i].Bounds.CY)/12700)
				}
				if c.Bounds.Y < c.CellBounds.Y || c.Bounds.Y+c.Bounds.CY > c.CellBounds.Y+c.CellBounds.CY {
					t.Errorf("cell %d: shape leaves its grown cell", i)
				}
				if c.Bounds.Y != res.Cells[0].Bounds.Y {
					t.Errorf("cell %d: shapes no longer share a top edge", i)
				}
			}
			if len(res.AccentBars) != 2 {
				t.Fatalf("dividers = %d, want 2", len(res.AccentBars))
			}
			for _, bar := range res.AccentBars {
				if bar.Bounds.CY < bottom-top-eps {
					t.Errorf("divider is %.0fpt tall, want the grown row's %.0fpt", float64(bar.Bounds.CY)/12700, float64(bottom-top)/12700)
				}
			}
			// The band is composed at the optical centre: more air below
			// than above, and neither band a third of the area.
			above, below := top-grown.Bounds.Y, grown.Bounds.Y+grown.Bounds.CY-bottom
			if above > below || float64(below) >= float64(grown.Bounds.CY)/3 {
				t.Errorf("bands above/below = %.0f/%.0fpt of a 400pt area", float64(above)/12700, float64(below)/12700)
			}
		})
	}
}

// Growth is the caller's decision and only for a single content-sized row: a
// grid without ComposeGrow, a block of two rows and a dense row keep the
// heights the policy gave them before.
func TestComposeGrowLeavesOtherBlocksAlone(t *testing.T) {
	height := func(g *Grid) int64 {
		_, top, bottom := resolvedBlock(t, g)
		return bottom - top
	}
	plain := height(loneRowGrid(100))

	twoRows := loneRowGrid(100)
	twoRows.Rows = append(twoRows.Rows, twoRows.Rows[0])
	twoRows.RowGap = 8
	want := height(twoRows)
	twoRows.ComposeGrow = true
	if got := height(twoRows); got != want {
		t.Errorf("two-row block grew from %d to %d EMU", want, got)
	}

	dense := loneRowGrid(320)
	want = height(dense)
	dense.ComposeGrow = true
	if got := height(dense); got != want {
		t.Errorf("dense row (80%% of the area) grew from %d to %d EMU", want, got)
	}

	if plain >= PtToEMU(180)-12700 {
		t.Errorf("a row without ComposeGrow is %.0fpt tall: it must keep its content-sized height", float64(plain)/12700)
	}
}
