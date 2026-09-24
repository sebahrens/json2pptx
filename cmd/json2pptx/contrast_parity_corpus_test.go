//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

var predictedContrastColors = regexp.MustCompile(`— (#[0-9A-Fa-f]{6}) → (#[0-9A-Fa-f]{6}) \(on (#[0-9A-Fa-f]{6}),`)

type contrastDecision struct {
	Slide                 int
	Original, Replacement string
	Background            string
}

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

func contrastDecisionsFromFindings(findings []patterns.FitFinding) (map[contrastDecision]bool, error) {
	decisions := make(map[contrastDecision]bool)
	for _, finding := range findings {
		colors := predictedContrastColors.FindStringSubmatch(finding.Message)
		if len(colors) != 4 {
			return nil, fmt.Errorf("cannot read predicted contrast colors from %q", finding.Message)
		}
		slide := slidepath.SlideIndex(finding.Path)
		if slide < 0 {
			return nil, fmt.Errorf("invalid contrast finding path %q", finding.Path)
		}
		decision := contrastDecision{Slide: slide, Original: strings.ToUpper(colors[1]),
			Replacement: strings.ToUpper(colors[2]), Background: strings.ToUpper(colors[3])}
		decisions[decision] = true
	}
	return decisions, nil
}
