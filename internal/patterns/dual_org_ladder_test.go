package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDualOrgLadder_Registration(t *testing.T) {
	p, ok := Default().Get("dual-org-ladder")
	if !ok {
		t.Fatal("expected dual-org-ladder to be registered in default registry")
	}
	if p.Name() != "dual-org-ladder" {
		t.Errorf("Name() = %q, want %q", p.Name(), "dual-org-ladder")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func TestDualOrgLadder_Taxonomy(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" {
		t.Errorf("taxonomy fields must be populated: %+v", tx)
	}
	if len(tx.NarrativeRole) == 0 {
		t.Errorf("narrative role must have at least one value")
	}
	if len(tx.PairsWith) == 0 {
		t.Errorf("PairsWith must have at least one sibling")
	}
}

func validDualOrgLadderValues(n int) *DualOrgLadderValues {
	pool := []DualOrgLadderRow{
		{ANameField: "Bob Jones", ATitle: "Executive Sponsor", BNameField: "Betty Smith", BTitle: "Engagement Partner"},
		{ANameField: "Alex Chen", ATitle: "Steering Committee", BNameField: "Maria Lopez", BTitle: "Account Director"},
		{ANameField: "Sara Patel", ATitle: "Programme Lead", BNameField: "Tom Becker", BTitle: "Delivery Lead"},
		{ANameField: "Jordan Park", ATitle: "Workstream Owner", BNameField: "Lila Romero", BTitle: "Senior Consultant"},
		{ANameField: "Priya Rao", ATitle: "Subject Matter Expert", BNameField: "Hana Sato", BTitle: "Functional Lead"},
		{ANameField: "Marc Dubois", ATitle: "Change Champion", BNameField: "Ravi Iyer", BTitle: "Solution Architect"},
	}
	if n > len(pool) {
		n = len(pool)
	}
	rows := make([]DualOrgLadderRow, n)
	copy(rows, pool[:n])
	return &DualOrgLadderValues{
		OrgA: "Client Organisation",
		OrgB: "Consulting Firm",
		Rows: rows,
	}
}

func TestDualOrgLadder_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	for _, n := range []int{2, 3, 4, 5, 6} {
		if err := p.Validate(validDualOrgLadderValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected validation error: %v", n, err)
		}
	}
}

func TestDualOrgLadder_Validate_TooFewRows(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(1)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 2 rows")
	}
	if !strings.Contains(err.Error(), "team-bios") {
		t.Errorf("expected sibling hint mentioning team-bios, got: %v", err)
	}
}

func TestDualOrgLadder_Validate_TooManyRows(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(6)
	v.Rows = append(v.Rows, DualOrgLadderRow{
		ANameField: "Seventh", ATitle: "Extra", BNameField: "Seventh", BTitle: "Extra",
	})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 6 rows")
	}
}

func TestDualOrgLadder_Validate_MissingOrgName(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	v.OrgA = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank org_a")
	}
}

func TestDualOrgLadder_Validate_MissingRoleFields(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	cases := []struct {
		name string
		mut  func(*DualOrgLadderRow)
	}{
		{"missing a_name", func(r *DualOrgLadderRow) { r.ANameField = "" }},
		{"missing a_title", func(r *DualOrgLadderRow) { r.ATitle = "" }},
		{"missing b_name", func(r *DualOrgLadderRow) { r.BNameField = "" }},
		{"missing b_title", func(r *DualOrgLadderRow) { r.BTitle = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validDualOrgLadderValues(3)
			tc.mut(&v.Rows[1])
			if err := p.Validate(v, nil, nil); err == nil {
				t.Fatalf("expected validation error when %s", tc.name)
			}
		})
	}
}

func TestDualOrgLadder_Validate_FieldsTooLong(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	v.Rows[0].ANameField = strings.Repeat("X", dualOrgLadderNameMaxChars+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for over-long a_name")
	}
}

func TestDualOrgLadder_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	// 3 body rows + 1 header = 4 valid keys (0..3). 99 is out of range.
	overrides := map[int]any{99: &DualOrgLadderCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestDualOrgLadder_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if len(grid.Rows) != 5 {
		t.Fatalf("expected 5 rows (1 header + 4 body), got %d", len(grid.Rows))
	}
	for i, row := range grid.Rows {
		if len(row.Cells) != 2 {
			t.Errorf("row %d: expected 2 cells, got %d", i, len(row.Cells))
		}
	}
	if string(grid.Columns) != "2" {
		t.Errorf("expected columns=2, got %s", string(grid.Columns))
	}
}

func TestDualOrgLadder_Expand_BoundaryRowCounts(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	for _, n := range []int{2, 6} {
		v := validDualOrgLadderValues(n)
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("n=%d: Expand failed: %v", n, err)
		}
		if got := len(grid.Rows); got != n+1 {
			t.Errorf("n=%d: expected %d rows (header + body), got %d", n, n+1, got)
		}
	}
}

func TestDualOrgLadder_Expand_HeaderCellsUseAccentFill(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	header := grid.Rows[0]
	if !strings.Contains(string(header.Cells[0].Shape.Fill), "accent1") {
		t.Errorf("left header fill should default to accent1, got %q", string(header.Cells[0].Shape.Fill))
	}
	if !strings.Contains(string(header.Cells[1].Shape.Fill), "accent2") {
		t.Errorf("right header fill should default to accent2, got %q", string(header.Cells[1].Shape.Fill))
	}
}

