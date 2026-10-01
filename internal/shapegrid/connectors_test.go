package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func filled(geom string) *ShapeSpec {
	return &ShapeSpec{Geometry: geom, Fill: json.RawMessage(`"accent1"`)}
}

// Connectors only join visible, non-spacer shapes.
func TestResolveRowConnectors_SkipsSpacers(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 9144000, CY: 2000000},
		Columns: []float64{20, 20, 20, 20, 20},
		Rows: []Row{{
			Connector: &ConnectorSpec{Style: "line"},
			Cells: []Cell{
				{Shape: filled("rect")},
				{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"none"`)}}, // spacer
				{Shape: filled("rect")},
				{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: json.RawMessage(`"note"`)}}, // unboxed label
				{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`{"color":"dk2","width":0.75}`)}},
			},
		}},
	}
	res, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// Chain: cell0 → cell2 (across the spacer) → cell4 (outlined box).
	if len(res.Connectors) != 2 {
		t.Fatalf("want 2 connectors, got %d", len(res.Connectors))
	}
	ids := map[uint32]int{}
	for i, c := range res.Cells {
		ids[c.ID] = i
	}
	pairs := [][2]int{
		{ids[res.Connectors[0].SourceID], ids[res.Connectors[0].TargetID]},
		{ids[res.Connectors[1].SourceID], ids[res.Connectors[1].TargetID]},
	}
	if pairs[0] != [2]int{0, 2} || pairs[1] != [2]int{2, 4} {
		t.Errorf("connector pairs = %v, want [[0 2] [2 4]]", pairs)
	}
}

// A row_span parent fans out to each child row exactly once, with elbows for
// children whose centre is off the parent's centre line.
func TestResolveRowConnectors_FanOutFromSpan(t *testing.T) {
	conn := &ConnectorSpec{Style: "line"}
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 9144000, CY: 3000000},
		Columns: []float64{50, 50},
		Rows: []Row{
			{Connector: conn, Cells: []Cell{{RowSpan: 3, Shape: filled("rect")}, {Shape: filled("rect")}}},
			{Connector: conn, Cells: []Cell{{Shape: filled("rect")}}},
			{Connector: conn, Cells: []Cell{{Shape: filled("rect")}}},
		},
	}
	res, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Connectors) != 3 {
		t.Fatalf("want 3 fan-out connectors, got %d", len(res.Connectors))
	}
	parentID := res.Cells[0].ID
	for i, c := range res.Connectors {
		if c.SourceID != parentID {
			t.Errorf("connector %d source %d, want spanning parent %d", i, c.SourceID, parentID)
		}
		if c.StartSite != pptx.ConnectionSiteIndex(pptx.GeomRect, pptx.SideRight) ||
			c.EndSite != pptx.ConnectionSiteIndex(pptx.GeomRect, pptx.SideLeft) {
			t.Errorf("connector %d sites %d→%d, want right→left", i, c.StartSite, c.EndSite)
		}
	}
	// Top child is above the parent centre, middle is level, bottom below.
	if !res.Connectors[0].Elbow || !res.Connectors[0].FlipV {
		t.Errorf("top child: want elbow with FlipV, got %+v", res.Connectors[0])
	}
	if res.Connectors[1].Elbow {
		t.Errorf("middle child is level with the parent: want straight, got elbow")
	}
	if !res.Connectors[2].Elbow || res.Connectors[2].FlipV {
		t.Errorf("bottom child: want elbow without FlipV, got %+v", res.Connectors[2])
	}
}

func TestIsVisibleFillAndLine(t *testing.T) {
	cases := []struct {
		raw  string
		fill bool
		line bool
	}{
		{``, false, false},
		{`"none"`, false, false},
		{`"accent1"`, true, true},
		{`{"color":"accent1","alpha":40}`, true, true},
		{`{"color":"none"}`, false, false},
		{`{"color":"dk2","width":0}`, true, false},
	}
	for _, c := range cases {
		if got := isVisibleFill(json.RawMessage(c.raw)); got != c.fill {
			t.Errorf("isVisibleFill(%s) = %v, want %v", c.raw, got, c.fill)
		}
		if got := isVisibleLine(json.RawMessage(c.raw)); got != c.line {
			t.Errorf("isVisibleLine(%s) = %v, want %v", c.raw, got, c.line)
		}
	}
}

// Explicit links join cells across rows: same-column links run straight
// down, cross-column links turn in the gutter as an elbow, and a link to a
// spacer or a missing cell is dropped (go-slide-creator-0b3f6).
func TestResolveLinks_CrossRowRouting(t *testing.T) {
	arrow := &ConnectorSpec{Style: "arrow"}
	spacer := &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"none"`)}
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 9144000, CY: 3000000},
		Columns: []float64{50, 50},
		ColGap:  14,
		RowGap:  10,
		Rows: []Row{
			{Cells: []Cell{{Shape: filled("roundRect")}, {Shape: filled("roundRect")}}},
			{Cells: []Cell{{Shape: filled("roundRect")}, {Shape: spacer}}},
		},
		Links: []Link{
			{FromRow: 0, FromCol: 0, ToRow: 1, ToCol: 0, Spec: arrow}, // straight down
			{FromRow: 1, FromCol: 0, ToRow: 0, ToCol: 1, Spec: arrow}, // lane change: elbow
			{FromRow: 0, FromCol: 1, ToRow: 1, ToCol: 1, Spec: arrow}, // spacer target: dropped
			{FromRow: 0, FromCol: 0, ToRow: 5, ToCol: 0, Spec: arrow}, // out of range: dropped
		},
	}
	res, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Connectors) != 2 {
		t.Fatalf("want 2 connectors, got %d: %+v", len(res.Connectors), res.Connectors)
	}
	down, turn := res.Connectors[0], res.Connectors[1]
	if down.Elbow || down.Bounds.CX > 1 {
		t.Errorf("same-column link should be a straight vertical line, got %+v", down)
	}
	if !turn.Elbow || !turn.FlipV {
		t.Errorf("cross-column link to a higher row should be an upward elbow, got %+v", turn)
	}
	for _, c := range res.Connectors {
		if c.Spec != arrow {
			t.Errorf("link lost its spec: %+v", c)
		}
	}
}
