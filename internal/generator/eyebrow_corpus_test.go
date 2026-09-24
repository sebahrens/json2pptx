package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// These wire types use local XML names because the generated PPTX uses real
// a:/p: namespaces, while the generator's mutable model is decoded before its
// output namespace fixup.
type eyebrowWireSlide struct {
	Shapes []eyebrowWireShape `xml:"cSld>spTree>sp"`
}

type eyebrowWireShape struct {
	Paragraphs []eyebrowWireParagraph `xml:"txBody>p"`
}

type eyebrowWireParagraph struct {
	Properties *struct {
		Algn string `xml:"algn,attr"`
	} `xml:"pPr"`
	Runs []struct {
		Properties struct {
			FontSize int `xml:"sz,attr"`
		} `xml:"rPr"`
		Text string `xml:"t"`
	} `xml:"r"`
}

func TestTitleEyebrowAcrossTemplateCorpus(t *testing.T) {
	for _, path := range testutil.TestTemplatePaths() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			reader, err := template.OpenTemplate(path)
			if err != nil {
				t.Fatal(err)
			}
			layouts, err := template.ParseLayouts(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			layoutID := ""
			for _, layout := range layouts {
				if template.EffectiveCanonicalType(&layout) == types.CanonicalLayoutTitleSlide {
					layoutID = layout.ID
					break
				}
			}
			if layoutID == "" {
				t.Fatal("template has no title-slide layout")
			}
			output := filepath.Join(t.TempDir(), "eyebrow.pptx")
			_, err = Generate(context.Background(), GenerationRequest{
				TemplatePath: path, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: layoutID, Eyebrow: "STRATEGY", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "A clear decision"},
					{PlaceholderID: "subtitle", Type: ContentTitleSlideTitle, Value: "Supporting context"},
				}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			var slide eyebrowWireSlide
			located := false
			for _, entry := range z.File {
				if !strings.HasPrefix(entry.Name, "ppt/slides/slide") || !strings.HasSuffix(entry.Name, ".xml") {
					continue
				}
				r, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, readErr := io.ReadAll(r)
				_ = r.Close()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !strings.Contains(string(data), "STRATEGY") {
					continue
				}
				if err := xml.Unmarshal(data, &slide); err != nil {
					t.Fatal(err)
				}
				located = true
				break
			}
			if !located {
				t.Fatal("generated title slide not found in PPTX")
			}
			found := false
			for _, shape := range slide.Shapes {
				if len(shape.Paragraphs) < 2 {
					continue
				}
				eb, title := shape.Paragraphs[0], shape.Paragraphs[1]
				if len(eb.Runs) == 0 || eb.Runs[0].Text != "STRATEGY" {
					continue
				}
				found = true
				if eb.Properties == nil || (title.Properties != nil && eb.Properties.Algn != title.Properties.Algn) || (title.Properties == nil && eb.Properties.Algn != "") {
					t.Errorf("eyebrow alignment differs from title: eyebrow=%+v title=%+v", eb.Properties, title.Properties)
				}
				if got := eb.Runs[0].Properties.FontSize; got < 1200 {
					t.Errorf("eyebrow size = %d, want at least 1200", got)
				}
				break
			}
			if !found {
				t.Fatal("generated title slide has no eyebrow before its title")
			}
		})
	}
}
