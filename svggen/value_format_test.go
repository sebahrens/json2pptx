package svggen

import "testing"

// go-slide-creator-66qb. Every value label went through a hardcoded "%.0f".
// A seven-bar series [4.6 … 6.5] printed three distinct labels for seven bars,
// denying its own axis and its own "+6% a year" headline; an ARR line
// [61.2 … 86.4] printed 61 64 65 66 71 75 81 86 on a slide whose bullet said
// "from EUR 66.0M to EUR 86.4M". The chart contradicted the narrative beside it.

func TestAutoValueFormatKeepsLabelsDistinct(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{"the reported seven-bar series", []float64{4.6, 4.9, 5.2, 5.5, 5.8, 6.2, 6.5}, "%.1f"},
		{"the reported ARR series", []float64{61.2, 64.0, 65.1, 66.0, 71.3, 75.8, 81.2, 86.4}, "%.1f"},
		{"integers stay integers", []float64{1240, 865, 413}, "%.0f"},
		{"one decimal is not enough", []float64{1.01, 1.02, 1.03}, "%.2f"},
		{"beyond two decimals we stop", []float64{1.0001, 1.0002}, "%.2f"},
		{"no values", nil, "%.0f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autoValueFormat("", tt.values); got != tt.want {
				t.Errorf("autoValueFormat = %q, want %q", got, tt.want)
			}
			// The default sentinel behaves the same as an empty format.
			if got := autoValueFormat(defaultValueFormat, tt.values); got != tt.want {
				t.Errorf("autoValueFormat(default) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAutoValueFormatRespectsAnExplicitFormat(t *testing.T) {
	values := []float64{4.6, 4.9, 5.2}
	for _, explicit := range []string{"%.3f", "€%.1fM", "%.0f%%"} {
		if got := autoValueFormat(explicit, values); got != explicit {
			t.Errorf("caller format %q was overridden with %q", explicit, got)
		}
	}
}

func TestFormatValueGroupsThousands(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{48200, "48,200"},
		{12400, "12,400"},
		{1240, "1,240"},
		{860, "860"},
		{-12400, "-12,400"},
		{1234567, "1,234,567"},
		{999, "999"},
	}
	for _, tt := range tests {
		if got := formatValueGrouped(tt.value, "%.0f"); got != tt.want {
			t.Errorf("formatValueGrouped(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestFormatValueGroupedKeepsDecimalsAndUnits(t *testing.T) {
	if got := formatValueGrouped(12400.5, "%.1f"); got != "12,400.5" {
		t.Errorf("decimals lost: %q", got)
	}
	if got := formatValueGrouped(12400, "€%.0fM"); got != "€12,400M" {
		t.Errorf("prefix/suffix mangled: %q", got)
	}
}

func TestGroupThousandsLeavesShortNumbersAlone(t *testing.T) {
	for _, s := range []string{"860", "0", "-5", "", "abc", "1.5"} {
		if got := groupThousands(s); got != s {
			t.Errorf("groupThousands(%q) = %q", s, got)
		}
	}
}

func TestChartDataValuesFlattensEverySeries(t *testing.T) {
	data := ChartData{Series: []ChartSeries{
		{Name: "a", Values: []float64{1, 2}},
		{Name: "b", Values: []float64{3}},
	}}
	got := chartDataValues(data)
	if len(got) != 3 || got[2] != 3 {
		t.Errorf("chartDataValues = %v", got)
	}
}
