package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// What a table-highlight gives up before its text (go-slide-creator-dwha2).
//
// Two agent runs sent a status board of five rows and three columns with a
// line of detail under each name and a takeaway. At the tightest row padding
// it was 5pt (one board) and 35pt (the other, with a tag line on two rows)
// taller than the 253pt content area of the shipped templates under a
// takeaway band and a source line, and was refused at 11.8pt and 9.8pt. The
// height was there, none of it in the text: the padding above the header
// labels, a legend row sized for a wrapped legend, the tag's own line, a point
// of row padding, and a legend that reads "Green / Amber / Red".

// The content areas the boards were sent to, in points: the shipped templates
// under a takeaway band and a source line.
var thShippedAreasPt = map[string][2]float64{
	"midnight-blue":   {796.8, 253.1},
	"forest-green":    {828.0, 253.1},
	"blue-corporate":  {827.5, 248.7},
	"modern-template": {824.4, 239.3},
	"modern":          {852.9, 214.8},
	"p-style":         {899.3, 263.7},
}

func thAreaCtx(area [2]float64) ExpandContext {
	ctx := testThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: int64(area[0] * 12700), Height: int64(area[1] * 12700)}
	return ctx
}

// auditBoardValues is the ITGC results board (persona i).
func auditBoardValues() *TableHighlightValues {
	col := 2
	v := &TableHighlightValues{
		CornerLabel:  "Domain",
		Criteria:     []TableHighlightCriterion{{Label: "Controls tested", Scale: "text"}, {Label: "Exceptions", Scale: "text"}, {Label: "Rating", Scale: "rag"}},
		HighlightCol: &col,
	}
	for _, o := range [][5]string{
		{"Access management", "Joiners, movers, leavers; privileged access", "14", "3", "red"},
		{"Change management", "Standard and emergency change", "11", "1", "amber"},
		{"Computer operations", "Batch, backup, incident and job scheduling", "9", "0", "green"},
		{"Program development", "SDLC gates, testing and release approval", "6", "1", "amber"},
		{"Third-party / cloud", "Provider assurance and contract controls", "8", "2", "red"},
	} {
		v.Options = append(v.Options, TableHighlightOption{Name: o[0], Detail: o[1], Scores: []TableHighlightScore{TableHighlightScore(o[2]), TableHighlightScore(o[3]), TableHighlightScore(o[4])}})
	}
	return v
}

// appetiteBoardValues is the risk appetite board (persona g): two tagged rows.
func appetiteBoardValues() *TableHighlightValues {
	row, col := 2, 0
	v := &TableHighlightValues{
		CornerLabel:    "Risk type",
		Criteria:       []TableHighlightCriterion{{Label: "Status", Scale: "rag"}, {Label: "Current", Scale: "text"}, {Label: "Limit", Scale: "text"}},
		HighlightRow:   &row,
		HighlightRows:  []int{4},
		HighlightCol:   &col,
		HighlightLabel: "Breached",
	}
	for _, o := range [][5]string{
		{"Credit", "Non-performing loans", "green", "NPL 2.1%", "3%"},
		{"Liquidity", "Liquidity coverage ratio", "green", "LCR 148%", "110%"},
		{"Operational", "Annual losses", "red", "EUR 11.2M", "EUR 8M"},
		{"Conduct", "Complaints per 10k customers", "amber", "37", "30"},
		{"Cyber", "Critical vulnerabilities unpatched >30 days", "red", "2", "0"},
	} {
		v.Options = append(v.Options, TableHighlightOption{Name: o[0], Detail: o[1], Scores: []TableHighlightScore{TableHighlightScore(o[2]), TableHighlightScore(o[3]), TableHighlightScore(o[4])}})
	}
	return v
}

