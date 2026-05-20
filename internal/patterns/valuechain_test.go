package patterns

import (
	"strings"
	"testing"
)

func TestValueChain_Registration(t *testing.T) {
	p, ok := Default().Get("value-chain")
	if !ok {
		t.Fatal("expected value-chain to be registered in default registry")
	}
	if p.Name() != "value-chain" {
		t.Errorf("Name() = %q, want %q", p.Name(), "value-chain")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validValueChainValues(n int) *ValueChainValues {
	steps := make([]ValueChainStep, n)
	labels := []string{"Extraction", "Processing", "Manufacturing", "Distribution", "Retail", "Service", "Recycling", "Disposal", "Recovery", "Renewal"}
	descs := []string{
		"Mining raw materials and managing EPC contracts.",
		"Refining ore into intermediate inputs.",
		"Converting inputs into finished goods.",
		"Moving product through wholesale channels.",
		"Reaching end customers via partner stores.",
		"Supporting customers throughout product lifecycle.",
		"Recovering materials for reuse.",
		"Decommissioning end-of-life inventory.",
		"Returning value back into supply.",
		"Closing the loop on the chain.",
	}
	for i := 0; i < n; i++ {
		steps[i] = ValueChainStep{Label: labels[i], Description: descs[i]}
	}
	return &ValueChainValues{Steps: steps}
}

func TestValueChain_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("value-chain")
	for _, n := range []int{4, 5, 8, 10} {
		t.Run(t.Name(), func(t *testing.T) {
			if err := p.Validate(validValueChainValues(n), nil, nil); err != nil {
				t.Errorf("n=%d: unexpected validation error: %v", n, err)
			}
		})
	}
}

func TestValueChain_Validate_TooFewSteps(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(3)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 4 steps")
	}
	if !strings.Contains(err.Error(), "process-flow") {
		t.Errorf("expected sibling hint mentioning process-flow, got: %v", err)
	}
}

func TestValueChain_Validate_TooManySteps(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(10)
	v.Steps = append(v.Steps, ValueChainStep{Label: "Eleventh"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 10 steps")
	}
}

func TestValueChain_Validate_MissingLabel(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[2].Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank label")
	}
}

func TestValueChain_Validate_LabelTooLong(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[1].Label = strings.Repeat("X", 41)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for label > 40 chars")
	}
}

func TestValueChain_Validate_DescriptionTooLong(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[1].Description = strings.Repeat("X", 181)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for description > 180 chars")
	}
}

func TestValueChain_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	overrides := map[int]any{99: &ValueChainCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestValueChain_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(8)
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 2 {
		t.Fatalf("expected 2 rows (label + description), got %d", got)
	}
	if got := len(grid.Rows[0].Cells); got != 8 {
		t.Errorf("expected 8 label cells, got %d", got)
	}
	if got := len(grid.Rows[1].Cells); got != 8 {
		t.Errorf("expected 8 description cells, got %d", got)
	}
	if string(grid.Columns) != "8" {
		t.Errorf("expected columns=8, got %s", string(grid.Columns))
	}
	if grid.Rows[0].Connector == nil {
		t.Error("expected connector on the label row (arrows between steps)")
	}
}

func TestValueChain_Expand_BoundaryStepCounts(t *testing.T) {
	p, _ := Default().Get("value-chain")
	for _, n := range []int{4, 10} {
		t.Run(t.Name(), func(t *testing.T) {
			v := validValueChainValues(n)
			grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
			if err != nil {
				t.Fatalf("n=%d: Expand failed: %v", n, err)
			}
			if got := len(grid.Rows[0].Cells); got != n {
				t.Errorf("n=%d: expected %d label cells, got %d", n, n, got)
			}
			if got := len(grid.Rows[1].Cells); got != n {
				t.Errorf("n=%d: expected %d description cells, got %d", n, n, got)
			}
		})
	}
}

func TestValueChain_Expand_HighlightDefaultColor(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[2].Highlight = true
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Step 2 (highlighted) should fill with accent2 (default highlight color).
	highlighted := grid.Rows[0].Cells[2]
	if highlighted.Shape == nil {
		t.Fatal("expected shape on highlighted label cell")
	}
	if !strings.Contains(string(highlighted.Shape.Fill), "accent2") {
		t.Errorf("expected highlighted label fill to include accent2, got %q", string(highlighted.Shape.Fill))
	}
	// Step 0 (not highlighted) should fill with dk1.
	plain := grid.Rows[0].Cells[0]
	if !strings.Contains(string(plain.Shape.Fill), "dk1") {
		t.Errorf("expected non-highlighted label fill to include dk1, got %q", string(plain.Shape.Fill))
	}
}

func TestValueChain_Expand_HighlightCustomColor(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	v.HighlightColor = "accent3"
	v.Steps[0].Highlight = true
	v.Steps[3].Highlight = true
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for _, idx := range []int{0, 3} {
		cell := grid.Rows[0].Cells[idx]
		if !strings.Contains(string(cell.Shape.Fill), "accent3") {
			t.Errorf("step %d: expected fill to include accent3, got %q", idx, string(cell.Shape.Fill))
		}
	}
}

func TestValueChain_Expand_DescriptionFillIsBg1(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, cell := range grid.Rows[1].Cells {
		if cell.Shape == nil {
			t.Fatalf("description cell %d: expected shape", i)
		}
		if !strings.Contains(string(cell.Shape.Fill), "bg1") {
			t.Errorf("description cell %d: expected bg1 fill, got %q", i, string(cell.Shape.Fill))
		}
	}
}

func TestValueChain_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	overrides := &ValueChainOverrides{Accent: "accent4"}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Connector color should follow the accent override.
	if grid.Rows[0].Connector == nil {
		t.Fatal("expected connector on label row")
	}
	if grid.Rows[0].Connector.Color != "accent4" {
		t.Errorf("expected connector color accent4, got %q", grid.Rows[0].Connector.Color)
	}
}

func TestValueChain_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	cellOverrides := map[int]any{1: &ValueChainCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[1].AccentBar == nil {
		t.Error("expected accent bar on label cell with override")
	}
}

func TestValueChain_Schema(t *testing.T) {
	p, _ := Default().Get("value-chain")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestValueChain_Taxonomy(t *testing.T) {
	p, _ := Default().Get("value-chain")
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

func TestValueChain_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"porter value chain", "Porter value chain across primary activities", &ContentHints{ItemCount: 6}},
		{"supply chain mapping", "supply chain mapping with 5 steps", &ContentHints{ItemCount: 5}},
		{"operations chain", "operations chain from raw to retail", &ContentHints{ItemCount: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "value-chain" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected value-chain in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}
