package generator

import (
	"archive/zip"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestBodySizeForDensity(t *testing.T) {
	for _, tc := range []struct {
		name                string
		source, count, want int
	}{
		{"small template", 1400, 5, 1800},
		{"large template", 3200, 5, 1800},
		{"in range", 2000, 5, 2000},
		{"upper boundary", 1800, 7, 1800},
		{"dense", 2200, 10, 1400},
		{"dense in range", 1200, 10, 1200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := BodySizeForDensity(tc.source, tc.count)
			if got != tc.want {
				t.Errorf("size = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPopulatedBodyTypographyIsTemplateIndependent(t *testing.T) {
	for _, source := range []int{1400, 2000, 3200} {
		shape := &shapeXML{
			NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
			ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 9000000, CY: 4500000}}},
			TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{}, ListStyle: &listStyleXML{},
				Paragraphs: []paragraphXML{emptyParagraph()}},
		}
		var findings []patterns.FitFinding
		err := populateShapeText(shape, ContentItem{PlaceholderID: "body", Type: ContentBullets,
			Value: []string{"Manual handoffs", "Five-day close", "Fragmented reporting", "Rework", "Delayed decisions"}}, 0, "Arial",
			withInheritedTextStyle(template.InheritedTextStyle{SizeHPt: source}, "Arial"),
			withFindingsCollector(&findings, "/slides/0/content/0"))
		if err != nil {
			t.Fatal(err)
		}
		want := "1800"
		if source == 2000 {
			want = ""
		}
		for _, para := range shape.TextBody.Paragraphs {
			for _, run := range para.Runs {
				if got := run.RunProperties.FontSize; got != want {
					t.Errorf("source=%d run size=%q, want %q", source, got, want)
				}
			}
		}
		found := false
		for _, f := range findings {
			if f.Code == patterns.ErrCodeTextSizeOffTarget {
				found = true
				if f.Action != "info" || f.Path != "/slides/0/content/0" {
					t.Errorf("source=%d finding=%+v", source, f)
				}
			}
		}
		if found != (source != 2000) {
			t.Errorf("source=%d off-target finding=%v, want %v (%v)", source, found, source != 2000, findings)
		}
		if !strings.Contains(shape.TextBody.BodyProperties.Inner, "normAutofit") {
			t.Errorf("source=%d missing autofit", source)
		}
	}
}

func TestExplicitBodyFontSizeBypassesNormalization(t *testing.T) {
	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
		ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 9000000, CY: 4500000}}},
		TextBody:            &textBodyXML{BodyProperties: &bodyPropertiesXML{}, ListStyle: &listStyleXML{}, Paragraphs: []paragraphXML{emptyParagraph()}},
	}
	if err := populateShapeText(shape, ContentItem{PlaceholderID: "body", Type: ContentBullets,
		Value: []string{"One", "Two", "Three", "Four", "Five"}, FontSize: 2400}, 0, "Arial"); err != nil {
		t.Fatal(err)
	}
	for _, para := range shape.TextBody.Paragraphs {
		for _, run := range para.Runs {
			if run.RunProperties.FontSize != "2400" {
				t.Errorf("explicit author size lost: %+v", run.RunProperties)
			}
		}
	}
}

func TestBodyTypographyReportsAutofitBelowDensityTarget(t *testing.T) {
	shape := denseBodyShape(6, "1800")
	var findings []patterns.FitFinding
	applySmartAutofitWithOptions(shape,
		withBodyTypography(),
		withFindingsCollector(&findings, "/slides/0/content/0"))
	for _, finding := range findings {
		if finding.Code == patterns.ErrCodeTextSizeOffTarget {
			if finding.Action != "review" || finding.Fix == nil || finding.Fix.Kind != "reduce_text" {
				t.Fatalf("undersized result did not give actionable review: %+v", finding)
			}
			if pt, ok := finding.Fix.Params["actual_pt"].(float64); !ok || pt >= 16 {
				t.Fatalf("actual_pt = %v, want <16", finding.Fix.Params["actual_pt"])
			}
			return
		}
	}
	t.Fatalf("missing TEXT_SIZE_OFF_TARGET review: %+v", findings)
}

