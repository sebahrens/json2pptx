package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

const (
	// chromeDefaultScheme is the scheme color footer and page-number runs are
	// emitted in when nothing overrides them.
	chromeDefaultScheme = "tx1"
	// chromeSwapSource labels contrast swaps made on deck chrome, so a
	// contrast_autofixed finding says which surface moved.
	chromeSwapSource = "chrome"
)

// chromeSwapPath is the JSON pointer a chrome contrast swap is reported at.
func chromeSwapPath(slideIndex int) string {
	return slidepath.SlideField(slideIndex, "chrome")
}

// Chrome contrast (go-slide-creator-hln7).
//
// Deck chrome — the footer line and the page number — is injected as literal
// <a:schemeClr val="tx1"/>, which is right for almost every layout and exactly
// wrong for a layout that inverts its color map. modern-template's section
// divider paints its background with schemeClr tx1 under
// <a:overrideClrMapping tx1="lt1" .../>, so both the background and the chrome
// resolve to lt1: white text on a white slide, with no finding emitted anywhere
// because contrast enforcement only ever looked at placeholders and shape grids.
//
// Two things were wrong. The color map override was ignored, so the background
// was read as dk1 (dark) when it renders lt1 (white); and chrome never went
// through contrast enforcement at all. Both are fixed here: the layout's
// override is parsed and applied wherever a scheme color is resolved, and the
// chrome color is chosen against the background the slide will actually show.

// overrideClrMappingRegexp matches a layout's <a:overrideClrMapping> element,
// capturing its attribute list.
var overrideClrMappingRegexp = regexp.MustCompile(`<a:overrideClrMapping\s+([^/>]+)/?>`)

// clrMapAttrRegexp matches one name="value" pair inside a color map element.
var clrMapAttrRegexp = regexp.MustCompile(`([A-Za-z0-9]+)="([^"]+)"`)

// parseLayoutColorMapOverride returns a layout's <p:clrMapOvr> mapping — the
// scheme-slot renaming the layout applies on top of the master's map (e.g.
// tx1 -> lt1 on an inverted section divider). Returns nil when the layout
// inherits the master mapping, which is the common case.
func parseLayoutColorMapOverride(layoutXML []byte) map[string]string {
	m := overrideClrMappingRegexp.FindSubmatch(layoutXML)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for _, kv := range clrMapAttrRegexp.FindAllStringSubmatch(string(m[1]), -1) {
		out[kv[1]] = kv[2]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// applyColorMapOverride maps a scheme color name through a layout's color map
// override. Names the override does not mention pass through unchanged.
func applyColorMapOverride(scheme string, override map[string]string) string {
	if mapped, ok := override[scheme]; ok && mapped != "" {
		return mapped
	}
	return scheme
}

// resolveSchemeColorMapped resolves a scheme color to hex the way the renderer
// will: through the layout's color map override first, then the theme.
func resolveSchemeColorMapped(scheme string, override map[string]string, themeColors []types.ThemeColor) string {
	return resolveSchemeColorToHex(applyColorMapOverride(scheme, override), themeColors)
}

// chromeVerdict is what the contrast check concluded for one layout's chrome.
type chromeVerdict struct {
	// hex is the color to pin chrome to; empty means leave it inheriting
	// schemeClr tx1.
	hex string
	// swap is the contrast_autofixed record for the change, nil when nothing
	// changed.
	swap *ContrastSwap
	// unfixable reports that the default is unreadable and nothing in the
	// template's palette reads better. Surfaced as a warning rather than a swap:
	// claiming an auto-fix that did not happen is worse than saying so.
	unfixable bool
	// ratio and bgHex describe the unreadable combination, for that warning.
	ratio float64
	bgHex string
}

// chromeTextColor picks the color deck chrome should be drawn in on a layout.
//
// The verdict is empty when the default (schemeClr tx1) already clears WCAG AA
// against the layout background — the overwhelmingly common case, left untouched
// so chrome keeps inheriting the template's palette.
func chromeTextColor(layoutXML []byte, themeColors []types.ThemeColor, slideIndex int) chromeVerdict {
	bgHex := extractLayoutBackgroundColor(layoutXML, themeColors)
	if bgHex == "" {
		return chromeVerdict{} // unknown background: leave chrome alone rather than guess
	}
	bg, err := svggen.ParseColor(bgHex)
	if err != nil {
		return chromeVerdict{}
	}

	override := parseLayoutColorMapOverride(layoutXML)
	defaultHex := resolveSchemeColorMapped(chromeDefaultScheme, override, themeColors)
	if defaultHex == "" {
		return chromeVerdict{}
	}
	current, err := svggen.ParseColor(defaultHex)
	if err != nil {
		return chromeVerdict{}
	}

	before := current.ContrastWith(bg)
	if before >= svggen.WCAGAANormal {
		return chromeVerdict{}
	}

	pick := pickThemeTextColor(bg, themeColors, svggen.WCAGAANormal)
	after := pick.Color.ContrastWith(bg)
	if after <= before {
		return chromeVerdict{unfixable: true, ratio: before, bgHex: strings.ToUpper(bg.Hex())}
	}
	return chromeVerdict{
		hex: pick.Hex,
		swap: &ContrastSwap{
			OriginalColor:   strings.ToUpper(current.Hex()),
			ReplacedColor:   pick.Hex,
			BackgroundColor: strings.ToUpper(bg.Hex()),
			RatioBefore:     before,
			RatioAfter:      after,
			SlideIndex:      slideIndex,
			Path:            chromeSwapPath(slideIndex),
			Source:          chromeSwapSource,
		},
	}
}

// chromeTextColorForLayout resolves the chrome color for one slide's layout,
// recording a contrast swap when the default would be unreadable. Layouts are
// resolved repeatedly across a deck, so the answer is memoized per layout.
//
// Returns "" to mean "leave chrome inheriting schemeClr tx1".
func (ctx *singlePassContext) chromeTextColorForLayout(layoutID string, slideIndex int) string {
	if layoutID == "" {
		return ""
	}
	if ctx.chromeColorCache == nil {
		ctx.chromeColorCache = map[string]chromeVerdict{}
	}
	verdict, ok := ctx.chromeColorCache[layoutID]
	if !ok {
		layoutData, err := ctx.readLayoutFile(layoutID)
		if err != nil {
			ctx.chromeColorCache[layoutID] = chromeVerdict{}
			return ""
		}
		verdict = chromeTextColor(layoutData, ctx.themeColors, slideIndex)
		ctx.chromeColorCache[layoutID] = verdict
	}
	switch {
	case verdict.swap != nil:
		// Every slide that shows the chrome gets its own finding: the decision is
		// per-layout but the evidence an agent reads is per-slide.
		swap := *verdict.swap
		swap.SlideIndex = slideIndex
		swap.Path = chromeSwapPath(slideIndex)
		ctx.contrastSwaps = append(ctx.contrastSwaps, swap)
	case verdict.unfixable:
		ctx.warnings = append(ctx.warnings, fmt.Sprintf(
			"slide %d: footer/page-number chrome is unreadable on layout %q (ratio %.1f against %s) and no template color reads better; set chrome text explicitly or use a different layout",
			slideIndex+1, layoutID, verdict.ratio, verdict.bgHex))
	}
	return verdict.hex
}
