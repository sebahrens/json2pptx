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
