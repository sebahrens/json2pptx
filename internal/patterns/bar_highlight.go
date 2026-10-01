package patterns

import (
	"fmt"
	"math"
	"strings"
)

// Bar charts show the point their slide title makes (go-slide-creator-sdxii):
// every bar is a neutral dk1 tint and only the highlighted bar(s) take the
// accent. These are the pattern-side counterparts of svggen's BarNeutralInk /
// WaterfallTotalInk / WaterfallIncreaseInk.

// barNeutralTone is a non-highlighted bar: dk1 at 38%.
var barNeutralTone = fillTone{Color: "dk1", LumMod: 38000, LumOff: 62000}

// highlightAccent is the fill of a highlighted bar: an explicit accent or
// semantic_accent override when the author gave one, otherwise the template's
// primary fill (ExpandContext.DefaultAccent, accent1 on most templates) — not
// the deck's rotated accent, because the highlight is the one emphasis on the
// slide and must read the same on every chart.
func highlightAccent(ctx ExpandContext, accent, semanticAccent string) string {
	if accent == "" && semanticAccent == "" {
		return ctx.DefaultAccent()
	}
	return ctx.ResolveAccent(accent, semanticAccent)
}

// highlightSchema describes a values.highlight list: 0-based indices or labels.
func highlightSchema(desc string) *Schema {
	return ArraySchema(OneOfSchema(IntegerSchema(0, 99), StringSchema(0)), 0, 0).WithDescription(desc)
}

// resolveBarHighlight maps a highlight list (0-based indices and/or labels,
// matched exactly then case-insensitively) onto bar indices. An entry that
// matches no bar is an error.
func resolveBarHighlight(entries []any, labels []string) (map[int]bool, error) {
	out := make(map[int]bool, len(entries))
	for _, e := range entries {
		idx, ok := barHighlightIndex(e, labels)
		if !ok {
			return nil, fmt.Errorf("highlight entry %v matches no bar: use a 0-based index below %d or one of the bar labels", e, len(labels))
		}
		out[idx] = true
	}
	return out, nil
}

func barHighlightIndex(e any, labels []string) (int, bool) {
	switch v := e.(type) {
	case string:
		for i, l := range labels {
			if l == v {
				return i, true
			}
		}
		want := strings.ToLower(strings.TrimSpace(v))
		for i, l := range labels {
			if strings.ToLower(strings.TrimSpace(l)) == want {
				return i, true
			}
		}
	case float64:
		if v == math.Trunc(v) && v >= 0 && int(v) < len(labels) {
			return int(v), true
		}
	case int:
		if v >= 0 && v < len(labels) {
			return v, true
		}
	}
	return 0, false
}

// topBarIndex is the default highlight of a ranked chart: its largest bar
// (the first one on a tie).
func topBarIndex(values []float64) int {
	best := 0
	for i := 1; i < len(values); i++ {
		if values[i] > values[best] {
			best = i
		}
	}
	return best
}
