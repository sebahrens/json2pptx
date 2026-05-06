package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeroDetail(t *testing.T) {
	p := &heroDetail{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "hero-detail" {
			t.Errorf("Name() = %q, want %q", p.Name(), "hero-detail")
		}
		if p.Version() != 1 {
			t.Errorf("Version() = %d, want 1", p.Version())
		}
		if p.CellsHint() != "1 + 2-4" {
			t.Errorf("CellsHint() = %q, want %q", p.CellsHint(), "1 + 2-4")
		}
		tax := p.Taxonomy()
		if tax.Category != "hero" {
			t.Errorf("Category = %q, want %q", tax.Category, "hero")
		}
		if tax.DensityClass != "low" {
			t.Errorf("DensityClass = %q, want %q", tax.DensityClass, "low")
		}
	})

	t.Run("interfaces", func(t *testing.T) {
		if !p.SupportsCallout() {
			t.Error("SupportsCallout() should be true")
		}
		if !p.SupportsInlineMarkdown() {
			t.Error("SupportsInlineMarkdown() should be true")
		}
		configs := p.BudgetConfigurations()
		if len(configs) != 3 {
			t.Errorf("BudgetConfigurations() returned %d configs, want 3", len(configs))
		}
	})

	t.Run("schema_valid_json_schema", func(t *testing.T) {
		s := p.Schema()
		if s == nil {
			t.Fatal("Schema() returned nil")
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Fatalf("Schema marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Schema unmarshal: %v", err)
		}
		if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("missing $schema draft 2020-12")
		}
		if m["type"] != "object" {
			t.Errorf("root type = %v, want object", m["type"])
		}
	})

	t.Run("validate_happy_path", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "$2.4B", Label: "TAM by FY27"},
			Details: []HeroDetailItem{
				{Title: "Growth", Body: "42% CAGR"},
				{Title: "Segment", Body: "Financial services"},
			},
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_four_details", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "99.9%", Label: "Uptime"},
			Details: []HeroDetailItem{
				{Title: "Q1"}, {Title: "Q2"}, {Title: "Q3"}, {Title: "Q4"},
			},
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_missing_hero_value", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "", Label: "TAM"},
			Details: []HeroDetailItem{{Title: "A"}, {Title: "B"}},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "hero.value is required") {
			t.Errorf("error %q does not mention hero.value required", err)
		}
	})

	t.Run("validate_missing_hero_label", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "$1B", Label: ""},
			Details: []HeroDetailItem{{Title: "A"}, {Title: "B"}},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "hero.label is required") {
			t.Errorf("error %q does not mention hero.label required", err)
		}
	})

	t.Run("validate_too_few_details", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "$1B", Label: "TAM"},
			Details: []HeroDetailItem{{Title: "Only one"}},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "details count") {
			t.Errorf("error %q does not mention details count", err)
		}
	})

	t.Run("validate_too_many_details", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "$1B", Label: "TAM"},
			Details: []HeroDetailItem{
				{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}, {Title: "E"},
			},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "details count") {
			t.Errorf("error %q does not mention details count", err)
		}
	})

	t.Run("validate_detail_title_required", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "$1B", Label: "TAM"},
			Details: []HeroDetailItem{{Title: ""}, {Title: "B"}},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "details[0].title is required") {
			t.Errorf("error %q does not mention details[0].title required", err)
		}
	})

	t.Run("validate_invalid_style", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "$1B", Label: "TAM"},
			Details: []HeroDetailItem{{Title: "A"}, {Title: "B"}},
		}
		ovr := &HeroDetailOverrides{Style: "fancy"}
		err := p.Validate(v, ovr, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "overrides.style") {
			t.Errorf("error %q does not mention overrides.style", err)
		}
	})

	t.Run("expand_cards_style", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "$2.4B", Label: "Addressable market"},
			Details: []HeroDetailItem{
				{Title: "Growth", Body: "42% CAGR"},
				{Title: "Segment", Body: "Financial services"},
				{Title: "Outlook", Body: "Strong tailwinds"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(grid.Rows))
		}
		// Row 1: hero (1 cell spanning 3 columns)
		if len(grid.Rows[0].Cells) != 1 {
			t.Fatalf("hero row: expected 1 cell, got %d", len(grid.Rows[0].Cells))
		}
		if grid.Rows[0].Cells[0].ColSpan != 3 {
			t.Errorf("hero cell ColSpan = %d, want 3", grid.Rows[0].Cells[0].ColSpan)
		}
		heroText := string(grid.Rows[0].Cells[0].Shape.Text)
		if !strings.Contains(heroText, "$2.4B") {
			t.Errorf("hero text does not contain value: %s", heroText)
		}
		// Row 2: 3 detail cards
		if len(grid.Rows[1].Cells) != 3 {
			t.Fatalf("detail row: expected 3 cells, got %d", len(grid.Rows[1].Cells))
		}
		detailText := string(grid.Rows[1].Cells[0].Shape.Text)
		if !strings.Contains(detailText, "Growth") {
			t.Errorf("first detail text does not contain title: %s", detailText)
		}
		// Cards style should have accent fill
		fillStr := string(grid.Rows[1].Cells[0].Shape.Fill)
		if !strings.Contains(fillStr, "accent1") {
			t.Errorf("cards style should have accent fill, got: %s", fillStr)
		}
	})

	t.Run("expand_minimal_style", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "42%", Label: "Growth"},
			Details: []HeroDetailItem{
				{Title: "A", Body: "Detail A"},
				{Title: "B", Body: "Detail B"},
			},
		}
		ovr := &HeroDetailOverrides{Style: "minimal"}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Minimal style should have lt1 fill and accent bar
		fillStr := string(grid.Rows[1].Cells[0].Shape.Fill)
		if !strings.Contains(fillStr, "lt1") {
			t.Errorf("minimal style should have lt1 fill, got: %s", fillStr)
		}
		if grid.Rows[1].Cells[0].AccentBar == nil {
			t.Error("minimal style should have accent bar")
		}
	})

	t.Run("expand_with_icon", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "$1B", Label: "Revenue"},
			Details: []HeroDetailItem{
				{Icon: "trending-up", Title: "Growth", Body: "Strong"},
				{Title: "Margin", Body: "Healthy"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// First detail cell should have an icon
		if grid.Rows[1].Cells[0].Shape.Icon == nil {
			t.Error("first detail cell should have icon")
		}
		if grid.Rows[1].Cells[0].Shape.Icon.Name != "trending-up" {
			t.Errorf("icon name = %q, want %q", grid.Rows[1].Cells[0].Shape.Icon.Name, "trending-up")
		}
		// Second detail cell should not have an icon
		if grid.Rows[1].Cells[1].Shape.Icon != nil {
			t.Error("second detail cell should not have icon")
		}
	})

	t.Run("expand_with_context", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{
				Value:   "$2.4B",
				Label:   "TAM by FY27",
				Context: "Source: McKinsey 2025",
			},
			Details: []HeroDetailItem{
				{Title: "A"}, {Title: "B"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		heroText := string(grid.Rows[0].Cells[0].Shape.Text)
		if !strings.Contains(heroText, "McKinsey") {
			t.Errorf("hero text should contain context: %s", heroText)
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "42%", Label: "Growth"},
			Details: []HeroDetailItem{{Title: "A"}, {Title: "B"}},
		}
		ovr := &HeroDetailOverrides{Accent: "accent3"}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		heroText := string(grid.Rows[0].Cells[0].Shape.Text)
		if !strings.Contains(heroText, "accent3") {
			t.Errorf("hero text should use accent3: %s", heroText)
		}
	})

	t.Run("expand_row_heights", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero:    HeroDetailHero{Value: "$1B", Label: "Revenue"},
			Details: []HeroDetailItem{{Title: "A"}, {Title: "B"}},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid.Rows[0].Height != 40 {
			t.Errorf("hero row height = %v, want 40", grid.Rows[0].Height)
		}
	})

	t.Run("golden_default", func(t *testing.T) {
		v := &HeroDetailValues{
			Hero: HeroDetailHero{Value: "$2.4B", Label: "Addressable AI consulting market by FY27"},
			Details: []HeroDetailItem{
				{Title: "Market Growth", Body: "42% CAGR driven by enterprise adoption"},
				{Title: "Key Segment", Body: "Financial services leads with 35% share"},
				{Title: "Outlook", Body: "Projected to reach $5.1B by FY30"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "hero-detail", "default.golden.json")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			t.Log("golden file updated")
			return
		}

		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
		}

		if string(got) != string(want) {
			t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}
