package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// iconDiscGrid is a one-cell grid whose contained cell stacks two discs: a
// neutral one carrying an icon (layer 0) and a second, textless one.
func iconDiscGrid(icon *IconInput) *jsonschema.ShapeGridInput {
	disc := func(x float64, ic *IconInput) jsonschema.LayerInput {
		return jsonschema.LayerInput{
			Frame: jsonschema.LayerFrameInput{X: x, Y: 0.3, W: 0.3, H: 0.3},
			Shape: &ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`), Line: json.RawMessage(`"none"`), Icon: ic},
		}
	}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{
			Fit:    "contain",
			Layers: []jsonschema.LayerInput{disc(0.1, icon), disc(0.6, nil)},
		}}}},
	}
}

func iconLayerCell(t *testing.T, cells []shapegrid.ResolvedCell) shapegrid.ResolvedCell {
	t.Helper()
	for _, c := range cells {
		if c.Layer && c.IconSpec != nil {
			return c
		}
	}
	t.Fatalf("no resolved layer carries an icon: %+v", cells)
	return shapegrid.ResolvedCell{}
}

// A layer shape's icon is written as one native SVG picture, centred in the
// layer's frame and painted in the template's colour for its scheme fill.
func TestLayerIconIsWrittenInTheLayerFrame(t *testing.T) {
	theme := &GridDiagramContext{ThemeColors: []types.ThemeColor{
		{Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk1", RGB: "#000000"}, {Name: "lt2", RGB: "#EEEEEE"}, {Name: "accent1", RGB: "#1F4E79"},
	}}
	grid := iconDiscGrid(&IconInput{Name: "shield", Fill: "accent1"})
	res, err := resolveShapeGrid(grid, newAllocFrom(200), nil, nil, 12192000, 6858000, theme)
	if err != nil || res == nil {
		t.Fatalf("resolveShapeGrid: %v", err)
	}
	if len(res.Shapes) != 2 {
		t.Errorf("wrote %d shapes, want the two discs (an icon is a picture, not a shape)", len(res.Shapes))
	}
	if len(res.IconInserts) != 1 {
		t.Fatalf("wrote %d icon pictures, want the layer's one", len(res.IconInserts))
	}
	layer, ins := iconLayerCell(t, res.Cells), res.IconInserts[0]
	if ins.ExtentCX != ins.ExtentCY || ins.ExtentCX <= 0 {
		t.Errorf("icon is %d x %d EMU, want a square", ins.ExtentCX, ins.ExtentCY)
	}
	if dx, dy := (ins.OffsetX+ins.ExtentCX/2)-(layer.Bounds.X+layer.Bounds.CX/2), (ins.OffsetY+ins.ExtentCY/2)-(layer.Bounds.Y+layer.Bounds.CY/2); dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		t.Errorf("icon centre is (%d, %d) EMU off its layer's", dx, dy)
	}
	if ins.ExtentCX >= layer.Bounds.CX {
		t.Errorf("icon (%d EMU) is not smaller than its disc (%d EMU)", ins.ExtentCX, layer.Bounds.CX)
	}
	svg := string(ins.SVGData)
	if !strings.Contains(strings.ToUpper(svg), "#1F4E79") {
		t.Errorf("icon SVG is not painted in the theme's accent1 (#1F4E79):\n%.300s", svg)
	}
	if strings.Contains(svg, "accent1") {
		t.Errorf("icon SVG carries the scheme name, which is not an SVG paint")
	}
	if ins.Alt == "" {
		t.Error("icon picture has no alt text")
	}

	// The shapes-only pass (validate-time contrast prediction) writes the same
	// shapes and no picture.
	only, err := resolveShapeGrid(iconDiscGrid(&IconInput{Name: "shield", Fill: "accent1"}), newAllocFrom(200), nil, nil, 12192000, 6858000, &GridDiagramContext{ShapesOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(only.IconInserts) != 0 || len(only.Shapes) != len(res.Shapes) {
		t.Errorf("shapes-only pass wrote %d pictures and %d shapes, want 0 and %d", len(only.IconInserts), len(only.Shapes), len(res.Shapes))
	}
}

// The icon follows its layer into a nested grid cell and a grouped cell.
func TestLayerIconFollowsItsCellIntoNestedGrids(t *testing.T) {
	outer := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[50, 50]`),
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			{Grid: textGrid("Why it matters")},
			{Grid: iconDiscGrid(&IconInput{Name: "shield"})},
		}}},
	}
	res, err := resolveShapeGrid(outer, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
	if err != nil || res == nil {
		t.Fatalf("resolveShapeGrid: %v", err)
	}
	if len(res.IconInserts) != 1 {
		t.Fatalf("nested grid wrote %d icon pictures, want 1", len(res.IconInserts))
	}
	if ins := res.IconInserts[0]; ins.OffsetX < 12192000/2 {
		t.Errorf("icon at x=%d EMU is not in the right-hand nested cell", ins.OffsetX)
	}

	grouped := iconDiscGrid(&IconInput{Name: "shield"})
	grouped.Rows[0].Cells[0].Group = true
	res, err = resolveShapeGrid(grouped, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
	if err != nil || res == nil {
		t.Fatalf("resolveShapeGrid (grouped): %v", err)
	}
	if len(res.IconInserts) != 1 || len(res.Shapes) != 1 {
		t.Errorf("grouped cell wrote %d pictures and %d top-level shapes, want 1 picture beside 1 group", len(res.IconInserts), len(res.Shapes))
	}
}

// A layer icon's source is checked like a cell's: an unknown bundled name, a
// non-SVG path and two sources at once are reported at the layer's icon, in
// the slide's own grid and in a nested one.
func TestLayerIconSourceIsValidatedAtTheLayer(t *testing.T) {
	slides := []SlideInput{
		{ShapeGrid: iconDiscGrid(&IconInput{Name: "sheild"})},
		{ShapeGrid: &jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			{Grid: iconDiscGrid(&IconInput{Path: "logo.png"})},
		}}}}},
		{ShapeGrid: iconDiscGrid(&IconInput{Name: "shield", Path: "logo.svg"})},
		{ShapeGrid: iconDiscGrid(&IconInput{Name: "shield"})},
	}
	got := map[string]diagnostics.Code{}
	for _, d := range resolveLocalAssetPaths(slides, t.TempDir()) {
		got[d.Path] = d.Code
	}
	want := map[string]diagnostics.Code{
		"/slides/0/shape_grid/rows/0/cells/0/layers/0/shape/icon":                     diagnostics.CodeIconBundledNameUnknown,
		"/slides/1/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/layers/0/shape/icon": diagnostics.CodeIconPathExtInvalid,
	}
	for path, code := range want {
		if got[path] != code {
			t.Errorf("%s: code = %q, want %s (all: %v)", path, got[path], code, got)
		}
	}
	if _, ok := got["/slides/2/shape_grid/rows/0/cells/0/layers/0/shape/icon"]; !ok {
		t.Errorf("an icon with both name and path is not reported at the layer: %v", got)
	}
	for path := range got {
		if strings.HasPrefix(path, "/slides/3/") {
			t.Errorf("a valid layer icon is reported: %s", path)
		}
	}
}

