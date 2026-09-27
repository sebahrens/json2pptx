package generator

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernSectionTaglineAlignedNativeSourceAndOutput(t *testing.T) {
	const source = "../../templates/modern-template.pptx"
	check := func(path string) {
		t.Helper()
		var layout struct {
			Shapes []struct {
				NonVisual struct {
					Properties struct {
						Name string `xml:"name,attr"`
					} `xml:"cNvPr"`
				} `xml:"nvSpPr"`
				Transform struct {
					Offset struct {
						X int64 `xml:"x,attr"`
						Y int64 `xml:"y,attr"`
					} `xml:"off"`
					Extent struct {
						CX int64 `xml:"cx,attr"`
						CY int64 `xml:"cy,attr"`
					} `xml:"ext"`
				} `xml:"spPr>xfrm"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideLayouts/slideLayout2.xml")), &layout); err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, shape := range layout.Shapes {
			name, frame := shape.NonVisual.Properties.Name, shape.Transform
			switch name {
			case "title":
				found[name] = true
				if frame.Offset.X != 1450428 || frame.Offset.Y != 2352408 || frame.Extent.CX != 9145991 || frame.Extent.CY != 2268577 {
					t.Error("native title geometry changed")
				}
			case "body":
				found[name] = true
				if frame.Offset.X != 1450428 || frame.Offset.Y != 4700000 || frame.Extent.CX != 9903372 || frame.Extent.CY != 2057400 {
					t.Error("tagline not aligned while preserving right edge and vertical frame")
				}
			case "Section Number":
				found[name] = true
				if frame.Offset.X != 7886700 || frame.Offset.Y != 572408 || frame.Extent.CX != 3182938 || frame.Extent.CY != 1600000 {
					t.Error("section-number geometry changed")
				}
			}
		}
		for _, name := range []string{"title", "body", "Section Number"} {
			if !found[name] {
				t.Errorf("missing native %s", name)
			}
		}
	}
	check(source)
	for _, tc := range []struct{ title, tagline string }{
		{"Service performance", "Delivery overview"},
		{"Quarterly operating review and service delivery priorities", "Delivery overview"},
		{"Service performance", "Internal review of service delivery, ownership and reporting requirements"},
	} {
		t.Run(tc.title+tc.tagline, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "section.pptx")
			_, _, err := generateSinglePass(context.Background(), GenerationRequest{
				TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: tc.title},
					{PlaceholderID: "body", Type: ContentText, Value: tc.tagline},
					{PlaceholderID: "Section Number", Type: ContentText, Value: "07"},
				}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			check(output)
			slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
			for _, text := range []string{tc.title, tc.tagline, "07"} {
				if !strings.Contains(slide, text) {
					t.Errorf("required content lost: %s", text)
				}
			}
		})
	}
}
