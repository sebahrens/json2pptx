package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// layeredSlide is a slide whose grid has one contained cell: a ring segment
// (layer 0) and a small badge (layer 1) that carries badgeText.
func layeredSlide(badgeText string) string {
	text, _ := json.Marshal(badgeText)
	return `{"slide_type": "blank", "content": [
	  {"placeholder_id": "title", "type": "text", "text_value": "Delivery runs in four phases"}],
	  "shape_grid": {"columns": 1, "rows": [{"cells": [{"fit": "contain", "layers": [
	    {"name": "segment-1", "frame": {"x": 0, "y": 0, "w": 1, "h": 1},
	     "shape": {"geometry": "blockArc", "fill": "accent1", "line": "none",
	               "adjustments": {"adj1": 16200000, "adj2": 0, "adj3": 20000}}},
	    {"name": "badge-1", "frame": {"x": 0.72, "y": 0.06, "w": 0.12, "h": 0.12},
	     "shape": {"geometry": "ellipse", "fill": "dk2", "line": "none",
	               "text": {"content": ` + string(text) + `, "color": "lt1", "align": "ctr", "vertical_align": "ctr"}}}
	  ]}]}]}}`
}

const layerBadgePath = "/slides/1/shape_grid/rows/0/cells/0/layers/1"

func writeLayerDeck(t *testing.T, slide string) (string, any) {
	t.Helper()
	deck := map[string]any{"template": "midnight-blue", "slides": []any{verdictJSON(verdictCover), verdictJSON(slide)}}
	data, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "layers.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, deck
}

// A layer whose text cannot be read is refused by every surface, at the
// layer's own path — validate and generate give one verdict.
func TestLayerTextOverflowHasOneVerdictAtTheLayerPath(t *testing.T) {
	mc := testMCPConfig(t)
	path, _ := writeLayerDeck(t, layeredSlide("A badge label that is far too long to sit in a small circle on the ring"))
	verdict := assertOneVerdict(t, verdictSurfaces(t, mc, path, true))
	if verdict.Valid {
		t.Fatalf("verdict = %s, want the unreadable layer text refused", verdict)
	}
	found := false
	for _, e := range verdict.Errors {
		if strings.Contains(e, "TEXT_BELOW_READABLE_MIN") && strings.HasSuffix(e, "@ "+layerBadgePath+"/shape/text") {
			found = true
		}
		if strings.Contains(e, "/cells/0/shape") {
			t.Errorf("error %q names the cell's own shape; the text is in a layer", e)
		}
	}
	if !found {
		t.Errorf("no TEXT_BELOW_READABLE_MIN at %s/shape/text in %s", layerBadgePath, verdict)
	}
}

// A short badge on the same layer is valid everywhere.
func TestLayeredCellIsValidOnEverySurface(t *testing.T) {
	mc := testMCPConfig(t)
	path, _ := writeLayerDeck(t, layeredSlide("1"))
	if verdict := assertOneVerdict(t, verdictSurfaces(t, mc, path, true)); !verdict.Valid {
		t.Fatalf("verdict = %s, want a layered cell with a one-character badge valid", verdict)
	}
}

// validate -fit-report reports layer findings at .../cells/<c>/layers/<i>, and
// the repair it suggests names the layer, not its cell.
func TestFitReportNamesTheLayer(t *testing.T) {
	mc := testMCPConfig(t)
	_, deck := writeLayerDeck(t, layeredSlide("A badge label that is far too long to sit in a small circle on the ring"))
	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, textContent(res))
	}
	var atLayer, cellPaths int
	for _, raw := range answerFindings(t, doc) {
		f, _ := raw.(map[string]any)
		p, _ := f["path"].(string)
		if ev, ok := f["evidence"].(map[string]any); ok && p == "" {
			p, _ = ev["path"].(string)
		}
		if !strings.Contains(p, "/shape_grid/rows/0/cells/0") {
			continue
		}
		if !strings.HasPrefix(p, layerBadgePath) {
			t.Errorf("finding %v at %q is not under the layer %s", f["code"], p, layerBadgePath)
			continue
		}
		atLayer++
		for _, cp := range stringsUnderKey(f, "cell_path") {
			cellPaths++
			if cp != layerBadgePath {
				t.Errorf("finding %v suggests cell_path %q, want the layer %s", f["code"], cp, layerBadgePath)
			}
		}
	}
	if atLayer == 0 {
		t.Fatalf("the fit report has no finding under %s:\n%s", layerBadgePath, textContent(res))
	}
	if cellPaths == 0 {
		t.Errorf("no finding under %s carries a cell_path to repair", layerBadgePath)
	}
}

