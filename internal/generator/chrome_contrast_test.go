package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-hln7. modern-template's section divider is the shape that
// broke chrome: it fills its background with schemeClr tx1 under a color map
// override that renames tx1 to lt1, so the background renders white -- and the
// footer, emitted as literal schemeClr tx1, rendered white on it.
const invertedLayoutXML = `<?xml version="1.0"?><p:sldLayout xmlns:p="p" xmlns:a="a">` +
	`<p:cSld name="Section Divider"><p:bg><p:bgPr><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>` +
	`<p:spTree/></p:cSld>` +
	`<p:clrMapOvr><a:overrideClrMapping bg1="dk1" tx1="lt1" bg2="dk2" tx2="lt2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr>` +
	`</p:sldLayout>`

// plainLayoutXML is the ordinary case: a light background, no override. Chrome
// must be left alone here.
const plainLayoutXML = `<?xml version="1.0"?><p:sldLayout xmlns:p="p" xmlns:a="a">` +
	`<p:cSld name="One Content"><p:bg><p:bgPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></p:bgPr></p:bg><p:spTree/></p:cSld>` +
	`</p:sldLayout>`

// darkLayoutXML is a genuinely dark background with no override: tx1 resolves
// to near-black, which is unreadable, so chrome must be lightened.
const darkLayoutXML = `<?xml version="1.0"?><p:sldLayout xmlns:p="p" xmlns:a="a">` +
	`<p:cSld name="Dark Divider"><p:bg><p:bgPr><a:solidFill><a:srgbClr val="14213D"/></a:solidFill></p:bgPr></p:bg><p:spTree/></p:cSld>` +
	`</p:sldLayout>`

func modernThemeColors() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#2C3932"},
		{Name: "lt2", RGB: "#F2F2F2"},
		{Name: "accent1", RGB: "#6C5CE7"},
	}
}

func TestParseLayoutColorMapOverride(t *testing.T) {
	got := parseLayoutColorMapOverride([]byte(invertedLayoutXML))
	if got["tx1"] != "lt1" {
		t.Errorf("tx1 maps to %q, want lt1", got["tx1"])
	}
	if got["bg1"] != "dk1" {
		t.Errorf("bg1 maps to %q, want dk1", got["bg1"])
	}
	if o := parseLayoutColorMapOverride([]byte(plainLayoutXML)); o != nil {
		t.Errorf("a layout with no override should report none, got %v", o)
	}
}

// TestExtractLayoutBackgroundHonoursColorMapOverride is the first half of the
// bug: read literally, this layout's background is dk1 (near-black); the slide
// actually renders white. Everything downstream that picks a text color against
// the background was working from the wrong color.
func TestExtractLayoutBackgroundHonoursColorMapOverride(t *testing.T) {
	got := extractLayoutBackgroundColor([]byte(invertedLayoutXML), modernThemeColors())
	if got != "#FFFFFF" {
		t.Errorf("background = %q, want #FFFFFF (tx1 renamed to lt1 by the layout's color map)", got)
	}
}

// TestChromeTextColorOnInvertedLayout is the bug itself: chrome must not be
// drawn in the background's own color.
func TestChromeTextColorOnInvertedLayout(t *testing.T) {
	v := chromeTextColor([]byte(invertedLayoutXML), modernThemeColors(), 2)
	if v.hex == "" {
		t.Fatal("chrome left at schemeClr tx1, which renders white on this white layout — the footer is invisible")
	}
	if strings.EqualFold(v.hex, "#FFFFFF") {
		t.Errorf("chrome color = %s, which is the background color", v.hex)
	}
	if v.swap == nil {
		t.Fatal("no contrast swap recorded, so no contrast_autofixed finding reaches the agent")
	}
	if v.swap.RatioBefore >= 4.5 {
		t.Errorf("RatioBefore = %.2f; the whole point is that it was below AA", v.swap.RatioBefore)
	}
	if v.swap.RatioAfter < 4.5 {
		t.Errorf("RatioAfter = %.2f, still below WCAG AA", v.swap.RatioAfter)
	}
	if v.swap.Source != chromeSwapSource {
		t.Errorf("swap source = %q, want %q so the finding names the surface", v.swap.Source, chromeSwapSource)
	}
	if v.swap.Path != "/slides/2/chrome" {
		t.Errorf("swap path = %q, want /slides/2/chrome", v.swap.Path)
	}
}

// TestChromeTextColorLeavesOrdinaryLayoutsAlone pins the blast radius: on a
// normal light layout chrome keeps inheriting the template's scheme color, so
// existing decks render byte-identically.
func TestChromeTextColorLeavesOrdinaryLayoutsAlone(t *testing.T) {
	v := chromeTextColor([]byte(plainLayoutXML), modernThemeColors(), 0)
	if v.hex != "" || v.swap != nil || v.unfixable {
		t.Errorf("ordinary layout should be untouched, got %+v", v)
	}
	// No background at all is also not an invitation to guess.
	v = chromeTextColor([]byte(`<p:sldLayout><p:cSld><p:spTree/></p:cSld></p:sldLayout>`), modernThemeColors(), 0)
	if v.hex != "" || v.swap != nil {
		t.Errorf("layout with no background should be untouched, got %+v", v)
	}
}

// TestChromeTextColorOnDarkLayout covers the other direction: a dark background
// with no override, where tx1 is near-black and chrome must go light.
func TestChromeTextColorOnDarkLayout(t *testing.T) {
	v := chromeTextColor([]byte(darkLayoutXML), modernThemeColors(), 1)
	if v.hex == "" {
		t.Fatal("near-black chrome on a #14213D background was left as is")
	}
	if v.swap == nil || v.swap.RatioAfter < 4.5 {
		t.Fatalf("chrome not raised to WCAG AA: %+v", v.swap)
	}
}

// TestChromeFillPinsColorOnlyWhenAsked checks the emitted XML: an empty color
// keeps the scheme reference, a set color pins an explicit srgbClr.
func TestChromeFillPinsColorOnlyWhenAsked(t *testing.T) {
	pos := computeDefaultFooterPositions(6858000)
	cfg := &FooterConfig{Enabled: true, LeftText: "Confidential", PageNumberFormat: "{current} / {total}", TotalSlides: 10}

	inherited := generateFooterShapes(pos, cfg, 100, "", "")
	if !strings.Contains(inherited, `<a:schemeClr val="tx1"/>`) {
		t.Errorf("default chrome should inherit schemeClr tx1:\n%s", inherited)
	}
	if strings.Contains(inherited, "<a:srgbClr") {
		t.Errorf("default chrome should not pin an explicit color:\n%s", inherited)
	}

	pinned := generateFooterShapes(pos, cfg, 100, "", "#2C3932")
	if strings.Contains(pinned, `<a:schemeClr val="tx1"/>`) {
		t.Errorf("pinned chrome still carries the scheme color that would be invisible:\n%s", pinned)
	}
	if n := strings.Count(strings.ToUpper(pinned), `<A:SRGBCLR VAL="2C3932"`); n < 2 {
		t.Errorf("expected every chrome run pinned to 2C3932, found %d:\n%s", n, pinned)
	}
}
