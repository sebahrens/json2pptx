package patterns

import (
	"testing"
)

func TestAgenda_Validate_Basic(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"Introduction", "Analysis", "Strategy"},
	}
	if err := a.Validate(v, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgenda_Validate_TooFew(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"Only One"},
	}
	if err := a.Validate(v, nil, nil); err == nil {
		t.Error("expected error for fewer than 2 items")
	}
}

func TestAgenda_Validate_TooMany(t *testing.T) {
	a := &agenda{}

	items := make([]string, 11)
	for i := range items {
		items[i] = "Section"
	}
	v := &AgendaValues{Items: items}
	if err := a.Validate(v, nil, nil); err == nil {
		t.Error("expected error for more than 10 items")
	}
}

func TestAgenda_Validate_EmptyItem(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"OK", ""},
	}
	if err := a.Validate(v, nil, nil); err == nil {
		t.Error("expected error for empty item")
	}
}

func TestAgenda_Validate_HighlightOutOfRange(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{Items: []string{"A", "B"}}
	ovr := &AgendaOverrides{Highlight: 5}
	if err := a.Validate(v, ovr, nil); err == nil {
		t.Error("expected error for highlight > item count")
	}
}

func TestAgenda_Expand_Basic(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"Introduction", "Analysis", "Strategy"},
	}
	ctx := ExpandContext{}

	grid, err := a.Expand(ctx, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
	}

	// Each row should have 2 cells: number badge + title
	for i, row := range grid.Rows {
		if len(row.Cells) != 2 {
			t.Errorf("row[%d]: expected 2 cells, got %d", i, len(row.Cells))
		}
	}
}

func TestAgenda_Expand_WithHighlight(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"A", "B", "C"},
	}
	ovr := &AgendaOverrides{Highlight: 2}
	ctx := ExpandContext{}

	grid, err := a.Expand(ctx, v, ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
	}
}

func TestAgenda_Registry(t *testing.T) {
	_, ok := Default().Get("agenda")
	if !ok {
		t.Error("agenda pattern not found in default registry")
	}
}
