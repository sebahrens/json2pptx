package portabilitycheck

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestParseSpTreeTopLevelShapes(t *testing.T) {
	xmlData := `<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title 1"/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="10" y="20"/><a:ext cx="30" cy="40"/></a:xfrm></p:spPr></p:sp>` +
		`<p:grpSp><p:nvGrpSpPr><p:cNvPr id="3" name="Group"/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="4" name="Child"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="1" y="1"/><a:ext cx="1" cy="1"/></a:xfrm></p:spPr></p:sp>` +
		`</p:grpSp>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="5" name="Body"/><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>` +
		`</p:spTree></p:cSld></p:sld>`
	shapes, err := parseSpTree([]byte(xmlData))
	if err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 3 {
		t.Fatalf("got %d top-level shapes, want 3: %+v", len(shapes), shapes)
	}
	if shapes[0].PhType != "title" || shapes[0].Bounds != (Rect{10, 20, 30, 40}) {
		t.Errorf("title = %+v", shapes[0])
	}
	if shapes[1].Name != "Group" || shapes[1].Bounds != (Rect{100, 200, 300, 400}) {
		t.Errorf("group must take its own xfrm, not a child's: %+v", shapes[1])
	}
	if shapes[2].PhType != "body" || shapes[2].Bounds != (Rect{}) {
		t.Errorf("implicit body placeholder = %+v", shapes[2])
	}
}

func TestCheckSlideFlagsBandAndContentViolations(t *testing.T) {
	layout := types.LayoutMetadata{
		ID: "slideLayout2", MasterPath: "ppt/slideMasters/slideMaster1.xml",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 838200, Y: 365125, Width: 10515600, Height: 1325563}},
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}},
		},
		FooterRegions: []types.ChromeRegion{{Type: "ftr", X: 838200, Y: 6356350, Width: 4114800, Height: 365125}},
	}
	p := &template.TemplateProfile{SlideWidth: 12192000, SlideHeight: 6858000, Layouts: []types.LayoutMetadata{layout}}
	logo := Rect{11430000, 4572000, 640080, 1600200}
	logos := map[string][]Rect{layout.MasterPath: {logo}}

	// Legacy geometry: slide-percentage takeaway overlapping footers and logo,
	// body running under the band, a shape hanging off the canvas.
	bad := DeckSlide{Number: 1, LayoutID: "slideLayout2", Shapes: []DeckShape{
		{Name: "Takeaway", Bounds: Rect{457200, 6000000, 11277600, 360000}},
		{Name: "body", PhType: "body", Bounds: Rect{838200, 1825625, 10515600, 4351338}},
		{Name: "Card", Bounds: Rect{12000000, 100, 500000, 100}},
	}}
	got := strings.Join(checkSlide(bad, p, logos), "\n")
	for _, want := range []string{"want layout body column", "overlaps footer placeholder", "overlaps the master logo", "runs into the takeaway/source band", "outside the"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing violation %q in:\n%s", want, got)
		}
	}

	// Layout-derived geometry passes.
	frame := p.ChromeFrame("slideLayout2", true, false)
	good := DeckSlide{Number: 2, LayoutID: "slideLayout2", Shapes: []DeckShape{
		{Name: "Takeaway", Bounds: Rect{frame.Takeaway.X, frame.Takeaway.Y, frame.Takeaway.CX, frame.Takeaway.CY}},
		{Name: "body", PhType: "body", Bounds: Rect{838200, 1825625, 10515600, frame.Content.Bottom() - 1825625}},
	}}
	if v := checkSlide(good, p, logos); len(v) != 0 {
		t.Errorf("unexpected violations: %v", v)
	}
}
