package pptx

import "testing"

// go-slide-creator-vn35f: the autofit height measure wrapped every paragraph
// at the full text width. A bulleted paragraph's lines start at its left
// margin (the bullet hangs in it), so "Run the runtime" — 85pt at 12pt — was
// measured as one line in a text area 90pt wide whose bullet lines are 76pt
// wide, and every renderer set it on two.

func hangingBulletBody(text string, bullet bool) *TextBody {
	p := Paragraph{Align: "l", Runs: []Run{{Text: text, FontSize: 1200}}}
	if bullet {
		p.Bullet = &BulletDef{Char: DefaultBulletChar, Font: DefaultBulletFont}
		p.MarginL = BulletMarginLeft
		p.Indent = BulletIndent
	}
	return &TextBody{
		Wrap: "square", Anchor: "t", AutoFit: "normAutofit",
		Insets: [4]int64{180000, 45720, 180000, 180000}, Paragraphs: []Paragraph{p},
	}
}

func TestAutofitMeasureTakesTheHangingIndentOffABulletsLines(t *testing.T) {
	const text = "Run the runtime"
	// A 118.3pt-wide cell: 89.9pt of text area, 75.9pt of bullet line.
	width := int64(118.3 * 12700)
	oneLine := RectEmu{CX: width, CY: int64(34 * 12700)}  // insets + one 12pt line
	twoLines := RectEmu{CX: width, CY: int64(50 * 12700)} // insets + two

	plain := hangingBulletBody(text, false)
	if !AutofitFitsFor(plain, oneLine) {
		t.Fatal("an unbulleted 85pt line must fit a 90pt text area on one line: the fixture is wrong")
	}
	bullet := hangingBulletBody(text, true)
	if AutofitFitsFor(bullet, oneLine) {
		t.Error("a bulleted 85pt line is held to fit one line of a 76pt bullet column")
	}
	if !AutofitFitsFor(bullet, twoLines) {
		t.Error("the bulleted line does not fit the two lines it wraps onto")
	}
	if s := AutofitScaleFor(bullet, oneLine); s >= 1 {
		t.Errorf("AutofitScaleFor = %.2f for a bullet that needs a second line, want a shrink", s)
	}

	// A line balancer's margin on an unbulleted heading takes no height: it
	// is only set where it keeps the line count.
	balanced := hangingBulletBody(text, false)
	balanced.Paragraphs[0].MarginR = BulletMarginLeft
	if !AutofitFitsFor(balanced, oneLine) {
		t.Error("an unbulleted paragraph's balancing margin must not add a measured line")
	}
}
