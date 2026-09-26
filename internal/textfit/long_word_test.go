package textfit

import (
	"strings"
	"testing"
)

// A word wider than the box wraps by character wherever it sits in the
// paragraph: prefixing a short word can never lower the line count, and the
// overflow estimate must see the wrapped word too (go-slide-creator-s1uvj.12).
func TestMeasureRun_LongWordAfterFirstWordWrapsByCharacter(t *testing.T) {
	width := int64(3 * 914400)
	url := "https://example.com/" + strings.Repeat("abcdefghij", 18)

	bare := mustMeasure(t, url, "Liberation Sans", 14, width, 0)
	prefixed := mustMeasure(t, "See "+url, "Liberation Sans", 14, width, 0)
	if bare.Lines < 3 {
		t.Fatalf("bare url measured %d lines; test needs a word several lines wide", bare.Lines)
	}
	if prefixed.Lines < bare.Lines {
		t.Errorf("'See '+url = %d lines, bare url = %d; a leading word cannot lower the line count", prefixed.Lines, bare.Lines)
	}

	capped := mustMeasure(t, "See "+url, "Liberation Sans", 14, width, 2)
	if capped.Fits || capped.OverflowChars == 0 {
		t.Errorf("'See '+url capped at 2 lines: fits=%v overflow=%d, want overflow", capped.Fits, capped.OverflowChars)
	}
	if capped.OverflowChars >= len([]rune(url)) {
		t.Errorf("overflow %d should exclude the part of the url that fits in 2 lines (url is %d runes)", capped.OverflowChars, len([]rune(url)))
	}
}

func TestCalculate_LongWordAfterFirstWordNeedsShrink(t *testing.T) {
	url := "https://example.com/" + strings.Repeat("abcdefghij", 18)
	p := Params{WidthEMU: 3 * 914400, HeightEMU: 914400, FontSizeHPt: 1400, FontName: "Liberation Sans"}

	p.Paragraphs = []string{url}
	bare, err := Calculate(p)
	if err != nil {
		t.Fatal(err)
	}
	p.Paragraphs = []string{"See " + url}
	prefixed, err := Calculate(p)
	if err != nil {
		t.Fatal(err)
	}
	if bare.FontScale == 0 {
		t.Fatalf("bare url should need shrink in a 1in-high box, got scale 0")
	}
	if prefixed.FontScale == 0 || prefixed.FontScale > bare.FontScale {
		t.Errorf("'See '+url scale = %d, bare = %d; the prefixed text needs at least as much shrink", prefixed.FontScale, bare.FontScale)
	}
}
