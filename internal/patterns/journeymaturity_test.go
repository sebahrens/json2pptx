package patterns

import (
	"strings"
	"testing"
)

func TestJourneyMaturity_Registration(t *testing.T) {
	p, ok := Default().Get("journey-maturity-model")
	if !ok {
		t.Fatal("expected journey-maturity-model to be registered in default registry")
	}
	if p.Name() != "journey-maturity-model" {
		t.Errorf("Name() = %q, want %q", p.Name(), "journey-maturity-model")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validJourneyMaturityValues(n int) *JourneyMaturityValues {
	labels := []string{"Initial", "Developing", "Defined", "Managed", "Optimising", "Innovating"}
	descs := []string{
		"Ad hoc processes; no formal practices in place.",
		"Repeatable practices; informal governance.",
		"Standardised practices; documented playbooks.",
		"Quantitatively measured outcomes; continuous review.",
		"Continuous improvement embedded across the organisation.",
		"Practices drive new revenue streams and partnerships.",
	}
	stages := make([]JourneyMaturityStage, n)
	for i := 0; i < n; i++ {
		stages[i] = JourneyMaturityStage{Label: labels[i], Description: descs[i]}
	}
	return &JourneyMaturityValues{Stages: stages}
}

func TestJourneyMaturity_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(t.Name(), func(t *testing.T) {
			if err := p.Validate(validJourneyMaturityValues(n), nil, nil); err != nil {
				t.Errorf("n=%d: unexpected validation error: %v", n, err)
			}
		})
	}
}

func TestJourneyMaturity_Validate_TooFewStages(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(3)
	v.Stages = v.Stages[:2]
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 stages")
	}
	if !strings.Contains(err.Error(), "value-chain") {
		t.Errorf("expected sibling hint mentioning value-chain, got: %v", err)
	}
}

func TestJourneyMaturity_Validate_TooManyStages(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(6)
	v.Stages = append(v.Stages, JourneyMaturityStage{Label: "Seventh"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 6 stages")
	}
}

func TestJourneyMaturity_Validate_MissingLabel(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	v.Stages[2].Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank label")
	}
}

func TestJourneyMaturity_Validate_LabelTooLong(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	v.Stages[1].Label = strings.Repeat("X", 41)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for label > 40 chars")
	}
}

func TestJourneyMaturity_Validate_DescriptionTooLong(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	v.Stages[1].Description = strings.Repeat("X", 181)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for description > 180 chars")
	}
}

func TestJourneyMaturity_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	overrides := map[int]any{99: &JourneyMaturityCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestJourneyMaturity_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 3 {
		t.Fatalf("expected 3 rows (header + body + marker), got %d", got)
	}
	for r, expectedHeight := range []float64{30, 50, 20} {
		if grid.Rows[r].Height != expectedHeight {
			t.Errorf("row %d: expected height %.0f, got %.0f", r, expectedHeight, grid.Rows[r].Height)
		}
	}
	for r := 0; r < 3; r++ {
		if got := len(grid.Rows[r].Cells); got != 4 {
			t.Errorf("row %d: expected 4 cells, got %d", r, got)
		}
	}
	if string(grid.Columns) != "4" {
		t.Errorf("expected columns=4, got %s", string(grid.Columns))
	}
	if grid.Rows[0].Connector == nil {
		t.Error("expected connector on the header row (arrows between stages)")
	}
}

func TestJourneyMaturity_Expand_BoundaryStageCounts(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	for _, n := range []int{3, 6} {
		t.Run(t.Name(), func(t *testing.T) {
			v := validJourneyMaturityValues(n)
			grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
			if err != nil {
				t.Fatalf("n=%d: Expand failed: %v", n, err)
			}
			for r := 0; r < 3; r++ {
				if got := len(grid.Rows[r].Cells); got != n {
					t.Errorf("n=%d row %d: expected %d cells, got %d", n, r, n, got)
				}
			}
		})
	}
}

func TestJourneyMaturity_Expand_CurrentStageMarker(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(5)
	v.Stages[2].Current = true
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	currentMarker := grid.Rows[2].Cells[2]
	if currentMarker.Shape == nil {
		t.Fatal("expected shape on current-stage marker cell")
	}
	if currentMarker.Shape.Geometry != "triangle" {
		t.Errorf("expected triangle geometry on current-stage marker, got %q", currentMarker.Shape.Geometry)
	}
	if currentMarker.Shape.Rotation != 180 {
		t.Errorf("expected rotation 180 for downward triangle, got %v", currentMarker.Shape.Rotation)
	}

	for i, cell := range grid.Rows[2].Cells {
		if i == 2 {
			continue
		}
		if cell.Shape == nil {
			t.Fatalf("non-current marker cell %d: expected non-nil shape", i)
		}
		if cell.Shape.Geometry != "rect" {
			t.Errorf("non-current marker cell %d: expected rect geometry, got %q", i, cell.Shape.Geometry)
		}
	}

	currentHeader := grid.Rows[0].Cells[2]
	if currentHeader.AccentBar == nil {
		t.Error("expected accent bar on current stage header")
	}
	if currentHeader.AccentBar != nil && currentHeader.AccentBar.Position != "left" {
		t.Errorf("expected left-positioned accent bar, got %q", currentHeader.AccentBar.Position)
	}

	for i, cell := range grid.Rows[0].Cells {
		if i == 2 {
			continue
		}
		if cell.AccentBar != nil {
			t.Errorf("header %d: did not expect accent bar on non-current header", i)
		}
	}
}

