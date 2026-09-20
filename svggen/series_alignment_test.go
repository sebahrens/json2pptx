package svggen

import (
	"strings"
	"testing"
)

// renderErrFor validates a chart request and returns the error text.
func renderErrFor(t *testing.T, chartType string, data map[string]any) string {
	t.Helper()
	d := DefaultRegistry().Get(chartType)
	if d == nil {
		t.Fatalf("no diagram registered for %q", chartType)
	}
	err := d.Validate(&RequestEnvelope{Type: chartType, Data: data})
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestSeriesLengthMismatchIsRefused: a short series used to render as a
// missing bar with every gate reporting success (go-slide-creator-pcrp).
func TestSeriesLengthMismatchIsRefused(t *testing.T) {
	for _, chartType := range []string{"bar_chart", "line_chart", "area_chart", "stacked_bar_chart", "stacked_area_chart"} {
		t.Run(chartType, func(t *testing.T) {
			got := renderErrFor(t, chartType, map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series":     []any{map[string]any{"name": "Revenue", "values": []any{10.0}}},
			})
			if got == "" {
				t.Fatal("a 1-value series against 4 categories was accepted")
			}
			for _, want := range []string{"4 categories", `"Revenue"`, "1 value"} {
				if !strings.Contains(got, want) {
					t.Errorf("error %q does not mention %q", got, want)
				}
			}
		})
	}
}

// TestNonNumericValuesAreRefused: a quoted figure rendered an empty plot with
// a format-error label.
func TestNonNumericValuesAreRefused(t *testing.T) {
	got := renderErrFor(t, "bar_chart", map[string]any{
		"categories": []any{"A", "B"},
		"series":     []any{map[string]any{"name": "Revenue", "values": []any{"1", "two"}}},
	})
	if got == "" {
		t.Fatal("string values were accepted")
	}
	if !strings.Contains(got, "must be numbers") || !strings.Contains(got, "values[0]") {
		t.Errorf("error %q should name the field and the offending index", got)
	}
}

// TestAlignedSeriesPass: the check must not fire on correct data.
func TestAlignedSeriesPass(t *testing.T) {
	if got := renderErrFor(t, "bar_chart", map[string]any{
		"categories": []any{"Q1", "Q2"},
		"series": []any{
			map[string]any{"name": "Revenue", "values": []any{10.0, 12.0}},
			map[string]any{"name": "Cost", "values": []any{4.0, 5.0}},
		},
	}); got != "" {
		t.Errorf("aligned data was refused: %s", got)
	}
}

// TestPointChartsAreExempt: scatter and bubble carry point objects in the same
// `values` field. They are neither numbers nor one-per-category, and refusing
// them would break every scatter chart in the corpus.
func TestPointChartsAreExempt(t *testing.T) {
	cases := map[string][]any{
		"scatter_chart": {map[string]any{"x": 1.0, "y": 2.0}, map[string]any{"x": 3.0, "y": 4.0}},
		"bubble_chart":  {map[string]any{"x": 1.0, "y": 2.0, "size": 5.0}},
	}
	for chartType, points := range cases {
		t.Run(chartType, func(t *testing.T) {
			if got := renderErrFor(t, chartType, map[string]any{
				"series": []any{map[string]any{"name": "Points", "values": points}},
			}); got != "" {
				t.Errorf("%s point data was refused: %s", chartType, got)
			}
		})
	}
}

// TestTimeSeriesIsExemptFromTheLengthCheck: a time series takes its x positions
// from its own time fields, so there are no categories to align against — but
// its values must still be numbers.
func TestTimeSeriesIsExemptFromTheLengthCheck(t *testing.T) {
	if got := renderErrFor(t, "line_chart", map[string]any{
		"series": []any{map[string]any{
			"name":         "Sales",
			"time_strings": []any{"2026-01", "2026-02", "2026-03"},
			"values":       []any{10.0, 12.0, 15.0},
		}},
	}); got != "" {
		t.Errorf("a time series was refused: %s", got)
	}
	if got := renderErrFor(t, "line_chart", map[string]any{
		"series": []any{map[string]any{
			"name":         "Sales",
			"time_strings": []any{"2026-01"},
			"values":       []any{"ten"},
		}},
	}); !strings.Contains(got, "must be numbers") {
		t.Errorf("a time series with a string value should still be refused, got %q", got)
	}
}

// TestManySeriesIssuesAreSummarised keeps an error about a twelve-series chart
// readable.
func TestManySeriesIssuesAreSummarised(t *testing.T) {
	series := make([]any, 0, 12)
	for i := 0; i < 12; i++ {
		series = append(series, map[string]any{"values": []any{1.0}})
	}
	got := renderErrFor(t, "bar_chart", map[string]any{
		"categories": []any{"A", "B", "C"},
		"series":     series,
	})
	if !strings.Contains(got, "and 9 more") {
		t.Errorf("error should summarise beyond %d issues, got %q", maxListedSeriesIssues, got)
	}
}
