package generator

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestYellowStatementArtworkIsQuietInGeneratedPackage(t *testing.T) {
	templatePath := "../../templates/modern-yellow.pptx"
	check := func(path string) {
		t.Helper()
		var layout struct {
			Shapes []struct {
				Identity struct {
					ID   string `xml:"id,attr"`
					Name string `xml:"name,attr"`
				} `xml:"nvSpPr>cNvPr"`
				Offset struct {
					X int64 `xml:"x,attr"`
					Y int64 `xml:"y,attr"`
				} `xml:"spPr>xfrm>off"`
				Extent struct {
					CX int64 `xml:"cx,attr"`
					CY int64 `xml:"cy,attr"`
				} `xml:"spPr>xfrm>ext"`
				Shadow struct {
					Value string `xml:"val,attr"`
				} `xml:"spPr>effectLst>outerShdw>schemeClr>alpha"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideLayouts/slideLayout4.xml")), &layout); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, shape := range layout.Shapes {
			if shape.Identity.ID != "5" || shape.Identity.Name != "Freeform: Shape 4" {
				continue
			}
			found++
			if shape.Offset.X != 0 || shape.Offset.Y != -1750154 || shape.Extent.CX != 12192000 || shape.Extent.CY != 2110154 || shape.Offset.Y+shape.Extent.CY != 360000 || shape.Shadow.Value != "8000" {
				t.Fatalf("Statement artwork is not a quiet native edge: %+v", shape)
			}
		}
		if found != 1 {
			t.Fatalf("expected exactly one preserved Statement artwork, got%d", found)
		}
	}
	check(templatePath)
	output := filepath.Join(t.TempDir(), "statement.pptx")
	required := []string{"Required statement parent", "\tRequired statement child", "Required statement conclusion"}
	_, _, err := generateSinglePass(context.Background(), GenerationRequest{TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout4", Content: []ContentItem{{PlaceholderID: "body", Type: ContentBullets, Value: required}}}}})
	if err != nil {
		t.Fatal(err)
	}
	check(output)
	for _, part := range []string{"ppt/theme/theme1.xml", "ppt/slideMasters/slideMaster1.xml", "ppt/slideLayouts/slideLayout4.xml", "ppt/slideLayouts/_rels/slideLayout4.xml.rels"} {
		if readZipFileString(t, templatePath, part) != readZipFileString(t, output, part) {
			t.Fatalf("native payload changed: %s", part)
		}
	}
	slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
	for _, text := range required {
		if strings.Count(slide, "<a:t>"+strings.TrimPrefix(text, "\t")+"</a:t>") != 1 {
			t.Fatalf("required source text missing or duplicated: %q", text)
		}
	}
}
