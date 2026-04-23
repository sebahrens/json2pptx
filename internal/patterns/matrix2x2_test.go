package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatrix2x2(t *testing.T) {
	p := &matrix2x2{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "matrix-2x2" {
			t.Errorf("Name() = %q, want %q", p.Name(), "matrix-2x2")
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
		values    Matrix2x2Values
		overrides *Matrix2x2Overrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path",
			values: Matrix2x2Values{
				XAxisLabel: "Market Share",
				YAxisLabel: "Market Growth",
				TopLeft:    Matrix2x2Quadrant{Header: "Stars", Body: "High growth, high share"},
				TopRight:   Matrix2x2Quadrant{Header: "Question Marks", Body: "High growth, low share"},
				BottomLeft: Matrix2x2Quadrant{Header: "Cash Cows", Body: "Low growth, high share"},
				BottomRight: Matrix2x2Quadrant{Header: "Dogs", Body: "Low growth, low share"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_no_body",
			values: Matrix2x2Values{
				XAxisLabel: "Urgency",
				YAxisLabel: "Importance",
				TopLeft:    Matrix2x2Quadrant{Header: "Do First"},
				TopRight:   Matrix2x2Quadrant{Header: "Schedule"},
				BottomLeft: Matrix2x2Quadrant{Header: "Delegate"},
				BottomRight: Matrix2x2Quadrant{Header: "Eliminate"},
			},
			wantNoErr: true,
		},
		{
			name: "missing_x_axis_label",
			values: Matrix2x2Values{
				YAxisLabel: "Growth",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			wantErr: "x_axis_label is required",
		},
		{
			name: "missing_y_axis_label",
			values: Matrix2x2Values{
				XAxisLabel: "Share",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			wantErr: "y_axis_label is required",
		},
		{
			name: "missing_quadrant_header",
			values: Matrix2x2Values{
				XAxisLabel: "X",
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: ""},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			wantErr: "top_right.header is required",
		},
		{
			name: "header_exceeds_maxlen",
			values: Matrix2x2Values{
				XAxisLabel: "X",
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: strings.Repeat("x", 81)},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			wantErr: "top_left.header exceeds maxLength 80",
		},
		{
			name: "body_exceeds_maxlen",
			values: Matrix2x2Values{
				XAxisLabel: "X",
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: "A", Body: strings.Repeat("x", 201)},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			wantErr: "top_left.body exceeds maxLength 200",
		},
		{
			name: "x_axis_label_exceeds_maxlen",
			values: Matrix2x2Values{
				XAxisLabel: strings.Repeat("x", 61),
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			wantErr: "x_axis_label exceeds maxLength 60",
		},
		{
			name: "cell_override_out_of_range",
			values: Matrix2x2Values{
				XAxisLabel: "X",
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			cellOvr: map[int]any{
				5: &Matrix2x2CellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name: "invalid_cell_override_key",
			values: Matrix2x2Values{
				XAxisLabel: "X",
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			cellOvr: map[int]any{
				0: &struct {
					BadKey string `json:"bad_key"`
				}{BadKey: "nope"},
			},
			wantErr: `unknown key "bad_key"`,
		},
		{
			name: "accent_override",
			values: Matrix2x2Values{
				XAxisLabel: "X",
				YAxisLabel: "Y",
				TopLeft:    Matrix2x2Quadrant{Header: "A"},
				TopRight:   Matrix2x2Quadrant{Header: "B"},
				BottomLeft: Matrix2x2Quadrant{Header: "C"},
				BottomRight: Matrix2x2Quadrant{Header: "D"},
			},
			overrides: &Matrix2x2Overrides{TextOverrides: TextOverrides{Accent: "accent3"}},
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
	t.Run("expand_happy_path", func(t *testing.T) {
		vals := Matrix2x2Values{
			XAxisLabel: "Market Share",
			YAxisLabel: "Market Growth",
			TopLeft:    Matrix2x2Quadrant{Header: "Stars", Body: "High growth, high share"},
			TopRight:   Matrix2x2Quadrant{Header: "Question Marks", Body: "High growth, low share"},
			BottomLeft: Matrix2x2Quadrant{Header: "Cash Cows", Body: "Low growth, high share"},
			BottomRight: Matrix2x2Quadrant{Header: "Dogs", Body: "Low growth, low share"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		// 3 rows: header + 2 body rows
		if len(grid.Rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
		}
		// Row 0: corner + x-axis label (2 cells)
		if len(grid.Rows[0].Cells) != 2 {
			t.Errorf("row[0] expected 2 cells, got %d", len(grid.Rows[0].Cells))
		}
		// Row 1: y-axis + TL + TR (3 cells)
		if len(grid.Rows[1].Cells) != 3 {
			t.Errorf("row[1] expected 3 cells, got %d", len(grid.Rows[1].Cells))
		}
		// Row 2: BL + BR (2 cells, y-axis spans from row 1)
		if len(grid.Rows[2].Cells) != 2 {
			t.Errorf("row[2] expected 2 cells, got %d", len(grid.Rows[2].Cells))
		}

		// X-axis label should have accent fill
		var xFill string
		if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Fill, &xFill); err != nil {
			t.Fatalf("x-axis fill unmarshal: %v", err)
		}
		if xFill != "accent1" {
			t.Errorf("x-axis fill = %q, want %q", xFill, "accent1")
		}

		// Y-axis label should have rotation=270
		if grid.Rows[1].Cells[0].Shape.Rotation != 270 {
			t.Errorf("y-axis rotation = %v, want 270", grid.Rows[1].Cells[0].Shape.Rotation)
		}

		// Y-axis label should have row_span=2
		if grid.Rows[1].Cells[0].RowSpan != 2 {
			t.Errorf("y-axis row_span = %d, want 2", grid.Rows[1].Cells[0].RowSpan)
		}

		// Quadrant cells should have lt1 fill
		var qFill string
		if err := json.Unmarshal(grid.Rows[1].Cells[1].Shape.Fill, &qFill); err != nil {
			t.Fatalf("quadrant fill unmarshal: %v", err)
		}
		if qFill != "lt1" {
			t.Errorf("quadrant fill = %q, want %q", qFill, "lt1")
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := Matrix2x2Values{
			XAxisLabel: "X",
			YAxisLabel: "Y",
			TopLeft:    Matrix2x2Quadrant{Header: "A"},
			TopRight:   Matrix2x2Quadrant{Header: "B"},
			BottomLeft: Matrix2x2Quadrant{Header: "C"},
			BottomRight: Matrix2x2Quadrant{Header: "D"},
		}
		ovr := &Matrix2x2Overrides{TextOverrides: TextOverrides{Accent: "accent5"}}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// X-axis label should use the overridden accent
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "accent5" {
			t.Errorf("x-axis fill = %q, want %q", fill, "accent5")
		}
	})

	t.Run("expand_accent_bar_override", func(t *testing.T) {
		vals := Matrix2x2Values{
			XAxisLabel: "X",
			YAxisLabel: "Y",
			TopLeft:    Matrix2x2Quadrant{Header: "A"},
			TopRight:   Matrix2x2Quadrant{Header: "B"},
			BottomLeft: Matrix2x2Quadrant{Header: "C"},
			BottomRight: Matrix2x2Quadrant{Header: "D"},
		}
		cellOvr := map[int]any{
			0: &Matrix2x2CellOverride{AccentBar: true},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Cell 0 (top_left) should have accent bar — it's in row[1].Cells[1]
		ab := grid.Rows[1].Cells[1].AccentBar
		if ab == nil {
			t.Fatal("cell[0] (top_left) should have accent bar")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", ab.Color, "accent1")
		}
		// Cell 1 (top_right) should not
		if grid.Rows[1].Cells[2].AccentBar != nil {
			t.Error("cell[1] (top_right) should not have accent bar")
		}
	})

	t.Run("expand_no_body", func(t *testing.T) {
		vals := Matrix2x2Values{
			XAxisLabel: "X",
			YAxisLabel: "Y",
			TopLeft:    Matrix2x2Quadrant{Header: "A"},
			TopRight:   Matrix2x2Quadrant{Header: "B"},
			BottomLeft: Matrix2x2Quadrant{Header: "C"},
			BottomRight: Matrix2x2Quadrant{Header: "D"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Should still produce a valid grid
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
		}
	})

	// Quadrants array unmarshal tests
	t.Run("unmarshal_quadrants_array", func(t *testing.T) {
		input := `{
			"x_axis_label": "X",
			"y_axis_label": "Y",
			"quadrants": [
				{"header": "TL", "body": "top-left"},
				{"header": "TR", "body": "top-right"},
				{"header": "BL", "body": "bottom-left"},
				{"header": "BR", "body": "bottom-right"}
			]
		}`
		var vals Matrix2x2Values
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if vals.TopLeft.Header != "TL" || vals.TopLeft.Body != "top-left" {
			t.Errorf("top_left = %+v, want {TL, top-left}", vals.TopLeft)
		}
		if vals.TopRight.Header != "TR" {
			t.Errorf("top_right.header = %q, want %q", vals.TopRight.Header, "TR")
		}
		if vals.BottomLeft.Header != "BL" {
			t.Errorf("bottom_left.header = %q, want %q", vals.BottomLeft.Header, "BL")
		}
		if vals.BottomRight.Header != "BR" {
			t.Errorf("bottom_right.header = %q, want %q", vals.BottomRight.Header, "BR")
		}
	})

	t.Run("unmarshal_quadrants_string_shorthand", func(t *testing.T) {
		input := `{
			"x_axis_label": "X",
			"y_axis_label": "Y",
			"quadrants": ["Stars | High growth", "Questions | Low share", "Cash Cows", "Dogs | Low everything"]
		}`
		var vals Matrix2x2Values
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if vals.TopLeft.Header != "Stars" || vals.TopLeft.Body != "High growth" {
			t.Errorf("top_left = %+v, want {Stars, High growth}", vals.TopLeft)
		}
		if vals.BottomLeft.Header != "Cash Cows" || vals.BottomLeft.Body != "" {
			t.Errorf("bottom_left = %+v, want {Cash Cows, ''}", vals.BottomLeft)
		}
	})

	t.Run("unmarshal_named_form_still_works", func(t *testing.T) {
		input := `{
			"x_axis_label": "X",
			"y_axis_label": "Y",
			"top_left": {"header": "A"},
			"top_right": {"header": "B"},
			"bottom_left": {"header": "C"},
			"bottom_right": {"header": "D"}
		}`
		var vals Matrix2x2Values
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if vals.TopLeft.Header != "A" {
			t.Errorf("top_left.header = %q, want %q", vals.TopLeft.Header, "A")
		}
		if vals.BottomRight.Header != "D" {
			t.Errorf("bottom_right.header = %q, want %q", vals.BottomRight.Header, "D")
		}
	})

	t.Run("validate_quadrants_array_via_unmarshal", func(t *testing.T) {
		input := `{
			"x_axis_label": "X",
			"y_axis_label": "Y",
			"quadrants": ["A", "B", "C", "D"]
		}`
		var vals Matrix2x2Values
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if err := p.Validate(&vals, nil, nil); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
	})

	t.Run("expand_quadrants_array", func(t *testing.T) {
		input := `{
			"x_axis_label": "Market Share",
			"y_axis_label": "Growth",
			"quadrants": [
				"Stars | High growth",
				"Questions | Low share",
				"Cash Cows | Stable",
				"Dogs | Decline"
			]
		}`
		var vals Matrix2x2Values
		if err := json.Unmarshal([]byte(input), &vals); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := Matrix2x2Values{
			XAxisLabel: "Market Share",
			YAxisLabel: "Market Growth",
			TopLeft:    Matrix2x2Quadrant{Header: "Stars", Body: "High growth, high share"},
			TopRight:   Matrix2x2Quadrant{Header: "Question Marks", Body: "High growth, low share"},
			BottomLeft: Matrix2x2Quadrant{Header: "Cash Cows", Body: "Low growth, high share"},
			BottomRight: Matrix2x2Quadrant{Header: "Dogs", Body: "Low growth, low share"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "matrix-2x2", "default.golden.json")
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
