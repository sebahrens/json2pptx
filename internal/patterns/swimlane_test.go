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
	for ci := 1; ci < len(grid.Rows[0].Cells); ci++ {
		if got := string(grid.Rows[0].Cells[ci].Shape.Line); got != paperSurfaceHairline {
			t.Errorf("page-colored lane cell %d line = %s, want hairline", ci, got)
		}
		if got := string(grid.Rows[1].Cells[ci].Shape.Line); got != "" {
			t.Errorf("tinted lane cell %d unexpectedly has hairline %s", ci, got)
		}
	}
}

func TestSwimlane_EmptyPaperStepHasOutline(t *testing.T) {
	p, _ := Default().Get("swimlane")
	vals := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "A", Steps: []string{""}},
		{Actor: "B", Steps: []string{"Done"}},
	}}
	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(grid.Rows[0].Cells[1].Shape.Line); got != paperSurfaceHairline {
		t.Errorf("empty paper step line = %s, want hairline", got)
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
