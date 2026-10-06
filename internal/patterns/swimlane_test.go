package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// swimlaneTiles is overrides.style "tiles": the look before
// go-slide-creator-vx7wk, which the tests below pin as still reachable.
var swimlaneTiles = &SwimlaneOverrides{Style: swimlaneStyleTiles}

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

	grid, err := p.Expand(ExpandContext{}, vals, swimlaneTiles, nil)
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

// The default look (go-slide-creator-vx7wk): every lane is one row band headed
// by a pentagon actor tab; steps are pentagons in an accent tint, each as tall
// as the others; the one highlighted step is the only solid accent; and an
// arrow is drawn only where the pentagon cannot say where the flow goes next.
func TestSwimlaneBandsDefault(t *testing.T) {
	p, _ := Default().Get("swimlane")
	ctx := testThemeCtx()
	vals := &SwimlaneValues{
		Lanes: []SwimlaneLane{
			{Actor: "Customer", Steps: []string{"Report incident", "", "", "Confirm and close"}},
			{Actor: "Service desk", Steps: []string{"", "Log and classify", "Assign to resolver", ""}},
			{Actor: "Engineering", Steps: []string{"", "", "", ""}},
		},
		Highlight: []int{1, 2},
	}
	if err := p.Validate(vals, &SwimlaneOverrides{}, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for name, ovr := range map[string]any{"nil overrides": nil, "empty overrides": &SwimlaneOverrides{}, "style bands": &SwimlaneOverrides{Style: swimlaneStyleBands}} {
		grid, err := p.Expand(ctx, vals, ovr, nil)
		if err != nil {
			t.Fatalf("%s: Expand: %v", name, err)
		}
		if len(grid.Rows) != 3 {
			t.Fatalf("%s: %d rows, want one per lane", name, len(grid.Rows))
		}
		accent := ctx.ResolveAccent("", "")
		solid, tileH := 0, 0.0
		for r, row := range grid.Rows {
			if got := string(row.Band); got != string(neutralFillJSON(NeutralTint4)) {
				t.Errorf("%s row %d: band = %s, want the 4%% surface", name, r, got)
			}
			if row.Rule != "" {
				t.Errorf("%s row %d: rule = %q, want none (the band is the lane)", name, r, row.Rule)
			}
			if row.MaxHeight <= 0 {
				t.Errorf("%s row %d: lane has no max_height, want a content-sized band", name, r)
			}
			// actor tab + 4 steps + the zero-width edge column
			if len(row.Cells) != 6 {
				t.Fatalf("%s row %d: %d cells, want 6", name, r, len(row.Cells))
			}
			tab := row.Cells[0].Shape
			if tab.Geometry != "homePlate" || string(tab.Fill) != string(neutralTone(swimlaneTabTint).fillJSON()) || string(tab.Line) != `"none"` {
				t.Errorf("%s row %d: actor tab = %s fill %s line %s, want an unlined neutral pentagon", name, r, tab.Geometry, tab.Fill, tab.Line)
			}
			if !strings.Contains(string(tab.Text), `"bold":true`) {
				t.Errorf("%s row %d: actor label is not bold: %s", name, r, tab.Text)
			}
			for j, c := range row.Cells[1:5] {
				if vals.Lanes[r].Steps[j] == "" {
					if string(c.Shape.Fill) != `"none"` || len(c.Shape.Text) != 0 {
						t.Errorf("%s row %d step %d: an empty position must stay unpainted", name, r, j)
					}
					continue
				}
				if c.Shape.Geometry != "homePlate" || string(c.Shape.Line) != `"none"` || c.Shape.Adjustments["adj"] <= 0 {
					t.Errorf("%s row %d step %d: %s line %s adj %v, want an unlined pentagon with a stated point", name, r, j, c.Shape.Geometry, c.Shape.Line, c.Shape.Adjustments)
				}
				if tileH == 0 {
					tileH = c.MaxHeight
				}
				if c.MaxHeight != tileH || tileH < swimlaneTileMinPt {
					t.Errorf("%s row %d step %d: max_height %.1f, want every step %.1f", name, r, j, c.MaxHeight, tileH)
				}
				want := string(inactiveTintTone(accent).fillJSON())
				if vals.highlighted(r, j) {
					want = string(fillTone{Color: accent}.fillJSON())
					solid++
				}
				if got := string(c.Shape.Fill); got != want {
					t.Errorf("%s row %d step %d: fill = %s, want %s", name, r, j, got, want)
				}
			}
			if last := row.Cells[5]; last.Shape != nil {
				t.Errorf("%s row %d: the edge column must be an empty spacer", name, r)
			}
		}
		if solid != 1 {
			t.Errorf("%s: %d solid accent steps, want the highlighted one only", name, solid)
		}
		// Report -> Log changes lane (arrow); Log -> Assign is the next column
		// of the same lane (the pentagon points at it: no arrow); Assign ->
		// Confirm changes lane (arrow).
		want := [][2][2]int{{{0, 1}, {1, 2}}, {{1, 3}, {0, 4}}}
		if len(grid.Links) != len(want) {
			t.Fatalf("%s: links = %+v, want %d", name, grid.Links, len(want))
		}
		for i, l := range grid.Links {
			if l.From != want[i][0] || l.To != want[i][1] {
				t.Errorf("%s link %d = %v -> %v, want %v -> %v", name, i, l.From, l.To, want[i][0], want[i][1])
			}
			if c := l.Connector; c == nil || c.Style != "arrow" || c.Width != swimlaneConnectorPt || c.Color == accent {
				t.Errorf("%s link %d connector = %+v, want one neutral %.1fpt arrow", name, i, c, swimlaneConnectorPt)
			}
		}
	}
}

// A step that is followed by the same lane further on, or by an earlier
// column, still gets its arrow: only the adjacent next column is implied.
func TestSwimlaneBandsArrowWhereThePentagonCannotPoint(t *testing.T) {
	vals := &SwimlaneValues{
		Lanes: []SwimlaneLane{
			{Actor: "A", Steps: []string{"One", "Two", "", "Four"}},
			{Actor: "B", Steps: []string{"", "", "Three", ""}},
		},
		Flow: [][2]int{{0, 0}, {0, 1}, {0, 3}, {1, 2}, {0, 0}},
	}
	links := swimlaneBandLinks(vals, "dk1")
	want := [][2][2]int{
		{{0, 2}, {0, 4}}, // same lane, a column skipped
		{{0, 4}, {1, 3}}, // back one column, another lane
		{{1, 3}, {0, 1}}, // loop back to the start
	}
	if len(links) != len(want) {
		t.Fatalf("links = %+v, want %d", links, len(want))
	}
	for i, l := range links {
		if l.From != want[i][0] || l.To != want[i][1] {
			t.Errorf("link %d = %v -> %v, want %v -> %v", i, l.From, l.To, want[i][0], want[i][1])
		}
	}
}

// The actor tab is as wide as its labels need: narrower than the default
// share for short one-line labels, wider for a long word in a narrow area.
func TestSwimlaneBandsActorTabFollowsItsLabels(t *testing.T) {
	ctx := testThemeCtx()
	lanes := func(actors ...string) *SwimlaneValues {
		v := &SwimlaneValues{}
		for _, a := range actors {
			v.Lanes = append(v.Lanes, SwimlaneLane{Actor: a, Steps: []string{"Do", "", ""}})
		}
		return v
	}
	short := swimlaneBandSizing(ctx, lanes("IT", "HR"), scaleSubheadPt)
	if short.actorPct != swimlaneActorMinPct {
		t.Errorf("short labels: actor tab %.0f%%, want the %.0f%% minimum", short.actorPct, swimlaneActorMinPct)
	}
	narrow := ctx
	narrow.LayoutBounds.Width, narrow.LayoutBounds.Height = 300*12700, 300*12700
	long := swimlaneBandSizing(narrow, lanes("Procurement", "Finance"), scaleSubheadPt)
	if long.actorPct <= swimlaneActorPct || long.actorPct > swimlaneActorMaxPct {
		t.Errorf("a long word in a narrow area: actor tab %.0f%%, want between %.0f%% and %.0f%%", long.actorPct, swimlaneActorPct, swimlaneActorMaxPct)
	}
}

func TestSwimlaneHighlightAndStyleValidation(t *testing.T) {
	p, _ := Default().Get("swimlane")
	lanes := []SwimlaneLane{
		{Actor: "A", Steps: []string{"One", ""}},
		{Actor: "B", Steps: []string{"", "Two"}},
	}
	for name, hl := range map[string][]int{
		"one number":        {0},
		"three numbers":     {0, 0, 0},
		"lane out of range": {2, 0},
		"step out of range": {0, 2},
		"empty step":        {0, 1},
	} {
		if err := p.Validate(&SwimlaneValues{Lanes: lanes, Highlight: hl}, nil, nil); err == nil {
			t.Errorf("%s: highlight %v must be rejected", name, hl)
		}
	}
	if err := p.Validate(&SwimlaneValues{Lanes: lanes, Highlight: []int{1, 1}}, nil, nil); err != nil {
		t.Errorf("highlight [1,1]: %v", err)
	}
	if err := p.Validate(&SwimlaneValues{Lanes: lanes}, &SwimlaneOverrides{Style: "cards"}, nil); err == nil {
		t.Error(`overrides.style "cards" must be rejected`)
	}
	for _, style := range swimlaneStyles {
		if err := p.Validate(&SwimlaneValues{Lanes: lanes}, &SwimlaneOverrides{Style: style}, nil); err != nil {
			t.Errorf("overrides.style %q: %v", style, err)
		}
	}
	// The tiles style honours the highlight too: one solid accent tile.
	grid, err := p.Expand(testThemeCtx(), &SwimlaneValues{Lanes: lanes, Highlight: []int{1, 1}}, swimlaneTiles, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(grid.Rows[1].Cells[2].Shape.Fill), string(fillTone{Color: testThemeCtx().ResolveAccent("", "")}.fillJSON()); got != want {
		t.Errorf("tiles highlight fill = %s, want %s", got, want)
	}
}

// overrides.style "tiles": every lane is bounded by a full-width rule, and the
// step tiles are the only filled shapes: the actor label is unfilled text at
// the left of its band (go-slide-creator-jz5r9).
func TestSwimlaneLanesAreRuledBands(t *testing.T) {
	p, _ := Default().Get("swimlane")
	for lanes := 2; lanes <= 6; lanes++ {
		vals := &SwimlaneValues{}
		for i := 0; i < lanes; i++ {
			steps := make([]string, 4)
			steps[i%4] = "Do the work"
			vals.Lanes = append(vals.Lanes, SwimlaneLane{Actor: "Team", Steps: steps})
		}
		grid, err := p.Expand(testThemeCtx(), vals, swimlaneTiles, nil)
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
	grid, err := p.Expand(ExpandContext{}, vals, swimlaneTiles, nil)
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
	for _, ovr := range []any{nil, swimlaneTiles} {
		grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		checkSwimlaneEmptyStep(t, grid)
	}
}

func checkSwimlaneEmptyStep(t *testing.T, grid *jsonschema.ShapeGridInput) {
	t.Helper()
	// An empty position is a spacer, not a blank lane-tinted tile that
	// makes the swimlane read as a table (go-slide-creator-0b3f6).
	if got := string(grid.Rows[0].Cells[1].Shape.Fill); got != `"none"` {
		t.Errorf("empty step fill = %s, want none", got)
	}
	if got := string(grid.Rows[0].Cells[1].Shape.Line); got != `"none"` {
		t.Errorf("empty step line = %s, want none", got)
	}
}

// overrides.style "tiles": consecutive steps are joined in reading order —
// column by column, top lane first — with accent arrows, skipping empty
// positions, so lane hand-offs are visible (go-slide-creator-0b3f6).
func TestSwimlane_LinksFollowReadingOrder(t *testing.T) {
	p, _ := Default().Get("swimlane")
	vals := &SwimlaneValues{Lanes: []SwimlaneLane{
		{Actor: "Customer", Steps: []string{"Report incident", "", "Confirm workaround", "Close ticket"}},
		{Actor: "Service desk", Steps: []string{"Log and classify", "Assign to resolver", "", "Verify fix"}},
		{Actor: "Engineering", Steps: []string{"", "Diagnose root cause", "Deploy fix", ""}},
	}}
	ctx := ExpandContext{}
	grid, err := p.Expand(ctx, vals, swimlaneTiles, nil)
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
