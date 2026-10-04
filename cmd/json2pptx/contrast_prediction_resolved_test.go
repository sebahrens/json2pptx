package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

var contrastResolvedTheme = []types.ThemeColor{
	{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
	{Name: "dk2", RGB: "#1F2937"}, {Name: "lt2", RGB: "#F3F4F6"},
	{Name: "accent1", RGB: "#F28C28"}, {Name: "accent2", RGB: "#2563EB"},
}

func contrastResolvedZone(slideW, slideH int64) *shapegrid.ContentZone {
	return &shapegrid.ContentZone{
		TitleBottom: 1300000, FooterTop: 6300000, LeftMargin: 600000, RightEdge: 11600000,
		SlideWidth: slideW, SlideHeight: slideH,
	}
}

// generationGridShapes resolves grid the way convertPresentationSlides does.
func generationGridShapes(t *testing.T, grid *ShapeGridInput, zone *shapegrid.ContentZone, slideW, slideH int64) *ShapeGridResult {
	t.Helper()
	alloc := &pptx.ShapeIDAllocator{}
	alloc.SetMinID(200)
	rendered, err := resolveShapeGrid(grid, alloc, nil, zone, slideW, slideH, &GridDiagramContext{ThemeColors: contrastResolvedTheme, SlideNum: 1})
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}

// The contrast prediction reads the shapes generation writes, not the authored
// ones. Resolution changes text sizes — here the composition policy steps a
// sparse block's 14pt labels to 18pt — and 18pt is large text, held to WCAG's
// 3:1 instead of 4.5:1. Compiled from the authored spec, validate predicted a
// swap generation never made (TestContrastParityCorpus, varied-pitch-deck
// slide 7) and missed one it did (patterns-smoke slide 6).
func TestContrastPredictionCompilesResolvedGridShapes(t *testing.T) {
	const slideW, slideH = int64(12192000), int64(6858000)
	var grid ShapeGridInput
	if err := json.Unmarshal([]byte(`{
		"columns": 3, "vertical_align": "auto",
		"rows": [{"max_height": 60, "cells": [
			{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "North", "size": 14, "color": "accent1"}}},
			{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "South", "size": 14, "color": "accent1"}}},
			{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "West", "size": 14, "color": "accent1"}}}
		]}]
	}`), &grid); err != nil {
		t.Fatal(err)
	}
	zone := contrastResolvedZone(slideW, slideH)
	rendered := generationGridShapes(t, &grid, zone, slideW, slideH)
	predicted := predictedGridShapes(&grid, 0, nil, zone, slideW, slideH, contrastResolvedTheme, "")
	if predicted == nil || len(predicted.Shapes) != 3 || len(rendered.Shapes) != 3 {
		t.Fatalf("predicted %+v, want the 3 shapes generation writes (%d)", predicted, len(rendered.Shapes))
	}
	for i, shape := range predicted.Shapes {
		if !bytes.Equal(shape, rendered.Shapes[i]) {
			t.Errorf("predicted shape %d (%s) is not the shape generation writes:\n%s", i, predicted.ShapeSources[i].Path, shape)
		}
		// Guard the premise: the block is composed, so its type is stepped.
		if !bytes.Contains(shape, []byte(`sz="1800"`)) || bytes.Contains(shape, []byte(`sz="1400"`)) {
			t.Errorf("predicted shape %d keeps its authored 14pt; the composed block renders at 18pt:\n%s", i, shape)
		}
	}
}

// go-slide-creator-2ciwz: prediction and generation share one shape-producing
// path. The predicted list is generation's list — connectors, accent bars, a
// table, a grouped cell, an image label, a native diagram and a nested grid at
// the same flat indices, byte for byte — every index maps back to the element
// the author wrote, and a diagram cell that renders to an image is not
// rendered to predict.
func TestContrastPredictionSharesGenerationShapeList(t *testing.T) {
	const slideW, slideH = int64(12192000), int64(6858000)
	var grid ShapeGridInput
	if err := json.Unmarshal([]byte(`{
		"columns": 3,
		"links": [{"from": [0, 0], "to": [2, 2], "connector": {"style": "arrow"}}],
		"rows": [
			{"connector": {"style": "arrow"}, "cells": [
				{"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "Plan", "size": 12, "color": "lt1"}}, "accent_bar": {"position": "left", "color": "accent2"}},
				{"col_span": 2, "group": true, "shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "Build", "size": 12, "color": "lt1"}}}
			]},
			{"rule": "above", "cells": [
				{"table": {"headers": ["A", "B"], "rows": [["1", "2"]]}},
				{"diagram": {"type": "bar_chart", "data": {"categories": ["a", "b"], "series": [{"name": "s", "values": [1, 2]}]}}},
				{"composite": {"split": "top", "ratio": 0.4,
					"text": {"geometry": "rect", "fill": "dk2", "text": {"content": "Read", "size": 12, "color": "dk1"}},
					"sub_diagram": {"type": "bar_chart", "data": {"categories": ["a"], "series": [{"name": "s", "values": [1]}]}}}}
			]},
			{"cells": [
				{"image": {"path": "missing.png", "alt": "photo", "overlay": {"color": "dk1", "alpha": 40}, "text": {"content": "Caption", "size": 14, "color": "lt1"}}},
				{"shape": {"geometry": "rect", "fill": "lt2", "icon": {"name": "shield"}, "text": {"content": "Ship", "size": 12, "color": "dk1"}}},
				{"grid": {"columns": 2, "rows": [{"connector": {"style": "line"}, "cells": [
					{"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "In", "size": 11, "color": "lt1"}}},
					{"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Out", "size": 11, "color": "dk1"}}}
				]}]}}
			]}
		]
	}`), &grid); err != nil {
		t.Fatal(err)
	}
	zone := contrastResolvedZone(slideW, slideH)
	rendered := generationGridShapes(t, &grid, zone, slideW, slideH)
	predicted := predictedGridShapes(&grid, 0, nil, zone, slideW, slideH, contrastResolvedTheme, "")
	if predicted == nil {
		t.Fatal("no predicted shapes for a grid generation renders")
	}

	// Generation rendered both bar charts to images; the prediction did not,
	// and resolved no icon either.
	if len(rendered.IconInserts) < 3 {
		t.Fatalf("premise: generation should insert two diagrams and an icon, got %d", len(rendered.IconInserts))
	}
	if len(predicted.IconInserts) != 0 {
		t.Errorf("prediction rendered %d diagram / icon pictures; it must render none", len(predicted.IconInserts))
	}

	if len(predicted.Shapes) != len(rendered.Shapes) || len(predicted.ShapeSources) != len(predicted.Shapes) || len(rendered.ShapeSources) != len(rendered.Shapes) {
		t.Fatalf("shape lists differ: predicted %d shapes / %d sources, generation %d / %d",
			len(predicted.Shapes), len(predicted.ShapeSources), len(rendered.Shapes), len(rendered.ShapeSources))
	}
	var paths []string
	for i := range rendered.Shapes {
		if !bytes.Equal(predicted.Shapes[i], rendered.Shapes[i]) {
			t.Errorf("shape %d (%s) differs from the one generation writes:\npredicted  %s\ngeneration %s",
				i, predicted.ShapeSources[i].Path, predicted.Shapes[i], rendered.Shapes[i])
		}
		if predicted.ShapeSources[i].Path != rendered.ShapeSources[i].Path {
			t.Errorf("shape %d: predicted source %q, generation %q", i, predicted.ShapeSources[i].Path, rendered.ShapeSources[i].Path)
		}
		paths = append(paths, strings.TrimPrefix(predicted.ShapeSources[i].Path, "/slides/0/shape_grid"))
	}
	// Connectors first, then the cells in resolved order, accent bars and row
	// rules, and the nested grid's own list last — re-rooted under its cell.
	want := []string{
		"/rows/0/connector",
		"/links/0",
		"/rows/0/cells/0/shape/text",
		"/rows/0/cells/1/shape/text", // the group stands for its cell
		"/rows/1/cells/0/table",
		"/rows/1/cells/2/composite/text/text",
		"/rows/2/cells/0/image/overlay",
		"/rows/2/cells/0/image/text",
		"/rows/2/cells/1/shape/text",
		"/rows/0/cells/0/accent_bar",
		"", // the rule above row 1 belongs to the grid
		"/rows/2/cells/2/grid/rows/0/connector",
		"/rows/2/cells/2/grid/rows/0/cells/0/shape/text",
		"/rows/2/cells/2/grid/rows/0/cells/1/shape/text",
	}
	if strings.Join(paths, "\n") != strings.Join(want, "\n") {
		t.Errorf("flat shape index → authored path:\n got %q\nwant %q", paths, want)
	}
	if !bytes.Contains(rendered.Shapes[3], []byte("<p:grpSp>")) || !bytes.Contains(rendered.Shapes[4], []byte("<a:tbl>")) {
		t.Errorf("premise: shape 3 should be the grouped cell and shape 4 the table")
	}

	// A decision the pass makes on one shape of that list is reported at the
	// authored text it is about, never at the flat index.
	swaps := generator.PredictCompiledGridContrast(rendered.Shapes, contrastResolvedTheme, 0, "#FFFFFF")
	if len(swaps) == 0 {
		t.Fatal("premise: white on accent1 (#F28C28) and dk1 on dk2 need a repair")
	}
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &grid}}}
	findings := contrastPredictions(collectContrastPreflightFindings(input, nil, contrastResolvedTheme))
	authoredText := 0
	for _, f := range findings {
		if strings.Contains(f.Path, "/shapes/") {
			t.Errorf("finding at %s still names a flat shape index: %s", f.Path, f.Message)
		}
		if strings.HasSuffix(f.Path, "/shape/text") || strings.HasSuffix(f.Path, "/composite/text/text") {
			authoredText++
		}
	}
	if authoredText == 0 {
		t.Errorf("no finding names an authored cell's text: %+v", findings)
	}
}

