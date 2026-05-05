package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullQuote(t *testing.T) {
	p := &pullQuote{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "pull-quote" {
			t.Errorf("Name() = %q, want %q", p.Name(), "pull-quote")
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
		v := &PullQuoteValues{Quote: "The future is already here.", Attribution: "William Gibson"}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_full_fields", func(t *testing.T) {
		v := &PullQuoteValues{
			Quote:       "The best way to predict the future is to invent it.",
			Attribution: "Alan Kay",
			Role:        "Computer Scientist",
			AccentSide:  "right",
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_missing_quote", func(t *testing.T) {
		v := &PullQuoteValues{Quote: "", Attribution: "Someone"}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "values.quote is required") {
			t.Errorf("error %q does not mention values.quote required", err)
		}
	})

	t.Run("validate_missing_attribution", func(t *testing.T) {
		v := &PullQuoteValues{Quote: "Some quote.", Attribution: ""}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "values.attribution is required") {
			t.Errorf("error %q does not mention values.attribution required", err)
		}
	})

	t.Run("validate_invalid_accent_side", func(t *testing.T) {
		v := &PullQuoteValues{Quote: "Quote.", Attribution: "Author", AccentSide: "top"}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "accent_side") {
			t.Errorf("error %q does not mention accent_side", err)
		}
	})

	t.Run("validate_accent_side_none", func(t *testing.T) {
		v := &PullQuoteValues{Quote: "Quote.", Attribution: "Author", AccentSide: "none"}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("expand_default", func(t *testing.T) {
		v := &PullQuoteValues{
			Quote:       "The best way to predict the future is to invent it.",
			Attribution: "Alan Kay",
			Role:        "Computer Scientist",
		}
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
		cell := grid.Rows[0].Cells[0]
		if cell.Shape == nil {
			t.Fatal("cell.Shape is nil")
		}
		// Default accent side is left
		if cell.AccentBar == nil {
			t.Fatal("expected accent bar with default left position")
		}
		if cell.AccentBar.Position != "left" {
			t.Errorf("accent bar position = %q, want %q", cell.AccentBar.Position, "left")
		}
		if cell.AccentBar.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", cell.AccentBar.Color, "accent1")
		}
		// Check text has quote content
		textStr := string(cell.Shape.Text)
		if !strings.Contains(textStr, "predict the future") {
			t.Errorf("text does not contain quote: %s", textStr)
		}
		if !strings.Contains(textStr, "Alan Kay") {
			t.Errorf("text does not contain attribution: %s", textStr)
		}
		if !strings.Contains(textStr, "Computer Scientist") {
			t.Errorf("text does not contain role: %s", textStr)
		}
	})

	t.Run("expand_no_accent_bar", func(t *testing.T) {
		v := &PullQuoteValues{
			Quote:       "No bar.",
			Attribution: "Author",
			AccentSide:  "none",
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		cell := grid.Rows[0].Cells[0]
		if cell.AccentBar != nil {
			t.Error("expected no accent bar when accent_side=none")
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		v := &PullQuoteValues{Quote: "Test.", Attribution: "Author"}
		ovr := &PullQuoteOverrides{Accent: "accent4"}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		cell := grid.Rows[0].Cells[0]
		if cell.AccentBar == nil {
			t.Fatal("expected accent bar")
		}
		if cell.AccentBar.Color != "accent4" {
			t.Errorf("accent bar color = %q, want %q", cell.AccentBar.Color, "accent4")
		}
	})

	t.Run("golden_default", func(t *testing.T) {
		v := &PullQuoteValues{
			Quote:       "The best way to predict the future is to invent it.",
			Attribution: "Alan Kay",
			Role:        "Computer Scientist",
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "pull-quote", "default.golden.json")
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
