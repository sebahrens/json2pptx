package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimelineHorizontal(t *testing.T) {
	p := &timelineHorizontal{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "timeline-horizontal" {
			t.Errorf("Name() = %q, want %q", p.Name(), "timeline-horizontal")
		}
		if p.UseWhen() != "Linear timeline with stops" {
			t.Errorf("UseWhen() = %q, want %q", p.UseWhen(), "Linear timeline with stops")
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
		values    TimelineHorizontalValues
		overrides *TimelineHorizontalOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "happy_path_3_stops",
			values: TimelineHorizontalValues{
				{Label: "Phase 1", Date: "Q1 2025", Body: "Planning"},
				{Label: "Phase 2", Date: "Q2 2025", Body: "Development"},
				{Label: "Phase 3", Date: "Q3 2025", Body: "Launch"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_7_stops",
			values: TimelineHorizontalValues{
				{Label: "Step 1"},
				{Label: "Step 2"},
				{Label: "Step 3"},
				{Label: "Step 4"},
				{Label: "Step 5"},
				{Label: "Step 6"},
				{Label: "Step 7"},
			},
			wantNoErr: true,
		},
		{
			name: "happy_path_labels_only",
			values: TimelineHorizontalValues{
				{Label: "Start"},
				{Label: "Middle"},
				{Label: "End"},
			},
			wantNoErr: true,
		},
		{
			name: "too_few_stops_hints_icon_row",
			values: TimelineHorizontalValues{
				{Label: "Only one"},
				{Label: "Only two"},
			},
			wantErr: "icon-row",
		},
		{
			name: "too_many_stops",
			values: TimelineHorizontalValues{
				{Label: "1"}, {Label: "2"}, {Label: "3"}, {Label: "4"},
				{Label: "5"}, {Label: "6"}, {Label: "7"}, {Label: "8"},
			},
			wantErr: "at most 7 stops",
		},
		{
			name: "missing_label",
			values: TimelineHorizontalValues{
				{Label: "", Date: "Q1"},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
			},
			wantErr: "values[0].label is required",
		},
		{
			name: "label_exceeds_maxlen",
			values: TimelineHorizontalValues{
				{Label: strings.Repeat("x", 61)},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
			},
			wantErr: "exceeds maxLength 60",
		},
		{
			name: "date_exceeds_maxlen",
			values: TimelineHorizontalValues{
				{Label: "Phase 1", Date: strings.Repeat("d", 31)},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
			},
			wantErr: "exceeds maxLength 30",
		},
		{
			name: "body_exceeds_maxlen",
			values: TimelineHorizontalValues{
				{Label: "Phase 1", Body: strings.Repeat("b", 201)},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
			},
			wantErr: "exceeds maxLength 200",
		},
		{
			name: "invalid_cell_override_key",
			values: TimelineHorizontalValues{
				{Label: "Phase 1"},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
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
			values: TimelineHorizontalValues{
				{Label: "Phase 1"},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
			},
			cellOvr: map[int]any{
				7: &TimelineHorizontalCellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name: "accent_override",
			values: TimelineHorizontalValues{
				{Label: "Phase 1"},
				{Label: "Phase 2"},
				{Label: "Phase 3"},
			},
			overrides: &TimelineHorizontalOverrides{Accent: "accent2"},
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
	t.Run("expand_default_3_stops", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1", Date: "Q1 2025", Body: "Planning"},
			{Label: "Phase 2", Date: "Q2 2025", Body: "Development"},
			{Label: "Phase 3", Date: "Q3 2025", Body: "Launch"},
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
		// Check columns matches stop count
		var cols int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 3 {
			t.Errorf("columns = %d, want 3", cols)
		}
		// Check connector is present
		if grid.Rows[0].Connector == nil {
			t.Fatal("expected connector on row")
		}
		if grid.Rows[0].Connector.Style != "arrow" {
			t.Errorf("connector style = %q, want %q", grid.Rows[0].Connector.Style, "arrow")
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

	t.Run("expand_7_stops_dynamic_columns", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "S1"}, {Label: "S2"}, {Label: "S3"}, {Label: "S4"},
			{Label: "S5"}, {Label: "S6"}, {Label: "S7"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if len(grid.Rows[0].Cells) != 7 {
			t.Fatalf("expected 7 cells, got %d", len(grid.Rows[0].Cells))
		}
		var cols int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 7 {
			t.Errorf("columns = %d, want 7", cols)
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1"},
			{Label: "Phase 2"},
			{Label: "Phase 3"},
		}
		ovr := &TimelineHorizontalOverrides{Accent: "accent4"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		for i, cell := range grid.Rows[0].Cells {
			var fill string
			if err := json.Unmarshal(cell.Shape.Fill, &fill); err != nil {
				t.Fatalf("cell[%d] fill unmarshal: %v", i, err)
			}
			if fill != "accent4" {
				t.Errorf("cell[%d] fill = %q, want %q", i, fill, "accent4")
			}
		}
		// Connector should also use accent4
		if grid.Rows[0].Connector.Color != "accent4" {
			t.Errorf("connector color = %q, want %q", grid.Rows[0].Connector.Color, "accent4")
		}
	})

	t.Run("expand_connector_line_override", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1"},
			{Label: "Phase 2"},
			{Label: "Phase 3"},
		}
		ovr := &TimelineHorizontalOverrides{Connector: "line"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid.Rows[0].Connector.Style != "line" {
			t.Errorf("connector style = %q, want %q", grid.Rows[0].Connector.Style, "line")
		}
	})

	t.Run("expand_accent_bar_override", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1"},
			{Label: "Phase 2"},
			{Label: "Phase 3"},
		}
		cellOvr := map[int]any{
			1: &TimelineHorizontalCellOverride{AccentBar: true},
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
		if ab.Position != "top" {
			t.Errorf("accent bar position = %q, want %q", ab.Position, "top")
		}
	})

	t.Run("expand_labels_only_omits_date_body", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Start"},
			{Label: "Middle"},
			{Label: "End"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Check that text only contains 1 paragraph (label only, no date/body)
		for i, cell := range grid.Rows[0].Cells {
			var textObj struct {
				Paragraphs []json.RawMessage `json:"paragraphs"`
			}
			if err := json.Unmarshal(cell.Shape.Text, &textObj); err != nil {
				t.Fatalf("cell[%d] text unmarshal: %v", i, err)
			}
			if len(textObj.Paragraphs) != 1 {
				t.Errorf("cell[%d] expected 1 paragraph (label only), got %d", i, len(textObj.Paragraphs))
			}
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1", Date: "Q1 2025", Body: "Planning"},
			{Label: "Phase 2", Date: "Q2 2025", Body: "Development"},
			{Label: "Phase 3", Date: "Q3 2025", Body: "Launch"},
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "timeline-horizontal", "default.golden.json")
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
