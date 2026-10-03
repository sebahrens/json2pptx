package patterns

import (
	"strings"
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
		t.Fatalf("expected 2 rows, got %d", len(grid.Rows))
	}
	// 1 actor + 3 steps = 4 columns
	for lane := 0; lane < 2; lane++ {
		row := grid.Rows[lane]
		if len(row.Cells) != 4 {
			t.Fatalf("lane %d: expected 4 cells, got %d", lane, len(row.Cells))
		}
		// Step tiles share one neutral tint and no cell is outlined
		// (go-slide-creator-pgdkp).
		for ci := 1; ci < len(row.Cells); ci++ {
			cell := row.Cells[ci]
			if got := string(cell.Shape.Fill); got != neutral8JSON {
				t.Errorf("lane %d cell %d fill = %s, want %s", lane, ci, got, neutral8JSON)
			}
			if got := string(cell.Shape.Line); got != `"none"` {
				t.Errorf("lane %d cell %d line = %s, want none", lane, ci, got)
			}
			if cell.MaxHeight < swimlaneTileMinPt {
				t.Errorf("lane %d cell %d: tile max_height %.0f, want a content-sized tile of at least %.0fpt", lane, ci, cell.MaxHeight, swimlaneTileMinPt)
			}
		}
	}
}

// Every lane is bounded by a full-width rule, and the step tiles are the
// only filled shapes: the actor label is unfilled text at the left of its
// band (go-slide-creator-jz5r9).
func TestSwimlaneLanesAreRuledBands(t *testing.T) {
	p, _ := Default().Get("swimlane")
	for lanes := 2; lanes <= 6; lanes++ {
		vals := &SwimlaneValues{}
		for i := 0; i < lanes; i++ {
			steps := make([]string, 4)
			steps[i%4] = "Do the work"
			vals.Lanes = append(vals.Lanes, SwimlaneLane{Actor: "Team", Steps: steps})
		}
		grid, err := p.Expand(testThemeCtx(), vals, nil, nil)
		if err != nil {
			t.Fatalf("lanes=%d: %v", lanes, err)
		}
		// Lane i stays grid row i (links and overlay anchors address it);
		// the rules sit in the gaps: above and below the first lane, below
		// every other.
		if got := len(grid.Rows); got != lanes {
			t.Fatalf("lanes=%d: %d rows, want one per lane", lanes, got)
		}
		for r, row := range grid.Rows {
			want := "below"
			if r == 0 {
				want = "both"
			}
			if row.Rule != want {
				t.Errorf("lanes=%d row %d: rule = %q, want %q", lanes, r, row.Rule, want)
			}
			actor := row.Cells[0].Shape
			if got := string(actor.Fill); got != `"none"` {
				t.Errorf("lanes=%d row %d: actor label fill = %s, want none", lanes, r, got)
			}
			for ci, c := range row.Cells[1:] {
				filled := string(c.Shape.Fill) != `"none"`
				if hasText := len(c.Shape.Text) > 0; filled != hasText {
					t.Errorf("lanes=%d row %d step %d: filled=%t but has text=%t", lanes, r, ci, filled, hasText)
				}
			}
		}
	}
}

// values.flow states the arrow order; without it a column shared by two
// lanes is reported (go-slide-creator-v786r).
func TestSwimlaneFlowOrder(t *testing.T) {
	p, _ := Default().Get("swimlane")
	warner := p.(PostExpandWarner)
	vals := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "Customer", Steps: []string{"Submit request", "", "Approve fix"}},
		{Actor: "Support", Steps: []string{"Triage", "Investigate", "Resolve"}},
		{Actor: "Engineering", Steps: []string{"", "Fix bug", "Deploy"}},
	}}

	// Derived order: down each column, then on to the next — and a finding.
	warnings := warner.PostExpandWarnings(ExpandContext{}, vals, nil)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], ErrCodeSwimlaneFlowAmbiguous+": ") || !strings.Contains(warnings[0], "values.flow") {
		t.Fatalf("shared columns without flow: warnings = %v, want one %s naming values.flow", warnings, ErrCodeSwimlaneFlowAmbiguous)
	}

	vals.Flow = [][2]int{{0, 0}, {1, 0}, {1, 1}, {2, 1}, {2, 2}, {1, 2}, {0, 2}}
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if w := warner.PostExpandWarnings(ExpandContext{}, vals, nil); len(w) != 0 {
		t.Errorf("a stated flow must not be reported: %v", w)
	}
	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Grid row = lane; grid column = step + 1.
	want := [][2][2]int{
		{{0, 1}, {1, 1}}, // Submit request -> Triage
		{{1, 1}, {1, 2}}, // -> Investigate
		{{1, 2}, {2, 2}}, // -> Fix bug
		{{2, 2}, {2, 3}}, // -> Deploy
		{{2, 3}, {1, 3}}, // -> Resolve
		{{1, 3}, {0, 3}}, // -> Approve fix
	}
	if len(grid.Links) != len(want) {
		t.Fatalf("links = %+v, want %d", grid.Links, len(want))
	}
	for i, l := range grid.Links {
		if l.From != want[i][0] || l.To != want[i][1] {
			t.Errorf("link %d = %v -> %v, want %v -> %v", i, l.From, l.To, want[i][0], want[i][1])
		}
	}

	// One step per column needs no flow and draws no finding.
	stair := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "Customer", Steps: []string{"Submit", "", ""}},
		{Actor: "Support", Steps: []string{"", "Triage", ""}},
		{Actor: "Engineering", Steps: []string{"", "", "Fix"}},
	}}
	if w := warner.PostExpandWarnings(ExpandContext{}, stair, nil); len(w) != 0 {
		t.Errorf("one step per column must not be reported: %v", w)
	}

	for name, flow := range map[string][][2]int{
		"lane out of range": {{0, 0}, {3, 0}},
		"step out of range": {{0, 0}, {1, 3}},
		"empty step":        {{0, 0}, {0, 1}},
		"self link":         {{0, 0}, {0, 0}},
		"single entry":      {{0, 0}},
	} {
		bad := &SwimlaneValues{Lanes: vals.Lanes, Flow: flow}
		if err := p.Validate(bad, nil, nil); err == nil {
			t.Errorf("%s: flow %v must be rejected", name, flow)
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
