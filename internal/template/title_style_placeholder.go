package template

import "github.com/sebahrens/json2pptx/internal/types"

// applyInheritedTitleText fills the measured-fit text style of a title
// placeholder (all-caps, line spacing, and the size when the font resolver
// found none) from the master titleStyle, overridden by the layout
// placeholder's own lstStyle level 1.
func applyInheritedTitleText(info *types.PlaceholderInfo, shape *shapeXML, masterFonts *MasterFontStyles) {
	var st InheritedTextStyle
	if masterFonts != nil {
		st = masterFonts.TitleText
	}
	if shape.TextBody != nil && shape.TextBody.ListStyle != nil && shape.TextBody.ListStyle.Lvl1pPr != nil {
		lvl := shape.TextBody.ListStyle.Lvl1pPr
		if lvl.LnSpc != nil && lvl.LnSpc.SpcPct != nil && lvl.LnSpc.SpcPct.Val > 0 {
			st.LineSpacingPct = lvl.LnSpc.SpcPct.Val / 1000
		}
		if lvl.DefRPr != nil {
			if lvl.DefRPr.Cap != "" {
				st.CapsAll = lvl.DefRPr.Cap == "all"
			}
			if lvl.DefRPr.Size > 0 {
				st.SizeHPt = lvl.DefRPr.Size
			}
		}
	}
	info.TextCaps = st.CapsAll
	info.LineSpacingPct = st.LineSpacingPct
	if info.FontSize == 0 {
		info.FontSize = st.SizeHPt
	}
}
