package slides

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// besideCycleRegion is one region of every kind a column can hold next to a
// cycle region, with or without a heading.
func besideCycleRegion(kind string, heading bool) map[string]any {
	var r map[string]any
	switch kind {
	case RegionKPIs:
		r = map[string]any{"kind": kind, "kpis": []any{
			map[string]any{"value": "34%", "label": "Less rework"},
			map[string]any{"value": "12", "label": "Named owners"},
			map[string]any{"value": "4.6", "label": "Satisfaction"},
		}}
	case RegionStat:
		r = map[string]any{"kind": kind, "value": "34%", "label": "Less rework"}
	case RegionTimeline:
		r = map[string]any{"kind": kind, "milestones": []any{
			map[string]any{"date": "Jan", "label": "Pilot"},
			map[string]any{"date": "Apr", "label": "Three lines"},
			map[string]any{"date": "Sep", "label": "All sites"},
		}}
	case RegionTable:
		r = map[string]any{"kind": kind, "headers": []any{"Line", "Jan", "Sep"}, "rows": []any{
			[]any{"Claims", "18%", "11%"},
			[]any{"Billing", "12%", "9%"},
		}}
	case RegionChart:
		r = map[string]any{"kind": kind, "chart": map[string]any{"type": "bar", "data": map[string]any{
			"categories": []any{"Jan", "Sep"},
			"series":     []any{map[string]any{"name": "Rework", "values": []any{18.0, 12.0}}},
		}}}
	case RegionImage:
		r = map[string]any{"kind": kind, "image": map[string]any{"path": "desk.png", "alt": "The service desk"}}
	}
	if heading {
		r["heading"] = "Since January"
	}
	return r
}

func compileBesideCycle(t *testing.T, arrangement string, regions ...map[string]any) *deckinput.ShapeGridInput {
	t.Helper()
	list := make([]any, len(regions))
	for i, r := range regions {
		list[i] = r
	}
	slide, _, err := CompileRegions(Input{Title: "The loop", Body: map[string]any{"arrangement": arrangement, "regions": list}})
	if err != nil {
		t.Fatalf("CompileRegions: %v", err)
	}
	return slide.ShapeGrid
}

func nestedPattern(t *testing.T, cell *deckinput.GridCellInput) deckinput.PatternInput {
	t.Helper()
	var p deckinput.PatternInput
	if err := json.Unmarshal(cell.Pattern, &p); err != nil {
		t.Fatalf("nested pattern: %v", err)
	}
	return p
}

func besideCycleLoop() map[string]any {
	return map[string]any{"kind": "cycle", "size_pct": 60.0, "phases": []any{"Plan", "Do", "Check", "Act"}}
}

// A region that is only as tall as its content — a KPI row, a stat, a
// timeline — has no top edge in common with the ring next to it: without a
// heading its pattern is centred in the column by the pattern's own
// vertical_align (go-slide-creator-n3q0o).
func TestPatternRegionBesideACycleIsCentred(t *testing.T) {
	for _, kind := range []string{RegionKPIs, RegionStat, RegionTimeline} {
		t.Run(kind, func(t *testing.T) {
			grid := compileBesideCycle(t, ArrangeColumns, besideCycleLoop(), besideCycleRegion(kind, false))
			cell := grid.Rows[0].Cells[1]
			if len(cell.Pattern) == 0 || cell.Grid != nil {
				t.Fatalf("cell = %+v, want the nested pattern alone", cell)
			}
			if p := nestedPattern(t, cell); p.VerticalAlign != "center" {
				t.Errorf("%s vertical_align = %q, want center", p.Name, p.VerticalAlign)
			}
			// The cycle keeps its own placement, on either side of the region.
			if p := nestedPattern(t, grid.Rows[0].Cells[0]); p.VerticalAlign != "" {
				t.Errorf("the cycle region's pattern carries vertical_align %q", p.VerticalAlign)
			}
			mirrored := compileBesideCycle(t, ArrangeColumns, besideCycleRegion(kind, false), besideCycleLoop())
			if p := nestedPattern(t, mirrored.Rows[0].Cells[0]); p.VerticalAlign != "center" {
				t.Errorf("left of the cycle: %s vertical_align = %q, want center", p.Name, p.VerticalAlign)
			}
		})
	}
}

