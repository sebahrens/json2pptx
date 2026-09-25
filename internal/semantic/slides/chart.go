package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// chartInsightsValues mirrors patterns.ChartInsightsSplitValues for emission.
type chartInsightsValues struct {
	Chart         *types.DiagramSpec `json:"chart,omitempty"`
	InsightsTitle string             `json:"insights_title,omitempty"`
	Insights      []string           `json:"insights"`
	SoWhat        string             `json:"so_what,omitempty"`
	Source        string             `json:"source,omitempty"`
}

// ChartInsightMaxInsights is the insight-bullet cap of the chart-insights-split
// pattern. Beyond it the compiler degrades to a native two-column slide (chart
// beside the full insight list), so validation and the explain planner share
// this bound to stay in step with compile.
const ChartInsightMaxInsights = 6

// ChartInsightInsightCount returns the number of insight items a chart_insight
// payload will compile with, mirroring CompileChartInsight: the "insights" list
// (or the single "insight" callout) — or, when those are absent but a chart
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
// compile to the chart-insights-split pattern rather than degrading to the
// native two-column fallback: it must resolve to 1–ChartInsightMaxInsights
// usable insight items (bullets or a lone callout).
func ChartInsightPatternFeasible(body map[string]any) bool {
	n := ChartInsightInsightCount(body)
	return n >= 1 && n <= ChartInsightMaxInsights
}

// CompileChartInsight compiles a chart-insight slide. When the payload exposes
// 1–ChartInsightMaxInsights insight items it emits a chart-insights-split
// pattern (left chart panel + right insights), including the chart only when it
// carries a type and a non-empty data payload — the pattern renders insights
// full-width otherwise. Without usable insights (or beyond the cap) it degrades
// to a native two-column (chart + insights) or content slide so the deck still
// compiles without losing either.
func CompileChartInsight(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	insights, insightsField := chartInsights(in.Body)

	// A chart_insight may carry a usable chart and a takeaway but no explicit
	// insight bullets. The content fallback below cannot render a chart, so
	// without this the chart silently disappears even though validation passed.
	// Treat the takeaway as the single insight so the chart-insights-split
	// pattern still emits the chart. A scalar "insight" becomes a callout.
	if len(insights) == 0 && in.Takeaway != "" && chartSpec(in.Body) != nil {
		insights = []string{in.Takeaway}
		insightsField = "takeaway"
	}

	// A lone insight that IS the takeaway would print the same sentence twice:
	// once in the pattern and once verbatim in the takeaway bar. The pattern
	// carries the slide's content, so the band is what gives way
	// (go-slide-creator-pyxn).
	in.Takeaway = dropDuplicateTakeaway(in.Takeaway, insights)

	if len(insights) == 0 || len(insights) > ChartInsightMaxInsights {
		return compileChartFallback(in, insights, insightsField)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	vals := chartInsightsValues{Insights: insights}
	if insightsField == "insight" {
		// A single implication is a callout, not a one-item list headed
		// "Key Insights". The pattern accepts callout-only content.
		vals.Insights = []string{}
		vals.SoWhat = strField(in.Body, "insight")
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.so_what",
			SemanticPath: in.semSlide() + ".insight",
		})
	}
	if insight := strField(in.Body, "insight"); insight != "" && insightsField == "insights" {
		// When an author supplies both evidence bullets and one implication,
		// the scalar insight is the callout, not another copy in the footer.
		duplicate := false
		for _, bullet := range insights {
			if strings.EqualFold(strings.TrimSpace(bullet), strings.TrimSpace(insight)) {
				duplicate = true
				break
			}
		}
		if duplicate && strField(in.Body, "takeaway") == "" && in.Takeaway == insight {
			// The scalar alias is already one of the evidence bullets.
			// Do not reprint it in the footer, even for a multi-bullet list.
			in.Takeaway = ""
		} else if !duplicate {
			vals.SoWhat = insight
			links = append(links, SourceLink{
				RawPath:      in.rawSlide() + ".pattern.values.so_what",
				SemanticPath: in.semSlide() + ".insight",
			})
			if strField(in.Body, "takeaway") == "" && in.Takeaway == insight {
				in.Takeaway = ""
			}
		}
	}
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
	slide.Pattern.Overrides, err = json.Marshal(patterns.ChartInsightsSplitOverrides{TitleSize: 14, BulletSize: 14})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal chart-insight design defaults: %w", err)
	}
	if len(vals.Insights) > 0 {
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.insights",
			SemanticPath: in.semSlide() + "." + insightsField,
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// dropDuplicateTakeaway returns "" when the takeaway is the slide's only
// insight — the same sentence in two places on one slide is not emphasis, it
// reads as a mistake. A takeaway that summarises SEVERAL insights is kept: it
// is saying something the bullets do not.
func dropDuplicateTakeaway(takeaway string, insights []string) string {
	if takeaway == "" || len(insights) != 1 {
		return takeaway
	}
	if strings.EqualFold(strings.TrimSpace(insights[0]), strings.TrimSpace(takeaway)) {
		return ""
	}
	return takeaway
}

// ChartInsightFallbackSlideType returns the native slide_type the density
// fallback compiles to: a two-column slide (chart left in "body", insights
// right in "body_2") when a renderable chart is present, so both survive, or a
// plain content slide when there is no chart. The explain planner consults this
// so its alternative stays in step with compile.
func ChartInsightFallbackSlideType(body map[string]any) string {
	if chartSpec(body) != nil {
		return "two-column"
	}
	return "content"
}

// compileChartFallback renders the title, every insight bullet, the chart (when
// renderable) and the takeaway when the chart-insights-split shape does not fit.
// With a chart it compiles to a two-column slide — chart in "body", insights in
// "body_2" — so the two never contend for one placeholder (a one-content layout
// would resolve body_2 onto body and the chart would bury every insight).
func compileChartFallback(in Input, insights []string, insightsField string) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: ChartInsightFallbackSlideType(in.Body)}
	// Pin the canonical two-column layout rather than leaving the heuristic to
	// infer it. It is resolved per template by ResolveCanonicalLayoutID, and on
	// a template whose scoring quirks pick a divider the insights land in a
	// "Section Number" placeholder and run off the slide
	// (go-slide-creator-ujya). Slides without a chart stay single-column.
	if slide.SlideType == "two-column" {
		slide.LayoutID = "two-column"
	}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	if len(insights) > 0 {
		placeholder := "body"
		if chartSpec(in.Body) != nil {
			placeholder = "body_2"
		}
		idx := appendContent(slide, bulletsContent(placeholder, insights))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + insightsField,
		})
	}

	// A density fallback may change the composition, but it must not turn a
	// chart-backed claim into unsupported prose. Preserve the complete diagram
	// payload as native content and keep the insights alongside it.
	if chart := chartSpec(in.Body); chart != nil {
		if chart.Alt == "" {
			chart.Alt = visualAltText(in)
		}
		idx := appendContent(slide, diagramContent("body", chart))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].diagram_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".chart",
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
