package patterns

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestKPINupAllVariantsRegistered(t *testing.T) {
	for n := 2; n <= 6; n++ {
		name := fmt.Sprintf("kpi-%dup", n)
		p, ok := Default().Get(name)
		if !ok {
			t.Errorf("%s not registered", name)
			continue
		}
		if p.Name() != name {
			t.Errorf("Name() = %q, want %q", p.Name(), name)
		}
		if p.Version() != 2 {
			t.Errorf("%s Version() = %d, want 2", name, p.Version())
		}
		if p.CellsHint() != fmt.Sprintf("%d", n) {
			t.Errorf("%s CellsHint() = %q, want %q", name, p.CellsHint(), fmt.Sprintf("%d", n))
		}

		// Schema should be valid
		s := p.Schema()
		if s == nil {
			t.Errorf("%s Schema() returned nil", name)
			continue
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Errorf("%s Schema marshal: %v", name, err)
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Errorf("%s Schema unmarshal: %v", name, err)
			continue
		}
		props := m["properties"].(map[string]any)
		valSchema := props["values"].(map[string]any)
		if valSchema["minItems"] != float64(n) {
			t.Errorf("%s values minItems = %v, want %d", name, valSchema["minItems"], n)
		}
		if valSchema["maxItems"] != float64(n) {
			t.Errorf("%s values maxItems = %v, want %d", name, valSchema["maxItems"], n)
		}

		// Exemplar should expand successfully
		ex, ok := p.(Exemplar)
		if !ok {
			t.Errorf("%s does not implement Exemplar", name)
			continue
		}
		vals := ex.ExemplarValues()
		grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
		if err != nil {
			t.Errorf("%s Expand exemplar: %v", name, err)
			continue
		}
		if len(grid.Rows[0].Cells) != n {
			t.Errorf("%s Expand produced %d cells, want %d", name, len(grid.Rows[0].Cells), n)
		}
	}
}

func TestKPINupSiblingHint(t *testing.T) {
	// Validating kpi-3up with 5 cells should hint kpi-5up.
	p, _ := Default().Get("kpi-3up")
	cells := KPINupValues{
		{Big: "1", Small: "a"},
		{Big: "2", Small: "b"},
		{Big: "3", Small: "c"},
		{Big: "4", Small: "d"},
		{Big: "5", Small: "e"},
	}
	err := p.Validate(&cells, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for wrong count")
	}
	errMsg := err.Error()
	if got := "kpi-5up"; !contains(errMsg, got) {
		t.Errorf("error %q should reference %q", errMsg, got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestKPINupNewVariantUnder30LOC(t *testing.T) {
	// Acceptance criterion: adding a parametric variant requires <30 LOC.
	// This is a documentation test — the KPINupConfig struct + registration
	// in kpi_variants.go is ~10 lines per variant.
	cfg := KPINupConfig{
		Count:        7,
		DensityClass: "high",
		Exemplars: []KPICell{
			{Big: "1", Small: "a"},
			{Big: "2", Small: "b"},
			{Big: "3", Small: "c"},
			{Big: "4", Small: "d"},
			{Big: "5", Small: "e"},
			{Big: "6", Small: "f"},
			{Big: "7", Small: "g"},
		},
	}
	p := NewKPINup(cfg)
	if p.Name() != "kpi-7up" {
		t.Errorf("Name() = %q, want kpi-7up", p.Name())
	}
	vals := KPINupValues(cfg.Exemplars)
	if err := p.Validate(&vals, nil, nil); err != nil {
		t.Errorf("Validate: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows[0].Cells) != 7 {
		t.Errorf("expected 7 cells, got %d", len(grid.Rows[0].Cells))
	}
}
