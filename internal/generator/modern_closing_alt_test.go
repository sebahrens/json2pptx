package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"testing"
)

func TestModernClosingArtworkDescriptionRetained(t *testing.T) {
	const source = "../../templates/modern-template.pptx"
	const expected = "Decorative pale lavender wash fading into white\n\nAI-generated content may be incorrect."
	check := func(t *testing.T, path string) {
		t.Helper()
		var layout struct {
			Pictures []struct {
				Properties struct {
					ID          int    `xml:"id,attr"`
					Name        string `xml:"name,attr"`
					Description string `xml:"descr,attr"`
				} `xml:"nvPicPr>cNvPr"`
			} `xml:"cSld>spTree>pic"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideLayouts/slideLayout5.xml")), &layout); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, picture := range layout.Pictures {
			if picture.Properties.ID == 8 && picture.Properties.Name == "Picture 7" {
				found++
				if picture.Properties.Description != expected {
					t.Errorf("native Closing artwork description=%q, want%q", picture.Properties.Description, expected)
				}
			}
		}
		if found != 1 {
			t.Fatalf("native Closing artwork count=%d, want1", found)
		}
	}
	check(t, source)
	for _, exclude := range []bool{false, true} {
		t.Run(fmt.Sprintf("exclude-template-slides=%v", exclude), func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "closing-description.pptx")
			_, err := Generate(context.Background(), GenerationRequest{
				TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: exclude,
				Slides: []SlideSpec{{LayoutID: "slideLayout5", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: "Accessibility metadata control"},
					{PlaceholderID: "subtitle", Type: ContentText, Value: "Original artwork and text geometry remain unchanged"},
				}}}, ValidateOutput: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			check(t, output)
		})
	}
}