// A relative icon path on a layer is resolved against the deck's directory.
func TestLayerIconPathIsResolvedAgainstTheDeck(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mark.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`), 0o600); err != nil {
		t.Fatal(err)
	}
	icon := &IconInput{Path: "mark.svg", Alt: "Company mark"}
	slides := []SlideInput{{ShapeGrid: iconDiscGrid(icon)}}
	if diags := resolveLocalAssetPaths(slides, dir); len(diags) != 0 {
		t.Fatalf("diagnostics for a valid icon path: %+v", diags)
	}
	if !filepath.IsAbs(icon.Path) {
		t.Errorf("icon path %q was not made absolute", icon.Path)
	}
}

// A layer icon's URL is seen by the URL resolver, and a pathed icon without
// alt text gets the alt-text finding at the layer.
func TestLayerIconURLAndAltTextAreSeen(t *testing.T) {
	grid := iconDiscGrid(&IconInput{URL: "https://example.com/mark.svg"})
	if !gridHasURLReferences(grid) {
		t.Error("gridHasURLReferences misses a layer icon's url")
	}
	if gridHasURLReferences(iconDiscGrid(&IconInput{Name: "shield"})) {
		t.Error("gridHasURLReferences reports a bundled layer icon")
	}

	const want = "/slides/0/shape_grid/rows/0/cells/0/layers/0/shape/icon"
	found := false
	for _, f := range cellAltFindingsAt(0, grid.Rows[0].Cells[0], true, "/slides/0/shape_grid/rows/0/cells/0") {
		if f.Path == want {
			found = true
		}
	}
	if !found {
		t.Errorf("no alt-text finding at %s for a layer icon loaded from a url", want)
	}
	if f := cellAltFindingsAt(0, iconDiscGrid(&IconInput{Name: "shield"}).Rows[0].Cells[0], true, "/c"); len(f) != 0 {
		t.Errorf("a bundled layer icon needs no alt text, got %+v", f)
	}
}

// A hex fill on a layer icon breaks constrained design mode like any other
// colour, and a misspelled icon key is reported instead of being dropped.
func TestLayerIconColourAndKeysAreChecked(t *testing.T) {
	grid := iconDiscGrid(&IconInput{Name: "shield", Fill: "#FF0000"})
	const fillPath = "/slides/0/shape_grid/rows/0/cells/0/layers/0/shape/icon/fill"
	found := false
	for _, f := range checkShapeGridAt(grid, 1, "/slides/0/shape_grid", nil, false) {
		found = found || f.Path == fillPath
	}
	if !found {
		t.Errorf("design mode did not report the hex icon fill at %s", fillPath)
	}
	for _, f := range checkShapeGridAt(iconDiscGrid(&IconInput{Name: "shield", Fill: "accent1"}), 1, "/slides/0/shape_grid", nil, false) {
		if strings.Contains(f.Path, "/icon") {
			t.Errorf("a scheme icon fill is reported: %+v", f)
		}
	}

	raw := json.RawMessage(`{"template":"t","slides":[{"shape_grid":{"rows":[{"cells":[{"layers":[
		{"frame":{"x":0,"y":0,"w":1,"h":1},"shape":{"geometry":"ellipse","icon":{"name":"shield","colour":"accent1","scale":0.5}}}
	]}]}]}}]}`)
	got := map[string]bool{}
	for _, w := range checkInputUnknownKeys(raw) {
		got[w.Path] = true
	}
	base := "/slides/0/shape_grid/rows/0/cells/0/layers/0/shape"
	if !got[base+"/icon/colour"] {
		t.Errorf("unknown key %s/icon/colour not reported; got %v", base, got)
	}
	for _, ok := range []string{"/icon", "/icon/name", "/icon/scale"} {
		if got[base+ok] {
			t.Errorf("working key %s%s reported as unknown", base, ok)
		}
	}
}

// End to end: a deck with an icon on a layer is valid on every surface and
// the generated slide embeds the icon as an SVG picture.
func TestLayerIconDeckGeneratesWithTheIconEmbedded(t *testing.T) {
	slide := `{"slide_type": "blank", "content": [
	  {"placeholder_id": "title", "type": "text", "text_value": "Six capabilities surround the platform"}],
	  "shape_grid": {"columns": 1, "rows": [{"cells": [{"fit": "contain", "layers": [
	    {"name": "disc", "frame": {"x": 0.3, "y": 0.3, "w": 0.4, "h": 0.4},
	     "shape": {"geometry": "ellipse", "fill": "lt2", "line": "none", "icon": {"name": "shield", "fill": "dk1"}}}
	  ]}]}]}}`
	mc := testMCPConfig(t)
	path, _ := writeLayerDeck(t, slide)
	if verdict := assertOneVerdict(t, verdictSurfaces(t, mc, path, true)); !verdict.Valid {
		t.Fatalf("verdict = %s, want a layer with an icon valid", verdict)
	}

	outDir := t.TempDir()
	if err := runJSONMode(path, "", testutil.TemplatesDir(), outDir, "", false, false, "", "warn", false, "strict", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(outDir, "*.pptx"))
	if len(matches) != 1 {
		t.Fatalf("generate wrote %d decks, want 1", len(matches))
	}
	zr, err := zip.OpenReader(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	svgs := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/media/") && strings.HasSuffix(f.Name, ".svg") {
			svgs++
		}
	}
	if svgs != 1 {
		t.Errorf("the deck embeds %d SVG pictures, want the layer's icon", svgs)
	}
}