func TestJourneyMaturity_Expand_NoCurrentMarker(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, cell := range grid.Rows[2].Cells {
		if cell.Shape == nil {
			t.Fatalf("marker cell %d: expected non-nil shape, got nil", i)
		}
		if cell.Shape.Geometry != "rect" {
			t.Errorf("marker cell %d: expected rect (no current stage), got %q", i, cell.Shape.Geometry)
		}
	}
}

func TestJourneyMaturity_Expand_AutoNumbersStages(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, cell := range grid.Rows[0].Cells {
		want := []string{"1. Initial", "2. Developing", "3. Defined", "4. Managed"}[i]
		if !strings.Contains(string(cell.Shape.Text), want) {
			t.Errorf("header %d: expected to contain %q, got %q", i, want, string(cell.Shape.Text))
		}
	}
}

func TestJourneyMaturity_Expand_RespectsExplicitNumber(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(3)
	v.Stages[0].Number = 5
	v.Stages[1].Number = 6
	v.Stages[2].Number = 7
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if !strings.Contains(string(grid.Rows[0].Cells[0].Shape.Text), "5. Initial") {
		t.Errorf("expected header text to start with '5. Initial', got %q", string(grid.Rows[0].Cells[0].Shape.Text))
	}
}

func TestJourneyMaturity_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	v.Stages[1].Current = true
	overrides := &JourneyMaturityOverrides{Accent: "accent4"}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Connector == nil || grid.Rows[0].Connector.Color != "accent4" {
		t.Errorf("expected connector color accent4 from override, got %+v", grid.Rows[0].Connector)
	}
	currentMarker := grid.Rows[2].Cells[1]
	if !strings.Contains(string(currentMarker.Shape.Fill), "accent4") {
		t.Errorf("expected current marker fill to follow accent override, got %q", string(currentMarker.Shape.Fill))
	}
	currentHeader := grid.Rows[0].Cells[1]
	if !strings.Contains(string(currentHeader.Shape.Fill), "accent4") {
		t.Errorf("expected current header fill to follow accent override, got %q", string(currentHeader.Shape.Fill))
	}
}

func TestJourneyMaturity_PostExpandWarnings_MultipleCurrent(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	v.Stages[0].Current = true
	v.Stages[2].Current = true

	warner, ok := p.(PostExpandWarner)
	if !ok {
		t.Fatal("journey-maturity-model must implement PostExpandWarner for multiple-current advisory")
	}
	warnings := warner.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(warnings) == 0 {
		t.Fatal("expected MULTIPLE_CURRENT_STAGES warning when more than one stage is current")
	}
	if !strings.Contains(warnings[0], "MULTIPLE_CURRENT_STAGES") {
		t.Errorf("expected MULTIPLE_CURRENT_STAGES code, got %q", warnings[0])
	}
}

func TestJourneyMaturity_PostExpandWarnings_SingleCurrent(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	v.Stages[1].Current = true

	warner, ok := p.(PostExpandWarner)
	if !ok {
		t.Fatal("journey-maturity-model must implement PostExpandWarner")
	}
	warnings := warner.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for single current stage, got %v", warnings)
	}
}

func TestJourneyMaturity_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	v := validJourneyMaturityValues(4)
	cellOverrides := map[int]any{1: &JourneyMaturityCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[1].AccentBar == nil {
		t.Error("expected accent bar on header cell with cell override")
	}
}

func TestJourneyMaturity_Schema(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestJourneyMaturity_Taxonomy(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want %q", tax.Category, "structural")
	}
	if tax.DensityClass != "medium" {
		t.Errorf("DensityClass = %q, want %q", tax.DensityClass, "medium")
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("expected NarrativeRole to be non-empty")
	}
	if len(tax.PairsWith) == 0 {
		t.Error("expected PairsWith to be non-empty")
	}
}

func TestJourneyMaturity_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"digital maturity", "digital maturity model with 5 stages", &ContentHints{ItemCount: 5}},
		{"capability maturity", "capability maturity ladder across 4 stages", &ContentHints{ItemCount: 4}},
		{"customer journey maturity", "customer journey maturity progression", &ContentHints{ItemCount: 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "journey-maturity-model" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected journey-maturity-model in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}
