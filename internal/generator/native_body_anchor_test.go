package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Verify native anchors through actual layout copying, population and packaging,
// including a few long paragraphs that must not be mistaken for sparse ink.
func TestNativeBodyAnchorPreservedInGeneratedPackage(t *testing.T) {
	for _, anchor := range []string{"", "t", "ctr", "b"} {
		for _, kind := range []ContentType{ContentBullets, ContentBodyAndBullets} {
			t.Run(fmt.Sprintf("%s/%s", anchor, kind), func(t *testing.T) {
				dir := t.TempDir()
				anchorAttr := ""
				if anchor != "" {
					anchorAttr = ` anchor="` + anchor + `"`
				}
				fixture := `<p:sp><p:nvSpPr><p:cNvPr id="99" name="case_body"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="500000" y="1800000"/><a:ext cx="11000000" cy="4000000"/></a:xfrm></p:spPr><p:txBody><a:bodyPr` + anchorAttr + `/><a:lstStyle><a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:t>Original body prompt</a:t></a:r></a:p></p:txBody></p:sp>`
				templatePath, output := filepath.Join(dir, "native.pptx"), filepath.Join(dir, "output.pptx")
				writeDisclosureAlignmentTemplate(t, templatePath, fixture)
				cases := []string{"Required first case retains its complete explanation across multiple rendered lines instead of being classified as visually sparse solely because it is one paragraph. " + strings.Repeat("Complete supporting evidence. ", 6), "Required second complete case."}
				var value any = cases
				wantText := append([]string(nil), cases...)
				if kind == ContentBodyAndBullets {
					value = BodyAndBulletsContent{Body: "Required group heading", Bullets: cases}
					wantText = append([]string{"Required group heading"}, cases...)
				}
				_, _, err := generateSinglePass(context.Background(), GenerationRequest{TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{{PlaceholderID: "idx:7", Type: kind, Value: value}}}}})
				if err != nil {
					t.Fatal(err)
				}
				var slide struct {
					Shapes []struct {
						Placeholder struct {
							Index string `xml:"idx,attr"`
							Kind  string `xml:"type,attr"`
						} `xml:"nvSpPr>nvPr>ph"`
						Text struct {
							Properties struct {
								Anchor string `xml:"anchor,attr"`
							} `xml:"bodyPr"`
							Paragraphs []struct {
								Runs []struct {
									Text string `xml:"t"`
								} `xml:"r"`
							} `xml:"p"`
						} `xml:"txBody"`
					} `xml:"cSld>spTree>sp"`
				}
				if err := xml.Unmarshal([]byte(readZipFileString(t, output, "ppt/slides/slide1.xml")), &slide); err != nil {
					t.Fatal(err)
				}
				matched := 0
				for _, shape := range slide.Shapes {
					if shape.Placeholder.Index != "7" || shape.Placeholder.Kind != "body" {
						continue
					}
					matched++
					gotAnchor, wantAnchor := shape.Text.Properties.Anchor, anchor
					if gotAnchor == "" {
						gotAnchor = "t"
					}
					if wantAnchor == "" {
						wantAnchor = "t"
					}
					if gotAnchor != wantAnchor {
						t.Fatalf("native anchor %q replaced with %q", wantAnchor, gotAnchor)
					}
					var gotText []string
					for _, p := range shape.Text.Paragraphs {
						var text string
						for _, run := range p.Runs {
							text += run.Text
						}
						gotText = append(gotText, text)
					}
					if !reflect.DeepEqual(gotText, wantText) {
						t.Fatalf("source content changed: %+v", gotText)
					}
				}
				if matched != 1 {
					t.Fatalf("native body emitted %d times", matched)
				}
			})
		}
	}
}
