package generator

import (
	"strings"
	"testing"
)

// footerPositions builds a master-style footer position set: a left date box, a
// footer box beside it, and a narrow slide-number box against the right edge —
// the geometry modern-template ships and the bug reproduced on.
func footerPositions(sldNumWidth int64) map[string]*transformXML {
	const slideRight = 12192000
	box := func(x, cx int64) *transformXML {
		return &transformXML{
			Offset: offsetXML{X: x, Y: 6356350},
			Extent: extentXML{CX: cx, CY: 365125},
		}
	}
	return map[string]*transformXML{
		"type:dt":     box(838200, 2743200),
		"type:ftr":    box(4038600, 4114800),
		"type:sldNum": box(slideRight-838200-sldNumWidth, sldNumWidth),
	}
}

// TestPageNumberBoxFitsTheFormattedString is the go-slide-creator-pss1z
// acceptance test: a 0.5in box sized for three digits cannot hold "10 / 10", so
// the box grows leftward from its fixed right edge until the string fits.
func TestPageNumberBoxFitsTheFormattedString(t *testing.T) {
	positions := footerPositions(457200) // the old 3-digit minimum
	original := *positions["type:sldNum"]

	sizing := resolvePageNumberSizing(positions, "{current} / {total}", 10, "Arial")
	if sizing == nil {
		t.Fatal("no sizing returned for a slide-number position")
	}
	if sizing.box.Extent.CX <= original.Extent.CX {
		t.Errorf("box width %d did not grow from %d", sizing.box.Extent.CX, original.Extent.CX)
	}
	if got, want := sizing.box.Offset.X+sizing.box.Extent.CX, original.Offset.X+original.Extent.CX; got != want {
		t.Errorf("right edge moved: %d, want %d (the box grows leftward)", got, want)
	}
	if sizing.fontSize != footerFontSize {
		t.Errorf("font shrank to %d although there was room to grow", sizing.fontSize)
	}
	// The widest string must fit the usable width at the chosen size.
	widest := widestPageNumberText("{current} / {total}", 10)
	if w := footerTextWidthEMU(widest, sizing.fontSize, "Arial"); w > sizing.box.Extent.CX-2*91440 {
		t.Errorf("%q is %d EMU wide but the usable box is %d", widest, w, sizing.box.Extent.CX-2*91440)
	}
	// positions must not have been mutated: the caller lays the left footer out
	// against the returned copy.
	if *positions["type:sldNum"] != original {
		t.Errorf("resolvePageNumberSizing mutated the shared positions map")
	}
}

// TestWidestPageNumberText pins what is measured: the current number is a field
// PowerPoint fills in, so the widest case is both numbers at the deck's highest.
func TestWidestPageNumberText(t *testing.T) {
	cases := []struct {
		format string
		total  int
		want   string
	}{
		{"{current} / {total}", 10, "10 / 10"},
		{"{current} / {total}", 120, "120 / 120"},
		{"Slide {current} of {total}", 9, "Slide 9 of 9"},
		{"{current}", 250, "250"},
		{"", 42, "42"},
		{"{current} / {total}", 0, "1 / 1"},
	}
	for _, tc := range cases {
		if got := widestPageNumberText(tc.format, tc.total); got != tc.want {
			t.Errorf("widestPageNumberText(%q, %d) = %q, want %q", tc.format, tc.total, got, tc.want)
		}
	}
}

// TestPageNumberBoxKeepsTheLeftFooterUsable pins the growth limit: a very long
// page-number format shrinks its own type rather than eating the left footer.
func TestPageNumberBoxKeepsTheLeftFooterUsable(t *testing.T) {
	positions := footerPositions(457200)
	format := "Slide {current} of {total} — Project Atlas confidential draft, do not circulate"

	sizing := resolvePageNumberSizing(positions, format, 120, "Arial")
	if sizing == nil {
		t.Fatal("no sizing returned")
	}
	limit := pageNumberLeftLimit(positions)
	if sizing.box.Offset.X < limit {
		t.Errorf("box left edge %d crossed the left-footer limit %d", sizing.box.Offset.X, limit)
	}
	if sizing.box.Extent.CX > maxPageNumberWidth {
		t.Errorf("box grew to %d, past the %d cap — a page number is not a second footer line", sizing.box.Extent.CX, maxPageNumberWidth)
	}
	if sizing.fontSize >= footerFontSize {
		t.Errorf("font size %d, want a shrink when the box could not grow enough", sizing.fontSize)
	}
	if sizing.fontSize < footerMinFontSize {
		t.Errorf("font size %d fell below the floor %d", sizing.fontSize, footerMinFontSize)
	}
}

// TestPageNumberBoxLeavesRoomyTemplatesAlone pins that a template whose
// slide-number box is already wide enough is untouched — no geometry churn for
// decks that never had the bug.
func TestPageNumberBoxLeavesRoomyTemplatesAlone(t *testing.T) {
	positions := footerPositions(1828800) // 2in
	original := *positions["type:sldNum"]

	sizing := resolvePageNumberSizing(positions, "{current} / {total}", 10, "Arial")
	if sizing == nil {
		t.Fatal("no sizing returned")
	}
	if *sizing.box != original {
		t.Errorf("box changed to %+v from %+v", *sizing.box, original)
	}
	if sizing.fontSize != footerFontSize {
		t.Errorf("font size = %d, want the default %d", sizing.fontSize, footerFontSize)
	}
}

// TestFooterShapesDoNotOverlap pins the reason the box is sized before the left
// footer is laid out: the left box is clamped against the slide-number box, so
// widening one after the other would overlap them.
func TestFooterShapesDoNotOverlap(t *testing.T) {
	positions := footerPositions(457200)
	config := &FooterConfig{
		LeftText:         "Confidential — Project ATLAS | Northwind Corp",
		PageNumberFormat: "{current} / {total}",
		TotalSlides:      10,
	}
	xml := generateFooterShapes(positions, config, 900, "Arial", "", 1)
	if !strings.Contains(xml, "Footer Left") || !strings.Contains(xml, "Footer Right") {
		t.Fatalf("expected both footer shapes:\n%s", xml)
	}

	sizing := resolvePageNumberSizing(positions, config.PageNumberFormat, config.TotalSlides, "Arial")
	left := leftFooterBox(withSldNum(positions, sizing.box))
	if got, want := left.Offset.X+left.Extent.CX, sizing.box.Offset.X; got > want {
		t.Errorf("left footer ends at %d, past the page-number box at %d", got, want)
	}
}

// TestPageNumberShapesDoNotWrap pins the backstop: both slide-number shapes ask
// for wrap="none", so a miss overflows on one line instead of stacking.
func TestPageNumberShapesDoNotWrap(t *testing.T) {
	pos := footerPositions(457200)["type:sldNum"]
	formatted := generateFormattedSlideNumShape(992, "Footer Right", pos, "{current} / {total}", 30, footerFontSize, "")
	plain := generateSlideNumShape(993, "Footer Right", pos, footerFontSize, "")
	for name, xml := range map[string]string{"formatted": formatted, "plain": plain} {
		if !strings.Contains(xml, `wrap="none"`) {
			t.Errorf("%s slide-number shape does not set wrap=none:\n%s", name, xml)
		}
	}
}
