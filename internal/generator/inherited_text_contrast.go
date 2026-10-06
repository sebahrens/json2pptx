package generator

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
	"github.com/sebahrens/json2pptx/svggen"
)

// Inherited text contrast (go-slide-creator-ucmgr).
//
// The contrast pass rewrote scheme colors it found in the SLIDE's own XML. Text
// that names no color at all was invisible to it, because there was nothing to
// rewrite: examples/basic-deck.json on modern-template put five white bullets on
// a white section divider and the pass emitted neither a fix nor a finding. The
// color came from the master's bodyStyle (schemeClr tx1) through the layout's
// clrMapOvr (tx1 -> lt1), two files the pass never opened.
//
// So the inherited color is resolved the way the renderer resolves it — slide
// lstStyle, then the LAYOUT's placeholder lstStyle, then the master's txStyles,
// each mapped through the layout's color map override — and when the answer
// fails WCAG against the background the slide actually shows, an explicit color
// is written onto the slide's runs. A swap is recorded either way, so the deck
// reports what it did.

// inheritedColorSource names where a resolved inherited color came from, for the
// swap record an agent reads.
const (
	inheritedSourceLayout = "layout-lstStyle"
	inheritedSourceMaster = "master-txStyles"
)

// layoutShapeRegexp matches one <p:sp> element in a layout or master.
var layoutShapeRegexp = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)

// phAttrsRegexp matches a placeholder element, capturing its attribute list.
var phAttrsRegexp = regexp.MustCompile(`<p:ph\s*([^/>]*)/?>`)

// lstStyleRegexp matches an <a:lstStyle> element and captures its body.
var lstStyleRegexp = regexp.MustCompile(`(?s)<a:lstStyle>(.*?)</a:lstStyle>`)

// solidFillSchemeRegexp captures the scheme color of the first solidFill in a
// fragment; solidFillSRGBRegexp does the same for a literal sRGB value.
var (
	solidFillSchemeRegexp = regexp.MustCompile(`<a:solidFill>\s*<a:schemeClr val="([^"]+)"`)
	solidFillSRGBRegexp   = regexp.MustCompile(`<a:solidFill>\s*<a:srgbClr val="([0-9A-Fa-f]{6})"`)
)

// inheritedSizeRegexp captures a default run size (hundredths of a point).
var inheritedSizeRegexp = regexp.MustCompile(`<a:defRPr[^>]*\bsz="(\d+)"`)

// inheritedBoldRegexp captures a default run's bold flag.
var inheritedBoldRegexp = regexp.MustCompile(`<a:defRPr[^>]*\bb="([01])"`)

// masterStyleForPlaceholder maps a placeholder type to the master text style
// that supplies its defaults.
func masterStyleForPlaceholder(phType string) string {
	switch phType {
	case "title", "ctrTitle":
		return "titleStyle"
	case "body", "subTitle", "":
		return "bodyStyle"
	default:
		return "otherStyle"
	}
}

// inheritedTextStyleFragment returns the styling fragment one list level
// (1-based, as in lvlNpPr) of a placeholder inherits and where it came from. It
// looks in the layout's matching placeholder first, then the master's text
// styles — the renderer's own order.
func inheritedTextStyleFragment(layoutXML, masterXML []byte, ph *placeholderXML, level int) (fragment, source string) {
	if ph == nil {
		return "", ""
	}
	if frag := layoutPlaceholderLevelStyle(layoutXML, ph, level); frag != "" {
		return frag, inheritedSourceLayout
	}
	if frag := masterTextStyleLevel(masterXML, masterStyleForPlaceholder(ph.Type), level); frag != "" {
		return frag, inheritedSourceMaster
	}
	return "", ""
}

// listStyleLevelElement returns the <a:lvlNpPr> element of a list style for a
// 1-based level, "" when the style does not define that level.
func listStyleLevelElement(lstStyleInner string, level int) string {
	for _, loc := range listLevelRegexp.FindAllStringSubmatchIndex(lstStyleInner, -1) {
		if lstStyleInner[loc[2]:loc[3]] == strconv.Itoa(level) {
			return lstStyleInner[loc[0]:loc[1]]
		}
	}
	return ""
}

