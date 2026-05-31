package generator

import (
	"strings"
	"testing"
)

// TestGenerateTakeawayShape verifies that the takeaway shape XML contains
// the expected text, bold formatting, font size, dark text color, and the
// distinct accent-tinted band fill plus accent rule (go-slide-creator-5ovr).
func TestGenerateTakeawayShape(t *testing.T) {
	xml := generateTakeawayShape("Revenue doubled year over year.", 100)
	if xml == "" {
		t.Fatal("generateTakeawayShape returned empty string")
	}

	wants := []string{
		"Revenue doubled year over year.",
		`name="Takeaway"`,
		`sz="1200"`,                  // 12pt
		`b="1"`,                      // bold
		"1F1F1F",                     // dark gray text color
		`<a:schemeClr val="accent1"`, // accent-driven band fill / rule
		`<a:lumMod val="20000"/>`,    // light wash: 20% luminance retained
		`<a:lumOff val="80000"/>`,    // light wash: +80% luminance
		`<a:ln w="12700"`,            // 1pt accent rule framing the band
	}
	for _, want := range wants {
		if !strings.Contains(xml, want) {
			t.Errorf("generateTakeawayShape() missing %q in:\n%s", want, xml)
		}
	}

	// The band must NOT render as a plain no-fill text box — the QA finding
	// that triggered the band treatment.
	if strings.Contains(xml, "<a:noFill/>") {
		t.Errorf("takeaway should have a band fill, not noFill:\n%s", xml)
	}
}

// TestInsertTakeaway verifies that insertTakeaway places the shape inside
// the spTree (before its closing tag) without corrupting the surrounding XML.
func TestInsertTakeaway(t *testing.T) {
	const slide = `<?xml version="1.0"?><p:sld xmlns:p="x"><p:cSld><p:spTree><p:sp/></p:spTree></p:cSld></p:sld>`
	out, err := insertTakeaway([]byte(slide), "Hello world")
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
