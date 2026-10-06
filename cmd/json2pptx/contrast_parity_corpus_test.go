//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// TestContrastParityCorpus shares the existing integration corpus scope but
// compares the decisions an agent sees in validate with actual generator
// repairs. It does not launch LibreOffice; CorpusHeadlessOpen covers package
// opening separately. AllTestTemplateNames includes ignored p-style locally.
func TestContrastParityCorpus(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "examples", "*.json"))
	if err != nil || len(examples) == 0 {
		t.Fatalf("find example corpus: %v (%d files)", err, len(examples))
	}
	templatesDir := testutil.TemplatesDir()
	for _, templateName := range testutil.AllTestTemplateNames() {
		layouts, theme, width, height := fitReportGeometry(templateName, templatesDir)
		if layouts == nil || theme == nil {
			t.Fatalf("cannot analyze template %q", templateName)
		}
		for _, examplePath := range examples {
			t.Run(strings.TrimSuffix(filepath.Base(examplePath), ".json")+"/"+templateName, func(t *testing.T) {
				data, err := os.ReadFile(examplePath)
				if err != nil {
					t.Fatal(err)
				}
				var input PresentationInput
				if err := json.Unmarshal(data, &input); err != nil {
					t.Fatal(err)
				}
				input.Template = templateName
				input.OutputFilename = "contrast-parity.pptx"
				// Resolve relative image paths against the example's own
				// directory, as the CLI does for a -json file.
				resolveLocalAssetPaths(input.Slides, filepath.Dir(examplePath))
				applyDefaults(&input)
				resolveCanonicalLayoutIDs(input.Slides, layouts)
				effectiveTheme := *theme
				if input.ThemeOverride != nil {
					effectiveTheme, _ = effectiveTheme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
				}
				predicted, err := contrastDecisionsFromFindings(contrastPredictions(collectFitFindings(&input, layouts, width, height, &effectiveTheme)))
				if err != nil {
					t.Fatal(err)
				}
				result, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{
					OutputDir: t.TempDir(), TemplatesDir: templatesDir, StrictFit: "off", OutputValidation: "off",
					AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
				})
				if cleanup != nil {
					defer cleanup()
				}
				if err != nil {
					t.Fatal(err)
				}
				actual := make(map[contrastDecision]bool)
				for _, swap := range result.GenResult.ContrastSwaps {
					decision := contrastDecision{Slide: swap.SlideIndex,
						Original: strings.ToUpper(swap.OriginalColor), Replacement: strings.ToUpper(swap.ReplacedColor),
						Background: strings.ToUpper(swap.BackgroundColor)}
					actual[decision] = true
				}
				if !reflect.DeepEqual(predicted, actual) {
					sample := result.GenResult.ContrastSwaps
					if len(sample) > 5 {
						sample = sample[:5]
					}
					t.Errorf("contrast decision-set mismatch: predicted=%v actual=%v first_swaps=%+v", predicted, actual, sample)
				}
			})
		}
	}
}

// TestExpansionParityCorpus is go-slide-creator-sw78d over the example corpus:
// on every template, the grid shapes validate predicts for a deck — slide
// patterns, compose envelopes and nested cell patterns expanded with the
// template's metadata — are byte for byte the shapes generation writes, and
// generation leaves the parsed deck as it found it (go-slide-creator-8jp05).
func TestExpansionParityCorpus(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "examples", "*.json"))
	if err != nil || len(examples) == 0 {
		t.Fatalf("find example corpus: %v (%d files)", err, len(examples))
	}
	for _, templateName := range testutil.AllTestTemplateNames() {
		for _, examplePath := range examples {
			t.Run(strings.TrimSuffix(filepath.Base(examplePath), ".json")+"/"+templateName, func(t *testing.T) {
				data, err := os.ReadFile(examplePath)
				if err != nil {
					t.Fatal(err)
				}
				var input PresentationInput
				if err := json.Unmarshal(data, &input); err != nil {
					t.Fatal(err)
				}
				input.Template = templateName
				resolveLocalAssetPaths(input.Slides, filepath.Dir(examplePath))
				generated, predicted, ok := gridShapesBothWays(t, &input, templateName)
				if !ok {
					t.Skip("generation refuses this example on this template")
				}
				assertGridShapeParity(t, generated, predicted)
			})
		}
	}
}
