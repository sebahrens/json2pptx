package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCompiledGridContrastPreflightMatchesGeneration(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "dk2", RGB: "#1B2A4A"},
		{Name: "lt1", RGB: "#FFFFFF"},
	}
	for _, tc := range []struct {
		name       string
		fills      []string
		text       string
		wantSwaps  int
		wantShared int
	}{
		{
			name: "sibling color shared across three fills", fills: []string{"#EEEEEE", "#DDDDDD", "#CCCCCC"},
			text: `{"content":"Shared","color":"#FFFFFF","size":12}`, wantSwaps: 1, wantShared: 3,
		},
		{
			name:  "small caption raises whole body threshold",
			fills: []string{"#8F8F8F"},
			text: `{"paragraphs":[` +
				`{"content":"Large","color":"#FFFFFF","size":28},` +
				`{"content":"Caption","color":"#111111","size":12}]}`,
			wantSwaps: 1,
		},
		{
			name:      "large white text on medium fill remains readable",
			fills:     []string{"#8F8F8F"},
			text:      `{"content":"Large","color":"#FFFFFF","size":28}`,
			wantSwaps: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := GridRowInput{}
			for _, fill := range tc.fills {
				row.Cells = append(row.Cells, &GridCellInput{Shape: &ShapeSpecInput{
					Geometry: "rect", Fill: json.RawMessage(`"` + fill + `"`), Text: json.RawMessage(tc.text),
				}})
			}
			input := &PresentationInput{
				Template: "midnight-blue", OutputFilename: "contrast-parity.pptx",
				Slides: []SlideInput{{SlideType: "blank", ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{row}}}},
			}
			applyDefaults(input)
			predicted := contrastPredictions(collectContrastPreflightFindings(input, nil, theme))
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			var actual []generator.ContrastSwap
			for _, swap := range result.GenResult.ContrastSwaps {
				if strings.HasPrefix(swap.Source, "shape_grid") {
					actual = append(actual, swap)
				}
			}
			if len(actual) != tc.wantSwaps {
				t.Fatalf("generated %d contrast swaps, want %d: %+v", len(actual), tc.wantSwaps, actual)
			}
			if len(predicted) != len(actual) {
				t.Fatalf("predicted %d contrast swaps, generated %d; predictions=%+v swaps=%+v", len(predicted), len(actual), predicted, actual)
			}
			for i, swap := range actual {
				if swap.Cells != tc.wantShared {
					t.Errorf("swap covers %d cells, want %d", swap.Cells, tc.wantShared)
				}
				if !strings.Contains(predicted[i].Message, swap.OriginalColor+" → "+swap.ReplacedColor+" (on "+swap.BackgroundColor) {
					t.Errorf("prediction %d does not match generated swap %+v: %q", i, swap, predicted[i].Message)
				}
				if swap.Cells > 1 && predicted[i].Fix != nil {
					t.Errorf("shared swap must not offer a single-cell repair: %+v", predicted[i].Fix)
				}
			}
		})
	}
}

func TestSharedGridContrastPredictionAcrossLocalTemplates(t *testing.T) {
	for _, templateName := range testutil.AllTestTemplateNames() {
		t.Run(templateName, func(t *testing.T) {
			analysis, err := getOrAnalyzeTemplate(filepath.Join(testutil.TemplatesDir(), templateName+".pptx"), template.NewMemoryCache(0))
			if err != nil {
				t.Fatal(err)
			}
			row := GridRowInput{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"#EEEEEE"`), Text: json.RawMessage(`{"content":"One","color":"#FFFFFF","size":12}`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"#DDDDDD"`), Text: json.RawMessage(`{"content":"Two","color":"#FFFFFF","size":12}`)}},
			}}
			input := &PresentationInput{Template: templateName, OutputFilename: "contrast-parity.pptx",
				Slides: []SlideInput{{SlideType: "blank", ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{row}}}}}
			applyDefaults(input)
			predicted := contrastPredictions(collectContrastPreflightFindings(input, analysis.Layouts, analysis.Theme.Colors))
			if len(predicted) != 1 {
				t.Fatalf("predicted %d swaps, want one group decision: %+v", len(predicted), predicted)
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
			var actual []generator.ContrastSwap
			for _, swap := range result.GenResult.ContrastSwaps {
				if swap.Source == "shape_grid_group" {
					actual = append(actual, swap)
				}
			}
			if len(actual) != 1 || actual[0].Cells != 2 {
				t.Fatalf("generated group swaps = %+v, want one two-cell decision", actual)
			}
			if !strings.Contains(predicted[0].Message, actual[0].OriginalColor+" → "+actual[0].ReplacedColor+" (on "+actual[0].BackgroundColor) {
				t.Errorf("prediction %q disagrees with actual %+v", predicted[0].Message, actual[0])
			}
		})
	}
}