// stringsUnderKey collects every string stored under key anywhere in v.
func stringsUnderKey(v any, key string) []string {
	var out []string
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if s, ok := child.(string); ok && k == key {
				out = append(out, s)
				continue
			}
			out = append(out, stringsUnderKey(child, key)...)
		}
	case []any:
		for _, child := range x {
			out = append(out, stringsUnderKey(child, key)...)
		}
	}
	return out
}

// reduce_cell_text with a layer's cell_path shortens the layer's text and
// leaves the cell's own shape alone.
func TestReduceCellTextReachesALayer(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{
		Shape: &ShapeSpecInput{Geometry: "rect", Text: json.RawMessage(`"The cell's own text stays exactly as it was written"`)},
		Layers: []jsonschema.LayerInput{
			{Frame: jsonschema.LayerFrameInput{W: 1, H: 1}, Shape: &ShapeSpecInput{Geometry: "blockArc"}},
			{Frame: jsonschema.LayerFrameInput{W: 0.2, H: 0.2}, Shape: &ShapeSpecInput{Geometry: "ellipse", Text: json.RawMessage(`{"content":"A badge label that is far too long"}`)}},
		},
	}}}}}}}}
	cell := input.Slides[0].ShapeGrid.Rows[0].Cells[0]
	own := string(cell.Shape.Text)

	fix := applyReduceCellText(input, 0, map[string]any{"cell_path": "/slides/0/shape_grid/rows/0/cells/0/layers/1", "max_chars": 8})
	if !fix.Applied {
		t.Fatalf("reduce_cell_text on a layer was not applied: %+v", fix)
	}
	var got struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(cell.Layers[1].Shape.Text, &got); err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(got.Content)); n > 8 || got.Content == "" {
		t.Errorf("layer text = %q (%d runes), want at most 8", got.Content, n)
	}
	if string(cell.Shape.Text) != own {
		t.Errorf("the cell's own text changed to %s", cell.Shape.Text)
	}
	if fix := applyReduceCellText(input, 0, map[string]any{"cell_path": "/slides/0/shape_grid/rows/0/cells/0/layers/0", "max_chars": 8}); fix.Applied {
		t.Errorf("reduce_cell_text applied to a layer without text: %+v", fix)
	}
	if fix := applyReduceCellText(input, 0, map[string]any{"cell_path": "/slides/0/shape_grid/rows/0/cells/0/layers/7", "max_chars": 8}); fix.Applied {
		t.Errorf("reduce_cell_text applied to a layer that does not exist: %+v", fix)
	}
}

