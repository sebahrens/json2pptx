package patterns

import (
	"testing"
)

func TestKPIInline_Registered(t *testing.T) {
	_, ok := Default().Get("kpi-inline")
	if !ok {
		t.Fatal("kpi-inline pattern not registered")
	}
}

func TestKPIInline_ExpandBasic(t *testing.T) {
	p, _ := Default().Get("kpi-inline")

	vals := KPINupValues{
		{Big: "$4.2M", Small: "ARR"},
		{Big: "127%", Small: "NRR"},
		{Big: "12d", Small: "Cycle"},
	}

	grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// Must have bounds set to ~25% height
	if grid.Bounds == nil {
		t.Fatal("expected bounds to be set for inline variant")
	}
	if grid.Bounds.Height != 25 {
		t.Errorf("expected bounds height 25, got %v", grid.Bounds.Height)
	}

	if len(grid.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
	}
}

func TestKPIInline_ValidateMinCells(t *testing.T) {
	p, _ := Default().Get("kpi-inline")

	vals := KPINupValues{
		{Big: "$4.2M", Small: "ARR"},
	}
	if err := p.Validate(&vals, nil, nil); err == nil {
		t.Error("expected validation error for < 2 cells")
	}
}

func TestKPIInline_ValidateMaxCells(t *testing.T) {
	p, _ := Default().Get("kpi-inline")

	vals := KPINupValues{
		{Big: "1", Small: "A"},
		{Big: "2", Small: "B"},
		{Big: "3", Small: "C"},
		{Big: "4", Small: "D"},
		{Big: "5", Small: "E"},
		{Big: "6", Small: "F"},
		{Big: "7", Small: "G"},
	}
	if err := p.Validate(&vals, nil, nil); err == nil {
		t.Error("expected validation error for > 6 cells")
	}
}

func TestKPIInline_TaxonomyDensityLow(t *testing.T) {
	p, _ := Default().Get("kpi-inline")
	tax := p.Taxonomy()
	if tax.DensityClass != "low" {
		t.Errorf("expected DensityClass 'low', got %q", tax.DensityClass)
	}
}
