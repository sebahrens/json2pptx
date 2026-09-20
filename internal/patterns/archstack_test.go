package patterns

import (
	"encoding/json"
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

// TestArchStack_RailsAreThinRotatedBands pins the fix for go-slide-creator-pr3g:
// two rails at 12% each took a quarter of the slide's width to say "Security"
// and "Monitoring", leaving two tall empty columns beside the stack.
func TestArchStack_RailsAreThinRotatedBands(t *testing.T) {
	p, _ := Default().Get("arch-stack")
	ctx := testThemeCtx()
	vals := &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Presentation", Description: "React, Next.js"},
			{Label: "Services", Description: "Go services"},
			{Label: "Data", Description: "PostgreSQL"},
		},
		SideRails: []string{"Security", "Monitoring"},
	}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatalf("columns: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("got %d columns, want tier + 2 rails", len(cols))
	}
	for i, w := range cols[1:] {
		if w != archStackRailWidthPct {
			t.Errorf("rail %d width = %.1f%%, want %.1f%%", i, w, archStackRailWidthPct)
		}
	}
	if want := 100 - 2*archStackRailWidthPct; cols[0] != want {
		t.Errorf("tier column = %.1f%%, want %.1f%%", cols[0], want)
	}

	// The rail label is rotated, so the band only needs one line of width.
	var rail struct {
		Vert       string `json:"vert"`
		Paragraphs []struct {
			Content string `json:"content"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Text, &rail); err != nil {
		t.Fatalf("rail text: %v", err)
	}
	if rail.Vert != "vert270" {
		t.Errorf("rail vert = %q, want vert270", rail.Vert)
	}
	if len(rail.Paragraphs) != 1 || rail.Paragraphs[0].Content != "Security" {
		t.Errorf("rail label = %+v, want the authored label", rail.Paragraphs)
	}
}
