package patterns

import (
	"testing"
)

func TestPyramid_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("pyramid")
	if !ok {
		t.Fatal("pyramid pattern not registered")
	}

	vals := &PyramidValues{
		Tiers: []string{"Strategy", "Tactics", "Execution"},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(grid.Rows))
	}
}

func TestPyramid_ValidateTooFew(t *testing.T) {
	p, ok := Default().Get("pyramid")
	if !ok {
		t.Fatal("pyramid pattern not registered")
	}

	vals := &PyramidValues{Tiers: []string{"A", "B"}}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for < 3 tiers")
	}
}

func TestPyramid_ValidateTooMany(t *testing.T) {
	p, ok := Default().Get("pyramid")
	if !ok {
		t.Fatal("pyramid pattern not registered")
	}

	vals := &PyramidValues{Tiers: []string{"A", "B", "C", "D", "E", "F"}}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for > 5 tiers")
	}
}
