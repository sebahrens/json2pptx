package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// resolvePatternGrid expands a registered pattern and resolves it to absolute
// geometry exactly as the generate path does (minus XML emission).
func resolvePatternGrid(t *testing.T, name string, values any) *shapegrid.ResolveResult {
	t.Helper()
	p, ok := patterns.Default().Get(name)
	if !ok {
		t.Fatalf("pattern %q not registered", name)
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	in, err := p.Expand(ctx, values, nil, nil)
	if err != nil {
		t.Fatalf("expand %s: %v", name, err)
	}
	cols, err := resolveColumnsDTO(in.Columns, in.Rows)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	colGap, rowGap := in.ColGap, in.RowGap
	if colGap == 0 {
		colGap = in.Gap
	}
	if rowGap == 0 {
		rowGap = in.Gap
	}
	grid := &shapegrid.Grid{
		Bounds:  shapegrid.DefaultBounds(12192000, 6858000),
		Columns: cols,
		Rows:    convertGridRows(in.Rows),
		ColGap:  colGap,
		RowGap:  rowGap,
	}
	if err := shapegrid.Validate(grid); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	res, err := shapegrid.Resolve(grid, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatalf("resolve %s: %v", name, err)
	}
	return res
}

func overlapsInterior(a, b pptx.RectEmu) bool {
	return a.X < b.X+b.CX && b.X < a.X+a.CX && a.Y < b.Y+b.CY && b.Y < a.Y+a.CY
}

// go-slide-creator-2szb: the dual-org-ladder connector must run between the
// paired cards (in the column gap), never across a card and its text.
func TestDualOrgLadder_ConnectorsStayInGap(t *testing.T) {
	vals := &patterns.DualOrgLadderValues{
		OrgA: "Client", OrgB: "Consulting Firm",
		Rows: []patterns.DualOrgLadderRow{
			{ANameField: "Bob Jones", ATitle: "Executive Sponsor", BNameField: "Betty Smith", BTitle: "Engagement Partner"},
			{ANameField: "Alex Chen", ATitle: "Steering Committee", BNameField: "Maria Lopez", BTitle: "Account Director"},
			{ANameField: "Sara Patel", ATitle: "Programme Lead", BNameField: "Tom Becker", BTitle: "Delivery Lead"},
		},
	}
	res := resolvePatternGrid(t, "dual-org-ladder", vals)
	if len(res.Connectors) != 3 {
		t.Fatalf("want one connector per paired row (3), got %d", len(res.Connectors))
	}
	byID := map[uint32]shapegrid.ResolvedCell{}
	for _, c := range res.Cells {
		byID[c.ID] = c
	}
	for i, conn := range res.Connectors {
		src, tgt := byID[conn.SourceID], byID[conn.TargetID]
		if conn.Bounds.X < src.Bounds.X+src.Bounds.CX || conn.Bounds.X+conn.Bounds.CX > tgt.Bounds.X {
			t.Errorf("connector %d x-range [%d,%d] leaves the gap [%d,%d]", i,
				conn.Bounds.X, conn.Bounds.X+conn.Bounds.CX, src.Bounds.X+src.Bounds.CX, tgt.Bounds.X)
		}
		for _, c := range res.Cells {
			if overlapsInterior(conn.Bounds, c.Bounds) {
				t.Errorf("connector %d bounds %+v cross cell %d bounds %+v", i, conn.Bounds, c.ID, c.Bounds)
			}
		}
		// Sites must be the side midpoints facing each other: right of the
		// left card (rect site 3) → left of the right card (rect site 1).
		if conn.StartSite != pptx.ConnectionSiteIndex(pptx.GeomRect, pptx.SideRight) ||
			conn.EndSite != pptx.ConnectionSiteIndex(pptx.GeomRect, pptx.SideLeft) {
			t.Errorf("connector %d sites %d→%d, want right→left", i, conn.StartSite, conn.EndSite)
		}
		if conn.Elbow {
			t.Errorf("connector %d between level cards should be straight", i)
		}
	}
}

// go-slide-creator-doh6: driver-tree connectors fan out root → every branch
// → every leaf, as elbows, and never touch the annotation column or spacers.
func TestDriverTree_ConnectorsFanOutAsElbows(t *testing.T) {
	vals := &patterns.DriverTreeValues{
		Root: patterns.DriverTreeNode{Label: "EBITDA uplift", Unit: "$42m"},
		Branches: []patterns.DriverTreeBranch{
			{Label: "Revenue", Leaves: []string{"Price", "Cross-sell", "New markets"}, Annotation: "Largest lever"},
			{Label: "COGS", Leaves: []string{"Suppliers", "Yield"}},
			{Label: "Opex", Leaves: []string{"Shared services"}},
		},
	}
	res := resolvePatternGrid(t, "driver-tree", vals)

	byID := map[uint32]shapegrid.ResolvedCell{}
	var root shapegrid.ResolvedCell
	for _, c := range res.Cells {
		byID[c.ID] = c
		if c.RowIdx == 0 && c.ColIdx == 0 {
			root = c
		}
	}
	wantRootLinks, wantLeafLinks := 3, 6
	var rootLinks, leafLinks int
	for i, conn := range res.Connectors {
		src, tgt := byID[conn.SourceID], byID[conn.TargetID]
		switch {
		case src.ID == root.ID && tgt.ColIdx == 1:
			rootLinks++
		case src.ColIdx == 1 && tgt.ColIdx == 2:
			leafLinks++
		default:
			t.Errorf("connector %d links col %d → col %d (only root→branch and branch→leaf allowed)", i, src.ColIdx, tgt.ColIdx)
		}
		srcMid := src.Bounds.Y + src.Bounds.CY/2
		tgtMid := tgt.Bounds.Y + tgt.Bounds.CY/2
		if d := srcMid - tgtMid; (d > 12700 || d < -12700) && !conn.Elbow {
			t.Errorf("connector %d between different heights must be an elbow", i)
		}
		if conn.FlipV != (tgtMid < srcMid) {
			t.Errorf("connector %d FlipV=%v but target is above=%v", i, conn.FlipV, tgtMid < srcMid)
		}
	}
	if rootLinks != wantRootLinks || leafLinks != wantLeafLinks {
		t.Errorf("root→branch %d (want %d), branch→leaf %d (want %d)", rootLinks, wantRootLinks, leafLinks, wantLeafLinks)
	}
}
