package patterns

import (
	"strings"
	"testing"
)

func TestDriverTree_Registration(t *testing.T) {
	p, ok := Default().Get("driver-tree")
	if !ok {
		t.Fatal("expected driver-tree to be registered in default registry")
	}
	if p.Name() != "driver-tree" {
		t.Errorf("Name() = %q, want %q", p.Name(), "driver-tree")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validDriverTreeValues(branches, leavesPerBranch int) *DriverTreeValues {
	v := &DriverTreeValues{
		Root: DriverTreeNode{Label: "Net Benefits", Unit: "($m USD)"},
	}
	branchLabels := []string{"Revenue Benefits", "Cost Benefits", "Risk Benefits", "Capital Benefits"}
	leafLabels := []string{
		"Reduce unscheduled outages",
		"Increase scheduling flexibility",
		"Lower maintenance spend",
		"Reduce overtime hours",
	}
	for i := 0; i < branches; i++ {
		leaves := make([]string, leavesPerBranch)
		for j := 0; j < leavesPerBranch; j++ {
			leaves[j] = leafLabels[j]
		}
		v.Branches = append(v.Branches, DriverTreeBranch{
			Label:  branchLabels[i],
			Unit:   "($m USD)",
			Leaves: leaves,
		})
	}
	return v
}

func TestDriverTree_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	cases := []struct{ branches, leaves int }{
		{2, 1}, {2, 4}, {3, 2}, {4, 1}, {4, 4},
	}
	for _, c := range cases {
		t.Run(t.Name(), func(t *testing.T) {
			if err := p.Validate(validDriverTreeValues(c.branches, c.leaves), nil, nil); err != nil {
				t.Errorf("branches=%d leaves=%d: unexpected error: %v", c.branches, c.leaves, err)
			}
		})
	}
}

func TestDriverTree_Validate_MissingRoot(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	v.Root.Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank root label")
	}
}

func TestDriverTree_Validate_TooFewBranches(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	v.Branches = v.Branches[:1]
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 2 branches")
	}
	if !strings.Contains(err.Error(), "card-grid") {
		t.Errorf("expected sibling hint mentioning card-grid, got: %v", err)
	}
}

func TestDriverTree_Validate_TooManyBranches(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(4, 2)
	v.Branches = append(v.Branches, DriverTreeBranch{Label: "Extra", Leaves: []string{"x"}})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 4 branches")
	}
}

func TestDriverTree_Validate_TooManyLeaves(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 4)
	v.Branches[0].Leaves = append(v.Branches[0].Leaves, "fifth")
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 4 leaves on a branch")
	}
}

func TestDriverTree_Validate_NoLeaves(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	v.Branches[1].Leaves = nil
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for branch with no leaves")
	}
}

func TestDriverTree_Validate_LabelTooLong(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	v.Branches[0].Label = strings.Repeat("X", 61)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for branch label > 60 chars")
	}
}

func TestDriverTree_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	overrides := map[int]any{99: &DriverTreeCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestDriverTree_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(3, 2)
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	// 3 branches × 2 leaves = 6 rows
	if got := len(grid.Rows); got != 6 {
		t.Fatalf("expected 6 rows (totalLeaves), got %d", got)
	}
	// Columns should be a JSON-array of widths
	if got := string(grid.Columns); !strings.HasPrefix(got, "[") {
		t.Errorf("expected columns to be a JSON array, got %s", got)
	}
}

func TestDriverTree_Expand_RootSpansAllRows(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 3) // 6 total leaves
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Root cell lives at the very first slot of row 0 with rowspan = totalLeaves.
	rootCell := grid.Rows[0].Cells[0]
	if rootCell == nil {
		t.Fatal("expected root cell on row 0")
	}
	if rootCell.RowSpan != 6 {
		t.Errorf("expected root rowspan=6, got %d", rootCell.RowSpan)
	}
}

