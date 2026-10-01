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
			if lvl.DefRPr.Bold != "" {
				st.Bold = xmlBoolAttr(lvl.DefRPr.Bold)
			}
			if lvl.DefRPr.Spc != "" {
				st.SpcHPt = xmlIntAttr(lvl.DefRPr.Spc)
			}
		}
	}
	info.Anchor = resolveTitleAnchor(shape, masterFonts)
	info.TextInsetLeftEMU = resolveTitleLeftInset(shape, masterFonts)
	info.TextCaps = st.CapsAll
	info.TextBold = st.Bold
	info.CharSpacingHPt = st.SpcHPt
	info.LineSpacingPct = st.LineSpacingPct
	info.SpcBefPt = st.SpcBefPt
	if info.FontSize == 0 {
		info.FontSize = st.SizeHPt
	}
}

// resolveTitleAnchor returns the effective vertical anchor of a title
// placeholder: the layout shape's own bodyPr anchor, else the master title
// placeholder's, else the OOXML default "t". It returns "" when the master
// could not be resolved, so callers never guess an anchor.
func resolveTitleAnchor(shape *shapeXML, masterFonts *MasterFontStyles) string {
	if shape.TextBody != nil && shape.TextBody.BodyPr != nil && shape.TextBody.BodyPr.Anchor != "" {
		return shape.TextBody.BodyPr.Anchor
	}
	if masterFonts == nil {
		return ""
	}
	if masterFonts.TitleAnchor != "" {
		return masterFonts.TitleAnchor
	}
	return "t"
}

// defaultTextInsetLeftEMU is the OOXML bodyPr lIns default (0.1in).
const defaultTextInsetLeftEMU = 91440

// resolveTitleLeftInset returns the effective left text inset of a title
// placeholder: the layout shape's own bodyPr lIns, else the master title's,
// else the OOXML default 91440 EMU.
func resolveTitleLeftInset(shape *shapeXML, masterFonts *MasterFontStyles) int64 {
	if shape.TextBody != nil && shape.TextBody.BodyPr != nil && shape.TextBody.BodyPr.LIns != "" {
		return int64(xmlIntAttr(shape.TextBody.BodyPr.LIns))
	}
	if masterFonts != nil && masterFonts.TitleLeftInset != "" {
		return int64(xmlIntAttr(masterFonts.TitleLeftInset))
	}
	return defaultTextInsetLeftEMU
}

// applyInheritedBodyText fills the measured-fit text style of a body / content
// placeholder from the master bodyStyle, overridden by the layout
// placeholder's own lstStyle level 1.
//
// The space-before is the part that matters: the renderer budgets it per
// paragraph when the shape carries no explicit size, and the preflight has to
// budget the same or it predicts a font scale the render never applies
// (go-slide-creator-nlrg).
func applyInheritedBodyText(info *types.PlaceholderInfo, shape *shapeXML, masterFonts *MasterFontStyles) {
	var st InheritedTextStyle
	if masterFonts != nil {
		st = masterFonts.BodyText
	}
	if shape.TextBody != nil && shape.TextBody.ListStyle != nil && shape.TextBody.ListStyle.Lvl1pPr != nil {
		lvl := shape.TextBody.ListStyle.Lvl1pPr
		if lvl.LnSpc != nil && lvl.LnSpc.SpcPct != nil && lvl.LnSpc.SpcPct.Val > 0 {
			st.LineSpacingPct = lvl.LnSpc.SpcPct.Val / 1000
		}
		if lvl.DefRPr != nil && lvl.DefRPr.Size > 0 {
			st.SizeHPt = lvl.DefRPr.Size
		}
	}
	info.LineSpacingPct = st.LineSpacingPct
	info.SpcBefPt = st.SpcBefPt
	if info.FontSize == 0 {
		info.FontSize = st.SizeHPt
	}
}
