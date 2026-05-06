package patterns

import (
	"testing"
)

func TestProcessFlow_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("process-flow")
	if !ok {
		t.Fatal("process-flow pattern not registered")
	}

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
	if len(grid.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
	}
	// Check connector
	if grid.Rows[0].Connector == nil {
		t.Error("expected connector on the row")
	}
}

func TestProcessFlow_ExpandChevronAndArrowTypes(t *testing.T) {
	p, ok := Default().Get("process-flow")
	if !ok {
		t.Fatal("process-flow pattern not registered")
	}

	vals := &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "Gather", Type: "chevron"},
			{Label: "Process", Type: "chevron"},
			{Label: "Deliver", Type: "arrow"},
		},
	}

	// Validate should pass
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
	}

	// Check geometry values
	wantGeom := []string{"chevron", "chevron", "rightArrow"}
	for i, cell := range grid.Rows[0].Cells {
		if cell.Shape == nil {
			t.Fatalf("cell %d: expected shape, got nil", i)
		}
		if cell.Shape.Geometry != wantGeom[i] {
			t.Errorf("cell %d: geometry = %q, want %q", i, cell.Shape.Geometry, wantGeom[i])
		}
	}
}

func TestProcessFlow_ValidateInvalidType(t *testing.T) {
	p, ok := Default().Get("process-flow")
	if !ok {
		t.Fatal("process-flow pattern not registered")
	}

	vals := &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "A", Type: "step"},
			{Label: "B", Type: "invalid"},
			{Label: "C", Type: "step"},
		},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for invalid type")
	}
}
