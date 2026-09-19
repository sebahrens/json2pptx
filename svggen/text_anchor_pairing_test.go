package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// go-slide-creator-s27x (builder half). fixSVGTextAlignment pairs each emitted
// <text> element with the alignment DrawText recorded, BY INDEX. Its regex only
// matched the positioned form `<text x="…">`, so every ROTATED label —
// `<text transform="translate(…) rotate(-90)">`, which carries no x — was
// skipped while its recorded alignment stayed in the slice. From that point on
// every element took the PREVIOUS element's anchor and baseline.
//
// On a 2x2 matrix, which always draws a rotated y-axis title, that gave the
// first quadrant caption text-anchor="middle" and hung it half outside its own
// quadrant.

var anchorAttrRe = regexp.MustCompile(`text-anchor="(\w+)"`)
var baselineAttrRe = regexp.MustCompile(`dominant-baseline="([\w-]+)"`)

// textElementAttrs returns (anchor, baseline) for the <text> element carrying
// the given content; "-" when the attribute is absent.
func textElementAttrs(svg, content string) (anchor, baseline string, found bool) {
	re := regexp.MustCompile(`<text\b([^>]*)>(?:<tspan[^>]*>)?` + regexp.QuoteMeta(content) + `<`)
	m := re.FindStringSubmatch(svg)
	if m == nil {
		return "", "", false
	}
	anchor, baseline = "-", "-"
	if a := anchorAttrRe.FindStringSubmatch(m[1]); a != nil {
		anchor = a[1]
	}
	if b := baselineAttrRe.FindStringSubmatch(m[1]); b != nil {
		baseline = b[1]
	}
	return anchor, baseline, true
}

// TestTextAnchorSurvivesRotatedText draws a left-aligned label AFTER a rotated
// one and asserts it keeps its own (absent) anchor rather than inheriting the
// rotated label's.
func TestTextAnchorSurvivesRotatedText(t *testing.T) {
	b := NewSVGBuilder(400, 200)

	b.Push()
	b.SetFontSize(12)
	b.DrawText("centered-first", 200, 20, TextAlignCenter, TextBaselineTop)
	b.Pop()

	// A rotated label: emitted as <text transform="…"> with no x attribute.
	b.Push()
	b.SetFontSize(12)
	b.RotateAround(-90, 20, 100)
	b.DrawText("rotated-axis-title", 20, 100, TextAlignCenter, TextBaselineMiddle)
	b.Pop()

	b.Push()
	b.SetFontSize(12)
	b.DrawText("left-after-rotation", 40, 150, TextAlignLeft, TextBaselineMiddle)
	b.Pop()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	svg := string(doc.Content)

	anchor, baseline, found := textElementAttrs(svg, "left-after-rotation")
	if !found {
		t.Fatalf("label not emitted:\n%s", svg)
	}
	if anchor != "-" {
		t.Errorf("left-aligned label got text-anchor=%q — it inherited the rotated label's alignment", anchor)
	}
	if baseline != "central" {
		t.Errorf("baseline = %q, want central (its own TextBaselineMiddle)", baseline)
	}

	// And the element before the rotation must be untouched.
	anchor, baseline, found = textElementAttrs(svg, "centered-first")
	if !found {
		t.Fatal("first label not emitted")
	}
	if anchor != "middle" || baseline != "text-before-edge" {
		t.Errorf("first label = (%s, %s), want (middle, text-before-edge)", anchor, baseline)
	}
}

// TestMatrixQuadrantCaptionKeepsItsAnchor is the concrete symptom: the first
// quadrant caption comes after the rotated y-axis title.
func TestMatrixQuadrantCaptionKeepsItsAnchor(t *testing.T) {
	const w, h = 1104, 456
	b := NewSVGBuilder(w, h)
	cfg := DefaultMatrix2x2Config(w, h)
	cfg.QuadrantLabels = [4]string{"Major projects", "Quick wins", "Deprioritise", "Fill-ins"}
	data := Matrix2x2Data{QuadrantItems: [4][]string{{"ERP upgrade"}, {"Pricing reset"}, {"Office move"}, {"Partner portal"}}}
	if err := NewMatrix2x2Chart(b, cfg).Draw(data); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	svg := string(doc.Content)

	// Both left-hand quadrant headings are drawn left-aligned by the same code;
	// they must come out the same way.
	first, _, ok1 := textElementAttrs(svg, "Major projects")
	second, _, ok2 := textElementAttrs(svg, "Deprioritise")
	if !ok1 || !ok2 {
		t.Fatalf("quadrant captions missing:\n%s", strings.Split(svg, "><")[0])
	}
	if first != second {
		t.Errorf("first caption anchor %q differs from the second %q — same call site, so one of them inherited someone else's", first, second)
	}
	if first != "-" {
		t.Errorf("left-aligned caption got text-anchor=%q, which hangs it outside its quadrant", first)
	}
}
