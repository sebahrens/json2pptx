package generator

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Title line balancing (go-slide-creator-58dhw).
//
// A title that wraps with one word alone on its last line reads as an
// accident. After the measured fit is baked, a single-paragraph title with
// such a widow gets a right margin (marR) that narrows its measure until the
// lines even out; the line count and the fitted size never change
// (textfit.BalancedMarginEMU). Centred titles split the margin across both
// sides so they stay centred. Titles whose alignment cannot be resolved, or
// that are right-aligned or justified, are left alone.

// balanceTitleLines applies the balancing margin to a fitted title.
func balanceTitleLines(shape *shapeXML, params textfit.Params, result textfit.FitResult, cfg *autofitConfig) {
	if result.Overflow || shape.TextBody == nil || len(params.Paragraphs) != 1 || len(shape.TextBody.Paragraphs) != 1 {
		return
	}
	para := &shape.TextBody.Paragraphs[0]
	align := ""
	if para.Properties != nil {
		align = para.Properties.Algn
	}
	if align == "" {
		if cfg.inherited == nil {
			return // alignment unknown: a one-sided margin could shift a centred title
		}
		align = cfg.inherited.Align
	}
	if align != "" && align != "l" && align != "ctr" {
		return
	}
	sizeHPt := params.FontSizeHPt
	if result.FontScale > 0 {
		sizeHPt = scaleHPt(sizeHPt, result.FontScale)
	}
	if sizeHPt <= 0 {
		return
	}
	lines := func(s string, width int64) int {
		m, err := textfit.MeasureRun(s, params.FontName, float64(sizeHPt)/100, width, 0)
		if err != nil || (m.FontSubstituted && !arialMetric(params.FontName)) {
			return -1 // without the title's own metrics a margin could add a line
		}
		return m.Lines
	}
	margin := int(textfit.BalancedMarginEMU(lines, params.Paragraphs[0], params.WidthEMU))
	if margin <= 0 {
		return
	}
	props := paragraphPropertiesXML{}
	if para.Properties != nil {
		props = *para.Properties // copy: pPr pointers may be shared
	}
	right := margin
	if align == "ctr" {
		left := margin / 2
		if props.MarL != nil {
			left += *props.MarL
		}
		props.MarL = &left
		right = margin - margin/2
	}
	props.MarR = &right
	para.Properties = &props
}

// arialMetric reports fonts the Liberation Sans fallback measures faithfully.
func arialMetric(font string) bool {
	switch strings.ToLower(strings.TrimSpace(font)) {
	case "arial", "helvetica", "liberation sans", "arimo":
		return true
	}
	return false
}

var layoutTitleSpRegexp = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
var layoutTitlePhRegexp = regexp.MustCompile(`<p:ph\b[^>]*\btype="(?:title|ctrTitle)"`)
var layoutLstStyleRegexp = regexp.MustCompile(`(?s)<a:lstStyle>(.*?)</a:lstStyle>`)

// layoutTitleAlign returns the level-1 alignment the layout's title
// placeholder declares in its own lstStyle, or "" when it declares none.
func (ctx *singlePassContext) layoutTitleAlign(layoutID string) string {
	data, err := ctx.readFileWithSyntheticFallback(LayoutPath(layoutID))
	if err != nil {
		return ""
	}
	for _, sp := range layoutTitleSpRegexp.FindAllString(string(data), -1) {
		if !layoutTitlePhRegexp.MatchString(sp) {
			continue
		}
		if m := layoutLstStyleRegexp.FindStringSubmatch(sp); m != nil {
			return template.ListStyleLvl1Align(m[1])
		}
		return ""
	}
	return ""
}
