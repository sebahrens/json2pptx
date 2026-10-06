package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCardGridCellUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    CardGridCell
		wantErr string
	}{
		{
			name:  "object_form",
			input: `{"header":"Title","body":"Content"}`,
			want:  CardGridCell{Header: "Title", Body: "Content"},
		},
		{
			name:  "string_shorthand",
			input: `"Title | Content"`,
			want:  CardGridCell{Header: "Title", Body: "Content"},
		},
		{
			name:  "string_with_pipe_in_body",
			input: `"Title | Body with | pipes"`,
			want:  CardGridCell{Header: "Title", Body: "Body with | pipes"},
		},
		{
			name:    "string_no_pipe",
			input:   `"NoPipeSeparator"`,
			wantErr: `must be "Header | Body"`,
		},
		{
			name:    "invalid_json",
			input:   `[1,2,3]`,
			wantErr: "must be string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got CardGridCell
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

	// Round-trip: string form and object form must expand identically
	t.Run("string_object_expand_equivalence", func(t *testing.T) {
		objJSON := `{"columns":2,"rows":1,"cells":[{"header":"A","body":"B"},{"header":"C","body":"D"}]}`
		strJSON := `{"columns":2,"rows":1,"cells":["A | B","C | D"]}`
		assertExpandEquivalent(t, objJSON, strJSON, func(raw string) (any, error) {
			var values CardGridValues
			if err := json.Unmarshal([]byte(raw), &values); err != nil {
				return nil, err
			}
			return (&cardGrid{}).Expand(ExpandContext{}, &values, nil, nil)
		})
	})
}

// Short cards stay content-sized (go-slide-creator-wntyw): no stretch towards
// the zone height, so no mostly empty panels.
func TestCardGridShortCardsStayContentSized(t *testing.T) {
	pat := &cardGrid{}
	vals := &CardGridValues{Columns: 2, Rows: 1, Cells: []CardGridCell{
		{Header: "Growth", Body: "12%"}, {Header: "Margin", Body: "42%"},
	}}
	grid, err := pat.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, zoneH := sizingAreaPt(fullThemeCtx())
	// An open card is a heading row, a rule row and a body row; the card is
	// the three together.
	if len(grid.Rows) != 3 {
		t.Fatalf("rows = %d, want the heading, rule and body rows of one row of open cards", len(grid.Rows))
	}
	cardH := 2 * grid.RowGap
	for r, row := range grid.Rows {
		if row.MaxHeight <= 0 {
			t.Errorf("row %d has no max height: it would stretch to the zone", r)
		}
		cardH += row.MaxHeight
	}
	if cardH > zoneH*0.4 {
		t.Errorf("card row %.1fpt stretches past 40%% of the %.1fpt zone for two short cards", cardH, zoneH)
	}
	if grid.VerticalAlign != GridVerticalAlignDefault {
		t.Errorf("card grid vertical_align = %q, want the middle-anchored default", grid.VerticalAlign)
	}
}

func TestCardGridCellUnmarshalReturnsValidationError(t *testing.T) {
	tests := []invalidShapeCase{
		{"string_no_pipe", `"NoPipe"`},
		{"array_instead_of_object", `[1,2]`},
	}
	assertInvalidShapeErrors(t, tests, "Header | Body", func(raw string) error {
		var c CardGridCell
		return json.Unmarshal([]byte(raw), &c)
	})
}

func TestCardGrid(t *testing.T) {
	p := &cardGrid{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "card-grid" {
			t.Errorf("Name() = %q, want %q", p.Name(), "card-grid")
		}
		if p.UseWhen() == "" {
			t.Error("UseWhen() must not be empty (D6)")
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
			// A grid larger than the card count leaves the last row short
			// (go-slide-creator-0w4va).
			name: "fewer_cells_than_grid",
			values: CardGridValues{
				Columns: 2,
				Rows:    2,
				Cells: []CardGridCell{
					{Header: "A", Body: "B"},
					{Header: "C", Body: "D"},
					{Header: "E", Body: "F"},
				},
			},
			wantNoErr: true,
		},
		{
			name: "no_columns_or_rows",
			values: CardGridValues{
				Cells: []CardGridCell{
					{Header: "A", Body: "B"},
					{Header: "C", Body: "D"},
					{Header: "E", Body: "F"},
					{Header: "G", Body: "H"},
					{Header: "I", Body: "J"},
				},
			},
			wantNoErr: true,
		},
		{
			name: "columns_only_too_many_rows",
			values: CardGridValues{
				Columns: 1,
				Cells:   make([]CardGridCell, 6),
			},
			wantErr: "columns=1 allows at most 5 rows",
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
			name: "rows_negative",
			values: CardGridValues{
				Columns: 2,
				Rows:    -1,
				Cells:   []CardGridCell{{Header: "A", Body: "B"}},
			},
			wantErr: "rows must be 1–5",
		},
		{
			name: "no_cells",
			values: CardGridValues{
				Columns: 2,
				Cells:   []CardGridCell{},
			},
			wantErr: "cells must contain at least 1",
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
			name: "more_cells_than_grid",
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
			// More cards than the grid holds: the fix is the grid, not a
			// different pattern.
			wantErr: "cells holds 9 cards but the grid has room for 4",
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
			overrides: &CardGridOverrides{TextOverrides: TextOverrides{Accent: "accent3"}},
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

	// KPI-shaped cells should produce a swap_pattern warning.
	t.Run("validate_kpi_shaped_3_cells_suggests_swap", func(t *testing.T) {
		vals := CardGridValues{
			Columns: 3,
			Rows:    1,
			Cells: []CardGridCell{
				{Header: "$4.2M", Body: "ARR"},
				{Header: "127%", Body: "NRR"},
				{Header: "12d", Body: "Sales cycle"},
			},
		}
		err := p.Validate(&vals, nil, nil)
		if err == nil {
			t.Fatal("expected swap_pattern warning for KPI-shaped cells, got nil")
		}
		if !strings.Contains(err.Error(), "kpi-3up") {
			t.Errorf("expected suggestion for kpi-3up, got: %v", err)
		}
	})

	// Non-metric headers should NOT trigger KPI swap suggestion.
	t.Run("validate_text_headers_no_swap", func(t *testing.T) {
		vals := CardGridValues{
			Columns: 3,
			Rows:    1,
			Cells: []CardGridCell{
				{Header: "Strategy", Body: "Details"},
				{Header: "Operations", Body: "Details"},
				{Header: "Finance", Body: "Details"},
			},
		}
		err := p.Validate(&vals, nil, nil)
		if err != nil {
			t.Errorf("unexpected error for non-metric headers: %v", err)
		}
	})

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
		// The default is the open card: per row of cards a heading row, a rule
		// row and a body row, and one spacer row between the two rows of cards.
		open, err := p.Expand(ExpandContext{}, &vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand (open default): %v", err)
		}
		if open == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(open.Rows) != 7 {
			t.Fatalf("open default: expected 7 rows (2 x heading/rule/body + 1 spacer), got %d", len(open.Rows))
		}
		if string(open.Columns) != "3" {
			t.Errorf("open default: columns = %s, want 3", open.Columns)
		}
		if open.ColGap != cardGridOpenColGapPt || open.RowGap != cardGridOpenRowGapPt || open.Gap != 0 {
			t.Errorf("open default: gaps col=%v row=%v gap=%v, want col %v, row %v, no uniform gap",
				open.ColGap, open.RowGap, open.Gap, cardGridOpenColGapPt, cardGridOpenRowGapPt)
		}
		if spacer := open.Rows[3]; len(spacer.Cells) != 1 || spacer.Cells[0].Shape != nil || spacer.Cells[0].ColSpan != 3 ||
			spacer.MinHeight <= 0 || spacer.MinHeight != spacer.MaxHeight {
			t.Errorf("open default: row 3 = %+v, want one empty cell across the 3 columns at a fixed height", spacer)
		}
		for band := 0; band < 2; band++ {
			headRow, ruleRow, bodyRow := open.Rows[4*band], open.Rows[4*band+1], open.Rows[4*band+2]
			if headRow.MinHeight <= 0 || headRow.MinHeight != headRow.MaxHeight {
				t.Errorf("open default: heading row %d height %v..%v, want fixed", band, headRow.MinHeight, headRow.MaxHeight)
			}
			if ruleRow.MinHeight != cardGridOpenRulePt || ruleRow.MaxHeight != cardGridOpenRulePt {
				t.Errorf("open default: rule row %d height %v..%v, want %vpt", band, ruleRow.MinHeight, ruleRow.MaxHeight, cardGridOpenRulePt)
			}
			if bodyRow.MinHeight != 0 || bodyRow.MaxHeight <= 0 {
				t.Errorf("open default: body row %d height %v..%v, want a max height only", band, bodyRow.MinHeight, bodyRow.MaxHeight)
			}
			for _, row := range []struct {
				name string
				row  []*jsonschema.GridCellInput
			}{{"heading", headRow.Cells}, {"rule", ruleRow.Cells}, {"body", bodyRow.Cells}} {
				if len(row.row) != 3 {
					t.Fatalf("open default: %s row %d has %d cells, want 3", row.name, band, len(row.row))
				}
			}
			for col := 0; col < 3; col++ {
				n := band*3 + col + 1
				head, rule, body := headRow.Cells[col], ruleRow.Cells[col], bodyRow.Cells[col]
				if head.AccentBar != nil || rule.AccentBar != nil || body.AccentBar != nil {
					t.Errorf("open default: card %d carries an accent bar", n)
				}
				if got := string(head.Shape.Fill); got != `"none"` {
					t.Errorf("open default: card %d heading fill = %s, want none", n, got)
				}
				if got := string(rule.Shape.Fill); got != string(neutralFillJSON(NeutralTint60)) {
					t.Errorf("open default: card %d rule fill = %s, want neutral 60%%", n, got)
				}
				if rule.BleedTop != 0 || len(rule.Shape.Text) != 0 {
					t.Errorf("open default: card %d rule bleed_top = %v text = %s, want a plain 1pt rule", n, rule.BleedTop, rule.Shape.Text)
				}
				if got := string(body.Shape.Fill); got != `"none"` {
					t.Errorf("open default: card %d body fill = %s, want none", n, got)
				}
				var headText, bodyText cardTextObj
				if err := json.Unmarshal(head.Shape.Text, &headText); err != nil {
					t.Fatalf("heading text unmarshal: %v", err)
				}
				if err := json.Unmarshal(body.Shape.Text, &bodyText); err != nil {
					t.Fatalf("body text unmarshal: %v", err)
				}
				if len(headText.Paragraphs) != 1 || headText.VerticalAlign != "b" {
					t.Fatalf("open default: card %d heading = %+v, want one bottom-anchored paragraph", n, headText)
				}
				if hp := headText.Paragraphs[0]; hp.Content != fmt.Sprintf("Card %d", n) || !hp.Bold || hp.Color != "dk1" {
					t.Errorf("open default: card %d heading paragraph = %+v, want bold dk1 %q", n, hp, fmt.Sprintf("Card %d", n))
				}
				if len(bodyText.Paragraphs) != 1 || bodyText.VerticalAlign != "t" {
					t.Fatalf("open default: card %d body = %+v, want one top-anchored paragraph", n, bodyText)
				}
				if bp := bodyText.Paragraphs[0]; bp.Content != fmt.Sprintf("Desc %d", n) || bp.Bold || bp.Color != "dk1" {
					t.Errorf("open default: card %d body paragraph = %+v, want regular dk1 %q", n, bp, fmt.Sprintf("Desc %d", n))
				}
			}
		}

		// The filled tile, on request: one cell per card.
		grid, err := p.Expand(ExpandContext{}, &vals, &CardGridOverrides{Style: "filled"}, nil)
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
		// A filled tile takes the accent as its fill.
		ovr := &CardGridOverrides{TextOverrides: TextOverrides{Accent: "accent5"}, Style: "filled"}
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

		// The open default spends the accent only on an emphasised card's
		// rule: a plain open card stays neutral whatever the accent, and the
		// card that asks for accent_bar stands on a rule in that accent.
		open, err := p.Expand(ExpandContext{}, &vals, &CardGridOverrides{TextOverrides: TextOverrides{Accent: "accent5"}}, nil)
		if err != nil {
			t.Fatalf("Expand (open default): %v", err)
		}
		if len(open.Rows) != 3 {
			t.Fatalf("open default: expected 3 rows (heading, rule, body), got %d", len(open.Rows))
		}
		if got := string(open.Rows[0].Cells[0].Shape.Fill); got != `"none"` {
			t.Errorf("open default: heading fill = %s, want none", got)
		}
		if got := string(open.Rows[1].Cells[0].Shape.Fill); got != string(neutralFillJSON(NeutralTint60)) {
			t.Errorf("open default: rule fill = %s, want neutral 60%%", got)
		}
		if strings.Contains(string(open.Rows[0].Cells[0].Shape.Text)+string(open.Rows[2].Cells[0].Shape.Text), "accent5") {
			t.Errorf("open default: plain card text takes the accent: %s / %s", open.Rows[0].Cells[0].Shape.Text, open.Rows[2].Cells[0].Shape.Text)
		}
		emph, err := p.Expand(ExpandContext{}, &vals, &CardGridOverrides{TextOverrides: TextOverrides{Accent: "accent5"}},
			map[int]any{0: &CardGridCellOverride{AccentBar: true}})
		if err != nil {
			t.Fatalf("Expand (open, accent_bar): %v", err)
		}
		if got := string(emph.Rows[1].Cells[0].Shape.Fill); got != `"accent5"` {
			t.Errorf("open default: emphasised rule fill = %s, want %q", got, "accent5")
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
		// On the open default accent_bar is the card's rule: the accent, 1pt
		// heavier (bled upwards), and no AccentBar on any cell.
		open, err := p.Expand(ExpandContext{}, &vals, nil, cellOvr)
		if err != nil {
			t.Fatalf("Expand (open default): %v", err)
		}
		if len(open.Rows) != 3 {
			t.Fatalf("open default: expected 3 rows (heading, rule, body), got %d", len(open.Rows))
		}
		for r, row := range open.Rows {
			if len(row.Cells) != 2 {
				t.Fatalf("open default: row %d has %d cells, want 2", r, len(row.Cells))
			}
			for ci, cell := range row.Cells {
				if cell.AccentBar != nil {
					t.Errorf("open default: row %d cell %d carries an AccentBar; the rule is the accent bar", r, ci)
				}
			}
		}
		emphRule, plainRule := open.Rows[1].Cells[0], open.Rows[1].Cells[1]
		if got := string(emphRule.Shape.Fill); got != `"accent1"` {
			t.Errorf("open default: emphasised rule fill = %s, want %q", got, "accent1")
		}
		if emphRule.BleedTop != 1 {
			t.Errorf("open default: emphasised rule bleed_top = %v, want 1pt", emphRule.BleedTop)
		}
		if got := string(plainRule.Shape.Fill); got != string(neutralFillJSON(NeutralTint60)) {
			t.Errorf("open default: plain rule fill = %s, want neutral 60%%", got)
		}
		if plainRule.BleedTop != 0 {
			t.Errorf("open default: plain rule bleed_top = %v, want 0", plainRule.BleedTop)
		}
		if open.Rows[0].Cells[0].BleedTop != 0 || open.Rows[2].Cells[0].BleedTop != 0 {
			t.Errorf("open default: only the rule bleeds, got heading %v body %v", open.Rows[0].Cells[0].BleedTop, open.Rows[2].Cells[0].BleedTop)
		}

		// A filled tile carries the bar itself.
		grid, err := p.Expand(ExpandContext{}, &vals, &CardGridOverrides{Style: "filled"}, cellOvr)
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

func TestCardGridStyles(t *testing.T) {
	p := &cardGrid{}
	cells := []CardGridCell{
		{Header: "Card 1", Body: "Description 1"},
		{Header: "Card 2", Body: "Description 2"},
		{Header: "Card 3", Body: "Description 3"},
		{Header: "Card 4", Body: "Description 4"},
	}
	vals := &CardGridValues{Columns: 2, Rows: 2, Cells: cells}

	t.Run("filled", func(t *testing.T) {
		grid, err := p.Expand(ExpandContext{}, vals, &CardGridOverrides{Style: "filled"}, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if len(grid.Rows) != 2 || len(grid.Rows[0].Cells) != 2 {
			t.Fatalf("filled: want 2 rows of 2 one-cell cards, got %d rows", len(grid.Rows))
		}
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "accent1" {
			t.Errorf("filled: fill = %q, want %q", fill, "accent1")
		}
	})

	// No style: the open card (heading, rule, body rows; 4N-1 grid rows).
	t.Run("open_default", func(t *testing.T) {
		for _, ovr := range []*CardGridOverrides{nil, {}, {Style: "open"}} {
			var o any
			eff := &CardGridOverrides{}
			if ovr != nil {
				o, eff = ovr, ovr
			}
			if got := cardGridStyle(vals, eff); got != "open" {
				t.Errorf("cardGridStyle(%+v) = %q, want open", ovr, got)
			}
			if err := p.Validate(vals, o, nil); err != nil {
				t.Fatalf("Validate(%+v): %v", ovr, err)
			}
			grid, err := p.Expand(ExpandContext{}, vals, o, nil)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if len(grid.Rows) != 7 {
				t.Fatalf("open (%+v): rows = %d, want 7", ovr, len(grid.Rows))
			}
			if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"none"` {
				t.Errorf("open (%+v): heading fill = %s, want none", ovr, got)
			}
			if got := string(grid.Rows[1].Cells[0].Shape.Fill); got != string(neutralFillJSON(NeutralTint60)) {
				t.Errorf("open (%+v): rule fill = %s, want neutral 60%%", ovr, got)
			}
			if got := string(grid.Rows[2].Cells[0].Shape.Fill); got != `"none"` {
				t.Errorf("open (%+v): body fill = %s, want none", ovr, got)
			}
		}
	})

	// A deck that asks for a card surface, or attaches a secondary chart, is
	// asking for a tile: the default falls back to the filled card.
	t.Run("default_falls_back_to_filled", func(t *testing.T) {
		withSecondary := &CardGridValues{Columns: 2, Rows: 2, Cells: append([]CardGridCell(nil), cells...)}
		withSecondary.Cells[1].Secondary = &SecondaryChart{}
		for _, tc := range []struct {
			name string
			vals *CardGridValues
			ovr  *CardGridOverrides
		}{
			{"card_fill", vals, &CardGridOverrides{CardFill: "#FFF5ED"}},
			{"line_color", vals, &CardGridOverrides{LineColor: "dk1"}},
			{"line_width", vals, &CardGridOverrides{LineWidth: 1}},
			{"border", vals, &CardGridOverrides{Border: "accent"}},
			{"secondary", withSecondary, &CardGridOverrides{}},
		} {
			if got := cardGridStyle(tc.vals, tc.ovr); got != "filled" {
				t.Errorf("%s: cardGridStyle = %q, want filled", tc.name, got)
			}
			// An explicit style still wins over the fallback.
			explicit := *tc.ovr
			explicit.Style = "open"
			if got := cardGridStyle(tc.vals, &explicit); got != "open" {
				t.Errorf("%s with style open: cardGridStyle = %q, want open", tc.name, got)
			}
		}
		grid, err := p.Expand(ExpandContext{}, vals, &CardGridOverrides{CardFill: "#FFF5ED"}, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if len(grid.Rows) != 2 || len(grid.Rows[0].Cells) != 2 {
			t.Fatalf("card_fill: want 2 rows of 2 one-cell cards, got %d rows", len(grid.Rows))
		}
		if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"#FFF5ED"` {
			t.Errorf("card_fill: fill = %s, want the authored surface", got)
		}
		bordered, err := p.Expand(ExpandContext{}, vals, &CardGridOverrides{Border: "accent"}, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if len(bordered.Rows) != 2 {
			t.Fatalf("border: rows = %d, want the 2 rows of filled cards", len(bordered.Rows))
		}
		if got := string(bordered.Rows[0].Cells[0].Shape.Fill); got != `"accent1"` {
			t.Errorf("border: fill = %s, want the filled card's accent1", got)
		}
	})

	t.Run("accent_stripe", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "accent-stripe"}
		grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// No white card on white paper: the neutral 4% surface.
		if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != neutral4JSON {
			t.Errorf("accent-stripe: fill = %s, want neutral 4%%", got)
		}
		ab := grid.Rows[0].Cells[0].AccentBar
		if ab == nil {
			t.Fatal("accent-stripe: expected accent bar")
		}
		if ab.Position != "left" {
			t.Errorf("accent-stripe: accent bar position = %q, want %q", ab.Position, "left")
		}
		if ab.Color != "accent1" {
			t.Errorf("accent-stripe: accent bar color = %q, want %q", ab.Color, "accent1")
		}
	})

	t.Run("numbered_badge", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "numbered-badge"}
		numberedCells := []CardGridCell{
			{Header: "1. Launch", Body: "Description"},
			{Header: "2. Growth", Body: "Description"},
			{Header: "Card 3", Body: "No prefix"},
			{Header: "Card 4", Body: "No prefix"},
		}
		numberedVals := &CardGridValues{Columns: 2, Rows: 2, Cells: numberedCells}
		grid, err := p.Expand(ExpandContext{}, numberedVals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// No white card on white paper: the neutral 4% surface.
		if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != neutral4JSON {
			t.Errorf("numbered-badge: fill = %s, want neutral 4%%", got)
		}
	})

	t.Run("icon_card", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "icon-card"}
		iconCells := []CardGridCell{
			{Header: "Launch", Body: "Description", Icon: &IconRef{Name: "rocket"}},
			{Header: "Growth", Body: "Description", Icon: &IconRef{Name: "trending-up"}},
			{Header: "Revenue", Body: "Description"},
			{Header: "Target", Body: "Description"},
		}
		iconVals := &CardGridValues{Columns: 2, Rows: 2, Cells: iconCells}
		// Validation should accept bundled icon names and missing icons alike.
		if err := p.Validate(iconVals, ovr, nil); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		grid, err := p.Expand(ExpandContext{}, iconVals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// No white card on white paper: the neutral 4% surface.
		if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != neutral4JSON {
			t.Errorf("icon-card: fill = %s, want neutral 4%%", got)
		}
		ab := grid.Rows[0].Cells[0].AccentBar
		if ab == nil {
			t.Fatal("icon-card: expected accent bar")
		}
		if ab.Position != "top" {
			t.Errorf("icon-card: accent bar position = %q, want %q", ab.Position, "top")
		}
		// Cell with bundled icon must have an IconInput overlay (not a text glyph paragraph).
		shape0 := grid.Rows[0].Cells[0].Shape
		if shape0.Icon == nil {
			t.Fatal("icon-card: expected IconInput overlay on cell with icon")
		}
		if shape0.Icon.Name != "rocket" {
			t.Errorf("icon-card overlay Name = %q, want %q", shape0.Icon.Name, "rocket")
		}
		// Text content must be header + body only — no icon glyph paragraph.
		var text0 map[string]any
		if err := json.Unmarshal(shape0.Text, &text0); err != nil {
			t.Fatalf("text unmarshal: %v", err)
		}
		paras0 := text0["paragraphs"].([]any)
		if len(paras0) != 2 {
			t.Fatalf("icon-card: expected 2 paragraphs (header + body), got %d", len(paras0))
		}
		if paras0[0].(map[string]any)["content"] != "Launch" {
			t.Errorf("icon-card: first paragraph = %v, want header %q", paras0[0], "Launch")
		}
		// Cell without icon: no overlay, no bullet fallback paragraph.
		shape1 := grid.Rows[1].Cells[0].Shape
		if shape1.Icon != nil {
			t.Errorf("icon-card: cell without icon should not have IconInput overlay, got %+v", shape1.Icon)
		}
		var text1 map[string]any
		if err := json.Unmarshal(shape1.Text, &text1); err != nil {
			t.Fatalf("text unmarshal: %v", err)
		}
		paras1 := text1["paragraphs"].([]any)
		if len(paras1) != 2 {
			t.Fatalf("icon-card (no icon): expected 2 paragraphs (header + body), got %d", len(paras1))
		}
		if first := paras1[0].(map[string]any)["content"]; first == "\u2022" {
			t.Errorf("icon-card: bullet fallback should be removed, got %q", first)
		}
	})

	t.Run("icon_card_rejects_emoji", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "icon-card"}
		emojiCells := []CardGridCell{
			{Header: "Launch", Body: "Description", Icon: &IconRef{Name: "\U0001F680"}},
			{Header: "Growth", Body: "Description", Icon: &IconRef{Name: "trending-up"}},
			{Header: "Revenue", Body: "Description"},
			{Header: "Target", Body: "Description"},
		}
		emojiVals := &CardGridValues{Columns: 2, Rows: 2, Cells: emojiCells}
		err := p.Validate(emojiVals, ovr, nil)
		if err == nil {
			t.Fatal("expected validation error for emoji icon, got nil")
		}
		if !strings.Contains(err.Error(), "cells[0].icon") {
			t.Errorf("error %q should mention cells[0].icon", err.Error())
		}
		if !strings.Contains(err.Error(), "bundled icon name") {
			t.Errorf("error %q should mention bundled icon name", err.Error())
		}
	})

	t.Run("icon_card_rejects_unknown_name", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "icon-card"}
		cells := []CardGridCell{
			{Header: "A", Body: "B", Icon: &IconRef{Name: "not-a-real-icon-xyz"}},
		}
		vals := &CardGridValues{Columns: 1, Rows: 1, Cells: cells}
		err := p.Validate(vals, ovr, nil)
		if err == nil {
			t.Fatal("expected validation error for unknown icon name, got nil")
		}
	})

	t.Run("tinted", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "tinted"}
		fallback, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand without surface metadata: %v", err)
		}
		// No surface metadata: two neutral steps, never an outlined white card
		// (go-slide-creator-pgdkp).
		if got := string(fallback.Rows[0].Cells[0].Shape.Fill); got != neutral4JSON {
			t.Errorf("fallback card 0 fill = %s, want neutral 4%%", got)
		}
		if got := string(fallback.Rows[0].Cells[1].Shape.Fill); got != neutral8JSON {
			t.Errorf("fallback card 1 fill = %s, want neutral 8%%", got)
		}
		if got := string(fallback.Rows[0].Cells[0].Shape.Line); got != `"none"` {
			t.Errorf("fallback card line = %s, want none", got)
		}
		ctx := ExpandContext{Metadata: &types.TemplateMetadata{SurfaceTints: map[string]string{"subtle": "lt2", "paper": "lt1"}}}
		grid, err := p.Expand(ctx, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// The declared subtle surface is kept; the page-coloured paper role
		// becomes the neutral 4% step, and no card is outlined
		// (go-slide-creator-pgdkp).
		if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"lt2"` {
			t.Errorf("tinted: cell 0 fill = %s, want the declared lt2", got)
		}
		if got := string(grid.Rows[0].Cells[1].Shape.Fill); got != neutral4JSON {
			t.Errorf("tinted: cell 1 fill = %s, want neutral 4%%", got)
		}
		for ci := 0; ci < 2; ci++ {
			if got := string(grid.Rows[0].Cells[ci].Shape.Line); got != `"none"` {
				t.Errorf("tinted card %d line = %s, want none", ci, got)
			}
		}
		ovr.Border = "none"
		grid, err = p.Expand(ctx, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand with explicit border override: %v", err)
		}
		if got := string(grid.Rows[0].Cells[1].Shape.Line); got != `"none"` {
			t.Errorf("tinted paper card explicit border = %s, want none", got)
		}
	})

	t.Run("invalid_style", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "invalid-style"}
		err := p.Validate(vals, ovr, nil)
		if err == nil {
			t.Fatal("expected error for invalid style")
		}
		if !strings.Contains(err.Error(), "overrides.style") {
			t.Errorf("error %q should mention overrides.style", err)
		}
		if want := "must be one of open, filled, accent-stripe, numbered-badge, icon-card, tinted, soft-card"; !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should list the styles: %q", err, want)
		}
	})
}

func TestCardGridSoftCardAndSurfaceOverrides(t *testing.T) {
	p := &cardGrid{}
	cells := []CardGridCell{
		{Header: "Card 1", Body: "Description 1"},
		{Header: "Card 2", Body: "Description 2"},
	}
	vals := &CardGridValues{Columns: 2, Rows: 1, Cells: cells}

	// soft-card: neutral 4% surface, explicit no border, dark text.
	t.Run("soft_card_style", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "soft-card"}
		grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		shape := grid.Rows[0].Cells[0].Shape
		if got := string(shape.Fill); got != neutral4JSON {
			t.Errorf("soft-card: fill = %s, want the neutral 4%% surface", got)
		}
		var line string
		if err := json.Unmarshal(shape.Line, &line); err != nil {
			t.Fatalf("line unmarshal: %v", err)
		}
		if line != "none" {
			t.Errorf("soft-card: line = %q, want %q (no visible border)", line, "none")
		}
		// Body text must stay dark (dk1) for contrast on the pale surface.
		var text map[string]any
		if err := json.Unmarshal(shape.Text, &text); err != nil {
			t.Fatalf("text unmarshal: %v", err)
		}
		paras := text["paragraphs"].([]any)
		if got := paras[1].(map[string]any)["color"]; got != "dk1" {
			t.Errorf("soft-card: body color = %v, want dk1", got)
		}
		if grid.Rows[0].Cells[0].AccentBar != nil {
			t.Error("soft-card: expected no accent bar")
		}
	})

	t.Run("template_surface_wins", func(t *testing.T) {
		ctx := ExpandContext{Metadata: &types.TemplateMetadata{SurfaceTints: map[string]string{"subtle": "lt2"}}}
		grid, err := p.Expand(ctx, vals, &CardGridOverrides{Style: "soft-card"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"lt2"` {
			t.Fatalf("explicit subtle surface replaced: %s", got)
		}
	})

	// card_fill paints a caller-supplied hex surface across styles (#FFF5ED).
	t.Run("card_fill_override_soft_card", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "soft-card", CardFill: "#FFF5ED"}
		grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var fill string
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "#FFF5ED" {
			t.Errorf("card_fill: fill = %q, want %q", fill, "#FFF5ED")
		}
	})

	// numbered-badge can coexist with a soft surface via card_fill (text stays dark).
	t.Run("card_fill_coexists_with_numbered_badge", func(t *testing.T) {
		badgeCells := []CardGridCell{
			{Header: "1. Launch", Body: "Description"},
			{Header: "2. Growth", Body: "Description"},
		}
		bv := &CardGridValues{Columns: 2, Rows: 1, Cells: badgeCells}
		ovr := &CardGridOverrides{Style: "numbered-badge", CardFill: "#FFF5ED"}
		grid, err := p.Expand(ExpandContext{}, bv, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		shape := grid.Rows[0].Cells[0].Shape
		var fill string
		if err := json.Unmarshal(shape.Fill, &fill); err != nil {
			t.Fatalf("fill unmarshal: %v", err)
		}
		if fill != "#FFF5ED" {
			t.Errorf("numbered-badge+card_fill: fill = %q, want %q", fill, "#FFF5ED")
		}
		// numbered-badge keeps its dark header/body paragraphs.
		var text map[string]any
		if err := json.Unmarshal(shape.Text, &text); err != nil {
			t.Fatalf("text unmarshal: %v", err)
		}
		paras := text["paragraphs"].([]any)
		if len(paras) != 3 {
			t.Fatalf("numbered-badge: expected 3 paragraphs (badge+header+body), got %d", len(paras))
		}
		if got := paras[2].(map[string]any)["color"]; got != "dk1" {
			t.Errorf("numbered-badge+card_fill: body color = %v, want dk1", got)
		}
	})

	// explicit line_color/line_width produces an object line override.
	t.Run("line_color_width_override", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "soft-card", LineColor: "#888888", LineWidth: 0.75}
		grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var line struct {
			Color string  `json:"color"`
			Width float64 `json:"width"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Line, &line); err != nil {
			t.Fatalf("line unmarshal: %v", err)
		}
		if line.Color != "#888888" || line.Width != 0.75 {
			t.Errorf("line override = %+v, want {#888888 0.75}", line)
		}
	})

	// border:accent uses the resolved accent color at 1pt.
	t.Run("border_accent", func(t *testing.T) {
		ovr := &CardGridOverrides{TextOverrides: TextOverrides{Accent: "accent2"}, Style: "soft-card", Border: "accent"}
		grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var line struct {
			Color string  `json:"color"`
			Width float64 `json:"width"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Line, &line); err != nil {
			t.Fatalf("line unmarshal: %v", err)
		}
		if line.Color != "accent2" || line.Width != 1 {
			t.Errorf("border:accent line = %+v, want {accent2 1}", line)
		}
	})

	// Validation: bad color, bad border keyword, out-of-range width.
	t.Run("validate_rejects_bad_overrides", func(t *testing.T) {
		bad := []struct {
			name    string
			ovr     *CardGridOverrides
			wantErr string
		}{
			{"bad_card_fill", &CardGridOverrides{CardFill: "salmon"}, "card_fill"},
			{"bad_line_color", &CardGridOverrides{LineColor: "zzz"}, "line_color"},
			{"line_width_too_high", &CardGridOverrides{LineWidth: 99}, "line_width"},
			{"bad_border", &CardGridOverrides{Border: "thick"}, "border"},
		}
		for _, tc := range bad {
			t.Run(tc.name, func(t *testing.T) {
				err := p.Validate(vals, tc.ovr, nil)
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q should mention %q", err.Error(), tc.wantErr)
				}
			})
		}
	})

	// Valid overrides pass validation, including hex without "#".
	t.Run("validate_accepts_good_overrides", func(t *testing.T) {
		ovr := &CardGridOverrides{Style: "soft-card", CardFill: "FFF5ED", LineColor: "dk1", LineWidth: 1, Border: "none"}
		if err := p.Validate(vals, ovr, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCardGridRecommendedSoftCard(t *testing.T) {
	p := &cardGrid{}
	ctx := fullThemeCtx()
	for i := range ctx.Theme.Colors {
		if ctx.Theme.Colors[i].Name == "accent1" {
			ctx.Theme.Colors[i].RGB = "#FD5108"
		}
	}
	v := &CardGridValues{Columns: 2, Rows: 1, Cells: []CardGridCell{
		{Header: "Build", Body: "Slow"},
		{Header: "Buy", Body: "Fast", Recommended: true},
	}}
	grid, err := p.Expand(ctx, v, &CardGridOverrides{Style: "soft-card"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plain := grid.Rows[0].Cells[0].Shape
	selected := grid.Rows[0].Cells[1].Shape
	if string(plain.Fill) != neutral4JSON && string(plain.Fill) != `"lt2"` {
		t.Fatalf("unselected fill = %s, want a neutral surface", plain.Fill)
	}
	if string(selected.Fill) == string(plain.Fill) {
		t.Fatalf("selected fill = %s", selected.Fill)
	}
	var selectedText cardTextObj
	if err := json.Unmarshal(selected.Text, &selectedText); err != nil {
		t.Fatal(err)
	}
	fg, fgOK := resolveThemeColor(ctx, selectedText.Paragraphs[0].Color)
	// Measured against the fill as painted: a mid-tone accent is shaded
	// until lt1 reads (go-slide-creator-v9tup).
	selTone, _ := parseFillTone(selected.Fill)
	bg, bgOK := effectiveFillColor(ctx, selTone)
	if !strings.Contains(string(selected.Text), "RECOMMENDED") || !fgOK || !bgOK || fg.ContrastWith(bg) < 4.5 {
		t.Fatalf("selected text missing badge/contrast: %s", selected.Text)
	}
	if err := p.Validate(v, &CardGridOverrides{Style: "filled"}, nil); err == nil || !strings.Contains(err.Error(), "recommended is supported only by soft-card") {
		t.Fatalf("filled style silently drops recommendation: %v", err)
	}
	withFill, err := p.Expand(ctx, v, &CardGridOverrides{Style: "soft-card", CardFill: "#FFF5ED"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(withFill.Rows[0].Cells[0].Shape.Fill); got != `"#FFF5ED"` {
		t.Fatalf("unselected card lost card_fill override: %s", got)
	}
	if got := string(withFill.Rows[0].Cells[1].Shape.Fill); got != string(selected.Fill) {
		t.Fatalf("card_fill hid recommended selection: %s", got)
	}
}

func TestCardGridSoftCardGolden(t *testing.T) {
	p := &cardGrid{}
	vals := CardGridValues{
		Columns: 2,
		Rows:    2,
		Cells: []CardGridCell{
			{Header: "Card 1", Body: "Description 1"},
			{Header: "Card 2", Body: "Description 2"},
			{Header: "Card 3", Body: "Description 3"},
			{Header: "Card 4", Body: "Description 4"},
		},
	}
	ovr := &CardGridOverrides{Style: "soft-card", CardFill: "#FFF5ED"}
	grid, err := p.Expand(ExpandContext{}, &vals, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	got, err := json.MarshalIndent(grid, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "card-grid", "soft-card.golden.json")
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
}

func TestExtractNumberPrefix(t *testing.T) {
	tests := []struct {
		header        string
		fallback      int
		wantBadge     string
		wantRemainder string
	}{
		{"1. Launch", 1, "1", "Launch"},
		{"2) Growth", 2, "2", "Growth"},
		{"10. Ten", 10, "10", "Ten"},
		{"No prefix", 1, "1", "No prefix"},
		{"", 3, "3", ""},
		{"1.", 1, "1", "1."},
	}

	for _, tc := range tests {
		t.Run(tc.header, func(t *testing.T) {
			badge, remainder := extractNumberPrefix(tc.header, tc.fallback)
			if badge != tc.wantBadge {
				t.Errorf("badge = %q, want %q", badge, tc.wantBadge)
			}
			if remainder != tc.wantRemainder {
				t.Errorf("remainder = %q, want %q", remainder, tc.wantRemainder)
			}
		})
	}
}

// TestCardGrid_HeadersShareABodyBaseline pins the fix for go-slide-creator-ommn:
// a header that wraps to two lines while its neighbour's fits on one pushed
// only that card's body down, so three panels meant to read as one comparison
// came out ragged (measured: 19.2pt apart in the rendered PDF). This is the
// one-cell (filled) card, whose short header is padded with filler lines; the
// open default solves it structurally — see
// TestCardGrid_OpenHeadingsShareARule.
func TestCardGrid_HeadersShareABodyBaseline(t *testing.T) {
	p, _ := Default().Get("card-grid")
	ctx := testThemeCtx()
	vals := &CardGridValues{
		Columns: 3,
		Rows:    1,
		Cells: []CardGridCell{
			{Header: "A | Double down on parcel automation", Body: "Highest return, but it needs the Q3 capex envelope."},
			{Header: "B | Acquire a regional freight forwarder", Body: "Buys network density fast; integration risk is front-loaded."},
			{Header: "C | Exit freight", Body: "Releases capital immediately and removes the loss-making lane."},
		},
	}
	grid, err := p.Expand(ctx, vals, &CardGridOverrides{Style: "filled"}, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// What must match across the row is the number of rendered LINES before
	// the body — a wrapping header is one paragraph over two lines, a padded
	// short header is two paragraphs over two lines.
	contentW, _ := contentAreaPt(ctx)
	textW := equalColumnWidthPt(contentW, 3, 10) - 2*defaultShapeInsetLRPt
	want := -1
	for i, cell := range grid.Rows[0].Cells {
		var obj cardTextObj
		if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		lines := 0
		for _, para := range obj.Paragraphs[:len(obj.Paragraphs)-1] {
			if strings.TrimSpace(para.Content) == "" {
				lines++ // a blank padding line
				continue
			}
			lines += measuredLines(para.Content, ctx.Theme.BodyFont, para.Bold, para.Size, textW)
		}
		if want < 0 {
			want = lines
		}
		if lines != want {
			t.Errorf("cell %d: %d lines before the body, want %d — bodies do not share a baseline", i, lines, want)
		}
	}
	if want < 2 {
		t.Fatalf("expected the short header to be padded to the wrapping headers' height, lines = %d", want)
	}

	// The short header ("C | Exit freight") is the one that gained a blank line.
	var third cardTextObj
	if err := json.Unmarshal(grid.Rows[0].Cells[2].Shape.Text, &third); err != nil {
		t.Fatalf("third cell text: %v", err)
	}
	if got := third.Paragraphs[0].Content; got != "C | Exit freight" {
		t.Errorf("third card header = %q, want the authored header unchanged", got)
	}
	if got := strings.TrimSpace(third.Paragraphs[1].Content); got != "" {
		t.Errorf("third card paragraph 1 = %q, want a blank padding line", got)
	}
}

// TestCardGrid_OpenHeadingsShareARule is the same row on the open default: the
// headings are their own grid row, fixed at the tallest heading's height and
// bottom-anchored, so a one-line heading stands on the same rule as a wrapped
// one and the bodies start level — with no filler paragraphs.
func TestCardGrid_OpenHeadingsShareARule(t *testing.T) {
	p, _ := Default().Get("card-grid")
	ctx := testThemeCtx()
	vals := &CardGridValues{
		Columns: 3,
		Rows:    1,
		Cells: []CardGridCell{
			{Header: "A | Double down on parcel automation", Body: "Highest return, but it needs the Q3 capex envelope."},
			{Header: "B | Acquire a regional freight forwarder", Body: "Buys network density fast; integration risk is front-loaded."},
			{Header: "C | Exit freight", Body: "Releases capital immediately and removes the loss-making lane."},
		},
	}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Fatalf("rows = %d, want heading, rule and body", len(grid.Rows))
	}
	headRow, ruleRow, bodyRow := grid.Rows[0], grid.Rows[1], grid.Rows[2]

	contentW, _ := contentAreaPt(ctx)
	textW := equalColumnWidthPt(contentW, 3, grid.ColGap) - 2*defaultShapeInsetLRPt
	if grid.ColGap != ctx.Gap(cardGridOpenColGapPt) {
		t.Errorf("col gap = %v, want %v", grid.ColGap, ctx.Gap(cardGridOpenColGapPt))
	}
	tallest, maxLines, minLines := 0.0, 0, 99
	for i, cell := range headRow.Cells {
		var obj cardTextObj
		if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
			t.Fatalf("heading %d text: %v", i, err)
		}
		if len(obj.Paragraphs) != 1 {
			t.Fatalf("heading %d has %d paragraphs, want the authored heading alone (no filler lines)", i, len(obj.Paragraphs))
		}
		para := obj.Paragraphs[0]
		if para.Content != vals.Cells[i].Header || !para.Bold || para.Color != "dk1" {
			t.Errorf("heading %d = %+v, want the bold dk1 authored header", i, para)
		}
		if obj.VerticalAlign != "b" {
			t.Errorf("heading %d vertical_align = %q, want b (standing on its rule)", i, obj.VerticalAlign)
		}
		lines := measuredLines(para.Content, ctx.Theme.BodyFont, para.Bold, para.Size, textW)
		maxLines, minLines = max(maxLines, lines), min(minLines, lines)
		if h := shapeTextHeightPt(ctx.Theme.BodyFont, cell.Shape.Text, textW); h > tallest {
			tallest = h
		}
	}
	if maxLines < 2 || minLines != 1 {
		t.Fatalf("fixture drifted: headings take %d..%d lines, want a one-line heading beside a wrapped one", minLines, maxLines)
	}
	// One fixed height for the whole heading row: it holds the tallest
	// (wrapped) heading, so it is taller — by at least a line of heading type
	// — than the same row with one-line headings.
	if headRow.MinHeight != headRow.MaxHeight || headRow.MinHeight != math.Ceil(headRow.MinHeight) {
		t.Errorf("heading row height %v..%v, want one fixed whole-point height", headRow.MinHeight, headRow.MaxHeight)
	}
	if headRow.MinHeight < tallest {
		t.Errorf("heading row height %v is under the tallest heading's %v text height", headRow.MinHeight, tallest)
	}
	short := &CardGridValues{Columns: 3, Rows: 1, Cells: []CardGridCell{
		{Header: "A", Body: vals.Cells[0].Body}, {Header: "B", Body: vals.Cells[1].Body}, {Header: "C", Body: vals.Cells[2].Body},
	}}
	shortGrid, err := p.Expand(ctx, short, nil, nil)
	if err != nil {
		t.Fatalf("Expand (one-line headings): %v", err)
	}
	headerSize := ResolveSize(0, sizeHeaderPt)
	if grew := headRow.MinHeight - shortGrid.Rows[0].MinHeight; grew < headerSize*float64(maxLines-1) {
		t.Errorf("heading row grew %vpt for %d-line headings, want at least %d more line(s) of %vpt type", grew, maxLines, maxLines-1, headerSize)
	}
	// The body row is unaffected by the heading wrap: bodies start level.
	if bodyRow.MaxHeight != shortGrid.Rows[2].MaxHeight {
		t.Errorf("body row max height %v differs from %v with one-line headings", bodyRow.MaxHeight, shortGrid.Rows[2].MaxHeight)
	}
	if ruleRow.MinHeight != cardGridOpenRulePt || ruleRow.MaxHeight != cardGridOpenRulePt {
		t.Errorf("rule row height %v..%v, want %vpt", ruleRow.MinHeight, ruleRow.MaxHeight, cardGridOpenRulePt)
	}
	for i, cell := range bodyRow.Cells {
		var obj cardTextObj
		if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
			t.Fatalf("body %d text: %v", i, err)
		}
		if len(obj.Paragraphs) != 1 || obj.Paragraphs[0].Content != vals.Cells[i].Body || obj.VerticalAlign != "t" {
			t.Errorf("body %d = %+v, want the authored body alone, top-anchored under the rule", i, obj)
		}
	}
}

// TestCardGrid_EqualHeadersAreNotPadded checks the pass is a no-op when every
// header already occupies the same number of lines (the one-cell filled card;
// an open card never pads).
func TestCardGrid_EqualHeadersAreNotPadded(t *testing.T) {
	p, _ := Default().Get("card-grid")
	ctx := testThemeCtx()
	vals := &CardGridValues{
		Columns: 3,
		Rows:    1,
		Cells: []CardGridCell{
			{Header: "Speed", Body: "Ship the first wave in March."},
			{Header: "Cost", Body: "Hold the run rate flat through H2."},
			{Header: "Risk", Body: "One integration team, one cutover."},
		},
	}
	grid, err := p.Expand(ctx, vals, &CardGridOverrides{Style: "filled"}, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != 3 {
		t.Fatalf("want one row of three one-cell cards, got %d rows", len(grid.Rows))
	}
	for i, cell := range grid.Rows[0].Cells {
		var obj cardTextObj
		if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		if len(obj.Paragraphs) != 2 {
			t.Errorf("cell %d has %d paragraphs, want the unpadded header + body", i, len(obj.Paragraphs))
		}
	}
}
