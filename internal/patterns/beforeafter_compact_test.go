package patterns

import (
	"testing"
)

func TestBeforeAfterCompact_Registered(t *testing.T) {
	_, ok := Default().Get("before-after-compact")
	if !ok {
		t.Fatal("before-after-compact pattern not registered")
	}
}

func TestBeforeAfterCompact_ExpandBasic(t *testing.T) {
	p, _ := Default().Get("before-after-compact")

	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Before", Items: []string{"Slow"}},
		After:  BeforeAfterColumn{Header: "After", Items: []string{"Fast"}},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// Must have bounds set to ~60% height
	if grid.Bounds == nil {
		t.Fatal("expected bounds to be set for compact variant")
	}
	if grid.Bounds.Height != 60 {
		t.Errorf("expected bounds height 60, got %v", grid.Bounds.Height)
	}

	if len(grid.Rows) != 2 {
		t.Errorf("expected 2 rows (header + body), got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 header cells, got %d", len(grid.Rows[0].Cells))
	}
}

func TestBeforeAfterCompact_ValidateMissingHeader(t *testing.T) {
	p, _ := Default().Get("before-after-compact")

	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "", Items: []string{"Slow"}},
		After:  BeforeAfterColumn{Header: "After", Items: []string{"Fast"}},
	}
	if err := p.Validate(vals, nil, nil); err == nil {
		t.Error("expected validation error for missing before.header")
	}
}

func TestBeforeAfterCompact_TaxonomyDensityLow(t *testing.T) {
	p, _ := Default().Get("before-after-compact")
	tax := p.Taxonomy()
	if tax.DensityClass != "low" {
		t.Errorf("expected DensityClass 'low', got %q", tax.DensityClass)
	}
}
