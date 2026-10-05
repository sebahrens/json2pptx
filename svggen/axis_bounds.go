package svggen

import (
	"fmt"
	"math"
)

// AxisBounds carries the authored value-axis bounds of a Cartesian chart:
// data.y_min and data.y_max. A nil bound is computed from the data.
//
// Bar, waterfall, line and area charts start their value axis at zero whenever
// no plotted value is negative: a bar's length and a band's height are read
// against the baseline, and a count or revenue line zoomed onto its own range
// overstates the trend (go-slide-creator-929jm). An author who wants a zoomed
// axis — an index around 100, a bridge whose steps are small against its
// totals — says so here; the chart then keeps its value axis so the truncation
// is visible, and chart.axis_not_zero reports a y_min that hides more than
// half of the smallest bar drawn from the baseline.
type AxisBounds struct {
	Min *float64
	Max *float64
}

// apply overrides the computed domain with the authored bounds.
func (ab AxisBounds) apply(lo, hi float64) (float64, float64) {
	if ab.Min != nil {
		lo = *ab.Min
	}
	if ab.Max != nil {
		hi = *ab.Max
	}
	return lo, hi
}

// zoomed reports whether the authored minimum sits above zero, i.e. the axis
// does not start at the baseline.
func (ab AxisBounds) zoomed() bool {
	return ab.Min != nil && *ab.Min > 0
}

// parseAxisBounds reads data.y_min / data.y_max. Both are optional numbers;
// when both are present y_min must be below y_max.
func parseAxisBounds(data map[string]any) (AxisBounds, error) {
	var ab AxisBounds
	for _, key := range []string{"y_min", "y_max"} {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		v, ok := toFloat64Value(raw)
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
			return ab, &ValidationError{
				Field:   "data." + key,
				Code:    ErrCodeInvalidType,
				Message: fmt.Sprintf("%s must be a finite number (the value-axis %s), got %v", key, key[2:], raw),
				Value:   raw,
			}
		}
		v2 := v
		if key == "y_min" {
			ab.Min = &v2
		} else {
			ab.Max = &v2
		}
	}
	if ab.Min != nil && ab.Max != nil && *ab.Min >= *ab.Max {
		return ab, &ValidationError{
			Field:   "data.y_min",
			Code:    ErrCodeInvalidValue,
			Message: fmt.Sprintf("y_min (%v) must be below y_max (%v)", *ab.Min, *ab.Max),
			Value:   *ab.Min,
		}
	}
	return ab, nil
}

// checkAxisBoundsCoverData rejects a y_min above the lowest plotted value or a
// y_max below the highest: the mark would be cut, and a cut bar misstates the
// number it stands for. what names the lowest / highest mark in the message.
func checkAxisBoundsCoverData(ab AxisBounds, dataMin, dataMax float64, what string) error {
	if ab.Min != nil && *ab.Min > dataMin {
		return &ValidationError{
			Field:   "data.y_min",
			Code:    ErrCodeInvalidValue,
			Message: fmt.Sprintf("y_min %v is above the lowest %s (%v), which would cut it off the chart; set y_min to %v or less", *ab.Min, what, dataMin, dataMin),
			Value:   *ab.Min,
		}
	}
	if ab.Max != nil && *ab.Max < dataMax {
		return &ValidationError{
			Field:   "data.y_max",
			Code:    ErrCodeInvalidValue,
			Message: fmt.Sprintf("y_max %v is below the highest %s (%v), which would cut it off the chart; set y_max to %v or more", *ab.Max, what, dataMax, dataMax),
			Value:   *ab.Max,
		}
	}
	return nil
}

// reportAxisNotZero emits chart.axis_not_zero when an authored y_min hides
// more than half of the smallest bar drawn from the baseline (smallest > 0,
// labelled label). The chart is still drawn as authored — the axis says so —
// but the reader should know the shortest bar shows less than half its length.
func reportAxisNotZero(b *SVGBuilder, ab AxisBounds, smallest float64, label, chartType string) {
	if b == nil || !ab.zoomed() || smallest <= 0 {
		return
	}
	hidden := *ab.Min / smallest
	if hidden <= 0.5 {
		return
	}
	b.AddFinding(Finding{
		Field:    "data.y_min",
		Code:     FindingAxisNotZero,
		Severity: "info",
		Message: fmt.Sprintf("y_min %v starts the value axis above zero and hides %.0f%% of the smallest bar (%q = %v); the axis is kept so the truncation is visible — remove y_min for a zero baseline",
			*ab.Min, hidden*100, label, smallest),
		Fix: &FixSuggestion{
			Kind: FixKindReplaceValue,
			Params: map[string]any{
				"field":        "data.y_min",
				"authored":     *ab.Min,
				"smallest_bar": smallest,
				"hidden_share": math.Round(hidden*100) / 100,
				"diagram_type": chartType,
			},
		},
	})
}

