package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// filledArcPathRe matches a filled, stroked path — the shape a pie or donut
// slice is emitted as. Legend swatches carry a bare fill attribute instead.
var filledArcPathRe = regexp.MustCompile(`<path d="[^"]*" style="fill:#[0-9a-fA-F]+;stroke:`)

// go-slide-creator-s1uvj.30: ArcSeries.Draw returned early only on total==0,
// so a pie whose values summed below zero ([-1,-2]) rendered a full pie with
// 33%/67% labels, and a negative slice among positives ([-1,2,3]) shrank the
// denominator so the labels read 50% + 75% and the arcs overlapped. Negative
// slices must be excluded before totalling (and reported), and nothing is
// drawn when the remaining total is <= 0.
func TestPieChart_NegativeSlicesExcluded(t *testing.T) {
	for _, typ := range []string{"pie_chart", "donut_chart"} {
		t.Run(typ+"/all negative draws nothing", func(t *testing.T) {
			output := renderPieForTest(t, typ, []any{"A", "B"}, []any{-1.0, -2.0})
			svg := string(output.SVG.Content)
			if n := len(filledArcPathRe.FindAllString(svg, -1)); n != 0 {
				t.Errorf("drew %d arcs for a pie with no positive values; want none", n)
			}
			for _, pct := range []string{"33%", "67%"} {
				if strings.Contains(svg, ">"+pct+"<") || strings.Contains(svg, " "+pct+"<") {
					t.Errorf("SVG labels a slice %s for a pie with no positive total", pct)
				}
			}
			if findFindingByCode(output.Findings, FindingZeroSumPie) == nil {
				t.Errorf("expected %s finding, got %v", FindingZeroSumPie, output.Findings)
			}
			if findFindingByCode(output.Findings, FindingNegativePieSlice) == nil {
				t.Errorf("expected %s finding, got %v", FindingNegativePieSlice, output.Findings)
			}
		})

		t.Run(typ+"/negative slice excluded from total", func(t *testing.T) {
			output := renderPieForTest(t, typ, []any{"Loss", "B", "C"}, []any{-1.0, 2.0, 3.0})
			svg := string(output.SVG.Content)
			if n := len(filledArcPathRe.FindAllString(svg, -1)); n != 2 {
				t.Errorf("drew %d arcs, want 2 (the positive slices only)", n)
			}
			for _, bad := range []string{"50%", "75%"} {
				if strings.Contains(svg, bad+"<") {
					t.Errorf("SVG still labels a slice %s (negative value shrank the denominator)", bad)
				}
			}
			for _, want := range []string{"40%", "60%"} {
				if !strings.Contains(svg, want+"<") {
					t.Errorf("SVG lacks the %s label for the positive slices", want)
				}
			}
			f := findFindingByCode(output.Findings, FindingNegativePieSlice)
			if f == nil {
				t.Fatalf("expected %s finding, got %v", FindingNegativePieSlice, output.Findings)
			}
			if f.Severity != "warning" || f.Fix == nil || f.Fix.Kind != FixKindReplaceValue {
				t.Errorf("finding = %+v, want warning with replace_value fix", f)
			}
			if findFindingByCode(output.Findings, FindingZeroSumPie) != nil {
				t.Errorf("unexpected %s finding: the positive slices still total 5", FindingZeroSumPie)
			}
		})
	}
}

func renderPieForTest(t *testing.T, typ string, labels, values []any) *RenderOutput {
	t.Helper()
	req := &RequestEnvelope{
		Type:   typ,
		Data:   map[string]any{"labels": labels, "values": values},
		Style:  StyleSpec{ShowValues: true},
		Output: OutputSpec{Width: 600, Height: 400},
	}
	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("render %s: %v", typ, err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}
	return output
}
