package shapegrid

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Line balancing (go-slide-creator-58dhw).
//
// A bold heading, statement or person's name that wraps with one word alone
// on its last line ("Production use is now the norm, not the / exception")
// reads as an accident. DrawingML has no balanced wrapping, so the paragraph
// gets a right margin (marR) that narrows its measure until the lines even
// out, plus slack so a renderer whose metrics differ slightly still wraps the
// same way. The line count never changes: the narrowed measure is only
// accepted when it wraps to the same number of lines with at least two words
// on the last one.
//
// The paragraphs after a balanced one state their own side margins (zero
// unless they carry one): LibreOffice otherwise keeps the heading's margin for
// them, and an image-text-split body under a balanced heading wrapped 106pt
// short of its column, onto a line more than its row was sized for
// (go-slide-creator-alcw7).

// balanceMaxWords bounds balancing to heading-length text.
const balanceMaxWords = 16

// balanceHeadingLines narrows bold paragraphs that end in a one-word line.
func balanceHeadingLines(tb *pptx.TextBody, bounds pptx.RectEmu) {
	if tb == nil || tb.Vert != "" {
		return
	}
	insets := pptx.EffectiveTextInsets(tb, bounds)
	width := bounds.CX - insets[0] - insets[2]
	balanced := false
	for i := range tb.Paragraphs {
		p := &tb.Paragraphs[i]
		p.ExplicitMargins = balanced
		params, text, ok := balanceCandidate(p, width-p.MarginL)
		if !ok {
			continue
		}
		params.FontName, _ = pptx.ParagraphFitFace(*p, tb.ThemeFonts)
		margin := balancedMargin(params, text)
		if margin <= 0 {
			continue
		}
		balanced = true
		switch p.Align {
		case "ctr":
			p.MarginL += margin / 2
			p.MarginR = margin - margin/2
		case "r":
			p.MarginL += margin
		default:
			p.MarginR = margin
		}
	}
}

// unifyCardAlignment gives a card one alignment: when centred paragraphs (a
// heading, a numeral) sit above left-aligned text in the same shape, they
// move to the left edge too. Shapes that are all centred — KPI tiles, labels,
// chevrons — and right-aligned figures are unchanged.
func unifyCardAlignment(tb *pptx.TextBody) {
	if tb == nil {
		return
	}
	firstLeft := -1
	for i, p := range tb.Paragraphs {
		if (p.Align == "l" || p.Align == "just") && paragraphHasText(p) {
			firstLeft = i
			break
		}
	}
	for i := 0; i < firstLeft; i++ {
		if tb.Paragraphs[i].Align == "ctr" {
			tb.Paragraphs[i].Align = "l"
		}
	}
}

func paragraphHasText(p pptx.Paragraph) bool {
	for _, r := range p.Runs {
		if strings.TrimSpace(r.Text) != "" {
			return true
		}
	}
	return false
}

// balanceCandidate returns measurement params for a bold, unbulleted,
// heading-length paragraph, in the Liberation Sans stand-in; the caller sets
// the face the paragraph renders in where the measurer has it.
func balanceCandidate(p *pptx.Paragraph, widthEMU int64) (textfit.StyledMeasureParams, string, bool) {
	if widthEMU <= 0 || p.Bullet != nil || p.MarginR != 0 || len(p.Runs) == 0 {
		return textfit.StyledMeasureParams{}, "", false
	}
	var text strings.Builder
	size := 0
	for _, r := range p.Runs {
		if r.FieldType != "" || r.Spacing != 0 || strings.Contains(r.Text, "\n") {
			return textfit.StyledMeasureParams{}, "", false
		}
		if strings.TrimSpace(r.Text) != "" && !r.Bold {
			return textfit.StyledMeasureParams{}, "", false
		}
		text.WriteString(r.Text)
		size = max(size, r.FontSize)
	}
	words := strings.Fields(text.String())
	if size <= 0 || len(words) < 3 || len(words) > balanceMaxWords {
		return textfit.StyledMeasureParams{}, "", false
	}
	params := textfit.StyledMeasureParams{FontName: "Liberation Sans", FontPt: float64(size) / 100, WidthEMU: widthEMU}
	return params, strings.Join(words, " "), true
}

// balancedMargin returns the right margin (EMU) that removes a one-word last
// line without adding a line (textfit.BalancedMarginEMU), measured bold.
func balancedMargin(params textfit.StyledMeasureParams, text string) int64 {
	lines := func(s string, width int64) int {
		params.Runs = []textfit.StyledRun{{Text: s, Bold: true}}
		params.WidthEMU = width
		m, err := textfit.MeasureStyledRuns(params)
		if err != nil {
			return -1
		}
		return m.Lines
	}
	return textfit.BalancedMarginEMU(lines, text, params.WidthEMU)
}
