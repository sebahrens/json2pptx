package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultBMCValues() BMCCanvasValues {
	return BMCCanvasValues{
		KeyPartners:       BMCCell{Header: "Key Partners", Bullets: []string{"Supplier A", "Partner B"}},
		KeyActivities:     BMCCell{Header: "Key Activities", Bullets: []string{"Manufacturing", "Problem solving"}},
		KeyResources:      BMCCell{Header: "Key Resources", Bullets: []string{"Physical assets", "IP"}},
		ValuePropositions: BMCCell{Header: "Value Propositions", Bullets: []string{"Newness", "Performance", "Customization"}},
		CustomerRelations: BMCCell{Header: "Customer Relationships", Bullets: []string{"Personal assistance", "Self-service"}},
		Channels:          BMCCell{Header: "Channels", Bullets: []string{"Direct sales", "Web"}},
		CustomerSegments:  BMCCell{Header: "Customer Segments", Bullets: []string{"Mass market", "Niche market"}},
		CostStructure:     BMCCell{Header: "Cost Structure", Bullets: []string{"Fixed costs", "Variable costs"}},
		RevenueStreams:     BMCCell{Header: "Revenue Streams", Bullets: []string{"Asset sale", "Subscription"}},
	}
}

func TestBMCCanvas(t *testing.T) {
	p := &bmcCanvas{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "bmc-canvas" {
			t.Errorf("Name() = %q, want %q", p.Name(), "bmc-canvas")
		}
		if p.UseWhen() == "" {
			t.Error("UseWhen() must not be empty (D6)")
		}
		if !strings.Contains(p.UseWhen(), "card-grid") {
			t.Error("UseWhen() should mention card-grid as an alternative")
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

	// Validation tests
	tests := []struct {
		name      string
		values    BMCCanvasValues
		overrides *BMCCanvasOverrides
		cellOvr   map[int]any
		wantErr   string
		wantNoErr bool
	}{
		{
			name:      "happy_path",
			values:    defaultBMCValues(),
			wantNoErr: true,
		},
		{
			name: "missing_header",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.KeyPartners.Header = ""
				return v
			}(),
			wantErr: "key_partners.header is required",
		},
		{
			name: "header_exceeds_maxlen",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.ValuePropositions.Header = strings.Repeat("x", 61)
				return v
			}(),
			wantErr: "value_propositions.header exceeds maxLength 60",
		},
		{
			name: "empty_bullets",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.Channels.Bullets = nil
				return v
			}(),
			wantErr: "channels.bullets must have at least 1 item",
		},
		{
			name: "empty_bullets_hints_card_grid",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.Channels.Bullets = nil
				return v
			}(),
			wantErr: "card-grid",
		},
		{
			name: "too_many_bullets",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.CostStructure.Bullets = make([]string, 11)
				for i := range v.CostStructure.Bullets {
					v.CostStructure.Bullets[i] = "item"
				}
				return v
			}(),
			wantErr: "cost_structure.bullets exceeds maximum 10",
		},
		{
			name: "empty_bullet_string",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.RevenueStreams.Bullets = []string{"good", ""}
				return v
			}(),
			wantErr: "revenue_streams.bullets[1] must not be empty",
		},
		{
			name: "bullet_exceeds_maxlen",
			values: func() BMCCanvasValues {
				v := defaultBMCValues()
				v.KeyActivities.Bullets = []string{strings.Repeat("x", 201)}
				return v
			}(),
			wantErr: "key_activities.bullets[0] exceeds maxLength 200",
		},
		{
			name:   "cell_override_out_of_range",
			values: defaultBMCValues(),
			cellOvr: map[int]any{
				99: &BMCCanvasCellOverride{AccentBar: true},
			},
			wantErr: "out of range",
		},
		{
			name:   "invalid_cell_override_key",
			values: defaultBMCValues(),
			cellOvr: map[int]any{
				0: &struct {
					BadKey string `json:"bad_key"`
				}{BadKey: "nope"},
			},
			wantErr: `unknown key "bad_key"`,
		},
		{
			name:      "accent_override",
			values:    defaultBMCValues(),
			overrides: &BMCCanvasOverrides{Accent: "accent3"},
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
	t.Run("expand_default", func(t *testing.T) {
		vals := defaultBMCValues()
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		// 3 rows
		if len(grid.Rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
		}
		// Row 0: 5 cells
		if len(grid.Rows[0].Cells) != 5 {
			t.Errorf("row[0] expected 5 cells, got %d", len(grid.Rows[0].Cells))
		}
		// Row 1: 2 cells (3 span from row 0)
		if len(grid.Rows[1].Cells) != 2 {
			t.Errorf("row[1] expected 2 cells, got %d", len(grid.Rows[1].Cells))
		}
		// Row 2: 2 cells (with col spans)
		if len(grid.Rows[2].Cells) != 2 {
			t.Errorf("row[2] expected 2 cells, got %d", len(grid.Rows[2].Cells))
		}
		// Columns should be 5
		var cols int
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols != 5 {
			t.Errorf("columns = %d, want 5", cols)
		}
		// Key Partners (row 0, cell 0) should have rowSpan=2
		if grid.Rows[0].Cells[0].RowSpan != 2 {
			t.Errorf("key_partners row_span = %d, want 2", grid.Rows[0].Cells[0].RowSpan)
		}
		// Cost Structure (row 2, cell 0) should have colSpan=3
		if grid.Rows[2].Cells[0].ColSpan != 3 {
			t.Errorf("cost_structure col_span = %d, want 3", grid.Rows[2].Cells[0].ColSpan)
		}
		// Revenue Streams (row 2, cell 1) should have colSpan=2
		if grid.Rows[2].Cells[1].ColSpan != 2 {
			t.Errorf("revenue_streams col_span = %d, want 2", grid.Rows[2].Cells[1].ColSpan)
		}
		// Fill should be lt1 (light background for BMC cells)
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "lt1" {
			t.Errorf("fill = %q, want %q", fill, "lt1")
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		vals := defaultBMCValues()
		ovr := &BMCCanvasOverrides{Accent: "accent5"}
		grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Header color in text should reference accent5
		var textObj struct {
			Paragraphs []struct {
				Color string `json:"color"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &textObj); err != nil {
			t.Fatalf("text unmarshal: %v", err)
		}
		if len(textObj.Paragraphs) == 0 {
			t.Fatal("no paragraphs in text")
		}
		if textObj.Paragraphs[0].Color != "accent5" {
			t.Errorf("header color = %q, want %q", textObj.Paragraphs[0].Color, "accent5")
		}
	})

	t.Run("expand_accent_bar_override", func(t *testing.T) {
		vals := defaultBMCValues()
		cellOvr := map[int]any{
			3: &BMCCanvasCellOverride{AccentBar: true}, // value_propositions
		}
		grid, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// Value Propositions is row 0, cell index 2 (0=KP, 1=KA, 2=VP)
		vpCell := grid.Rows[0].Cells[2]
		if vpCell.AccentBar == nil {
			t.Fatal("value_propositions should have accent bar")
		}
		if vpCell.AccentBar.Color != "accent1" {
			t.Errorf("accent bar color = %q, want %q", vpCell.AccentBar.Color, "accent1")
		}
		if vpCell.AccentBar.Position != "top" {
			t.Errorf("accent bar position = %q, want %q", vpCell.AccentBar.Position, "top")
		}
		// Key Partners (cell 0) should NOT have accent bar
		if grid.Rows[0].Cells[0].AccentBar != nil {
			t.Error("key_partners should not have accent bar")
		}
	})

	// Golden file test
	t.Run("golden_default", func(t *testing.T) {
		vals := defaultBMCValues()
		grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "bmc-canvas", "default.golden.json")
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