func TestDriverTree_Expand_BranchSpansLeaves(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 3)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Row 0 cells: [root(rs=6), branch1(rs=3), leaf, …]
	if len(grid.Rows[0].Cells) < 3 {
		t.Fatalf("row 0 expected at least 3 cells, got %d", len(grid.Rows[0].Cells))
	}
	branch1 := grid.Rows[0].Cells[1]
	if branch1 == nil || branch1.RowSpan != 3 {
		t.Errorf("expected branch[0] rowspan=3 on row 0, got %+v", branch1)
	}
	// Rows 1, 2 are still inside branch 1 — they should only carry the leaf cell.
	for _, r := range []int{1, 2} {
		if got := len(grid.Rows[r].Cells); got != 1 {
			t.Errorf("row %d: expected 1 cell (leaf only, root+branch covered by rowspan), got %d", r, got)
		}
	}
	// Row 3 starts branch 2 — first cell should be the branch with rowspan=3.
	branch2 := grid.Rows[3].Cells[0]
	if branch2 == nil || branch2.RowSpan != 3 {
		t.Errorf("expected branch[1] rowspan=3 on row 3, got %+v", branch2)
	}
}

func TestDriverTree_Expand_AnnotationColumnPresent(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	v.Branches[0].Annotation = "Primary value driver"
	v.Branches[1].Annotation = "Secondary value driver"
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Row 0 holds root + branch1 + leaf + annot1 (4 cells).
	if got := len(grid.Rows[0].Cells); got != 4 {
		t.Errorf("row 0: expected 4 cells (root+branch+leaf+annot), got %d", got)
	}
	annot := grid.Rows[0].Cells[3]
	if annot == nil {
		t.Fatal("expected annotation cell at row 0 col 3")
	}
	if annot.RowSpan != 2 {
		t.Errorf("expected annotation rowspan=2, got %d", annot.RowSpan)
	}
	// Row 2 starts branch 2 — branch + leaf + annot (3 cells, root covered by rowspan).
	if got := len(grid.Rows[2].Cells); got != 3 {
		t.Errorf("row 2: expected 3 cells (branch2+leaf+annot2, root covered), got %d", got)
	}
}

func TestDriverTree_Expand_AnnotationColumnAbsent(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Row 0: root + branch1 + leaf (3 cells, no annotation column).
	if got := len(grid.Rows[0].Cells); got != 3 {
		t.Errorf("row 0: expected 3 cells, got %d", got)
	}
	// Row 1: leaf only (root + branch1 covered by rowspans).
	if got := len(grid.Rows[1].Cells); got != 1 {
		t.Errorf("row 1: expected 1 cell (leaf only), got %d", got)
	}
}

func TestDriverTree_Expand_ConnectorPresent(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, row := range grid.Rows {
		if row.Connector == nil {
			t.Errorf("row %d: expected connector spec for visible tree lines", i)
		}
	}
}

func TestDriverTree_Expand_RootFillIsAccent(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	root := grid.Rows[0].Cells[0]
	if root.Shape == nil {
		t.Fatal("expected root cell to have shape")
	}
	if !strings.Contains(string(root.Shape.Fill), "accent1") {
		t.Errorf("expected root fill to include accent1, got %q", string(root.Shape.Fill))
	}
}

func TestDriverTree_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	ovr := &DriverTreeOverrides{Accent: "accent4"}
	grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	root := grid.Rows[0].Cells[0]
	if !strings.Contains(string(root.Shape.Fill), "accent4") {
		t.Errorf("expected root fill to honour accent override, got %q", string(root.Shape.Fill))
	}
	if grid.Rows[0].Connector == nil || grid.Rows[0].Connector.Color != "accent4" {
		t.Errorf("expected connector color accent4, got %+v", grid.Rows[0].Connector)
	}
}

func TestDriverTree_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	v := validDriverTreeValues(2, 2)
	// Cell index 1 is the first branch (root is 0).
	co := map[int]any{1: &DriverTreeCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, co)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Branch cell already has an accent bar by default; override is a no-op
	// success path (does not duplicate or crash).
	if grid.Rows[0].Cells[1].AccentBar == nil {
		t.Error("expected branch cell to retain accent bar after override")
	}
}

func TestDriverTree_Schema(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestDriverTree_Taxonomy(t *testing.T) {
	p, _ := Default().Get("driver-tree")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want structural", tax.Category)
	}
	if tax.DensityClass != "medium" {
		t.Errorf("DensityClass = %q, want medium", tax.DensityClass)
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("expected non-empty NarrativeRole")
	}
}

func TestDriverTree_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"value driver tree", "value driver tree showing benefit decomposition", &ContentHints{ItemCount: 3}},
		{"cost driver tree", "cost driver tree across 3 categories", &ContentHints{ItemCount: 3}},
		{"metric decomposition", "decompose net benefits into revenue and cost drivers", &ContentHints{ItemCount: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "driver-tree" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected driver-tree in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}
