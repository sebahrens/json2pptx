package generator

import (
	"fmt"
	"regexp"
	"strings"
)

// nestedUnmarkedIndentEMU is the indent per nesting depth of a sub-item
// under an unmarked parent level (18pt).
const nestedUnmarkedIndentEMU = 228600

// bulletMarkerElemRegexp matches every bullet-marker child of a paragraph
// property block that would make a paragraph render a glyph or number.
var bulletMarkerElemRegexp = regexp.MustCompile(
	`(?s)<a:bu(?:Char|AutoNum|Blip|None|Font|FontTx|Clr|ClrTx|SzPct|SzPts|SzTx)\b[^>]*?(?:/>|>.*?</a:bu(?:Char|AutoNum|Blip|None|Font|FontTx|Clr|ClrTx|SzPct|SzPts|SzTx)>)`)

// levelMarkerState reports how a paragraph property block or list-style
// level block declares its bullet: "none" (buNone), "marked" (buChar,
// buAutoNum, buBlip) or "" (inherits).
func levelMarkerState(inner string) string {
	switch {
	case strings.Contains(inner, "<a:buNone"):
		return "none"
	case strings.Contains(inner, "<a:buChar"), strings.Contains(inner, "<a:buAutoNum"), strings.Contains(inner, "<a:buBlip"):
		return "marked"
	}
	return ""
}

// listStyleLevelBlock returns the lvlNpPr block for a 0-based level.
func listStyleLevelBlock(inner string, level int) string {
	tag := fmt.Sprintf("a:lvl%dpPr", level+1)
	start := strings.Index(inner, "<"+tag)
	if start < 0 {
		return ""
	}
	block := inner[start:]
	if end := strings.Index(block, "</"+tag+">"); end >= 0 {
		return block[:end]
	}
	if end := strings.Index(block, "/>"); end >= 0 {
		return block[:end]
	}
	return block
}

// baseLevelUnmarked reports whether paragraphs at baseLevel render without
// a marker: their own properties say buNone, or they inherit and the shape's
// (layout-copied) list style disables bullets at that level.
func baseLevelUnmarked(paras []paragraphXML, listStyle *listStyleXML, baseLevel int) bool {
	for _, p := range paras {
		if p.Properties == nil || p.Properties.Level == nil || *p.Properties.Level != baseLevel {
			continue
		}
		if state := levelMarkerState(p.Properties.Inner); state != "" {
			return state == "none"
		}
		break // the first base-level paragraph decides; it inherits
	}
	if listStyle == nil {
		return false
	}
	return levelMarkerState(listStyleLevelBlock(listStyle.Inner, baseLevel)) == "none"
}

// unmarkNestedUnderUnmarkedBase keeps marker use consistent across levels
// (go-slide-creator-yjl8d). A layout may disable bullets on its first body
// level (abstract's content layout sets lvl1 buNone); a nested item then
// inherits the master's next-level glyph and deep margin, so marked children
// sit under unmarked parents and the hierarchy reads inverted. When the base
// level renders unmarked, deeper paragraphs are unmarked too and indented a
// modest 18pt per depth from the parent text edge. paras are the bullet
// paragraphs (no column header); listStyle is the shape's list style.
func unmarkNestedUnderUnmarkedBase(paras []paragraphXML, listStyle *listStyleXML, baseLevel int) {
	if !baseLevelUnmarked(paras, listStyle, baseLevel) {
		return
	}
	for i := range paras {
		props := paras[i].Properties
		if props == nil || props.Level == nil || *props.Level <= baseLevel {
			continue
		}
		cloned := *props
		depth := *props.Level - baseLevel
		marL := depth * nestedUnmarkedIndentEMU
		indent := 0
		cloned.MarL = &marL
		cloned.Indent = &indent
		cloned.Inner = insertBuNone(bulletMarkerElemRegexp.ReplaceAllString(cloned.Inner, ""))
		paras[i].Properties = &cloned
	}
}

// insertBuNone adds <a:buNone/> in schema order: after spacing and bullet
// formatting, before tabLst / defRPr / extLst.
func insertBuNone(inner string) string {
	pos := len(inner)
	for _, tag := range []string{"<a:tabLst", "<a:defRPr", "<a:extLst"} {
		if i := strings.Index(inner, tag); i >= 0 && i < pos {
			pos = i
		}
	}
	return inner[:pos] + `<a:buNone/>` + inner[pos:]
}
