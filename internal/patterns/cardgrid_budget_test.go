package patterns

import (
	"strings"
	"testing"
)

// card-grid's schema caps a card body at 300 characters, which is the budget of
// a SMALL grid: the same 300 characters render at 2.6pt in a 5x5. A single
// maxLength cannot say both, so the shape-scaled budget says the second half
// (go-slide-creator-0g6p).
func TestCardGridBodyBudgetScalesWithTheGrid(t *testing.T) {
	// Measured: the largest body whose worst card still renders above the
	// readable floor, by shape.
	cases := []struct {
		columns, rows, want int
	}{
		{1, 1, 300}, {2, 1, 300}, {2, 2, 300},
		{3, 2, 220}, {4, 2, 160}, {3, 3, 100},
		{4, 3, 60}, {5, 3, 40}, {4, 4, 20}, {5, 5, 20},
	}
	for _, c := range cases {
		if got := cardGridBodyBudget(c.columns, c.rows); got != c.want {
			t.Errorf("cardGridBodyBudget(%d, %d) = %d, want %d", c.columns, c.rows, got, c.want)
		}
	}
	// The budget never grows as the grid does.
	prev := cardGridBodyBudget(1, 1)
	for cells := 2; cells <= 25; cells++ {
		got := cardGridBodyBudget(cells, 1)
		if got > prev {
			t.Errorf("budget at %d cells (%d) is larger than at %d (%d)", cells, got, cells-1, prev)
		}
		prev = got
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

	long := strings.Repeat("W", 61)
	dense := &CardGridValues{Columns: 4, Rows: 3}
	for i := 0; i < 12; i++ {
		dense.Cells = append(dense.Cells, CardGridCell{Header: "Header", Body: long})
	}
	warnings := warner.PostExpandWarnings(ExpandContext{}, dense, nil)
	if len(warnings) != 12 {
		t.Fatalf("got %d warnings for 12 over-budget cards, want 12: %v", len(warnings), warnings)
	}
	for _, want := range []string{ErrCodeBodyTooLong, "cells[0].body is 61 characters", "4x3 grid holds about 60"} {
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
		dense.Cells[i].Body = strings.Repeat("W", 60)
	}
	if got := warner.PostExpandWarnings(ExpandContext{}, dense, nil); len(got) != 0 {
		t.Errorf("4x3 cards at their 60-character budget warned: %v", got)
	}
}
