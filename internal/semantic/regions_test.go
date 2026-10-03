package semantic

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// Regions slides (go-slide-creator-fn2ka): one title over 2–3 typed regions,
// compiled onto one shape grid at the authored proportions.

func regionsChart() map[string]any {
	return map[string]any{"type": "line_chart", "data": map[string]any{
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series":     []any{map[string]any{"name": "Revenue", "values": []any{12, 14, 17, 21}}},
	}}
}

func regionsSpec(body map[string]any) *DeckSpec {
	body["title"] = "Growth funds the launch"
	return &DeckSpec{
		Meta:   DeckMeta{Title: "Launch review", Template: "midnight-blue", Source: "Finance ledger"},
		Slides: []SlideSpec{{Kind: KindRegions, Body: body}},
	}
}

func chartStatTimeline() map[string]any {
	return map[string]any{
		"arrangement": "main_left",
		"regions": []any{
			map[string]any{"kind": "chart", "size_pct": 65, "heading": "Quarterly revenue", "unit": "€m", "chart": regionsChart(), "source": "Ledger"},
			map[string]any{"kind": "stat", "size_pct": 40, "value": "32%", "label": "Gross margin", "context": "Up 4 points"},
			map[string]any{"kind": "timeline", "size_pct": 60, "milestones": []any{
				map[string]any{"label": "Design", "date": "Oct"},
				map[string]any{"label": "Pilot", "date": "Nov"},
				map[string]any{"label": "Rollout", "date": "Dec"},
			}},
		},
		"takeaway": "The margin pays for the rollout.",
	}
}

func compileRegionsOK(t *testing.T, body map[string]any) (*deckinput.SlideInput, *CompileResult) {
	t.Helper()
	input, res, err := Compile(regionsSpec(body), CompileOptions{Strict: StrictnessStrict})
	if err != nil {
		t.Fatalf("compile: %v (diagnostics %v)", err, res.Diagnostics)
	}
	if len(input.Slides) != 1 {
		t.Fatalf("want one slide, got %d", len(input.Slides))
	}
	return &input.Slides[0], res
}

