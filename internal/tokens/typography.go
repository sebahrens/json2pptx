package tokens

type ViewingMode string
type TextRole string

const (
	ViewingModeReport       ViewingMode = "dense-report"
	ViewingModePresentation ViewingMode = "live-presentation"

	TextRoleTitle     TextRole = "title"
	TextRoleBody      TextRole = "body"
	TextRoleCardTitle TextRole = "card-title"
	TextRoleCardBody  TextRole = "card-body"
	TextRoleKPIValue  TextRole = "kpi-value"
	TextRoleFootnote  TextRole = "footnote"
)

// MinReadableHPt is the policy floor, separate from geometric fit. Dense
// reports retain compact card/footnote roles; projected presentations require
// larger title/body copy. Explicit author sizes remain visible as violations.
func MinReadableHPt(mode ViewingMode, role TextRole) int {
	if mode == "" {
		mode = ViewingModeReport
	}
	if role == "" {
		role = TextRoleBody
	}
	if mode == ViewingModePresentation {
		switch role {
		case TextRoleTitle:
			return 2800
		case TextRoleBody:
			return 1400
		case TextRoleCardTitle:
			return 1400
		case TextRoleCardBody:
			return 1200
		case TextRoleKPIValue:
			return 2000
		case TextRoleFootnote:
			return 800
		}
	}
	switch role {
	case TextRoleTitle:
		return 2000
	case TextRoleBody:
		return 1000
	case TextRoleCardTitle:
		return CardTitleMinHPt
	case TextRoleCardBody:
		return CardBodyMinHPt
	case TextRoleKPIValue:
		return 1800
	case TextRoleFootnote:
		return FootnoteMinHPt
	default:
		return 1000
	}
}
