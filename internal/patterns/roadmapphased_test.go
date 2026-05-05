package patterns

import (
	"testing"
)

func TestRoadmapPhased_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("roadmap-phased")
	if !ok {
		t.Fatal("roadmap-phased pattern not registered")
	}

	vals := &RoadmapPhasedValues{
		Phases: []string{"Q1", "Q2", "Q3"},
		Workstreams: []RoadmapWorkstream{
			{Name: "Platform", Items: []string{"Auth", "API", "Scale"}},
			{Name: "Frontend", Items: []string{"Design", "Build", "Polish"}},
		},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// 1 header row + 2 workstream rows = 3
	if len(grid.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(grid.Rows))
	}
	// 1 label col + 3 phases = 4
	if len(grid.Rows[0].Cells) != 4 {
		t.Errorf("expected 4 header cells, got %d", len(grid.Rows[0].Cells))
	}
}

func TestRoadmapPhased_ValidateItemsMismatch(t *testing.T) {
	p, ok := Default().Get("roadmap-phased")
	if !ok {
		t.Fatal("roadmap-phased pattern not registered")
	}

	vals := &RoadmapPhasedValues{
		Phases: []string{"Q1", "Q2", "Q3"},
		Workstreams: []RoadmapWorkstream{
			{Name: "Platform", Items: []string{"Auth", "API"}}, // only 2, need 3
			{Name: "Frontend", Items: []string{"Design", "Build", "Polish"}},
		},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for items count mismatch")
	}
}
