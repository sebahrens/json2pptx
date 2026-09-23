package svggen

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
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

// ---------------------------------------------------------------------------
// One formatter per chart (go-slide-creator-e2ck9).
//
// The axis and the data labels used to be formatted by different code with
// different rules. On [1240, 865, 413] the axis printed "1000 / 1200 / 1400"
// while the bars printed "1,240 / 865 / 413"; past 9999 the axis switched to
// compact notation ("1.2M") while the labels kept grouping digits. Inside one
// chart, two number formats.
//
// A ValueFormatter is built once per chart from its own values and used by the
// ticks, the data labels and any in-mark label. Each side's old rule survives as
// the SHARED rule: compact once the numbers are large, grouped digits with
// enough decimals to stay distinct otherwise. A ValueFormatSpec from the caller
// replaces the lot, uniformly — which is what lets an EUR deck get "€1.2M" on
// both the axis and the bars.
// ---------------------------------------------------------------------------

// compactThreshold is the magnitude at which the shared default switches to
// compact notation. It is the axis's long-standing rule (anything over 9999
// makes a four-character tick label), now applied to labels too.
const compactThreshold = 9999.0

// ValueFormatter renders a chart's numbers. A nil *ValueFormatter formats with
// the legacy per-site behaviour, so a renderer that has not been handed one
// still works.
type ValueFormatter struct {
	// printf is the verb used for plain / percent / currency styles.
	printf string
	// compact renders 1.2K / 3.4M rather than grouped digits.
	compact bool
	// group inserts thousands separators into the integer part.
	group bool
	// prefix and suffix wrap the formatted number.
	prefix, suffix string
	// percent appends a % sign after the number (before suffix).
	percent bool
	// scale is applied before formatting. Fractional percent inputs use 100;
	// other styles and already-scaled percentages use 1.
	scale float64
	// compactDecimals is the decimal places for compact notation; -1 means the
	// compact default (one place, trailing ".0" trimmed).
	compactDecimals int
	// decimalsFixed records that the precision was chosen by the caller, so the
	// axis must not raise it to match its tick step.
	decimalsFixed bool
}

// WithDecimals returns a formatter showing exactly n decimal places, unless the
// caller fixed the precision. The axis uses it to keep the precision its tick
// STEP needs rather than the precision the data has: a domain of [0, 1] ticks
// every 0.2 and must print "0.2", while a whole-numbered domain must print "1"
// and not "1.0" just because some data point is fractional. Notation — grouping,
// compact, prefix, suffix — stays shared, which is what the ticks and the labels
// disagreed about (go-slide-creator-e2ck9).
func (f *ValueFormatter) WithDecimals(n int) *ValueFormatter {
	if f == nil || f.decimalsFixed || f.compact || n < 0 {
		return f
	}
	if printfDecimals(f.printf) == n {
		return f
	}
	out := *f
	out.printf = "%." + strconv.Itoa(n) + "f"
	return &out
}

// printfDecimals reads the precision out of a "%.Nf" verb, returning 0 for
// anything it does not recognise.
func printfDecimals(format string) int {
	i := strings.Index(format, ".")
	if i < 0 || i+1 >= len(format) {
		return 0
	}
	digits := 0
	for j := i + 1; j < len(format) && format[j] >= '0' && format[j] <= '9'; j++ {
		digits = digits*10 + int(format[j]-'0')
	}
	return digits
}