// thWrittenAtSize fails for every text cell of the expanded table that the
// writer would shrink in its resolved box.
func thWrittenAtSize(t *testing.T, name string, ctx ExpandContext, v *TableHighlightValues) {
	t.Helper()
	grid, err := (&tableHighlight{}).Expand(ctx, v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyGridDefaults(grid)
	res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
	texts := 0
	for _, cell := range res.Cells {
		if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 {
			continue
		}
		texts++
		if scale := statusBoardScale(t, ctx, cell); scale < 1 {
			t.Errorf("%s: %s is written with a %.0f%% shrink", name, cell.ShapeSpec.Text, scale*100)
		}
	}
	if texts < len(v.Options)+len(v.Criteria) {
		t.Fatalf("%s: only %d text cells resolved", name, texts)
	}
}

func TestTableHighlightStatusBoardsFitTheShippedContentAreas(t *testing.T) {
	pat := &tableHighlight{}
	for name, area := range thShippedAreasPt {
		for board, v := range map[string]*TableHighlightValues{"audit": auditBoardValues(), "appetite": appetiteBoardValues()} {
			ctx := thAreaCtx(area)
			if got := pat.PostExpandWarnings(ctx, v, nil); len(got) != 0 {
				t.Errorf("%s board in %s's %.0fpt area: %v", board, name, area[1], got)
				continue
			}
			l := newTHLayout(ctx, v, &TableHighlightOverrides{})
			l.fit()
			if l.total() > l.areaH || l.bodySize < scaleBodyPt || l.detailSize < scaleBodyPt {
				t.Errorf("%s board in %s: %.1fpt in %.1fpt at %v/%vpt", board, name, l.total(), l.areaH, l.bodySize, l.detailSize)
			}
			// Tight is not cramped: no row padding under the table's floor.
			if l.padPt != 0 && l.padPt < thRowPadFloorPt {
				t.Errorf("%s board in %s: row padding %vpt", board, name, l.padPt)
			}
			thWrittenAtSize(t, board+" in "+name, ctx, v)
		}
	}
}

// The steps are taken in order and only as far as the table needs.
func TestTableHighlightGivesUpAirInOrder(t *testing.T) {
	steps := func(heightPt float64, v *TableHighlightValues) *thLayout {
		l := newTHLayout(thAreaCtx([2]float64{830, heightPt}), v, &TableHighlightOverrides{})
		l.fit()
		if l.total() > l.areaH {
			t.Fatalf("board does not fit %.0fpt: %.1fpt", heightPt, l.total())
		}
		return l
	}
	// (statusBoardValues: details that hold one line in the standard column.)
	// Room for the board at the shared padding steps: nothing new is given up.
	roomy := steps(262, statusBoardValues())
	if roomy.headTrim || roomy.slimLegend || roomy.tagInline || len(roomy.legend) != 1 || roomy.cols[0] != 30 || roomy.padPt < 5 {
		t.Errorf("262pt: head=%v slim=%v tag=%v legend=%v col=%v pad=%v, want only the shared padding steps", roomy.headTrim, roomy.slimLegend, roomy.tagInline, roomy.legend, roomy.cols[0], roomy.padPt)
	}
	// 253pt: the header padding and the legend's height; the legend stays.
	mid := steps(253, statusBoardValues())
	if !mid.headTrim || !mid.slimLegend || len(mid.legend) != 1 || mid.padPt < 5 {
		t.Errorf("253pt: head=%v slim=%v legend=%v pad=%v, want a trimmed header, a slim legend and 5pt rows", mid.headTrim, mid.slimLegend, mid.legend, mid.padPt)
	}
	// 240pt: a point of row padding before the legend goes.
	low := steps(240, statusBoardValues())
	if low.padPt != thRowPadFloorPt || len(low.legend) != 1 {
		t.Errorf("240pt: pad=%v legend=%v, want %vpt rows and the legend", low.padPt, low.legend, thRowPadFloorPt)
	}
	// 215pt: the unworded RAG legend goes last.
	last := steps(215, statusBoardValues())
	if len(last.legend) != 0 {
		t.Errorf("215pt: legend=%v, want the default RAG legend dropped", last.legend)
	}
}

// A RAG legend the author worded or asked for is content: the table is
// reported, not stripped of it.
func TestTableHighlightKeepsAWordedOrRequestedLegend(t *testing.T) {
	pat := &tableHighlight{}
	ctx := thAreaCtx([2]float64{830, 215})
	worded := auditBoardValues()
	worded.LegendLabelsRAG = []string{"Effective", "Needs improvement", "Ineffective"}
	on := true
	asked := auditBoardValues()
	asked.ShowLegend = &on
	for name, v := range map[string]*TableHighlightValues{"legend_labels_rag": worded, "show_legend: true": asked} {
		l := newTHLayout(ctx, v, &TableHighlightOverrides{})
		l.fit()
		if len(l.legend) != 1 {
			t.Errorf("%s: the legend row was dropped", name)
		}
		got := pat.PostExpandWarnings(ctx, v, nil)
		if len(got) == 0 || !strings.Contains(got[0], "needs about 234pt") || !strings.Contains(got[0], "holds 215pt") {
			t.Errorf("%s: want the capacity finding with the measured need, got %v", name, got)
		}
	}
	// A Harvey legend names what the balls mean and always stays.
	harvey := auditBoardValues()
	harvey.Criteria[2].Scale = "harvey"
	for i := range harvey.Options {
		harvey.Options[i].Scores[2] = "2"
	}
	l := newTHLayout(ctx, harvey, &TableHighlightOverrides{})
	l.fit()
	if len(l.legend) != 1 || l.needPt <= l.areaH {
		t.Errorf("harvey board in 215pt: legend=%v need=%.1f, want the legend kept and the table reported", l.legend, l.needPt)
	}
}

// The highlight tag joins its name's line when every tagged name holds it on
// one line, and keeps its own line when one does not.
func TestTableHighlightTagJoinsTheNameLine(t *testing.T) {
	ctx := thAreaCtx(thShippedAreasPt["midnight-blue"])
	v := appetiteBoardValues()
	l := newTHLayout(ctx, v, &TableHighlightOverrides{})
	l.fit()
	if !l.tagInline {
		t.Fatalf("253pt: the tag keeps its own line (total %.1fpt)", l.total())
	}
	for i := range l.rowPt {
		if l.rowPt[i] != l.rowPt[0] {
			t.Errorf("rows are not uniform with the tag beside the name: %v", l.rowPt)
			break
		}
	}
	paras := l.nameParas(2, "dk1", fillTone{Color: "none"})
	if len(paras) != 2 || paras[0].Content != "<b>Operational</b>"+thTagSep+"Breached" || paras[0].Bold || paras[0].Size != scaleBodyPt {
		t.Errorf("tagged name cell = %+v, want the bold name, the tag after it and the detail", paras)
	}
	if untagged := l.nameParas(0, "dk1", fillTone{Color: "none"}); len(untagged) != 2 || untagged[0].Content != "Credit" || !untagged[0].Bold {
		t.Errorf("untagged name cell = %+v", untagged)
	}

	long := appetiteBoardValues()
	long.Options[2].Name = "Operational resilience and outsourcing"
	ll := newTHLayout(ctx, long, &TableHighlightOverrides{})
	ll.fit()
	if ll.tagInline {
		t.Errorf("a name that fills its line took the tag beside it")
	}
	if paras := ll.nameParas(2, "dk1", fillTone{Color: "none"}); len(paras) != 3 || paras[2].Content != "Breached" {
		t.Errorf("tag paragraph = %+v, want the tag on its own line", paras)
	}
}

// A detail that wraps in the standard option column is given a wider one when
// that keeps it on one line; the width is measured, never assumed.
func TestTableHighlightWidensTheOptionColumnForOneLineDetails(t *testing.T) {
	v := auditBoardValues()
	for i := range v.Options {
		v.Options[i].Detail = "Joiners, movers and leavers; privileged access review"
	}
	ctx := thAreaCtx([2]float64{830, 253})
	narrow := newTHLayout(ctx, v, &TableHighlightOverrides{})
	narrow.tight, narrow.legendPt, narrow.padPt = true, thLegendCompactPt, 5
	narrow.measure(11, 12, 12)
	l := newTHLayout(ctx, v, &TableHighlightOverrides{})
	l.fit()
	if l.cols[0] <= 30 || l.rowPt[0] >= narrow.rowPt[0] || l.total() > l.areaH {
		t.Fatalf("option column %.0f%%, rows %.0fpt (%.0fpt at 30%%), total %.1fpt in %.1fpt: want a wider column and one-line details", l.cols[0], l.rowPt[0], narrow.rowPt[0], l.total(), l.areaH)
	}
	sum := 0.0
	for _, c := range l.cols {
		sum += c
	}
	if sum < 99.99 || sum > 100.01 || len(l.cols) != 4 {
		t.Errorf("columns %v do not share the table", l.cols)
	}
	thWrittenAtSize(t, "widened", ctx, v)

	// Text scores that need their columns keep them: a wider option column
	// that wraps the scores is not taken.
	wide := auditBoardValues()
	wide.Criteria = []TableHighlightCriterion{{Label: "Owner", Scale: "text"}, {Label: "Status", Scale: "text"}, {Label: "Next step", Scale: "text"}}
	for i := range wide.Options {
		wide.Options[i].Detail = "Joiners, movers and leavers; privileged access review"
		wide.Options[i].Scores = []TableHighlightScore{"Head of IT operations", "Remediation under way", "Re-test in the first quarter"}
	}
	lw := newTHLayout(thAreaCtx([2]float64{830, 300}), wide, &TableHighlightOverrides{})
	lw.fit()
	base := newTHLayout(thAreaCtx([2]float64{830, 300}), wide, &TableHighlightOverrides{})
	base.tight, base.legendPt, base.padPt, base.headTrim = true, thLegendCompactPt, 5, true
	base.measure(11, 12, 12)
	if lw.cols[0] > 30 && lw.rowPt[0] > base.rowPt[0] {
		t.Errorf("option column widened to %.0f%% and the rows grew from %.0fpt to %.0fpt", lw.cols[0], base.rowPt[0], lw.rowPt[0])
	}
}

// The slim legend row holds its 12pt labels: a row the writer has to shrink
// them in is the defect go-slide-creator-z0up fixed.
func TestTableHighlightSlimLegendHoldsItsLabels(t *testing.T) {
	ctx := thAreaCtx(thShippedAreasPt["midnight-blue"])
	v := auditBoardValues()
	v.LegendLabelsRAG = []string{"Operating effectively", "Needs improvement", "Not operating"}
	cell := thLegendCell(ctx, v, thScaleRAG, "dk2", nil, 4, true)
	if cell.Grid == nil || len(cell.Grid.Rows) != 1 || cell.Grid.Rows[0].MaxHeight != thLegendSlimRowPt {
		t.Fatalf("slim legend row = %+v", cell.Grid)
	}
	var cols []float64
	if err := json.Unmarshal(cell.Grid.Columns, &cols); err != nil {
		t.Fatal(err)
	}
	innerW := thShippedAreasPt["midnight-blue"][0] - 2*SubGridInsetPt
	labels := 0
	for i, c := range cell.Grid.Rows[0].Cells {
		if c.Shape == nil || len(c.Shape.Text) == 0 {
			continue
		}
		labels++
		if !strings.Contains(string(c.Shape.Text), `"size":12`) {
			t.Errorf("legend label %s is not set at 12pt", c.Shape.Text)
		}
		if need := writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, innerW*cols[i]/100, 0); need > thLegendSlimRowPt {
			t.Errorf("legend label %s needs %.0fpt in a %.0fpt row", c.Shape.Text, need, thLegendSlimRowPt)
		}
	}
	if labels != 3 {
		t.Fatalf("%d legend labels", labels)
	}
	if thLegendSlimPt != thLegendSlimRowPt+2*SubGridInsetPt {
		t.Errorf("slim legend row %vpt does not hold its nested row and the sub-grid inset", thLegendSlimPt)
	}
}
