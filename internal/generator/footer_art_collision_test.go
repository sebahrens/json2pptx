package generator

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestParseFooterObstacles(t *testing.T) {
	const layout = `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
	<p:sp><p:nvSpPr><p:cNvPr name="Brand"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="7700000" y="6350000"/><a:ext cx="1000000" cy="300000"/></a:xfrm></p:spPr><p:txBody><a:p><a:r><a:t>Brand mark</a:t></a:r></a:p></p:txBody></p:sp>
	<p:sp><p:nvSpPr><p:cNvPr name="Placeholder"/><p:nvPr><p:ph type="ftr"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="6350000"/><a:ext cx="1000000" cy="300000"/></a:xfrm></p:spPr><p:txBody><a:p><a:r><a:t>Footer</a:t></a:r></a:p></p:txBody></p:sp>
	<p:sp><p:nvSpPr><p:cNvPr name="Band"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="6300000"/><a:ext cx="9144000" cy="500000"/></a:xfrm><a:solidFill><a:srgbClr val="333333"/></a:solidFill></p:spPr></p:sp>
	<p:sp><p:nvSpPr><p:cNvPr name="Badge"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="1000000" y="6350000"/><a:ext cx="300000" cy="300000"/></a:xfrm><a:solidFill><a:srgbClr val="333333"/></a:solidFill></p:spPr></p:sp>
	<p:pic><p:nvPicPr><p:cNvPr name="Logo"/><p:nvPr/></p:nvPicPr><p:spPr><a:xfrm><a:off x="200000" y="6350000"/><a:ext cx="500000" cy="300000"/></a:xfrm></p:spPr></p:pic>
	<p:pic><p:nvPicPr><p:cNvPr name="Wide art"/><p:nvPr/></p:nvPicPr><p:spPr><a:xfrm><a:off x="2000000" y="6350000"/><a:ext cx="5500000" cy="300000"/></a:xfrm></p:spPr></p:pic>
	<p:pic><p:nvPicPr><p:cNvPr name="Backdrop"/><p:nvPr/></p:nvPicPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="9144000" cy="6858000"/></a:xfrm></p:spPr></p:pic>
	<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr name="Footer table"/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="7500000" y="6350000"/><a:ext cx="1000000" cy="300000"/></p:xfrm></p:graphicFrame>
	</p:spTree></p:cSld></p:sldLayout>`
	got, err := parseFooterObstacles([]byte(layout), 9144000, 6858000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got[0].name != "Brand" || got[1].name != "Badge" || got[2].name != "Logo" || got[3].name != "Wide art" || got[4].name != "Footer table" {
		t.Fatalf("obstacles = %#v, want Brand, Badge, Logo, Wide art and Footer table only", got)
	}
	if got[0].box.Offset.X != 7700000 || got[2].box.Extent.CX != 500000 {
		t.Errorf("wrong obstacle geometry: %#v", got)
	}
	if _, err := parseFooterObstacles([]byte(`<p:sldLayout><p:cSld>`), 9144000, 6858000); err == nil {
		t.Error("malformed layout XML should fail")
	}
}

func TestParseFooterObstaclesMapsGroupedArtwork(t *testing.T) {
	const layout = `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
	<p:grpSp><p:grpSpPr><a:xfrm><a:off x="6000000" y="6000000"/><a:ext cx="2000000" cy="1000000"/><a:chOff x="0" y="0"/><a:chExt cx="1000000" cy="500000"/></a:xfrm></p:grpSpPr>
	<p:sp><p:nvSpPr><p:cNvPr name="Grouped brand"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="250000" y="100000"/><a:ext cx="250000" cy="100000"/></a:xfrm></p:spPr><p:txBody><a:p><a:r><a:t>Logo</a:t></a:r></a:p></p:txBody></p:sp>
	</p:grpSp></p:spTree></p:cSld></p:sldLayout>`
	got, err := parseFooterObstacles([]byte(layout), 12192000, 6858000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("grouped obstacles = %#v, want one", got)
	}
	box := got[0].box
	if box.Offset.X != 6500000 || box.Offset.Y != 6200000 || box.Extent.CX != 500000 || box.Extent.CY != 200000 {
		t.Errorf("grouped artwork box = %+v, want 500000x200000 at (6500000,6200000)", box)
	}
}

func TestFooterObstaclesRespectMasterVisibilityAndReportBadXML(t *testing.T) {
	const master = `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr name="Master brand"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="8000000" y="6300000"/><a:ext cx="800000" cy="300000"/></a:xfrm></p:spPr><p:txBody><a:p><a:r><a:t>Brand</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sldMaster>`
	const rels = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster99.xml"/></Relationships>`
	files := map[string][]byte{
		"ppt/slideLayouts/_rels/slideLayout99.xml.rels": []byte(rels),
		"ppt/slideMasters/slideMaster99.xml":            []byte(master),
	}
	newContext := func(showMaster string) *singlePassContext {
		copyFiles := make(map[string][]byte, len(files)+1)
		for name, data := range files {
			copyFiles[name] = data
		}
		copyFiles["ppt/slideLayouts/slideLayout99.xml"] = []byte(`<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" showMasterSp="` + showMaster + `"><p:cSld><p:spTree/></p:cSld></p:sldLayout>`)
		return &singlePassContext{OutputContext: OutputContext{syntheticFiles: copyFiles}, SlideContext: SlideContext{slideWidth: 12192000, slideHeight: 6858000}}
	}
	visible := newContext("1")
	got := visible.footerObstaclesForLayout("slideLayout99")
	if len(got) != 1 || got[0].name != "Master brand" {
		t.Fatalf("visible master obstacles = %#v", got)
	}
	if got := newContext("false").footerObstaclesForLayout("slideLayout99"); len(got) != 0 {
		t.Errorf("master art appeared despite showMasterSp=false: %#v", got)
	}
	broken := newContext("1")
	broken.syntheticFiles["ppt/slideMasters/slideMaster99.xml"] = []byte(`<p:sldMaster><p:cSld>`)
	if got := broken.footerObstaclesForLayout("slideLayout99"); len(got) != 0 || len(broken.warnings) == 0 {
		t.Errorf("malformed master should emit warning and no obstacles: got=%#v warnings=%#v", got, broken.warnings)
	}
}

func TestFooterPlacementAvoidsInheritedArtwork(t *testing.T) {
	positions := footerPositions(457200)
	config := &FooterConfig{Enabled: true, LeftText: "Confidential", PageNumberFormat: "{current} / {total}", TotalSlides: 10}
	page := resolvePageNumberSizing(positions, config.PageNumberFormat, config.TotalSlides, "Arial")
	brand := footerObstacle{name: "Brand", box: *page.box}
	brand.box.Offset.X -= 200000
	brand.box.Extent.CX += 200000
	adjusted := clearPageNumberBox(page.box, []footerObstacle{brand}, pageNumberLeftLimit(positions), 12192000)
	if adjusted == nil {
		t.Fatal("page number should find a clear position")
	}
	if footerBoxesOverlap(*adjusted, brand.box) || adjusted.Offset.X == page.box.Offset.X {
		t.Errorf("page number was not moved clear of the brand: %+v", adjusted)
	}
	if adjusted.Offset.X < pageNumberLeftLimit(positions) {
		t.Errorf("page number crossed the left footer limit: %+v", adjusted)
	}
	footerXML := generateFooterShapesAvoiding(positions, config, 100, "Arial", "", 0, []footerObstacle{brand}, 12192000)
	if !strings.Contains(footerXML, "Footer Left") || !strings.Contains(footerXML, "Footer Right") {
		t.Fatalf("expected both footers after shift: %s", footerXML)
	}
	var emitted struct {
		Shapes []shapeXML `xml:"sp"`
	}
	wrapper := `<p:spTree xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` + footerXML + `</p:spTree>`
	if err := xml.Unmarshal([]byte(wrapper), &emitted); err != nil {
		t.Fatalf("parse emitted footer: %v", err)
	}
	var emittedRight *transformXML
	for _, shape := range emitted.Shapes {
		if shape.NonVisualProperties.ConnectionNonVisual.Name == "Footer Right" {
			emittedRight = shape.ShapeProperties.Transform
		}
	}
	if emittedRight == nil || *emittedRight != *adjusted {
		t.Errorf("emitted page number box = %+v, want shifted %+v", emittedRight, adjusted)
	}

	blocked := footerObstacle{name: "Full footer zone", box: transformXML{
		Offset: offsetXML{X: pageNumberLeftLimit(positions), Y: page.box.Offset.Y},
		Extent: extentXML{CX: 12192000 - pageNumberLeftLimit(positions), CY: page.box.Extent.CY},
	}}
	if got := clearPageNumberBox(page.box, []footerObstacle{blocked}, pageNumberLeftLimit(positions), 12192000); got != nil {
		t.Errorf("blocked page number = %+v, want drop", got)
	}
	footerXML = generateFooterShapesAvoiding(positions, config, 100, "Arial", "", 0, []footerObstacle{blocked}, 12192000)
	if strings.Contains(footerXML, "Footer Right") {
		t.Errorf("blocked page number was still emitted: %s", footerXML)
	}
}

func TestLeftFooterUsesClearIntervalOrDrops(t *testing.T) {
	box := &transformXML{Offset: offsetXML{X: 500000, Y: 6300000}, Extent: extentXML{CX: 4000000, CY: 350000}}
	logo := footerObstacle{name: "Logo", box: transformXML{Offset: offsetXML{X: 500000, Y: 6300000}, Extent: extentXML{CX: 800000, CY: 350000}}}
	adjusted := clearLeftFooterBox(box, []footerObstacle{logo})
	if adjusted == nil || adjusted.Offset.X <= logo.box.Offset.X+logo.box.Extent.CX || footerBoxesOverlap(*adjusted, logo.box) {
		t.Errorf("left footer did not clear logo: %+v", adjusted)
	}
	if *box == *adjusted {
		t.Error("input box was mutated or not moved")
	}
	if got := clearLeftFooterBox(box, nil); got != box {
		t.Errorf("unobstructed footer changed: %+v", got)
	}
	short := &transformXML{Offset: box.Offset, Extent: extentXML{CX: 500000, CY: box.Extent.CY}}
	farAway := footerObstacle{name: "distant art", box: transformXML{Offset: offsetXML{X: 8000000, Y: box.Offset.Y}, Extent: extentXML{CX: 200000, CY: box.Extent.CY}}}
	if got := clearLeftFooterBox(short, []footerObstacle{farAway}); got != short {
		t.Errorf("unrelated artwork dropped a narrow footer: %+v", got)
	}
	wall := footerObstacle{name: "wall", box: *box}
	if got := clearLeftFooterBox(box, []footerObstacle{wall}); got != nil {
		t.Errorf("fully blocked left footer = %+v, want drop", got)
	}
}
