package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTeamBios_Registration(t *testing.T) {
	p, ok := Default().Get("team-bios")
	if !ok {
		t.Fatal("expected team-bios to be registered in default registry")
	}
	if p.Name() != "team-bios" {
		t.Errorf("Name() = %q, want %q", p.Name(), "team-bios")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func TestTeamBios_Taxonomy(t *testing.T) {
	p, _ := Default().Get("team-bios")
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" {
		t.Errorf("taxonomy fields must be populated: %+v", tx)
	}
	if len(tx.NarrativeRole) == 0 {
		t.Errorf("narrative role must have at least one value")
	}
}

func validTeamBiosValues(n int) *TeamBiosValues {
	pool := []TeamBiosMember{
		{Name: "Jane Smith", Role: "Project Lead", Bio: "10 years in supply-chain strategy."},
		{Name: "Arun Patel", Role: "Lead Engineer", Bio: "Built data platforms at two scale-ups."},
		{Name: "Lila Romero", Role: "Design Lead", Bio: "Service-design specialist. Ex-IDEO."},
		{Name: "Tom Becker", Role: "Client Partner", Bio: "Account director for top-10 retailers."},
		{Name: "Mei Wong", Role: "Data Scientist", Bio: "Forecasting models for retail demand."},
		{Name: "Pedro Alvarez", Role: "Program Manager", Bio: "PMO lead for cross-border rollouts."},
		{Name: "Hana Sato", Role: "Engagement Manager", Bio: "Public sector engagements in EMEA."},
		{Name: "Ravi Iyer", Role: "Solution Architect", Bio: "Cloud architecture across hyperscalers."},
	}
	if n > len(pool) {
		n = len(pool)
	}
	members := make([]TeamBiosMember, n)
	copy(members, pool[:n])
	return &TeamBiosValues{Members: members}
}

func TestTeamBios_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("team-bios")
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if err := p.Validate(validTeamBiosValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected validation error: %v", n, err)
		}
	}
}

func TestTeamBios_Validate_TooMany(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(8)
	v.Members = append(v.Members, TeamBiosMember{Name: "Ninth", Role: "Extra"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 8 members")
	}
}

func TestTeamBios_Validate_MissingName(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(3)
	v.Members[1].Name = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank name")
	}
}

func TestTeamBios_Validate_MissingRole(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(3)
	v.Members[0].Role = ""
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for missing role")
	}
}

func TestTeamBios_Validate_BioTooLong(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(2)
	v.Members[0].Bio = strings.Repeat("X", teamBiosBioMaxChars+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for bio over char cap")
	}
}

func TestTeamBios_Validate_PhotoLabelTooLong(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(2)
	v.Members[0].PhotoLabel = "WAYTOOLONG"
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for photo_label > 8 chars")
	}
}

func TestTeamBios_Expand_FourMembers_SingleRowPair(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Fatalf("expected 2 grid rows (photo + text) for 4 members, got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 4 || len(grid.Rows[1].Cells) != 4 {
		t.Errorf("expected 4 cells in each row, got %d / %d",
			len(grid.Rows[0].Cells), len(grid.Rows[1].Cells))
	}
}

func TestTeamBios_Expand_FiveMembers_TwoRowPairs(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(5)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("expected 4 rows (2 photo + 2 text) for 5 members, got %d", len(grid.Rows))
	}
	// The fifth member sits in the second card-row, column 0. Columns 1–3 of the
	// second card-row must be filler cells (no text content).
	secondTextRow := grid.Rows[3]
	if len(secondTextRow.Cells) != 4 {
		t.Fatalf("second text row should still have 4 cells (with fillers), got %d", len(secondTextRow.Cells))
	}
	// Filler cells must have nil text (no name / role / bio).
	for i := 1; i < 4; i++ {
		if secondTextRow.Cells[i].Shape == nil {
			t.Errorf("filler cell %d missing shape", i)
			continue
		}
		if len(secondTextRow.Cells[i].Shape.Text) > 0 {
			t.Errorf("filler cell %d should have no text, got %s", i, string(secondTextRow.Cells[i].Shape.Text))
		}
	}
}

func TestTeamBios_Expand_EightMembers_FullSecondRow(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(8)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("expected 4 rows for 8 members, got %d", len(grid.Rows))
	}
	// All cells in all rows should have shapes (no fillers).
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell.Shape == nil {
				t.Errorf("row %d col %d: missing shape", ri, ci)
			}
		}
	}
}

