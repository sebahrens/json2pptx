package patterns

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func heatmapValues(t *testing.T) *CapabilityHeatmapValues {
	t.Helper()
	p, ok := Default().Get("capability-heatmap")
	if !ok {
		t.Fatal("capability-heatmap is not registered")
	}
	return p.(Exemplar).ExemplarValues().(*CapabilityHeatmapValues)
}

func TestCapabilityHeatmap_Metadata(t *testing.T) {
	p, ok := Default().Get("capability-heatmap")
	if !ok {
		t.Fatal("capability-heatmap is not registered")
	}
	if p.Name() != "capability-heatmap" || p.Version() != 1 {
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
	for _, want := range []string{`"tiers"`, `"columns"`, `"sublabel"`, `"show_legend"`, `"header_shape"`, `"cell_size"`, "2020-12"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema missing %s", want)
		}
	}
}

func TestCapabilityHeatmap_Validate(t *testing.T) {
	p, _ := Default().Get("capability-heatmap")
	if err := p.Validate(heatmapValues(t), nil, nil); err != nil {
		t.Fatalf("exemplar rejected: %v", err)
	}

	cases := []struct {
		name string
		mut  func(v *CapabilityHeatmapValues)
		want string
	}{
		{"too few columns", func(v *CapabilityHeatmapValues) { v.Columns = v.Columns[:2] }, "table-highlight"},
		{"too many columns", func(v *CapabilityHeatmapValues) {
			for len(v.Columns) < 9 {
				v.Columns = append(v.Columns, v.Columns[0])
			}
		}, "split"},
		{"one tier", func(v *CapabilityHeatmapValues) {
			v.Tiers = v.Tiers[:1]
			for i := range v.Columns {
				for j := range v.Columns[i].Cells {
					v.Columns[i].Cells[j].Tier = 0
				}
			}
		}, "card-grid"},
		{"five tiers", func(v *CapabilityHeatmapValues) {
			v.Tiers = append(v.Tiers, CapabilityHeatmapTier{Label: "X"}, CapabilityHeatmapTier{Label: "Y"})
		}, "tiers"},
		{"tier out of range", func(v *CapabilityHeatmapValues) { v.Columns[1].Cells[0].Tier = 3 }, "columns[1].cells[0].tier"},
		{"negative tier", func(v *CapabilityHeatmapValues) { v.Columns[0].Cells[2].Tier = -1 }, "columns[0].cells[2].tier"},
		{"missing header", func(v *CapabilityHeatmapValues) { v.Columns[2].Header = " " }, "columns[2].header"},
		{"missing cell text", func(v *CapabilityHeatmapValues) { v.Columns[0].Cells[0].Text = "" }, "columns[0].cells[0].text"},
		{"empty column", func(v *CapabilityHeatmapValues) { v.Columns[0].Cells = nil }, "columns[0].cells"},
		{"seven cells", func(v *CapabilityHeatmapValues) {
			for len(v.Columns[0].Cells) < 7 {
				v.Columns[0].Cells = append(v.Columns[0].Cells, CapabilityHeatmapCell{Text: "x"})
			}
		}, "columns[0].cells"},
		{"cell too long", func(v *CapabilityHeatmapValues) { v.Columns[0].Cells[0].Text = strings.Repeat("x", 61) }, "(61 chars)"},
		{"tier label too long", func(v *CapabilityHeatmapValues) { v.Tiers[0].Label = strings.Repeat("x", 31) }, "tiers[0].label"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := heatmapValues(t)
			tc.mut(v)
			err := p.Validate(v, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}

	t.Run("multi-byte text inside the budget", func(t *testing.T) {
		v := heatmapValues(t)
		v.Columns[0].Cells[0].Text = strings.Repeat("ü", 59) + "€"
		v.Columns[0].Header = strings.Repeat("ä", 40)
		if err := p.Validate(v, nil, nil); err != nil {
			t.Fatalf("60 characters rejected by a byte count: %v", err)
		}
	})
	t.Run("header_shape enum", func(t *testing.T) {
		if err := p.Validate(heatmapValues(t), &CapabilityHeatmapOverrides{HeaderShape: "chevron"}, nil); err == nil {
			t.Fatal("unknown header_shape accepted")
		}
	})
	t.Run("cell override out of range", func(t *testing.T) {
		v := heatmapValues(t)
		total := len(v.Columns)
		for _, c := range v.Columns {
			total += len(c.Cells)
		}
		if err := p.Validate(v, nil, map[int]any{total - 1: &CellOverride{AccentBar: true}}); err != nil {
			t.Fatalf("last valid index rejected: %v", err)
		}
		if err := p.Validate(v, nil, map[int]any{total: &CellOverride{AccentBar: true}}); err == nil {
			t.Fatal("out-of-range cell override accepted")
		}
	})
}

func TestCapabilityHeatmap_ExpandStructure(t *testing.T) {
	p, _ := Default().Get("capability-heatmap")
	v := heatmapValues(t)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	maxCells := 0
	for _, c := range v.Columns {
		maxCells = max(maxCells, len(c.Cells))
	}
	// header + cell rows + legend
	if got, want := len(grid.Rows), 1+maxCells+1; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if string(grid.Columns) != "4" {
		t.Errorf("columns = %s, want 4", grid.Columns)
	}
	header := grid.Rows[0].Cells[0].Shape
	if header.Geometry != "homePlate" || header.Adjustments["adj"] <= 0 {
		t.Errorf("header should be a homePlate with an adj, got %s %v", header.Geometry, header.Adjustments)
	}
	if !strings.Contains(string(header.Text), `"inset_right"`) || !strings.Contains(string(header.Text), "Up to 30% capacity unlock") {
		t.Errorf("header text should clear the point and carry the sublabel: %s", header.Text)
	}
	// Column 2 (Operations) has two cells: rows 3 and 4 leave it empty.
	if c := grid.Rows[3].Cells[2]; c.Shape != nil {
		t.Errorf("short column should leave an empty cell, got %+v", c.Shape)
	}
	// Tier fills: tier 0 is the accent itself, tier 1 a tint, tier 2 a grey.
	if got := string(grid.Rows[1].Cells[0].Shape.Fill); got != `"accent1"` {
		t.Errorf("tier 0 fill = %s", got)
	}
	if got := string(grid.Rows[2].Cells[0].Shape.Fill); !strings.Contains(got, "accent1") || !strings.Contains(got, "lumOff") {
		t.Errorf("tier 1 fill = %s", got)
	}
	if got := string(grid.Rows[3].Cells[0].Shape.Fill); !strings.Contains(got, "lt1") {
		t.Errorf("tier 2 fill = %s", got)
	}
	legend := grid.Rows[len(grid.Rows)-1].Cells[0]
	if legend.ColSpan != 4 || legend.Grid == nil || len(legend.Grid.Rows[0].Cells) != 2*len(v.Tiers) {
		t.Errorf("legend should span the grid with a swatch+label per tier: %+v", legend)
	}
	if legend.Grid.Rows[0].MaxHeight <= 0 {
		t.Error("legend's nested row needs an explicit height")
	}
}

func TestCapabilityHeatmap_ExpandOverrides(t *testing.T) {
	p, _ := Default().Get("capability-heatmap")
	v := heatmapValues(t)
	hide := false
	grid, err := p.Expand(ExpandContext{}, v, &CapabilityHeatmapOverrides{Accent: "accent3", HeaderShape: "rect", ShowLegend: &hide}, map[int]any{
		0: &CellOverride{AccentBar: true},
		4: &CellOverride{AccentBar: true}, // first cell of column 0
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(grid.Rows); got != 1+4 {
		t.Errorf("hidden legend should drop the legend row, got %d rows", got)
	}
	if g := grid.Rows[0].Cells[0].Shape.Geometry; g != "rect" {
		t.Errorf("header_shape rect ignored: %s", g)
	}
	if got := string(grid.Rows[1].Cells[0].Shape.Fill); got != `"accent3"` {
		t.Errorf("accent override: tier 0 fill = %s", got)
	}
	if grid.Rows[0].Cells[0].AccentBar == nil || grid.Rows[1].Cells[0].AccentBar == nil {
		t.Error("cell overrides 0 (header) and 4 (first cell) should carry accent bars")
	}
	if grid.Rows[1].Cells[1].AccentBar != nil {
		t.Error("accent bar leaked to an unaddressed cell")
	}
}

// Text on every tier fill is chosen by measurement: white on the accent, dark
// on the tints and greys, on every bundled palette.
func TestCapabilityHeatmap_TierInkIsMeasured(t *testing.T) {
	p, _ := Default().Get("capability-heatmap")
	for name, colors := range bundledThemeColors {
		t.Run(name, func(t *testing.T) {
			ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
			v := heatmapValues(t)
			grid, err := p.Expand(ctx, v, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for r := 1; r <= 3; r++ {
				for _, cell := range grid.Rows[r].Cells {
					if cell.Shape == nil {
						continue
					}
					var fill fillTone
					if err := json.Unmarshal(cell.Shape.Fill, &fill.Color); err != nil {
						var obj struct {
							Color  string `json:"color"`
							LumMod int    `json:"lumMod"`
							LumOff int    `json:"lumOff"`
						}
						if err := json.Unmarshal(cell.Shape.Fill, &obj); err != nil {
							t.Fatal(err)
						}
						fill = fillTone{Color: obj.Color, LumMod: obj.LumMod, LumOff: obj.LumOff}
					}
					var text patternTextObj
					if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
						t.Fatal(err)
					}
					ratio, ok := schemeContrastOnTone(ctx, text.Paragraphs[0].Color, fill)
					if !ok || ratio < 4.5 {
						t.Errorf("row %d %q: ink %s on %+v reads %.2f:1", r, text.Paragraphs[0].Content, text.Paragraphs[0].Color, fill, ratio)
					}
				}
			}
		})
	}
}

func schemeContrastOnTone(ctx ExpandContext, ink string, tone fillTone) (float64, bool) {
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return 0, false
	}
	c, ok := resolveThemeColor(ctx, ink)
	if !ok {
		return 0, false
	}
	return c.ContrastWith(fill), true
}

func TestCapabilityHeatmap_Warnings(t *testing.T) {
	p := &capabilityHeatmap{}
	if got := p.PostExpandWarnings(ExpandContext{}, heatmapValues(t), nil); len(got) != 0 {
		t.Fatalf("exemplar should be clean: %v", got)
	}
	v := heatmapValues(t)
	for len(v.Columns) < 8 {
		v.Columns = append(v.Columns, v.Columns[0])
	}
	v.Columns[5].Header = "Supercalifragilistic"
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeTextExceedsShape+":") || !strings.Contains(got[0], "Supercalifragilistic") {
		t.Fatalf("unbreakable header should warn TEXT_EXCEEDS_SHAPE: %v", got)
	}

	v = heatmapValues(t)
	for len(v.Columns) < 8 {
		v.Columns = append(v.Columns, CapabilityHeatmapColumn{Header: "Extra", Cells: []CapabilityHeatmapCell{{Text: "x"}}})
	}
	for i := range v.Columns {
		for len(v.Columns[i].Cells) < 6 {
			v.Columns[i].Cells = append(v.Columns[i].Cells, CapabilityHeatmapCell{Text: strings.Repeat("activity ", 6), Tier: 1})
		}
	}
	got = p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") {
		t.Fatalf("overfull heatmap should warn BODY_TOO_LONG: %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}

func TestCapabilityHeatmap_Golden(t *testing.T) {
	p, _ := Default().Get("capability-heatmap")
	grid, err := p.Expand(ExpandContext{}, heatmapValues(t), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "capability-heatmap", "default.golden.json"))
}
