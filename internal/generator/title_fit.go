package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Title measured fit (go-slide-creator-6cjs / go-slide-creator-vjwn).
//
// Generated title placeholders inherit their font size, all-caps and line
// spacing from the slide master's titleStyle. Measuring a title without those
// (the old behaviour: a 20pt mixed-case default) concluded that a 104-char
// title "fits" a 45pt all-caps, 80%-line-spaced title band, so no fontScale was
// written and renderers squashed the lines together. Both the render-time
// autofit and the preflight / quality checks now measure titles through the
// helpers below, against the same resolved style and box.

// baseLineSpacing is textfit's single-spacing line-height multiplier.
const baseLineSpacing = 1.2

// applyInheritedStyleToParams folds an inherited text style (all-caps, line
// spacing) into textfit params. Font size and font name are resolved by the
// caller because they have their own precedence rules.
func applyInheritedStyleToParams(p *textfit.Params, st template.InheritedTextStyle) {
	if st.CapsAll {
		upper := make([]string, len(p.Paragraphs))
		for i, s := range p.Paragraphs {
			upper[i] = strings.ToUpper(s)
		}
		p.Paragraphs = upper
	}
	if st.LineSpacingPct > 0 {
		p.LineSpacing = baseLineSpacing * float64(st.LineSpacingPct) / 100.0
	}
}

// resolveStyleFontName maps a raw inherited typeface to a concrete family:
// theme major tokens (+mj-*) resolve to majorFont, minor tokens (+mn-*) to
// minorFont, explicit names pass through.
func resolveStyleFontName(typeface, majorFont, minorFont string) string {
	switch {
	case strings.HasPrefix(typeface, "+mj"):
		return majorFont
	case strings.HasPrefix(typeface, "+mn"):
		return minorFont
	default:
		return typeface
	}
}

// TitleFitInput describes a title to measure against its resolved placeholder.
type TitleFitInput struct {
	Title     string
	WidthEMU  int64
	HeightEMU int64
	// Style is the inherited title text style (size, caps, line spacing).
	Style template.InheritedTextStyle
	// FontName is the concrete font family (theme tokens already resolved).
	FontName string
}

// titleFitParams builds the textfit params for a title measurement.
func titleFitParams(in TitleFitInput) textfit.Params {
	p := textfit.Params{
		WidthEMU:    in.WidthEMU,
		HeightEMU:   in.HeightEMU,
		FontSizeHPt: in.Style.SizeHPt,
		FontName:    in.FontName,
		Paragraphs:  []string{in.Title},
		ExactWidths: true,
	}
	applyInheritedStyleToParams(&p, in.Style)
	return p
}

// MeasureTitleFit runs the measured autofit calculation for a title in its
// resolved placeholder. The result's Overflow flag is true when the title
// does not fit even at the minimum font scale and line-spacing reduction.
func MeasureTitleFit(in TitleFitInput) (textfit.FitResult, error) {
	if in.Title == "" || in.WidthEMU <= 0 || in.HeightEMU <= 0 {
		return textfit.FitResult{}, nil
	}
	return textfit.Calculate(titleFitParams(in))
}

// estimateFittingChars returns the rune length of the longest word-prefix of
// the (single-paragraph) params text that fits without overflow. Returns 0
// when even the first word overflows or measurement fails.
func estimateFittingChars(p textfit.Params) int {
	if len(p.Paragraphs) == 0 {
		return 0
	}
	words := strings.Fields(p.Paragraphs[0])
	lo, hi := 0, len(words) // invariant: words[:lo] fits (or lo==0)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		trial := p
		trial.Paragraphs = []string{strings.Join(words[:mid], " ")}
		r, err := textfit.Calculate(trial)
		if err != nil {
			return 0
		}
		if r.Overflow {
			hi = mid - 1
		} else {
			lo = mid
		}
	}
	return len([]rune(strings.Join(words[:lo], " ")))
}

