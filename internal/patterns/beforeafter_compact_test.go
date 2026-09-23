package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
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

func TestBeforeAfterCompact_LightPanelsTallChevronAndMargins(t *testing.T) {
	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Today", Items: []string{"Manual handoffs"}},
		After:  BeforeAfterColumn{Header: "Target", Items: []string{"Automated routing"}},
	}
	grid, err := (&beforeAfterCompact{}).Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Bounds == nil || grid.Bounds.Height != 60 {
		t.Errorf("compact height cap changed: %+v", grid.Bounds)
	}
	separator := grid.Rows[0].Cells[1]
	if separator.RowSpan != 2 || separator.Shape.Geometry != "chevron" || len(separator.Shape.Text) != 0 {
		t.Errorf("separator must span both rows without inner arrow: %+v", separator)
	}
	if len(grid.Rows[1].Cells) != 2 {
		t.Fatalf("body must have two panels, got %d", len(grid.Rows[1].Cells))
	}
	for _, cell := range []*jsonschema.GridCellInput{
		grid.Rows[0].Cells[0], grid.Rows[0].Cells[2],
		grid.Rows[1].Cells[0], grid.Rows[1].Cells[1],
	} {
		body, err := shapegrid.ResolveTextInput(cell.Shape.Text)
		if err != nil {
			t.Fatal(err)
		}
		for side, got := range body.Insets {
			if got < 180000 || got > 180010 {
				t.Errorf("panel inset side %d = %d EMU, want 0.5 cm", side, got)
			}
		}
	}
	for _, cell := range grid.Rows[1].Cells {
		if got := string(cell.Shape.Fill); got != `{"color":"accent1","tint":12000}` {
			t.Errorf("body panel fill = %s, want light accent tint", got)
		}
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

func TestBeforeAfterCompact_RejectsCopyBeyondCompactBudget(t *testing.T) {
	pat := &beforeAfterCompact{}
	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Today", Items: []string{"A", "B", "C", "D"}},
		After:  BeforeAfterColumn{Header: "Target", Items: []string{"A", "B", "C", "D"}},
	}
	if err := pat.Validate(vals, nil, nil); err != nil {
		t.Fatalf("four short items should fit: %v", err)
	}
	vals.Before.Items = append(vals.Before.Items, "E")
	if err := pat.Validate(vals, nil, nil); err == nil || !strings.Contains(err.Error(), "before.items") {
		t.Errorf("five items should be rejected at before.items: %v", err)
	}
	vals.Before.Items = vals.Before.Items[:4]
	vals.After.Items[0] = strings.Repeat("x", 134)
	if err := pat.Validate(vals, nil, nil); err == nil || !strings.Contains(err.Error(), "after.items[0]") {
		t.Errorf("134-character item should be rejected: %v", err)
	}
}

func TestBeforeAfterCompact_TaxonomyDensityLow(t *testing.T) {
	p, _ := Default().Get("before-after-compact")
	tax := p.Taxonomy()
	if tax.DensityClass != "low" {
		t.Errorf("expected DensityClass 'low', got %q", tax.DensityClass)
	}
}