// go-slide-creator-x54jd: a pattern nested in a grid cell is expanded for the
// prediction exactly as generation expands it (expandNestedCellPatternsInBounds
// in the slide's content rectangle), so both run the contrast pass over one
// shape list. Resolving the unexpanded grid left the pattern's shapes out.
// go-slide-creator-i1x53: a shape the nested pattern wrote is attributed to
// the authored pattern, never to the grid it expanded to.
func TestContrastPredictionExpandsNestedCellPatterns(t *testing.T) {
	const slideW, slideH = int64(12192000), int64(6858000)
	const gridJSON = `{
		"columns": 2,
		"rows": [{"cells": [
			{"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "Plan", "size": 12, "color": "lt1"}}},
			{"pattern": {"name": "kpi-2up", "values": [{"big": "42%", "small": "Margin"}, {"big": "3.1x", "small": "Return"}]}}
		]}]
	}`
	parse := func() *ShapeGridInput {
		var grid ShapeGridInput
		if err := json.Unmarshal([]byte(gridJSON), &grid); err != nil {
			t.Fatal(err)
		}
		return &grid
	}
	zone := contrastResolvedZone(slideW, slideH)
	content := pptx.RectEmu{X: zone.LeftMargin, Y: zone.TitleBottom, CX: zone.RightEdge - zone.LeftMargin, CY: zone.FooterTop - zone.TitleBottom}
	theme := &types.ThemeInfo{Colors: contrastResolvedTheme}

	// Generation: expand in place, then resolve (convertSinglePresentationSlide).
	generated := parse()
	if err := expandNestedCellPatternsInBounds(generated, patterns.ExpandContext{
		ContentZone: zone, SlideWidth: slideW, SlideHeight: slideH,
		LayoutBounds: patterns.LayoutBounds{X: content.X, Y: content.Y, Width: content.CX, Height: content.CY},
		Theme:        *theme,
	}, content, patterns.Default(), true); err != nil {
		t.Fatal(err)
	}
	rendered := generationGridShapes(t, generated, zone, slideW, slideH)

	authored := parse()
	predicted, sources := predictedSlideGridShapes(authored, 0, nestedExpansionGeometry{
		geom: GridGeometry{Zone: zone}, contentBounds: content, slideWidth: slideW, slideHeight: slideH, theme: theme,
	})
	if predicted == nil {
		t.Fatal("no predicted shapes for a grid generation renders")
	}
	if len(authored.Rows[0].Cells[1].Pattern) == 0 || authored.Rows[0].Cells[1].Grid != nil {
		t.Error("the prediction expanded the authored grid in place")
	}
	if len(predicted.Shapes) != len(rendered.Shapes) || len(rendered.Shapes) < 3 || len(sources) != len(predicted.Shapes) {
		t.Fatalf("predicted %d shapes / %d sources, generation %d (want the cell plus the pattern's shapes)",
			len(predicted.Shapes), len(sources), len(rendered.Shapes))
	}
	inPattern := 0
	for i := range rendered.Shapes {
		if !bytes.Equal(predicted.Shapes[i], rendered.Shapes[i]) {
			t.Errorf("shape %d (%s) differs from the one generation writes:\npredicted  %s\ngeneration %s",
				i, sources[i].Path, predicted.Shapes[i], rendered.Shapes[i])
		}
		if strings.Contains(sources[i].Path, "/cells/1/grid") {
			t.Errorf("shape %d is attributed to the expanded grid %s; the author wrote a pattern", i, sources[i].Path)
		}
		if strings.HasPrefix(sources[i].Path, "/slides/0/shape_grid/rows/0/cells/1/pattern") {
			inPattern++
			if len(sources[i].Text) != 0 {
				t.Errorf("shape %d of the nested pattern carries an authored text spec; no cell-level repair applies", i)
			}
		}
	}
	if inPattern == 0 {
		t.Errorf("no shape is attributed to the nested pattern: %+v", sources)
	}
}