// Under a heading the heading and the visual are one block centred in the
// column: a centred grid of the fixed heading row and a content row capped at
// the visual's band. A table is such a block with or without a heading.
func TestHeadedRegionBesideACycleIsOneCentredBlock(t *testing.T) {
	cases := []struct {
		kind    string
		heading bool
		bandPt  float64
	}{
		{RegionKPIs, true, regionKPIBandPt},
		{RegionStat, true, regionStatBandPt},
		{RegionTimeline, true, regionTimelineBandPt},
		{RegionTable, true, 3 * regionTableRowPt},
		{RegionTable, false, 3 * regionTableRowPt},
	}
	for _, tc := range cases {
		name := tc.kind
		if tc.heading {
			name += "/heading"
		}
		t.Run(name, func(t *testing.T) {
			grid := compileBesideCycle(t, ArrangeColumns, besideCycleLoop(), besideCycleRegion(tc.kind, tc.heading))
			block := grid.Rows[0].Cells[1].Grid
			if block == nil {
				t.Fatal("the region is not a nested grid")
			}
			if block.VerticalAlign != "center" {
				t.Errorf("block vertical_align = %q, want center", block.VerticalAlign)
			}
			wantRows := 1
			if tc.heading {
				wantRows = 2
				if h := block.Rows[0]; h.MinHeight != regionHeadingRowPt || h.MaxHeight != regionHeadingRowPt ||
					!strings.Contains(string(h.Cells[0].Shape.Text), "Since January") {
					t.Errorf("heading row = %+v", h)
				}
			}
			if len(block.Rows) != wantRows {
				t.Fatalf("block has %d rows, want %d", len(block.Rows), wantRows)
			}
			content := block.Rows[wantRows-1]
			if content.MaxHeight != tc.bandPt || content.Height != 0 {
				t.Errorf("content row max_height = %v (height %v), want the %vpt band", content.MaxHeight, content.Height, tc.bandPt)
			}
			if c := content.Cells[0]; len(c.Pattern) == 0 && c.Table == nil {
				t.Errorf("content cell = %+v, want the region's pattern or table", c)
			}
		})
	}
}

// A chart or an image fills the height of its column, so it keeps the rest of
// the cell under a top heading; and nothing changes for a region that is not
// beside a cycle, or that is stacked rather than side by side.
func TestRegionsThatFillOrStandElsewhereKeepTheirPlacement(t *testing.T) {
	for _, kind := range []string{RegionChart, RegionImage} {
		grid := compileBesideCycle(t, ArrangeColumns, besideCycleLoop(), besideCycleRegion(kind, true))
		block := grid.Rows[0].Cells[1].Grid
		if block == nil || block.VerticalAlign != "" || block.Rows[1].MaxHeight != 0 {
			t.Errorf("%s beside a cycle: block = %+v, want a top heading over a row that takes the rest", kind, block)
		}
		bare := compileBesideCycle(t, ArrangeColumns, besideCycleLoop(), besideCycleRegion(kind, false))
		if c := bare.Rows[0].Cells[1]; c.Grid != nil {
			t.Errorf("%s beside a cycle without a heading is wrapped: %+v", kind, c)
		}
	}

	chart := besideCycleRegion(RegionChart, false)
	chart["size_pct"] = 60.0
	for _, heading := range []bool{false, true} {
		grid := compileBesideCycle(t, ArrangeColumns, chart, besideCycleRegion(RegionKPIs, heading))
		cell := grid.Rows[0].Cells[1]
		if heading {
			if cell.Grid == nil || cell.Grid.VerticalAlign != "" || cell.Grid.Rows[1].MaxHeight != 0 {
				t.Errorf("kpis beside a chart under a heading: %+v", cell.Grid)
			}
			cell = cell.Grid.Rows[1].Cells[0]
		}
		if p := nestedPattern(t, cell); p.VerticalAlign != "" {
			t.Errorf("kpis beside a chart (heading %v) carry vertical_align %q", heading, p.VerticalAlign)
		}
	}

	rows := compileBesideCycle(t, ArrangeRows, map[string]any{"kind": "cycle", "phases": []any{"Plan", "Do", "Check", "Act"}}, besideCycleRegion(RegionKPIs, false))
	if p := nestedPattern(t, rows.Rows[1].Cells[0]); p.VerticalAlign != "" {
		t.Errorf("kpis under a cycle (rows) carry vertical_align %q", p.VerticalAlign)
	}
}

// The source links of a centred block still lead from the raw cell back to
// the region's fields.
func TestCentredRegionKeepsItsSourceLinks(t *testing.T) {
	slide, links, err := CompileRegions(Input{Title: "The loop", Body: map[string]any{"arrangement": ArrangeColumns,
		"regions": []any{besideCycleLoop(), besideCycleRegion(RegionTable, false), besideCycleRegion(RegionKPIs, true)}}})
	if err != nil || slide == nil {
		t.Fatalf("CompileRegions: %v", err)
	}
	want := map[string]string{
		".shape_grid.rows[0].cells[1].grid.rows[0].cells[0].table.rows":        ".regions[1].rows",
		".shape_grid.rows[0].cells[2].grid.rows[0].cells[0]":                   ".regions[2].heading",
		".shape_grid.rows[0].cells[2].grid.rows[1].cells[0].pattern.values[0]": ".regions[2].kpis[0]",
	}
	for raw, sem := range want {
		found := false
		for _, l := range links {
			if strings.HasSuffix(l.RawPath, raw) && strings.HasSuffix(l.SemanticPath, sem) {
				found = true
			}
		}
		if !found {
			t.Errorf("no link from …%s to …%s in %+v", raw, sem, links)
		}
	}
}
