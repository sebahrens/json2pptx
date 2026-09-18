package svggen

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// go-slide-creator-m7ga: a degenerate domain (all-zero series, a single data
// point, a null coerced to 0) made tickStep return 0. log10(0) is -Inf, so
// precision became math.MaxInt and the format string "%.9223372036854775807f"
// was rejected by fmt — the literal Go error "%!(NOVERB)%!(EXTRA float64=0)"
// was drawn onto the slide as the axis label.
func TestLinearScaleTickFormat_DegenerateDomains(t *testing.T) {
	domains := []struct {
		name     string
		min, max float64
	}{
		{"all zero", 0, 0},
		{"flat non-zero", 5, 5},
		{"flat negative", -12.5, -12.5},
		{"sub-unit span", 0, 0.001},
		{"denormal span", -1e-30, 1e-30},
		{"normal span", 0, 100},
		{"tiny positive", 1e-12, 1e-12},
	}

	for _, d := range domains {
		t.Run(d.name, func(t *testing.T) {
			format := NewLinearScale(d.min, d.max).TickFormat(5)
			rendered := fmt.Sprintf(format, 0.0)
			if strings.Contains(rendered, "NOVERB") || strings.Contains(rendered, "%!") {
				t.Fatalf("format %q renders a Go format error: %q", format, rendered)
			}
		})
	}
}

// Precision must stay within a readable, representable range for any domain,
// including ones that would otherwise drive it to math.MaxInt.
func TestLinearScaleTickFormat_PrecisionIsClamped(t *testing.T) {
	for _, d := range []struct{ min, max float64 }{
		{0, 0}, {0, 1e-300}, {-math.MaxFloat64, math.MaxFloat64}, {0, 100},
	} {
		format := NewLinearScale(d.min, d.max).TickFormat(5)
		var precision int
		if _, err := fmt.Sscanf(format, "%%.%df", &precision); err != nil {
			t.Fatalf("domain %v..%v produced an unparseable format %q: %v", d.min, d.max, format, err)
		}
		if precision < 0 || precision > maxTickPrecision {
			t.Errorf("domain %v..%v precision = %d, want 0..%d", d.min, d.max, precision, maxTickPrecision)
		}
	}
}

// End-to-end: a flat series must not put a format-error string on the chart.
func TestRenderFlatSeries_NoFormatErrorInSVG(t *testing.T) {
	chartTypes := []string{"bar_chart", "line_chart", "area_chart", "stacked_bar_chart", "grouped_bar_chart"}
	seriesValues := map[string][]any{
		"all zero":          {0.0, 0.0},
		"single zero point": {0.0},
		"flat non-zero":     {5.0, 5.0},
	}

	for _, ct := range chartTypes {
		for name, values := range seriesValues {
			t.Run(ct+"/"+name, func(t *testing.T) {
				categories := make([]any, len(values))
				for i := range values {
					categories[i] = fmt.Sprintf("C%d", i+1)
				}
				series := []any{map[string]any{"name": "s1", "values": values}}
				if ct == "grouped_bar_chart" {
					// grouped_bar_chart requires at least 2 series.
					series = append(series, map[string]any{"name": "s2", "values": values})
				}
				doc, err := Render(&RequestEnvelope{
					Type: ct,
					Data: map[string]any{
						"categories": categories,
						"series":     series,
					},
				})
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				svg := string(doc.Content)
				if strings.Contains(svg, "NOVERB") || strings.Contains(svg, "%!(EXTRA") {
					t.Errorf("rendered SVG carries a Go format error string")
				}
			})
		}
	}
}