// layoutPlaceholderLevelStyle returns the list-style fragment for one level
// (1-based) of the layout shape holding the same placeholder, or "" when the
// layout leaves that level to the master.
//
// A list style that defines no first level styles it through whatever it
// states outside the level elements; the other levels' elements are not the
// first level's style, so they are left out of that answer (a colour stated
// on lvl2pPr alone used to be read as the first level's).
func layoutPlaceholderLevelStyle(layoutXML []byte, ph *placeholderXML, level int) string {
	for _, sp := range layoutShapeRegexp.FindAllString(string(layoutXML), -1) {
		if !placeholderMatches(sp, ph) {
			continue
		}
		body := lstStyleRegexp.FindStringSubmatch(sp)
		if body == nil {
			return ""
		}
		if lvl := listStyleLevelElement(body[1], level); lvl != "" {
			return lvl
		}
		if level == 1 {
			return strings.TrimSpace(listLevelRegexp.ReplaceAllString(body[1], ""))
		}
		return ""
	}
	return ""
}

// layoutPlaceholderSolidFill resolves a placeholder's own area fill. That
// fill is painted above the slide background, so contrast against <p:bg>
// alone can produce the opposite text color from the one the viewer needs.
func layoutPlaceholderSolidFill(layoutXML []byte, ph *placeholderXML, themeColors []types.ThemeColor, slideBackground string, override map[string]string) string {
	if ph == nil {
		return ""
	}
	for _, sp := range layoutShapeRegexp.FindAllString(string(layoutXML), -1) {
		if placeholderMatches(sp, ph) {
			fill := extractShapeFillHex([]byte(sp), themeColors, slideBackground)
			if fill == "" || len(override) == 0 {
				return fill
			}
			// Scheme colors on a layout shape obey its clrMapOvr just like
			// inherited text. Resolve the mapped base before applying tint/alpha.
			spPr := []byte(sp)
			if end := bytes.Index(spPr, []byte("</p:spPr>")); end >= 0 {
				spPr = spPr[:end]
			}
			if m := shapeFillSchemeRegexp.FindSubmatch(spPr); len(m) > 1 {
				return applyShapeFillModifiers(resolveSchemeColorMapped(string(m[1]), override, themeColors), spPr, themeColors, slideBackground)
			}
			return fill
		}
	}
	return ""
}

// placeholderMatches reports whether a layout shape carries the same placeholder
// identity as a slide shape. A placeholder is identified by idx when it has one
// (several body placeholders differ only by index) and by type otherwise.
func placeholderMatches(shapeXML string, ph *placeholderXML) bool {
	m := phAttrsRegexp.FindStringSubmatch(shapeXML)
	if m == nil {
		return false
	}
	attrs := m[1]
	layoutType := attrValue(attrs, "type")
	layoutIdx := attrValue(attrs, "idx")

	if ph.Index != nil {
		return layoutIdx == strconv.Itoa(*ph.Index)
	}
	if ph.Type != "" {
		// A title placeholder is spelled either way across templates.
		if isTitleType(ph.Type) {
			return isTitleType(layoutType)
		}
		return layoutType == ph.Type && layoutIdx == ""
	}
	return layoutType == "" && layoutIdx == ""
}

func isTitleType(t string) bool { return t == "title" || t == "ctrTitle" }

// attrValue reads one attribute out of an attribute list.
func attrValue(attrs, name string) string {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `="([^"]*)"`)
	if m := re.FindStringSubmatch(attrs); m != nil {
		return m[1]
	}
	return ""
}

// masterTextStyleLevel returns the fragment for one level (1-based) of one of
// the master's <p:txStyles> entries (titleStyle / bodyStyle / otherStyle). A
// style that defines no first level answers with its whole body for level 1.
func masterTextStyleLevel(masterXML []byte, style string, level int) string {
	if style == "" {
		return ""
	}
	re := regexp.MustCompile(`(?s)<p:` + style + `>(.*?)</p:` + style + `>`)
	m := re.FindSubmatch(masterXML)
	if m == nil {
		return ""
	}
	if lvl := listStyleLevelElement(string(m[1]), level); lvl != "" {
		return lvl
	}
	if level == 1 {
		return string(m[1])
	}
	return ""
}

// inheritedColorFromFragment resolves the text color a styling fragment implies,
// mapped through the layout's color map override. It returns the hex value and
// the scheme name it came from ("" for a literal sRGB color).
func inheritedColorFromFragment(fragment string, override map[string]string, themeColors []types.ThemeColor) (hex, scheme string) {
	if m := solidFillSchemeRegexp.FindStringSubmatch(fragment); m != nil {
		return resolveSchemeColorMapped(m[1], override, themeColors), m[1]
	}
	if m := solidFillSRGBRegexp.FindStringSubmatch(fragment); m != nil {
		return "#" + strings.ToUpper(m[1]), ""
	}
	return "", ""
}

