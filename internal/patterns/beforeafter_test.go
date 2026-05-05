package patterns

import (
	"testing"
)

func TestBeforeAfter_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("before-after")
	if !ok {
		t.Fatal("before-after pattern not registered")
	}

	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Before", Items: []string{"Slow", "Manual"}},
		After:  BeforeAfterColumn{Header: "After", Items: []string{"Fast", "Automated"}},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Errorf("expected 2 rows (header + body), got %d", len(grid.Rows))
	}
	// Header row has 3 cells (before, chevron, after)
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 header cells, got %d", len(grid.Rows[0].Cells))
	}
}

func TestBeforeAfter_ValidateMissingHeader(t *testing.T) {
	p, ok := Default().Get("before-after")
	if !ok {
		t.Fatal("before-after pattern not registered")
	}

	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "", Items: []string{"Slow"}},
		After:  BeforeAfterColumn{Header: "After", Items: []string{"Fast"}},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for missing before.header")
	}
}
