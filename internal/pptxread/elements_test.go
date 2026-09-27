package pptxread

import (
	"encoding/xml"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-csclk.25: read dropped p:pic, p:cxnSp, a:fld text,
// hyperlink targets and mc:AlternateContent content.
func TestCollectShapeTree_PicturesFieldsLinksAltContent(t *testing.T) {
	const src = `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/>
  <p:txBody><a:p><a:r><a:rPr><a:hlinkClick r:id="rId3"/></a:rPr><a:t>Site</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Num"/><p:nvPr><p:ph type="sldNum"/></p:nvPr></p:nvSpPr><p:spPr/>
  <p:txBody><a:p><a:fld id="{X}" type="slidenum"><a:t>7</a:t></a:fld></a:p></p:txBody></p:sp>
<p:pic><p:nvPicPr><p:cNvPr id="4" name="Chart" descr="Bar chart"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
  <p:blipFill><a:blip r:embed="rId2"/></p:blipFill>
  <p:spPr><a:xfrm><a:off x="1" y="2"/><a:ext cx="3" cy="4"/></a:xfrm></p:spPr></p:pic>
<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="5" name="Line"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>
  <p:spPr><a:prstGeom prst="line"/></p:spPr></p:cxnSp>
<mc:AlternateContent><mc:Choice Requires="a14">
  <p:sp><p:nvSpPr><p:cNvPr id="6" name="Eq"/><p:nvPr/></p:nvSpPr><p:spPr/><p:txBody><a:p><a:r><a:t>E=mc2</a:t></a:r></a:p></p:txBody></p:sp>
</mc:Choice><mc:Fallback>
  <p:sp><p:nvSpPr><p:cNvPr id="6" name="Eq"/><p:nvPr/></p:nvSpPr><p:spPr/><p:txBody><a:p><a:r><a:t>E=mc2</a:t></a:r></a:p></p:txBody></p:sp>
</mc:Fallback></mc:AlternateContent>
</p:spTree></p:cSld></p:sld>`

	var sld slideDocument
	if err := xml.Unmarshal([]byte(src), &sld); err != nil {
		t.Fatal(err)
	}
	rels := pptx.NewRelationships()
	if err := rels.AddWithID("rId2", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image", "../media/image1.png"); err != nil {
		t.Fatal(err)
	}
	rels.AddExternal("http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink", "https://example.com")
	rc := &readContext{partDir: "ppt/slides", rels: rels}

	slide := &Slide{}
	collectShapeTreeCtx(slide, &sld.CSld.SpTree, identityTransform, rc)

	if len(slide.Placeholders) != 2 {
		t.Fatalf("placeholders = %+v", slide.Placeholders)
	}
	if got := slide.Placeholders[0].Hyperlinks; len(got) != 1 || got[0] != "https://example.com" {
		t.Errorf("title hyperlinks = %v, want [https://example.com]", got)
	}
	if got := slide.Placeholders[1].Text; got != "7" {
		t.Errorf("slide-number field text = %q, want 7", got)
	}
	if len(slide.Pictures) != 1 {
		t.Fatalf("pictures = %+v", slide.Pictures)
	}
	pic := slide.Pictures[0]
	if pic.Name != "Chart" || pic.AltText != "Bar chart" || pic.Media != "ppt/media/image1.png" || pic.Bounds == nil {
		t.Errorf("picture = %+v", pic)
	}
	var connectors, eq int
	for _, s := range slide.Shapes {
		if s.Connector {
			connectors++
		}
		if s.Text == "E=mc2" {
			eq++
		}
	}
	if connectors != 1 {
		t.Errorf("connectors = %d, want 1", connectors)
	}
	if eq != 1 {
		t.Errorf("AlternateContent text read %d times, want exactly 1", eq)
	}
}
