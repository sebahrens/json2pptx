package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestAuthoredTextDoesNotInheritNativePromptLinks(t *testing.T) {
	for _, nativeEvent := range []string{"hlinkClick", "hlinkMouseOver"} {
		for _, paired := range []bool{false, true} {
			for _, authored := range []string{"none", "external", "slide"} {
				t.Run(fmt.Sprintf("%s/paired=%t/%s", nativeEvent, paired, authored), func(t *testing.T) {
					dir := t.TempDir()
					interaction := `<a:` + nativeEvent + ` r:id="rIdNativePrompt"/>`
					if paired {
						interaction = `<a:` + nativeEvent + ` r:id="rIdNativePrompt"></a:` + nativeEvent + `>`
					}
					fixture := `<p:sp><p:nvSpPr><p:cNvPr id="99" name="legal_disclosure"/><p:cNvSpPr/><p:nvPr><p:ph type="subTitle" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="500000" y="5600000"/><a:ext cx="11000000" cy="500000"/></a:xfrm></p:spPr><p:txBody><a:bodyPr anchor="b"/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="1100" i="1"><a:latin typeface="Georgia"/>` + interaction + `</a:rPr><a:t>Native linked prompt</a:t></a:r></a:p></p:txBody></p:sp>`
					templatePath := filepath.Join(dir, "template.pptx")
					writeDisclosureAlignmentTemplate(t, templatePath, fixture, `<Relationship Id="rIdNativePrompt" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.invalid/native-prompt" TargetMode="External"/>`)
					var link *LinkSpec
					if authored == "external" {
						link = &LinkSpec{URL: "https://example.com/authored"}
					} else if authored == "slide" {
						link = &LinkSpec{Slide: 1}
					}
					output := filepath.Join(dir, "generated.pptx")
					_, err := Generate(context.Background(), GenerationRequest{
						TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
						Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{
							{PlaceholderID: "title", Type: ContentText, Value: "Required title"},
							{PlaceholderID: "legal_disclosure", Type: ContentText, Value: "Required disclosure", Link: link},
						}}},
					})
					if err != nil {
						t.Fatal(err)
					}
					slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
					if strings.Contains(slide, "rIdNativePrompt") || strings.Contains(slide, "hlinkMouseOver") {
						t.Error("replacement text inherited the native prompt interaction")
					}
					for _, required := range []string{"Required title", "Required disclosure", `typeface="Georgia"`, `i="1"`} {
						if !strings.Contains(slide, required) {
							t.Errorf("lost native styling or required source: %s", required)
						}
					}
					var rels pptx.RelationshipsXML
					if err := xml.Unmarshal([]byte(readZipFileString(t, output, "ppt/slides/_rels/slide1.xml.rels")), &rels); err != nil {
						t.Fatal(err)
					}
					links := 0
					for _, rel := range rels.Relationships {
						if !strings.Contains(slide, `r:id="`+rel.ID+`"`) {
							continue
						}
						links++
						if authored == "external" && (rel.Target != link.URL || rel.Type != pptx.RelTypeHyperlink || rel.TargetMode != "External") {
							t.Errorf("wrong external authored relationship: %+v", rel)
						}
						if authored == "slide" && (rel.Target != "slide1.xml" || rel.Type != pptx.RelTypeSlide) {
							t.Errorf("wrong authored slide relationship: %+v", rel)
						}
					}
					want := 1
					if authored == "none" {
						want = 0
					}
					if links != want || strings.Count(slide, "<a:hlinkClick ") != want {
						t.Errorf("resolved authored links=%d, want %d", links, want)
					}
				})
			}
		}
	}
}
