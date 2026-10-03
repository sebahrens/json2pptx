package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// go-slide-creator-vg73u: a list at its documented maximum, with one-line
// copy, fits the shortest shipped content areas by giving up row padding — the
// text keeps a readable size, and a list that fits at the uniform margin is
// left exactly as it was.

// shortAreaCtx is modern's blank-title content area under a takeaway band,
// the shortest area a shipped template gives a pattern.
var shortAreaCtx = ExpandContext{LayoutBounds: LayoutBounds{Width: 10831550, Height: 3073400}}

// rowPadInsetOf returns the inset_top / inset_bottom a cell's text is written
// with; ok is false when it keeps the uniform margin.
func rowPadInsetOf(t *testing.T, text json.RawMessage) (top, bottom float64, ok bool) {
	t.Helper()
	var obj struct {
		InsetTop    *float64 `json:"inset_top"`
		InsetBottom *float64 `json:"inset_bottom"`
	}
	if err := json.Unmarshal(text, &obj); err != nil {
		t.Fatalf("text: %v", err)
	}
	if obj.InsetTop == nil || obj.InsetBottom == nil {
		return 0, 0, false
	}
	return *obj.InsetTop, *obj.InsetBottom, true
}

func TestNextStepsSixActionsFitAShortArea(t *testing.T) {
	vals := &NextStepsValues{Decisions: []string{"Approve the €1.2M pod budget for FY27"}}
	for i := 0; i < nextStepsMaxActions; i++ {
		vals.Actions = append(vals.Actions, NextStepsAction{Action: fmt.Sprintf("Confirm pilot scope and success metrics %d", i+1), Owner: "COO", Date: "15 Oct"})
	}
	p := &nextSteps{}
	for name, ctx := range map[string]ExpandContext{
		"modern (311pt)":        {LayoutBounds: LayoutBounds{Width: 10831550, Height: 3949700}},
		"midnight-blue (349pt)": {LayoutBounds: LayoutBounds{Width: 10515600, Height: 4432300}},
	} {
		if w := p.PostExpandWarnings(ctx, vals, nil); len(w) != 0 {
			t.Errorf("%s: six one-line actions are refused: %v", name, w)
		}
		lay := layoutNextSteps(ctx, vals, &NextStepsOverrides{})
		_, areaH := sizingAreaPt(ctx)
		if lay.padPt <= 0 || lay.natural() > areaH {
			t.Errorf("%s: pad=%v natural=%.0f area=%.0f, want tightened rows that fit", name, lay.padPt, lay.natural(), areaH)
		}
		if lay.actionSize < scaleBodyPt || lay.metaSize < scaleBodyPt {
			t.Errorf("%s: action/meta at %v/%vpt, below the 12pt floor", name, lay.actionSize, lay.metaSize)
		}
		grid, err := p.Expand(ctx, vals, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Rules stay between the rows, and every row cell carries the padding.
		if grid.Rows[1].MaxHeight != nextStepsRulePt {
			t.Errorf("%s: row 1 is not a rule", name)
		}
		top, bottom, ok := rowPadInsetOf(t, grid.Rows[2].Cells[1].Shape.Text)
		if !ok || top != lay.padPt || bottom != lay.padPt {
			t.Errorf("%s: action cell insets = %v/%v (set=%v), want %v", name, top, bottom, ok, lay.padPt)
		}
	}

	// Three actions fit at the uniform margin and write no inset.
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := rowPadInsetOf(t, grid.Rows[2].Cells[1].Shape.Text); ok {
		t.Error("a list that fits at the uniform margin wrote a text inset")
	}

	// Six two-line actions and three decisions do not fit the short area at
	// any padding: that is still reported, with the least height it needs.
	long := &NextStepsValues{Decisions: []string{strings.Repeat("d", 100), strings.Repeat("e", 100), strings.Repeat("f", 100)}}
	for i := 0; i < nextStepsMaxActions; i++ {
		long.Actions = append(long.Actions, NextStepsAction{Action: strings.Repeat("word ", 17), Owner: "COO", Date: "15 Oct"})
	}
	if w := p.PostExpandWarnings(shortAreaCtx, long, nil); len(w) != 1 || !strings.Contains(w[0], "tightest rows") {
		t.Errorf("over-full list: warnings = %v", w)
	}
}

func TestExecSummaryFivePointsFitAShortArea(t *testing.T) {
	vals := &ExecSummaryValues{BottomLine: "Fund an SMB retention pod in Q3 and hold the enterprise motion as is."}
	for i := 0; i < execSummaryMaxPoints; i++ {
		vals.Points = append(vals.Points, ExecSummaryPoint{Lead: fmt.Sprintf("Growth is ahead of plan. %d", i+1)})
	}
	p := &execSummary{}
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 10831550, Height: 3949700}}
	if w := p.PostExpandWarnings(ctx, vals, nil); len(w) != 0 {
		t.Fatalf("five one-line points and a bottom line are refused: %v", w)
	}
	_, lay := layoutExecSummary(ctx, vals, &ExecSummaryOverrides{})
	_, areaH := sizingAreaPt(ctx)
	if lay.padPt <= 0 || lay.natural() > areaH || lay.leadSize < scaleBodyPt {
		t.Fatalf("pad=%v natural=%.0f area=%.0f lead=%v, want tightened rows that fit at 12pt or larger", lay.padPt, lay.natural(), areaH, lay.leadSize)
	}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The number is the row's tallest text: no nudge, the bare padding.
	if top, bottom, ok := rowPadInsetOf(t, grid.Rows[0].Cells[0].Shape.Text); !ok || top != lay.padPt || bottom != lay.padPt {
		t.Errorf("number cell insets = %v/%v (set=%v), want %v", top, bottom, ok, lay.padPt)
	}
	// The lead keeps its baseline nudge on top of the padding.
	if top, _, ok := rowPadInsetOf(t, grid.Rows[0].Cells[1].Shape.Text); !ok || top <= lay.padPt {
		t.Errorf("lead cell inset_top = %v (set=%v), want the padding plus the baseline nudge", top, ok)
	}
}

