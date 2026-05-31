package patterns

import (
	"encoding/json"
	"testing"
)

// firstParagraphSize extracts paragraphs[0].size from a process-flow cell's text.
func firstParagraphSize(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	var obj struct {
		Paragraphs []struct {
			Size float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal text: %v", err)
	}
	if len(obj.Paragraphs) == 0 {
		t.Fatal("no paragraphs in cell text")
	}
	return obj.Paragraphs[0].Size
}

func processFlowStepFont(t *testing.T, n int, ovr any) float64 {
	t.Helper()
	p, ok := Default().Get("process-flow")
	if !ok {
		t.Fatal("process-flow pattern not registered")
	}
	steps := make([]ProcessFlowStep, n)
	for i := range steps {
		steps[i] = ProcessFlowStep{Label: "Step", Type: "step"}
	}
	grid, err := p.Expand(ExpandContext{}, &ProcessFlowValues{Steps: steps}, ovr, nil)
	if err != nil {
		t.Fatalf("Expand n=%d: %v", n, err)
	}
	return firstParagraphSize(t, grid.Rows[0].Cells[0].Shape.Text)
}

func TestProcessFlow_DefaultFontScalesDownWithCount(t *testing.T) {
	// n<=4 keeps the historical 12pt so size-metric goldens stay stable.
	for _, n := range []int{3, 4} {
		if got := processFlowStepFont(t, n, nil); got != 12 {
			t.Errorf("n=%d default font = %g, want 12", n, got)
		}
	}
	// Larger counts shrink so short labels fit narrower boxes without wrapping.
	if processFlowStepFont(t, 8, nil) >= processFlowStepFont(t, 4, nil) {
		t.Errorf("8-step font (%g) should be smaller than 4-step font (%g)",
			processFlowStepFont(t, 8, nil), processFlowStepFont(t, 4, nil))
	}
	if got := processFlowStepFont(t, 5, nil); got != 11 {
		t.Errorf("n=5 default font = %g, want 11", got)
	}
}

func TestProcessFlow_BodySizeOverrideWinsOverScaling(t *testing.T) {
	ovr := &ProcessFlowOverrides{BodySize: 18}
	if got := processFlowStepFont(t, 8, ovr); got != 18 {
		t.Errorf("with body_size=18 override, 8-step font = %g, want 18", got)
	}
}

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
