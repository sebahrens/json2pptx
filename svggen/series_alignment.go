package svggen

import (
	"fmt"
	"strings"
)

// Series / category alignment (go-slide-creator-pcrp).
//
// {categories: [Q1,Q2,Q3,Q4], series: [{name: Rev, values: [10]}]} used to
// validate clean and render one bar over four ticks — a confidently wrong
// chart from the single most likely hand-authoring slip, a truncated array
// after an edit. A string among the numbers did the same: the plot came back
// empty with a format-error label and every gate said the deck was fine.
//
// Both are now refused where every other chart-shape error is refused, so the
// deterministic check an agent is told to run before rendering can see them.

// maxListedSeriesIssues caps how many per-series problems one message names, so
// a chart with twelve broken series produces a readable error rather than a
// wall of text.
const maxListedSeriesIssues = 3

// seriesName returns a series' name for messages, falling back to its index.
func seriesName(s map[string]any, idx int) string {
	if n, ok := s["name"].(string); ok && strings.TrimSpace(n) != "" {
		return fmt.Sprintf("%q", n)
	}
	return fmt.Sprintf("series[%d]", idx)
}

// validateSeriesValues checks every series' values against the category count:
// each value must be a number, and (when categoryCount > 0) there must be
// exactly one per category. A categoryCount of 0 skips the length check for
// charts whose x-axis is not categorical (scatter, bubble, time series).
func validateSeriesValues(seriesSlice []map[string]any, categoryCount int, chartType string) error {
	var lengthIssues, typeIssues []string
	for i, s := range seriesSlice {
		raw, has := s["values"]
		if !has {
			continue // the per-chart validators own the "values is required" case
		}
		nums, ok := toFloat64Slice(raw)
		if !ok {
			typeIssues = append(typeIssues, fmt.Sprintf("%s: %s", seriesName(s, i), describeNonNumeric(raw)))
			continue
		}
		if categoryCount > 0 && len(nums) != categoryCount {
			lengthIssues = append(lengthIssues, fmt.Sprintf("%s has %d value(s)", seriesName(s, i), len(nums)))
		}
	}

	// A wrong type is reported first: a series whose values are strings has no
	// meaningful length to compare.
	if len(typeIssues) > 0 {
		return &ValidationError{
			Field: "data.series[].values",
			Code:  ErrCodeInvalidType,
			Message: fmt.Sprintf("%s 'values' must be numbers — %s. Quote-wrapped numbers are not accepted: write [10, 20, 30], not [\"10\", \"20\"]",
				chartType, joinIssues(typeIssues)),
		}
	}
	if len(lengthIssues) > 0 {
		return &ValidationError{
			Field: "data.series[].values",
			Code:  ErrCodeConstraint,
			Message: fmt.Sprintf("%s has %d categories but %s. Every series needs exactly one value per category — pad the short series with 0 or drop the extra categories",
				chartType, categoryCount, joinIssues(lengthIssues)),
		}
	}
	return nil
}

// describeNonNumeric names the first offending element of a values array, so
// the author sees which entry to fix rather than the whole array.
func describeNonNumeric(raw any) string {
	list, ok := raw.([]any)
	if !ok {
		return fmt.Sprintf("values is %T, want an array of numbers", raw)
	}
	for i, item := range list {
		switch item.(type) {
		case float64, int, int64:
			continue
		case nil:
			return fmt.Sprintf("values[%d] is null", i)
		default:
			return fmt.Sprintf("values[%d] is %#v", i, item)
		}
	}
	return "values contains a non-numeric entry"
}

// joinIssues renders up to maxListedSeriesIssues issues, summarising the rest.
func joinIssues(issues []string) string {
	if len(issues) <= maxListedSeriesIssues {
		return strings.Join(issues, "; ")
	}
	shown := strings.Join(issues[:maxListedSeriesIssues], "; ")
	return fmt.Sprintf("%s (and %d more)", shown, len(issues)-maxListedSeriesIssues)
}

// categoryCountOf returns how many categories data declares, or 0 when it
// declares none.
func categoryCountOf(data map[string]any) int {
	cats, ok := data["categories"]
	if !ok {
		return 0
	}
	catSlice, ok := toStringSlice(cats)
	if !ok {
		return 0
	}
	return len(catSlice)
}
