package patterns

import (
	"strings"
	"testing"
)

func TestCardGridBodyBudgetIsStableBeforeAuthoring(t *testing.T) {
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 11277600, Height: 5029200}}
	values := &CardGridValues{Columns: 4, Rows: 3, Cells: make([]CardGridCell, 12)}
	for i := range values.Cells {
		values.Cells[i] = CardGridCell{Header: "Header", Body: "Short"}
	}
	short := CardGridBodyBudgets(ctx, values, nil)
	for i := range values.Cells {
		values.Cells[i].Body = strings.Repeat("W", 300)
	}
	long := CardGridBodyBudgets(ctx, values, nil)
	if len(short) != 12 || len(long) != 12 {
		t.Fatalf("budget count = %d/%d, want 12", len(short), len(long))
	}
	for i := range short {
		if short[i] <= 0 || short[i] > 300 || short[i] != long[i] {
			t.Errorf("cell %d budget changed with body content: %d -> %d", i, short[i], long[i])
		}
	}
	wide := CardGridBodyBudgets(ctx, &CardGridValues{Columns: 2, Rows: 2, Cells: values.Cells[:4]}, nil)
	if wide[0] <= short[0] {
		t.Errorf("2x2 budget %d should exceed 4x3 budget %d", wide[0], short[0])
	}
	largeFont := CardGridBodyBudgets(ctx, values, &CardGridOverrides{TextOverrides: TextOverrides{BodySize: 18}})
	if largeFont[0] >= short[0] {
		t.Errorf("18pt budget %d should be below 12pt budget %d", largeFont[0], short[0])
	}
}

func TestCardGridBodyBudgetReservesHeaderAndMedia(t *testing.T) {
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 11277600, Height: 5029200}}
	values := &CardGridValues{Columns: 4, Rows: 3, Cells: make([]CardGridCell, 12)}
	for i := range values.Cells {
		values.Cells[i] = CardGridCell{Header: "Header", Body: "Body"}
	}
	base := CardGridBodyBudgets(ctx, values, nil)[0]
	values.Cells[0].Header = strings.Repeat("Long header ", 5)
	withHeader := CardGridBodyBudgets(ctx, values, nil)
	if withHeader[0] >= base || withHeader[1] >= base {
		t.Errorf("wrapped header should reduce capacity for all cards in its row: base %d, got %v", base, withHeader[:4])
	}
	values.Cells[0].Header = "Header"
	values.Cells[0].Icon = &IconRef{Name: "rocket", Position: "top"}
	values.Cells[1].Secondary = &SecondaryChart{}
	withMedia := CardGridBodyBudgets(ctx, values, nil)
	if withMedia[0] >= base || withMedia[1] >= base || withMedia[2] != base {
		t.Errorf("icon/chart should reserve their own area only: base %d, got %v", base, withMedia[:4])
	}
	values.Cells[0].Icon.Position = "left"
	withLeftIcon := CardGridBodyBudgets(ctx, values, nil)
	if withLeftIcon[0] >= base || withLeftIcon[0] == withMedia[0] {
		t.Errorf("left icon should reduce width rather than top-icon height: base=%d top=%d left=%d", base, withMedia[0], withLeftIcon[0])
	}
	values.Cells[0].Icon = nil
	noIcon := CardGridBodyBudgets(ctx, values, &CardGridOverrides{Style: "icon-card"})
	if noIcon[0] != base {
		t.Errorf("icon-card style without an icon should not lose icon space: got %d, want %d", noIcon[0], base)
	}
	values.Cells[1].Secondary = nil
	values.Cells[0].Header = "1. Launch"
	withPrefix := CardGridBodyBudgets(ctx, values, &CardGridOverrides{Style: "numbered-badge"})[0]
	values.Cells[0].Header = "Launch"
	withoutPrefix := CardGridBodyBudgets(ctx, values, &CardGridOverrides{Style: "numbered-badge"})[0]
	if withPrefix != withoutPrefix {
		t.Errorf("numbered-badge prefix should be measured as the separate badge, got %d vs %d", withPrefix, withoutPrefix)
	}
}

// The warning names the shape and the number the author has to hit, which is
// what "shorten the text" on its own does not say.
func TestCardGridWarnsWithTheBudgetForItsShape(t *testing.T) {
	pat, ok := Default().Get("card-grid")
	if !ok {
		t.Fatal("card-grid not registered")
	}
	warner, ok := pat.(PostExpandWarner)
	if !ok {
		t.Fatal("card-grid does not emit post-expand warnings")
	}

	dense := &CardGridValues{Columns: 4, Rows: 3}
	for i := 0; i < 12; i++ {
		dense.Cells = append(dense.Cells, CardGridCell{Header: "Header"})
	}
	budget := CardGridBodyBudgets(ExpandContext{}, dense, nil)[0]
	if budget <= 0 || budget >= 300 {
		t.Fatalf("dense budget = %d, want within (0,300)", budget)
	}
	for i := range dense.Cells {
		dense.Cells[i].Body = strings.Repeat("W", budget+1)
	}
	warnings := warner.PostExpandWarnings(ExpandContext{}, dense, nil)
	if len(warnings) != 12 {
		t.Fatalf("got %d warnings for 12 over-budget cards, want 12: %v", len(warnings), warnings)
	}
	for _, want := range []string{ErrCodeBodyTooLong, "cells[0].body is", "this card holds about"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning %q is missing %q", warnings[0], want)
		}
	}

	// The same body in a small grid is exactly what the schema promises, and
	// says nothing.
	small := &CardGridValues{Columns: 2, Rows: 1, Cells: []CardGridCell{
		{Header: "Header", Body: strings.Repeat("W", 300)}, {Header: "Header", Body: strings.Repeat("W", 300)},
	}}
	if got := warner.PostExpandWarnings(ExpandContext{}, small, nil); len(got) != 0 {
		t.Errorf("a 2x1 grid at the schema maximum warned: %v", got)
	}
	for i := range dense.Cells {
		dense.Cells[i].Body = strings.Repeat("W", budget)
	}
	if got := warner.PostExpandWarnings(ExpandContext{}, dense, nil); len(got) != 0 {
		t.Errorf("4x3 cards at their measured budget warned: %v", got)
	}
}
