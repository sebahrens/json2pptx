package generator

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestBusinessTwoContentPreservesReadableNativeLeading(t *testing.T) {
	templatePath := "../../templates/business-template.pptx"
	check := func(path string) {
		t.Helper()
		type level struct {
			Leading struct {
				Value string `xml:"val,attr"`
			} `xml:"lnSpc>spcPct"`
			Font struct {
				Size string `xml:"sz,attr"`
			} `xml:"defRPr"`
		}
		var layout struct {
			Shapes []struct {
				Identity struct {
					Name string `xml:"name,attr"`
				} `xml:"nvSpPr>cNvPr"`
				First  level `xml:"txBody>lstStyle>lvl1pPr"`
				Second level `xml:"txBody>lstStyle>lvl2pPr"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideLayouts/slideLayout5.xml")), &layout); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, shape := range layout.Shapes {
			if shape.Identity.Name != "body" && shape.Identity.Name != "body_2" {
				continue
			}
			if seen[shape.Identity.Name] {
				t.Fatal("duplicate native body placeholder")
			}
			seen[shape.Identity.Name] = true
			if shape.First.Leading.Value != "110000" || shape.Second.Leading.Value != "110000" || shape.First.Font.Size != "1800" || shape.Second.Font.Size != "1600" {
				t.Errorf("native parent/child leading or font size drifted on %s: %+v %+v", shape.Identity.Name, shape.First, shape.Second)
			}
		}
		if !seen["body"] || !seen["body_2"] {
			t.Fatal("native editable bullet columns missing")
		}
		master := readZipFileString(t, path, "ppt/slideMasters/slideMaster1.xml")
		if strings.Count(master, "Confidential 2024") != 1 {
			t.Fatal("native master legal text must remain exactly once")
		}
	}
	check(templatePath)
	output := filepath.Join(t.TempDir(), "business-leading.pptx")
	_, _, err := generateSinglePass(context.Background(), GenerationRequest{
		TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: "slideLayout5", Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentText, Value: "Required title"},
			{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Left complete parent", "\tLeft complete child"}},
			{PlaceholderID: "body_2", Type: ContentBullets, Value: []string{"Right complete parent", "\tRight complete child"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	check(output)
	slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
	for _, text := range []string{"Required title", "Left complete parent", "Left complete child", "Right complete parent", "Right complete child"} {
		if strings.Count(slide, "<a:t>"+text+"</a:t>") != 1 {
			t.Errorf("required complete text %q must remain exactly once", text)
		}
	}
}
