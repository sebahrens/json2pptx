package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIconRow(t *testing.T) {
	p := &iconRow{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "icon-row" {
			t.Errorf("Name() = %q, want %q", p.Name(), "icon-row")
		}
		if p.UseWhen() != "Icon + caption row" {
			t.Errorf("UseWhen() = %q, want %q", p.UseWhen(), "Icon + caption row")
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
		name      string
		values    IconRowValues
		overrides *IconRowOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path_3_items",
			values: IconRowValues{
				{Icon: "🚀", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_5_items",
			values: IconRowValues{
				{Icon: "🚀", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
				{Icon: "🎯", Caption: "Target"},
				{Icon: "⚡", Caption: "Speed"},
			},
			wantNoErr: true,
		},
		{
			name: "too_few_items_hints_kpi3up",
			values: IconRowValues{
				{Icon: "🚀", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
			},
			wantErr: "kpi-3up",
		},
		{
			name: "too_many_items",
			values: IconRowValues{
				{Icon: "1", Caption: "a"},
				{Icon: "2", Caption: "b"},
				{Icon: "3", Caption: "c"},
				{Icon: "4", Caption: "d"},
				{Icon: "5", Caption: "e"},
				{Icon: "6", Caption: "f"},
			},
			wantErr: "at most 5 items",
		},
		{
			name: "missing_icon",
			values: IconRowValues{
				{Icon: "", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
			},
			wantErr: "values[0].icon is required",
		},
		{
			name: "missing_caption",
			values: IconRowValues{
				{Icon: "🚀", Caption: ""},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
			},
			wantErr: "values[0].caption is required",
		},
		{
			name: "icon_exceeds_maxlen",
			values: IconRowValues{
				{Icon: "this-icon-name-is-way-too-long", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
			},
			wantErr: "exceeds maxLength 20",
		},
		{
			name: "invalid_cell_override_key",
			values: IconRowValues{
				{Icon: "🚀", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
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
			values: IconRowValues{
				{Icon: "🚀", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
			},
			cellOvr: map[int]any{
				5: &IconRowCellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name: "accent_override",
			values: IconRowValues{
				{Icon: "🚀", Caption: "Launch"},
				{Icon: "📈", Caption: "Growth"},
				{Icon: "💰", Caption: "Revenue"},
			},
			overrides: &IconRowOverrides{Accent: "accent3"},
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
	t.Run("expand_default_3_items", func(t *testing.T) {
		vals := IconRowValues{
			{Icon: "🚀", Caption: "Launch"},
			{Icon: "📈", Caption: "Growth"},
			{Icon: "💰", Caption: "Revenue"},
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
		// Check columns matches item count
		var cols int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 3 {
			t.Errorf("columns = %d, want 3", cols)
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

	t.Run("expand_5_items_dynamic_columns", func(t *testing.T) {
		vals := IconRowValues{
			{Icon: "🚀", Caption: "Launch"},
			{Icon: "📈", Caption: "Growth"},
			{Icon: "💰", Caption: "Revenue"},
			{Icon: "🎯", Caption: "Target"},
			{Icon: "⚡", Caption: "Speed"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if len(grid.Rows[0].Cells) != 5 {
			t.Fatalf("expected 5 cells, got %d", len(grid.Rows[0].Cells))
		}
		var cols int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 5 {
			t.Errorf("columns = %d, want 5", cols)
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := IconRowValues{
			{Icon: "🚀", Caption: "Launch"},
			{Icon: "📈", Caption: "Growth"},
			{Icon: "💰", Caption: "Revenue"},
		}
		ovr := &IconRowOverrides{Accent: "accent3"}
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
		vals := IconRowValues{
			{Icon: "🚀", Caption: "Launch"},
			{Icon: "📈", Caption: "Growth"},
			{Icon: "💰", Caption: "Revenue"},
		}
		cellOvr := map[int]any{
			1: &IconRowCellOverride{AccentBar: true},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid.Rows[0].Cells[0].AccentBar != nil {
			t.Error("cell[0] should not have accent bar")
		}
		ab := grid.Rows[0].Cells[1].AccentBar
		if ab == nil {
			t.Fatal("cell[1] should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := IconRowValues{
			{Icon: "🚀", Caption: "Launch"},
			{Icon: "📈", Caption: "Growth"},
			{Icon: "💰", Caption: "Revenue"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "icon-row", "default.golden.json")
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
