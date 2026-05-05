package patterns

import (
	"testing"
)

func TestArchStack_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("arch-stack")
	if !ok {
		t.Fatal("arch-stack pattern not registered")
	}

	vals := &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Presentation", Description: "React"},
			{Label: "Logic", Description: "Go"},
			{Label: "Data", Description: "PostgreSQL"},
		},
		SideRails: []string{"Security"},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(grid.Rows))
	}
	// 1 tier col + 1 side rail = 2 columns per row
	if len(grid.Rows[0].Cells) != 2 {
		t.Errorf("expected 2 cells in first row, got %d", len(grid.Rows[0].Cells))
	}
}

func TestArchStack_ExpandNoRails(t *testing.T) {
	p, ok := Default().Get("arch-stack")
	if !ok {
		t.Fatal("arch-stack pattern not registered")
	}

	vals := &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Frontend"},
			{Label: "Backend"},
			{Label: "Database"},
		},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(grid.Rows))
	}
	// No side rails → 1 column
	if len(grid.Rows[0].Cells) != 1 {
		t.Errorf("expected 1 cell per row (no side rails), got %d", len(grid.Rows[0].Cells))
	}
}

func TestArchStack_ValidateTooFewTiers(t *testing.T) {
	p, ok := Default().Get("arch-stack")
	if !ok {
		t.Fatal("arch-stack pattern not registered")
	}

	vals := &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "A"},
			{Label: "B"},
		},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for < 3 tiers")
	}
}
