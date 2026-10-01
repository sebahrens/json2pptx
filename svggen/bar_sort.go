package svggen

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Item comparison ("A is larger than B") reads fastest when the bars are
// ranked, so a single-series bar chart of categories that are not periods is
// sorted descending by default; data.sort overrides (go-slide-creator-oocqj).
const (
	SortDesc = "desc"
	SortAsc  = "asc"
	SortNone = "none"
)

// ordinalCategoryRe matches category labels whose order carries meaning —
// ranges and bands with digits, rating scales and stages — which a value sort
// would scramble.
var ordinalCategoryRe = regexp.MustCompile(`(?i)(\d|^(very |extremely |somewhat |strongly )?(low|medium|mid|high|small|large|poor|fair|good|excellent|agree|disagree|neutral|never|rarely|sometimes|often|always|none|basic|advanced|beginner|intermediate|expert)$|^(stage|step|phase|tier|level|wave|round|gen|generation|version|v|series|pre-seed|seed|ipo)\b)`)

// looksOrdinal reports whether any category reads as a point on a scale.
func looksOrdinal(categories []string) bool {
	for _, c := range categories {
		if ordinalCategoryRe.MatchString(strings.TrimSpace(c)) {
			return true
		}
	}
	return false
}

// resolveSortMode reads data.sort and fills in the default: descending for a
// single-series chart whose categories are neither periods nor an ordinal
// scale, otherwise the author's order.
func resolveSortMode(data map[string]any, chart ChartData, allowDefault bool) (string, error) {
	raw, set := data["sort"]
	if set && raw != nil {
		mode, ok := raw.(string)
		mode = strings.ToLower(strings.TrimSpace(mode))
		switch {
		case ok && (mode == SortDesc || mode == SortAsc || mode == SortNone):
			return mode, nil
		case ok && mode == "":
			// fall through to the default
		default:
			return "", &ValidationError{
				Field:   "data.sort",
				Code:    ErrCodeInvalidValue,
				Message: fmt.Sprintf("data.sort must be \"desc\", \"asc\" or \"none\", got %v", raw),
				Value:   raw,
			}
		}
	}
	if !allowDefault || len(chart.Series) != 1 || len(chart.Categories) < 3 ||
		looksLikeTimeCategories(chart.Categories) || looksOrdinal(chart.Categories) {
		return SortNone, nil
	}
	return SortDesc, nil
}

// applyCategorySort reorders a chart's categories (and every series' values,
// point labels and the resolved highlight) by value: the single series, or
// the per-category total across series. The sort is stable, so ties keep the
// author's order. Highlights resolved by name or index follow their bar.
func applyCategorySort(chart *ChartData, mode string) {
	n := len(chart.Categories)
	if mode != SortDesc && mode != SortAsc || n < 2 {
		return
	}
	for _, s := range chart.Series {
		if len(s.Values) != n {
			return // misaligned payload: leave the author's order alone
		}
	}
	totals := make([]float64, n)
	for _, s := range chart.Series {
		for i, v := range s.Values {
			totals[i] += v
		}
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		if mode == SortAsc {
			return totals[order[a]] < totals[order[b]]
		}
		return totals[order[a]] > totals[order[b]]
	})
	newIndex := make([]int, n)
	for to, from := range order {
		newIndex[from] = to
	}
	cats := make([]string, n)
	for to, from := range order {
		cats[to] = chart.Categories[from]
	}
	chart.Categories = cats
	for si := range chart.Series {
		s := &chart.Series[si]
		s.Values = permuteFloats(s.Values, order)
		if len(s.labelValues) == n {
			s.labelValues = permuteFloats(s.labelValues, order)
		}
		if len(s.Labels) == n {
			labels := make([]string, n)
			for to, from := range order {
				labels[to] = s.Labels[from]
			}
			s.Labels = labels
		}
	}
	for i, h := range chart.Highlight {
		if h >= 0 && h < n {
			chart.Highlight[i] = newIndex[h]
		}
	}
}

func permuteFloats(values []float64, order []int) []float64 {
	out := make([]float64, len(order))
	for to, from := range order {
		out[to] = values[from]
	}
	return out
}

// sortBarCategories applies data.sort (and the ranked default when
// allowDefault) to a bar-family chart.
func sortBarCategories(req *RequestEnvelope, chart *ChartData, allowDefault bool) error {
	mode, err := resolveSortMode(req.Data, *chart, allowDefault)
	if err != nil {
		return err
	}
	applyCategorySort(chart, mode)
	return nil
}
