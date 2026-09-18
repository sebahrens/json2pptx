package tokens

import "strings"

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
	// TextRoleCaption is short supporting text: KPI labels and deltas, axis
	// or chip captions. It shares the footnote floor in dense reports.
	TextRoleCaption TextRole = "caption"
)

// ParseViewingMode maps the deck-level viewing_mode input ("present" /
// "read", plus the internal token names) to a ViewingMode. Empty and unknown
// values default to the presentation mode: decks are projected unless the
// author says they are read on screen or in print.
func ParseViewingMode(s string) ViewingMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "read", "report", string(ViewingModeReport):
		return ViewingModeReport
	default:
		return ViewingModePresentation
	}
}

// ViewingModeInputName returns the agent-facing input name for a mode
// ("present" or "read").
func ViewingModeInputName(m ViewingMode) string {
	if m == ViewingModeReport {
		return "read"
	}
	return "present"
}

// MinReadableHPt is the policy floor, separate from geometric fit. Projected
// presentations need 12pt body copy and 10pt captions; dense reports (read on
// screen or printed) retain compact card/footnote roles. Explicit author sizes
// remain visible as violations.
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
			return 2000
		case TextRoleBody, TextRoleCardTitle, TextRoleCardBody:
			return 1200
		case TextRoleKPIValue:
			return 1800
		case TextRoleFootnote, TextRoleCaption:
			return 1000
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
	case TextRoleFootnote, TextRoleCaption:
		return FootnoteMinHPt
	default:
		return 1000
	}
}
