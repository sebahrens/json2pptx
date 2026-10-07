package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// chartParityFixture loads what both passes need from one bundled template.
type chartParityFixture struct {
	path        string
	layouts     []types.LayoutMetadata
	w, h        int64
	synthetic   map[string][]byte
	theme       types.ThemeInfo
	metadata    *types.TemplateMetadata
	dataPalette []string
}

func loadChartParityFixture(t *testing.T, name string) chartParityFixture {
	t.Helper()
	fx := chartParityFixture{path: filepath.Join("..", "..", "templates", name+".pptx")}
	reader, err := template.OpenTemplate(fx.path)
	if err != nil {
		t.Fatalf("OpenTemplate(%s): %v", name, err)
	}
	defer func() { _ = reader.Close() }()
	if fx.layouts, err = template.ParseLayouts(reader); err != nil {
		t.Fatalf("ParseLayouts(%s): %v", name, err)
	}
	fx.w, fx.h = template.ParseSlideDimensions(reader)
	fx.theme = template.ParseTheme(reader)
	analysis := &types.TemplateAnalysis{TemplatePath: fx.path, SlideWidth: fx.w, SlideHeight: fx.h, Layouts: fx.layouts, Theme: fx.theme}
	template.SynthesizeIfNeeded(reader, analysis)
	if analysis.Synthesis != nil {
		fx.synthetic = analysis.Synthesis.SyntheticFiles
	}
	fx.metadata, _ = template.ParseMetadata(reader)
	fx.dataPalette = resolveDataPalette(fx.metadata, fx.theme.Colors)
	return fx
}

// go-slide-creator-xlkwt: on a template that declares a data_palette (every
// bundled one does) the dry render measures a chart on that palette and the
// template's semantic accents, as generate draws it.
// It runs as a subtest of TestPlaceholderChartDryRenderRequestMatchesGenerate.
func dryRenderRequestUsesTheDeclaredPalette(t *testing.T) {
	bounds := types.BoundingBox{Width: 9_000_000, Height: 4_000_000}
	for _, name := range []string{"warm-coral", "midnight-blue"} {
		fx := loadChartParityFixture(t, name)
		if len(fx.dataPalette) == 0 {
			t.Fatalf("%s declares no data_palette: the fixture no longer tests the declared-palette path", name)
		}
		tmpl := chartTemplateStyleOf(&fx.theme)
		if !reflect.DeepEqual(tmpl.dataPalette, fx.dataPalette) {
			t.Errorf("%s: template style palette = %v, want the declared %v", name, tmpl.dataPalette, fx.dataPalette)
		}

		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A", "B"}, "series": []any{
				map[string]any{"name": "North", "values": []any{1.0, 2.0}},
				map[string]any{"name": "South", "values": []any{2.0, 1.0}},
			}},
		}
		// What generate hands its converter for a placeholder chart
		// (processDiagramContent): the theme, the declared palette, the
		// semantic accents.
		rendered := *spec
		rendered.Style = &types.DiagramStyle{ThemeColors: fx.theme.Colors, DataPalette: fx.dataPalette}
		if fx.metadata != nil {
			rendered.Style.SemanticAccents = fx.metadata.SemanticAccents
		}
		want := generator.DiagramRenderRequest(&rendered, fx.theme.Colors, "warn")

		for _, grid := range []bool{false, true} {
			styled := tmpl.apply(spec, grid)
			if spec.Style != nil {
				t.Fatalf("%s: apply changed the authored spec", name)
			}
			got := dryRenderRequest(styled, fx.theme.Colors, fx.theme.BodyFont, "warn", grid, bounds, tokens.ViewingModePresentation)
			if !reflect.DeepEqual(got.Style.DataPalette, want.Style.DataPalette) || got.Style.DataPaletteFixed != want.Style.DataPaletteFixed {
				t.Errorf("%s (grid=%v): series palette %v (fixed %v), generate draws %v (fixed %v)", name, grid,
					got.Style.DataPalette, got.Style.DataPaletteFixed, want.Style.DataPalette, want.Style.DataPaletteFixed)
			}
			if !reflect.DeepEqual(got.Style.ThemeColors, want.Style.ThemeColors) {
				t.Errorf("%s (grid=%v): theme colours differ from generate", name, grid)
			}
			// A grid cell takes the palette only; a placeholder also the
			// semantic accents (the two render paths differ, and so must we).
			if !grid && got.Style.SemanticAccents != want.Style.SemanticAccents {
				t.Errorf("%s: semantic accents %+v, generate uses %+v", name, got.Style.SemanticAccents, want.Style.SemanticAccents)
			}
		}

		// A chart that names its own colours keeps them on both paths.
		own := *spec
		own.Style = &types.DiagramStyle{Colors: []string{"accent2", "#123456"}}
		if styled := tmpl.apply(&own, false); len(styled.Style.DataPalette) != 0 {
			t.Errorf("%s: the template palette overrode authored style.colors", name)
		}
	}
}

