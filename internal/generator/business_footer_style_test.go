package generator

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestBusinessNativeFooterPreservesReadableStyleAndLegalText(t *testing.T) {
	templatePath := "../../templates/business-template.pptx"
	assertFooterStyle := func(t *testing.T, path string) {
		t.Helper()
		var master struct {
			Shapes []struct {
				Text  []string `xml:"txBody>p>r>t"`
				Style struct {
					Size string `xml:"sz,attr"`
					Fill struct {
						Scheme string `xml:"val,attr"`
						Tint   struct {
							Value string `xml:"val,attr"`
						} `xml:"tint"`
					} `xml:"solidFill>schemeClr"`
				} `xml:"txBody>lstStyle>lvl1pPr>defRPr"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideMasters/slideMaster1.xml")), &master); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, shape := range master.Shapes {
			if strings.Join(shape.Text, "") != "Confidential 2024" {
				continue
			}
			found++
			if shape.Style.Size != "1323" || shape.Style.Fill.Scheme != "tx1" || shape.Style.Fill.Tint.Value != "90000" {
				t.Errorf("native footer size/color drifted: %+v", shape.Style)
			}
		}
		if found != 1 {
			t.Errorf("native legal footer count=%d; want1", found)
		}
	}
	assertFooterStyle(t, templatePath)
	for _, layout := range []string{"slideLayout3", "slideLayout4"} {
		t.Run(layout, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "business-footer.pptx")
			_, _, err := generateSinglePass(context.Background(), GenerationRequest{
				TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: layout, Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			assertFooterStyle(t, output)
			var slide struct {
				Text []string `xml:"cSld>spTree>sp>txBody>p>r>t"`
			}
			if err := xml.Unmarshal([]byte(readZipFileString(t, output, "ppt/slides/slide1.xml")), &slide); err != nil {
				t.Fatal(err)
			}
			text := strings.Join(slide.Text, "\n")
			for _, required := range []string{"Required title", "Confidential 2024"} {
				if strings.Count(text, required) != 1 {
					t.Errorf("required text %q must remain exactly once", required)
				}
			}
		})
	}
}

func TestBusinessTitleBackgroundPreservesQuietNativeAccentFill(t *testing.T) {
	templatePath := "../../templates/business-template.pptx"
	check := func(path string) {
		t.Helper()
		var layout struct {
			Fill struct {
				Solid *struct {
					Color struct {
						Scheme string `xml:"val,attr"`
						Mod    struct {
							Value string `xml:"val,attr"`
						} `xml:"lumMod"`
						Off struct {
							Value string `xml:"val,attr"`
						} `xml:"lumOff"`
					} `xml:"schemeClr"`
				} `xml:"solidFill"`
				Gradient *struct{} `xml:"gradFill"`
			} `xml:"cSld>bg>bgPr"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideLayouts/slideLayout4.xml")), &layout); err != nil {
			t.Fatal(err)
		}
		if layout.Fill.Gradient != nil || layout.Fill.Solid == nil {
			t.Fatal("Title must retain a native solid background, not the banded gradient")
		}
		color := layout.Fill.Solid.Color
		if color.Scheme != "accent1" || color.Mod.Value != "5000" || color.Off.Value != "95000" {
			t.Errorf("native light accent fill drifted: %+v", color)
		}
	}
	check(templatePath)
	output := filepath.Join(t.TempDir(), "title-background.pptx")
	_, _, err := generateSinglePass(context.Background(), GenerationRequest{
		TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: "slideLayout4", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	check(output)
}
