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
		if !strings.Contains(p.UseWhen(), "prefer") {
			t.Errorf("UseWhen() lacks contrastive language: %q", p.UseWhen())
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
		// Connector must not be emitted in dots style: a horizontal line
		// between adjacent rounded rectangles slashes through centered text
		// inside the cells (regression guard for go-slide-creator-2krk).
		if grid.Rows[0].Connector != nil {
			t.Errorf("dots style must not emit a row connector, got %+v", grid.Rows[0].Connector)
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

	// Chevron style tests
	t.Run("expand_chevron_style", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Discovery", Date: "30 Apr", Body: "Research"},
			{Label: "Design", Date: "15 May"},
			{Label: "Build", Date: "01 Jun"},
			{Label: "Test", Date: "15 Jul"},
			{Label: "Launch", Date: "01 Aug"},
			{Label: "Scale", Date: "30 Sep"},
		}
		ovr := &TimelineHorizontalOverrides{Style: "chevron", Accent: "accent2"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Should have 2 rows: chevrons + dates
		if len(grid.Rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(grid.Rows))
		}
		// Chevron row
		if len(grid.Rows[0].Cells) != 6 {
			t.Fatalf("expected 6 chevron cells, got %d", len(grid.Rows[0].Cells))
		}
		// No connector on chevron row (shapes are connected visually)
		if grid.Rows[0].Connector != nil {
			t.Error("chevron style should not have connectors")
		}
		// Check geometry is homePlate
		for i, cell := range grid.Rows[0].Cells {
			if cell.Shape.Geometry != "homePlate" {
				t.Errorf("cell[%d] geometry = %q, want homePlate", i, cell.Shape.Geometry)
			}
		}
		// First cell should have shade modifier (gradient)
		var firstFill struct {
			Color string `json:"color"`
			Shade int    `json:"shade"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &firstFill); err != nil {
			t.Fatalf("first cell fill unmarshal: %v", err)
		}
		if firstFill.Color != "accent2" {
			t.Errorf("first cell fill color = %q, want accent2", firstFill.Color)
		}
		if firstFill.Shade == 0 {
			t.Error("first cell should have shade modifier for gradient")
		}
		// Last cell should have tint modifier
		var lastFill struct {
			Color string `json:"color"`
			Tint  int    `json:"tint"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[5].Shape.Fill, &lastFill); err != nil {
			t.Fatalf("last cell fill unmarshal: %v", err)
		}
		if lastFill.Tint == 0 {
			t.Error("last cell should have tint modifier for gradient")
		}
		// Date row cells should have "none" fill
		for i, cell := range grid.Rows[1].Cells {
			var fill string
			if err := json.Unmarshal(cell.Shape.Fill, &fill); err != nil {
				t.Fatalf("date cell[%d] fill unmarshal: %v", i, err)
			}
			if fill != "none" {
				t.Errorf("date cell[%d] fill = %q, want none", i, fill)
			}
		}
		// Gap should be 0 for chevron style
		if grid.Gap != 0 {
			t.Errorf("gap = %g, want 0 for chevron style", grid.Gap)
		}
	})

	t.Run("expand_chevron_3_stops_middle_is_plain", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Start", Date: "Jan"},
			{Label: "Middle", Date: "Jun"},
			{Label: "End", Date: "Dec"},
		}
		ovr := &TimelineHorizontalOverrides{Style: "chevron"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Middle cell (index 1) should be plain accent (no modifiers)
		var midFill string
		if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Fill, &midFill); err != nil {
			t.Fatalf("middle cell fill unmarshal: %v", err)
		}
		if midFill != "accent1" {
			t.Errorf("middle cell fill = %q, want plain \"accent1\"", midFill)
		}
	})

	// Gantt style tests
	t.Run("expand_gantt_style", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Discovery", Date: "Apr 30", EndDate: "May 15"},
			{Label: "Design", Date: "May 15", EndDate: "Jun 01"},
			{Label: "Build", Date: "Jun 01", EndDate: "Jul 15"},
		}
		ovr := &TimelineHorizontalOverrides{Style: "gantt"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Each stop is its own row with 2 cells (label + bar)
		if len(grid.Rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
		}
		for i, row := range grid.Rows {
			if len(row.Cells) != 2 {
				t.Errorf("row[%d] expected 2 cells, got %d", i, len(row.Cells))
			}
		}
		// Columns should be [30, 70]
		var cols []int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if len(cols) != 2 || cols[0] != 30 || cols[1] != 70 {
			t.Errorf("columns = %v, want [30, 70]", cols)
		}
		// Bar cell should contain date range in text
		var barText struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Text, &barText); err != nil {
			t.Fatalf("bar text unmarshal: %v", err)
		}
		if barText.Paragraphs[0].Content != "Apr 30 → May 15" {
			t.Errorf("bar text = %q, want %q", barText.Paragraphs[0].Content, "Apr 30 → May 15")
		}
	})

	t.Run("validate_end_date_only_gantt", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1", Date: "Jan", EndDate: "Feb"},
			{Label: "Phase 2", Date: "Mar"},
			{Label: "Phase 3", Date: "May"},
		}
		// Without gantt style, end_date should error
		err := p.Validate(&vals, nil, nil)
		if err == nil {
			t.Fatal("expected error for end_date without gantt style")
		}
		if !strings.Contains(err.Error(), "end_date is only valid with style \"gantt\"") {
			t.Errorf("error = %q, want mention of end_date/gantt", err.Error())
		}

		// With gantt style, it should pass
		ovr := &TimelineHorizontalOverrides{Style: "gantt"}
		err = p.Validate(&vals, ovr, nil)
		if err != nil {
			t.Errorf("unexpected error with gantt style: %v", err)
		}
	})

	t.Run("validate_end_date_maxlen", func(t *testing.T) {
		vals := TimelineHorizontalValues{
			{Label: "Phase 1", Date: "Jan", EndDate: strings.Repeat("e", 31)},
			{Label: "Phase 2"},
			{Label: "Phase 3"},
		}
		ovr := &TimelineHorizontalOverrides{Style: "gantt"}
		err := p.Validate(&vals, ovr, nil)
		if err == nil {
			t.Fatal("expected error for long end_date")
		}
		if !strings.Contains(err.Error(), "exceeds maxLength 30") {
			t.Errorf("error = %q, want maxLength mention", err.Error())
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
