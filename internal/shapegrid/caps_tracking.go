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
// against the text area inside bounds — the shape's text rectangle.
//
// The lines are measured in the face the label renders in where the measurer
// has it (pptx.ParagraphFitFace), as the fit decisions are: the Liberation
// Sans stand-in runs wider than Calibri, so it tracked a word it had judged
// too wide for its box untracked (both measures broke it, "the same number
// of lines") and left a label with room untracked (go-slide-creator-217cd).
func trackCapsLabels(tb *pptx.TextBody, bounds pptx.RectEmu) {
	for _, l := range capsLabels(tb, bounds) {
		if !l.keeps {
			continue
		}
		p := &tb.Paragraphs[l.para]
		for j := range p.Runs {
			if p.Runs[j].FieldType == "" {
				p.Runs[j].Spacing = l.spc
			}
		}
	}
}

// capsLabel is one caps-label paragraph of a body: its size, the tracking it
// would take and whether it keeps its lines with that tracking.
type capsLabel struct {
	para    int
	sizeHPt int
	spc     int
	keeps   bool
}

// capsLabels lists the caps-label paragraphs of tb in the text rectangle
// bounds.
func capsLabels(tb *pptx.TextBody, bounds pptx.RectEmu) []capsLabel {
	if tb == nil || tb.Vert != "" {
		return nil
	}
	insets := pptx.EffectiveTextInsets(tb, bounds)
	// The clamp's room for renderer variance is not room for tracking
	// (go-slide-creator-v74wv).
	width := int64(float64(bounds.CX-insets[0]-insets[2]) / pptx.WordFitSlackFor(tb))
	var out []capsLabel
	for i := range tb.Paragraphs {
		p := &tb.Paragraphs[i]
		sizeHPt, ok := capsLabelSize(p)
		if !ok {
			continue
		}
		spc := int(math.Round(float64(sizeHPt) * capsTrackingEm))
		if spc <= 0 {
			continue
		}
		face, _ := pptx.ParagraphFitFace(*p, tb.ThemeFonts)
		out = append(out, capsLabel{para: i, sizeHPt: sizeHPt, spc: spc, keeps: trackingKeepsLines(p, face, sizeHPt, width-p.MarginL, spc)})
	}
	return out
}

// Peer labels are tracked together (go-slide-creator-5uqfo).
//
// Tracking was decided a shape at a time, so of process-grid-2row's two row
// labels "PRODUCTION" was letter-spaced and "DESIGN PROCESS" was not, and the
// pair read as two styles. Caps labels of one size in one grid are peers —
// column headers, row labels, step names: when one of them has no room for
// tracking, none of that size is tracked.

// shareCapsTracking marks the shape cells whose caps labels must stay
// untracked because a peer of the same size cannot be tracked. It runs once
// on the final result, like writeSharedShrink.
func shareCapsTracking(cells []ResolvedCell) {
	bySize := map[int][]int{}
	blocked := map[int]bool{}
	for i := range cells {
		c := &cells[i]
		if c.Kind != CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 || c.ShapeSpec.NoCapsTracking {
			continue
		}
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb == nil {
			continue
		}
		for j := 0; j < 4; j++ {
			tb.Insets[j] += c.TextInsets[j]
		}
		tb.ThemeFonts = c.ShapeSpec.ThemeFonts
		w, h := pptx.PresetTextRect(c.ShapeSpec.Geometry, c.ShapeSpec.Adjustments, c.Bounds)
		for _, l := range capsLabels(tb, pptx.RectEmu{CX: w, CY: h}) {
			bySize[l.sizeHPt] = append(bySize[l.sizeHPt], i)
			if !l.keeps {
				blocked[l.sizeHPt] = true
			}
		}
	}
	for size := range blocked {
		for _, i := range bySize[size] {
			if cells[i].ShapeSpec.NoCapsTracking {
				continue
			}
			spec := *cells[i].ShapeSpec
			spec.NoCapsTracking = true
			cells[i].ShapeSpec = &spec
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
// lines with spc (hundredths of a point) added after every character,
// measured in face: the face the label renders in where the measurer has it
// (pptx.ParagraphFitFace), else the Liberation Sans stand-in.
func trackingKeepsLines(p *pptx.Paragraph, face string, sizeHPt int, widthEMU int64, spc int) bool {
	if widthEMU <= 0 {
		return false
	}
	runs := make([]textfit.StyledRun, 0, len(p.Runs))
	chars := 0
	for _, r := range p.Runs {
		runs = append(runs, textfit.StyledRun{Text: r.Text, Bold: r.Bold})
		chars += len([]rune(r.Text))
	}
	params := textfit.StyledMeasureParams{Runs: runs, FontName: face, FontPt: float64(sizeHPt) / 100, WidthEMU: widthEMU}
	plain, err := textfit.MeasureStyledRuns(params)
	if err != nil {
		return false
	}
	// Charging the whole label's added width against every line is
	// conservative for a multi-line label; labels are short enough to accept.
	params.WidthEMU = widthEMU - int64(chars*spc)*127
	if params.WidthEMU <= 0 {
		return false
	}
	tracked, err := textfit.MeasureStyledRuns(params)
	if err != nil || tracked.Lines != plain.Lines {
		return false
	}
	// A word that does not hold a line is broken with or without tracking —
	// "the same number of lines" — and tracking only widens the break
	// (go-slide-creator-69ums): every word must hold the tracked line.
	var text strings.Builder
	bold := false
	for _, r := range p.Runs {
		text.WriteString(r.Text)
		bold = bold || r.Bold
	}
	for _, word := range strings.Fields(text.String()) {
		params.Runs = []textfit.StyledRun{{Text: word, Bold: bold}}
		if m, err := textfit.MeasureStyledRuns(params); err != nil || m.Lines > 1 {
			return false
		}
	}
	return true
}
