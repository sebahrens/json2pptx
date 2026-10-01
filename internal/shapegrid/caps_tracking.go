package shapegrid

import (
	"math"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Caps-label tracking (go-slide-creator-58dhw).
//
// Uppercase labels — "BOTTOM LINE", "CASE STUDY", "WHY", axis and column
// labels — set tight read as shouting; typographic practice letter-spaces
// them. Every shape paragraph that is a short, bold all-caps label at text
// size gets +7% tracking (0.07em, OOXML a:rPr spc), provided the wider label
// still wraps to the same number of lines — so a label that barely fits its
// cell is left untouched rather than pushed onto a second line.
//
// Only bold paragraphs qualify: header, eyebrow and badge roles are set bold,
// while regular-weight caps in body cells are initialisms ("VP CS", "CFO" in
// a next-steps owner column, table cells, KPI comparators) whose word gaps
// would visibly widen next to untracked mixed-case neighbours
// (go-slide-creator-y1476).

// capsTrackingEm is the added letter-spacing, as a fraction of the font size.
const capsTrackingEm = 0.07

// capsLabelMaxRunes and capsLabelMaxHPt bound what counts as a label: short
// text at or below the lead step. Display headings keep their drawn spacing.
const (
	capsLabelMaxRunes = 40
	capsLabelMaxHPt   = 1800
)

// trackCapsLabels adds caps tracking to the label paragraphs of tb, measured
// against the text area inside bounds.
func trackCapsLabels(tb *pptx.TextBody, bounds pptx.RectEmu) {
	if tb == nil || tb.Vert != "" {
		return
	}
	insets := pptx.EffectiveTextInsets(tb, bounds)
	width := bounds.CX - insets[0] - insets[2]
	for i := range tb.Paragraphs {
		p := &tb.Paragraphs[i]
		sizeHPt, ok := capsLabelSize(p)
		if !ok {
			continue
		}
		spc := int(math.Round(float64(sizeHPt) * capsTrackingEm))
		if spc <= 0 || !trackingKeepsLines(p, sizeHPt, width-p.MarginL, spc) {
			continue
		}
		for j := range p.Runs {
			if p.Runs[j].FieldType == "" {
				p.Runs[j].Spacing = spc
			}
		}
	}
}

// capsLabelSize reports the paragraph's font size when it is a caps label:
// at least three letters, every letter uppercase, short, at text size, bold
// (a header / eyebrow / badge role), and not already letter-spaced.
func capsLabelSize(p *pptx.Paragraph) (int, bool) {
	var text strings.Builder
	size := 0
	for _, r := range p.Runs {
		if r.FieldType != "" || r.Spacing != 0 {
			return 0, false
		}
		if !r.Bold && strings.TrimSpace(r.Text) != "" {
			return 0, false
		}
		text.WriteString(r.Text)
		size = max(size, r.FontSize)
	}
	label := strings.TrimSpace(text.String())
	if size <= 0 || size > capsLabelMaxHPt || len([]rune(label)) > capsLabelMaxRunes {
		return 0, false
	}
	letters := 0
	for _, r := range label {
		if unicode.IsLetter(r) {
			if !unicode.IsUpper(r) {
				return 0, false
			}
			letters++
		}
	}
	return size, letters >= 3
}

// trackingKeepsLines reports whether the label wraps to the same number of
// lines with spc (hundredths of a point) added after every character.
func trackingKeepsLines(p *pptx.Paragraph, sizeHPt int, widthEMU int64, spc int) bool {
	if widthEMU <= 0 {
		return false
	}
	runs := make([]textfit.StyledRun, 0, len(p.Runs))
	chars := 0
	for _, r := range p.Runs {
		runs = append(runs, textfit.StyledRun{Text: r.Text, Bold: r.Bold})
		chars += len([]rune(r.Text))
	}
	params := textfit.StyledMeasureParams{Runs: runs, FontName: "Liberation Sans", FontPt: float64(sizeHPt) / 100, WidthEMU: widthEMU}
	plain, err := textfit.MeasureStyledRuns(params)
	if err != nil {
		return false
	}
	// Charging the whole label's added width against every line is
	// conservative for a multi-line label; labels are short enough to accept.
	params.WidthEMU = widthEMU - int64(chars*spc)*127
	tracked, err := textfit.MeasureStyledRuns(params)
	return err == nil && params.WidthEMU > 0 && tracked.Lines == plain.Lines
}
