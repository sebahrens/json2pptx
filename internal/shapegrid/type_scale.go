package shapegrid

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// growShapeText writes measured sizes back to a private copy of the resolved
// shape. Both OOXML generation and preflight consume that copy, so they cannot
// disagree about the font that occupies the cell. Authored input is untouched.
func growShapeText(spec *ShapeSpec, bounds pptx.RectEmu, overlay [4]int64, mode string) *ShapeSpec {
	if spec == nil || len(spec.Text) == 0 {
		return spec
	}
	tb, err := ResolveTextInput(spec.Text)
	if err != nil || len(tb.Paragraphs) == 0 {
		return spec
	}
	paras := scaleParagraphs(tb, spec.ThemeFonts)
	if len(paras) == 0 {
		return spec
	}
	width, height := scaledTextRect(bounds, tb, overlay)
	if width <= 0 || height <= 0 {
		return spec
	}
	target, caps := typeScalePolicy(mode)
	if target == 0 {
		return spec
	}
	maxScale := math.Inf(1)
	for i := range paras {
		cap := caps[paras[i].role]
		if ratio := cap / paras[i].fontPt; ratio < maxScale {
			maxScale = ratio
		}
	}
	maxScale = math.Min(maxScale, tokenGrowthCap(paras, width))
	if maxScale <= 1.01 {
		return spec
	}
	availablePt := float64(height) / 12700
	if !scaleBlockFits(paras, width, availablePt*target, 1) {
		return spec
	}
	lo, hi := 1.0, maxScale
	if scaleBlockFits(paras, width, availablePt*target, maxScale) {
		lo = maxScale
	} else {
		for i := 0; i < 12; i++ {
			mid := (lo + hi) / 2
			if scaleBlockFits(paras, width, availablePt*target, mid) {
				lo = mid
			} else {
				hi = mid
			}
		}
	}
	// OOXML sizes are hundredths of a point. Rounding downward keeps the
	// measured <=80% fit promise even at a wrap boundary.
	if lo < 1.02 {
		return spec
	}
	text, ok := writeScaledText(spec.Text, paras, lo)
	if !ok {
		return spec
	}
	copySpec := *spec
	copySpec.Text = text
	return &copySpec
}

type scaleParagraph struct {
	text       string
	fontPt     float64
	spaceAfter float64
	marginL    int64
	role       string
	bold       bool
	// face is the face the paragraph's fit is measured in; exact reports
	// that it is the face the paragraph renders in (pptx.ParagraphFitFace)
	// rather than the Liberation Sans stand-in.
	face  string
	exact bool
}

// exactTokenShare is the share of its line a token that must not break may
// take when it is measured in the face it renders in: the line less the
// rounding a renderer's own layout may differ by. A stand-in face keeps
// textfit.AtomicTokenWidthPct (go-slide-creator-5x4w4).
const exactTokenShare = 0.98

// tokenShare is the share of its line a must-not-break token may take.
func (p scaleParagraph) tokenShare() float64 {
	if p.exact {
		return exactTokenShare
	}
	return float64(textfit.AtomicTokenWidthPct) / 100
}

// tokenWidth measures tok at pt in the paragraph's face and weight.
func (p scaleParagraph) tokenWidth(tok string, pt float64) (float64, bool) {
	w, err := textfit.MeasureStyledLineWidth(tok, p.face, pt, p.bold)
	return float64(w), err == nil
}

// tokenGrowthCap is the largest scale at which every token that fits its line
// at the pattern's size still fits: a word, or a whole KPI value ("12 days").
// Growth only ever fills spare height, so it must not be what breaks a label
// mid-word ("PRODUCTI / ON") or a value from its unit ("$4.2" / "M") — the
// pattern measured those whole and growth measures by line count alone. The
// token keeps textfit.AtomicTokenWidthPct of the width where the grower
// measures a stand-in face, not the template font (go-slide-creator-b7qqg.14 /
// .15), and exactTokenShare where it measures the template's own. A token
// between that share and the full line keeps its size; tokens already wider
// than their line at scale 1 do not cap growth.
func tokenGrowthCap(paras []scaleParagraph, width int64) float64 {
	limit := math.Inf(1)
	for _, p := range paras {
		line := float64(width - p.marginL)
		allowed := line * p.tokenShare()
		if allowed <= 0 {
			continue
		}
		tokens := strings.Fields(p.text)
		if p.role == "kpi-value" {
			tokens = []string{strings.TrimSpace(p.text)}
		}
		for _, tok := range tokens {
			w, ok := p.tokenWidth(tok, p.fontPt)
			if !ok || w <= 0 || w > line {
				continue
			}
			limit = math.Min(limit, math.Max(allowed, w)/w)
		}
	}
	return limit
}