func TestTableHighlightSixOptionsFitAShortArea(t *testing.T) {
	v := &TableHighlightValues{HighlightLabel: "Recommended"}
	for _, c := range []string{"Capex", "Payback", "Execution risk", "Customer impact"} {
		v.Criteria = append(v.Criteria, TableHighlightCriterion{Label: c})
	}
	for i := 0; i < thMaxOptions; i++ {
		v.Options = append(v.Options, TableHighlightOption{Name: fmt.Sprintf("Hub consolidation %d", i+1), Scores: []TableHighlightScore{"1", "2", "3", "4"}})
	}
	row := 1
	v.HighlightRow = &row
	if w := (&tableHighlight{}).PostExpandWarnings(shortAreaCtx, v, nil); len(w) != 0 {
		t.Fatalf("six name-only options are refused: %v", w)
	}
	l := newTHLayout(shortAreaCtx, v, &TableHighlightOverrides{})
	l.fit()
	if l.padPt <= 0 || l.total() > l.areaH || l.bodySize < scaleBodyPt {
		t.Fatalf("pad=%v total=%.0f area=%.0f body=%v, want tightened rows that fit at 12pt", l.padPt, l.total(), l.areaH, l.bodySize)
	}
	// A symbol never touches the rules above and below it.
	for i, h := range l.rowPt {
		if h < thSymbolPt+4 {
			t.Errorf("row %d is %.0fpt, too short for a %vpt symbol", i, h, thSymbolPt)
		}
	}
}

func TestHouseKeepsItsRoofOnAShortArea(t *testing.T) {
	v := &StrategyHouseValues{
		Objective: "Become the trusted settlement platform",
		Pillars: []StrategyHousePillar{
			{Title: "Customer trust", Body: []string{"Transparent pricing", "Operational resilience"}},
			{Title: "Product velocity", Body: []string{"Weekly releases", "Shared platform"}},
			{Title: "Disciplined growth", Body: []string{"Enterprise focus"}},
		},
		FoundationLayers: []StrategyHouseLayer{{"One operating model across every market"}, {"People", "Data", "Controls"}},
	}
	sh := &strategyHouse{}
	if w := sh.heightWarning(shortAreaCtx, v, nil); w != "" {
		t.Fatalf("three pillars over two foundation levels are refused: %s", w)
	}
	fullW, areaH := contentAreaPt(shortAreaCtx)
	layout, err := BuildHouse(v.model(), v.style(shortAreaCtx, &StrategyHouseOverrides{}, nil), fullW, areaH)
	if err != nil {
		t.Fatal(err)
	}
	if layout.RoofFlattened || layout.Tight || layout.RoofRisePt < fullW*houseRoofMinPitch-1 {
		t.Fatalf("roof rise %.0fpt over %.0fpt (flattened=%v tight=%v): the gable must keep its minimum pitch", layout.RoofRisePt, fullW, layout.RoofFlattened, layout.Tight)
	}
	if layout.HeightPt > areaH {
		t.Errorf("house is %.0fpt in a %.0fpt area", layout.HeightPt, areaH)
	}
	// The bands gave up margin; the pillars kept the uniform one.
	rows := layout.Grid.Rows
	if _, _, ok := rowPadInsetOf(t, rows[len(rows)-1].Cells[0].Shape.Text); !ok {
		t.Error("foundation band kept the uniform margin on a short area")
	}
	if _, _, ok := rowPadInsetOf(t, rows[1].Cells[0].Shape.Text); ok {
		t.Error("pillar text margin was tightened")
	}

	// With height to spare nothing is tightened.
	roomy, err := BuildHouse(v.model(), v.style(ExpandContext{}, &StrategyHouseOverrides{}, nil), fullW, 400)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := rowPadInsetOf(t, roomy.Grid.Rows[len(roomy.Grid.Rows)-1].Cells[0].Shape.Text); ok {
		t.Error("a house with height to spare tightened its bands")
	}
}