// go-slide-creator-xlkwt: validate's predicted chart findings are the ones the
// render reports, on two templates with different declared palettes.
func TestChartDryRenderFindingsMatchGenerateAcrossTemplates(t *testing.T) {
	var cats []string
	var vals []float64
	for i := 0; i < 36; i++ {
		cats = append(cats, fmt.Sprintf("Business unit number %02d", i+1))
		vals = append(vals, float64(10+i))
	}
	chartSlide := func(title string, chart map[string]any) map[string]any {
		return map[string]any{"slide_type": "chart", "content": []map[string]any{
			{"placeholder_id": "title", "type": "text", "text_value": title},
			{"placeholder_id": "body", "type": "chart", "chart_value": chart},
		}}
	}
	slides := []map[string]any{
		chartSlide("The gauge value is past its range", map[string]any{"type": "gauge", "alt": "gauge",
			"data": map[string]any{"value": 140, "min": 0, "max": 100}}),
		chartSlide("The funnel widens at its second stage", map[string]any{"type": "funnel", "alt": "funnel",
			"data": map[string]any{"values": []map[string]any{{"label": "Leads", "value": 100}, {"label": "Qualified", "value": 180}, {"label": "Won", "value": 20}}}}),
		chartSlide("Thirty-six business units crowd one axis", map[string]any{"type": "bar", "alt": "bars",
			"data": map[string]any{"categories": cats, "series": []map[string]any{{"name": "Revenue", "values": vals}}}}),
		chartSlide("Three regions over four quarters", map[string]any{"type": "stacked_bar", "alt": "stack",
			"data": map[string]any{"categories": []string{"Q1", "Q2", "Q3", "Q4"}, "series": []map[string]any{
				{"name": "North", "values": []float64{12, 14, 15, 18}}, {"name": "South", "values": []float64{8, 9, 11, 12}}, {"name": "East", "values": []float64{5, 7, 6, 9}}}}}),
	}

	chartCodes := func(findings []patterns.FitFinding) []string {
		var out []string
		for _, f := range findings {
			if !strings.HasPrefix(f.Code, "chart.") {
				continue
			}
			parts := strings.Split(f.Path, "/") // /slides/{i}/...
			slide := ""
			if len(parts) > 2 {
				slide = parts[2]
			}
			out = append(out, "slide "+slide+": "+f.Code)
		}
		slices.Sort(out)
		return slices.Compact(out)
	}

	for _, name := range []string{"midnight-blue", "warm-coral"} {
		fx := loadChartParityFixture(t, name)
		raw, _ := json.Marshal(map[string]any{"template": name, "slides": slides})
		var input PresentationInput
		if err := strictUnmarshalJSON(raw, &input); err != nil {
			t.Fatalf("%s: unmarshal deck: %v", name, err)
		}
		applyDefaults(&input)

		validated := chartCodes(collectChartDryRenderFindingsInFrames(&input, fx.theme.Colors, fx.theme.BodyFont, "warn",
			fx.layouts, fx.w, fx.h, chartTemplateStyleOf(&fx.theme)))
		if len(validated) < 2 {
			t.Fatalf("%s: validate predicts %v; the deck is meant to raise chart findings", name, validated)
		}
		renderFindings, evidence := repairMC(t).collectRenderFindings(context.Background(), &input, fx.path,
			fx.layouts, fx.w, fx.h, fx.synthetic, fx.metadata, fx.dataPalette)
		if !evidence.Complete {
			t.Fatalf("%s: render incomplete: stage=%q detail=%q", name, evidence.Stage, evidence.Detail)
		}
		if rendered := chartCodes(renderFindings); !slices.Equal(validated, rendered) {
			t.Errorf("%s: validate and generate disagree on the chart findings:\n validate: %v\n generate: %v", name, validated, rendered)
		}
	}
}