func TestBodyTypographyPreflightMatchesTemplateNormalization(t *testing.T) {
	for _, source := range []int{1400, 2000, 3200} {
		findings := DetectTextAutofitPreflight(TextAutofitPreflightInput{
			Path: "/slides/0/content/0", Paragraphs: []string{"Manual handoffs", "Five-day close", "Fragmented reporting", "Rework", "Delayed decisions"},
			WidthEMU: 9000000, HeightEMU: 4500000, FontSizeHPt: source, FontName: "Arial", NormalizeBody: true,
		})
		var found *patterns.FitFinding
		for i := range findings {
			if findings[i].Code == patterns.ErrCodeTextSizeOffTarget {
				found = &findings[i]
			}
		}
		if source == 2000 {
			if found != nil {
				t.Errorf("in-range source %d unexpectedly reported: %+v", source, found)
			}
		} else if found == nil || found.Action != "info" || found.Path != "/slides/0/content/0" {
			t.Errorf("source %d: expected info normalization finding, got %+v", source, findings)
		}
	}
}

func TestDisplayPlaceholderKeepsDeclaredSize(t *testing.T) {
	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
		ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 9000000, CY: 4500000}}},
		TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{},
			ListStyle:  &listStyleXML{Inner: `<a:lvl1pPr><a:defRPr sz="4800"/></a:lvl1pPr>`},
			Paragraphs: []paragraphXML{emptyParagraph()}},
	}
	var findings []patterns.FitFinding
	if err := populateShapeText(shape, ContentItem{PlaceholderID: "body", Type: ContentText, Value: "02"}, 0, "Arial",
		withFindingsCollector(&findings, "/slides/0/content/0")); err != nil {
		t.Fatal(err)
	}
	if got := parseSzAttr(shape.TextBody.ListStyle.Inner); got != 4800 {
		t.Errorf("display size = %d, want 4800", got)
	}
	for _, f := range findings {
		if f.Code == patterns.ErrCodeTextSizeOffTarget {
			t.Errorf("display placeholder should not emit body-size finding: %+v", f)
		}
	}
}

func TestGeneratedFiveBulletsUsePortableBodySize(t *testing.T) {
	for _, templatePath := range testutil.TestTemplatePaths() {
		name := filepath.Base(templatePath)
		if name != "modern-template.pptx" && name != "business-template.pptx" && name != "p-style.pptx" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(templatePath)
			if err != nil {
				t.Fatal(err)
			}
			layouts, err := template.ParseLayouts(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			layoutID := layout.ResolveAllCanonicalLayouts(layouts)["content"]
			if layoutID == "" {
				t.Fatal("no canonical content layout")
			}
			var templateHPt int
			for _, candidate := range layouts {
				if candidate.ID == layoutID {
					for _, ph := range candidate.Placeholders {
						if ph.ID == "body" || ph.Type == "body" {
							templateHPt = ph.FontSize
							break
						}
					}
				}
			}
			output := filepath.Join(t.TempDir(), "five-bullets.pptx")
			_, err = Generate(context.Background(), GenerationRequest{
				TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: layoutID, Content: []ContentItem{{
					PlaceholderID: "body", Type: ContentBullets,
					Value: []string{"Manual handoffs", "Five-day close", "Fragmented reporting", "Rework", "Delayed decisions"},
				}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			slideXML := string(zipSlideXML(t, &z.Reader, 1))
			pos := strings.Index(slideXML, ">Manual handoffs<")
			if pos < 0 {
				t.Fatal("rendered slide missing first bullet")
			}
			runStart := strings.LastIndex(slideXML[:pos], "<a:r>")
			if runStart < 0 {
				t.Fatal("first bullet is not in a text run")
			}
			effective, _ := BodySizeForDensity(templateHPt, 5)
			got := parseSzAttr(slideXML[runStart:pos])
			if got == 0 {
				got = templateHPt // an in-range template size may remain inherited
			}
			if got != effective || got < 1600 || got > 2000 {
				t.Fatalf("first bullet size = %dpt/100 (template %d), want %d", got, templateHPt, effective)
			}
		})
	}
}
