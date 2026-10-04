package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// gridShapesBothWays returns, per slide, the grid shapes generation writes
// (convertPresentationSlides, as RunPresentation calls it) and the grid shapes
// validate predicts for the same deck (the expansion chain collectFitFindings
// runs before its contrast preflight). ok is false when generation refuses
// the deck; there is nothing to compare then.
func gridShapesBothWays(t *testing.T, input *PresentationInput, templateName string) (generated, predicted [][][]byte, ok bool) {
	t.Helper()
	templatePath := filepath.Join(testutil.TemplatesDir(), templateName+".pptx")
	layouts, _, width, height, metadata, theme, _ := analyzeTemplateLayouts(templatePath)
	if layouts == nil {
		t.Fatalf("cannot analyze template %q", templateName)
	}
	// Validate's theme the way the read-only tools that parse only the theme
	// get it (repair_slide, apply_patch, score_deck, preview): it has to carry
	// the template metadata by itself.
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	validateTheme := template.ParseTheme(reader)
	_ = reader.Close()
	if input.ThemeOverride != nil {
		theme, _ = theme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
		validateTheme, _ = validateTheme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
	}
	applyDefaults(input)
	resolveCanonicalLayoutIDs(input.Slides, layouts)

	// Validate first: it must predict from the deck as authored.
	predicted = make([][][]byte, len(input.Slides))
	fitInput, fitLayouts, _ := withDerivedFitLayouts(input, layouts, width, height)
	fitInput, _ = expandComposeForPreflightWithTheme(fitInput, width, height, &validateTheme, fitLayouts...)
	fitInput, _ = expandPatternsForFit(fitInput, width, height, &validateTheme, fitLayouts...)
	rhythm := resolvedValidRhythmGrid(fitInput, fitLayouts, width, height)
	sectionIndices := slideSectionIndices(fitInput.Slides, fitLayouts)
	for si, slide := range fitInput.Slides {
		if slide.ShapeGrid == nil {
			continue
		}
		geom, contentBounds := patternExpansionGeometry(slide, fitLayouts, width, height, rhythm)
		grid, _ := predictedSlideGridShapes(slide.ShapeGrid, si, nestedExpansionGeometry{
			geom: geom, contentBounds: contentBounds, slideWidth: width, slideHeight: height,
			theme: &validateTheme, strategy: patterns.AccentStrategy(input.AccentStrategy), slideIdx: si, sectionIdx: sectionIndices[si],
		})
		if grid != nil {
			predicted[si] = grid.Shapes
		}
	}

	var rhythmGrid *resolvedGrid
	if input.Grid != nil && validateGridConfig(input.Grid) == nil {
		rhythmGrid = resolveGrid(input.Grid, layouts, width, height)
	}
	authored, _ := json.Marshal(input)
	specs, _, _, err := convertPresentationSlides(input.Slides, layouts, width, height, metadata, rhythmGrid,
		patterns.AccentStrategy(input.AccentStrategy), &GridDiagramContext{
			ThemeColors: theme.Colors,
			DataPalette: resolveDataPalette(metadata, theme.Colors),
			FontFamily:  theme.BodyFont,
			TitleFont:   theme.TitleFont,
			ViewingMode: tokens.ParseViewingMode(input.ViewingMode),
		}, false)
	if err != nil {
		t.Logf("generation refuses the deck on %s: %v", templateName, err)
		return nil, nil, false
	}
	if after, _ := json.Marshal(input); !bytes.Equal(authored, after) {
		t.Errorf("generation rewrote the deck it was handed (%s)", templateName)
	}
	generated = make([][][]byte, len(specs))
	for i, spec := range specs {
		generated[i] = spec.RawShapeXML
	}
	return generated, predicted, true
}

// assertGridShapeParity fails for every slide whose predicted grid shapes are
// not, byte for byte, the shapes generation writes.
func assertGridShapeParity(t *testing.T, generated, predicted [][][]byte) {
	t.Helper()
	for si := range generated {
		var want [][]byte
		if si < len(predicted) {
			want = predicted[si]
		}
		if len(generated[si]) != len(want) {
			t.Errorf("slide %d: validate predicts %d grid shapes, generation writes %d", si+1, len(want), len(generated[si]))
			continue
		}
		for i := range want {
			if !bytes.Equal(want[i], generated[si][i]) {
				t.Errorf("slide %d shape %d: validate expands a different shape than generation writes\npredicted: %s\ngenerated: %s", si+1, i, want[i], generated[si][i])
				break
			}
		}
	}
}