// layerContrastSlide puts light text on a light fill either in the cell's own
// shape or in a layer of an otherwise empty cell.
func layerContrastSlide(inLayer bool) SlideInput {
	shape := &ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"#EEEEEE"`),
		Text:     json.RawMessage(`{"content":"Caption","color":"#FFFFFF","size":12}`),
	}
	cell := &GridCellInput{Shape: shape}
	if inLayer {
		cell = &GridCellInput{Layers: []jsonschema.LayerInput{{Name: "label", Frame: jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1}, Shape: shape}}}
	}
	return SlideInput{SlideType: "blank", ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{cell}}}}}
}

// The contrast preflight sees a layer's fill and text as it sees a cell
// shape's: the same prediction, at the layer's path.
func TestLayerContrastPreflightMatchesCellShape(t *testing.T) {
	predict := func(inLayer bool) []patterns.FitFinding {
		input := &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{layerContrastSlide(inLayer)}}
		applyDefaults(input)
		return contrastPredictions(collectContrastPreflightFindings(input, nil, s7wmhCmdTheme))
	}
	cell, layer := predict(false), predict(true)
	if len(cell) != 1 || len(layer) != 1 {
		t.Fatalf("predictions: cell shape %d, layer %d; want 1 each\ncell: %+v\nlayer: %+v", len(cell), len(layer), cell, layer)
	}
	if want := "/slides/0/shape_grid/rows/0/cells/0/shape/text"; !strings.HasPrefix(cell[0].Path, want) {
		t.Errorf("cell prediction path = %q, want under %q", cell[0].Path, want)
	}
	if want := "/slides/0/shape_grid/rows/0/cells/0/layers/0/shape/text"; !strings.HasPrefix(layer[0].Path, want) {
		t.Errorf("layer prediction path = %q, want under %q", layer[0].Path, want)
	}
	if cell[0].Code != layer[0].Code || cell[0].Action != layer[0].Action {
		t.Errorf("layer prediction %s/%s differs from the cell shape's %s/%s", layer[0].Code, layer[0].Action, cell[0].Code, cell[0].Action)
	}
	if cell[0].Fix == nil || layer[0].Fix == nil {
		t.Fatalf("a prediction has no fix: cell %+v, layer %+v", cell[0].Fix, layer[0].Fix)
	}
	for _, key := range []string{"from", "predicted_replacement"} {
		if cell[0].Fix.Params[key] != layer[0].Fix.Params[key] {
			t.Errorf("fix %s: layer %v, cell shape %v", key, layer[0].Fix.Params[key], cell[0].Fix.Params[key])
		}
	}
}

// Generation makes the swap the preflight predicted for the layer.
func TestLayerContrastSwapMatchesPreflight(t *testing.T) {
	input := &PresentationInput{Template: "midnight-blue", OutputFilename: "layer-contrast.pptx", Slides: []SlideInput{layerContrastSlide(true)}}
	applyDefaults(input)
	predictions := contrastPredictions(collectContrastPreflightFindings(input, nil, s7wmhCmdTheme))
	if len(predictions) != 1 {
		t.Fatalf("preflight predictions = %d, want 1", len(predictions))
	}
	result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, swap := range result.GenResult.ContrastSwaps {
		if swap.Source == "shape_grid" && swap.ReplacedColor == predictions[0].Fix.Params["predicted_replacement"] {
			matched = true
		}
	}
	if !matched {
		t.Errorf("render swaps %+v do not include the predicted replacement %v", result.GenResult.ContrastSwaps, predictions[0].Fix.Params["predicted_replacement"])
	}
}

// A frame that leaves its cell is refused by every surface with the layer
// named in the message.
func TestLayerFrameOutOfCellHasOneVerdict(t *testing.T) {
	mc := testMCPConfig(t)
	slide := strings.Replace(layeredSlide("1"), `"frame": {"x": 0.72, "y": 0.06, "w": 0.12, "h": 0.12}`, `"frame": {"x": 0.95, "y": 0.06, "w": 0.12, "h": 0.12}`, 1)
	path, deck := writeLayerDeck(t, slide)
	if verdict := assertOneVerdict(t, verdictSurfaces(t, mc, path, true)); verdict.Valid {
		t.Fatalf("verdict = %s, want a frame that leaves its cell refused", verdict)
	}
	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"LAYER_FRAME_OUT_OF_CELL", "layers[1]", "badge-1"} {
		if !strings.Contains(textContent(res), want) {
			t.Errorf("the refusal does not mention %s:\n%s", want, textContent(res))
		}
	}
}

// A layer shape is validated like a cell shape: an unknown geometry, a hex
// colour in constrained mode and an icon are all reported at the layer.
func TestLayerShapeIsValidatedLikeACellShape(t *testing.T) {
	grid := &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Layers: []jsonschema.LayerInput{
		{Frame: jsonschema.LayerFrameInput{W: 1, H: 1}, Shape: &ShapeSpecInput{Geometry: "blockArk", Fill: json.RawMessage(`"#FF0000"`)}},
		{Frame: jsonschema.LayerFrameInput{W: 1, H: 1}, Shape: &ShapeSpecInput{Geometry: "ellipse", TypeScale: "enormous", Icon: &IconInput{Name: "shield"}}},
	}}}}}}

	findings := checkShapeGridAt(grid, 1, "/slides/0/shape_grid", nil, false)
	hex := false
	for _, f := range findings {
		if strings.HasPrefix(f.Path, "/slides/0/shape_grid/rows/0/cells/0/layers/0/shape/fill") {
			hex = true
		}
	}
	if !hex {
		t.Errorf("design mode did not report the layer's hex fill: %+v", findings)
	}

	enums := checkGridTypeScaleEnums(grid, "/slides/0/shape_grid")
	if len(enums) != 1 || enums[0].Path != "/slides/0/shape_grid/rows/0/cells/0/layers/1/shape/type_scale" {
		t.Errorf("type_scale enum errors = %+v, want one at the layer", enums)
	}

	// The enum error is reported before the grid is walked; clear it so the
	// dry run reaches the shape checks.
	grid.Rows[0].Cells[0].Layers[1].Shape.TypeScale = ""
	data, err := json.Marshal(map[string]any{"template": "midnight-blue", "design_mode": "free", "slides": []any{
		map[string]any{"slide_type": "blank", "shape_grid": grid},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bad-layer.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		_ = runJSONDryRun(path, "", testutil.TemplatesDir(), "", "", false)
	})
	for _, want := range []string{`cell 1 layer 1: unknown geometry \"blockArk\"`, `cell 1 layer 2: \"icon\" is not supported on a layer shape`} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run does not report %s:\n%s", want, out)
		}
	}
}

