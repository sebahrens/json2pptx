package generator

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlueNativeBodyPreservesQuietArtworkAndChildLeading(t *testing.T) {
	templatePath := "../../templates/blue-corporate.pptx"
	check := func(path string) {
		t.Helper()
		type paragraphStyle struct {
			Leading struct {
				Value string `xml:"val,attr"`
			} `xml:"lnSpc>spcPct"`
			After struct {
				Value string `xml:"val,attr"`
			} `xml:"spcAft>spcPts"`
			Font struct {
				Size string `xml:"sz,attr"`
			} `xml:"defRPr"`
		}
		var layout struct {
			Shapes []struct {
				Identity struct {
					Name string `xml:"name,attr"`
				} `xml:"nvSpPr>cNvPr"`
				First  paragraphStyle `xml:"txBody>lstStyle>lvl1pPr"`
				Second paragraphStyle `xml:"txBody>lstStyle>lvl2pPr"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideLayouts/slideLayout2.xml")), &layout); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, shape := range layout.Shapes {
			if shape.Identity.Name != "body" {
				continue
			}
			found++
			if shape.First.After.Value != "0" || shape.Second.After.Value != "" {
				t.Errorf("unchanged native paragraph separation drifted: parent=%q child=%q", shape.First.After.Value, shape.Second.After.Value)
			}
			for _, style := range []paragraphStyle{shape.First, shape.Second} {
				if style.Leading.Value != "140000" || style.Font.Size != "2000" {
					t.Errorf("native parent/child leading or 20pt font drifted: %+v", style)
				}
			}
		}
		if found != 1 {
			t.Fatalf("native body count=%d, want 1", found)
		}
		for part, count := range map[string]int{"ppt/media/image3.svg": 24, "ppt/media/image4.svg": 4} {
			asset := readZipFileString(t, path, part)
			if strings.Count(asset, `fill-opacity="0.12"`) != count || strings.Contains(asset, `fill-opacity="0.45"`) || strings.Contains(asset, `fill-opacity="0.5"`) {
				t.Errorf("quiet native artwork opacity drifted in %s", part)
			}
		}
	}
	check(templatePath)
	output := filepath.Join(t.TempDir(), "blue-body.pptx")
	_, _, err := generateSinglePass(context.Background(), GenerationRequest{
		TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentText, Value: "Required title"},
			{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Required parent wording", "\tRequired child wording", "Required final wording"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	check(output)
	slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
	for _, text := range []string{"Required title", "Required parent wording", "Required child wording", "Required final wording"} {
		if strings.Count(slide, "<a:t>"+text+"</a:t>") != 1 {
			t.Errorf("complete required text %q must remain exactly once", text)
		}
	}
}
