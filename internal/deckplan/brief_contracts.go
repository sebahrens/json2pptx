package deckplan

import (
	"fmt"
	"regexp"
	"strings"
)

var reportingPeriod = regexp.MustCompile(`(?i)\b(?:(?:Q[1-4]|H[12])\s+)?FY\d{2,4}\b|\b(?:Q[1-4]|H[12]|(?:19|20)\d{2})\b`)

func metricIsPeriod(text string, metric []int) bool {
	for _, period := range reportingPeriod.FindAllStringIndex(text, -1) {
		if metric[0] >= period[0] && metric[0] < period[1] {
			token := text[period[0]:period[1]]
			// A bare year-shaped count at the end of "Employees 2026" is
			// still a quantity. Calendar labels precede a value or follow
			// an explicit time preposition.
			if len(token) == 4 && token[0] >= '0' && token[0] <= '9' && period[0] > 0 && namedMetric.FindString(text[period[1]:]) == "" {
				before := strings.Fields(strings.ToLower(text[:period[0]]))
				if len(before) > 0 {
					switch before[len(before)-1] {
					case "in", "by", "for", "year":
					default:
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

var actionOwnerDate = regexp.MustCompile(`^(.+?)\s*\(([^,()]+),\s*([^()]+)\)$`)

func nextStepsFields(n *namedSlide) map[string]any {
	if n.body == "" {
		return nil
	}
	fields := map[string]any{"decisions": []any{sentenceCase(strings.TrimSpace(n.body))}}
	var actions []any
	for _, item := range n.items {
		if m := actionOwnerDate.FindStringSubmatch(item); m != nil && factDated.MatchString(m[3]) {
			actions = append(actions, map[string]any{"action": strings.TrimSpace(m[1]), "owner": strings.TrimSpace(m[2]), "date": strings.TrimSpace(m[3])})
		}
	}
	if len(actions) > 0 {
		fields["actions"] = actions
	}
	return fields
}

var quarterSeriesCue = regexp.MustCompile(`(?i)\b(?:by quarters?|quarterly)\b`)
var quarterSeriesTail = regexp.MustCompile(`(?i)\s+by quarters?\s*$`)
var labelledQuarterValues = regexp.MustCompile(`^([^\d:;]+?):?\s+(\d+(?:\.\d+)?(?:\s*,\s*\d+(?:\.\d+)?){1,3})\s*$`)

// Quarter labels are ordinal positions only when the brief explicitly says
// "by quarter"; unlabeled sequences do not acquire invented dates.
func labelledQuarterSeries(n *namedSlide) *seriesData {
	if !quarterSeriesCue.MatchString(n.sentence) {
		return nil
	}
	sd := &seriesData{typ: "line", name: strings.TrimSpace(namedChangeVerb.ReplaceAllString(n.header, ""))}
	for _, segment := range topLevelSplit(n.body, ";") {
		text := quarterSeriesTail.ReplaceAllString(strings.TrimSpace(segment), "")
		m := labelledQuarterValues.FindStringSubmatch(text)
		if m == nil {
			return nil
		}
		cs := chartSeries{name: strings.TrimSpace(m[1])}
		for _, number := range strings.Split(m[2], ",") {
			value, ok := parseNumber(number)
			if !ok {
				return nil
			}
			cs.values = append(cs.values, value)
		}
		if len(sd.series) > 0 && len(cs.values) != len(sd.categories) {
			return nil
		}
		if len(sd.series) == 0 {
			for i := range cs.values {
				sd.categories = append(sd.categories, fmt.Sprintf("Q%d", i+1))
			}
		}
		sd.series = append(sd.series, cs)
	}
	if len(sd.series) == 0 {
		return nil
	}
	return sd
}

var bridgeAmount = regexp.MustCompile(`(?i)([+\-−]?\d[\d,]*(?:\.\d+)?)\s*(bn|mm|m|k)?\s*$`)
var bridgeParenthetical = regexp.MustCompile(`([^(),]+?)\s*\(([+\-−]\d[\d,]*(?:\.\d+)?\s*(?:bn|mm|m|k)?)\)`)

// A fiscal-period opening/closing pair with signed drivers is an additive
// bridge even when the brief names that slide only in its separate outline.
// A lone margin change, unsigned KPI list or mixed units is not such a bridge.
func fiscalBridgeFields(n *namedSlide) map[string]any {
	var totals, deltas []any
	unit := ""
	for _, item := range n.items {
		parts := []string{item}
		if matches := bridgeParenthetical.FindAllStringSubmatch(item, -1); len(matches) > 0 {
			parts = nil
			for _, m := range matches {
				label := strings.TrimSpace(m[1])
				if i := strings.LastIndex(strings.ToLower(label), " from "); i >= 0 {
					label = label[i+6:]
				}
				label = strings.TrimPrefix(label, "and ")
				parts = append(parts, strings.TrimSpace(label)+" "+m[2])
			}
		}
		for _, part := range parts {
			m := bridgeAmount.FindStringSubmatchIndex(part)
			if m == nil {
				continue
			}
			number := part[m[2]:m[3]]
			value, ok := parseNumber(number)
			if !ok {
				continue
			}
			suffix := ""
			if m[4] >= 0 {
				suffix = strings.ToLower(part[m[4]:m[5]])
			}
			if unit == "" {
				unit = suffix
			}
			if suffix != unit {
				return nil
			}
			label := strings.TrimSpace(part[:m[0]])
			signed := strings.HasPrefix(number, "+") || strings.HasPrefix(number, "-") || strings.HasPrefix(number, "−")
			if signed {
				deltas = append(deltas, map[string]any{"label": label, "type": "delta", "value": value})
			} else if reportingPeriod.MatchString(label) {
				totals = append(totals, map[string]any{"label": label, "type": "total", "value": value})
			}
		}
	}
	if len(totals) != 2 || len(deltas) < 2 {
		return nil
	}
	columns := append([]any{totals[0]}, deltas...)
	columns = append(columns, totals[1])
	out := map[string]any{"columns": columns}
	if unit != "" {
		out["unit"] = unit
	}
	return out
}

// Fill each requested region once. A mixed slide may receive its chart and
// stat from different sentences; claiming the whole slide on the first merge
// prevented the second region from ever receiving its data.
func regionAcceptsNamed(region map[string]any, n namedSlide) bool {
	switch {
	case region["kind"] == "stat" && n.kind == "kpi_snapshot":
		return region["value"] == nil || region["value"] == "__FILL__"
	case region["kind"] == "chart" && n.kind == "chart_insight":
		chart, _ := region["chart"].(map[string]any)
		data, _ := chart["data"].(map[string]any)
		return data == nil || strings.Contains(fmt.Sprint(data["categories"]), "__FILL__")
	case region["kind"] == "table" && n.kind == "table":
		return region["rows"] == nil || strings.Contains(fmt.Sprint(region["rows"]), "__FILL__")
	}
	return false
}

func regionsAcceptNamed(fields map[string]any, n namedSlide) bool {
	regions, _ := fields["regions"].([]any)
	for _, raw := range regions {
		region, _ := raw.(map[string]any)
		if regionAcceptsNamed(region, n) {
			return true
		}
	}
	return false
}

var riskAxisRating = regexp.MustCompile(`(?i)^(.+?)\s*[—–-]\s*(likelihood|probability|impact)\s+(low|medium|med|high),\s*(likelihood|probability|impact)\s+(low|medium|med|high)$`)
var riskAxisContinuation = regexp.MustCompile(`(?i)^(?:likelihood|probability|impact)\s+(?:low|medium|med|high)$`)

func riskRatingItems(body string) []string {
	var items []string
	for _, segment := range topLevelSplit(body, ";") {
		items = append(items, splitListItems(segment)...)
	}
	var out []string
	for _, item := range items {
		if len(out) > 0 && riskAxisContinuation.MatchString(item) {
			out[len(out)-1] += ", " + item
		} else {
			out = append(out, item)
		}
	}
	return out
}

func riskAxisWordItem(item string) map[string]any {
	m := riskAxisRating.FindStringSubmatch(item)
	if m == nil {
		return nil
	}
	out := map[string]any{"name": strings.TrimSpace(m[1])}
	for i := 2; i <= 4; i += 2 {
		axis := strings.ToLower(m[i])
		if axis == "probability" {
			axis = "likelihood"
		}
		value := strings.ToLower(m[i+1])
		if value == "med" {
			value = "medium"
		}
		out[axis] = value
	}
	if out["likelihood"] == nil || out["impact"] == nil {
		return nil
	}
	return out
}

// Two independent percentage metrics can populate a requested stat pane.
// Change clauses ("up 12%") and dates are not separate measures.
func independentMetricPair(items []string) bool {
	if len(items) != 2 || datedItems(items) > 0 {
		return false
	}
	for _, item := range items {
		m := firstMetric(item)
		if m == nil || !strings.Contains(item[m[0]:m[1]], "%") {
			return false
		}
		label := strings.TrimSpace(item[:m[0]])
		if label == "" || namedChangeVerb.MatchString(label) {
			return false
		}
		switch strings.ToLower(label) {
		case "up", "down", "against", "versus", "vs":
			return false
		}
	}
	return true
}