func TestTeamBios_Expand_DerivesInitialsWhenPhotoLabelOmitted(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := &TeamBiosValues{
		Members: []TeamBiosMember{
			{Name: "Jane Smith", Role: "PM"},
			{Name: "María García-López", Role: "Eng"},
			{Name: "Madonna", Role: "Lead"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	photoRow := grid.Rows[0]
	wantInitials := []string{"JS", "MG", "M"}
	for i, want := range wantInitials {
		var obj struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(photoRow.Cells[i].Shape.Text, &obj); err != nil {
			t.Fatalf("unmarshal photo text col %d: %v", i, err)
		}
		if len(obj.Paragraphs) == 0 || obj.Paragraphs[0].Content != want {
			t.Errorf("col %d photo label = %q, want %q", i, obj.Paragraphs[0].Content, want)
		}
	}
}

func TestTeamBios_Expand_HonorsExplicitPhotoLabel(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := &TeamBiosValues{
		Members: []TeamBiosMember{
			{Name: "Jane Smith", Role: "PM", PhotoLabel: "JSm"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	var obj struct {
		Paragraphs []struct {
			Content string `json:"content"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal photo text: %v", err)
	}
	if obj.Paragraphs[0].Content != "JSm" {
		t.Errorf("photo label = %q, want %q", obj.Paragraphs[0].Content, "JSm")
	}
}

func TestTeamBios_Expand_RoleUsesAccentColor(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(2)
	ovr := &TeamBiosOverrides{Accent: "accent3"}
	grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	textCell := grid.Rows[1].Cells[0]
	var obj struct {
		Paragraphs []struct {
			Content string `json:"content"`
			Color   string `json:"color"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(textCell.Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal text cell: %v", err)
	}
	if len(obj.Paragraphs) < 2 {
		t.Fatalf("expected at least 2 paragraphs (name + role), got %d", len(obj.Paragraphs))
	}
	if obj.Paragraphs[1].Color != "accent3" {
		t.Errorf("role color = %q, want %q", obj.Paragraphs[1].Color, "accent3")
	}
}

func TestTeamBios_Expand_AppliesCellOverride(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(3)
	co := map[int]any{
		1: &TeamBiosCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, co)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	textCell := grid.Rows[1].Cells[1]
	if textCell.AccentBar == nil {
		t.Fatal("expected accent bar on member 1's text cell")
	}
	if textCell.AccentBar.Position != "left" {
		t.Errorf("accent bar position = %q, want %q", textCell.AccentBar.Position, "left")
	}
}

func TestTeamBios_PostExpandWarnings_LongBio(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := &TeamBiosValues{
		Members: []TeamBiosMember{
			{Name: "Jane", Role: "PM", Bio: "Short bio."},
			{Name: "Arun", Role: "Eng", Bio: strings.Repeat("one ", teamBiosMaxBioWords+5)},
		},
	}
	warner, ok := p.(PostExpandWarner)
	if !ok {
		t.Fatal("team-bios should implement PostExpandWarner")
	}
	ws := warner.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(ws) != 1 {
		t.Fatalf("expected exactly 1 BODY_TOO_LONG warning, got %d: %v", len(ws), ws)
	}
	if !strings.Contains(ws[0], "BODY_TOO_LONG") {
		t.Errorf("expected warning to mention BODY_TOO_LONG, got: %s", ws[0])
	}
	if !strings.Contains(ws[0], "members[1]") {
		t.Errorf("expected warning to cite members[1], got: %s", ws[0])
	}
}

func TestTeamBios_PostExpandWarnings_ShortBiosSilent(t *testing.T) {
	p, _ := Default().Get("team-bios")
	v := validTeamBiosValues(4)
	warner := p.(PostExpandWarner)
	if ws := warner.PostExpandWarnings(ExpandContext{}, v, nil); len(ws) != 0 {
		t.Errorf("expected no warnings for short bios, got: %v", ws)
	}
}

func TestTeamBios_Schema_Valid(t *testing.T) {
	p, _ := Default().Get("team-bios")
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

func TestTeamBios_ExemplarValues_ExpandsCleanly(t *testing.T) {
	p, _ := Default().Get("team-bios")
	ex, ok := p.(Exemplar)
	if !ok {
		t.Fatal("team-bios does not implement Exemplar")
	}
	vals := ex.ExemplarValues()
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("exemplar values failed validation: %v", err)
	}
	if _, err := p.Expand(ExpandContext{}, vals, nil, nil); err != nil {
		t.Fatalf("exemplar Expand: %v", err)
	}
}

func TestDeriveInitials(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Jane Smith", "JS"},
		{"María García-López", "MG"},
		{"Madonna", "M"},
		{"  ", "?"},
		{"j", "J"},
		{"Jean-Luc Picard", "JL"}, // hyphen splits → "Jean", "Luc", "Picard" → JL
		{"li", "L"},
		{"123 Numbers", "N"}, // digits skipped until letter found
	}
	for _, tt := range tests {
		got := deriveInitials(tt.in)
		if got != tt.want {
			t.Errorf("deriveInitials(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