// newTitleOverflowFinding builds a TITLE_OVERFLOW fit finding for a title that
// overflows at the minimum autofit size. p must be the params the overflow was
// measured with.
func newTitleOverflowFinding(path, title string, p textfit.Params, res textfit.FitResult) patterns.FitFinding {
	fontPt := float64(p.FontSizeHPt) / 100.0
	if fontPt <= 0 {
		fontPt = 20
	}
	minFontPt := fontPt
	if res.FontScale > 0 {
		minFontPt = fontPt * float64(res.FontScale) / 100000.0
	}
	current := len([]rune(title))
	maxChars := estimateFittingChars(p)
	heightEMU, _ := textfit.MeasureHeight(p)
	f := patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "placeholder",
			Path:    path,
			Code:    patterns.ErrCodeTitleOverflow,
			Message: fmt.Sprintf(
				"title (%d chars) does not fit its %.1f\"x%.1f\" title placeholder even at %.0fpt (template size %.0fpt); lines will collide or spill",
				current, float64(p.WidthEMU)/914400.0, float64(p.HeightEMU)/914400.0, minFontPt, fontPt),
			Fix: &patterns.FixSuggestion{
				Kind: "shorten_title",
				Params: map[string]any{
					"current_chars": current,
					"max_chars":     maxChars,
					"font_pt":       fontPt,
					"min_font_pt":   minFontPt,
				},
			},
		},
		Action: "shrink_or_split",
		Allowed: &patterns.Extent{
			WidthEMU:  p.WidthEMU,
			HeightEMU: p.HeightEMU,
		},
	}
	if heightEMU > 0 {
		f.Measured = &patterns.Extent{WidthEMU: p.WidthEMU, HeightEMU: heightEMU}
		f.OverflowRatio = float64(heightEMU) / float64(p.HeightEMU)
	}
	return f
}

// DetectTitleOverflow measures a title against its resolved placeholder and
// returns a TITLE_OVERFLOW finding when it cannot fit at the minimum autofit
// size, or nil when it fits (or cannot be measured).
func DetectTitleOverflow(path string, in TitleFitInput) *patterns.FitFinding {
	res, err := MeasureTitleFit(in)
	if err != nil || !res.Overflow {
		return nil
	}
	f := newTitleOverflowFinding(path, in.Title, titleFitParams(in), res)
	return &f
}

// lnSpcElemRegexp matches an existing <a:lnSpc>…</a:lnSpc> element.
var lnSpcElemRegexp = regexp.MustCompile(`<a:lnSpc>.*?</a:lnSpc>`)

// bakeTitleFit writes a computed title fit into explicit run sizes and
// paragraph line spacing instead of relying on <a:normAutofit fontScale>.
// LibreOffice ignores the stored fontScale and re-runs its own shrink, which
// compresses line spacing until title lines collide; PowerPoint honours the
// scale only until the text is edited. Baking the size makes the title render
// identically everywhere. p must be the params the result was measured with.
func bakeTitleFit(shape *shapeXML, p textfit.Params, res textfit.FitResult) {
	if shape.TextBody == nil {
		return
	}
	if res.FontScale > 0 && p.FontSizeHPt > 0 {
		for i := range shape.TextBody.Paragraphs {
			para := &shape.TextBody.Paragraphs[i]
			for j := range para.Runs {
				run := &para.Runs[j]
				if run.RunProperties == nil {
					run.RunProperties = &runPropertiesXML{Lang: "en-US"}
				} else {
					rp := *run.RunProperties // copy: rPr pointers may be shared
					run.RunProperties = &rp
				}
				base := p.FontSizeHPt
				if sz, err := strconv.Atoi(run.RunProperties.FontSize); err == nil && sz > 0 {
					base = sz
				}
				run.RunProperties.FontSize = strconv.Itoa(scaleHPt(base, res.FontScale))
			}
		}
	}
	if res.LnSpcReduction > 0 {
		basePct := 100.0
		if p.LineSpacing > 0 {
			basePct = p.LineSpacing / baseLineSpacing * 100.0
		}
		val := int(basePct*(1.0-float64(res.LnSpcReduction)/100000.0)*1000.0 + 0.5)
		lnSpc := fmt.Sprintf(`<a:lnSpc><a:spcPct val="%d"/></a:lnSpc>`, val)
		for i := range shape.TextBody.Paragraphs {
			para := &shape.TextBody.Paragraphs[i]
			props := paragraphPropertiesXML{}
			if para.Properties != nil {
				props = *para.Properties // copy: pPr pointers are shared across paragraphs
			}
			// lnSpc is the first child of CT_TextParagraphProperties.
			props.Inner = lnSpc + lnSpcElemRegexp.ReplaceAllString(props.Inner, "")
			para.Properties = &props
		}
	}
}

// scaleHPt scales a font size (hundredths of a point) by an OOXML fontScale
// (thousandths of a percent), rounding down to a half point.
func scaleHPt(hpt, fontScale int) int {
	scaled := float64(hpt) * float64(fontScale) / 100000.0
	return int(scaled/50.0) * 50
}
