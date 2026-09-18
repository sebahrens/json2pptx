package generator

import (
	"image/color"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// footerLineWidthMM measures text on one line at sizeHPt (hundredths of a pt).
func footerLineWidthMM(t *testing.T, text, font string, sizeHPt int) float64 {
	t.Helper()
	ff, _, _ := fontcache.Resolve(font, "Arial")
	face := ff.Face(float64(sizeHPt)/100, color.Black, canvas.FontRegular, canvas.FontNormal)
	return canvas.NewTextLine(face, text, canvas.Left).Bounds().W()
}

func usableFooterMM(widthEMU int64) float64 { return float64(widthEMU-2*91440) / 36000 }

// masterFooterPositions mirrors the bundled mktemplate masters: dt (3in) at the
// left, ftr (4.5in) in the middle, sldNum (3in) at the right.
func masterFooterPositions() map[string]*transformXML {
	return map[string]*transformXML{
		"type:dt":     {Offset: offsetXML{X: 838200, Y: 6356350}, Extent: extentXML{CX: 2743200, CY: 365125}},
		"type:ftr":    {Offset: offsetXML{X: 4038600, Y: 6356350}, Extent: extentXML{CX: 4114800, CY: 365125}},
		"type:sldNum": {Offset: offsetXML{X: 8610600, Y: 6356350}, Extent: extentXML{CX: 2743200, CY: 365125}},
	}
}

// TestLeftFooterBoxSpansFooterPlaceholder verifies the left footer text box
// starts at the dt placeholder and extends across the ftr placeholder width
// (go-slide-creator-55i1) without reaching the slide-number box.
func TestLeftFooterBoxSpansFooterPlaceholder(t *testing.T) {
	pos := masterFooterPositions()
	box := leftFooterBox(pos)
	if box == nil {
		t.Fatal("expected a left footer box")
	}
	if box.Offset.X != 838200 {
		t.Errorf("box x = %d, want dt x 838200", box.Offset.X)
	}
	if got, want := box.Offset.X+box.Extent.CX, int64(4038600+4114800); got != want {
		t.Errorf("box right = %d, want ftr right %d", got, want)
	}
	if box.Offset.X+box.Extent.CX > pos["type:sldNum"].Offset.X {
		t.Error("left footer box overlaps the slide number")
	}
	if pos["type:dt"].Extent.CX != 2743200 {
		t.Error("leftFooterBox must not mutate the shared position map")
	}

	// An ftr running under the slide number is capped short of it.
	pos["type:ftr"].Extent.CX = 9000000
	if box := leftFooterBox(pos); box.Offset.X+box.Extent.CX != 8610600-footerBoxGap {
		t.Errorf("box right = %d, want capped at sldNum-gap %d", box.Offset.X+box.Extent.CX, 8610600-footerBoxGap)
	}

	// No dt position → no left footer.
	delete(pos, "type:dt")
	if leftFooterBox(pos) != nil {
		t.Error("expected nil box without a dt position")
	}
}

// TestFitFooterTextSingleLine verifies long footer text shrinks and then
// ellipsizes to stay on one line of the footer box.
func TestFitFooterTextSingleLine(t *testing.T) {
	const font = "Calibri"
	if ff, _, _ := fontcache.Resolve(font, "Arial"); ff == nil {
		t.Skip("no font metrics available on this host")
	}
	box := leftFooterBox(masterFooterPositions())

	short := "Acme Corp"
	if text, size := fitFooterText(short, box.Extent.CX, font); text != short || size != footerFontSize {
		t.Errorf("short text changed: %q @%d", text, size)
	}

	// The QA case: wraps to two lines in the old 3in dt box, fits on one
	// line of the widened dt+ftr box at the default size.
	long := "Acme Corporation | Confidential Strategy Review 2026"
	if w := footerLineWidthMM(t, long, font, footerFontSize); w <= usableFooterMM(2743200) {
		t.Logf("precondition: %q (%.1fmm) already fits the 3in box on this host", long, w)
	}
	text, size := fitFooterText(long, box.Extent.CX, font)
	if text != long || size != footerFontSize {
		t.Errorf("text that fits the widened box must be unchanged: %q @%d", text, size)
	}

	// Slightly too long: shrinks below the default size but stays whole.
	medium := "Acme Corporation | Confidential Strategy Review 2026 | Board of Directors pre-read"
	text, size = fitFooterText(medium, box.Extent.CX, font)
	if w := footerLineWidthMM(t, text, font, size); w > usableFooterMM(box.Extent.CX) {
		t.Errorf("fitted footer %q @%d is %.1fmm, exceeds one line (%.1fmm)", text, size, w, usableFooterMM(box.Extent.CX))
	}

	// Far too long for any size: ellipsized at the minimum size.
	huge := strings.Repeat("Confidential and proprietary information ", 8)
	text, size = fitFooterText(huge, box.Extent.CX, font)
	if size != footerMinFontSize || !strings.HasSuffix(text, "\u2026") {
		t.Errorf("expected ellipsized text at min size, got %q @%d", text, size)
	}
	if w := footerLineWidthMM(t, text, font, size); w > usableFooterMM(box.Extent.CX) {
		t.Errorf("ellipsized footer is %.1fmm, exceeds one line", w)
	}
}
