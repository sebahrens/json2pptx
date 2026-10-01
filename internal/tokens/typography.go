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

// Type scale (go-slide-creator-30471). Pattern and shape text sits on five
// text steps — title/display 28pt, lead 18pt, subhead 14pt, body 12pt,
// caption 10pt — plus a 40–48pt display step for KPI values. The role ladders
// in tokens.go (GridHeader*, CardTitle*, CardBody*, Footnote*) are ranges on
// this scale; the readability floors above are unchanged by it.
const (
	TypeScaleDisplayHPt = 2800 // titles and display headings
	TypeScaleLeadHPt    = 1800 // lead / section labels
	TypeScaleSubheadHPt = 1400 // subheads, card titles (bold)
	TypeScaleBodyHPt    = 1200 // body copy
	TypeScaleCaptionHPt = 1000 // captions, legends, dense table cells
	TypeScaleKPIMinHPt  = 4000 // KPI display figures, lower bound
	TypeScaleKPIMaxHPt  = 4800 // KPI display figures, upper bound

	// BodyTextMinHPt is the body minimum; DenseBodyTextMinHPt is allowed only
	// in tables and dense matrices. Neither loosens MinReadableHPt.
	BodyTextMinHPt      = 1100
	DenseBodyTextMinHPt = 1000

	// SourceLineHPt is the engine-rendered slide source line (9pt italic,
	// muted). It is chrome, not an authoring size.
	SourceLineHPt = 900
)

// Point mirrors of the type-scale steps, for pattern code that sizes text in
// points (go-slide-creator-vmdfm). Name a size by its step instead of a
// numeric literal.
const (
	TypeScaleDisplayPt = TypeScaleDisplayHPt / 100.0
	TypeScaleLeadPt    = TypeScaleLeadHPt / 100.0
	TypeScaleSubheadPt = TypeScaleSubheadHPt / 100.0
	TypeScaleBodyPt    = TypeScaleBodyHPt / 100.0
	TypeScaleCaptionPt = TypeScaleCaptionHPt / 100.0
	TypeScaleKPIMinPt  = TypeScaleKPIMinHPt / 100.0
	TypeScaleKPIMaxPt  = TypeScaleKPIMaxHPt / 100.0
	BodyTextMinPt      = BodyTextMinHPt / 100.0
	SourceLinePt       = SourceLineHPt / 100.0
)

// LineHeight is the single-spaced line pitch, as a multiple of the font
// size, that every text measurement assumes: PowerPoint's 100% line spacing
// renders a line about 1.2 em tall. Fit estimators, capacity budgets and the
// renderer's height checks share it so they agree on how tall a line is.
const LineHeight = 1.2

// OnTypeScaleHPt reports whether hpt is a size of the type scale: one of the
// text steps SnapTextHPt settles onto (10/11/12/14/18/28pt) or a size in the
// 40-48pt KPI display step.
func OnTypeScaleHPt(hpt int) bool {
	if hpt >= TypeScaleKPIMinHPt && hpt <= TypeScaleKPIMaxHPt {
		return true
	}
	for _, step := range typeScaleSteps {
		if step == hpt {
			return true
		}
	}
	return false
}

// typeScaleSteps are the sizes SnapTextHPt settles text onto, ascending. The
// 11pt body minimum is a step so dense body copy is never pushed to caption.
var typeScaleSteps = []int{TypeScaleCaptionHPt, BodyTextMinHPt, TypeScaleBodyHPt, TypeScaleSubheadHPt, TypeScaleLeadHPt, TypeScaleDisplayHPt}

// SnapTextHPt returns the largest type-scale step at or below hpt. Sizes below
// the caption step (footnotes) and at or above the display step (display
// headings and figures, which are measured to fit) are returned unchanged.
// Snapping only ever shrinks, so text that fit before still fits.
func SnapTextHPt(hpt int) int {
	if hpt < TypeScaleCaptionHPt || hpt >= TypeScaleDisplayHPt {
		return hpt
	}
	snapped := hpt
	for _, step := range typeScaleSteps {
		if step <= hpt {
			snapped = step
		}
	}
	return snapped
}
