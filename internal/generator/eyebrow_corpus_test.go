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
	NvSpPr struct {
		CNvPr struct {
			Name string `xml:"name,attr"`
		} `xml:"cNvPr"`
	} `xml:"nvSpPr"`
	Xfrm struct {
		Off struct {
			X int64 `xml:"x,attr"`
			Y int64 `xml:"y,attr"`
		} `xml:"off"`
		Ext struct {
			CX int64 `xml:"cx,attr"`
			CY int64 `xml:"cy,attr"`
		} `xml:"ext"`
	} `xml:"spPr>xfrm"`
	Paragraphs []eyebrowWireParagraph `xml:"txBody>p"`
}

type eyebrowWireParagraph struct {
	Properties *struct {
		Algn string `xml:"algn,attr"`
	} `xml:"pPr"`
	Runs []struct {
		Properties struct {
			FontSize int    `xml:"sz,attr"`
			Caps     string `xml:"cap,attr"`
			Fill     struct {
				Scheme struct {
					Value string `xml:"val,attr"`
				} `xml:"schemeClr"`
				RGB struct {
					Value string `xml:"val,attr"`
				} `xml:"srgbClr"`
			} `xml:"solidFill"`
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
			blankID := ""
			for _, layout := range layouts {
				if template.EffectiveCanonicalType(&layout) == types.CanonicalLayoutTitleSlide {
					layoutID = layout.ID
				}
				if template.EffectiveCanonicalType(&layout) == types.CanonicalLayoutBlank {
					blankID = layout.ID
				}
			}
			if layoutID == "" {
				t.Fatal("template has no title-slide layout")
			}
			output := filepath.Join(t.TempDir(), "eyebrow.pptx")
			slides := []SlideSpec{
				{LayoutID: layoutID, Eyebrow: "STRATEGY", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "A clear decision"},
					{PlaceholderID: "subtitle", Type: ContentTitleSlideTitle, Value: "Supporting context"},
				}},
				{LayoutID: layoutID, Eyebrow: "ONLY EYEBROW"},
			}
			if blankID != "" {
				slides = append(slides, SlideSpec{LayoutID: blankID, Eyebrow: "UNPLACED EYEBROW"})
			}
			result, err := Generate(context.Background(), GenerationRequest{
				TemplatePath: path, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: slides,
			})
			if err != nil {
				t.Fatal(err)
			}
			if blankID != "" {
				warned := false
				for _, warning := range result.Warnings {
					if strings.Contains(warning, "no title placeholder") {
						warned = true
					}
				}
				if !warned {
					t.Error("eyebrow on blank layout must report missing title placeholder")
				}
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			var slide eyebrowWireSlide
			located := false
			eyebrowOnlyFound := false
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
				if strings.Contains(string(data), "ONLY EYEBROW") {
					eyebrowOnlyFound = true
				}
				if !strings.Contains(string(data), "STRATEGY") {
					continue
				}
				if err := xml.Unmarshal(data, &slide); err != nil {
					t.Fatal(err)
				}
				seenIDs := map[string]bool{}
				for _, match := range shapeIDRegex.FindAllSubmatch(data, -1) {
					id := string(match[1])
					if seenIDs[id] {
						t.Errorf("eyebrow slide has duplicate shape id %s", id)
					}
					seenIDs[id] = true
				}
				located = true
			}
			if !located {
				t.Fatal("generated title slide not found in PPTX")
			}
			if !eyebrowOnlyFound {
				t.Fatal("eyebrow-only raw slide silently dropped its eyebrow")
			}
			var ebShape, titleShape *eyebrowWireShape
			for i := range slide.Shapes {
				shape := &slide.Shapes[i]
				for _, p := range shape.Paragraphs {
					for _, run := range p.Runs {
						switch run.Text {
						case "STRATEGY":
							ebShape = shape
						case "A clear decision":
							titleShape = shape
						}
					}
				}
			}
			if ebShape == nil || titleShape == nil || ebShape == titleShape {
				t.Fatal("eyebrow and title must occupy separate textboxes")
			}
			if ebShape.NvSpPr.CNvPr.Name != "Eyebrow" {
				t.Errorf("eyebrow shape name = %q", ebShape.NvSpPr.CNvPr.Name)
			}
			if ebShape.Xfrm.Off.Y+ebShape.Xfrm.Ext.CY > titleShape.Xfrm.Off.Y {
				t.Errorf("eyebrow bottom %d overlaps title top %d", ebShape.Xfrm.Off.Y+ebShape.Xfrm.Ext.CY, titleShape.Xfrm.Off.Y)
			}
			if ebShape.Xfrm.Off.X != titleShape.Xfrm.Off.X || ebShape.Xfrm.Ext.CX != titleShape.Xfrm.Ext.CX {
				t.Error("eyebrow and title must share their horizontal band")
			}
			eb := ebShape.Paragraphs[0]
			if got := eb.Runs[0].Properties.FontSize; got < 1200 || got > 1800 {
				t.Errorf("eyebrow size = %d, want 12–18pt", got)
			}
			if eb.Runs[0].Properties.Caps != "all" {
				t.Errorf("eyebrow cap style = %q, want all", eb.Runs[0].Properties.Caps)
			}
			if eb.Runs[0].Properties.Fill.Scheme.Value == "" && eb.Runs[0].Properties.Fill.RGB.Value == "" {
				t.Error("eyebrow has no independent text color")
			}
		})
	}
}
