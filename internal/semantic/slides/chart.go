package slides

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/types"
)

// chartInsightsValues mirrors patterns.ChartInsightsSplitValues for emission.
type chartInsightsValues struct {
	Chart         *types.DiagramSpec `json:"chart,omitempty"`
	InsightsTitle string             `json:"insights_title,omitempty"`
	Insights      []string           `json:"insights"`
	Source        string             `json:"source,omitempty"`
}

// ChartInsightMaxInsights is the insight-bullet cap of the chart-insights-split
// pattern. Beyond it the compiler degrades to a content slide and drops the
// chart entirely, so validation and the explain planner share this bound to stay
// in step with compile.
const ChartInsightMaxInsights = 6

// ChartInsightInsightCount returns the number of insight bullets a chart_insight
// payload will compile with, mirroring CompileChartInsight: the "insights" list
// (or the single "insight") — or, when those are absent but a renderable chart
// and a "takeaway" are present, the takeaway counts as a single insight (the
// takeaway-as-lone-insight fallback). Validation and the explain planner consult
// this so neither flags nor advertises a treatment that disagrees with compile.
func ChartInsightInsightCount(body map[string]any) int {
	insights, _ := chartInsights(body)
	if len(insights) == 0 && strField(body, "takeaway") != "" && chartSpec(body) != nil {
		return 1
	}
	return len(insights)
}

// ChartInsightPatternFeasible reports whether a chart_insight payload will
// compile to the chart-insights-split pattern rather than degrading to a content
// fallback (which drops the chart): it must resolve to 1–ChartInsightMaxInsights
// usable insight bullets.
func ChartInsightPatternFeasible(body map[string]any) bool {
	n := ChartInsightInsightCount(body)
	return n >= 1 && n <= ChartInsightMaxInsights
}

// CompileChartInsight compiles a chart-insight slide. When the payload exposes
// 1–ChartInsightMaxInsights insight bullets it emits a chart-insights-split
// pattern (left chart panel + right insights), including the chart only when it
// carries a type and a non-empty data payload — the pattern renders insights
// full-width otherwise. Without usable insights (or beyond the cap) it degrades
// to a content slide so the deck still compiles.
func CompileChartInsight(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	insights, insightsField := chartInsights(in.Body)

	// A chart_insight may carry a usable chart and a takeaway but no explicit
	// insight bullets. The content fallback below cannot render a chart, so
	// without this the chart silently disappears even though validation passed.
	// Treat the takeaway as the single insight so the chart-insights-split
	// pattern still emits the chart — mirroring the "insight" alias, where the
	// same line already serves as both the lone insight bullet and the takeaway.
	if len(insights) == 0 && in.Takeaway != "" && chartSpec(in.Body) != nil {
		insights = []string{in.Takeaway}
		insightsField = "takeaway"
	}

	if len(insights) == 0 || len(insights) > ChartInsightMaxInsights {
		return compileChartFallback(in, insights, insightsField)
	}

	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	vals := chartInsightsValues{Insights: insights}
	if chart := chartSpec(in.Body); chart != nil {
		vals.Chart = chart
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.chart",
			SemanticPath: in.semSlide() + ".chart",
		})
	}
	if src := strField(in.Body, "source"); src != "" {
		vals.Source = src
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.source",
			SemanticPath: in.semSlide() + ".source",
		})
	}

	encoded, err := json.Marshal(vals)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal chart-insights-split values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{
		Name:   "chart-insights-split",
		Values: encoded,
	}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.insights",
		SemanticPath: in.semSlide() + "." + insightsField,
	})

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileChartFallback renders a content slide carrying the title, any insight
// bullets, and the takeaway when the chart-insights-split shape does not fit.
func compileChartFallback(in Input, insights []string, insightsField string) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	if len(insights) > 0 {
		idx := appendContent(slide, bulletsContent("body", insights))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + insightsField,
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// chartInsights collects the insight bullets from the payload, preferring the
// "insights" list, then the single "insight" string. The returned string is the
// semantic field the bullets came from.
func chartInsights(body map[string]any) ([]string, string) {
	if list, ok := stringList(body, "insights"); ok && len(list) > 0 {
		return list, "insights"
	}
	if s := strField(body, "insight"); s != "" {
		return []string{s}, "insight"
	}
	return nil, "insights"
}

// chartSpec builds a DiagramSpec from the payload's "chart" field, returning nil
// when no chart, no type, or no data is present (the pattern requires a typed,
// data-bearing chart to render the left panel).
func chartSpec(body map[string]any) *types.DiagramSpec {
	raw, ok := body["chart"].(map[string]any)
	if !ok {
		return nil
	}
	ctype := strField(raw, "type")
	data, hasData := raw["data"].(map[string]any)
	if ctype == "" || !hasData || len(data) == 0 {
		return nil
	}
	return &types.DiagramSpec{
		Type:  ctype,
		Title: strField(raw, "title"),
		Data:  data,
	}
}
