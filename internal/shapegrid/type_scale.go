package shapegrid

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
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
	paras := scaleParagraphs(tb)
	if len(paras) == 0 {
		return spec
	}
	width, height := scaledTextRect(bounds, tb.Insets, overlay)
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
}

func scaleParagraphs(tb *pptx.TextBody) []scaleParagraph {
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
		paras = append(paras, scaleParagraph{
			text: text.String(), fontPt: fontPt,
			spaceAfter: float64(p.SpaceAfter) / 100,
			marginL:    p.MarginL,
			role:       inferScaleRole(text.String(), fontPt, bold, len(tb.Paragraphs), hasKPI),
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
	case "comfortable":
		return 0.70, map[string]float64{"body": 16, "caption": 13, "kpi-value": 42}
	case "presentation":
		return 0.80, map[string]float64{"body": 18, "caption": 14, "kpi-value": 48}
	default:
		return 0, nil
	}
}

func scaledTextRect(bounds pptx.RectEmu, authored, overlay [4]int64) (int64, int64) {
	insets := authored
	for i := range insets {
		insets[i] += overlay[i]
	}
	// A bodyPr with no lIns/rIns/tIns/bIns uses OOXML defaults: 0.1in
	// horizontally and 0.05in vertically. Once any inset is explicit, the
	// renderer emits all four values, including zeros.
	if insets == [4]int64{} {
		insets = [4]int64{91440, 45720, 91440, 45720}
	}
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
		m, err := textfit.MeasureRun(p.text, "Liberation Sans", pt, usableWidth, 0)
		if err != nil {
			return false
		}
		height += float64(m.Lines)*pt*1.2 + p.spaceAfter
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
