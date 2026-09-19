package svggen

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Number formatting for data labels (go-slide-creator-66qb).
//
// Every value label went through fmt.Sprintf with a hardcoded "%.0f". On a
// slide titled "ARR grew eight straight quarters to EUR 86.4M", whose insight
// bullet said "from EUR 66.0M to EUR 86.4M", the chart printed 61 64 65 66 71
// 75 81 86 — it contradicted the narrative beside it. A seven-bar series
// [4.6 … 6.5] printed three distinct labels for seven bars, flatly denying its
// own axis and its own "+6% a year" headline. Only the waterfall auto-detected
// fractions; bar, line, area, funnel and gauge did not.
//
// Two rules, both applied only when the caller left the format at its default:
// show enough decimals for the labels to be DISTINCT, and group thousands so
// 12400 reads 12,400.

// defaultValueFormat is the printf verb every chart config starts with. It is
// the sentinel for "the caller did not choose a format", which is what lets the
// auto-detection below run without overriding an explicit choice.
const defaultValueFormat = "%.0f"

// maxAutoDecimals caps the auto-detected precision. Beyond two decimals a data
// label stops being readable at chart size.
const maxAutoDecimals = 2

// autoValueFormat returns the format to use for a set of values: the caller's
// format when they chose one, otherwise a precision that keeps the labels
// distinct.
//
// "Distinct" is the test that matters. [4.6, 4.9, 5.2, 5.5, 5.8, 6.2, 6.5] at
// %.0f collapses to three labels across seven bars; at %.1f all seven differ.
func autoValueFormat(format string, values []float64) string {
	if format != "" && format != defaultValueFormat {
		return format
	}
	if !anyFractional(values) {
		return defaultValueFormat
	}
	for decimals := 1; decimals <= maxAutoDecimals; decimals++ {
		f := "%." + strconv.Itoa(decimals) + "f"
		if labelsAreDistinct(values, f) {
			return f
		}
	}
	return "%." + strconv.Itoa(maxAutoDecimals) + "f"
}

// anyFractional reports whether any value has a fractional part worth showing.
func anyFractional(values []float64) bool {
	for _, v := range values {
		if math.Abs(v-math.Trunc(v)) > 1e-9 {
			return true
		}
	}
	return false
}

// labelsAreDistinct reports whether formatting every value at the given format
// keeps values that differ looking different.
func labelsAreDistinct(values []float64, format string) bool {
	seen := make(map[string]float64, len(values))
	for _, v := range values {
		label := fmt.Sprintf(format, v)
		if prev, ok := seen[label]; ok && math.Abs(prev-v) > 1e-9 {
			return false
		}
		seen[label] = v
	}
	return true
}

// formatValueGrouped formats a value and groups its integer part in threes, so
// a funnel reads "12,400" rather than "12400" and a bar reads "1,240" rather
// than "1240". Values under a thousand are returned unchanged, and so is any
// formatted string whose integer part is already three digits or fewer — a
// caller-supplied format that carries its own separators is left alone by
// groupThousands.
func formatValueGrouped(value float64, format string) string {
	s := formatValue(value, format)
	if math.Abs(value) < 1000 {
		return s
	}
	return groupThousands(s)
}

// groupThousands inserts commas into the first run of digits in a formatted
// number, leaving any prefix, sign, decimals and suffix untouched — so
// "€12400M" becomes "€12,400M" and "12400.5" becomes "12,400.5". A run of three
// digits or fewer is left alone, which also means a caller-supplied format that
// already carries its own separators is not touched.
func groupThousands(s string) string {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	end := start
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	intPart := s[start:end]
	if len(intPart) <= 3 {
		return s
	}
	var b strings.Builder
	b.WriteString(s[:start])
	for i := 0; i < len(intPart); i++ {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(intPart[i])
	}
	b.WriteString(s[end:])
	return b.String()
}

// chartDataValues flattens every series value in a ChartData, which is what the
// precision auto-detect measures.
func chartDataValues(data ChartData) []float64 {
	var out []float64
	for _, s := range data.Series {
		out = append(out, s.Values...)
	}
	return out
}
