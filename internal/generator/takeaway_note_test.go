package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestGenerateTakeawayShape verifies the slide takeaway is the shared
// takeaway component (go-slide-creator-7b5o6): a flush 3pt accent1 bar and
// 14pt bold dk1 text inset 12pt from it, top-anchored, with no fill and no
// outline — not the old peach wash framed by a 1pt accent rule.
func TestGenerateTakeawayShape(t *testing.T) {
	bounds := pptx.RectEmu{X: 838200, Y: 5700000, CX: 10515600, CY: 514350}
	xml := generateTakeawayShapesInBounds("Revenue doubled year over year.", 100, bounds, takeawayStyle{})
	if xml == "" {
		t.Fatal("generateTakeawayShapesInBounds returned empty string")
	}

	wants := []string{
		"Revenue doubled year over year.",
		`name="Takeaway Bar"`,
		`name="Takeaway"`,
		`sz="1400"`,                       // 14pt
		`b="1"`,                           // bold
		`<a:schemeClr val="dk1"`,          // theme ink, not a hex
		`<a:schemeClr val="accent1"`,      // the bar
		`<a:ext cx="38100"`,               // 3pt bar
		`<a:off x="838200" y="5700000"/>`, // bar flush at the band's left edge
		`lIns="190500"`,                   // bar (3pt) + 12pt to the text
		`anchor="t"`,                      // top-anchored
	}
	for _, want := range wants {
		if !strings.Contains(xml, want) {
			t.Errorf("generateTakeawayShapesInBounds() missing %q in:\n%s", want, xml)
		}
	}
	for _, reject := range []string{"lumMod", "lumOff", "1F1F1F", `<a:ln w="12700"`} {
		if strings.Contains(xml, reject) {
			t.Errorf("takeaway must carry no tint, rule or hex ink; found %q in:\n%s", reject, xml)
		}
	}
	// One line of text: the band shrinks to the words instead of running the
	// bar the whole reserved rectangle.
	if h := takeawayBandHeight("Revenue doubled year over year.", bounds, takeawayStyle{}); h >= bounds.CY {
		t.Errorf("one-line band height = %d, want < reserved %d", h, bounds.CY)
	}
	if !strings.Contains(generateTakeawayShapesInBounds("x", 1, bounds, takeawayStyle{InkHex: "#F0F0F0"}), `srgbClr val="F0F0F0"`) {
		t.Error("a dark layout's measured ink must replace dk1")
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
