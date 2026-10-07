package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestGenerateTakeawayShape verifies the slide takeaway is the shared
// takeaway band (go-slide-creator-3a1rm, -fmmec): one shape the width of the
// body column, filled with the dark structural neutral, carrying 14pt bold
// text in the ink measured against it, 12pt in from each side and centred —
// not the 3pt accent bar beside unfilled dk1 text it was before.
func TestGenerateTakeawayShape(t *testing.T) {
	bounds := pptx.RectEmu{X: 838200, Y: 5700000, CX: 10515600, CY: 661797}
	xml := generateTakeawayShapesInBounds("Revenue doubled year over year.", 100, bounds, takeawayStyle{})
	if xml == "" {
		t.Fatal("generateTakeawayShapesInBounds returned empty string")
	}

	wants := []string{
		"Revenue doubled year over year.",
		`name="Takeaway"`,
		`sz="1400"`,                       // 14pt
		`b="1"`,                           // bold
		`<a:schemeClr val="dk2"`,          // the band: a theme neutral, not a hex
		`<a:schemeClr val="lt1"`,          // the ink that reads on it
		`<a:off x="838200" y="5700000"/>`, // flush at the band's left edge
		`<a:ext cx="10515600"`,            // the whole body column
		`lIns="152400"`, `rIns="152400"`,  // 12pt in from each side
		`tIns="101600"`, `bIns="101600"`, // 8pt above and below
		`anchor="ctr"`, // centred on the band's height
	}
	for _, want := range wants {
		if !strings.Contains(xml, want) {
			t.Errorf("generateTakeawayShapesInBounds() missing %q in:\n%s", want, xml)
		}
	}
	for _, reject := range []string{"Takeaway Bar", "accent1", "srgbClr", `<a:ln w="12700"`} {
		if strings.Contains(xml, reject) {
			t.Errorf("takeaway must carry no bar, accent, hex or rule; found %q in:\n%s", reject, xml)
		}
	}
	if n := strings.Count(xml, "<p:sp>"); n != 1 {
		t.Errorf("the band is one shape, got %d", n)
	}
	// One line of text: the band shrinks to the words and their padding
	// instead of filling the whole reserved rectangle.
	if h := takeawayBandHeight("Revenue doubled year over year.", bounds, takeawayStyle{}); h != 35*12700 {
		t.Errorf("one-line band height = %.1fpt, want 35pt", float64(h)/12700)
	}

	// Where dk2 is black the band is a charcoal off dk1, never a second black.
	black := takeawayStyle{ThemeColors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "dk2", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "lt2", RGB: "#EEEEEE"},
	}}
	charcoal := generateTakeawayShapesInBounds("x", 1, bounds, black)
	for _, want := range []string{`<a:schemeClr val="dk1"><a:lumMod val="85000"/><a:lumOff val="15000"/>`, `<a:schemeClr val="lt1"`} {
		if !strings.Contains(charcoal, want) {
			t.Errorf("black-dk2 band missing %q in:\n%s", want, charcoal)
		}
	}
}

// TestInsertTakeaway verifies that insertTakeaway places the shape inside
// the spTree (before its closing tag) without corrupting the surrounding XML.
func TestInsertTakeaway(t *testing.T) {
	const slide = `<?xml version="1.0"?><p:sld xmlns:p="x"><p:cSld><p:spTree><p:sp/></p:spTree></p:cSld></p:sld>`
	out, err := insertTakeaway([]byte(slide), "Hello world", pptx.RectEmu{X: 1, Y: 2, CX: 3, CY: 4})
	if err != nil {
		t.Fatalf("insertTakeaway error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "Hello world") {
		t.Errorf("inserted slide missing takeaway text:\n%s", got)
	}
	if !strings.Contains(got, "</p:spTree>") || !strings.Contains(got, "</p:sld>") {
		t.Errorf("inserted slide missing closing tags:\n%s", got)
	}
	// The takeaway shape must appear before the spTree closes.
	idxTakeaway := strings.Index(got, "Hello world")
	idxClose := strings.Index(got, "</p:spTree>")
	if idxTakeaway < 0 || idxClose < 0 || idxTakeaway >= idxClose {
		t.Errorf("takeaway shape not inside spTree: takeaway@%d, close@%d", idxTakeaway, idxClose)
	}
}
