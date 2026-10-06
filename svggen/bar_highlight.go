package svggen

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// A chart should show the point its slide title makes, so a single-series bar
// chart paints every bar in a neutral tint and only the bar(s) the title is
// about in accent1 (go-slide-creator-sdxii). The waterfall uses the same
// inks: totals neutral 60%, decreases accent1, increases a legible accent1
// tint or shade (waterfallIncreaseInk).
const (
	// BarNeutralInk is the dk1 share of a neutral (non-highlighted) bar.
	BarNeutralInk = 0.38
	// WaterfallTotalInk is the dk1 share of a waterfall total / subtotal bar.
	WaterfallTotalInk = 0.60
	// WaterfallIncreaseShare is the accent1 share (over the background) a
	// waterfall increase bar is painted in when that tint is legible.
	WaterfallIncreaseShare = 0.55
	// waterfallIncreaseMinContrast is the least contrast an increase bar
	// keeps against the chart background (WCAG non-text 3:1).
	waterfallIncreaseMinContrast = 3.0

	// labelledBarSlotShare is a bar's width as a share of its category slot
	// (band plus gap) when every bar carries its value label.
	labelledBarSlotShare = 0.60
	// labelledBaselinePt is the value-axis baseline width in labelled mode.
	labelledBaselinePt = 0.75
	// labelledValueFontPt is the data-label size in labelled mode.
	labelledValueFontPt = 10.0
	// labelledValueGapPt is the gap between a bar end and its data label.
	labelledValueGapPt = 4.0
)

// labelledValueFont is the data-label size in labelled mode: the 10pt caption
// step, raised to the viewing floor when the chart is read in the room
// (Typography.ReadableFloor, 12pt in live-presentation mode).
func labelledValueFont(style *StyleGuide) float64 {
	if style == nil || style.Typography == nil {
		return labelledValueFontPt
	}
	return math.Max(labelledValueFontPt, style.Typography.ReadableFloor)
}

// NeutralInk returns dk1 (the palette's primary text colour) at share of full
// strength, flattened onto the palette background so it is an opaque fill.
func NeutralInk(p *Palette, share float64) Color {
	bg := p.Background
	if bg.A < 1 {
		bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
	}
	ink := p.TextPrimary
	ink.A = math.Max(0, math.Min(1, share))
	return ink.BlendOver(bg)
}

// waterfallIncreaseInk is the fill of a waterfall increase bar: a tint of
// accent1 when it keeps 3:1 against the background, otherwise a shade of it,
// and in either case at least MinSeriesDeltaE away from the decrease (accent1)
// and total (neutral) fills, so the three classes stay distinct. The old
// neutral 35% grey nearly vanished on white (go-slide-creator-rmm0x).
func waterfallIncreaseInk(p *Palette, total, decrease Color) Color {
	bg := p.Background
	if bg.A < 1 {
		bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
	}
	tint := func(share float64) Color {
		c := p.Accent1
		c.A = share
		return c.BlendOver(bg)
	}
	candidates := []Color{
		tint(WaterfallIncreaseShare),
		tint(0.7),
		p.Accent1.Darken(0.35),
		p.Accent1.Darken(0.55),
		NeutralInk(p, 0.85),
	}
	for _, c := range candidates {
		if c.ContrastWith(bg) >= waterfallIncreaseMinContrast &&
			deltaE76(c, decrease) >= MinSeriesDeltaE && deltaE76(c, total) >= MinSeriesDeltaE {
			return c
		}
	}
	for _, c := range candidates {
		if c.ContrastWith(bg) >= waterfallIncreaseMinContrast {
			return c
		}
	}
	return candidates[0]
}