// go-slide-creator-i1x53: on a DeckSpec slide the authored path of a contrast
// repair maps to the slide's place in the spec, the same semantic path the
// prediction gets, so validate and generation name one location there too.
func TestContrastAutofixedOnDeckSpecSlideMapsToItsSemanticPath(t *testing.T) {
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Contrast", Template: "midnight-blue", DesignMode: "free"},
		Slides: []semantic.SlideSpec{
			{Kind: semantic.KindTitle, Body: map[string]any{"title": "Contrast"}},
			{Kind: semantic.KindRawJSON2pptx, Body: map[string]any{"slide": map[string]any{
				"layout_id": "blank-title",
				"shape_grid": map[string]any{"columns": 1, "rows": []any{map[string]any{"cells": []any{map[string]any{
					"shape": map[string]any{"geometry": "rect", "fill": "#FFE8D4", "text": map[string]any{"content": "Pale", "size": 14, "color": "lt1"}},
				}}}}},
			}}},
		},
	}
	input, compiled, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatal(err)
	}
	applyDefaults(input)
	theme := []types.ThemeColor{{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1B2A4A"}}
	predicted := contrastPredictions(collectContrastPreflightFindings(input, nil, theme))
	if len(predicted) != 1 || predicted[0].Path != "/slides/1/shape_grid/rows/0/cells/0/shape/text" {
		t.Fatalf("predictions = %+v; want one at the authored cell text", predicted)
	}
	grid := predictedGridShapes(input.Slides[1].ShapeGrid, 1, nil, nil, 0, 0, theme, "")
	swaps := generator.PredictCompiledGridContrast(grid.Shapes, theme, 1, "#FFFFFF")
	generator.AttributeGridSwaps(swaps, grid.ShapeSources, 1)
	fixed := contrastSwapsToFindings(swaps, input, theme)
	if len(fixed) != 1 || fixed[0].Path != predicted[0].Path {
		t.Fatalf("contrast_autofixed = %+v; want it at %s", fixed, predicted[0].Path)
	}
	got, want := semanticDiagFromFit(compiled.SourceMap, fixed[0]), semanticDiagFromFit(compiled.SourceMap, predicted[0])
	if got.SemanticPath == "" || got.SemanticPath != want.SemanticPath || !strings.HasPrefix(got.SemanticPath, "slides[1]") {
		t.Errorf("contrast_autofixed semantic path %q, contrast_predicted %q; want one path inside slides[1]", got.SemanticPath, want.SemanticPath)
	}
	if got.RawPath != fixed[0].Path {
		t.Errorf("raw_path = %q, want the authored raw path %q kept as evidence", got.RawPath, fixed[0].Path)
	}

	// go-slide-creator-sw78d: the DeckSpec render result reports the forecast
	// for a swap validate predicted (and nothing more, so the two surfaces
	// agree), and contrast_autofixed — at the same semantic path — for a swap
	// generation made that the forecast did not name.
	if extra := unpredictedContrastSwapFindings(fixed, predicted); len(extra) != 0 {
		t.Errorf("a predicted swap is reported a second time: %+v", extra)
	}
	contrastDiags := func(swaps []generator.ContrastSwap) map[string][]string {
		res := buildSemanticRenderSuccess(input, compiled, RenderResult{
			GenResult:     &generator.GenerationResult{ContrastSwaps: swaps},
			TemplateTheme: types.ThemeInfo{Colors: theme},
		}, time.Now())
		byCode := map[string][]string{}
		for _, d := range res.Diagnostics {
			if strings.Contains(d.Code, "contrast_") {
				code := d.Code[strings.Index(d.Code, "contrast_"):]
				byCode[code] = append(byCode[code], d.SemanticPath)
			}
		}
		return byCode
	}
	forecast := contrastDiags(swaps)
	if len(forecast[patterns.ErrCodeContrastPredicted]) != 1 || len(forecast["contrast_autofixed"]) != 0 {
		t.Fatalf("render with the predicted swap reports %v; want the one forecast and no contrast_autofixed", forecast)
	}
	missed := swaps[0]
	missed.OriginalColor, missed.ReplacedColor = "#FFF2CC", "#333333" // a swap no forecast names
	withMissed := contrastDiags([]generator.ContrastSwap{swaps[0], missed})
	if len(withMissed[patterns.ErrCodeContrastPredicted]) != 1 {
		t.Errorf("forecast changed when generation made a second swap: %v", withMissed)
	}
	if paths := withMissed["contrast_autofixed"]; len(paths) != 1 || paths[0] != want.SemanticPath {
		t.Errorf("unpredicted render-time swap reported as %v; want one contrast_autofixed at %q", withMissed, want.SemanticPath)
	}
}
