package template

import (
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
)

// InheritedTextStyle captures the inherited paragraph/run defaults that change
// how much text physically fits in a placeholder: font size, all-caps, line
// spacing, space-before and the latin typeface. Placeholders on generated
// slides usually carry none of these explicitly — they inherit them from the
// slide master's txStyles — so measuring text without them under-estimates the
// rendered height (e.g. a 45pt all-caps title measured as 20pt mixed case).
type InheritedTextStyle struct {
	// SizeHPt is the font size in hundredths of a point (0 = unknown).
	SizeHPt int
	// CapsAll is true when text renders in all caps (cap="all").
	CapsAll bool
	// LineSpacingPct is the lnSpc spcPct value in percent (e.g. 80 for
	// <a:spcPct val="80000"/>). 0 means unspecified (single spacing).
	LineSpacingPct int
	// SpcBefPt is the space-before in points (spcPts only; 0 when unset).
	SpcBefPt float64
	// Typeface is the raw latin typeface (may be a theme token like "+mj-lt").
	Typeface string
	// Align is the level-1 paragraph alignment ("l", "ctr", "r", …); "" when
	// the style declares none (OOXML then aligns left).
	Align string
}

type masterTitleStyleXML struct {
	XMLName xml.Name            `xml:"sldMaster"`
	Lvl1    *titleLevelPropsXML `xml:"txStyles>titleStyle>lvl1pPr"`
}

type masterBodyStyleXML struct {
	XMLName xml.Name            `xml:"sldMaster"`
	Lvl1    *titleLevelPropsXML `xml:"txStyles>bodyStyle>lvl1pPr"`
}

type titleLevelPropsXML struct {
	Algn   string      `xml:"algn,attr"`
	LnSpc  *spacingXML `xml:"lnSpc"`
	SpcBef *spacingXML `xml:"spcBef"`
	DefRPr *struct {
		Size  int           `xml:"sz,attr"`
		Cap   string        `xml:"cap,attr"`
		Latin *latinFontXML `xml:"latin"`
	} `xml:"defRPr"`
}

type spacingXML struct {
	SpcPct *struct {
		Val int `xml:"val,attr"`
	} `xml:"spcPct"`
	SpcPts *struct {
		Val int `xml:"val,attr"`
	} `xml:"spcPts"`
}

// ParseMasterTitleStyle extracts the level-1 title text style from a slide
// master's <p:txStyles><p:titleStyle>. Returns the zero value when the master
// cannot be parsed or declares no title style.
func ParseMasterTitleStyle(masterData []byte) InheritedTextStyle {
	var m masterTitleStyleXML
	if err := xml.Unmarshal(masterData, &m); err != nil || m.Lvl1 == nil {
		return InheritedTextStyle{}
	}
	return inheritedFromLevelProps(m.Lvl1)
}

// ParseMasterBodyStyle extracts the level-1 body text style from a slide
// master's <p:txStyles><p:bodyStyle>. Returns the zero value when the master
// cannot be parsed or declares no body style.
//
// The space-before it carries is what a body placeholder's autofit has to
// budget for: with fourteen bullets, a 10pt spcBef is 140pt of height the
// preflight was not counting, and validate predicted a 70% font scale where
// the renderer applied 50% (go-slide-creator-nlrg).
func ParseMasterBodyStyle(masterData []byte) InheritedTextStyle {
	var m masterBodyStyleXML
	if err := xml.Unmarshal(masterData, &m); err != nil || m.Lvl1 == nil {
		return InheritedTextStyle{}
	}
	return inheritedFromLevelProps(m.Lvl1)
}

// inheritedFromLevelProps converts one parsed lvl1pPr into an InheritedTextStyle.
func inheritedFromLevelProps(lvl *titleLevelPropsXML) InheritedTextStyle {
	st := InheritedTextStyle{Align: lvl.Algn}
	if lvl.LnSpc != nil && lvl.LnSpc.SpcPct != nil && lvl.LnSpc.SpcPct.Val > 0 {
		st.LineSpacingPct = lvl.LnSpc.SpcPct.Val / 1000
	}
	if lvl.SpcBef != nil && lvl.SpcBef.SpcPts != nil {
		st.SpcBefPt = float64(lvl.SpcBef.SpcPts.Val) / 100.0
	}
	if r := lvl.DefRPr; r != nil {
		st.SizeHPt = r.Size
		st.CapsAll = r.Cap == "all"
		if r.Latin != nil {
			st.Typeface = r.Latin.Typeface
		}
	}
	return st
}

