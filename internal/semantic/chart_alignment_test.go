package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// chartSpecYAML builds a one-chart DeckSpec around a chart.data literal.
func chartSpecYAML(data string) string {
	return `meta:
  title: Chart check
  template: midnight-blue
slides:
  - kind: title
    title: Chart check
  - kind: chart_insight
    title: Quarterly revenue
    takeaway: Revenue grew through the year.
    insight: One line of interpretation.
    chart:
      type: bar
      data:
` + data
}

// chartFindings validates a spec and returns its diagnostics.
func chartFindings(t *testing.T, data string) []diagnostics.Diagnostic {
	t.Helper()
	spec, parseDiags := ParseYAML([]byte(chartSpecYAML(data)))
	if len(parseDiags) > 0 {
		t.Fatalf("parse spec: %v", parseDiags)
	}
	return Validate(spec, StrictnessWarn)
}

// findCode returns the first diagnostic with the given code, or nil.
func findCode(ds []diagnostics.Diagnostic, code diagnostics.Code) *diagnostics.Diagnostic {
	if found := findingsWithCode(ds, code); len(found) > 0 {
		return &found[0]
	}
	return nil
}

// TestChartSeriesLengthMismatchIsAnError: a truncated values array is the most
// likely hand-authoring slip and used to validate clean, then render one bar
// over four ticks (go-slide-creator-pcrp).
func TestChartSeriesLengthMismatchIsAnError(t *testing.T) {
	ds := chartFindings(t, `        categories: [Q1, Q2, Q3, Q4]
        series:
          - name: Revenue
            values: [10]
`)
	d := findCode(ds, diagnostics.CodeChartSeriesLengthMismatch)
	if d == nil {
		t.Fatalf("expected CHART_SERIES_LENGTH_MISMATCH, got %v", codesOf(ds))
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("severity = %q, want error — svggen refuses this at render time", d.Severity)
	}
	if d.Path != "slides[1].chart.data.series[0].values" {
		t.Errorf("path = %q, want the series' values array", d.Path)
	}
	for _, want := range []string{"4 categories", `"Revenue"`, "1 value"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not mention %q", d.Message, want)
		}
	}
}

// TestChartValueNotNumericIsAnError: a quoted figure renders an empty plot with
// a format-error label.
func TestChartValueNotNumericIsAnError(t *testing.T) {
	ds := chartFindings(t, `        categories: [A, B]
        series:
          - name: Revenue
            values: ["1", "two"]
`)
	d := findCode(ds, diagnostics.CodeChartValueNotNumeric)
	if d == nil {
		t.Fatalf("expected CHART_VALUE_NOT_NUMERIC, got %v", codesOf(ds))
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("severity = %q, want error", d.Severity)
	}
	if d.Path != "slides[1].chart.data.series[0].values[0]" {
		t.Errorf("path = %q, want the offending element", d.Path)
	}
	// The type complaint wins: a series of strings has no length worth
	// comparing.
	if findCode(ds, diagnostics.CodeChartSeriesLengthMismatch) != nil {
		t.Error("a non-numeric series should not also report a length mismatch")
	}
}

// TestChartNullValueIsReported: nulls used to be coerced to 0 without a word.
func TestChartNullValueIsReported(t *testing.T) {
	ds := chartFindings(t, `        categories: [A, B]
        series:
          - name: Revenue
            values: [1, null]
`)
	d := findCode(ds, diagnostics.CodeChartValueNotNumeric)
	if d == nil {
		t.Fatalf("expected CHART_VALUE_NOT_NUMERIC for a null, got %v", codesOf(ds))
	}
	if !strings.Contains(d.Message, "null") {
		t.Errorf("message %q should say the value is null", d.Message)
	}
}

// TestAlignedChartValidatesClean: the check must not fire on correct data, and
// must not fire when the chart declares no categories at all (the shape check
// owns that case).
func TestAlignedChartValidatesClean(t *testing.T) {
	cases := map[string]string{
		"aligned": `        categories: [Q1, Q2, Q3]
        series:
          - name: Revenue
            values: [10, 12, 15]
          - name: Cost
            values: [4, 5, 6]
`,
		"floats and negatives": `        categories: [A, B]
        series:
          - name: Delta
            values: [-1.5, 2.25]
`,
		"labels alias": `        labels: [A, B]
        series:
          - name: Revenue
            values: [1, 2]
`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			ds := chartFindings(t, data)
			if d := findCode(ds, diagnostics.CodeChartSeriesLengthMismatch); d != nil {
				t.Errorf("correct data reported a mismatch: %s", d.Message)
			}
			if d := findCode(ds, diagnostics.CodeChartValueNotNumeric); d != nil {
				t.Errorf("correct data reported a non-numeric value: %s", d.Message)
			}
		})
	}
}

// codesOf lists the codes of a diagnostic set for failure messages.
func codesOf(ds []diagnostics.Diagnostic) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
}

// TestMisshapenChartListNamesTheKey is go-slide-creator-xp1x: a series given as
// a map produced "declares no data series (found keys "categories", "series")",
// which contradicted itself and pointed the fix at the whole data block.
func TestMisshapenChartListNamesTheKey(t *testing.T) {
	ds := chartFindings(t, `        categories: [a, b]
        series:
          Rev: [1, 2]
`)
	d := findCode(ds, diagnostics.CodeSemanticFieldType)
	if d == nil {
		t.Fatalf("expected SEMANTIC_FIELD_TYPE, got %v", codesOf(ds))
	}
	if d.Path != "slides[1].chart.data.series" {
		t.Errorf("path = %q, want the series key itself", d.Path)
	}
	for _, want := range []string{"is an object", `"Rev"`, "array of objects", `"values"`} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not mention %q", d.Message, want)
		}
	}
	// The self-contradicting message must be gone.
	if other := findCode(ds, diagnostics.CodeSemanticDensity); other != nil &&
		strings.Contains(other.Message, "declares no data series") {
		t.Errorf("the generic message still fires alongside: %s", other.Message)
	}
}

// TestMissingChartSeriesKeepsTheGenericMessage: when the key is absent there is
// nothing to retype, and the original guidance is right.
func TestMissingChartSeriesKeepsTheGenericMessage(t *testing.T) {
	ds := chartFindings(t, `        categories: [a, b]
        totals: [1, 2]
`)
	d := findCode(ds, diagnostics.CodeSemanticDensity)
	if d == nil {
		t.Fatalf("expected SEMANTIC_DENSITY, got %v", codesOf(ds))
	}
	if !strings.Contains(d.Message, "declares no data series") {
		t.Errorf("message = %q, want the missing-series guidance", d.Message)
	}
}
