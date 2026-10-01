package generator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestChartValueSubtitleAndFootnoteRender pins the chart_value subtitle /
// footnote surface RULES.md documents: both used to be rejected as unknown
// fields and dropped, so a chart's source attribution never reached the slide
// (go-slide-creator-p0zs7).
func TestChartValueSubtitleAndFootnoteRender(t *testing.T) {
	const footnote = "Source: Internal finance data, FY26 close"
	const subtitle = "Quarterly, $M"
	cases := map[string]string{
		"bar":         `{"type":"bar","title":"Revenue","subtitle":"Quarterly, $M","footnote":"Source: Internal finance data, FY26 close","data":{"categories":["Q1","Q2","Q3"],"series":[{"name":"Revenue","values":[34,40,44]}]}}`,
		"line":        `{"type":"line","title":"Revenue","subtitle":"Quarterly, $M","footnote":"Source: Internal finance data, FY26 close","data":{"categories":["Q1","Q2","Q3"],"series":[{"name":"Revenue","values":[34,40,44]}]}}`,
		"pie":         `{"type":"pie","title":"Mix","subtitle":"Quarterly, $M","footnote":"Source: Internal finance data, FY26 close","data":{"A":40,"B":35,"C":25}}`,
		"area":        `{"type":"area","title":"Revenue","subtitle":"Quarterly, $M","footnote":"Source: Internal finance data, FY26 close","data":{"categories":["Q1","Q2","Q3"],"series":[{"name":"Revenue","values":[34,40,44]}]}}`,
		"stacked_bar": `{"type":"stacked_bar","title":"Revenue","subtitle":"Quarterly, $M","footnote":"Source: Internal finance data, FY26 close","data":{"categories":["Q1","Q2"],"series":[{"name":"A","values":[3,4]},{"name":"B","values":[1,2]}]}}`,
		"donut":       `{"type":"donut","title":"Mix","subtitle":"Quarterly, $M","footnote":"Source: Internal finance data, FY26 close","data":{"A":40,"B":35,"C":25}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var chart types.ChartSpec //nolint:staticcheck // chart_value surface
			if err := json.Unmarshal([]byte(raw), &chart); err != nil {
				t.Fatal(err)
			}
			if chart.Footnote != footnote || chart.Subtitle != subtitle {
				t.Fatalf("decoded subtitle/footnote = %q / %q", chart.Subtitle, chart.Footnote)
			}
			res, err := RenderDiagramSpecWithMetadata(chart.ToDiagramSpec(), nil, 0, true)
			if err != nil {
				t.Fatal(err)
			}
			svg := string(res.SVG)
			if !strings.Contains(svg, "Internal finance data") {
				t.Errorf("footnote text missing from the %s chart SVG", name)
			}
			if !strings.Contains(svg, "Quarterly") {
				t.Errorf("subtitle text missing from the %s chart SVG", name)
			}
		})
	}
}
