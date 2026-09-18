package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func barChart(seriesNames ...string) *types.DiagramSpec {
	series := make([]any, 0, len(seriesNames))
	for _, n := range seriesNames {
		series = append(series, map[string]any{"name": n, "values": []any{1, 2, 3}})
	}
	return &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"categories": []any{"A", "B", "C"}, "series": series}}
}

func TestChartInsightsSplit_ChartCaption(t *testing.T) {
	cases := []struct {
		name string
		v    *ChartInsightsSplitValues
		want string
	}{
		{"series name + unit", &ChartInsightsSplitValues{Chart: barChart("Revenue"), Unit: "$M"}, "Revenue ($M)"},
		{"unit already in name", &ChartInsightsSplitValues{Chart: barChart("Revenue ($M)"), Unit: "$M"}, "Revenue ($M)"},
		{"series name only", &ChartInsightsSplitValues{Chart: barChart("Revenue")}, "Revenue"},
		{"multi-series unit only", &ChartInsightsSplitValues{Chart: barChart("Plan", "Actual"), Unit: "k units"}, "Values in k units"},
		{"multi-series no unit", &ChartInsightsSplitValues{Chart: barChart("Plan", "Actual")}, ""},
		{"explicit label wins", &ChartInsightsSplitValues{Chart: barChart("Revenue"), ChartLabel: "Net revenue, $M"}, "Net revenue, $M"},
		{"chart title suppresses caption", &ChartInsightsSplitValues{Chart: &types.DiagramSpec{Type: "bar_chart", Title: "Revenue", Data: barChart("Revenue").Data}}, ""},
		{"no chart", &ChartInsightsSplitValues{}, ""},
	}
	for _, tc := range cases {
		if got := chartCaption(tc.v); got != tc.want {
			t.Errorf("%s: caption = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestChartInsightsSplit_DataLabels(t *testing.T) {
	on, off := true, false
	src := barChart("Revenue")
	if got := chartWithDataLabels(src, nil); got.Style == nil || !got.Style.ShowValues {
		t.Error("bar chart with few points should get value labels by default")
	}
	if src.Style != nil {
		t.Error("caller's chart spec must not be mutated")
	}
	if got := chartWithDataLabels(src, &off); got.Style.ShowValues {
		t.Error("data_labels:false must switch labels off")
	}

	line := &types.DiagramSpec{Type: "line_chart", Data: map[string]any{"categories": []any{"a", "b"}, "series": []any{
		map[string]any{"name": "Plan", "values": []any{1, 2}}, map[string]any{"name": "Actual", "values": []any{2, 3}},
	}}}
	if chartWithDataLabels(line, nil).Style.ShowValues {
		t.Error("multi-series line charts should not get colliding labels by default")
	}
	if !chartWithDataLabels(line, &on).Style.ShowValues {
		t.Error("data_labels:true must force labels on")
	}

	many := barChart("x")
	vals := make([]any, 20)
	for i := range vals {
		vals[i] = i
	}
	many.Data["series"] = []any{map[string]any{"name": "x", "values": vals}}
	if chartWithDataLabels(many, nil).Style.ShowValues {
		t.Error("charts with many points should not be labelled by default")
	}

	configured := barChart("x")
	configured.Data["data_labels"] = map[string]any{"format": "%.1f"}
	if chartWithDataLabels(configured, nil).Style.ShowValues {
		t.Error("an explicit data.data_labels payload is left to svggen")
	}
	pie := &types.DiagramSpec{Type: "pie_chart", Data: map[string]any{"values": []any{1, 2}}}
	if chartWithDataLabels(pie, nil).Style.ShowValues {
		t.Error("non label-friendly chart types keep their defaults")
	}
}

func TestChartInsightsSplit_HeadlineAndSoWhatColumn(t *testing.T) {
	p := &chartInsightsSplit{}
	v := p.ExemplarValues().(*ChartInsightsSplitValues)
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("exemplar must validate: %v", err)
	}
	grid, err := p.Expand(fullThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	col := grid.Rows[0].Cells[1]
	if col.Grid == nil || len(col.Grid.Rows) != 3 {
		t.Fatalf("insights column should stack headline / insights / so-what, got %+v", col)
	}
	head := cellText(t, col.Grid.Rows[0].Cells[0].Shape.Text).Paragraphs
	if head[0].Content != "+75%" || head[0].Size < 24 || !head[0].Bold || head[1].Size < 12 {
		t.Errorf("headline paragraphs = %+v", head)
	}
	soWhat := col.Grid.Rows[2].Cells[0]
	if soWhat.AccentBar == nil || !strings.Contains(string(soWhat.Shape.Fill), "lumMod") {
		t.Errorf("so-what should be a tinted, accent-barred callout: %+v", soWhat)
	}
	if txt := cellText(t, soWhat.Shape.Text).Paragraphs[0]; !strings.HasPrefix(txt.Content, "<b>So what:</b>") || txt.Size < 12 {
		t.Errorf("so-what text = %+v", txt)
	}
	if col.AccentBar != nil {
		t.Error("the stacked column carries no divider (nested-grid accent bars are not drawn)")
	}
	sum := 0.0
	for _, r := range col.Grid.Rows {
		sum += r.Height
	}
	if sum < 99 || sum > 101 {
		t.Errorf("column rows should share 100%%, got %.1f", sum)
	}
	// Source row is pinned in points so its 12pt floor never autofits.
	src := grid.Rows[len(grid.Rows)-1]
	if src.MinHeight != cisSourceRowPt || src.MaxHeight != cisSourceRowPt {
		t.Errorf("source row = %+v", src)
	}
}

func TestChartInsightsSplit_DenseColumnStepsDown(t *testing.T) {
	p := &chartInsightsSplit{}
	v := &ChartInsightsSplitValues{
		Chart:    barChart("x"),
		Headline: &ChartInsightsHeadline{Value: "+12%"},
		Insights: []string{"a", "b", "c", "d", "e", "f"},
		SoWhat:   "Act now.",
	}
	grid, err := p.Expand(fullThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	col := grid.Rows[0].Cells[1].Grid
	if got := cellText(t, col.Rows[0].Cells[0].Shape.Text).Paragraphs[0].Size; got != 26 {
		t.Errorf("headline should step down to 26pt with 6 insights, got %v", got)
	}
	if got := cellText(t, col.Rows[2].Cells[0].Shape.Text).Paragraphs[0].Size; got != 12 {
		t.Errorf("so-what should step down to 12pt with 6 insights, got %v", got)
	}
	pinned, _ := p.Expand(fullThemeCtx(), v, &ChartInsightsSplitOverrides{HeadlineSize: 40}, nil)
	if got := cellText(t, pinned.Rows[0].Cells[1].Grid.Rows[0].Cells[0].Shape.Text).Paragraphs[0].Size; got != 40 {
		t.Errorf("headline_size override should pin the size, got %v", got)
	}
}

func TestChartInsightsSplit_NoChartKeepsExtras(t *testing.T) {
	p := &chartInsightsSplit{}
	v := &ChartInsightsSplitValues{Insights: []string{"a"}, SoWhat: "Act."}
	grid, err := p.Expand(fullThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c := grid.Rows[0].Cells; len(c) != 1 || c[0].Grid == nil {
		t.Fatalf("insights-only fallback should still carry the so-what column: %+v", c)
	}
}

func TestChartInsightsSplit_ValidateExtras(t *testing.T) {
	p := &chartInsightsSplit{}
	long := strings.Repeat("x", 200)
	cases := []struct {
		v    *ChartInsightsSplitValues
		want string
	}{
		{&ChartInsightsSplitValues{Insights: []string{"a"}, Headline: &ChartInsightsHeadline{Label: "no value"}}, "headline.value"},
		{&ChartInsightsSplitValues{Insights: []string{"a"}, Headline: &ChartInsightsHeadline{Value: long}}, "headline.value"},
		{&ChartInsightsSplitValues{Insights: []string{"a"}, Headline: &ChartInsightsHeadline{Value: "1", Label: long}}, "headline.label"},
		{&ChartInsightsSplitValues{Insights: []string{"a"}, SoWhat: long}, "so_what"},
		{&ChartInsightsSplitValues{Insights: []string{"a"}, ChartLabel: long}, "chart_label"},
		{&ChartInsightsSplitValues{Insights: []string{"a"}, Unit: long}, "unit"},
	}
	for _, tc := range cases {
		err := p.Validate(tc.v, nil, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("error = %v, want mention of %q", err, tc.want)
		}
	}
	raw := `{"insights":["a"],"headline":{"value":"+5%","label":"growth"},"so_what":"Act.","unit":"$M","chart_label":"Revenue"}`
	var v ChartInsightsSplitValues
	if err := json.Unmarshal([]byte(raw), &v); err != nil || v.Headline == nil || v.SoWhat == "" || v.Unit != "$M" || v.ChartLabel != "Revenue" {
		t.Errorf("new fields should decode: %+v %v", v, err)
	}
}
