package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCardGrid(t *testing.T) {
	p := &cardGrid{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "card-grid" {
			t.Errorf("Name() = %q, want %q", p.Name(), "card-grid")
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
		name      string
		values    CardGridValues
		overrides *CardGridOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path_2x3",
			values: CardGridValues{
				Columns: 3,
				Rows:    2,
				Cells: []CardGridCell{
					{Header: "Card 1", Body: "Description 1"},
					{Header: "Card 2", Body: "Description 2"},
					{Header: "Card 3", Body: "Description 3"},
					{Header: "Card 4", Body: "Description 4"},
					{Header: "Card 5", Body: "Description 5"},
					{Header: "Card 6", Body: "Description 6"},
				},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_1x1",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "Solo", Body: "Content"}},
			},
			wantNoErr: true,
		},
		{
			name: "cell_count_mismatch",
			values: CardGridValues{
				Columns: 2,
				Rows:    2,
				Cells: []CardGridCell{
					{Header: "A", Body: "B"},
					{Header: "C", Body: "D"},
					{Header: "E", Body: "F"},
				},
			},
			wantErr: "cells must contain exactly 4",
		},
		{
			name: "columns_too_high",
			values: CardGridValues{
				Columns: 6,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "A", Body: "B"}},
			},
			wantErr: "columns must be 1–5",
		},
		{
			name: "rows_zero",
			values: CardGridValues{
				Columns: 2,
				Rows:    0,
				Cells:   []CardGridCell{},
			},
			wantErr: "rows must be 1–5",
		},
		{
			name: "missing_header",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "", Body: "content"}},
			},
			wantErr: "cells[0].header is required",
		},
		{
			name: "missing_body",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "title", Body: ""}},
			},
			wantErr: "cells[0].body is required",
		},
		{
			name: "header_exceeds_maxlen",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: strings.Repeat("x", 81), Body: "ok"}},
			},
			wantErr: "exceeds maxLength 80",
		},
		{
			name: "body_exceeds_maxlen",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "ok", Body: strings.Repeat("x", 301)}},
			},
			wantErr: "exceeds maxLength 300",
		},
		{
			name: "bmc_sibling_hint",
			values: CardGridValues{
				Columns: 3,
				Rows:    3,
				Cells: func() []CardGridCell {
					// 9 cells but wrong count for the actual grid dimensions
					cells := make([]CardGridCell, 9)
					for i := range cells {
						cells[i] = CardGridCell{Header: "h", Body: "b"}
					}
					return cells
				}(),
			},
			wantNoErr: true, // 3x3 = 9 cells, matches
		},
		{
			name: "bmc_sibling_hint_mismatch",
			values: CardGridValues{
				Columns: 2,
				Rows:    2,
				Cells: func() []CardGridCell {
					cells := make([]CardGridCell, 9)
					for i := range cells {
						cells[i] = CardGridCell{Header: "h", Body: "b"}
					}
					return cells
				}(),
			},
			wantErr: "bmc-canvas",
		},
		{
			name: "invalid_cell_override_key",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "A", Body: "B"}},
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
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "A", Body: "B"}},
			},
			cellOvr: map[int]any{
				99: &CardGridCellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name: "accent_override",
			values: CardGridValues{
				Columns: 1,
				Rows:    1,
				Cells:   []CardGridCell{{Header: "A", Body: "B"}},
			},
			overrides: &CardGridOverrides{Accent: "accent3"},
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
	t.Run("expand_2x3", func(t *testing.T) {
		vals := CardGridValues{
			Columns: 3,
			Rows:    2,
			Cells: []CardGridCell{
				{Header: "Card 1", Body: "Desc 1"},
				{Header: "Card 2", Body: "Desc 2"},
				{Header: "Card 3", Body: "Desc 3"},
				{Header: "Card 4", Body: "Desc 4"},
				{Header: "Card 5", Body: "Desc 5"},
				{Header: "Card 6", Body: "Desc 6"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		// 2 rows
		if len(grid.Rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(grid.Rows))
		}
		// 3 cells per row
		for i, row := range grid.Rows {
			if len(row.Cells) != 3 {
				t.Errorf("row[%d] expected 3 cells, got %d", i, len(row.Cells))
			}
		}
		// Columns should be 3
		var cols int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 3 {
			t.Errorf("columns = %d, want 3", cols)
		}
		// Fill should be accent1
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "accent1" {
			t.Errorf("fill = %q, want %q", fill, "accent1")
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := CardGridValues{
			Columns: 1,
			Rows:    1,
			Cells:   []CardGridCell{{Header: "A", Body: "B"}},
		}
		ovr := &CardGridOverrides{Accent: "accent5"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "accent5" {
			t.Errorf("fill = %q, want %q", fill, "accent5")
		}
	})

	t.Run("expand_accent_bar_override", func(t *testing.T) {
		vals := CardGridValues{
			Columns: 2,
			Rows:    1,
			Cells: []CardGridCell{
				{Header: "A", Body: "B"},
				{Header: "C", Body: "D"},
			},
		}
		cellOvr := map[int]any{
			0: &CardGridCellOverride{AccentBar: true},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Cell 0 should have accent bar
		ab := grid.Rows[0].Cells[0].AccentBar
		if ab == nil {
			t.Fatal("cell[0] should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}
		if ab.Position != "top" {
			t.Errorf("accent bar position = %q, want %q", ab.Position, "top")
		}
		// Cell 1 should not
		if grid.Rows[0].Cells[1].AccentBar != nil {
			t.Error("cell[1] should not have accent bar")
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := CardGridValues{
			Columns: 3,
			Rows:    2,
			Cells: []CardGridCell{
				{Header: "Card 1", Body: "Description 1"},
				{Header: "Card 2", Body: "Description 2"},
				{Header: "Card 3", Body: "Description 3"},
				{Header: "Card 4", Body: "Description 4"},
				{Header: "Card 5", Body: "Description 5"},
				{Header: "Card 6", Body: "Description 6"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "card-grid", "default.golden.json")
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
