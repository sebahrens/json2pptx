package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatHero(t *testing.T) {
	p := &statHero{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "stat-hero" {
			t.Errorf("Name() = %q, want %q", p.Name(), "stat-hero")
		}
		if p.Version() != 1 {
			t.Errorf("Version() = %d, want 1", p.Version())
		}
		if p.CellsHint() != "1" {
			t.Errorf("CellsHint() = %q, want %q", p.CellsHint(), "1")
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
		v := &StatHeroValues{Value: "$2.4B", Label: "TAM by FY27"}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_full_fields", func(t *testing.T) {
		v := &StatHeroValues{
			Value:   "99.9%",
			Unit:    "uptime",
			Label:   "Service availability",
			Context: "Measured over trailing 12 months",
			Source:  "Internal monitoring dashboard",
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_missing_value", func(t *testing.T) {
		v := &StatHeroValues{Value: "", Label: "TAM"}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "values.value is required") {
			t.Errorf("error %q does not mention values.value required", err)
		}
	})

	t.Run("validate_missing_label", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: ""}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "values.label is required") {
			t.Errorf("error %q does not mention values.label required", err)
		}
	})

	t.Run("validate_value_too_long", func(t *testing.T) {
		v := &StatHeroValues{Value: "this value is way too long!!", Label: "TAM"}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "maxLength 20") {
			t.Errorf("error %q does not mention maxLength", err)
		}
	})

	t.Run("expand_minimal", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: "Addressable market"}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(grid.Rows))
		}
		if len(grid.Rows[0].Cells) != 1 {
			t.Fatalf("expected 1 cell, got %d", len(grid.Rows[0].Cells))
		}
		// Verify text contains the value
		cell := grid.Rows[0].Cells[0]
		if cell.Shape == nil {
			t.Fatal("cell.Shape is nil")
		}
		textStr := string(cell.Shape.Text)
		if !strings.Contains(textStr, "$2.4B") {
			t.Errorf("text does not contain value: %s", textStr)
		}
		if !strings.Contains(textStr, "Addressable market") {
			t.Errorf("text does not contain label: %s", textStr)
		}
	})

	t.Run("expand_with_unit", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Unit: "TAM", Label: "By FY27"}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		textStr := string(grid.Rows[0].Cells[0].Shape.Text)
		if !strings.Contains(textStr, "$2.4B TAM") {
			t.Errorf("text should contain value + unit: %s", textStr)
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		v := &StatHeroValues{Value: "42%", Label: "Growth"}
		ovr := &StatHeroOverrides{Accent: "accent3"}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		textStr := string(grid.Rows[0].Cells[0].Shape.Text)
		if !strings.Contains(textStr, "accent3") {
			t.Errorf("text should use accent3 color: %s", textStr)
		}
	})

	t.Run("golden_default", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: "Addressable AI consulting market by FY27"}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "stat-hero", "default.golden.json")
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
