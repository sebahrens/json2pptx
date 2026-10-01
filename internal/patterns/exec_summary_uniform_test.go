package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// go-slide-creator-a47pk: the list_slide_kinds executive_summary example
// rendered rows 1–2 with 12pt supports and row 3 with 14pt. The pattern sized
// every row at one type step, but the deck's type-scale growth then grew each
// cell on its own spare height, so the taller third row grew a step on its
// own. Expanded and resolved through the shape grid with the deck default
// type scale, every lead run and every support run must share one size.
func TestExecSummaryRowsShareOneSizePerColumn(t *testing.T) {
	values := &ExecSummaryValues{
		Points: []ExecSummaryPoint{
			{Lead: "Growth is ahead of plan.", Support: "Revenue grew 41% year over year to $48M, against a 30% plan."},
			{Lead: "Enterprise is carrying the mix.", Support: "It now drives 55% of new bookings, up from 38% last year."},
			{Lead: "SMB retention is the one real risk.", Support: "Monthly churn rose to 3.1%, concentrated in the sub-50-seat tier."},
		},
		BottomLine: "Fund an SMB retention pod in Q3 and hold the enterprise motion as is.",
	}
	areas := []struct {
		name string
		w, h float64
	}{
		{"abstract", 687, 294},
		{"midnight-blue", 796, 330},
		{"p-style", 899, 370},
	}
	for _, mode := range []string{"comfortable", "presentation"} {
		for _, a := range areas {
			t.Run(mode+"/"+a.name, func(t *testing.T) {
				ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}}
				grid, err := execSummaryPattern(t).Expand(ctx, values, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				ApplyGridDefaults(grid)
				grid.TypeScale = mode
				res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
				sizes := map[int]map[int]string{} // column -> run size -> first text
				for _, c := range res.Cells {
					if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 || c.RowIdx%2 != 0 || c.RowIdx >= 2*len(values.Points) {
						continue // rules, the bottom line
					}
					tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
					if err != nil || tb == nil {
						continue
					}
					for _, p := range tb.Paragraphs {
						for _, r := range p.Runs {
							if strings.TrimSpace(r.Text) == "" {
								continue
							}
							if sizes[c.ColIdx] == nil {
								sizes[c.ColIdx] = map[int]string{}
							}
							sizes[c.ColIdx][r.FontSize] = r.Text
						}
					}
				}
				for col, bySize := range sizes {
					if len(bySize) != 1 {
						t.Errorf("column %d renders at %d sizes: %v", col, len(bySize), bySize)
					}
				}
			})
		}
	}
}

// go-slide-creator-n7q73: plain-string points carry no supporting sentence.
// The lead used to sit in the 36% lead column beside an empty support column;
// without any support the lead spans the width.
func TestExecSummaryLeadsOnlySpanTheWidth(t *testing.T) {
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(796 * 12700), Height: int64(330 * 12700)}}
	values := &ExecSummaryValues{Points: []ExecSummaryPoint{
		{Lead: "Revenue grew 18% QoQ, ahead of plan."},
		{Lead: "Net revenue retention held at 118%."},
		{Lead: "SMB logo churn ticked up to 2.4%."},
		{Lead: "Sales cycle shortened to 41 days."},
	}}
	grid, err := execSummaryPattern(t).Expand(ctx, values, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cols := execSummaryColumns(true, false)
	if len(cols) != 2 || cols[1] != 100-execSummaryNumColPct {
		t.Fatalf("leads-only columns = %v, want number + full-width lead", cols)
	}
	for i := 0; i < len(values.Points); i++ {
		if got := len(grid.Rows[2*i].Cells); got != 2 {
			t.Errorf("point row %d has %d cells, want number + lead", i, got)
		}
	}
	// One line per lead at the full width: each point row is a single-line row.
	_, lay := layoutExecSummary(ctx, values, &ExecSummaryOverrides{})
	areaW, _ := sizingAreaPt(ctx)
	leadW := (areaW - execSummaryColGapPt) * cols[1] / 100
	oneLine := sizedBlockHeightPt(ctx, []sizedPara{{text: "Revenue", sizePt: lay.leadSize, bold: true}}, leadW)
	for i, p := range values.Points {
		if h := sizedBlockHeightPt(ctx, []sizedPara{{text: p.Lead, sizePt: lay.leadSize, bold: true}}, leadW); h > oneLine {
			t.Errorf("lead %d needs %.0fpt (one line is %.0fpt); it should hold one line across the width", i, h, oneLine)
		}
	}
	// With supports the lead column is at least 45% of the width.
	if c := execSummaryColumns(true, true); c[1] < 45 {
		t.Errorf("lead column = %v%%, want >= 45%%", c[1])
	}
}

// go-slide-creator-n7q73: with the template's content area the warning is the
// measured fit, not the worst-case character budget. Four 43–69 character
// leads with one-sentence supports and a bottom line fit on two lines each,
// yet the budget (42 lead characters) flagged every lead and blocked the
// quality gate.
func TestExecSummaryMeasuredWarningsReplaceCharacterBudgets(t *testing.T) {
	values := &ExecSummaryValues{
		Points: []ExecSummaryPoint{
			{Lead: "Revenue grew 18% YoY to $42M, ahead of plan", Support: "Enterprise bookings and the EMEA launch drove the step-up."},
			{Lead: "Gross margin reached 61%, 3 points above plan", Support: "Hosting consolidation cut COGS per customer by 9%."},
			{Lead: "SMB churn rose to 4.1% and is the one metric moving the wrong way", Support: "Exit surveys cite onboarding gaps; time-to-first-value is 31 days."},
			{Lead: "Q4 priorities are the enterprise tier, 12 AE hires and SMB onboarding", Support: "Approve the $2.4M hiring budget by 15 October to land Q1 ramp."},
		},
		BottomLine: "Approve the hiring budget and the onboarding fix by 15 October.",
	}
	if got := execSummaryBudgetWarnings(values); len(got) == 0 {
		t.Fatal("precondition: the character budgets flag these leads")
	}
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(796 * 12700), Height: int64(330 * 12700)}}
	if got := (&execSummary{}).PostExpandWarnings(ctx, values, nil); len(got) != 0 {
		t.Errorf("measured layout fits but warned: %v", got)
	}
}
