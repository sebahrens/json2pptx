package svggen

import (
	"strings"
	"testing"
)

// go-slide-creator-b7qqg.23: capability discovery promised "auto log-scale at
// 1000x range" for bar and grouped_bar after the renderer stopped doing that
// (go-slide-creator-gpjwj). Discovery must describe what the renderer does:
// wide ranges stay linear and report chart.wide_range_linear; log is opt-in
// and visibly labelled.
func TestChartCapabilities_NoAutoLogPromise(t *testing.T) {
	for _, c := range ChartCapabilities() {
		if c.DensityBehavior == nil {
			continue
		}
		db := strings.ToLower(*c.DensityBehavior)
		if strings.Contains(db, "auto log") || strings.Contains(db, "auto-log") {
			t.Errorf("%s density_behavior still promises automatic log scale: %q", c.Type, *c.DensityBehavior)
		}
		if c.SupportsLogScale != nil && *c.SupportsLogScale {
			if !strings.Contains(db, "chart.wide_range_linear") || !strings.Contains(db, `style.scale is "log"`) {
				t.Errorf("%s density_behavior should describe linear-by-default and the explicit log opt-in: %q", c.Type, *c.DensityBehavior)
			}
		}
	}
}

// The behaviour the capability text describes, checked against the renderer.
func TestWideRangeBars_LinearByDefault_LogOnlyWhenAsked(t *testing.T) {
	data := func() map[string]any {
		return map[string]any{
			"categories": []any{"A", "B", "C"},
			"series": []any{
				map[string]any{"name": "Rev", "values": []any{1.0, 50.0, 490000.0}},
				map[string]any{"name": "Cost", "values": []any{2.0, 40.0, 300000.0}},
			},
		}
	}
	for _, ct := range []string{"bar_chart", "grouped_bar_chart"} {
		out, err := RenderMultiFormatWithFindings(&RequestEnvelope{Type: ct, Data: data()}, "svg")
		if err != nil {
			t.Fatalf("%s: %v", ct, err)
		}
		found := false
		for _, f := range out.Findings {
			if f.Code == FindingWideRangeLinear {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: wide-range data did not report %s", ct, FindingWideRangeLinear)
		}
		if strings.Contains(strings.ToLower(string(out.SVG.Content)), "log scale") {
			t.Errorf("%s: default render drew a log axis", ct)
		}

		logOut, err := RenderMultiFormatWithFindings(&RequestEnvelope{Type: ct, Data: data(), Style: StyleSpec{Scale: "log"}}, "svg")
		if err != nil {
			t.Fatalf("%s log: %v", ct, err)
		}
		if !strings.Contains(strings.ToLower(string(logOut.SVG.Content)), "log scale") {
			t.Errorf("%s: explicit log axis is not visibly labelled", ct)
		}
	}
}
