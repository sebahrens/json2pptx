package patterns

import (
	"testing"
)

func TestStylishPanels_Registration(t *testing.T) {
	p, ok := Default().Get("stylish-panels")
	if !ok {
		t.Fatal("expected stylish-panels to be registered in default registry")
	}
	if p.Name() != "stylish-panels" {
		t.Errorf("Name() = %q, want %q", p.Name(), "stylish-panels")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
}

func TestStylishPanels_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	vals := &StylishPanelsValues{
		{Title: "Strategy", Body: []string{"Market analysis", "Competitive positioning"}},
		{Title: "Execution", Body: []string{"Sprint planning", "Resource allocation"}},
		{Title: "Measurement", Body: []string{"KPI tracking", "Quarterly reviews"}},
	}
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestStylishPanels_Validate_TooFew(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	vals := &StylishPanelsValues{
		{Title: "One", Body: []string{"a"}},
		{Title: "Two", Body: []string{"b"}},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 panels")
	}
}

func TestStylishPanels_Validate_TooMany(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	vals := &StylishPanelsValues{
		{Title: "A", Body: []string{"a"}},
		{Title: "B", Body: []string{"b"}},
		{Title: "C", Body: []string{"c"}},
		{Title: "D", Body: []string{"d"}},
		{Title: "E", Body: []string{"e"}},
		{Title: "F", Body: []string{"f"}},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for more than 5 panels")
	}
}

func TestStylishPanels_Validate_EmptyTitle(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	vals := &StylishPanelsValues{
		{Title: "", Body: []string{"bullet"}},
		{Title: "B", Body: []string{"bullet"}},
		{Title: "C", Body: []string{"bullet"}},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for empty title")
	}
}

func TestStylishPanels_Validate_EmptyBody(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	vals := &StylishPanelsValues{
		{Title: "A", Body: nil},
		{Title: "B", Body: []string{"bullet"}},
		{Title: "C", Body: []string{"bullet"}},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for empty body")
	}
}

func TestStylishPanels_Expand(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	vals := &StylishPanelsValues{
		{Title: "Strategy", Body: []string{"Market analysis", "Positioning"}},
		{Title: "Execution", Body: []string{"Sprint planning"}},
		{Title: "Measurement", Body: []string{"KPI tracking", "Reviews", "Adjustments"}},
	}
	ctx := ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if len(grid.Rows) != 2 {
		t.Errorf("expected 2 rows (header + body), got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 header cells, got %d", len(grid.Rows[0].Cells))
	}
	if len(grid.Rows[1].Cells) != 3 {
		t.Errorf("expected 3 body cells, got %d", len(grid.Rows[1].Cells))
	}
}

func TestStylishPanels_Schema(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestStylishPanels_Taxonomy(t *testing.T) {
	p, _ := Default().Get("stylish-panels")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want %q", tax.Category, "structural")
	}
	if tax.DensityClass != "medium" {
		t.Errorf("DensityClass = %q, want %q", tax.DensityClass, "medium")
	}
}

func TestStylishPanels_Recommend(t *testing.T) {
	reg := Default()
	result := Recommend(reg, "3 strategic pillars with bullet details", &ContentHints{ItemCount: 3}, 5)
	found := false
	for _, c := range result.Candidates {
		if c.PatternName == "stylish-panels" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stylish-panels in recommendations for 'pillars' intent; got %+v", result.Candidates)
	}
}