// NewValueFormatter resolves a formatter for a chart.
//
// spec is the caller's value_format, if any. values are the chart's own numbers,
// which the default rules measure. legacy is the printf verb already on the
// chart config — a caller-supplied format string (data_labels.format, e.g.
// "€%.1fM") still wins over the auto-detection, because it was already the
// documented way to format labels.
// allowCompact says whether the DEFAULT may switch to compact notation once the
// numbers are large. It is true for charts with a value axis, whose ticks have
// always compacted past 9999 — the labels now follow so one chart reads one way.
// It is false for funnel / gauge / treemap, which have no axis to agree with and
// whose labels are not short of room. An explicit value_format style wins either
// way.
func NewValueFormatter(spec *ValueFormatSpec, values []float64, legacy string, allowCompact bool) *ValueFormatter {
	if !spec.IsZero() {
		return valueFormatterFromSpec(spec, values)
	}
	// No spec: the shared default. A caller-supplied printf verb governs the
	// digits; the magnitude of the data decides compact vs grouped.
	f := &ValueFormatter{printf: autoValueFormat(legacy, values), compactDecimals: -1}
	if explicitFormat(legacy) {
		// An explicit format carries its own units and precision. Group its
		// digits (formatValueGrouped already did) but never rewrite it as
		// compact — "€%.1fM" is already compact, in the caller's own words.
		f.group = true
		f.decimalsFixed = true
		return f
	}
	if allowCompact && maxMagnitude(values) > compactThreshold {
		f.compact = true
		return f
	}
	f.group = true
	return f
}

// valueFormatterFromSpec builds the formatter an explicit value_format asks for.
func valueFormatterFromSpec(spec *ValueFormatSpec, values []float64) *ValueFormatter {
	f := &ValueFormatter{prefix: spec.Prefix, suffix: spec.Suffix, compactDecimals: -1, scale: 1}
	formatValues := values
	switch strings.ToLower(strings.TrimSpace(spec.Style)) {
	case "compact":
		f.compact = true
	case "percent":
		f.percent = true
		if maxMagnitude(values) <= 1+1e-9 {
			f.scale = 100
			formatValues = scaledValues(values, f.scale)
		}
	case "currency":
		f.group = true
		if f.prefix == "" {
			f.prefix = "¤"
		}
	default: // "", "plain"
		f.group = true
	}
	decimals := -1
	if spec.Decimals != nil {
		decimals = *spec.Decimals
		if decimals < 0 {
			decimals = 0
		}
		if decimals > maxSpecDecimals {
			decimals = maxSpecDecimals
		}
	}
	switch {
	case decimals >= 0:
		f.printf = "%." + strconv.Itoa(decimals) + "f"
	case f.percent, f.group:
		// Pick a precision that keeps the labels distinct, as the default does.
		f.printf = autoValueFormat(defaultValueFormat, formatValues)
	default:
		f.printf = defaultValueFormat
	}
	if f.compact && spec.Decimals != nil {
		f.compactDecimals = decimals
	}
	f.decimalsFixed = spec.Decimals != nil
	if spec.ThousandsSep != nil {
		f.group = *spec.ThousandsSep
	}
	return f
}

// maxSpecDecimals caps an explicit decimals request. Past three places a chart
// label is noise at chart size.
const maxSpecDecimals = 3

// explicitFormat reports whether a printf verb was chosen by the caller rather
// than left at the config default.
func explicitFormat(format string) bool {
	return format != "" && format != defaultValueFormat
}

// maxMagnitude returns the largest absolute value in a set.
func maxMagnitude(values []float64) float64 {
	out := 0.0
	for _, v := range values {
		if a := math.Abs(v); a > out {
			out = a
		}
	}
	return out
}

// Format renders one value.
func (f *ValueFormatter) Format(v float64) string {
	if f == nil {
		return formatValueGrouped(v, defaultValueFormat)
	}
	if f.scale != 0 {
		v *= f.scale
	}
	var body string
	switch {
	case f.compact:
		body = formatCompactDecimals(v, f.compactDecimals)
	default:
		body = formatValue(v, f.printf)
		if f.group {
			body = groupThousands(body)
		}
	}
	if f.percent {
		body += "%"
	}
	return f.prefix + body + f.suffix
}

func scaledValues(values []float64, scale float64) []float64 {
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = value * scale
	}
	return out
}

