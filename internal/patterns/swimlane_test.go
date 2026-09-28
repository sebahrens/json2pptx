package patterns

import (
	"testing"
)

func TestSwimlane_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("swimlane")
	if !ok {
		t.Fatal("swimlane pattern not registered")
	}

	vals := &SwimlaneValues{
		Lanes: []SwimlaneLane{
			{Actor: "Customer", Steps: []string{"Request", "Wait", "Receive"}},
			{Actor: "Support", Steps: []string{"Triage", "Fix", "Notify"}},
		},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(grid.Rows))
	}
	// 1 actor + 3 steps = 4 columns
	if len(grid.Rows[0].Cells) != 4 {
		t.Errorf("expected 4 cells per row, got %d", len(grid.Rows[0].Cells))
	}
	// Lanes alternate two neutral steps and no cell is outlined
	// (go-slide-creator-pgdkp).
	for ci := 1; ci < len(grid.Rows[0].Cells); ci++ {
		for ri, want := range []string{neutral4JSON, neutral8JSON} {
			cell := grid.Rows[ri].Cells[ci]
			if got := string(cell.Shape.Fill); got != want {
				t.Errorf("lane %d cell %d fill = %s, want %s", ri, ci, got, want)
			}
			if got := string(cell.Shape.Line); got != `"none"` {
				t.Errorf("lane %d cell %d line = %s, want none", ri, ci, got)
			}
		}
	}
}

func TestSwimlane_EmptyStepIsTintedNotOutlined(t *testing.T) {
	p, _ := Default().Get("swimlane")
	vals := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "A", Steps: []string{""}},
		{Actor: "B", Steps: []string{"Done"}},
	}}
	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(grid.Rows[0].Cells[1].Shape.Fill); got != neutral4JSON {
		t.Errorf("empty step fill = %s, want neutral 4%%", got)
	}
	if got := string(grid.Rows[0].Cells[1].Shape.Line); got != `"none"` {
		t.Errorf("empty step line = %s, want none", got)
	}
}

func TestSwimlane_ValidateMismatchedSteps(t *testing.T) {
	p, ok := Default().Get("swimlane")
	if !ok {
		t.Fatal("swimlane pattern not registered")
	}

	vals := &SwimlaneValues{
		Lanes: []SwimlaneLane{
			{Actor: "A", Steps: []string{"S1", "S2", "S3"}},
			{Actor: "B", Steps: []string{"S1", "S2"}}, // mismatch
		},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for mismatched step counts")
	}
}
