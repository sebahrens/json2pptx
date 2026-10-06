package template

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Per-level placeholder text colour (go-slide-creator-50xzk)
//
// PlaceholderInfo's FontColor / InheritedFontColor describe list level 0 only.
// A nested bullet renders at a deeper level, whose colour the layout's
// lstStyle or the master's text style may state separately; the render-time
// contrast passes check each level a paragraph sits on
// (internal/generator/text_contrast.go, inherited_text_contrast.go), so the
// validate-time preflight needs the same per-level colour or it stays silent
// about a swap generate goes on to make.

// maxListLevel is the deepest a:pPr lvl value OOXML defines (lvl9pPr).
const maxListLevel = 8

var (
	masterBodyStyleRe = regexp.MustCompile(`(?s)<p:bodyStyle>(.*?)</p:bodyStyle>`)
	listLevelElemRe   = regexp.MustCompile(`(?s)<a:lvl([1-9])pPr\b(?:[^>]*/>|[^>]*>(.*?)</a:lvl[1-9]pPr>)`)
)

// FirstBulletLevel returns the first level of a slide master's bodyStyle that
// draws a bullet marker (0-based: lvl1pPr is 0), which is the a:pPr lvl the
// generator writes a top-level bullet at. A level that carries <a:buNone/> is
// a heading level without a marker. It returns -1 when the master has no
// bodyStyle or every level it defines is unmarked.
func FirstBulletLevel(masterXML []byte) int {
	m := masterBodyStyleRe.FindSubmatch(masterXML)
	if m == nil {
		return -1
	}
	first := -1
	for _, lvl := range listLevelElemRe.FindAllSubmatch(m[1], -1) {
		level := int(lvl[1][0] - '1')
		if strings.Contains(string(lvl[2]), "buNone") {
			continue
		}
		if first < 0 || level < first {
			first = level
		}
	}
	return first
}

// masterLevelStyle returns the master text style a placeholder's list level
// inherits from, chosen by the placeholder's XML type exactly as the
// render-time inherited-colour pass chooses it: titleStyle for a title,
// bodyStyle for a body / subtitle / untyped placeholder, otherStyle for the
// rest. The title style is parsed at its first level only.
func masterLevelStyle(xmlType string, level int, masterFonts *MasterFontStyles) *FontStyle {
	if masterFonts == nil {
		return nil
	}
	switch xmlType {
	case "title", "ctrTitle":
		return nil
	case "body", "subTitle", "":
		return masterFonts.BodyStyle[level]
	default:
		return masterFonts.OtherStyle[level]
	}
}

// placeholderLevelText resolves the text colour of every list level below the
// first that the layout placeholder or its master states a colour for.
//
// The size and weight recorded with a colour are the ones the render-time
// pass reads its WCAG threshold from: the layout's own level element when the
// layout defines that level (whether or not it states a size), else the
// master's.
func placeholderLevelText(shape *shapeXML, xmlType string, masterFonts *MasterFontStyles, clrMapOvr map[string]string) []types.PlaceholderLevelText {
	var layoutList *listStyleXML
	if shape != nil && shape.TextBody != nil {
		layoutList = shape.TextBody.ListStyle
	}
	var out []types.PlaceholderLevelText
	for level := 1; level <= maxListLevel; level++ {
		layout := layoutList.level(level)
		master := masterLevelStyle(xmlType, level, masterFonts)

		entry := types.PlaceholderLevelText{Level: level}
		if layout != nil && layout.DefRPr != nil {
			entry.FontSize = layout.DefRPr.Size
			entry.Bold = layout.DefRPr.Bold == "1"
			entry.Color = solidFillColorRef(layout.DefRPr.SolidFill)
			entry.ColorMods = colorModifiersFromSolidFill(layout.DefRPr.SolidFill)
		}
		if entry.Color == "" && master != nil {
			entry.FromMaster = true
			entry.Color = master.FontColor
			entry.ColorMods = master.ColorMods
			if layout == nil {
				entry.FontSize = master.FontSize
				entry.Bold = master.Bold
			}
		}
		if entry.Color == "" {
			continue
		}
		entry.Color = inheritedPlaceholderColor(entry.Color, clrMapOvr)
		out = append(out, entry)
	}
	return out
}

// solidFillColorRef returns the colour a solid fill names: "#RRGGBB" for a
// literal, the scheme name for a scheme reference, "" for neither.
func solidFillColorRef(fill *solidFillXML) string {
	if fill == nil {
		return ""
	}
	if fill.SRGBColor != nil && fill.SRGBColor.Val != "" {
		return normalizeColorHex(fill.SRGBColor.Val)
	}
	if fill.SchemeColor != nil && fill.SchemeColor.Val != "" {
		return fill.SchemeColor.Val
	}
	return ""
}
