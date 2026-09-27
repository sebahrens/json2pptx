package generator

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernTitleSectionQuietArtwork(t *testing.T) {
	const source = "../../templates/modern-template.pptx"
	check := func(t *testing.T, path, part string) {
		t.Helper()
		var layout struct {
			Pictures []struct {
				Blip struct {
					Embed   string `xml:"embed,attr"`
					Opacity []struct {
						Amount int `xml:"amt,attr"`
					} `xml:"alphaModFix"`
				} `xml:"blipFill>blip"`
			} `xml:"cSld>spTree>pic"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, part)), &layout); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, picture := range layout.Pictures {
			if picture.Blip.Embed != "rId2" {
				continue
			}
			found++
			if len(picture.Blip.Opacity) != 1 || picture.Blip.Opacity[0].Amount != 15000 {
				t.Errorf("%s: native artwork does not use selected 15%% opacity", part)
			}
		}
		if found != 1 {
			t.Errorf("%s: found %d native rId2 artwork pictures, want one", part, found)
		}
	}
	for _, layout := range []string{"slideLayout1", "slideLayout2"} {
		part := "ppt/slideLayouts/" + layout + ".xml"
		check(t, source, part)
		for _, title := range []string{"Service performance", "Quarterly operating review and service delivery priorities"} {
			t.Run(layout+"/"+title, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "quiet-art.pptx")
				secondary := "body"
				if layout == "slideLayout1" {
					secondary = "subtitle"
				}
				content := []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: title},
					{PlaceholderID: secondary, Type: ContentText, Value: "Delivery overview"},
				}
				if layout == "slideLayout2" {
					content = append(content, ContentItem{PlaceholderID: "Section Number", Type: ContentText, Value: "07"})
				}
				_, _, err := generateSinglePass(context.Background(), GenerationRequest{
					TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true,
					Slides: []SlideSpec{{LayoutID: layout, Content: content}},
				})
				if err != nil {
					t.Fatal(err)
				}
				check(t, output, part)
				slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
				for _, item := range content {
					if !strings.Contains(slide, item.Value.(string)) {
						t.Errorf("wording lost: %v", item.Value)
					}
				}
			})
		}
	}
}
