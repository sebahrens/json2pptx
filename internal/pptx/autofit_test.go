package pptx

import (
	"strings"
	"testing"
)

// go-slide-creator-wvr0. Every native diagram wrote <a:normAutofit/> with no
// fontScale. LibreOffice recomputes the shrink, so our own renders showed the
// text merely small; PowerPoint applies the STORED scale — 100% when absent —
// and only recomputes on edit, so the same box overflowed for the person who
// opened the deck. Every visual-QA loop rendering through soffice was
// inspecting pixels the client would never see.

// bulletBody builds a normAutofit body of n single-line bullets at 12pt with
// the 6pt space-after the diagram builders use.
func bulletBody(n int) *TextBody {
	tb := &TextBody{
		Wrap:    "square",
		Anchor:  "t",
		Insets:  [4]int64{91440, 91440, 91440, 91440},
		AutoFit: "normAutofit",
	}
	for i := 0; i < n; i++ {
		tb.Paragraphs = append(tb.Paragraphs, Paragraph{
			SpaceAfter: 600,
			Runs:       []Run{{Text: "Enterprise Platform Subscriptions", FontSize: 1200}},
		})
	}
	return tb
}

// swotQuadrant is the box the reported SWOT stress slide used.
var swotQuadrant = RectEmu{CX: 5221224, CY: 1754057}

func shapeXMLFor(t *testing.T, tb *TextBody, bounds RectEmu) string {
	t.Helper()
	b, err := GenerateShape(ShapeOptions{ID: 7, Name: "Body", Geometry: GeomRect, Bounds: bounds, Text: tb})
	if err != nil {
		t.Fatalf("GenerateShape: %v", err)
	}
	return string(b)
}

func TestNormAutofitCarriesTheComputedScale(t *testing.T) {
	xml := shapeXMLFor(t, bulletBody(12), swotQuadrant)
	if strings.Contains(xml, "<a:normAutofit/>") {
		t.Error("an overflowing body still wrote a bare normAutofit — PowerPoint will render it at 100% and overflow")
	}
	if !strings.Contains(xml, `<a:normAutofit fontScale="`) {
		t.Fatalf("no fontScale written:\n%s", xml)
	}
	if !strings.Contains(xml, `lnSpcReduction="`) {
		t.Error("a shrink without a line-spacing reduction leaves the block airier than the renderer's own fit")
	}
}

func TestNormAutofitLeftBareWhenTextFits(t *testing.T) {
	xml := shapeXMLFor(t, bulletBody(2), swotQuadrant)
	if !strings.Contains(xml, "<a:normAutofit/>") {
		t.Errorf("text that fits should keep the bare element (no shrink):\n%s", xml)
	}
}

// TestAutofitAccountsForParagraphSpacing is the correction that made the
// prediction usable: twelve bullets at 6pt space-after carry 72pt of spacing
// the glyph measurement never sees. Ignoring it predicted 66% for a block that
// needed 28%, and pinning the text at 66% still overflowed the box.
func TestAutofitAccountsForParagraphSpacing(t *testing.T) {
	spaced := bulletBody(12)
	tight := bulletBody(12)
	for i := range tight.Paragraphs {
		tight.Paragraphs[i].SpaceAfter = 0
	}
	applyAutofitScale(spaced, swotQuadrant)
	applyAutofitScale(tight, swotQuadrant)

	if spaced.AutoFitFontScale == 0 || tight.AutoFitFontScale == 0 {
		t.Fatalf("expected both to shrink; got %d and %d", spaced.AutoFitFontScale, tight.AutoFitFontScale)
	}
	if spaced.AutoFitFontScale >= tight.AutoFitFontScale {
		t.Errorf("paragraph spacing did not tighten the prediction: spaced %d >= tight %d",
			spaced.AutoFitFontScale, tight.AutoFitFontScale)
	}
}

func TestApplyAutofitScaleLeavesOtherModesAlone(t *testing.T) {
	for _, mode := range []string{"spAutoFit", "noAutofit", ""} {
		tb := bulletBody(12)
		tb.AutoFit = mode
		applyAutofitScale(tb, swotQuadrant)
		if tb.AutoFitFontScale != 0 {
			t.Errorf("autofit mode %q was given a scale", mode)
		}
	}
}

func TestApplyAutofitScaleRespectsAnExplicitScale(t *testing.T) {
	tb := bulletBody(12)
	tb.AutoFitFontScale = 90000
	applyAutofitScale(tb, swotQuadrant)
	if tb.AutoFitFontScale != 90000 {
		t.Errorf("an explicitly set scale was overwritten with %d", tb.AutoFitFontScale)
	}
}

func TestApplyAutofitScaleNoOpWithoutABox(t *testing.T) {
	tb := bulletBody(12)
	applyAutofitScale(tb, RectEmu{})
	if tb.AutoFitFontScale != 0 {
		t.Errorf("a shape with no bounds was given a scale: %d", tb.AutoFitFontScale)
	}
	applyAutofitScale(nil, swotQuadrant) // must not panic
}

func TestParagraphTextAndSizeUsesTheSmallestRun(t *testing.T) {
	p := Paragraph{Runs: []Run{
		{Text: "Big ", FontSize: 2400},
		{Text: "small", FontSize: 900},
		{Text: " none"},
	}}
	text, pt := paragraphTextAndSize(p)
	if text != "Big small none" {
		t.Errorf("text = %q", text)
	}
	if pt != 9 {
		t.Errorf("size = %v, want the smallest declared run (9pt)", pt)
	}
	if _, pt := paragraphTextAndSize(Paragraph{Runs: []Run{{Text: "x"}}}); pt != 0 {
		t.Errorf("a paragraph with no declared size should report 0, got %v", pt)
	}
}
