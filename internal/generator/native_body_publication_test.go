package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

func TestNativeBodyReadabilityUsesPopulatedChildrenNotUnusedStyles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		native bool
		mode   tokens.ViewingMode
		child  string
		want   bool
	}{
		{"native child below present floor", true, tokens.ViewingModePresentation, "Child", true},
		{"same child above read floor", true, tokens.ViewingModeReport, "Child", false},
		{"generic prediction remains advisory", false, tokens.ViewingModePresentation, "Child", false},
		{"empty child and unused level ignored", true, tokens.ViewingModePresentation, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shape := &shapeXML{TextBody: &textBodyXML{ListStyle: &listStyleXML{Inner: `<a:lvl9pPr><a:defRPr sz="600"/></a:lvl9pPr>`}, Paragraphs: []paragraphXML{
				{Runs: []runXML{{Text: "Parent", RunProperties: &runPropertiesXML{FontSize: "1800"}}}},
				{Runs: []runXML{{Text: tc.child, RunProperties: &runPropertiesXML{FontSize: "1200"}}}},
			}}}
			var findings []patterns.FitFinding
			cfg := autofitConfig{findings: &findings, findingPath: "/slides/0/content/1", bodyTypography: tc.native, viewingMode: tc.mode, textRole: tokens.TextRoleBody}
			emitReadabilityFinding(&cfg, shape, textfit.Params{FontSizeHPt: 1800}, textfit.FitResult{FontScale: 90000}, 2)
			if (len(findings) != 0) != tc.want {
				t.Fatalf("findings=%+v want=%v", findings, tc.want)
			}
			if tc.want && (findings[0].Action != "refuse" || findings[0].Fix.Params["actual_pt"] != 10.8 || findings[0].Fix.Params["measurement_source"] != "generated_native_body") {
				t.Fatalf("wrong native finding: %+v", findings[0])
			}
		})
	}
}

func TestDirectGeneratorDoesNotPublishTinyNativeBody(t *testing.T) {
	bullets := make([]string, 10)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("C1-B%02d Service owner confirms the reporting deadline and documents unresolved handoffs before the monthly close. Café teams review résumé details and regional delivery constraints.", i)
	}
	bullets[1] = "\t" + bullets[1]
	for _, mode := range []string{"", "warn", "off", "strict"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%q/existing=%v", mode, existing), func(t *testing.T) {
				dir := t.TempDir()
				output := filepath.Join(dir, "deck.pptx")
				if existing {
					if err := os.WriteFile(output, []byte("original"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				request := GenerationRequest{TemplatePath: filepath.Join(testutil.TemplatesDir(), "blue-corporate.pptx"), OutputPath: output, StrictFit: mode, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: "Required operating review"},
					{PlaceholderID: "body", Type: ContentBullets, Value: bullets},
				}}}}
				before, err := json.Marshal(request.Slides)
				if err != nil {
					t.Fatal(err)
				}
				result, err := Generate(context.Background(), request)
				var failure *patterns.ValidationError
				if result != nil || !errors.As(err, &failure) || failure.Code != patterns.ErrCodeTextBelowReadableMin || failure.Path != "/slides/0/content/1" {
					t.Fatalf("tiny body published or wrong refusal: %+v %v", result, err)
				}
				if failure.Fix == nil || failure.Fix.Kind != "split_bullets" || failure.Fix.Params["max_items"] != 2 || !strings.Contains(failure.Message, "fixed slide count") {
					t.Fatalf("source-preserving grouped remedy missing: %+v", failure)
				}
				after, err := json.Marshal(request.Slides)
				if err != nil || string(before) != string(after) {
					t.Fatal("authored source mutated")
				}
				files, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				if existing {
					data, err := os.ReadFile(output)
					if err != nil || string(data) != "original" || len(files) != 1 {
						t.Fatal("refusal altered destination or leaked artifacts")
					}
				} else if len(files) != 0 {
					t.Fatal("refusal leaked artifacts")
				}
				request.Slides[0].Content[1].Value = bullets[:2]
				request.Slides[0].Content[1].FontSize = 1800
				if result, err := Generate(context.Background(), request); err != nil || result == nil {
					t.Fatalf("readable source-complete continuation control refused: %+v %v", result, err)
				}
			})
		}
	}
}

func TestNativeBodyReadabilitySingleParagraphPreservesSourceGuidance(t *testing.T) {
	var findings []patterns.FitFinding
	cfg := autofitConfig{findings: &findings, findingPath: "/slides/0/content/0", bodyTypography: true, viewingMode: tokens.ViewingModePresentation, textRole: tokens.TextRoleBody}
	emitReadabilityFinding(&cfg, nil, textfit.Params{FontSizeHPt: 1800}, textfit.FitResult{FontScale: 50000}, 1)
	if len(findings) != 1 || findings[0].Action != "refuse" {
		t.Fatalf("expected native body refusal: %+v", findings)
	}
	failure := SourcePreservingParagraphRepair(findings[0].ValidationError, []SlideSpec{{Content: []ContentItem{{Type: ContentText, Value: "Complete required source paragraph"}}}})
	if failure.Fix != nil || strings.Contains(failure.Message, "shorten") || !strings.Contains(failure.Message, "preserve all source") || !strings.Contains(failure.Message, "fixed slide count") {
		t.Fatalf("unsafe or missing manual continuation guidance: %+v", failure)
	}
	advisory := NewReadabilityFinding(ReadabilityFindingInput{EffectiveHPt: 900, Paragraphs: 1, Mode: tokens.ViewingModePresentation, Role: tokens.TextRoleBody})
	if advisory == nil || advisory.Action != "review" || advisory.Fix.Kind != "reduce_text" || !strings.Contains(advisory.Message, "shorten") {
		t.Fatalf("generic prediction policy unexpectedly changed: %+v", advisory)
	}
}
