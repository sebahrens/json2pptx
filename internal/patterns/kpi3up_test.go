package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKpi3up(t *testing.T) {
	p := &kpi3up{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "kpi-3up" {
			t.Errorf("Name() = %q, want %q", p.Name(), "kpi-3up")
		}
		if p.UseWhen() == "" {
			t.Error("UseWhen() must not be empty (D6)")
		}
		if p.Version() != 1 {
			t.Errorf("Version() = %d, want 1", p.Version())
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

	tests := []struct {
		name       string
		values     Kpi3upValues
		overrides  *Kpi3upOverrides
		cellOvr    map[int]any
		wantErr    string
		wantNoErr  bool
	}{
		{
			name: "happy_path",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			wantNoErr: true,
		},
		{
			name: "wrong_count_4_hints_kpi4up",
			values: Kpi3upValues{
				{Big: "1", Small: "a"},
				{Big: "2", Small: "b"},
				{Big: "3", Small: "c"},
				{Big: "4", Small: "d"},
			},
			wantErr: "kpi-4up",
		},
		{
			name: "wrong_count_2",
			values: Kpi3upValues{
				{Big: "1", Small: "a"},
				{Big: "2", Small: "b"},
			},
			wantErr: "exactly 3 cells",
		},
		{
			name: "missing_big",
			values: Kpi3upValues{
				{Big: "", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			wantErr: "values[0].big is required",
		},
		{
			name: "big_exceeds_maxlen",
			values: Kpi3upValues{
				{Big: "123456789", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			wantErr: "exceeds maxLength 8",
		},
		{
			name: "missing_small",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: ""},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			wantErr: "values[0].small is required",
		},
		{
			name: "invalid_cell_override_key",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			cellOvr: map[int]any{
				0: &struct {
					BadKey string `json:"bad_key"`
				}{BadKey: "nope"},
			},
			wantErr: `unknown key "bad_key"`,
		},
		{
			name: "accent_override_changes_color",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			overrides: &Kpi3upOverrides{Accent: "accent3"},
			wantNoErr: true,
		},
		{
			name: "font_size_cell_override",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			cellOvr: map[int]any{
				1: &Kpi3upCellOverride{FontSize: 48},
			},
			wantNoErr: true,
		},
		{
			name: "accent_bar_cell_override",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			cellOvr: map[int]any{
				1: &Kpi3upCellOverride{AccentBar: true},
			},
			wantNoErr: true,
		},
		{
			name: "accent_bar_and_emphasis_combined",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			cellOvr: map[int]any{
				1: &Kpi3upCellOverride{AccentBar: true, Emphasis: "bold"},
			},
			wantNoErr: true,
		},
		{
			name: "out_of_range_cell_override",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			cellOvr: map[int]any{
				5: &Kpi3upCellOverride{AccentBar: true},
			},
			wantErr: "cell_overrides key 5 out of range [0,2]",
		},
		{
			name: "unknown_key_cites_d15_whitelist",
			values: Kpi3upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
			},
			cellOvr: map[int]any{
				0: &struct {
					Animation string `json:"animation"`
				}{Animation: "fade"},
			},
			wantErr: "allowed keys per D15",
		},
	}

	for _, tc := range tests {
		t.Run("validate_"+tc.name, func(t *testing.T) {
			var ovr any
			if tc.overrides != nil {
				ovr = tc.overrides
			}
			err := p.Validate(&tc.values, ovr, tc.cellOvr)
			if tc.wantNoErr {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
			}
		})
	}

	// Expand tests
	t.Run("expand_default_accent", func(t *testing.T) {
		vals := Kpi3upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(grid.Rows))
		}
		if len(grid.Rows[0].Cells) != 3 {
			t.Fatalf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
		}
		// Check default fill is accent1
		for i, cell := range grid.Rows[0].Cells {
			if cell.Shape == nil {
				t.Fatalf("cell[%d].Shape is nil", i)
			}
			var fill string
			if err := json.Unmarshal(cell.Shape.Fill, &fill); err != nil {
				t.Fatalf("cell[%d] fill unmarshal: %v", i, err)
			}
			if fill != "accent1" {
				t.Errorf("cell[%d] fill = %q, want %q", i, fill, "accent1")
			}
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := Kpi3upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
		}
		ovr := &Kpi3upOverrides{Accent: "accent3"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		for i, cell := range grid.Rows[0].Cells {
			var fill string
			if err := json.Unmarshal(cell.Shape.Fill, &fill); err != nil {
				t.Fatalf("cell[%d] fill unmarshal: %v", i, err)
			}
			if fill != "accent3" {
				t.Errorf("cell[%d] fill = %q, want %q", i, fill, "accent3")
			}
		}
	})

	t.Run("expand_accent_bar_override", func(t *testing.T) {
		vals := Kpi3upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
		}
		cellOvr := map[int]any{
			1: &Kpi3upCellOverride{AccentBar: true},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Cell 0 should have no accent bar
		if grid.Rows[0].Cells[0].AccentBar != nil {
			t.Error("cell[0] should not have accent bar")
		}
		// Cell 1 should have accent bar
		ab := grid.Rows[0].Cells[1].AccentBar
		if ab == nil {
			t.Fatal("cell[1] should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}
	})

	// Acceptance-criteria test: accent_bar=true + emphasis=bold on cell 1
	t.Run("expand_accent_bar_and_emphasis_bold", func(t *testing.T) {
		vals := Kpi3upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
		}
		cellOvr := map[int]any{
			1: &Kpi3upCellOverride{AccentBar: true, Emphasis: "bold"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		// Cell 1 should have accent bar
		ab := grid.Rows[0].Cells[1].AccentBar
		if ab == nil {
			t.Fatal("cell[1] should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}

		// Cell 1 text should have bold on all paragraphs
		cell1Text := grid.Rows[0].Cells[1].Shape.Text
		var textObj struct {
			Paragraphs []map[string]any `json:"paragraphs"`
		}
		if err := json.Unmarshal(cell1Text, &textObj); err != nil {
			t.Fatalf("unmarshal cell[1] text: %v", err)
		}
		for i, para := range textObj.Paragraphs {
			bold, ok := para["bold"]
			if !ok || bold != true {
				t.Errorf("cell[1].paragraphs[%d].bold = %v, want true", i, bold)
			}
		}

		// Cell 0 should NOT have emphasis override (paragraphs retain defaults)
		cell0Text := grid.Rows[0].Cells[0].Shape.Text
		var cell0TextObj struct {
			Paragraphs []map[string]any `json:"paragraphs"`
		}
		if err := json.Unmarshal(cell0Text, &cell0TextObj); err != nil {
			t.Fatalf("unmarshal cell[0] text: %v", err)
		}
		// The big paragraph has bold:true by default, but the small one should NOT
		if len(cell0TextObj.Paragraphs) >= 2 {
			if _, hasItalic := cell0TextObj.Paragraphs[1]["italic"]; hasItalic {
				t.Error("cell[0] small paragraph should not have italic set")
			}
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := Kpi3upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "kpi3up", "default.golden.json")
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
