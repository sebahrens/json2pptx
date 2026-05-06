package patterns

import (
	"testing"
)

func TestProcessFlowCompact_Registered(t *testing.T) {
	_, ok := Default().Get("process-flow-compact")
	if !ok {
		t.Fatal("process-flow-compact pattern not registered")
	}
}

func TestProcessFlowCompact_ExpandBasic(t *testing.T) {
	p, _ := Default().Get("process-flow-compact")

	vals := &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "Start", Type: "step"},
			{Label: "Check", Type: "decision"},
			{Label: "End", Type: "step"},
		},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// Must have bounds set to ~35% height
	if grid.Bounds == nil {
		t.Fatal("expected bounds to be set for compact variant")
	}
	if grid.Bounds.Height != 35 {
		t.Errorf("expected bounds height 35, got %v", grid.Bounds.Height)
	}
	if grid.Bounds.Width != 100 {
		t.Errorf("expected bounds width 100, got %v", grid.Bounds.Width)
	}

	if len(grid.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
	}
	if grid.Rows[0].Connector == nil {
		t.Error("expected connector on the row")
	}
}

func TestProcessFlowCompact_ValidateMinSteps(t *testing.T) {
	p, _ := Default().Get("process-flow-compact")

	vals := &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "A"},
			{Label: "B"},
		},
	}
	if err := p.Validate(vals, nil, nil); err == nil {
		t.Error("expected validation error for < 3 steps")
	}
}

func TestProcessFlowCompact_TaxonomyDensityLow(t *testing.T) {
	p, _ := Default().Get("process-flow-compact")
	tax := p.Taxonomy()
	if tax.DensityClass != "low" {
		t.Errorf("expected DensityClass 'low', got %q", tax.DensityClass)
	}
}