func TestImageLabelContrastPredictionUsesCompiledTextShape(t *testing.T) {
	cell := &GridCellInput{Image: &GridImageInput{Path: "unused.png", Text: &GridImageTextInput{
		Content: "Caption", Color: "#FFFFFF", Size: 12,
	}}}
	input := &PresentationInput{Slides: []SlideInput{{
		Background: &BackgroundInput{Color: "#FFFFFF"},
		ShapeGrid:  &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{cell}}}},
	}}}
	findings := contrastPredictions(collectContrastPreflightFindings(input, nil, []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
	}))
	if len(findings) != 1 || !strings.HasSuffix(findings[0].Path, "/image/text") {
		t.Fatalf("image label prediction = %+v, want one finding on image/text", findings)
	}
	if findings[0].Fix != nil {
		t.Errorf("image label has no shape.text repair target: %+v", findings[0].Fix)
	}
}

func TestCompositeAndNestedGridShareContrastDecision(t *testing.T) {
	shape := &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"#EEEEEE"`),
		Text: json.RawMessage(`{"content":"Label","color":"#FFFFFF","size":12}`)}
	nested := &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: shape}}}}}
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{
		Rows: []GridRowInput{{Cells: []*GridCellInput{
			{Composite: &jsonschema.CompositeInput{Text: shape, SubDiagram: vennFrameFixture()}},
			{Grid: nested},
		}}},
	}}}}
	findings := contrastPredictions(collectContrastPreflightFindings(input, nil, []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
	}))
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "shared across 2 cells") {
		t.Fatalf("composite/nested prediction = %+v, want one shared decision", findings)
	}
	if findings[0].Fix != nil {
		t.Errorf("shared contrast decision cannot be repaired in one cell: %+v", findings[0].Fix)
	}
}

func TestTemplatePlaceholderContrastRepairsOnlyVisibleText(t *testing.T) {
	for _, tc := range []struct {
		template, layoutID, background, original string
		wantSwaps                                int
	}{
		{"business-template", "slideLayout1", "#000000", "#000000", 1},
	} {
		t.Run(tc.template, func(t *testing.T) {
			input := &PresentationInput{Template: tc.template, OutputFilename: "placeholder-contrast.pptx",
				Slides: []SlideInput{{SlideType: "title", LayoutID: tc.layoutID,
					Background: &BackgroundInput{Color: tc.background},
					Content:    []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Contrast test")}},
				}}}
			applyDefaults(input)
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			var actual []generator.ContrastSwap
			for _, swap := range result.GenResult.ContrastSwaps {
				if swap.Source == "lstStyle" || swap.Source == "master-txStyles" {
					actual = append(actual, swap)
				}
			}
			if len(actual) != tc.wantSwaps {
				t.Fatalf("visible placeholder repairs = %+v, want %d", actual, tc.wantSwaps)
			}
			for _, swap := range actual {
				if swap.OriginalColor != tc.original || swap.RatioAfter < 4.5 {
					t.Errorf("wrong modified foreground or unreadable repair: %+v", swap)
				}
			}
		})
	}
}

func TestModifiedTemplateTextColorPredictionUsesVisibleForeground(t *testing.T) {
	analysis, err := getOrAnalyzeTemplate(filepath.Join(testutil.TemplatesDir(), "blue-corporate.pptx"), template.NewMemoryCache(0))
	if err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{Template: "blue-corporate", Slides: []SlideInput{{
		SlideType: "title", LayoutID: "slideLayout3", Background: &BackgroundInput{Color: "#FFFFFF"},
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Contrast test")}},
	}}}
	applyDefaults(input)
	findings := contrastPredictions(collectContrastPreflightFindings(input, analysis.Layouts, analysis.Theme.Colors))
	if len(findings) != 2 {
		t.Fatalf("modified title and retained section-number predictions = %+v, want two", findings)
	}
	for _, finding := range findings {
		if !strings.Contains(finding.Message, "#F2F2F2 → #44546A") {
			t.Errorf("prediction lost visible foreground: %+v", finding)
		}
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
	var rendered []generator.ContrastSwap
	for _, swap := range result.GenResult.ContrastSwaps {
		if swap.Source == "lstStyle" || swap.Source == "layout-lstStyle" || swap.Source == "master-txStyles" {
			rendered = append(rendered, swap)
		}
	}
	if len(rendered) != len(findings) {
		t.Fatalf("predicted %d swaps, rendered %d: predictions=%+v rendered=%+v", len(findings), len(rendered), findings, rendered)
	}
	for _, swap := range rendered {
		if swap.OriginalColor != "#F2F2F2" || swap.ReplacedColor != "#44546A" || swap.RatioAfter < 4.5 {
			t.Errorf("rendered color disagrees with prediction: %+v", swap)
		}
	}
}

func TestContrastPreflightResolvesInjectedSectionNumberSlot(t *testing.T) {
	layout := &types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{
		{ID: "body", Type: types.PlaceholderBody},
		{ID: "Decorative Number", Type: types.PlaceholderBody, Role: types.PlaceholderRoleSectionNumber},
		{ID: "section_number", Type: types.PlaceholderBody},
		{ID: "Section Number", Type: types.PlaceholderBody, Role: types.PlaceholderRoleSectionNumber},
	}}
	if got := findContrastPlaceholderByID("section_number", layout); got == nil || got.ID != "Section Number" {
		t.Errorf("injected section_number resolved to %+v, want named Section Number", got)
	}
	if got := findContrastPlaceholderByID("body", layout); got == nil || got.ID != "body" {
		t.Errorf("exact body resolved to %+v", got)
	}
	if got := findContrastPlaceholderByID("missing", layout); got != nil {
		t.Errorf("unknown placeholder resolved to %+v", got)
	}
	layout.Placeholders = layout.Placeholders[:2]
	if got := findContrastPlaceholderByID("large_number", layout); got == nil || got.ID != "Decorative Number" {
		t.Errorf("role-only section slot resolved to %+v", got)
	}
	layout.Placeholders[0].Index = 1
	if got := findContrastPlaceholderByID("section_number", layout); got == nil || got.ID != "body" {
		t.Errorf("idx=1 section fallback resolved to %+v", got)
	}
}

func TestAuthoredBackgroundPlaceholderParityAcrossLocalTemplates(t *testing.T) {
	for _, templateName := range testutil.AllTestTemplateNames() {
		t.Run(templateName, func(t *testing.T) {
			analysis, err := getOrAnalyzeTemplate(filepath.Join(testutil.TemplatesDir(), templateName+".pptx"), template.NewMemoryCache(0))
			if err != nil {
				t.Fatal(err)
			}
			var chosen *types.LayoutMetadata
			var title *types.PlaceholderInfo
			for i := range analysis.Layouts {
				for pi := range analysis.Layouts[i].Placeholders {
					ph := &analysis.Layouts[i].Placeholders[pi]
					if ph.ID == "title" && ph.Type == types.PlaceholderTitle && (ph.FontColor != "" || ph.InheritedFontColor != "") {
						chosen, title = &analysis.Layouts[i], ph
						break
					}
				}
				if chosen != nil {
					break
				}
			}
			if chosen == nil {
				t.Fatal("no title placeholder with a resolved color")
			}
			foreground, mods := title.FontColor, title.FontColorMods
			if foreground == "" {
				foreground, mods = title.InheritedFontColor, title.InheritedFontColorMods
			}
			background := template.ResolveBackgroundRefHexWithMods(foreground, mods, analysis.Theme.Colors)
			if background == "" {
				t.Fatal("title color is not resolvable against the template theme")
			}
			input := &PresentationInput{Template: templateName, OutputFilename: "placeholder-parity.pptx", Slides: []SlideInput{{
				SlideType: "title", LayoutID: chosen.ID, Background: &BackgroundInput{Color: background},
				Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Contrast test")}},
			}}}
			applyDefaults(input)
			predicted := contrastPredictions(collectContrastPreflightFindings(input, analysis.Layouts, analysis.Theme.Colors))
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			var rendered []generator.ContrastSwap
			for _, swap := range result.GenResult.ContrastSwaps {
				if swap.Source == "run" || swap.Source == "lstStyle" || swap.Source == "layout-lstStyle" || swap.Source == "master-txStyles" {
					rendered = append(rendered, swap)
				}
			}
			if len(predicted) != len(rendered) {
				t.Fatalf("layout %s on %s: predicted %d, rendered %d; predictions=%+v rendered=%+v", chosen.ID, background, len(predicted), len(rendered), predicted, rendered)
			}
			matched := make([]bool, len(predicted))
			for _, swap := range rendered {
				want := swap.OriginalColor + " → " + swap.ReplacedColor + " (on " + swap.BackgroundColor
				found := false
				for i, finding := range predicted {
					if !matched[i] && strings.Contains(finding.Message, want) {
						matched[i], found = true, true
						break
					}
				}
				if !found {
					t.Errorf("layout %s: no matching prediction for rendered swap %+v; predictions=%+v", chosen.ID, swap, predicted)
				}
			}
		})
	}
}
