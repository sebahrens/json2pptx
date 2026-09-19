package generator

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/tdewolff/canvas"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// Page-number box sizing (go-slide-creator-pss1z).
//
// The slide-number placeholder was widened to a flat 0.5in — "enough for three
// digits" — and the page-number TEXT never went near the fitter that sizes the
// left footer. With chrome.page_numbers.format "{current} / {total}" on
// modern-template every page number rendered as "2 /" over "10": the string is
// six characters, the box was sized for three.
//
// The box is now measured from the widest string the field can display (the
// format with current AND total at their largest) using the same metrics the
// left footer is fitted with, and grows leftward from its fixed right edge. When
// the growth would eat the left footer, the number's font shrinks instead, on
// the same ladder — and wrap="none" is set as a backstop so a miss overflows
// rather than stacking two lines in the corner.

// minLeftFooterWidth is the width the left footer keeps when the page-number box
// grows leftward. Below this the left text has no room to say anything, so the
// page number shrinks its font instead of taking more space.
const minLeftFooterWidth int64 = 914400 // 1in

// maxPageNumberWidth caps how wide the page-number box may grow. A page number
// is a corner ornament, not a second footer line: "Slide 120 of 120" needs about
// 1.4in at 10.5pt, so 2in covers every sane format. A format longer than that is
// a sentence in the wrong field, and shrinking the type is the better answer
// than handing it half the footer.
const maxPageNumberWidth int64 = 1828800 // 2in

// pageNumberSizing is the resolved geometry and type size for one slide's page
// number.
type pageNumberSizing struct {
	// box is the sldNum placeholder, widened leftward when the number needs it.
	box *transformXML
	// fontSize is the size (hundredths of a point) the number is drawn at.
	fontSize int
}

// resolvePageNumberSizing measures the widest page number a deck can show and
// returns the box and font size to draw it with. positions is not mutated: the
// returned box is a copy, so the caller can lay the left footer out against it.
//
// A nil return means there is no slide-number position to size.
func resolvePageNumberSizing(positions map[string]*transformXML, format string, totalSlides int, fontName string) *pageNumberSizing {
	pos, ok := positions["type:sldNum"]
	if !ok || pos == nil {
		return nil
	}
	box := *pos
	sizing := &pageNumberSizing{box: &box, fontSize: footerFontSize}

	text := widestPageNumberText(format, totalSlides)
	if text == "" {
		return sizing
	}
	needed := footerTextWidthEMU(text, footerFontSize, fontName)
	if needed <= 0 {
		return sizing // no font metrics: leave the template's geometry alone
	}
	needed += 2 * 91440 // the 0.1in insets the shape carries

	if box.Extent.CX >= needed {
		return sizing
	}

	// Grow leftward from the fixed right edge, within the ornament's own cap and
	// never into the left footer's last inch.
	if needed > maxPageNumberWidth {
		needed = maxPageNumberWidth
	}
	rightEdge := box.Offset.X + box.Extent.CX
	leftLimit := pageNumberLeftLimit(positions)
	target := rightEdge - needed
	if leftLimit > 0 && target < leftLimit {
		target = leftLimit
	}
	if target < 0 {
		target = 0
	}
	if width := rightEdge - target; width > box.Extent.CX {
		box.Offset.X = target
		box.Extent.CX = width
	}

	// Still short? Shrink the type rather than wrap, on the left footer's ladder.
	sizing.fontSize = fitPageNumberFontSize(text, box.Extent.CX, fontName)
	return sizing
}

// pageNumberLeftLimit is the leftmost X the page-number box may take, so the
// left footer keeps a usable width. Zero means unconstrained (no left footer).
func pageNumberLeftLimit(positions map[string]*transformXML) int64 {
	dt, ok := positions["type:dt"]
	if !ok || dt == nil {
		return 0
	}
	return dt.Offset.X + minLeftFooterWidth + footerBoxGap
}

// widestPageNumberText renders the page-number format at its widest: both the
// current and the total at the deck's highest slide number, since the current
// number is a field PowerPoint fills in at display time.
func widestPageNumberText(format string, totalSlides int) string {
	widest := strconv.Itoa(maxInt(totalSlides, 1))
	if format == "" {
		return widest
	}
	out := strings.ReplaceAll(format, "{current}", widest)
	return strings.ReplaceAll(out, "{total}", widest)
}

// footerTextWidthEMU measures a single line of footer text, in EMU. It returns
// 0 when no font can be resolved, which callers read as "do not resize".
func footerTextWidthEMU(text string, fontSize int, fontName string) int64 {
	ff, _, _ := fontcache.Resolve(fontName, "Arial")
	if ff == nil {
		return 0
	}
	face := ff.Face(float64(fontSize)/100, color.Black, canvas.FontRegular, canvas.FontNormal)
	mm := canvas.NewTextLine(face, text, canvas.Left).Bounds().W()
	return int64(mm * float64(emuPerMM))
}

// fitPageNumberFontSize returns the largest footer size at which text fits
// widthEMU on one line, floored at footerMinFontSize. A page number is never
// ellipsized: "2 …" says nothing, so the floor wins and wrap="none" keeps it on
// one line.
func fitPageNumberFontSize(text string, widthEMU int64, fontName string) int {
	usable := widthEMU - 2*91440
	for size := footerFontSize; size > footerMinFontSize; size -= footerFontStep {
		if w := footerTextWidthEMU(text, size, fontName); w == 0 || w <= usable {
			return size
		}
	}
	return footerMinFontSize
}

// withSldNum returns a shallow copy of positions with the slide-number box
// replaced, so the left footer is laid out against the widened box rather than
// the template's original.
func withSldNum(positions map[string]*transformXML, box *transformXML) map[string]*transformXML {
	out := make(map[string]*transformXML, len(positions))
	for k, v := range positions {
		out[k] = v
	}
	out["type:sldNum"] = box
	return out
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