// inheritedTextThreshold returns the WCAG threshold for text whose size comes
// from an inherited fragment.
func inheritedTextThreshold(fragment string) float64 {
	pt := 0.0
	if m := inheritedSizeRegexp.FindStringSubmatch(fragment); m != nil {
		if hundredths, err := strconv.Atoi(m[1]); err == nil && hundredths > 0 {
			pt = float64(hundredths) / 100.0
		}
	}
	bold := false
	if m := inheritedBoldRegexp.FindStringSubmatch(fragment); m != nil {
		bold = m[1] == "1"
	}
	return contrastThresholdFor(pt, bold)
}

// levelNamesNoColor reports whether a shape leaves the text color of one list
// level (1-based) entirely inherited. A level that names a color — in the
// shape's list style or on one of its runs — is the existing pass's job:
// rewriting it here would double-fix it and record two swaps for one change.
//
// The question is asked per level because a list style may state a colour on
// one level a shape uses and leave another to the layout or master: the
// lstStyle pass fixes the first, and the second still needs resolving here
// (go-slide-creator-50xzk). A colour the slide states on a level no paragraph
// uses colours nothing, and the lstStyle pass skips it too
// (go-slide-creator-6s2w4).
func levelNamesNoColor(shape *shapeXML, level int) bool {
	if shape.TextBody == nil {
		return false
	}
	if shape.TextBody.ListStyle != nil && listStyleNamesColorOnUsedLevel(shape.TextBody.ListStyle.Inner, map[int]bool{level: true}) {
		return false
	}
	for pi := range shape.TextBody.Paragraphs {
		para := &shape.TextBody.Paragraphs[pi]
		if paragraphListLevel(para) != level {
			continue
		}
		for ri := range para.Runs {
			rPr := para.Runs[ri].RunProperties
			if rPr != nil && strings.Contains(rPr.Inner, "<a:solidFill>") {
				return false
			}
		}
	}
	return true
}

// shapeHasText reports whether a shape carries any run text worth colouring.
func shapeHasText(shape *shapeXML) bool {
	if shape.TextBody == nil {
		return false
	}
	for pi := range shape.TextBody.Paragraphs {
		for ri := range shape.TextBody.Paragraphs[pi].Runs {
			if strings.TrimSpace(shape.TextBody.Paragraphs[pi].Runs[ri].Text) != "" {
				return true
			}
		}
	}
	return false
}

// pinRunColors writes an explicit solid fill onto every run of a shape that
// sits on the given list level (1-based). The color goes on the runs rather
// than the shape's lstStyle so the slide states exactly what was checked: the
// paragraphs of one level, at that level's inherited colour.
func pinRunColors(shape *shapeXML, level int, hex string) {
	fill := fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, strings.TrimPrefix(strings.ToUpper(hex), "#"))
	for pi := range shape.TextBody.Paragraphs {
		para := &shape.TextBody.Paragraphs[pi]
		if paragraphListLevel(para) != level {
			continue
		}
		for ri := range para.Runs {
			run := &para.Runs[ri]
			if run.RunProperties == nil {
				run.RunProperties = &runPropertiesXML{}
			}
			// solidFill precedes the font elements in the rPr child order, and a
			// run this pass touches carries no fill of its own, so prepending is
			// both correct and enough.
			run.RunProperties.Inner = fill + run.RunProperties.Inner
		}
	}
}

// enforceInheritedTextContrast fixes placeholder text whose color is inherited
// rather than stated. It is the counterpart to enforceTextContrastInSlide, which
// only sees colors the slide itself names.
//
// masterXML may be nil: a layout that styles its own placeholders needs no
// master, and without either file there is nothing to resolve and nothing is
// changed.
func enforceInheritedTextContrast(slide *slideXML, layoutXML, masterXML []byte, bgHex string, themeColors []types.ThemeColor, slideIndex int, override map[string]string) []ContrastSwap {
	return enforceInheritedTextContrastExcept(slide, layoutXML, masterXML, bgHex, themeColors, slideIndex, override, nil)
}

