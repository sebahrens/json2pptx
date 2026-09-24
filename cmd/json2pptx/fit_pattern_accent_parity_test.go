package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// A section-keyed pattern after a divider uses accent2 in both validation and
// rendering. The light override makes a real contrast decision observable,
// rather than merely checking that both paths can expand a pattern.
func TestSectionKeyedPatternContrastParityAcrossLocalTemplates(t *testing.T) {
	for _, templateName := range testutil.AllTestTemplateNames() {
		t.Run(templateName, func(t *testing.T) {
			layouts, theme, width, height := fitReportGeometry(templateName, testutil.TemplatesDir())
			if theme == nil {
				t.Fatal("template theme unavailable")
			}
			input := &PresentationInput{
				Template: templateName, OutputFilename: "section-accent-parity.pptx",
				AccentStrategy: "section-keyed",
				ThemeOverride:  &ThemeInput{Colors: map[string]string{"accent2": "#EEEEEE"}},
				Slides: []SlideInput{
					{SlideType: "title"},
					{SlideType: "section"},
					{SlideType: "content", Pattern: &PatternInput{Name: "kpi-inline", Values: json.RawMessage(`[{"big":"42%","small":"Growth"},{"big":"1.2M","small":"ARR"}]`)}},
				},
			}
			applyDefaults(input)
			effectiveTheme, _ := theme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
			expanded, _ := expandPatternsForFit(input, width, height, &effectiveTheme, layouts...)
			grid := expanded.Slides[2].ShapeGrid
			if grid == nil || len(grid.Rows) == 0 || len(grid.Rows[0].Cells) == 0 || grid.Rows[0].Cells[0].Shape == nil {
				t.Fatalf("preflight did not expand KPI pattern: %+v", grid)
			}
			if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"accent2"` {
				t.Fatalf("preflight KPI fill = %s, want section-keyed accent2", got)
			}
			predicted := contrastPredictions(collectFitFindings(input, layouts, width, height, &effectiveTheme))
			found := false
			for _, finding := range predicted {
				if strings.HasPrefix(finding.Path, "/slides/2/") && strings.Contains(finding.Message, "(on #EEEEEE") {
					found = true
				}
			}
			if !found {
				t.Fatalf("no accent2 contrast prediction on slide 3: %+v", predicted)
			}
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
				AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, swap := range result.GenResult.ContrastSwaps {
				if swap.SlideIndex == 2 && swap.BackgroundColor == "#EEEEEE" {
					return
				}
			}
			t.Fatalf("render did not repair accent2 KPI text: %+v", result.GenResult.ContrastSwaps)
		})
	}
}

func TestSectionKeyedComposePreflightUsesSectionAccent(t *testing.T) {
	kpi := PatternInput{Name: "kpi-inline", Values: json.RawMessage(`[{"big":"42%","small":"Growth"},{"big":"1.2M","small":"ARR"}]`)}
	input := &PresentationInput{AccentStrategy: "section-keyed", Slides: []SlideInput{
		{SlideType: "title"}, {SlideType: "section"},
		{SlideType: "content", Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
			{Pattern: kpi, SizePct: 50}, {Pattern: kpi, SizePct: 50},
		}}},
	}}
	expanded := expandComposeForPreflight(input, 12192000, 6858000)
	grid := expanded.Slides[2].ShapeGrid
	if grid == nil {
		t.Fatal("compose preflight did not expand")
	}
	encoded, err := json.Marshal(grid)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"accent2"`) || strings.Contains(string(encoded), `"accent1"`) {
		t.Fatalf("section-keyed compose colors = %s, want accent2 only", encoded)
	}
}
