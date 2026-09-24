package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
)

func TestCardGridBudgetAndWarningStayConsistent(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  patterns.ExpandContext
	}{
		{"standard", patterns.ExpandContext{
			SlideWidth: 9144000, SlideHeight: 5143500,
			LayoutBounds: patterns.LayoutBounds{X: 457200, Y: 457200, Width: 8229600, Height: 4229100},
		}},
		{"wide", patterns.ExpandContext{
			SlideWidth: 12192000, SlideHeight: 6858000,
			LayoutBounds: patterns.LayoutBounds{X: 457200, Y: 1371600, Width: 11277600, Height: 5029200},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := patterns.CardGridValues{Columns: 4, Rows: 3, Cells: make([]patterns.CardGridCell, 12)}
			for i := range values.Cells {
				values.Cells[i] = patterns.CardGridCell{Header: "Header", Body: "Brief body"}
			}
			makeInput := func() *PatternInput {
				t.Helper()
				raw, err := json.Marshal(values)
				if err != nil {
					t.Fatal(err)
				}
				return &PatternInput{Name: "card-grid", Values: raw}
			}
			pi := makeInput()
			grid, warnings, err := expandPattern(pi, tc.ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			if len(warnings) != 0 {
				t.Fatalf("short copy warned: %v", warnings)
			}
			short, _ := computePatternCellBudgets(grid, tc.ctx, pi)
			if len(short) < 12 || short[0].MaxChars <= 10 || short[0].MaxChars >= 300 {
				t.Fatalf("unexpected measured card budget: %+v", short)
			}
			budget := short[0].MaxChars
			for i := range values.Cells {
				values.Cells[i].Body = strings.Repeat("W", budget+1)
			}
			pi = makeInput()
			grid, warnings, err = expandPattern(pi, tc.ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			long, densityWarnings := computePatternCellBudgets(grid, tc.ctx, pi)
			if len(warnings) != 12 || len(densityWarnings) < 12 {
				t.Fatalf("warnings = %d/%d, want 12 each: %v / %+v", len(warnings), len(densityWarnings), warnings, densityWarnings)
			}
			for i := 0; i < 12; i++ {
				b := long[i]
				if b.MaxChars != budget || b.ActualChars != budget+1 || b.Status != string(textcapacity.StatusOverflow) {
					t.Errorf("cell %d budget changed or status disagrees: %+v (original budget %d)", i, b, budget)
				}
				if !strings.Contains(warnings[i], fmt.Sprintf("cells[%d].body is %d characters; this card holds about %d", i, budget+1, budget)) {
					t.Errorf("warning %d disagrees with cell budget: %q", i, warnings[i])
				}
			}
		})
	}
}

func TestCardGridBudgetSurvivesUnresolvableExpandedGrid(t *testing.T) {
	values := patterns.CardGridValues{Columns: 2, Rows: 1, Cells: []patterns.CardGridCell{
		{Header: "A", Body: "First"}, {Header: "B", Body: "Second"},
	}}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	pi := &PatternInput{Name: "card-grid", Values: raw}
	ctx := patterns.ExpandContext{LayoutBounds: patterns.LayoutBounds{Width: 8229600, Height: 4229100}}
	budgets, _ := computePatternCellBudgets(nil, ctx, pi)
	if len(budgets) != 2 || budgets[0].MaxChars == 0 || budgets[1].ActualChars != len("Second") {
		t.Fatalf("pre-authoring budgets disappeared with an unresolved grid: %+v", budgets)
	}
	tiny := patterns.ExpandContext{LayoutBounds: patterns.LayoutBounds{Width: 10000, Height: 10000}}
	noRoom, _ := computePatternCellBudgets(nil, tiny, pi)
	if noRoom[0].MaxChars != 0 || noRoom[0].Status != string(textcapacity.StatusOverflow) || noRoom[0].DensityPct != 1000 {
		t.Errorf("zero-capacity card should read as overflow, not sparse: %+v", noRoom[0])
	}
}

func TestCardGridBudgetRespectsBoundsAndCallout(t *testing.T) {
	values := patterns.CardGridValues{Columns: 4, Rows: 3, Cells: make([]patterns.CardGridCell, 12)}
	for i := range values.Cells {
		values.Cells[i] = patterns.CardGridCell{Header: "Header", Body: "Brief"}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	ctx := patterns.ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: patterns.LayoutBounds{X: 457200, Y: 1371600, Width: 11277600, Height: 5029200},
	}
	budgetFor := func(pi *PatternInput) int {
		t.Helper()
		grid, _, err := expandPattern(pi, ctx, patterns.Default())
		if err != nil {
			t.Fatal(err)
		}
		budgets, _ := computePatternCellBudgets(grid, ctx, pi)
		if len(budgets) < 12 {
			t.Fatalf("budget count = %d, want 12", len(budgets))
		}
		return budgets[0].MaxChars
	}
	base := budgetFor(&PatternInput{Name: "card-grid", Values: raw})
	bounded := budgetFor(&PatternInput{Name: "card-grid", Values: raw, MaxHeightPct: 50})
	withCallout := budgetFor(&PatternInput{Name: "card-grid", Values: raw, Callout: &patterns.PatternCallout{Text: "Takeaway\nSecond line\nThird line"}})
	if bounded >= base || withCallout >= base {
		t.Errorf("bounds/callout should reduce the body budget: base=%d bounded=%d callout=%d", base, bounded, withCallout)
	}
	for i := range values.Cells {
		values.Cells[i].Body = strings.Repeat("X", withCallout+1)
	}
	longRaw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	pi := &PatternInput{Name: "card-grid", Values: longRaw, Callout: &patterns.PatternCallout{Text: "Takeaway\nSecond line\nThird line"}}
	grid, warnings, err := expandPattern(pi, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	budgets, _ := computePatternCellBudgets(grid, ctx, pi)
	if len(warnings) != 12 || budgets[0].MaxChars != withCallout || !strings.Contains(warnings[0], fmt.Sprintf("holds about %d", withCallout)) {
		t.Errorf("callout warning and reported budget disagree: warnings=%v budget=%+v", warnings, budgets[0])
	}
}