func TestDualOrgLadder_Expand_HeaderTextIsWhiteAndBold(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(2)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	var obj struct {
		Paragraphs []struct {
			Content string `json:"content"`
			Bold    bool   `json:"bold"`
			Color   string `json:"color"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal header text: %v", err)
	}
	if len(obj.Paragraphs) == 0 {
		t.Fatal("expected at least one paragraph in header")
	}
	if obj.Paragraphs[0].Content != v.OrgA {
		t.Errorf("header content = %q, want %q", obj.Paragraphs[0].Content, v.OrgA)
	}
	if !obj.Paragraphs[0].Bold {
		t.Error("expected header text to be bold")
	}
	if obj.Paragraphs[0].Color != "lt1" {
		t.Errorf("expected header text color lt1, got %q", obj.Paragraphs[0].Color)
	}
}

func TestDualOrgLadder_Expand_BodyCellsHaveBorderNoFill(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(2)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	body := grid.Rows[1]
	for i, cell := range body.Cells {
		if !strings.Contains(string(cell.Shape.Fill), "none") {
			t.Errorf("body cell %d: expected fill 'none', got %q", i, string(cell.Shape.Fill))
		}
		if len(cell.Shape.Line) == 0 {
			t.Errorf("body cell %d: expected a border line spec", i)
		}
	}
}

func TestDualOrgLadder_Expand_ConnectorsDefaultOn(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Header row should NOT have a connector — only body rows.
	if grid.Rows[0].Connector != nil {
		t.Error("header row should not have a connector")
	}
	for i := 1; i < len(grid.Rows); i++ {
		if grid.Rows[i].Connector == nil {
			t.Errorf("body row %d: expected default connector, got nil", i)
			continue
		}
		if grid.Rows[i].Connector.Style != "line" {
			t.Errorf("body row %d: expected line connector, got %q", i, grid.Rows[i].Connector.Style)
		}
		if grid.Rows[i].Connector.Color != "accent1" {
			t.Errorf("body row %d: expected connector color accent1, got %q", i, grid.Rows[i].Connector.Color)
		}
	}
}

func TestDualOrgLadder_Expand_ConnectorsExplicitlyDisabled(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	off := false
	v.ShowConnectors = &off
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i := 1; i < len(grid.Rows); i++ {
		if grid.Rows[i].Connector != nil {
			t.Errorf("body row %d: expected no connector when show_connectors=false", i)
		}
	}
}

func TestDualOrgLadder_Expand_AccentOverrides(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	ovr := &DualOrgLadderOverrides{AccentA: "accent4", AccentB: "accent5"}
	grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if !strings.Contains(string(grid.Rows[0].Cells[0].Shape.Fill), "accent4") {
		t.Errorf("left header should use accent4, got %q", string(grid.Rows[0].Cells[0].Shape.Fill))
	}
	if !strings.Contains(string(grid.Rows[0].Cells[1].Shape.Fill), "accent5") {
		t.Errorf("right header should use accent5, got %q", string(grid.Rows[0].Cells[1].Shape.Fill))
	}
	if grid.Rows[1].Connector == nil || grid.Rows[1].Connector.Color != "accent4" {
		t.Errorf("connector color should follow accent_a override, got %+v", grid.Rows[1].Connector)
	}
}

func TestDualOrgLadder_Expand_AppliesCellOverride(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	v := validDualOrgLadderValues(3)
	// Override body row index 2 (cell_overrides key 2 maps to v.Rows[1]).
	co := map[int]any{
		2: &DualOrgLadderCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, co)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	target := grid.Rows[2]
	if target.Cells[0].AccentBar == nil {
		t.Fatal("expected accent bar on row 2 left cell")
	}
	if target.Cells[1].AccentBar == nil {
		t.Fatal("expected accent bar on row 2 right cell")
	}
}

func TestDualOrgLadder_Schema_Valid(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	sch := p.Schema()
	if sch == nil {
		t.Fatal("Schema() returned nil")
	}
	data, err := json.Marshal(sch)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	if len(data) == 0 {
		t.Error("schema marshalled to empty bytes")
	}
}

func TestDualOrgLadder_ExemplarValues_ExpandsCleanly(t *testing.T) {
	p, _ := Default().Get("dual-org-ladder")
	ex, ok := p.(Exemplar)
	if !ok {
		t.Fatal("dual-org-ladder does not implement Exemplar")
	}
	vals := ex.ExemplarValues()
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("exemplar values failed validation: %v", err)
	}
	if _, err := p.Expand(ExpandContext{}, vals, nil, nil); err != nil {
		t.Fatalf("exemplar Expand: %v", err)
	}
}

func TestDualOrgLadder_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"joint venture team", "joint venture engagement team", &ContentHints{ItemCount: 4}},
		{"dual org ladder", "dual org ladder for client/consultant pairs", &ContentHints{ItemCount: 5}},
		{"client team pairs", "client and consulting partner team pairs", &ContentHints{ItemCount: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "dual-org-ladder" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected dual-org-ladder in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}
