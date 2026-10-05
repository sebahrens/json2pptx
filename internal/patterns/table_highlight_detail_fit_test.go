package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// A status board of five options with a one-line detail under each name
// (go-slide-creator-u8orh): readable when the content area holds the rows,
// and reported by what it is short of when it does not.

func statusBoardValues() *TableHighlightValues {
	v := &TableHighlightValues{
		CornerLabel: "Domain",
		Criteria:    []TableHighlightCriterion{{Label: "Controls tested", Scale: "text"}, {Label: "Exceptions", Scale: "text"}, {Label: "Rating", Scale: "rag"}},
	}
	for _, o := range [][2]string{
		{"Access management", "Joiners, movers, leavers"},
		{"Change management", "Standard and emergency change"},
		{"Computer operations", "Batch, backup and scheduling"},
		{"Program development", "SDLC gates and release approval"},
		{"Third-party / cloud", "Provider assurance"},
	} {
		v.Options = append(v.Options, TableHighlightOption{Name: o[0], Detail: o[1], Scores: []TableHighlightScore{"14", "3", "red"}})
	}
	return v
}

// statusBoardScale is the shrink the writer stores for a resolved text cell:
// the row's shared one when the grid set it, else the cell's own.
func statusBoardScale(t *testing.T, ctx ExpandContext, c shapegrid.ResolvedCell) float64 {
	t.Helper()
	if c.AutofitScale != 0 {
		return c.AutofitScale
	}
	tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
	if err != nil || tb == nil {
		t.Fatalf("cell text: %v", err)
	}
	for j := range tb.Insets {
		tb.Insets[j] += c.TextInsets[j]
	}
	return writtenScaleIn(ctx, tb, c.Bounds)
}

func statusBoardCtx(heightPt float64) ExpandContext {
	ctx := testThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: int64(830 * 12700), Height: int64(heightPt * 12700)}
	return ctx
}

func TestTableHighlightOneLineDetailsReadableWhereTheTableFits(t *testing.T) {
	pat := &tableHighlight{}
	v := statusBoardValues()

	// The area holds the table: no finding, and the rows are written at the
	// 12pt body size in a box their text fits.
	fits := statusBoardCtx(300)
	if got := pat.PostExpandWarnings(fits, v, nil); len(got) != 0 {
		t.Fatalf("five one-line details in a 300pt area: %v", got)
	}
	grid, err := pat.Expand(fits, v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyGridDefaults(grid)
	res := resolveGridAt(t, grid, pptx.RectEmu{CX: fits.LayoutBounds.Width, CY: fits.LayoutBounds.Height})
	checked := 0
	for _, cell := range res.Cells {
		if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 || !strings.Contains(string(cell.ShapeSpec.Text), "Joiners") {
			continue
		}
		checked++
		if scale := statusBoardScale(t, fits, cell); scale < 1 {
			t.Errorf("the name / detail cell is written with a %.0f%% shrink in an area that holds the table", scale*100)
		}
	}
	if checked != 1 {
		t.Fatalf("found %d name / detail cells for the first option", checked)
	}

	// An area the table is taller than: the capacity finding, and a detail
	// finding per option (what to drop).
	short := statusBoardCtx(230)
	got := pat.PostExpandWarnings(short, v, nil)
	if len(got) != 1+len(v.Options) || !strings.Contains(got[0], "needs about") || !strings.Contains(got[1], "no readable detail") {
		t.Fatalf("five one-line details in a 230pt area: %v", got)
	}

	// No measured area: the budget is the only answer there is.
	if got := pat.PostExpandWarnings(testThemeCtx(), v, nil); len(got) != len(v.Options) {
		t.Fatalf("five details with no measured area: %v", got)
	}
}

// An over-full table shrinks by what it is short of. Rows that went back to
// the uniform margin lost the same points from far less text height, and a
// table 5pt too tall was predicted at 6.5pt.
func TestTableHighlightOverfullTableShrinksByItsShortfall(t *testing.T) {
	pat := &tableHighlight{}
	v := statusBoardValues()
	l := newTHLayout(statusBoardCtx(1000), v, &TableHighlightOverrides{})
	l.tight, l.legendPt, l.padPt = true, thLegendCompactPt, rowPadStepsPt[len(rowPadStepsPt)-1]
	l.measure(11, 12, 12)
	need := l.total()

	for _, shortPt := range []float64{4, 12, 30} {
		ctx := statusBoardCtx(need - shortPt)
		grid, err := pat.Expand(ctx, v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ApplyGridDefaults(grid)
		res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
		worst := 1.0
		for _, cell := range res.Cells {
			if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 {
				continue
			}
			worst = min(worst, statusBoardScale(t, ctx, cell))
		}
		// The rows give up shortPt of need; text loses a little more than that
		// share because its margins do not shrink with it.
		share := (need - shortPt) / need
		if worst >= 1 {
			t.Errorf("%.0fpt short of %.0fpt: no cell shrinks", shortPt, need)
		}
		if floor := share - 0.12; worst < floor {
			t.Errorf("%.0fpt short of %.0fpt: text shrinks to %.0f%%, far under the table's own %.0f%%", shortPt, need, worst*100, share*100)
		}
	}
}