// A pattern that returns layers gets the same expansion stamps on them as on
// cell shapes: the type-scale policy and the faces its text was sized in.
func TestPatternStampsReachLayers(t *testing.T) {
	pinned := &ShapeSpecInput{Geometry: "ellipse", TypeScale: "compact"}
	open := &ShapeSpecInput{Geometry: "ellipse"}
	nested := &ShapeSpecInput{Geometry: "ellipse"}
	grid := &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
		{Layers: []jsonschema.LayerInput{{Shape: pinned}, {Shape: open}}},
		{Grid: &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Layers: []jsonschema.LayerInput{{Shape: nested}}}}}}}},
	}}}}
	fonts := jsonschema.MeasureFonts{Major: "Georgia", Minor: "Arial"}
	stampPatternTypeScale(grid, "comfortable")
	stampPatternMeasureFonts(grid, fonts)
	if pinned.TypeScale != "compact" || open.TypeScale != "comfortable" || nested.TypeScale != "comfortable" {
		t.Errorf("type scale stamps = %q / %q / %q, want the pinned layer kept and the others stamped", pinned.TypeScale, open.TypeScale, nested.TypeScale)
	}
	for name, s := range map[string]*ShapeSpecInput{"pinned": pinned, "open": open, "nested": nested} {
		if s.MeasureFonts != fonts {
			t.Errorf("%s layer measure fonts = %+v, want %+v", name, s.MeasureFonts, fonts)
		}
	}
	// The stamp reaches the writer: a converted layer measures in that face.
	rows := convertGridRows(grid.Rows)
	if got := rows[0].Cells[0].Layers[1].Shape.ThemeFonts; got.Major != "Georgia" || got.Minor != "Arial" {
		t.Errorf("converted layer theme fonts = %+v, want the stamped faces", got)
	}
}
