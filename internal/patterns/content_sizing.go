package patterns

import (
	"math"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// ---------------------------------------------------------------------------
// Content sizing helpers for patterns that size their own rows from the text
// they carry (table-highlight, exec-summary, image-text-split,
// chart-insights-split). Expand stays pure: measurement uses the embedded
// font metrics of internal/textfit (deterministic across platforms) and the
// content-area estimate from the expand context — no I/O.
// ---------------------------------------------------------------------------

const (
	sizingLineSpacing = 1.2  // renderer line height factor
	sizingInsetTBPt   = 7.2  // top / bottom inset assumed by the fit detectors (conservative vs the 3.6pt OOXML default)
	sizingInsetLRPt   = 7.2  // OOXML default left / right inset (0.1")
	sizingSafetyPt    = 4.0  // slack so rounding never pushes text into autofit
	sizingBoldWidth   = 0.94 // bold glyphs run ~6% wider than the regular face
	sizingEMUPerPt    = 12700.0
)

// sizedPara is one paragraph measured for height estimation.
type sizedPara struct {
	text         string
	sizePt       float64
	bold         bool
	spaceAfterPt float64
}

var sizingTagRe = regexp.MustCompile(`<[^>]+>`)

// paragraphLines returns how many lines p wraps to in a text frame widthPt
// wide (the frame width including the default left/right insets).
func paragraphLines(ctx ExpandContext, p sizedPara, widthPt float64) int {
	text := strings.TrimSpace(sizingTagRe.ReplaceAllString(p.text, ""))
	if text == "" {
		return 0
	}
	w := widthPt
	if p.bold {
		w *= sizingBoldWidth
	}
	font := strings.TrimSpace(ctx.Theme.BodyFont)
	if font == "" {
		font = "Arial"
	}
	lines := 0
	m, err := textfit.MeasureRun(text, font, p.sizePt, int64(w*sizingEMUPerPt), 0)
	if err != nil || m.Lines <= 0 {
		// Font cache unavailable: fall back to the average-advance estimate
		// (the frame width excludes the default insets).
		lines = estimateWrappedLines(text, p.sizePt, math.Max(w-2*sizingInsetLRPt, 1))
	} else {
		lines = m.Lines
	}
	// The fit report's character-capacity model (internal/textcapacity)
	// budgets lines × "n"-width characters and flags cells above 110% of
	// that budget (counting inline markup such as <b> tags). Reserve enough
	// lines to stay inside it too, so a pattern sized here never ships a
	// fit_overflow on its own content.
	cpl := math.Floor((widthPt - 2*sizingInsetLRPt) / (sizingCapacityEm * p.sizePt))
	if cpl >= 1 {
		capLines := int(math.Ceil(float64(len([]rune(strings.TrimSpace(p.text)))) / (cpl * sizingCapacitySlack)))
		if capLines > lines {
			lines = capLines
		}
	}
	return lines
}

const (
	sizingCapacityEm    = 0.556 // advance of "n" in Liberation Sans (the capacity model's probe glyph)
	sizingCapacitySlack = 1.08  // stay a little inside the 110% overflow band
)

// sizedBlockHeightPt estimates the rendered height (points) of paras inside a
// text frame widthPt wide, including the default top/bottom insets and a small
// safety margin.
func sizedBlockHeightPt(ctx ExpandContext, paras []sizedPara, widthPt float64) float64 {
	total := 0.0
	for i, p := range paras {
		lines := paragraphLines(ctx, p, widthPt)
		if lines == 0 {
			continue
		}
		total += float64(lines) * p.sizePt * sizingLineSpacing
		if i < len(paras)-1 {
			total += p.spaceAfterPt
		}
	}
	return total + 2*sizingInsetTBPt + sizingSafetyPt
}

// Generation expands patterns before the layout's content zone is resolved,
// so without explicit layout bounds the shape-grid default bounds overstate
// the real zone (bundled templates: 824-828pt × 333-360pt vs the 864 × 389pt
// default). Sizing against a conservatively shrunk default means rows can
// only gain slack when the real zone is larger — never lose it.
const (
	sizingDefaultWidthFrac  = 0.955
	sizingDefaultHeightFrac = 0.855
)

// sizingAreaPt returns the pattern's content-area size in points: the
// caller's layout bounds when known, else a conservative estimate derived from
// the shape-grid default bounds for the slide (16:9 when the context carries
// no slide size).
func sizingAreaPt(ctx ExpandContext) (w, h float64) {
	if ctx.LayoutBounds.Width > 0 && ctx.LayoutBounds.Height > 0 {
		return float64(ctx.LayoutBounds.Width) / sizingEMUPerPt, float64(ctx.LayoutBounds.Height) / sizingEMUPerPt
	}
	sw, sh := ctx.SlideWidth, ctx.SlideHeight
	if sw <= 0 || sh <= 0 {
		sw, sh = 12192000, 6858000
	}
	db := shapegrid.DefaultBounds(sw, sh)
	return float64(db.CX) * sizingDefaultWidthFrac / sizingEMUPerPt, float64(db.CY) * sizingDefaultHeightFrac / sizingEMUPerPt
}

// pctOf converts a point extent into a percentage of total (rounded to 0.1).
func pctOf(pt, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(pt/total*1000) / 10
}

// schemeContrast returns the WCAG contrast ratio between two colours (scheme
// names or hex) resolved against the expand context's theme; ok is false when
// either colour cannot be resolved.
func schemeContrast(ctx ExpandContext, fg, bg string) (float64, bool) {
	f, fok := resolveThemeColor(ctx, fg)
	b, bok := resolveThemeColor(ctx, bg)
	if !fok || !bok {
		return 0, false
	}
	return f.ContrastWith(b), true
}

// inkOnLight returns preferred when it reads against the lt1 background at
// minRatio or better, otherwise dk1. Without a theme it trusts preferred.
func inkOnLight(ctx ExpandContext, preferred string, minRatio float64) string {
	ratio, ok := schemeContrast(ctx, preferred, "lt1")
	if !ok || ratio >= minRatio {
		return preferred
	}
	return "dk1"
}
