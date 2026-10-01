package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// writeTemplateWithGrid copies a bundled template and sets its metadata
// "grid" block (go-slide-creator-5ms8c).
func writeTemplateWithGrid(t *testing.T, name string, grid map[string]any) string {
	t.Helper()
	src, err := zip.OpenReader(filepath.Join("..", "..", "templates", name+".pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	path := filepath.Join(t.TempDir(), name+"-grid.pptx")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	metadata := map[string]any{"version": "1.0"}
	for _, f := range src.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == template.MetadataFilePath {
			if err := json.Unmarshal(data, &metadata); err != nil {
				t.Fatal(err)
			}
			continue
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	metadata["grid"] = grid
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	w, err := zw.Create(template.MetadataFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func parseTemplateLayouts(t *testing.T, path string) ([]types.LayoutMetadata, int64, int64) {
	t.Helper()
	reader, err := template.OpenTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	w, h := template.ParseSlideDimensions(reader)
	return layouts, w, h
}

// The metadata grid reaches every geometry path through the parsed layouts:
// margin narrows the zone, gutter is the default grid gap in both the
// generation and the preflight resolver, title gap moves the measured-title
// zone top, and an undeclared template keeps the engine constants.
func TestTemplateGridMetadataReachesEngine(t *testing.T) {
	plain, w, h := parseTemplateLayouts(t, filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if types.TemplateGridOf(plain) != nil {
		t.Fatal("bundled midnight-blue unexpectedly declares a grid")
	}
	gridded, _, _ := parseTemplateLayouts(t, writeTemplateWithGrid(t, "midnight-blue", map[string]any{
		"margin_pct": 10, "columns": 6, "gutter_pt": 20, "title_gap_pt": 40,
	}))
	g := types.TemplateGridOf(gridded)
	if g == nil || g.MarginPct != 10 || g.Columns != 6 || g.GutterPt != 20 || g.TitleGapPt != 40 {
		t.Fatalf("parsed grid = %+v", g)
	}

	title := "Two columns"
	slide := SlideInput{
		LayoutID: "content",
		Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		ShapeGrid: &ShapeGridInput{
			Columns: json.RawMessage(`2`),
			Rows: []GridRowInput{{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
			}}},
		},
	}
	base := resolveGridGeometry(slide, plain, w, h)
	geom := resolveGridGeometry(slide, gridded, w, h)
	margin := w / 10
	if geom.Zone.LeftMargin < margin || geom.Zone.RightEdge > w-margin {
		t.Errorf("zone %d..%d not narrowed to the 10%% margin (%d..%d)", geom.Zone.LeftMargin, geom.Zone.RightEdge, margin, w-margin)
	}
	if base.Zone.GutterPt != 0 || geom.Zone.GutterPt != 20 {
		t.Errorf("zone gutter = %g (plain %g), want 20 (plain 0)", geom.Zone.GutterPt, base.Zone.GutterPt)
	}
	// title_gap_pt replaces the 18pt measured-title-to-body gap.
	probe := &types.PlaceholderInfo{Anchor: "t", FontSize: 2800, FontFamily: "Arial", Bounds: types.BoundingBox{Y: 400000, Width: 8000000, Height: 1300000}}
	def, ok1 := measuredTitleBottom(probe, "Two columns", types.TemplateGridOf(plain).TitleGapPtOrDefault())
	wide, ok2 := measuredTitleBottom(probe, "Two columns", g.TitleGapPtOrDefault())
	if !ok1 || !ok2 {
		t.Skip("no font metrics for the title measurement")
	}
	if d := float64(wide-def) / 12700; d < 21.9 || d > 22.1 {
		t.Errorf("title gap 40pt moved the measured title edge by %.1fpt, want 22pt", d)
	}

	gap := func(name string, cells []float64) {
		if len(cells) != 2 {
			t.Fatalf("%s: %d cells", name, len(cells))
		}
		if d := cells[1] / 12700; d < 19.9 || d > 20.1 {
			t.Errorf("%s: column gap %.1fpt, want the 20pt template gutter", name, d)
		}
	}
	res := resolveGridForStructural(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, w, h)
	if res == nil {
		t.Fatal("preflight grid did not resolve")
	}
	gap("preflight", []float64{0, float64(res.Cells[1].Bounds.X - res.Cells[0].Bounds.X - res.Cells[0].Bounds.CX)})
	gen, err := resolveShapeGrid(slide.ShapeGrid, pptx.NewShapeIDAllocator(nil), geom.OverrideBounds, geom.Zone, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	gap("generation", []float64{0, float64(gen.Cells[1].Bounds.X - gen.Cells[0].Bounds.X - gen.Cells[0].Bounds.CX)})

	resolved := template.ResolveTemplateGrid(gridded, w, h)
	if strings.Join(resolved.Declared, ",") != "margin_pct,columns,gutter_pt,title_gap_pt" || resolved.Frame == nil || resolved.Frame.LeftEMU < margin {
		t.Errorf("resolved grid = %+v", resolved)
	}
	defaults := template.ResolveTemplateGrid(plain, w, h)
	if len(defaults.Declared) != 0 || defaults.Columns != types.DefaultGridColumns || defaults.GutterPt != types.DefaultGridGutterPt || defaults.TitleGapPt != types.DefaultGridTitleGapPt {
		t.Errorf("default grid = %+v", defaults)
	}
}

// Out-of-range grid values are reported by validate-template and ignored.
func TestTemplateGridInvalidValuesAreReportedAndIgnored(t *testing.T) {
	path := writeTemplateWithGrid(t, "midnight-blue", map[string]any{"gutter_pt": -4, "columns": 12})
	reader, err := template.OpenTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	vr := template.ValidateTemplateMetadata(reader, false)
	found := false
	for _, d := range vr.Diagnostics {
		if d.Code == diagnostics.CodeTemplateGridInvalid {
			found = true
		}
	}
	if !found {
		t.Errorf("no %s diagnostic for gutter_pt -4: %+v", diagnostics.CodeTemplateGridInvalid, vr.Diagnostics)
	}
	layouts, _, _ := parseTemplateLayouts(t, path)
	if g := types.TemplateGridOf(layouts); g == nil || g.GutterPt != 0 || g.Columns != 12 {
		t.Errorf("sanitized grid = %+v, want gutter dropped and columns kept", g)
	}
}

// grid_violation runs by default against the template's content frame, at
// info weight: an off-frame block is reported, default pattern placement and
// title / section slides are not.
func TestGridViolationDefaultsToTemplateFrame(t *testing.T) {
	tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tctx.reader.Close() }()
	title := "Results so far"
	in := geomSlides(t, `[
{"layout_id":"content","pattern":{"name":"kpi-3up","values":[{"big":"42%","small":"Faster resolution"},{"big":"$3.1M","small":"Annual savings"},{"big":"4.6","small":"CSAT score"}]}},
{"layout_id":"content","shape_grid":{"bounds":{"x":30,"y":45,"width":50,"height":30},"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","fill":"accent1","text":"Off the frame"}}]}]}},
{"slide_type":"title","layout_id":"title","shape_grid":{"bounds":{"x":30,"y":45,"width":50,"height":30},"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","fill":"accent1","text":"Cover art"}}]}]}}
]`)
	for i := range in.Slides {
		in.Slides[i].Content = append([]ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}, in.Slides[i].Content...)
	}
	var got []patterns.FitFinding
	for _, f := range collectFitFindings(in, tctx.layouts, tctx.slideWidth, tctx.slideHeight, nil) {
		if f.Code == "grid_violation" {
			got = append(got, f)
		}
	}
	fields := map[string]bool{}
	for _, f := range got {
		if !strings.HasPrefix(f.Path, "/slides/1/") {
			t.Errorf("unexpected grid_violation at %s: %s", f.Path, f.Message)
			continue
		}
		if f.Action != "info" || f.Fix == nil || f.Fix.Params["frame"] != "template" {
			t.Errorf("default grid_violation must be an info finding against the template frame: %+v", f)
			continue
		}
		fields[f.Fix.Params["field"].(string)] = true
	}
	if !fields["content_top"] || !fields["left_margin"] {
		t.Errorf("off-frame block not reported on content_top and left_margin: %+v", got)
	}
}
