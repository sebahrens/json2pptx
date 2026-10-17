package svggen

import (
	"strings"
	"testing"
)

func TestGraphicsAuditThemeNeutralsAndStatus(t *testing.T) {
	guide := StyleGuideFromSpec(StyleSpec{ThemeColors: []ThemeColorInput{{Name: "dk1", RGB: "#F8EEDB"}, {Name: "dk2", RGB: "#E7CBA7"}, {Name: "lt1", RGB: "#142234"}, {Name: "lt2", RGB: "#243244"}, {Name: "accent1", RGB: "#66CCFF"}, {Name: "accent2", RGB: "#FF88DD"}}, SemanticAccents: SemanticAccentSpec{Positive: "accent1", Negative: "accent2"}})
	p := guide.Palette
	if p.Border == MustParseColor(DefaultThemeBorderHex) || p.TextMuted == MustParseColor(DefaultThemeTextMutedHex) || p.TextMuted.ContrastWith(p.Background) < WCAGAALarge {
		t.Fatalf("neutrals did not follow dark theme: %+v", p)
	}
	if stakeholderStatusColor("supporter", p) != p.Success || stakeholderStatusColor("blocker", p) != p.Error || stakeholderStatusColor("neutral", p) != p.TextMuted {
		t.Fatal("status colors bypass theme roles")
	}
}

func TestGraphicsAuditCollapsedPie(t *testing.T) {
	b := NewSVGBuilder(10, 10)
	chart := NewPieChart(b, DefaultPieChartConfig(10, 10))
	if err := chart.Draw(ChartData{Categories: []string{"A", "B"}, Series: []ChartSeries{{Values: []float64{1, 2}}}}); err != nil {
		t.Fatal(err)
	}
	svg, err := b.RenderToString()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
		t.Fatal("nonfinite SVG coordinates")
	}
	if findFindingByCode(b.Findings(), FindingPlotAreaCollapsed) == nil {
		t.Fatalf("missing collapse finding: %+v", b.Findings())
	}
}

func TestGraphicsAuditFlatLineRange(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []float64
		lo, hi float64
		field  string
	}{
		{"positive", []float64{95.1, 91}, 0, 120, "data.y_min"},
		{"negative", []float64{-95.1, -91}, -120, 0, "data.y_max"},
		{"mixed", []float64{-1, 2}, -100, 100, "data.y_min"},
		{"constant", []float64{0, 0}, 0, 100, ""},
		{"already_zoomed", []float64{91, 95}, 90, 100, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewSVGBuilder(600, 400)
			reportFlatLineRange(b, ChartData{Series: []ChartSeries{{Values: tc.values}}}, NewLinearScale(tc.lo, tc.hi))
			f := findFindingByCode(b.Findings(), FindingFlatRange)
			if tc.field == "" {
				if f != nil {
					t.Fatalf("unexpected finding %+v", f)
				}
				return
			}
			if f == nil || f.Field != tc.field || f.Fix == nil {
				t.Fatalf("missing usable axis suggestion %+v", f)
			}
			value := f.Fix.Params["value"].(float64)
			for _, v := range tc.values {
				if tc.field == "data.y_min" && value > v || tc.field == "data.y_max" && value < v {
					t.Fatal("suggestion clips data")
				}
			}
		})
	}
}

func TestGraphicsAuditLineDrawPreservesScaleAndReportsAdvice(t *testing.T) {
	b := NewSVGBuilder(640, 420)
	chart := NewLineChart(b, DefaultLineChartConfig(640, 420))
	data := ChartData{Categories: []string{"2024", "2025", "2026"}, Series: []ChartSeries{{Name: "Rate", Values: []float64{91, 93, 95}}}}
	if err := chart.Draw(data); err != nil {
		t.Fatal(err)
	}
	f := findFindingByCode(b.Findings(), FindingFlatRange)
	if f == nil || f.Fix == nil || f.Fix.Kind != FixKindExplicitScale || f.Fix.Params["axis_min"] != float64(0) || f.Fix.Params["data_min"] != float64(91) {
		t.Fatalf("missing factual, non-mutating scale advice: %+v", f)
	}
	if data.Axis.Min != nil || data.Axis.Max != nil {
		t.Fatal("draw mutated authored scale")
	}
}
