package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDisplayTextPreservesTemplateAlignment(t *testing.T) {
	for _, tc := range []struct {
		name, phType, id, shapeName, listAlign, paragraphAlign, want string
		kind                                                         ContentType
		fontSize                                                     string
	}{
		{name: "title centred by list", phType: "title", id: "title", listAlign: "ctr"},
		{name: "centred title", phType: "ctrTitle", id: "title", listAlign: "ctr"},
		{name: "subtitle centred by list", phType: "subTitle", id: "subtitle", listAlign: "ctr"},
		{name: "subtitle type without canonical alias", phType: "subTitle", id: "body", listAlign: "ctr"},
		{name: "title explicit right", phType: "title", id: "title", listAlign: "ctr", paragraphAlign: "r", want: "r"},
		{name: "subtitle explicit centre", phType: "subTitle", id: "subtitle", paragraphAlign: "ctr", want: "ctr"},
		{name: "canonical title alias", id: "title_2", listAlign: "r"},
		{name: "canonical subtitle alias", id: "subtitle_2", listAlign: "ctr"},
		{name: "section title explicit right", phType: "title", id: "title", paragraphAlign: "r", want: "r", kind: ContentSectionTitle},
		{name: "section numeral", phType: "body", id: "Section Number", shapeName: "Section Number", listAlign: "r"},
		{name: "section number alias", phType: "body", id: "section_no", listAlign: "r"},
		{name: "section underscore alias", phType: "body", id: "section_number", listAlign: "r"},
		{name: "large number alias", phType: "body", id: " LARGE_NUMBER ", listAlign: "r"},
		{name: "number source name", phType: "body", id: "body", shapeName: "Section Number", paragraphAlign: "r", want: "r"},
		{name: "display body threshold", phType: "body", id: "body", listAlign: "r", fontSize: "9000"},
		{name: "body below threshold", phType: "body", id: "body", listAlign: "r", fontSize: "8999", want: "l"},
		{name: "large ordinary object", phType: "obj", id: "body", listAlign: "r", fontSize: "9000", want: "l"},
		{name: "ordinary subtitle-like word", phType: "body", id: "subtitlebody", listAlign: "r", want: "l"},
		{name: "ordinary body remains left", phType: "body", id: "body", listAlign: "ctr", want: "l"},
		{name: "ordinary body policy unchanged", phType: "body", id: "body", paragraphAlign: "ctr", want: "l"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind := tc.kind
			if kind == "" {
				kind = ContentText
			}
			var pprops *paragraphPropertiesXML
			fontSize := tc.fontSize
			if fontSize == "" {
				fontSize = "2400"
			}
			if tc.paragraphAlign != "" {
				pprops = &paragraphPropertiesXML{Algn: tc.paragraphAlign}
			}
			shape := &shapeXML{
				NonVisualProperties: nonVisualPropertiesXML{
					ConnectionNonVisual: connectionNonVisualXML{Name: tc.shapeName},
					NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: tc.phType}},
				},
				ShapeProperties: shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 6000000, CY: 2000000}}},
				TextBody: &textBodyXML{
					BodyProperties: &bodyPropertiesXML{},
					ListStyle:      &listStyleXML{Inner: `<a:lvl1pPr algn="` + tc.listAlign + `"><a:defRPr sz="` + fontSize + `"/></a:lvl1pPr>`},
					Paragraphs:     []paragraphXML{{Properties: pprops, Runs: []runXML{{Text: "Source prompt"}}}},
				},
			}
			if err := populateShapeText(shape, ContentItem{PlaceholderID: tc.id, Type: kind, Value: "Required first line\nRequired second line"}, -1, ""); err != nil {
				t.Fatal(err)
			}
			if len(shape.TextBody.Paragraphs) != 2 {
				t.Fatal("required paragraphs lost")
			}
			for _, para := range shape.TextBody.Paragraphs {
				if para.Properties == nil || para.Properties.Algn != tc.want {
					t.Fatalf("paragraph properties=%+v; want alignment %q (empty inherits native list/master)", para.Properties, tc.want)
				}
			}
			if pprops != nil && pprops.Algn != tc.paragraphAlign {
				t.Fatal("population mutated source paragraph properties")
			}
		})
	}
}

// Exercise the emitted PPTX, not just the population helper: source alignment
// must survive layout copying, normalization, serialization and packaging.
func TestNativeDisplayAlignmentInGeneratedPPTX(t *testing.T) {
	for _, tc := range []struct {
		name, layout, id, alignment string
	}{
		{"modern-yellow", "slideLayout8", "title", "ctr"},
		{"modern-yellow", "slideLayout8", "subtitle", "ctr"},
		{"forest-green", "slideLayout4", "Section Number", "r"},
		{"p-style", "slideLayout4", "Section Number", "r"},
	} {
		t.Run(tc.name+"/"+tc.id, func(t *testing.T) {
			tpl := filepath.Join("../../templates", tc.name+".pptx")
			if _, err := os.Stat(tpl); os.IsNotExist(err) && tc.name == "p-style" {
				t.Skip("ignored local p-style template is not present")
			} else if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "alignment.pptx")
			_, _, err := generateSinglePass(context.Background(), GenerationRequest{
				TemplatePath: tpl, OutputPath: out, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: tc.layout, Content: []ContentItem{{PlaceholderID: tc.id, Type: ContentText, Value: "01"}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			r, err := zip.OpenReader(out)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			// Use namespace-aware decoding structs, independent of the generator's
			// prefix-oriented serialization structs.
			var slide struct {
				Shapes []struct {
					TextBody *struct {
						ListStyle  *listStyleXML `xml:"lstStyle"`
						Paragraphs []struct {
							Properties *paragraphPropertiesXML `xml:"pPr"`
							Runs       []runXML                `xml:"r"`
						} `xml:"p"`
					} `xml:"txBody"`
				} `xml:"cSld>spTree>sp"`
			}
			found := false
			for _, f := range r.File {
				if f.Name != "ppt/slides/slide1.xml" {
					continue
				}
				rd, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(rd)
				_ = rd.Close()
				if err != nil {
					t.Fatal(err)
				}
				if err := xml.Unmarshal(data, &slide); err != nil {
					t.Fatal(err)
				}
				found = true
			}
			if !found {
				t.Fatal("generated slide is missing")
			}
			matched := 0
			for _, shape := range slide.Shapes {
				if shape.TextBody == nil {
					continue
				}
				for _, para := range shape.TextBody.Paragraphs {
					for _, run := range para.Runs {
						if run.Text != "01" {
							continue
						}
						matched++
						if para.Properties != nil && para.Properties.Algn != "" && para.Properties.Algn != tc.alignment {
							t.Fatalf("emitted inline alignment %q overrides native %q", para.Properties.Algn, tc.alignment)
						}
						if shape.TextBody.ListStyle == nil || !strings.Contains(shape.TextBody.ListStyle.Inner, `algn="`+tc.alignment+`"`) {
							t.Fatalf("native list alignment %q missing from emitted XML", tc.alignment)
						}
					}
				}
			}
			if matched != 1 {
				t.Fatalf("required source appears %d times, want exactly once", matched)
			}
		})
	}
}
