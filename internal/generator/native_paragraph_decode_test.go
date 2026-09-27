package generator

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestNativeParagraphDecodesDrawingMLStyles(t *testing.T) {
	for _, prefix := range []string{"a", "drawing", ""} {
		t.Run(prefix, func(t *testing.T) {
			q := prefix
			namespace := `xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"`
			if prefix != "" {
				q += ":"
				namespace = `xmlns:` + prefix + `="http://schemas.openxmlformats.org/drawingml/2006/main"`
			}
			source := `<` + q + `p ` + namespace + `><` + q + `pPr algn="r" lvl="1" marL="123" indent="-45"><` + q + `buNone/></` + q + `pPr><` + q + `r><` + q + `rPr lang="de-DE" sz="1100" i="1"><` + q + `latin typeface="Georgia"/></` + q + `rPr><` + q + `t>Native source</` + q + `t></` + q + `r><` + q + `endParaRPr lang="de-DE"><` + q + `latin typeface="Georgia"/></` + q + `endParaRPr></` + q + `p>`
			var p paragraphXML
			if err := xml.Unmarshal([]byte(source), &p); err != nil {
				t.Fatal(err)
			}
			if p.Properties == nil || p.Properties.Algn != "r" || p.Properties.Level == nil || *p.Properties.Level != 1 || p.Properties.MarL == nil || *p.Properties.MarL != 123 || p.Properties.Indent == nil || *p.Properties.Indent != -45 || !strings.Contains(p.Properties.Inner, "buNone") {
				t.Fatalf("native paragraph styling lost: %+v", p.Properties)
			}
			if len(p.Runs) != 1 || p.Runs[0].Text != "Native source" || p.Runs[0].RunProperties == nil || p.Runs[0].RunProperties.Lang != "de-DE" || p.Runs[0].RunProperties.FontSize != "1100" || p.Runs[0].RunProperties.Italic != "1" || !strings.Contains(p.Runs[0].RunProperties.Inner, `typeface="Georgia"`) {
				t.Fatalf("native text/run styling lost: %+v", p.Runs)
			}
			if p.EndParaRPr == nil || p.EndParaRPr.Lang != "de-DE" || !strings.Contains(p.EndParaRPr.Inner, "Georgia") {
				t.Fatalf("native end-paragraph styling lost: %+v", p.EndParaRPr)
			}
		})
	}
}
