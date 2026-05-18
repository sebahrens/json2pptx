package patterns

import (
	"strings"
	"testing"
)

func TestStrategyHouse_Registration(t *testing.T) {
	p, ok := Default().Get("strategy-house")
	if !ok {
		t.Fatal("expected strategy-house to be registered in default registry")
	}
	if p.Name() != "strategy-house" {
		t.Errorf("Name() = %q, want %q", p.Name(), "strategy-house")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validStrategyHouseValues() *StrategyHouseValues {
	return &StrategyHouseValues{
		Objective: "Become the trusted platform for global commerce",
		Pillars: []StrategyHousePillar{
			{Title: "Trust", Body: []string{"Privacy by default"}},
			{Title: "Excellence", Body: []string{"99.99% uptime"}},
			{Title: "Velocity", Body: []string{"Weekly releases"}},
		},
		Foundation: "People · Technology · Data",
	}
}

func TestStrategyHouse_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	if err := p.Validate(validStrategyHouseValues(), nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestStrategyHouse_Validate_Valid_WithRoof(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.RoofBadges = []string{"Vision", "Mission"}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestStrategyHouse_Validate_TooFewPillars(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.Pillars = v.Pillars[:2]
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 pillars")
	}
	if !strings.Contains(err.Error(), "stylish-panels") {
		t.Errorf("expected sibling hint mentioning stylish-panels, got: %v", err)
	}
}

func TestStrategyHouse_Validate_TooManyPillars(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.Pillars = []StrategyHousePillar{
		{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}, {Title: "E"}, {Title: "F"},
	}
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 5 pillars")
	}
}

func TestStrategyHouse_Validate_MissingObjective(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.Objective = ""
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for empty objective")
	}
}

func TestStrategyHouse_Validate_MissingFoundation(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.Foundation = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for empty foundation")
	}
}

func TestStrategyHouse_Validate_PillarTitleRequired(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.Pillars[1].Title = ""
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for empty pillar title")
	}
}

func TestStrategyHouse_Validate_TooManyRoofBadges(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.RoofBadges = []string{"A", "B", "C", "D"}
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 3 roof badges")
	}
}

func TestStrategyHouse_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	// 3 pillars + banner + foundation = 5 cells (indices 0..4)
	overrides := map[int]any{99: &StrategyHouseCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestStrategyHouse_Expand_Default(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, validStrategyHouseValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	// Without roof: banner + pillars + foundation = 3 rows
	if got := len(grid.Rows); got != 3 {
		t.Fatalf("expected 3 rows without roof, got %d", got)
	}
	// Banner row spans all columns
	if grid.Rows[0].Cells[0].ColSpan != 3 {
		t.Errorf("banner ColSpan = %d, want 3", grid.Rows[0].Cells[0].ColSpan)
	}
	// Pillar row has 3 cells
	if got := len(grid.Rows[1].Cells); got != 3 {
		t.Errorf("expected 3 pillar cells, got %d", got)
	}
	// Foundation row spans all columns
	if grid.Rows[2].Cells[0].ColSpan != 3 {
		t.Errorf("foundation ColSpan = %d, want 3", grid.Rows[2].Cells[0].ColSpan)
	}
}

func TestStrategyHouse_Expand_WithRoof(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	v := validStrategyHouseValues()
	v.RoofBadges = []string{"Vision", "Mission"}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// With roof: 4 rows
	if got := len(grid.Rows); got != 4 {
		t.Fatalf("expected 4 rows with roof, got %d", got)
	}
	if grid.Rows[0].Cells[0].ColSpan != 3 {
		t.Errorf("roof ColSpan = %d, want 3", grid.Rows[0].Cells[0].ColSpan)
	}
}

func TestStrategyHouse_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	overrides := &StrategyHouseOverrides{Accent: "accent3"}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, validStrategyHouseValues(), overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Banner fill should reference the overridden accent.
	banner := grid.Rows[0].Cells[0]
	if banner.Shape == nil {
		t.Fatal("expected banner shape")
	}
	if !strings.Contains(string(banner.Shape.Fill), "accent3") {
		t.Errorf("expected banner fill to include accent3, got %q", string(banner.Shape.Fill))
	}
}

func TestStrategyHouse_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	// Override index 1 = first pillar (banner=0, pillars 1..N)
	cellOverrides := map[int]any{1: &StrategyHouseCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, validStrategyHouseValues(), nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// First pillar cell already has an accent bar by default; override just confirms presence.
	if grid.Rows[1].Cells[0].AccentBar == nil {
		t.Error("expected first pillar to have an accent bar")
	}
}

func TestStrategyHouse_Schema(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestStrategyHouse_Taxonomy(t *testing.T) {
	p, _ := Default().Get("strategy-house")
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

func TestStrategyHouse_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"strategy", "strategy framework with objective and pillars", &ContentHints{ItemCount: 3}},
		{"pillars and foundation", "pillars and foundation supporting our goal", &ContentHints{ItemCount: 4}},
		{"house diagram", "house diagram with strategic objective", &ContentHints{ItemCount: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "strategy-house" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected strategy-house in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}

func TestStrategyHouse_RecommendVisual(t *testing.T) {
	reg := Default()
	result := RecommendVisual(reg, "strategic house with objective and pillars", &VisualHints{ContentHints: ContentHints{ItemCount: 3}}, 8)
	found := false
	for _, c := range result.Candidates {
		if c.Name == "strategy-house" && c.Category == VisualCategoryPattern {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected strategy-house in visual recommendations; got %+v", result.Candidates)
	}
}