var (
	lstLnSpcPctRegexp = regexp.MustCompile(`<a:lnSpc>\s*<a:spcPct\s+val="(\d+)"`)
	lstCapRegexp      = regexp.MustCompile(`<a:defRPr\b[^>]*\bcap="([a-z]+)"`)
	lstSzRegexp       = regexp.MustCompile(`<a:defRPr\b[^>]*\bsz="(\d+)"`)
	lstLatinRegexp    = regexp.MustCompile(`<a:latin\s+typeface="([^"]+)"`)
	lstAlgnRegexp     = regexp.MustCompile(`^<a:lvl1pPr\b[^>]*\balgn="([a-z]+)"`)
)

// OverrideFromListStyle returns a copy of st with any level-1 overrides found
// in a placeholder's raw <a:lstStyle> inner XML (sz, cap, lnSpc, latin)
// applied. The layout/slide lstStyle wins over the master per OOXML
// inheritance.
func (st InheritedTextStyle) OverrideFromListStyle(lstStyleInner string) InheritedTextStyle {
	if lstStyleInner == "" {
		return st
	}
	// Only consider the level-1 block when present.
	lvl1 := lstStyleInner
	if i := strings.Index(lvl1, "<a:lvl1pPr"); i >= 0 {
		lvl1 = lvl1[i:]
		if j := strings.Index(lvl1, "</a:lvl1pPr>"); j >= 0 {
			lvl1 = lvl1[:j]
		}
	}
	if m := lstLnSpcPctRegexp.FindStringSubmatch(lvl1); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && v > 0 {
			st.LineSpacingPct = v / 1000
		}
	}
	if m := lstCapRegexp.FindStringSubmatch(lvl1); m != nil {
		st.CapsAll = m[1] == "all"
	}
	if m := lstSzRegexp.FindStringSubmatch(lvl1); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && v > 0 {
			st.SizeHPt = v
		}
	}
	if m := lstLatinRegexp.FindStringSubmatch(lvl1); m != nil {
		st.Typeface = m[1]
	}
	if m := lstAlgnRegexp.FindStringSubmatch(lvl1); m != nil {
		st.Align = m[1]
	}
	return st
}

// ListStyleLvl1Align returns the algn attribute of a raw <a:lstStyle> inner
// XML's level-1 paragraph properties, or "" when it declares none.
func ListStyleLvl1Align(lstStyleInner string) string {
	i := strings.Index(lstStyleInner, "<a:lvl1pPr")
	if i < 0 {
		return ""
	}
	if m := lstAlgnRegexp.FindStringSubmatch(lstStyleInner[i:]); m != nil {
		return m[1]
	}
	return ""
}

var bodyPrAnchorRegexp = regexp.MustCompile(`<a:bodyPr\b[^>]*\banchor="([a-z]+)"`)

// ParseMasterTitleAnchor returns the bodyPr anchor of the slide master's title
// placeholder, or "" when the master declares none (go-slide-creator-pymy7).
func ParseMasterTitleAnchor(masterData []byte) string {
	for _, sp := range strings.Split(string(masterData), "<p:sp>")[1:] {
		if end := strings.Index(sp, "</p:sp>"); end >= 0 {
			sp = sp[:end]
		}
		ph := strings.Index(sp, "<p:ph ")
		if ph < 0 {
			continue
		}
		tag := sp[ph:]
		if gt := strings.Index(tag, ">"); gt >= 0 {
			tag = tag[:gt]
		}
		if !strings.Contains(tag, `type="title"`) {
			continue
		}
		if m := bodyPrAnchorRegexp.FindStringSubmatch(sp); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}