// TrueMinus replaces the ASCII hyphen that marks a negative number with the
// typographic minus sign U+2212 ("−12", "−$4.5M"). A hyphen that is not
// followed by a digit, decimal point or currency sign is left alone, so unit
// words keep their hyphens.
func TrueMinus(label string) string {
	if !strings.Contains(label, "-") {
		return label
	}
	runes := []rune(label)
	var sb strings.Builder
	for i, r := range runes {
		if r == '-' && i+1 < len(runes) {
			next := runes[i+1]
			if unicode.IsDigit(next) || next == '.' || unicode.Is(unicode.Sc, next) {
				sb.WriteRune('−')
				continue
			}
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// resolveHighlight reads data.highlight — a list (or single entry) of 0-based
// category indices and/or category names — against the chart's categories.
// set reports whether the key was present at all; an explicit empty list means
// "highlight nothing". Names match exactly first, then case-insensitively.
func resolveHighlight(data map[string]any, categories []string) (indices []int, set bool, err error) {
	raw, ok := data["highlight"]
	if !ok || raw == nil {
		return nil, false, nil
	}
	entries, isList := toAnySlice(raw)
	if !isList {
		entries = []any{raw}
	}
	seen := make(map[int]bool, len(entries))
	indices = []int{}
	for _, e := range entries {
		idx, ok := highlightIndex(e, categories)
		if !ok {
			return nil, true, &ValidationError{
				Field:   "data.highlight",
				Code:    ErrCodeInvalidValue,
				Message: fmt.Sprintf("highlight entry %v matches no category: use a 0-based index below %d or one of the category names", e, len(categories)),
				Value:   e,
			}
		}
		if !seen[idx] {
			seen[idx] = true
			indices = append(indices, idx)
		}
	}
	return indices, true, nil
}

func highlightIndex(e any, categories []string) (int, bool) {
	if s, ok := e.(string); ok {
		for i, c := range categories {
			if c == s {
				return i, true
			}
		}
		want := strings.ToLower(strings.TrimSpace(s))
		for i, c := range categories {
			if strings.ToLower(strings.TrimSpace(c)) == want {
				return i, true
			}
		}
		return 0, false
	}
	v, ok := toFloat64Value(e)
	if !ok || v != math.Trunc(v) || v < 0 || int(v) >= len(categories) {
		return 0, false
	}
	return int(v), true
}

// timeCategoryRe matches category labels that name a period: years (2026,
// FY26, 2026E, '24), quarters and halves (Q1, Q1 2025, H2 FY25), months and
// ISO dates.
var timeCategoryRe = regexp.MustCompile(`(?i)^(` +
	`'\d{2}|(19|20)\d{2}[a-z]?|fy\s?'?\d{2,4}[a-z]?|cy\s?\d{2,4}|` +
	`(q[1-4]|h[12])(\s*(fy\s?)?'?\d{2,4})?|` +
	`(19|20)\d{2}\s*(q[1-4]|h[12])|` +
	`(jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\.?(\s*'?\d{2,4})?|` +
	`(19|20)\d{2}[-/]\d{1,2}([-/]\d{1,2})?|` +
	`(week|wk|w|month|m|year|yr|day|d)\s?\d{1,3}` +
	`)$`)

// looksLikeTimeCategories reports whether every category names a period, so
// the bars read as a time series whose latest value is the point.
func looksLikeTimeCategories(categories []string) bool {
	if len(categories) < 2 {
		return false
	}
	for _, c := range categories {
		if !timeCategoryRe.MatchString(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

// defaultHighlight picks the bar a single-series chart accents when the author
// named none: the last bar of a time series (the latest period), otherwise the
// top — largest — bar of a ranked or categorical chart.
func defaultHighlight(categories []string, values []float64) []int {
	n := len(values)
	if len(categories) < n {
		n = len(categories)
	}
	if n == 0 {
		return nil
	}
	if looksLikeTimeCategories(categories[:n]) {
		return []int{n - 1}
	}
	best := 0
	for i := 1; i < n; i++ {
		if values[i] > values[best] {
			best = i
		}
	}
	return []int{best}
}

// barHighlightColors returns the per-bar fills and bold flags for a
// single-series bar chart: neutral dk1 at BarNeutralInk, accent on the
// highlighted indices.
func barHighlightColors(p *Palette, accent Color, n int, highlight []int) ([]Color, []bool) {
	neutral := NeutralInk(p, BarNeutralInk)
	colors := make([]Color, n)
	bold := make([]bool, n)
	for i := range colors {
		colors[i] = neutral
	}
	for _, i := range highlight {
		if i >= 0 && i < n {
			colors[i] = accent
			bold[i] = true
		}
	}
	return colors, bold
}
