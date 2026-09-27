package generator

import (
	"context"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeQNameBindingsInGeneratedPPTX(t *testing.T) {
	for _, scope := range []string{"run", "shape", "override"} {
		t.Run(scope, func(t *testing.T) {
			declaration := ` xmlns:q="urn:required-choice"`
			shapeDecl, runDecl := "", declaration
			if scope == "shape" {
				shapeDecl, runDecl = declaration, ""
			}
			if scope == "override" {
				shapeDecl = ` xmlns:q="urn:outer-choice"`
			}
			fixture := `<p:sp` + shapeDecl + `><p:nvSpPr><p:cNvPr id="99" name="legal_disclosure"/><p:cNvSpPr/><p:nvPr><p:ph type="subTitle" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="500000" y="5600000"/><a:ext cx="11000000" cy="500000"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="1100"` + runDecl + `><a:latin typeface="Georgia"/><a:extLst><a:ext uri="urn:test"><x:extra xmlns:x="urn:vendor-extension" x:mode="q:choice"/></a:ext></a:extLst></a:rPr><a:t>Native prompt</a:t></a:r></a:p></p:txBody></p:sp>`
			dir := t.TempDir()
			source, output := filepath.Join(dir, "source.pptx"), filepath.Join(dir, "generated.pptx")
			writeDisclosureAlignmentTemplate(t, source, fixture)
			_, err := Generate(context.Background(), GenerationRequest{TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}, {PlaceholderID: "legal_disclosure", Type: ContentText, Value: "Required disclosure"}}}}})
			if err != nil {
				t.Fatal(err)
			}
			slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
			if !strings.Contains(slide, "Required disclosure") {
				t.Fatal("source missing")
			}
			decoder := xml.NewDecoder(strings.NewReader(slide))
			scopes := []map[string]string{}
			found := false
			for {
				tok, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				switch element := tok.(type) {
				case xml.StartElement:
					bindings := map[string]string{}
					if len(scopes) > 0 {
						for prefix, uri := range scopes[len(scopes)-1] {
							bindings[prefix] = uri
						}
					}
					for _, attr := range element.Attr {
						if attr.Name.Space == "xmlns" {
							bindings[attr.Name.Local] = attr.Value
						}
					}
					scopes = append(scopes, bindings)
					if element.Name.Space == "urn:vendor-extension" && element.Name.Local == "extra" {
						for _, attr := range element.Attr {
							if attr.Name.Space == "urn:vendor-extension" && attr.Name.Local == "mode" && attr.Value == "q:choice" {
								found = true
								if bindings["q"] != "urn:required-choice" {
									t.Errorf("emitted QName value %q lost required binding: %v", attr.Value, bindings)
								}
							}
						}
					}
				case xml.EndElement:
					scopes = scopes[:len(scopes)-1]
				}
			}
			if !found {
				t.Fatal("extension or its QName-valued attribute lost")
			}
		})
	}
}
