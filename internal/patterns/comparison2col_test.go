package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComparison2colRowUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Comparison2colRow
		wantErr string
	}{
		{
			name:  "object_form",
			input: `{"left":"Pro","right":"Con"}`,
			want:  Comparison2colRow{Left: "Pro", Right: "Con"},
		},
		{
			name:  "string_shorthand",
			input: `"Pro | Con"`,
			want:  Comparison2colRow{Left: "Pro", Right: "Con"},
		},
		{
			name:  "string_with_pipe_in_right",
			input: `"Left | Right with | pipes"`,
			want:  Comparison2colRow{Left: "Left", Right: "Right with | pipes"},
		},
		{
			name:    "string_no_pipe",
			input:   `"NoPipe"`,
			wantErr: `must be "Left | Right"`,
		},
		{
			name:    "invalid_json",
			input:   `[1,2]`,
			wantErr: "must be string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got Comparison2colRow
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	// Headers array form
	t.Run("headers_array_form", func(t *testing.T) {
		input := `{"headers":["Before","After"],"rows":[{"left":"A","right":"B"}]}`
		var vals Comparison2colValues
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if vals.HeaderLeft != "Before" {
			t.Errorf("HeaderLeft = %q, want %q", vals.HeaderLeft, "Before")
		}
		if vals.HeaderRight != "After" {
			t.Errorf("HeaderRight = %q, want %q", vals.HeaderRight, "After")
		}
		if vals.Headers != [2]string{"Before", "After"} {
			t.Errorf("Headers = %v, want [Before, After]", vals.Headers)
		}
	})

	t.Run("legacy_header_left_right_still_works", func(t *testing.T) {
		input := `{"header_left":"Pros","header_right":"Cons","rows":[{"left":"A","right":"B"}]}`
		var vals Comparison2colValues
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if vals.HeaderLeft != "Pros" {
			t.Errorf("HeaderLeft = %q, want %q", vals.HeaderLeft, "Pros")
		}
		if vals.HeaderRight != "Cons" {
			t.Errorf("HeaderRight = %q, want %q", vals.HeaderRight, "Cons")
		}
	})

	t.Run("headers_array_takes_precedence_over_legacy", func(t *testing.T) {
		input := `{"headers":["New L","New R"],"header_left":"Old L","header_right":"Old R","rows":[{"left":"A","right":"B"}]}`
		var vals Comparison2colValues
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if vals.HeaderLeft != "New L" {
			t.Errorf("HeaderLeft = %q, want %q (headers should take precedence)", vals.HeaderLeft, "New L")
		}
		if vals.HeaderRight != "New R" {
			t.Errorf("HeaderRight = %q, want %q (headers should take precedence)", vals.HeaderRight, "New R")
		}
	})

	t.Run("headers_array_expand_equivalence", func(t *testing.T) {
		arrayJSON := `{"headers":["Pros","Cons"],"rows":[{"left":"Fast","right":"Expensive"}]}`
		legacyJSON := `{"header_left":"Pros","header_right":"Cons","rows":[{"left":"Fast","right":"Expensive"}]}`

		var arrayVals, legacyVals Comparison2colValues
		if err := json.Unmarshal([]byte(arrayJSON), &arrayVals); err != nil {
			t.Fatalf("unmarshal array form: %v", err)
		}
		if err := json.Unmarshal([]byte(legacyJSON), &legacyVals); err != nil {
			t.Fatalf("unmarshal legacy form: %v", err)
		}

		p := &comparison2col{}
		arrayGrid, err := p.Expand(ExpandContext{}, &arrayVals, nil, nil)
		if err != nil {
			t.Fatalf("expand array: %v", err)
		}
		legacyGrid, err := p.Expand(ExpandContext{}, &legacyVals, nil, nil)
		if err != nil {
			t.Fatalf("expand legacy: %v", err)
		}

		arrayOut, _ := json.Marshal(arrayGrid)
		legacyOut, _ := json.Marshal(legacyGrid)
		if string(arrayOut) != string(legacyOut) {
			t.Errorf("expand outputs differ.\narray:  %s\nlegacy: %s", arrayOut, legacyOut)
		}
	})

	t.Run("marshal_uses_compact_headers_form", func(t *testing.T) {
		vals := Comparison2colValues{
			Headers:     [2]string{"Left", "Right"},
			HeaderLeft:  "Left",
			HeaderRight: "Right",
			Rows:        []Comparison2colRow{{Left: "A", Right: "B"}},
		}
		data, err := json.Marshal(vals)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		s := string(data)
		if !strings.Contains(s, `"headers"`) {
			t.Errorf("expected headers field in output: %s", s)
		}
		if strings.Contains(s, `"header_left"`) {
			t.Errorf("legacy header_left should not appear in output: %s", s)
		}
	})

	// Round-trip equivalence
	t.Run("string_object_expand_equivalence", func(t *testing.T) {
		objJSON := `{"rows":[{"left":"Fast","right":"Expensive"},{"left":"Reliable","right":"Complex"}]}`
		strJSON := `{"rows":["Fast | Expensive","Reliable | Complex"]}`

		var objVals, strVals Comparison2colValues
		if err := json.Unmarshal([]byte(objJSON), &objVals); err != nil {
			t.Fatalf("unmarshal object form: %v", err)
		}
		if err := json.Unmarshal([]byte(strJSON), &strVals); err != nil {
			t.Fatalf("unmarshal string form: %v", err)
		}

		p := &comparison2col{}
		objGrid, err := p.Expand(ExpandContext{}, &objVals, nil, nil)
		if err != nil {
			t.Fatalf("expand object: %v", err)
		}
		strGrid, err := p.Expand(ExpandContext{}, &strVals, nil, nil)
		if err != nil {
			t.Fatalf("expand string: %v", err)
		}

		objOut, _ := json.Marshal(objGrid)
		strOut, _ := json.Marshal(strGrid)
		if string(objOut) != string(strOut) {
			t.Errorf("expand outputs differ.\nobject: %s\nstring: %s", objOut, strOut)
		}
	})
}

func TestComparison2col(t *testing.T) {
	p := &comparison2col{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "comparison-2col" {
			t.Errorf("Name() = %q, want %q", p.Name(), "comparison-2col")
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
		values    Comparison2colValues
		overrides *Comparison2colOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path_with_headers",
			values: Comparison2colValues{
				HeaderLeft:  "Pros",
				HeaderRight: "Cons",
				Rows: []Comparison2colRow{
					{Left: "Fast", Right: "Expensive"},
					{Left: "Reliable", Right: "Complex"},
				},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_no_headers",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: "Before", Right: "After"},
				},
			},
			wantNoErr: true,
		},
		{
			name: "empty_rows",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{},
			},
			wantErr: "at least 1 row",
		},
		{
			name: "too_many_rows",
			values: Comparison2colValues{
				Rows: func() []Comparison2colRow {
					rows := make([]Comparison2colRow, 11)
					for i := range rows {
						rows[i] = Comparison2colRow{Left: "a", Right: "b"}
					}
					return rows
				}(),
			},
			wantErr: "at most 10 rows",
		},
		{
			name: "missing_left",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: "", Right: "something"},
				},
			},
			wantErr: "rows[0].left is required",
		},
		{
			name: "missing_right",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: "something", Right: ""},
				},
			},
			wantErr: "rows[0].right is required",
		},
		{
			name: "left_exceeds_maxlen",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: strings.Repeat("x", 201), Right: "ok"},
				},
			},
			wantErr: "exceeds maxLength 200",
		},
		{
			name: "invalid_cell_override_key",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: "Fast", Right: "Expensive"},
				},
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
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: "Fast", Right: "Expensive"},
				},
			},
			cellOvr: map[int]any{
				99: &Comparison2colCellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name: "accent_override",
			values: Comparison2colValues{
				Rows: []Comparison2colRow{
					{Left: "A", Right: "B"},
				},
			},
			overrides: &Comparison2colOverrides{Accent: "accent3"},
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
	t.Run("expand_with_headers", func(t *testing.T) {
		vals := Comparison2colValues{
			HeaderLeft:  "Pros",
			HeaderRight: "Cons",
			Rows: []Comparison2colRow{
				{Left: "Fast", Right: "Expensive"},
				{Left: "Reliable", Right: "Complex"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		// 1 header row + 2 body rows = 3 rows
		if len(grid.Rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
		}
		// Each row has 2 cells
		for i, row := range grid.Rows {
			if len(row.Cells) != 2 {
				t.Errorf("row[%d] expected 2 cells, got %d", i, len(row.Cells))
			}
		}
		// Header cells should have accent fill
		var headerFill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &headerFill); err != nil {
			t.Fatalf("header fill unmarshal: %v", err)
		}
		if headerFill != "accent1" {
			t.Errorf("header fill = %q, want %q", headerFill, "accent1")
		}
		// Body cells should have lt1 fill
		var bodyFill string
		if err := json.Unmarshal(grid.Rows[1].Cells[0].Shape.Fill, &bodyFill); err != nil {
			t.Fatalf("body fill unmarshal: %v", err)
		}
		if bodyFill != "lt1" {
			t.Errorf("body fill = %q, want %q", bodyFill, "lt1")
		}
	})

	t.Run("expand_without_headers", func(t *testing.T) {
		vals := Comparison2colValues{
			Rows: []Comparison2colRow{
				{Left: "Before", Right: "After"},
			},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// No headers -> just 1 body row
		if len(grid.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(grid.Rows))
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := Comparison2colValues{
			HeaderLeft:  "Left",
			HeaderRight: "Right",
			Rows: []Comparison2colRow{
				{Left: "A", Right: "B"},
			},
		}
		ovr := &Comparison2colOverrides{Accent: "accent5"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "accent5" {
			t.Errorf("header fill = %q, want %q", fill, "accent5")
		}
	})

	t.Run("expand_accent_bar_override", func(t *testing.T) {
		vals := Comparison2colValues{
			Rows: []Comparison2colRow{
				{Left: "A", Right: "B"},
			},
		}
		cellOvr := map[int]any{
			0: &Comparison2colCellOverride{AccentBar: true},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Cell 0 (left) should have accent bar
		ab := grid.Rows[0].Cells[0].AccentBar
		if ab == nil {
			t.Fatal("cell[0] should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}
		// Cell 1 (right) should not
		if grid.Rows[0].Cells[1].AccentBar != nil {
			t.Error("cell[1] should not have accent bar")
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := Comparison2colValues{
			HeaderLeft:  "Pros",
			HeaderRight: "Cons",
			Rows: []Comparison2colRow{
				{Left: "Fast", Right: "Expensive"},
				{Left: "Reliable", Right: "Complex"},
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

		goldenPath := filepath.Join("testdata", "comparison-2col", "default.golden.json")
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
