package patterns

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func frameworkValues(t *testing.T) *FrameworkGridValues {
	t.Helper()
	p, ok := Default().Get("framework-grid")
	if !ok {
		t.Fatal("framework-grid is not registered")
	}
	return p.(Exemplar).ExemplarValues().(*FrameworkGridValues)
}

func TestFrameworkGrid_Metadata(t *testing.T) {
	p, ok := Default().Get("framework-grid")
	if !ok {
		t.Fatal("framework-grid is not registered")
	}
	if p.Name() != "framework-grid" || p.Version() != 1 {
		t.Errorf("name/version = %q/%d", p.Name(), p.Version())
	}
	for _, sibling := range []string{"table-highlight", "value-chain", "card-grid", "matrix-2x2", "stylish-panels", "process-grid-2row"} {
		if !strings.Contains(p.UseWhen()+p.NotWhen(), sibling) {
			t.Errorf("use_when/not_when does not contrast with %s", sibling)
		}
	}
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" || len(tx.NarrativeRole) == 0 || len(tx.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tx)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) {
		t.Fatalf("schema does not marshal: %v", err)
	}
	for _, want := range []string{`"rows"`, `"cards"`, `"label_width_pct"`, `"title_size"`, `"body_size"`, `"cell_accent_mode"`, "2020-12"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema missing %s", want)
		}
	}
}

