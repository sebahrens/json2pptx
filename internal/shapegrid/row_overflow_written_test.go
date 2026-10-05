package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// kpiCellText is a KPI card's text as kpi-4up wrote it on midnight-blue: a
// figure over a caption, a reserved blank line and a comparator.
const kpiCellText = `{"paragraphs":[
	{"content":"14","size":32,"bold":true,"color":"accent1","align":"ctr","figure":true},
	{"content":"Findings raised","size":16,"color":"dk1","align":"ctr"},
	{"content":"\u00a0","size":13.6,"color":"dk1","align":"ctr"},
	{"content":"3 high / 6 med / 5 low","size":13.6,"color":"dk1","align":"ctr"}],
	"align":"ctr","vertical_align":"t"}`

// kpiRowFonts are the theme faces the pattern sized the row in; the writer
// measures in them, where the paragraph estimate assumes one line factor.
var kpiRowFonts = pptx.ThemeFonts{Major: "Calibri", Minor: "Calibri"}

// writtenFitRowPt is the shortest row (whole points) in which the writer
// stores text with no autofit shrink in a widthEMU-wide cell.
func writtenFitRowPt(t *testing.T, text string, widthEMU int64) float64 {
	t.Helper()
	tb, err := ResolveTextInput(json.RawMessage(text))
	if err != nil || tb == nil {
		t.Fatalf("resolve text: %v", err)
	}
	tb.ThemeFonts = kpiRowFonts
	for h := 20.0; h < 400; h++ {
		if pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: widthEMU, CY: PtToEMU(h)}) {
			return h
		}
	}
	t.Fatal("text fits no row under 400pt")
	return 0
}

func kpiRowGrid(maxHeightPt float64) *Grid {
	cell := func() Cell {
		return Cell{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(kpiCellText), ThemeFonts: kpiRowFonts}}
	}
	return &Grid{
		// The grid is exactly as tall as the row: nothing else gives it height.
		Bounds:  pptx.RectEmu{CX: 10000000, CY: PtToEMU(maxHeightPt)},
		Columns: []float64{25, 25, 25, 25},
		Rows:    []Row{{MinHeight: maxHeightPt, MaxHeight: maxHeightPt, Cells: []Cell{cell(), cell(), cell(), cell()}}},
	}
}

// A kpi-4up row its pattern had sized to the writer's own fit was reported as
// overflowing ("needs about 119pt of height and has 107pt") by the paragraph
// estimate, with a taller-row fix for a slide whose lower half was empty: the
// estimate adds line heights at one factor, the writer measures the same text
// in its face at the cell's resolved bounds. Where the writer stores every
// text cell of the row unshrunk, the row does not overflow
// (go-slide-creator-18dqh, journey g-A10).
func TestResolve_RowOverflowFollowsTheWritersMeasure(t *testing.T) {
	colW := int64(10000000-3*PtToEMU(8)) / 4
	fit := writtenFitRowPt(t, kpiCellText, colW) + 2

	// The estimate alone would report this row: guard the premise.
	g := kpiRowGrid(fit)
	if est := float64(estimateRowTextHeightEMU(g.Rows[0])) / 12700; est <= fit {
		t.Skipf("the paragraph estimate (%.0fpt) no longer exceeds the written fit (%.0fpt)", est, fit)
	}
	res, err := Resolve(g, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RowOverflows) != 0 {
		t.Errorf("a row at the written fit of its text (%.0fpt) was reported as overflowing: %+v", fit, res.RowOverflows)
	}

	// A row the writer does shrink the text in keeps its overflow.
	short := fit * 0.6
	res, err = Resolve(kpiRowGrid(short), newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RowOverflows) != 1 || res.RowOverflows[0].MaxHeightPt != short {
		t.Errorf("a %.0fpt row for text that needs %.0fpt must report its overflow, got %+v", short, fit, res.RowOverflows)
	}
}
