package generator

import (
	"context"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeStyleNamespacesInGeneratedPPTX(t *testing.T) {
	const drawingURI = "http://schemas.openxmlformats.org/drawingml/2006/main"
	for _, prefix := range []string{"a", "drawing", ""} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			q, declaration := prefix+":", `xmlns:`+prefix+`="`+drawingURI+`"`
			if prefix == "" {
				q, declaration = "", `xmlns="`+drawingURI+`"`
			}
			fixture := `<p:sp ` + declaration + `><p:nvSpPr><p:cNvPr id="99" name="legal_disclosure"/><p:cNvSpPr/><p:nvPr><p:ph type="subTitle" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><` + q + `xfrm><` + q + `off x="500000" y="5600000"/><` + q + `ext cx="11000000" cy="500000"/></` + q + `xfrm></p:spPr><p:txBody><` + q + `bodyPr anchor="b"><` + q + `noAutofit/></` + q + `bodyPr><` + q + `lstStyle><` + q + `lvl1pPr algn="r"><` + q + `defRPr sz="1100"><` + q + `latin typeface="Georgia"/></` + q + `defRPr></` + q + `lvl1pPr></` + q + `lstStyle><` + q + `p><` + q + `pPr algn="r"><` + q + `buNone/></` + q + `pPr><` + q + `r><` + q + `rPr sz="1100" i="1"><` + q + `latin typeface="Georgia"/></` + q + `rPr><` + q + `t>Native prompt</` + q + `t></` + q + `r><` + q + `endParaRPr lang="de-DE"><` + q + `latin typeface="Georgia"/></` + q + `endParaRPr></` + q + `p></p:txBody></p:sp>`
			dir := t.TempDir()
			templatePath, output := filepath.Join(dir, "template.pptx"), filepath.Join(dir, "generated.pptx")
			writeDisclosureAlignmentTemplate(t, templatePath, fixture)
			_, err := Generate(context.Background(), GenerationRequest{TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}, {PlaceholderID: "legal_disclosure", Type: ContentText, Value: "Required disclosure"}}}}})
			if err != nil {
				t.Fatal(err)
			}
			slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
			if !strings.Contains(slide, "Required disclosure") {
				t.Fatal("missing authored source")
			}
			decoder := xml.NewDecoder(strings.NewReader(slide))
			fonts := 0
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				start, ok := token.(xml.StartElement)
				if !ok {
					continue
				}
				for _, local := range []string{"latin", "buNone", "lvl1pPr", "defRPr", "noAutofit"} {
					if start.Name.Local == local && start.Name.Space != drawingURI {
						t.Errorf("%s has namespace %q, want DrawingML URI", local, start.Name.Space)
					}
				}
				if start.Name.Local == "latin" {
					for _, attr := range start.Attr {
						if attr.Name.Local == "typeface" && attr.Value == "Georgia" {
							fonts++
						}
					}
				}
			}
			if fonts == 0 {
				t.Fatal("native Georgia font lost")
			}
		})
	}
}