func TestFrameworkGrid_Validate(t *testing.T) {
	p, _ := Default().Get("framework-grid")
	if err := p.Validate(frameworkValues(t), nil, nil); err != nil {
		t.Fatalf("exemplar rejected: %v", err)
	}
	cases := []struct {
		name string
		mut  func(v *FrameworkGridValues)
		ovr  *FrameworkGridOverrides
		want string
	}{
		{"one row", func(v *FrameworkGridValues) { v.Rows = v.Rows[:1] }, nil, "card-grid"},
		{"seven rows", func(v *FrameworkGridValues) {
			for len(v.Rows) < 7 {
				v.Rows = append(v.Rows, v.Rows[0])
			}
		}, nil, "rows"},
		{"no cards", func(v *FrameworkGridValues) { v.Rows[1].Cards = nil }, nil, "rows[1].cards"},
		{"five cards", func(v *FrameworkGridValues) {
			for len(v.Rows[0].Cards) < 5 {
				v.Rows[0].Cards = append(v.Rows[0].Cards, FrameworkGridCard{Title: "x"})
			}
		}, nil, "rows[0].cards"},
		{"missing label", func(v *FrameworkGridValues) { v.Rows[0].Label = "" }, nil, "rows[0].label"},
		{"missing title", func(v *FrameworkGridValues) { v.Rows[2].Cards[1].Title = " " }, nil, "rows[2].cards[1].title"},
		{"body too long", func(v *FrameworkGridValues) { v.Rows[0].Cards[0].Body = strings.Repeat("b", 161) }, nil, "(161 chars)"},
		{"bad accent mode", func(*FrameworkGridValues) {}, &FrameworkGridOverrides{CellAccentMode: "rainbow"}, "cell_accent_mode"},
		{"label width out of range", func(*FrameworkGridValues) {}, &FrameworkGridOverrides{LabelWidthPct: 50}, "label_width_pct"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := frameworkValues(t)
			tc.mut(v)
			var ovr any
			if tc.ovr != nil {
				ovr = tc.ovr
			}
			err := p.Validate(v, ovr, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
	t.Run("multi-byte text inside the budget", func(t *testing.T) {
		v := frameworkValues(t)
		v.Rows[0].Cards[0].Title = strings.Repeat("é", 39) + "€"
		v.Rows[0].Label = strings.Repeat("ö", 40)
		if err := p.Validate(v, nil, nil); err != nil {
			t.Fatalf("40 characters rejected by a byte count: %v", err)
		}
	})
	t.Run("cell override keys", func(t *testing.T) {
		v := frameworkValues(t)
		total := 0
		for _, r := range v.Rows {
			total += 1 + len(r.Cards)
		}
		if err := p.Validate(v, nil, map[int]any{total: &CellOverride{AccentBar: true}}); err == nil {
			t.Fatal("out-of-range cell override accepted")
		}
		if err := p.Validate(v, nil, map[int]any{0: map[string]any{"geometry": "ellipse"}}); err == nil {
			t.Fatal("non-D15 key accepted")
		}
	})
}

func TestFrameworkGrid_ExpandStructure(t *testing.T) {
	p, _ := Default().Get("framework-grid")
	v := frameworkValues(t)
	grid, err := p.Expand(ExpandContext{}, v, nil, map[int]any{4: &CellOverride{AccentBar: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != len(v.Rows) {
		t.Fatalf("rows = %d, want %d", len(grid.Rows), len(v.Rows))
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 4 || cols[0] != fgDefaultLabelPct {
		t.Fatalf("columns = %s, want a label column plus 3 card columns", grid.Columns)
	}
	// Row 1 (Process) has two cards: its third card column is empty.
	if c := grid.Rows[1].Cells[3]; c.Shape != nil {
		t.Errorf("short row should leave trailing space empty, got %+v", c.Shape)
	}
	label := grid.Rows[0].Cells[0].Shape
	if !strings.Contains(string(label.Text), `"bold":true`) || !strings.Contains(string(label.Fill), "accent1") {
		t.Errorf("row label should be bold on an accent tint: fill=%s text=%s", label.Fill, label.Text)
	}
	card := grid.Rows[0].Cells[1].Shape
	if !strings.Contains(string(card.Text), `"color":"accent1"`) || !strings.Contains(string(card.Fill), "tint") {
		t.Errorf("card should carry an accent title on a pale wash: fill=%s text=%s", card.Fill, card.Text)
	}
	// Cell override index 4 is row 1's label (row 0 = label + 3 cards).
	if grid.Rows[1].Cells[0].AccentBar == nil {
		t.Error("cell override 4 should put an accent bar on the second row label")
	}
	for i, r := range grid.Rows {
		if r.MaxHeight <= 0 || r.MaxHeight != grid.Rows[0].MaxHeight {
			t.Errorf("row %d height %.0f: rows should share one content-sized height", i, r.MaxHeight)
		}
	}
}

func TestFrameworkGrid_CellAccentModes(t *testing.T) {
	p, _ := Default().Get("framework-grid")
	cases := []struct {
		mode, base string
		want       []string
	}{
		{"uniform", "accent1", []string{"accent1", "accent1", "accent1"}},
		{"alternate", "accent1", []string{"accent1", "accent2", "accent1"}},
		{"progressive", "accent1", []string{"accent1", "accent2", "accent3"}},
		{"uniform", "accent3", []string{"accent3", "accent3", "accent3"}},
		{"alternate", "accent3", []string{"accent3", "accent4", "accent3"}},
		{"progressive", "accent3", []string{"accent3", "accent4", "accent5"}},
	}
	for _, tc := range cases {
		t.Run(tc.mode+"/"+tc.base, func(t *testing.T) {
			grid, err := p.Expand(ExpandContext{}, frameworkValues(t), &FrameworkGridOverrides{Accent: tc.base, CellAccentMode: tc.mode}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for j, want := range tc.want {
				fill := string(grid.Rows[0].Cells[1+j].Shape.Fill)
				if !strings.Contains(fill, `"`+want+`"`) {
					t.Errorf("card %d fill %s, want a wash of %s", j, fill, want)
				}
			}
		})
	}
}

// The card title stays in the accent where it reads at the bar the render-time
// contrast pass applies, and falls back to measured theme ink where it does not.
func TestFrameworkGrid_TitleInkIsMeasured(t *testing.T) {
	p, _ := Default().Get("framework-grid")
	for name, colors := range bundledThemeColors {
		t.Run(name, func(t *testing.T) {
			ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
			grid, err := p.Expand(ctx, frameworkValues(t), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range grid.Rows {
				for _, cell := range row.Cells[1:] {
					if cell.Shape == nil {
						continue
					}
					var text patternTextObj
					if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
						t.Fatal(err)
					}
					ratio, ok := schemeContrastOnTone(ctx, text.Paragraphs[0].Color, fgCardTone("accent1"))
					if !ok || ratio < 4.5 {
						t.Errorf("%q title ink %s reads %.2f:1", text.Paragraphs[0].Content, text.Paragraphs[0].Color, ratio)
					}
				}
			}
		})
	}
}

func TestFrameworkGrid_Warnings(t *testing.T) {
	p := &frameworkGrid{}
	if got := p.PostExpandWarnings(ExpandContext{}, frameworkValues(t), nil); len(got) != 0 {
		t.Fatalf("exemplar should be clean: %v", got)
	}
	v := &FrameworkGridValues{}
	for i := 0; i < 6; i++ {
		row := FrameworkGridRow{Label: "Dimension"}
		for j := 0; j < 4; j++ {
			row.Cards = append(row.Cards, FrameworkGridCard{Title: "Lever", Body: strings.Repeat("word ", 32)})
		}
		v.Rows = append(v.Rows, row)
	}
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") || !strings.Contains(got[0], "rows[0]") {
		t.Fatalf("overfull framework should warn BODY_TOO_LONG: %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}

func TestFrameworkGrid_Golden(t *testing.T) {
	p, _ := Default().Get("framework-grid")
	grid, err := p.Expand(ExpandContext{}, frameworkValues(t), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "framework-grid", "default.golden.json"))
}