// smallestBaselineBar returns the smallest positive value among series values
// — the shortest bar drawn from the baseline on a bar / area chart — with the
// category it belongs to. ok is false when no value is positive.
func smallestBaselineBar(data ChartData) (value float64, label string, ok bool) {
	value = math.Inf(1)
	for _, s := range data.Series {
		for i, v := range s.Values {
			if v > 0 && v < value {
				value, ok = v, true
				if i < len(data.Categories) {
					label = data.Categories[i]
				} else {
					label = s.Name
				}
			}
		}
	}
	return value, label, ok
}

// smallestBaselineStack returns the smallest positive stack total of a
// stacked chart with its category.
func smallestBaselineStack(data ChartData) (value float64, label string, ok bool) {
	value = math.Inf(1)
	for i := range data.Categories {
		sum := 0.0
		for _, s := range data.Series {
			if i < len(s.Values) && s.Values[i] > 0 {
				sum += s.Values[i]
			}
		}
		if sum > 0 && sum < value {
			value, label, ok = sum, data.Categories[i], true
		}
	}
	return value, label, ok
}

// seriesValueRange returns the lowest and highest mark the series draw: raw
// values, or per-category stacks (positive and negative sums) when stacked.
// ok is false without any value.
func seriesValueRange(data ChartData, stacked bool) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(1), math.Inf(-1)
	if stacked {
		n := 0
		for _, s := range data.Series {
			if len(s.Values) > n {
				n = len(s.Values)
			}
		}
		for i := 0; i < n; i++ {
			pos, neg := 0.0, 0.0
			for _, s := range data.Series {
				if i < len(s.Values) {
					if v := s.Values[i]; v > 0 {
						pos += v
					} else {
						neg += v
					}
				}
			}
			lo, hi, ok = math.Min(lo, neg), math.Max(hi, pos), true
		}
		return lo, hi, ok
	}
	for _, s := range data.Series {
		for _, v := range s.Values {
			lo, hi, ok = math.Min(lo, v), math.Max(hi, v), true
		}
	}
	return lo, hi, ok
}

// checkChartAxisBounds validates the authored bounds of a categories+series
// chart against its data. Stacked charts measure their stacks, not their
// segments.
func checkChartAxisBounds(data ChartData, stacked bool) error {
	if data.Axis.Min == nil && data.Axis.Max == nil {
		return nil
	}
	lo, hi, ok := seriesValueRange(data, stacked)
	if !ok {
		return nil
	}
	what := "value"
	if stacked {
		what = "stack total"
	}
	return checkAxisBoundsCoverData(data.Axis, lo, hi, what)
}

// validateAxisBounds is the Validate-time counterpart of extractChartData's
// bound handling for the chart types that read data.y_min / data.y_max.
func validateAxisBounds(data map[string]any, stacked bool) error {
	ab, err := parseAxisBounds(data)
	if err != nil {
		return err
	}
	if ab.Min == nil && ab.Max == nil {
		return nil
	}
	cd := ChartData{Axis: ab}
	if seriesSlice, ok := toSeriesSlice(data["series"]); ok {
		for _, s := range seriesSlice {
			vals, _ := toFloat64Slice(s["values"])
			cd.Series = append(cd.Series, ChartSeries{Values: vals})
		}
	}
	return checkChartAxisBounds(cd, stacked)
}

// axisBoundsFieldSchemas describes data.y_min / data.y_max for the chart
// schemas that read them.
func axisBoundsFieldSchemas(fields map[string]*DataSchema) map[string]*DataSchema {
	fields["y_min"] = NumberDataSchema("Value-axis minimum. Omitted, the axis starts at zero whenever no value is negative (a bar's length and a band's height are read from the baseline; a count or revenue line zoomed onto its own range overstates the trend). Set it to zoom — an index around 100, a bridge whose steps are small against its totals; the chart then keeps its value axis so the truncation is visible, and a y_min that hides more than half of the smallest bar reports chart.axis_not_zero. Rejected above the lowest plotted value (the bar would be cut) and must be below y_max. Rounded down to the nearest tick (y_min 1 on a 2-step axis draws 0).")
	fields["y_max"] = NumberDataSchema("Value-axis maximum. Omitted, the axis is computed from the data with headroom for labels. Rejected below the highest plotted value; rounded up to the nearest tick.")
	return fields
}