// scaleParagraphs lists a body's paragraphs for a fit decision. fonts are the
// theme typefaces the shape's text was sized in (ShapeSpec.ThemeFonts); zero
// for an authored grid, whose paragraphs measure in the stand-in face.
func scaleParagraphs(tb *pptx.TextBody, fonts pptx.ThemeFonts) []scaleParagraph {
	paras := make([]scaleParagraph, 0, len(tb.Paragraphs))
	hasKPI := false
	for _, p := range tb.Paragraphs {
		for _, run := range p.Runs {
			if float64(run.FontSize)/100 >= 28 && strings.IndexFunc(run.Text, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
				hasKPI = true
			}
		}
	}
	for _, p := range tb.Paragraphs {
		var text strings.Builder
		fontPt := 0.0
		bold := false
		for _, run := range p.Runs {
			text.WriteString(run.Text)
			if pt := float64(run.FontSize) / 100; pt > fontPt {
				fontPt = pt
			}
			bold = bold || run.Bold
		}
		if strings.TrimSpace(text.String()) == "" || fontPt <= 0 {
			return nil
		}
		face, exact := pptx.ParagraphFitFace(p, fonts)
		paras = append(paras, scaleParagraph{
			face: face, exact: exact,
			text: text.String(), fontPt: fontPt,
			spaceAfter: float64(p.SpaceAfter) / 100,
			marginL:    p.MarginL,
			role:       inferScaleRole(text.String(), fontPt, bold, len(tb.Paragraphs), hasKPI),
			bold:       bold,
		})
	}
	return paras
}

func inferScaleRole(text string, fontPt float64, bold bool, paragraphCount int, hasKPI bool) string {
	if fontPt >= 28 && len([]rune(text)) <= 25 && strings.IndexFunc(text, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
		return "kpi-value"
	}
	if (paragraphCount == 1 || hasKPI) && !bold && len([]rune(text)) <= 40 {
		return "caption"
	}
	return "body"
}

func typeScalePolicy(mode string) (float64, map[string]float64) {
	switch mode {
	// Caps sit on the type scale (tokens.TypeScale*): grown text settles onto
	// a step through snapShapeTextToScale, so a cap between steps only adds
	// growth that the snap would take back.
	case "comfortable":
		return 0.70, map[string]float64{"body": 14, "caption": 12, "kpi-value": 40}
	case "presentation":
		return 0.80, map[string]float64{"body": 18, "caption": 14, "kpi-value": 48}
	default:
		return 0, nil
	}
}

// scaledTextRect is the text area the writer leaves inside bounds: the body's
// insets plus any icon-overlay reservation, clamped exactly as the writer
// clamps a degenerate shape (pptx.EffectiveTextInsets).
func scaledTextRect(bounds pptx.RectEmu, tb *pptx.TextBody, overlay [4]int64) (int64, int64) {
	body := *tb
	for i := range body.Insets {
		body.Insets[i] += overlay[i]
	}
	insets := pptx.EffectiveTextInsets(&body, bounds)
	return bounds.CX - insets[0] - insets[2], bounds.CY - insets[1] - insets[3]
}

func scaleBlockFits(paras []scaleParagraph, width int64, targetHeightPt, scale float64) bool {
	height := 0.0
	for _, p := range paras {
		usableWidth := width - p.marginL
		if usableWidth <= 0 {
			return false
		}
		pt := p.fontPt * scale
		m, err := textfit.MeasureRun(p.text, p.face, pt, usableWidth, 0)
		if err != nil {
			return false
		}
		height += float64(m.Lines)*pt*textLineHeightFactor + p.spaceAfter
		if height > targetHeightPt {
			return false
		}
	}
	return true
}