func enforceInheritedTextContrastExcept(slide *slideXML, layoutXML, masterXML []byte, bgHex string, themeColors []types.ThemeColor, slideIndex int, override map[string]string, excluded map[int]bool) []ContrastSwap {
	if slide == nil || len(layoutXML) == 0 {
		return nil
	}

	var swaps []ContrastSwap
	for i := range slide.CommonSlideData.ShapeTree.Shapes {
		if excluded[i] {
			continue
		}
		shape := &slide.CommonSlideData.ShapeTree.Shapes[i]
		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph == nil || !shapeHasText(shape) {
			continue
		}
		effectiveBG := bgHex
		if ownFill := layoutPlaceholderSolidFill(layoutXML, ph, themeColors, bgHex, override); ownFill != "" {
			effectiveBG = ownFill
		}
		bg, bgErr := svggen.ParseColor(effectiveBG)
		if bgErr != nil {
			continue
		}
		// Each level the text sits on is resolved and judged on its own: its
		// colour and size come from that level's fragment. One colour fixed the
		// same way on several levels is one decision, recorded once.
		used := usedListLevels(shape)
		recorded := map[string]bool{}
		for level := 1; level <= 9; level++ {
			if !used[level] || !levelNamesNoColor(shape, level) {
				continue
			}
			swap, ok := inheritedLevelSwap(layoutXML, masterXML, ph, level, bg, themeColors, override)
			if !ok {
				continue
			}
			pinRunColors(shape, level, swap.ReplacedColor)
			key := swap.OriginalColor + ">" + swap.ReplacedColor + "@" + swap.Source
			if recorded[key] {
				continue
			}
			recorded[key] = true
			swap.SlideIndex = slideIndex
			swap.Path = slidepath.Slide(slideIndex)
			swaps = append(swaps, swap)
		}
	}
	return swaps
}

// inheritedLevelSwap resolves the colour one list level (1-based) of a
// placeholder inherits and, when it fails WCAG on bg, the replacement for it.
func inheritedLevelSwap(layoutXML, masterXML []byte, ph *placeholderXML, level int, bg svggen.Color, themeColors []types.ThemeColor, override map[string]string) (ContrastSwap, bool) {
	fragment, source := inheritedTextStyleFragment(layoutXML, masterXML, ph, level)
	if fragment == "" {
		return ContrastSwap{}, false
	}
	hex, _ := inheritedColorFromFragment(fragment, override, themeColors)
	if hex == "" && source == inheritedSourceLayout {
		// A layout can override only the font size or weight. Its colorless
		// level fragment must not mask the color supplied by master txStyles.
		masterFragment := masterTextStyleLevel(masterXML, masterStyleForPlaceholder(ph.Type), level)
		hex, _ = inheritedColorFromFragment(masterFragment, override, themeColors)
		if hex != "" {
			source = inheritedSourceMaster
		}
	}
	if hex == "" {
		return ContrastSwap{}, false
	}
	current, perr := svggen.ParseColor(hex)
	if perr != nil {
		return ContrastSwap{}, false
	}
	threshold := inheritedTextThreshold(fragment)
	before := current.ContrastWith(bg)
	if before >= threshold {
		return ContrastSwap{}, false
	}
	// pickThemeTextColor ends in a tonal shade of the background, so it
	// always has an answer; a pick that is no better than what it replaces
	// would be a change for nothing, so leave the slide alone.
	pick := pickThemeTextColor(bg, themeColors, threshold)
	after := pick.Color.ContrastWith(bg)
	if after <= before {
		return ContrastSwap{}, false
	}
	return ContrastSwap{
		OriginalColor:   strings.ToUpper(current.Hex()),
		ReplacedColor:   pick.Hex,
		BackgroundColor: strings.ToUpper(bg.Hex()),
		RatioBefore:     before,
		RatioAfter:      after,
		Source:          source,
	}, true
}

// masterXMLForLayout reads the slide master a layout inherits from. It returns
// nil when the master cannot be resolved, which leaves inherited-color checking
// to whatever the layout itself declares.
func (ctx *singlePassContext) masterXMLForLayout(layoutID string) []byte {
	path := ctx.masterPathForLayout(layoutID)
	if path == "" {
		return nil
	}
	if ctx.masterXMLCache == nil {
		ctx.masterXMLCache = map[string][]byte{}
	}
	if data, ok := ctx.masterXMLCache[path]; ok {
		return data
	}
	data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, path)
	if err != nil {
		data = nil
	}
	ctx.masterXMLCache[path] = data
	return data
}