func cellPattern(t *testing.T, cell *deckinput.GridCellInput) deckinput.PatternInput {
	t.Helper()
	var p deckinput.PatternInput
	if cell == nil || len(cell.Pattern) == 0 {
		t.Fatalf("cell has no nested pattern: %+v", cell)
	}
	if err := json.Unmarshal(cell.Pattern, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// The chart-left, stat-over-timeline slide compiles to one grid whose columns
// and stacked rows are the authored shares, with every label, unit and source
// kept.
func TestCompileRegions_MainLeftPlacement(t *testing.T) {
	slide, res := compileRegionsOK(t, chartStatTimeline())
	if slide.LayoutID != "blank-title" || slide.Pattern != nil || slide.Compose != nil {
		t.Fatalf("want a blank-title shape-grid slide, got layout %q pattern %v", slide.LayoutID, slide.Pattern)
	}
	g := slide.ShapeGrid
	if g == nil || string(g.Columns) != "[65,35]" || len(g.Rows) != 1 || len(g.Rows[0].Cells) != 2 {
		t.Fatalf("want columns [65,35] in one row, got %s / %+v", g.Columns, g)
	}
	chartCell := g.Rows[0].Cells[0]
	if chartCell.Grid == nil || len(chartCell.Grid.Rows) != 2 {
		t.Fatalf("chart region with a heading wants a heading row over the chart, got %+v", chartCell)
	}
	heading, _ := json.Marshal(chartCell.Grid.Rows[0].Cells[0].Shape.Text)
	if !strings.Contains(string(heading), "Quarterly revenue (€m)") {
		t.Errorf("heading row lost the heading or unit: %s", heading)
	}
	if d := chartCell.Grid.Rows[1].Cells[0].Diagram; d == nil || d.Type != "line_chart" {
		t.Fatalf("chart cell lost its diagram: %+v", chartCell.Grid.Rows[1].Cells[0])
	}
	side := g.Rows[0].Cells[1].Grid
	if side == nil || len(side.Rows) != 2 || side.Rows[0].Height != 40 || side.Rows[1].Height != 60 {
		t.Fatalf("want the stack split 40/60, got %+v", side)
	}
	stat := cellPattern(t, side.Rows[0].Cells[0])
	if stat.Name != "stat-hero" || !strings.Contains(string(stat.Values), "Up 4 points") || !strings.Contains(string(stat.Values), "Gross margin") {
		t.Errorf("stat region: %s %s", stat.Name, stat.Values)
	}
	tl := cellPattern(t, side.Rows[1].Cells[0])
	if tl.Name != "timeline-horizontal" || strings.Count(string(tl.Values), `"label"`) != 3 {
		t.Errorf("timeline region: %s %s", tl.Name, tl.Values)
	}
	if slide.Source != "Ledger" {
		t.Errorf("region source should become the slide source, got %q", slide.Source)
	}
	if slide.Takeaway == "" || len(slide.Content) == 0 || slide.Content[0].PlaceholderID != "title" {
		t.Errorf("one shared title and takeaway expected, got %+v / %q", slide.Content, slide.Takeaway)
	}

	// Findings anywhere inside a region resolve to that region, in either
	// path spelling.
	for raw, want := range map[string]string{
		"/slides/0/shape_grid/rows/0/cells/1/grid/rows/1/cells/0/pattern/values/2/label":   "slides[0].regions[2].milestones",
		"/slides/0/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/rendered":                 "slides[0].regions[1]",
		"slides[0].shape_grid.rows[0].cells[1].grid.rows[0].cells[0].pattern.values.value": "slides[0].regions[1].value",
		"/slides/0/shape_grid/rows/0/cells/0/grid/rows/1/cells/0/diagram":                  "slides[0].regions[0].chart",
		"/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/shape/text":               "slides[0].regions[0].heading",
	} {
		if got, _, _ := res.SourceMap.ResolveSemantic(raw); got != want {
			t.Errorf("ResolveSemantic(%s) = %q, want %q", raw, got, want)
		}
	}
}

// The chart + table + narrative case, and every other arrangement, lands each
// region where the arrangement says.
func TestCompileRegions_Arrangements(t *testing.T) {
	table := map[string]any{"kind": "table", "headers": []any{"Segment", "Revenue"}, "rows": []any{[]any{"Enterprise", "€14m"}, []any{"SMB", "€7m"}}}
	text := map[string]any{"kind": "text", "body": "SMB churn rose to 3.1%.", "bullets": []any{"Fund a retention pod"}}
	chart := map[string]any{"kind": "chart", "chart": regionsChart()}
	cases := []struct {
		arrangement string
		check       func(t *testing.T, g *deckinput.ShapeGridInput)
	}{
		{"columns", func(t *testing.T, g *deckinput.ShapeGridInput) {
			if len(g.Rows) != 1 || len(g.Rows[0].Cells) != 3 || g.Rows[0].Cells[1].Table == nil {
				t.Fatalf("columns: %+v", g)
			}
		}},
		{"rows", func(t *testing.T, g *deckinput.ShapeGridInput) {
			if len(g.Rows) != 3 || g.Rows[2].Cells[0].Shape == nil {
				t.Fatalf("rows: %+v", g)
			}
		}},
		{"main_right", func(t *testing.T, g *deckinput.ShapeGridInput) {
			if g.Rows[0].Cells[1].Diagram == nil || g.Rows[0].Cells[0].Grid.Rows[0].Cells[0].Table == nil {
				t.Fatalf("main_right should put the chart right of the stack: %+v", g.Rows[0].Cells)
			}
		}},
		{"main_top", func(t *testing.T, g *deckinput.ShapeGridInput) {
			if len(g.Rows) != 2 || g.Rows[0].Cells[0].Diagram == nil || g.Rows[0].Height != 55 || g.Rows[1].Cells[0].Grid.Rows[0].Cells[1].Shape == nil {
				t.Fatalf("main_top: %+v", g)
			}
		}},
		{"main_bottom", func(t *testing.T, g *deckinput.ShapeGridInput) {
			// The band's two-row table needs 45% of the height, so the unset
			// main share gives up 5 of its default 60 (go-slide-creator-umev3).
			if g.Rows[1].Cells[0].Diagram == nil || g.Rows[1].Height != 55 || g.Rows[0].Height != 45 {
				t.Fatalf("main_bottom: %+v", g)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.arrangement, func(t *testing.T) {
			body := map[string]any{"arrangement": tc.arrangement, "takeaway": "Enterprise doubled.",
				"regions": []any{deepCopyValue(chart), deepCopyValue(table), deepCopyValue(text)}}
			slide, _ := compileRegionsOK(t, body)
			tc.check(t, slide.ShapeGrid)
			encoded, _ := json.Marshal(slide)
			for _, want := range []string{"Enterprise", "€7m", "SMB churn rose to 3.1%.", "Fund a retention pod", "line_chart"} {
				if !strings.Contains(string(encoded), want) {
					t.Errorf("compiled slide lost %q", want)
				}
			}
			// meta.source is applied by the raw deck-source default, which
			// reads grid cells; cmd/json2pptx pins it for regions slides.
			if slide.Source != "" {
				t.Errorf("compile should leave meta.source to the deck-source default, got slide source %q", slide.Source)
			}
		})
	}
}

// Every region rule fails before rendering, at the region field to edit.
func TestValidateRegions_ChildPaths(t *testing.T) {
	cases := []struct {
		name string
		edit func(body map[string]any)
		code string
		path string
	}{
		{"unknown region field", func(b map[string]any) { region(b, 1)["colour"] = "red" }, diagnostics.CodeSemanticUnknownField, "slides[0].regions[1].colour"},
		{"unknown chart key", func(b map[string]any) { region(b, 0)["chart"].(map[string]any)["series"] = []any{} }, diagnostics.CodeSemanticUnknownField, "slides[0].regions[0].chart.series"},
		{"unknown milestone key", func(b map[string]any) {
			region(b, 2)["milestones"].([]any)[0].(map[string]any)["owner"] = "Ana"
		}, diagnostics.CodeSemanticUnknownField, "slides[0].regions[2].milestones[0].owner"},
		{"chart without data", func(b map[string]any) { delete(region(b, 0)["chart"].(map[string]any), "data") }, diagnostics.CodeSemanticRequired, "slides[0].regions[0].chart.data"},
		{"chart series mismatch", func(b map[string]any) {
			region(b, 0)["chart"].(map[string]any)["data"].(map[string]any)["series"].([]any)[0].(map[string]any)["values"] = []any{1, 2}
		}, diagnostics.CodeChartSeriesLengthMismatch, "slides[0].regions[0].chart.data.series[0].values"},
		{"stat over budget", func(b map[string]any) { region(b, 1)["value"] = "EUR 1,186.42 million and more" }, diagnostics.CodeSemanticDensity, "slides[0].regions[1].value"},
		{"timeline too short", func(b map[string]any) {
			region(b, 2)["milestones"] = []any{"Design", "Pilot"}
		}, diagnostics.CodeSemanticDensity, "slides[0].regions[2].milestones"},
		{"unknown region kind", func(b map[string]any) { region(b, 1)["kind"] = "gauge" }, diagnostics.CodeSemanticUnknownKind, "slides[0].regions[1].kind"},
		{"size out of range", func(b map[string]any) { region(b, 0)["size_pct"] = 90 }, diagnostics.CodeSemanticDensity, "slides[0].regions[0].size_pct"},
		{"stack shares do not sum", func(b map[string]any) { region(b, 2)["size_pct"] = 70 }, diagnostics.CodeSemanticDensity, "slides[0].regions"},
		{"main arrangement needs three", func(b map[string]any) { b["regions"] = b["regions"].([]any)[:2] }, diagnostics.CodeSemanticDensity, "slides[0].regions"},
		{"unknown arrangement", func(b map[string]any) { b["arrangement"] = "grid" }, diagnostics.CodeSemanticFieldType, "slides[0].arrangement"},
		{"unknown slide field", func(b map[string]any) { b["compose"] = map[string]any{} }, diagnostics.CodeSemanticUnknownField, "slides[0].compose"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := chartStatTimeline()
			tc.edit(body)
			spec := regionsSpec(body)
			ds := Validate(spec, StrictnessWarn)
			found := false
			for _, d := range ds {
				if d.Code == tc.code && d.Path == tc.path && d.Severity == diagnostics.SeverityError {
					found = true
				}
			}
			if !found {
				t.Fatalf("want %s error at %s, got %v", tc.code, tc.path, ds)
			}
			if _, _, err := Compile(spec, CompileOptions{}); err == nil {
				t.Fatal("a regions slide with a region error must not compile")
			}
		})
	}
}

func region(body map[string]any, i int) map[string]any {
	return body["regions"].([]any)[i].(map[string]any)
}

// A nested pattern the raw gate refuses is reported at the region that wrote
// it, not at the slide.
func TestPreflightRegions_NestedPatternMapsToRegion(t *testing.T) {
	spec := regionsSpec(chartStatTimeline())
	input, res, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stat := input.Slides[0].ShapeGrid.Rows[0].Cells[1].Grid.Rows[0].Cells[0]
	stat.Pattern = json.RawMessage(`{"name":"stat-hero","values":{"value":"","label":"Gross margin"}}`)
	ds := preflightRawPatterns(input, res.SourceMap)
	if len(ds) == 0 {
		t.Fatal("an empty stat-hero value must fail the nested preflight")
	}
	if !strings.HasPrefix(ds[0].Path, "slides[0].regions[1]") {
		t.Errorf("nested preflight path = %q, want it under slides[0].regions[1]", ds[0].Path)
	}
}

// The kind's live contract: closed region variants in the schema, and the
// copy-ready example plus the bundled fixture validate clean under strict.
func TestRegionsContract(t *testing.T) {
	item := KindItemSchema(KindRegions)
	props := item["properties"].(map[string]any)
	regions := props["regions"].(map[string]any)
	variants := regions["items"].(map[string]any)["oneOf"].([]any)
	if len(variants) != 7 || regions["maxItems"] != 3 {
		t.Fatalf("want 7 closed region variants and maxItems 3, got %d / %v", len(variants), regions["maxItems"])
	}
	for _, v := range variants {
		if v.(map[string]any)["additionalProperties"] != false {
			t.Errorf("region variant %v is not closed", v.(map[string]any)["title"])
		}
	}
	if arr := props["arrangement"].(map[string]any); len(arr["enum"].([]any)) != 6 {
		t.Errorf("arrangement enum = %v", arr["enum"])
	}

	data, err := os.ReadFile("../../examples/semantic/regions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if ds := Check("regions.yaml", data, StrictnessStrict); diagnostics.HasErrors(ds) {
		t.Fatalf("examples/semantic/regions.yaml must validate clean under strict: %v", ds)
	}
	spec, _ := Parse("regions.yaml", data)
	input, _, err := Compile(spec, CompileOptions{Strict: StrictnessStrict})
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Slides) != 3 || input.Slides[1].ShapeGrid == nil || input.Slides[2].ShapeGrid == nil {
		t.Fatalf("fixture should compile to title + two region grids, got %d slides", len(input.Slides))
	}
}

// Shares the author left unset split by region kind: a stat takes less than
// the timeline stacked under it (go-slide-creator-vae7f).
func TestCompileRegions_UnsetSharesByKind(t *testing.T) {
	body := chartStatTimeline()
	for _, r := range body["regions"].([]any) {
		delete(r.(map[string]any), "size_pct")
	}
	slide, _ := compileRegionsOK(t, body)
	g := slide.ShapeGrid
	side := g.Rows[0].Cells[1].Grid
	if string(g.Columns) != "[60,40]" || side.Rows[0].Height != 40 || side.Rows[1].Height != 60 {
		t.Fatalf("want columns [60,40] and a 40/60 stat/timeline stack, got %s / %g/%g", g.Columns, side.Rows[0].Height, side.Rows[1].Height)
	}
}

// A regions slide counts as its main region's visual family, and as data
// evidence when it shows figures.
func TestNormalizeRegionsFamily(t *testing.T) {
	ir := Normalize(regionsSpec(chartStatTimeline()))
	if got := ir.Slides[0].Visual.Family; got != FamilyChart {
		t.Errorf("chart-led regions family = %q", got)
	}
	if !isDataBearingEvidence(ir.Slides[0]) {
		t.Error("a regions slide with a chart is data evidence")
	}
	body := chartStatTimeline()
	regions := body["regions"].([]any)
	regions[0], regions[2] = regions[2], regions[0]
	ir = Normalize(regionsSpec(body))
	if got := ir.Slides[0].Visual.Family; got != FamilyTimeline {
		t.Errorf("timeline-led regions family = %q", got)
	}
}
