package pptxread

import (
	"encoding/xml"
	"testing"
)

// go-slide-creator-s1uvj.27: text inside p:grpSp (native diagrams) was lost.
// Nested groups must be read, with child bounds mapped through each group's
// chOff/chExt -> off/ext transform.
func TestCollectShapeTree_NestedGroups(t *testing.T) {
	const src = `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>
<p:grpSp><p:nvGrpSpPr><p:cNvPr id="10" name="outer"/></p:nvGrpSpPr>
  <p:grpSpPr><a:xfrm><a:off x="1000" y="2000"/><a:ext cx="2000" cy="2000"/><a:chOff x="0" y="0"/><a:chExt cx="1000" cy="1000"/></a:xfrm></p:grpSpPr>
  <p:sp><p:nvSpPr><p:cNvPr id="11" name="Strengths"/><p:nvPr/></p:nvSpPr>
    <p:spPr><a:xfrm><a:off x="100" y="100"/><a:ext cx="200" cy="300"/></a:xfrm><a:prstGeom prst="rect"/></p:spPr>
    <p:txBody><a:p><a:r><a:t>Strong brand</a:t></a:r></a:p></p:txBody></p:sp>
  <p:grpSp><p:nvGrpSpPr><p:cNvPr id="12" name="inner"/></p:nvGrpSpPr>
    <p:grpSpPr><a:xfrm><a:off x="500" y="500"/><a:ext cx="100" cy="100"/><a:chOff x="0" y="0"/><a:chExt cx="100" cy="100"/></a:xfrm></p:grpSpPr>
    <p:sp><p:nvSpPr><p:cNvPr id="13" name="Threats"/><p:nvPr/></p:nvSpPr>
      <p:spPr><a:xfrm><a:off x="10" y="20"/><a:ext cx="30" cy="40"/></a:xfrm></p:spPr>
      <p:txBody><a:p><a:r><a:t>New entrants</a:t></a:r></a:p></p:txBody></p:sp>
  </p:grpSp>
</p:grpSp>
</p:spTree></p:cSld></p:sld>`

	var sld slideDocument
	if err := xml.Unmarshal([]byte(src), &sld); err != nil {
		t.Fatal(err)
	}
	slide := &Slide{}
	collectShapeTree(slide, &sld.CSld.SpTree, identityTransform)

	want := map[string]Rect{
		"Strong brand": {X: 1200, Y: 2200, Width: 400, Height: 600},
		"New entrants": {X: 2020, Y: 3040, Width: 60, Height: 80},
	}
	if len(slide.Shapes) != len(want) {
		t.Fatalf("got %d shapes, want %d: %+v", len(slide.Shapes), len(want), slide.Shapes)
	}
	for _, s := range slide.Shapes {
		w, ok := want[s.Text]
		if !ok {
			t.Errorf("unexpected shape %+v", s)
			continue
		}
		if s.Bounds == nil || *s.Bounds != w {
			t.Errorf("%q bounds = %+v, want %+v", s.Text, s.Bounds, w)
		}
	}
}
