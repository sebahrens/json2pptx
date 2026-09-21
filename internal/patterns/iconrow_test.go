package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIconRowItemUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    IconRowItem
		wantErr string
	}{
		{
			name:  "object_form",
			input: `{"icon":"🚀","caption":"Launch"}`,
			want:  IconRowItem{Icon: &IconRef{Name: "🚀"}, Caption: "Launch"},
		},
		{
			name:  "string_with_icon",
			input: `"🚀 | Launch"`,
			want:  IconRowItem{Icon: &IconRef{Name: "🚀"}, Caption: "Launch"},
		},
		{
			name:  "string_caption_only",
			input: `"Launch"`,
			want:  IconRowItem{Icon: nil, Caption: "Launch"},
		},
		{
			name:  "string_with_pipe_in_caption",
			input: `"rocket | A | B"`,
			want:  IconRowItem{Icon: &IconRef{Name: "rocket"}, Caption: "A | B"},
		},
		{
			name:    "invalid_json",
			input:   `[1,2]`,
			wantErr: "must be string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got IconRowItem
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
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	// Round-trip equivalence
	t.Run("string_object_expand_equivalence", func(t *testing.T) {
		objJSON := `[{"icon":"🚀","caption":"Launch"},{"icon":"📈","caption":"Growth"},{"icon":"💰","caption":"Revenue"}]`
		strJSON := `["🚀 | Launch","📈 | Growth","💰 | Revenue"]`
		assertExpandEquivalent(t, objJSON, strJSON, func(raw string) (any, error) {
			var values IconRowValues
			if err := json.Unmarshal([]byte(raw), &values); err != nil {
				return nil, err
			}
			return (&iconRow{}).Expand(ExpandContext{}, &values, nil, nil)
		})
	})
}

func TestIconRow(t *testing.T) {
	p := &iconRow{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "icon-row" {
			t.Errorf("Name() = %q, want %q", p.Name(), "icon-row")
		}
		if !strings.Contains(p.UseWhen(), "prefer") {
			t.Errorf("UseWhen() lacks contrastive language: %q", p.UseWhen())
		}
		if p.Version() != 2 {
			t.Errorf("Version() = %d, want 2", p.Version())
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

	// inlineSVG and remoteURL are valid non-bundled icon forms accepted by Validate.
	const inlineSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	const remoteURL = "https://example.com/icons/rocket.svg"

	tests := []struct {
		name      string
		values    IconRowValues
		overrides *IconRowOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path_3_items_bundled",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_5_items_bundled",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
				{Icon: &IconRef{Name: "target"}, Caption: "Target"},
				{Icon: &IconRef{Name: "bolt"}, Caption: "Speed"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_inline_svg",
			values: IconRowValues{
				{Icon: &IconRef{SVGData: inlineSVG}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_url",
			values: IconRowValues{
				{Icon: &IconRef{URL: remoteURL}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantNoErr: true,
		},
		{
			name: "too_few_items_hints_kpi3up",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
			},
			wantErr: "kpi-3up",
		},
		{
			name: "too_many_items",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: "a"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "b"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "c"},
				{Icon: &IconRef{Name: "target"}, Caption: "d"},
				{Icon: &IconRef{Name: "bolt"}, Caption: "e"},
				{Icon: &IconRef{Name: "rocket"}, Caption: "f"},
			},
			wantErr: "at most 5 items",
		},
		{
			name: "missing_icon",
			values: IconRowValues{
				{Icon: nil, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantErr: "values[0].icon is required",
		},
		{
			name: "missing_caption",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: ""},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantErr: "values[0].caption is required",
		},
		{
			name: "emoji_icon_rejected",
			values: IconRowValues{
				{Icon: &IconRef{Name: "🚀"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantErr: "must be a bundled icon name",
		},
		{
			name: "unknown_bundled_name_rejected",
			values: IconRowValues{
				{Icon: &IconRef{Name: "not-a-real-icon-name"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			wantErr: "must be a bundled icon name",
		},
		{
			name: "invalid_cell_override_key",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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
				{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			},
			cellOvr: map[int]any{
				5: &IconRowCellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name: "accent_override",
			values: IconRowValues{
				{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
				{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
				{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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
			{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
			{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
			{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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
			{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
			{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
			{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
			{Icon: &IconRef{Name: "target"}, Caption: "Target"},
			{Icon: &IconRef{Name: "bolt"}, Caption: "Speed"},
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
			{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
			{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
			{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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
			{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
			{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
			{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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
			{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
			{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
			{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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

// TestIconRow_ContentSizedRow pins the fix for go-slide-creator-tee7: the row
// used to carry no max_height and no vertical_align, so its cards stretched to
// the whole content zone with the icon and a five-word caption floating in a
// box three times taller than its content.
func TestIconRow_ContentSizedRow(t *testing.T) {
	p, _ := Default().Get("icon-row")
	ctx := testThemeCtx()

	for _, n := range []int{3, 4, 5} {
		items := make(IconRowValues, n)
		for i := range items {
			items[i] = IconRowItem{Icon: &IconRef{Name: "rocket"}, Caption: "Launch the platform"}
		}
		grid, err := p.Expand(ctx, &items, nil, nil)
		if err != nil {
			t.Fatalf("n=%d Expand: %v", n, err)
		}
		if grid.VerticalAlign != GridVerticalAlignDefault {
			t.Errorf("n=%d vertical_align = %q, want %q", n, grid.VerticalAlign, GridVerticalAlignDefault)
		}
		max := grid.Rows[0].MaxHeight
		if max <= 0 {
			t.Fatalf("n=%d row has no max_height: the cards stretch to the content zone", n)
		}
		_, areaH := sizingAreaPt(ctx)
		if max > areaH*iconRowMaxHeightFrac+0.5 {
			t.Errorf("n=%d max_height %.0fpt exceeds the %.0f%% cap of a %.0fpt area", n, max, iconRowMaxHeightFrac*100, areaH)
		}
		if max < iconRowMinHeightPt {
			t.Errorf("n=%d max_height %.0fpt is below the %.0fpt floor", n, max, iconRowMinHeightPt)
		}
	}
}

// TestIconRow_LongCaptionGrowsTheRow checks the row is sized from its content
// rather than pinned to the floor: a caption that wraps needs more height.
func TestIconRow_LongCaptionGrowsTheRow(t *testing.T) {
	p, _ := Default().Get("icon-row")
	ctx := testThemeCtx()

	short := IconRowValues{
		{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
		{Icon: &IconRef{Name: "trending-up"}, Caption: "Grow"},
		{Icon: &IconRef{Name: "shield"}, Caption: "Protect"},
		{Icon: &IconRef{Name: "users"}, Caption: "Hire"},
		{Icon: &IconRef{Name: "target"}, Caption: "Measure"},
	}
	long := make(IconRowValues, len(short))
	copy(long, short)
	long[3].Caption = "Hire the delivery pod and run the onboarding programme"

	shortGrid, err := p.Expand(ctx, &short, nil, nil)
	if err != nil {
		t.Fatalf("Expand short: %v", err)
	}
	longGrid, err := p.Expand(ctx, &long, nil, nil)
	if err != nil {
		t.Fatalf("Expand long: %v", err)
	}
	if longGrid.Rows[0].MaxHeight <= shortGrid.Rows[0].MaxHeight {
		t.Errorf("a wrapping caption should grow the row: short=%.0f long=%.0f",
			shortGrid.Rows[0].MaxHeight, longGrid.Rows[0].MaxHeight)
	}
}

// TestIconRow_SecondaryChartKeepsTheZone checks the deliberate exemption: a
// cell carrying a secondary chart becomes a composite stack that needs the
// height, so capping the row would squash the chart.
func TestIconRow_SecondaryChartKeepsTheZone(t *testing.T) {
	p, _ := Default().Get("icon-row")
	ctx := testThemeCtx()
	items := IconRowValues{
		{Icon: &IconRef{Name: "rocket"}, Caption: "Launch", Secondary: &SecondaryChart{
			Type: "sparkline", Values: []float64{1, 2, 3},
		}},
		{Icon: &IconRef{Name: "trending-up"}, Caption: "Grow"},
		{Icon: &IconRef{Name: "shield"}, Caption: "Protect"},
	}
	grid, err := p.Expand(ctx, &items, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if grid.Rows[0].MaxHeight != 0 {
		t.Errorf("a secondary chart should keep the full zone, got max_height %.0f", grid.Rows[0].MaxHeight)
	}
}