func writeScaledText(raw json.RawMessage, paras []scaleParagraph, scale float64) (json.RawMessage, bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		out, err := json.Marshal(map[string]any{"content": text, "size": scaledSize(paras[0].fontPt, scale)})
		return out, err == nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, false
	}
	if rawParas, ok := obj["paragraphs"]; ok {
		var defs []map[string]json.RawMessage
		if json.Unmarshal(rawParas, &defs) != nil || len(defs) != len(paras) {
			return nil, false
		}
		for i := range defs {
			defs[i]["size"], _ = json.Marshal(scaledSize(paras[i].fontPt, scale))
			// A suffix run (a unit beside its figure) keeps its proportion.
			var suffixSize float64
			if json.Unmarshal(defs[i]["suffix_size"], &suffixSize) == nil && suffixSize > 0 {
				defs[i]["suffix_size"], _ = json.Marshal(scaledSize(suffixSize, scale))
			}
		}
		obj["paragraphs"], _ = json.Marshal(defs)
	} else {
		obj["size"], _ = json.Marshal(scaledSize(paras[0].fontPt, scale))
	}
	out, err := json.Marshal(obj)
	return out, err == nil
}

func scaledSize(fontPt, scale float64) float64 {
	return math.Floor(fontPt*scale*100+1e-8) / 100
}

// snapShapeTextToScale settles every sized paragraph of a resolved shape onto
// the type scale (go-slide-creator-30471): off-scale pattern sizes such as
// 13/15/16/17/20/22pt become the step at or below them (12/14/14/14/18/18pt).
// Display figures — a short run containing a digit at 18pt or more, such as a
// step numeral, or a paragraph its pattern marks "figure" at FigureMinSizePt
// or more, such as a KPI value — keep their measured size, as does text under
// the caption step (footnotes) or at the display step and above. Snapping only
// shrinks, so fit and readability floors (which sit on scale steps) hold.
// Like growShapeText it rewrites a private copy consumed by both OOXML
// generation and preflight; the authored input is untouched.
func snapShapeTextToScale(spec *ShapeSpec) *ShapeSpec {
	if spec == nil || len(spec.Text) == 0 {
		return spec
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(spec.Text, &obj) != nil {
		return spec // string shorthand renders at the 14pt default, a scale step
	}
	changed := false
	snap := func(size float64, marked bool, texts ...string) (float64, bool) {
		if size <= 0 || keepsFigureSize(size, marked) {
			return size, false
		}
		for _, t := range texts {
			if size >= 18 && isDisplayFigure(t) {
				return size, false
			}
		}
		hpt := int(math.Round(size * 100))
		snapped := tokens.SnapTextHPt(hpt)
		if snapped == hpt {
			return size, false
		}
		return float64(snapped) / 100, true
	}
	if rawParas, ok := obj["paragraphs"]; ok {
		var defs []map[string]json.RawMessage
		if json.Unmarshal(rawParas, &defs) != nil {
			return spec
		}
		for i := range defs {
			var size float64
			var content string
			var marked bool
			_ = json.Unmarshal(defs[i]["size"], &size)
			_ = json.Unmarshal(defs[i]["content"], &content)
			_ = json.Unmarshal(defs[i]["figure"], &marked)
			if s, ok := snap(size, marked, content); ok {
				defs[i]["size"], _ = json.Marshal(s)
				changed = true
			}
		}
		if !changed {
			return spec
		}
		obj["paragraphs"], _ = json.Marshal(defs)
	} else {
		var size float64
		var content string
		_ = json.Unmarshal(obj["size"], &size)
		_ = json.Unmarshal(obj["content"], &content)
		s, ok := snap(size, false, strings.Split(content, "\n")...)
		if !ok {
			return spec
		}
		obj["size"], _ = json.Marshal(s)
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return spec
	}
	copySpec := *spec
	copySpec.Text = out
	return &copySpec
}

// FigureMinSizePt is the smallest size a paragraph marked "figure" keeps as
// measured: the floor a KPI value is fitted down to. Below it the mark is
// ignored and the text settles onto the scale like any other.
const FigureMinSizePt = 16.0

// keepsFigureSize reports whether a paragraph its pattern marked as a display
// figure is written at its measured size.
func keepsFigureSize(size float64, marked bool) bool {
	return marked && size >= FigureMinSizePt
}

// isDisplayFigure reports a short run that states a figure or numeral.
func isDisplayFigure(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && len([]rune(text)) <= 25 && strings.IndexFunc(text, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
}
