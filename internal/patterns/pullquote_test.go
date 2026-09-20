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
		if p.Version() != 2 {
			t.Errorf("Version() = %d, want 2", p.Version())
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
		// Quote and attribution are separate rows, so the quote's autofit
		// cannot shrink the attribution and their ink cannot collide
		// (go-slide-creator-36ny).
		if len(grid.Rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(grid.Rows))
		}
		// Both rows are content-sized, so the rule stops where the text does.
		if grid.Rows[0].MaxHeight <= 0 || grid.Rows[1].MaxHeight <= 0 {
			t.Errorf("rows are not content-sized: %v / %v", grid.Rows[0].MaxHeight, grid.Rows[1].MaxHeight)
		}

		// The accent rule is a column spanning both rows, so it cannot be
		// broken in half by the row gap.
		rule := grid.Rows[0].Cells[0]
		if rule.RowSpan != 2 {
			t.Errorf("accent rule row_span = %d, want 2", rule.RowSpan)
		}
		if rule.Shape == nil || !strings.Contains(string(rule.Shape.Fill), "accent1") {
			t.Errorf("accent rule fill = %v, want accent1", rule.Shape)
		}
		if len(grid.Rows[1].Cells) != 1 {
			t.Errorf("the attribution row lists %d cells; the rule's row-span already occupies its column", len(grid.Rows[1].Cells))
		}

		quoteText := string(grid.Rows[0].Cells[1].Shape.Text)
		attrText := string(grid.Rows[1].Cells[0].Shape.Text)
		if !strings.Contains(quoteText, "predict the future") {
			t.Errorf("quote row does not contain the quote: %s", quoteText)
		}
		if !strings.Contains(attrText, "Alan Kay") {
			t.Errorf("attribution row does not contain the attribution: %s", attrText)
		}
		if !strings.Contains(attrText, "Computer Scientist") {
			t.Errorf("attribution row does not contain the role: %s", attrText)
		}
		if strings.Contains(quoteText, "Alan Kay") {
			t.Error("the attribution is still inside the quote's text body")
		}
		// The gap under the quote is the quote cell's own bottom inset, which
		// is what keeps the attribution off its descenders.
		if !strings.Contains(quoteText, "inset_bottom") {
			t.Errorf("quote cell has no bottom inset: %s", quoteText)
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
		if len(grid.Rows[0].Cells) != 1 {
			t.Fatalf("accent_side=none must draw no rule column, got %d cells", len(grid.Rows[0].Cells))
		}
		if grid.Rows[0].Cells[0].AccentBar != nil {
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
		rule := grid.Rows[0].Cells[0]
		if rule.Shape == nil {
			t.Fatal("expected an accent rule column")
		}
		if !strings.Contains(string(rule.Shape.Fill), "accent4") {
			t.Errorf("accent rule fill = %s, want accent4", rule.Shape.Fill)
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