// valueFormatFindings reports conventions that the renderer can preserve but
// cannot infer with confidence. It deliberately does not refuse rendering:
// fractional percentages and a default currency symbol still produce useful
// output, while the finding makes the assumption visible to callers.
func valueFormatFindings(req *RequestEnvelope) []Finding {
	spec := req.Style.ValueFormat
	if spec == nil {
		return nil
	}
	style := strings.ToLower(strings.TrimSpace(spec.Style))
	switch style {
	case "percent":
		// Pie/donut percentages are computed from each slice's share, not
		// interpreted from the raw source values. Their scale is unambiguous.
		if req.Type == "pie_chart" || req.Type == "donut_chart" {
			return nil
		}
		values := requestFormattedValues(req.Data)
		if maxMagnitude(values) > 1+1e-9 {
			return []Finding{{
				Field:    "style.value_format",
				Code:     FindingPercentScaleAmbiguous,
				Message:  "percent values above 1 were preserved as already-scaled percentages; use fractions in [0,1] or plain formatting when the scale is not percentage points",
				Severity: core.SeverityWarning,
				Fix: &FixSuggestion{Kind: FixKindExplicitScale, Params: map[string]any{
					"style": "percent", "fractional_range": "0..1",
				}},
			}}
		}
	case "currency":
		if strings.TrimSpace(spec.Prefix) == "" {
			return []Finding{{
				Field:    "style.value_format.prefix",
				Code:     FindingCurrencyPrefixDefaulted,
				Message:  "currency format omitted prefix; defaulted to the generic \"¤\" marker — set prefix explicitly for the intended currency",
				Severity: core.SeverityWarning,
				Fix: &FixSuggestion{Kind: FixKindReplaceValue, Params: map[string]any{
					"field": "style.value_format.prefix", "defaulted_to": "¤",
				}},
			}}
		}
	}
	return nil
}

// requestFormattedValues extracts numeric leaves named value/values. Those are
// the fields consumed by chart value formatters; unrelated numeric layout data
// such as width, height, x and y must not influence scale inference.
func requestFormattedValues(data map[string]any) []float64 {
	var out []float64
	var walk func(any, bool)
	walk = func(value any, collect bool) {
		if collect {
			var appended bool
			out, appended = appendFormattedNumbers(out, value)
			if appended {
				return
			}
		}
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				walk(child, key == "value" || key == "values")
			}
		case []any:
			for _, child := range typed {
				walk(child, collect)
			}
		case []map[string]any:
			for _, child := range typed {
				walk(child, collect)
			}
		}
	}
	walk(data, false)
	return out
}

func appendFormattedNumbers(out []float64, value any) ([]float64, bool) {
	switch typed := value.(type) {
	case []float64:
		return append(out, typed...), true
	case []int:
		for _, child := range typed {
			out = append(out, float64(child))
		}
		return out, true
	case []int64:
		for _, child := range typed {
			out = append(out, float64(child))
		}
		return out, true
	case float64:
		return append(out, typed), true
	case float32:
		return append(out, float64(typed)), true
	case int:
		return append(out, float64(typed)), true
	case int64:
		return append(out, float64(typed)), true
	default:
		return out, false
	}
}

// FormatOr renders one value, falling back to the legacy per-site formatting
// when no formatter was threaded through. It is what the label sites call, so
// a renderer that has not been updated keeps its old output exactly.
func (f *ValueFormatter) FormatOr(v float64, legacy string) string {
	if f == nil {
		return formatValueGrouped(v, legacy)
	}
	return f.Format(v)
}

// ResolveValueFormatter sets c.ValueFmt from the caller's spec, the config's
// printf verb and the chart's own values. It is idempotent, so a chart that
// draws in several passes resolves the format once.
func (c *ChartConfig) ResolveValueFormatter(values []float64, allowCompact bool) *ValueFormatter {
	if c == nil {
		return nil
	}
	if c.ValueFmt == nil {
		c.ValueFmt = NewValueFormatter(c.ValueFormatSpec, values, c.ValueFormat, allowCompact)
	}
	return c.ValueFmt
}
