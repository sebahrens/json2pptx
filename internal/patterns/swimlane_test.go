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

func TestSwimlane_EmptyStepIsUnfilledNotOutlined(t *testing.T) {
	p, _ := Default().Get("swimlane")
	vals := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "A", Steps: []string{""}},
		{Actor: "B", Steps: []string{"Done"}},
	}}
	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// An empty position is a spacer, not a blank lane-tinted tile that
	// makes the swimlane read as a table (go-slide-creator-0b3f6).
	if got := string(grid.Rows[0].Cells[1].Shape.Fill); got != `"none"` {
		t.Errorf("empty step fill = %s, want none", got)
	}
	if got := string(grid.Rows[0].Cells[1].Shape.Line); got != `"none"` {
		t.Errorf("empty step line = %s, want none", got)
	}
}

// Consecutive steps are joined in reading order — column by column, top
// lane first — with accent arrows, skipping empty positions, so lane
// hand-offs are visible (go-slide-creator-0b3f6).
func TestSwimlane_LinksFollowReadingOrder(t *testing.T) {
	p, _ := Default().Get("swimlane")
	vals := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "Customer", Steps: []string{"Report incident", "", "Confirm workaround", "Close ticket"}},
		{Actor: "Service desk", Steps: []string{"Log and classify", "Assign to resolver", "", "Verify fix"}},
		{Actor: "Engineering", Steps: []string{"", "Diagnose root cause", "Deploy fix", ""}},
	}}
	ctx := ExpandContext{}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2][2]int{
		{{0, 1}, {1, 1}}, // Report incident -> Log and classify
		{{1, 1}, {1, 2}}, // -> Assign to resolver
		{{1, 2}, {2, 2}}, // -> Diagnose root cause
		{{2, 2}, {0, 3}}, // -> Confirm workaround (lane change across columns)
		{{0, 3}, {2, 3}}, // -> Deploy fix (skips the empty service-desk cell)
		{{2, 3}, {0, 4}}, // -> Close ticket
		{{0, 4}, {1, 4}}, // -> Verify fix
	}
	if len(grid.Links) != len(want) {
		t.Fatalf("links = %+v, want %d", grid.Links, len(want))
	}
	accent := ctx.ResolveAccent("", "")
	for i, l := range grid.Links {
		if l.From != want[i][0] || l.To != want[i][1] {
			t.Errorf("link %d = %v -> %v, want %v -> %v", i, l.From, l.To, want[i][0], want[i][1])
		}
		if l.Connector == nil || l.Connector.Style != "arrow" || l.Connector.Color != accent {
			t.Errorf("link %d connector = %+v, want accent arrow", i, l.Connector)
		}
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
