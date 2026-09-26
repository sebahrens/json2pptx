package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

func TestDirectStrictGeneratorDoesNotPublishParagraphLoss(t *testing.T) {
	bullets := make([]string, 10)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("P%02d %s", i, strings.Repeat("Required ownership and reporting evidence. ", 24))
	}
	for _, existing := range []bool{false, true} {
		for _, mode := range []string{"", "warn", "off", "strict"} {
			t.Run(fmt.Sprintf("mode=%q/existing=%v", mode, existing), func(t *testing.T) {
				dir := t.TempDir()
				output := filepath.Join(dir, "deck.pptx")
				if existing {
					if err := os.WriteFile(output, []byte("original-destination"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				request := GenerationRequest{TemplatePath: filepath.Join(testutil.RepoRoot(), "templates", "abstract.pptx"), OutputPath: output, StrictFit: mode, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout3", Content: []ContentItem{{PlaceholderID: "body", Type: ContentBullets, Value: bullets}}}}}
				result, err := Generate(context.Background(), request)
				if result != nil || !errors.Is(err, patterns.ErrTextTrimmed) {
					t.Fatalf("direct strict source loss published: result=%+v err=%v", result, err)
				}
				var loss *patterns.ValidationError
				if !errors.As(err, &loss) || loss.Code != patterns.ErrCodeTextTrimmed || loss.Path == "" {
					t.Fatalf("structured loss finding missing: %v", err)
				}
				files, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				if existing {
					data, err := os.ReadFile(output)
					if err != nil || string(data) != "original-destination" || len(files) != 1 {
						t.Fatalf("refusal changed original destination or leaked temporary output: %s %v %v", data, err, files)
					}
				} else if len(files) != 0 {
					t.Fatalf("refusal left output or temporary file: %v", files)
				}
				request.Slides[0].Content[0].Value = []string{"Complete short source statement"}
				if result, err := Generate(context.Background(), request); err != nil || result == nil {
					t.Fatalf("no-loss direct strict control refused: result=%+v err=%v", result, err)
				}
			})
		}
	}
}

func TestActualParagraphLossIsBlocking(t *testing.T) {
	for _, mode := range []string{"overflow", "readability"} {
		t.Run(mode, func(t *testing.T) {
			var paragraphs []paragraphXML
			var texts []string
			for i := 0; i < 14; i++ {
				text := fmt.Sprintf("P%02d Required service ownership and reporting evidence", i)
				texts = append(texts, text)
				paragraphs = append(paragraphs, paragraphXML{Runs: []runXML{{Text: text}}})
			}
			shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: paragraphs}}
			params := textfit.Params{Paragraphs: texts, WidthEMU: 4000000, HeightEMU: 1500000, FontSizeHPt: 2000, FontName: "Calibri"}
			var findings []patterns.FitFinding
			config := autofitConfig{findings: &findings, findingPath: "/slides/0/content/1"}
			code := patterns.ErrCodeTextTrimmed
			if mode == "overflow" {
				trimOverflowParagraphs(shape, params, &config)
			} else {
				code = patterns.ErrCodeReadabilityTrimmed
				params.HeightEMU = 3000000
				trimForReadability(shape, params, 90000, &config)
			}
			if len(shape.TextBody.Paragraphs) >= len(paragraphs) {
				t.Fatal("fixture did not actually drop authored paragraphs")
			}
			for _, finding := range findings {
				if finding.Code == code {
					if finding.Action != "refuse" || finding.Path != config.findingPath {
						t.Fatalf("actual text loss not blocking: %+v", finding)
					}
					return
				}
			}
			t.Fatalf("actual missing content emitted no %s finding", code)
		})
	}
}

func TestPredictedParagraphLossIsBlocking(t *testing.T) {
	for _, code := range []string{patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed} {
		t.Run(code, func(t *testing.T) {
			texts := make([]string, 8)
			for i := range texts {
				texts[i] = fmt.Sprintf("P%02d %s", i, strings.Repeat("Required service evidence. ", 3))
			}
			// Search the physical height boundary: both overflowing and barely
			// fitting-but-trimmed-for-readability bodies must block source loss.
			for height := int64(500000); height <= 4000000; height += 10000 {
				findings := DetectTextAutofitPreflight(TextAutofitPreflightInput{Path: "/slides/0/content/1", Paragraphs: texts, WidthEMU: 5000000, HeightEMU: height, FontSizeHPt: 2000, FontName: "Calibri"})
				for _, finding := range findings {
					if finding.Code != code {
						continue
					}
					if finding.Action != "refuse" {
						t.Fatalf("predicted source loss not blocking: %+v", finding)
					}
					return
				}
			}
			t.Fatal("fixture never reached the requested source-loss condition")
		})
	}
	if findings := DetectTextAutofitPreflight(TextAutofitPreflightInput{Paragraphs: []string{"Short, complete body"}, WidthEMU: 9000000, HeightEMU: 5000000, FontSizeHPt: 2000, FontName: "Calibri"}); len(findings) != 0 {
		t.Fatalf("no-loss control drew findings: %+v", findings)
	}
}
