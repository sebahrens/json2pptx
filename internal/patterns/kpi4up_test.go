package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKpi4up(t *testing.T) {
	p := &kpi4up{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "kpi-4up" {
			t.Errorf("Name() = %q, want %q", p.Name(), "kpi-4up")
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
		// Verify values array requires exactly 4 items
		props, ok := m["properties"].(map[string]any)
		if !ok {
			t.Fatal("missing properties")
		}
		valuesSchema, ok := props["values"].(map[string]any)
		if !ok {
			t.Fatal("missing values property")
		}
		if valuesSchema["minItems"] != float64(4) {
			t.Errorf("values minItems = %v, want 4", valuesSchema["minItems"])
		}
		if valuesSchema["maxItems"] != float64(4) {
			t.Errorf("values maxItems = %v, want 4", valuesSchema["maxItems"])
		}
	})

	tests := []struct {
		name      string
		values    Kpi4upValues
		overrides *Kpi4upOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			wantNoErr: true,
		},
		{
			name: "wrong_count_3_hints_kpi3up",
			values: Kpi4upValues{
				{Big: "1", Small: "a"},
				{Big: "2", Small: "b"},
				{Big: "3", Small: "c"},
			},
			wantErr: "kpi-3up",
		},
		{
			name: "wrong_count_5",
			values: Kpi4upValues{
				{Big: "1", Small: "a"},
				{Big: "2", Small: "b"},
				{Big: "3", Small: "c"},
				{Big: "4", Small: "d"},
				{Big: "5", Small: "e"},
			},
			wantErr: "exactly 4 cells",
		},
		{
			name: "missing_big",
			values: Kpi4upValues{
				{Big: "", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			wantErr: "values[0].big is required",
		},
		{
			name: "big_exceeds_maxlen",
			values: Kpi4upValues{
				{Big: "123456789", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			wantErr: "exceeds maxLength 8",
		},
		{
			name: "missing_small",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: ""},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			wantErr: "values[0].small is required",
		},
		{
			name: "invalid_cell_override_key",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			cellOvr: map[int]any{
				0: &struct {
					BadKey string `json:"bad_key"`
				}{BadKey: "nope"},
			},
			wantErr: `unknown key "bad_key"`,
		},
		{
			name: "cell_override_out_of_range",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			cellOvr: map[int]any{
				4: &KPICellOverride{AccentBar: true},
			},
			wantErr: "out of range [0,3]",
		},
		{
			name: "accent_override_changes_color",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			overrides: &Kpi4upOverrides{Accent: "accent3"},
			wantNoErr: true,
		},
		{
			name: "font_size_cell_override",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			cellOvr: map[int]any{
				2: &Kpi4upCellOverride{FontSize: 48},
			},
			wantNoErr: true,
		},
		{
			name: "accent_bar_cell_override",
			values: Kpi4upValues{
				{Big: "$4.2M", Small: "ARR"},
				{Big: "127%", Small: "NRR"},
				{Big: "12d", Small: "Sales cycle"},
				{Big: "98%", Small: "CSAT"},
			},
			cellOvr: map[int]any{
				1: &Kpi4upCellOverride{AccentBar: true},
			},
			wantNoErr: true,
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
		vals := Kpi4upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
			{Big: "98%", Small: "CSAT"},
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
		if len(grid.Rows[0].Cells) != 4 {
			t.Fatalf("expected 4 cells, got %d", len(grid.Rows[0].Cells))
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
		// Check columns = 4
		var cols float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 4 {
			t.Errorf("columns = %v, want 4", cols)
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := Kpi4upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
			{Big: "98%", Small: "CSAT"},
		}
		ovr := &Kpi4upOverrides{Accent: "accent3"}
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
		vals := Kpi4upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
			{Big: "98%", Small: "CSAT"},
		}
		cellOvr := map[int]any{
			2: &Kpi4upCellOverride{AccentBar: true},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Cell 0 should have no accent bar
		if grid.Rows[0].Cells[0].AccentBar != nil {
			t.Error("cell[0] should not have accent bar")
		}
		// Cell 2 should have accent bar
		ab := grid.Rows[0].Cells[2].AccentBar
		if ab == nil {
			t.Fatal("cell[2] should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := Kpi4upValues{
			{Big: "$4.2M", Small: "ARR"},
			{Big: "127%", Small: "NRR"},
			{Big: "12d", Small: "Sales cycle"},
			{Big: "98%", Small: "CSAT"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "kpi4up", "default.golden.json")
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