// expansionParityDeck exercises what the two expansion contexts used to
// disagree on: a semantic role fill resolved from template metadata (the
// waterfall's negative bars), heading-font numerals sized in the title font
// (agenda, next-steps), a compose envelope, and a pattern nested in an
// authored grid cell.
const expansionParityDeck = `{
	"accent_strategy": "rotate",
	"slides": [
		{"slide_type": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Bridge"}],
		 "pattern": {"name": "waterfall-bridge", "values": {"columns": [
			{"label": "FY24", "value": 100, "type": "total"}, {"label": "Price", "value": 20, "type": "delta"},
			{"label": "Churn", "value": -35, "type": "delta"}, {"label": "FY25", "value": 85, "type": "total"}]}}},
		{"slide_type": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Agenda"}],
		 "pattern": {"name": "agenda", "values": {"items": ["Where we are", "What changed", "What we do next"]}, "overrides": {"highlight": 2}}},
		{"slide_type": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Next steps"}],
		 "pattern": {"name": "next-steps", "values": {"actions": [
			{"action": "Agree the scope", "owner": "CFO", "date": "12 Jan"},
			{"action": "Staff the team", "owner": "COO", "date": "20 Jan"}]}}},
		{"slide_type": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Composed"}],
		 "compose": {"direction": "vertical", "segments": [
			{"pattern": {"name": "kpi-3up", "overrides": {"semantic_accent": "negative"}, "values": [
				{"big": "12%", "small": "Churn"}, {"big": "4.1", "small": "NPS drop"}, {"big": "9", "small": "Lost logos"}]}},
			{"pattern": {"name": "stat-hero", "values": {"value": "3.2x", "label": "Payback"}}}]}},
		{"slide_type": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Nested"}],
		 "shape_grid": {"columns": 2, "rows": [{"cells": [
			{"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Context", "size": 14, "color": "dk1"}}},
			{"pattern": {"name": "kpi-2up", "overrides": {"semantic_accent": "positive"}, "values": [
				{"big": "41%", "small": "Margin"}, {"big": "7", "small": "Markets"}]}}]}]}}
	]
}`

func parseExpansionParityDeck(t *testing.T) *PresentationInput {
	t.Helper()
	var input PresentationInput
	if err := json.Unmarshal([]byte(expansionParityDeck), &input); err != nil {
		t.Fatal(err)
	}
	return &input
}

// go-slide-creator-sw78d: validate expands every pattern — slide-level,
// composed and nested — in the context generation expands it in, template
// metadata included, so the shapes it predicts are the shapes generation
// writes.
func TestValidateExpandsPatternsLikeGeneration(t *testing.T) {
	templates := []string{"modern-template", "forest-green"}
	if !testing.Short() {
		templates = testutil.AllTestTemplateNames()
	}
	for _, templateName := range templates {
		t.Run(templateName, func(t *testing.T) {
			input := parseExpansionParityDeck(t)
			input.Template = templateName
			generated, predicted, ok := gridShapesBothWays(t, input, templateName)
			if !ok {
				t.Fatal("generation refuses the fixture deck")
			}
			for si, shapes := range generated {
				if len(shapes) == 0 {
					t.Errorf("slide %d renders no grid shapes; the fixture no longer exercises expansion", si+1)
				}
			}
			assertGridShapeParity(t, generated, predicted)
		})
	}
}

// The read-only passes get the template's metadata with its theme: one parse
// yields both, and a theme_override keeps them.
func TestThemeCarriesTemplateMetadata(t *testing.T) {
	_, theme, _, _ := fitReportGeometry("modern-template", testutil.TemplatesDir())
	if theme == nil || theme.Metadata == nil {
		t.Fatalf("theme carries no template metadata: %+v", theme)
	}
	if len(theme.Metadata.SemanticAccents) == 0 || len(theme.SemanticAccents) == 0 {
		t.Fatalf("modern-template declares semantic accents; theme has metadata %v, mirror %v", theme.Metadata.SemanticAccents, theme.SemanticAccents)
	}
	overridden, _ := theme.ApplyOverride(&types.ThemeOverride{Colors: map[string]string{"accent1": "#123456"}})
	if overridden.Metadata != theme.Metadata || len(overridden.SurfaceTints) != len(theme.SurfaceTints) {
		t.Errorf("theme_override dropped the template metadata: %+v", overridden)
	}
	ctx := slideExpandContext(theme, nil, nil, patterns.LayoutBounds{}, 0, 0, "", 0, 0)
	if ctx.Metadata != theme.Metadata {
		t.Error("a read-only expansion context does not carry the theme's metadata")
	}
}
