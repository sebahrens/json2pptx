package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// chart-insights-split pattern — left chart panel + right insights column
// ---------------------------------------------------------------------------
//
// Layout:
//   65/35 column split (75/25 when the insights column is sparse).
//     Left  (65%): chart diagram rendered via svggen.
//     Right (35%): 'Key Insights' label + bullet list of takeaways.
//   When chart is omitted: insights column expands to 100% width and the
//   pattern emits a CHART_PLACEHOLDER_EMPTY warning so agents know they
//   chose a chart-bearing pattern without providing a chart.
//
//   So-what extensions (go-slide-creator-pzrs):
//     - headline: a big accent number + label at the top of the insights
//       column (the one figure the audience should remember);
//     - so_what: the shared takeaway band at the bottom of the column;
//     - a series / unit caption above the chart ("Revenue ($M)"), explicit
//       via chart_label or derived from a single series name + unit — the
//       chart otherwise drops the series name because single-series charts
//       render without a legend;
//     - data labels on bar / column / line / area charts with few points
//       (overrides.data_labels forces on / off).

func init() {
	Default().Register(&chartInsightsSplit{})
}

type chartInsightsSplit struct{}

func (cis *chartInsightsSplit) Name() string { return "chart-insights-split" }
func (cis *chartInsightsSplit) Description() string {
	return "Left chart panel + right insights column with full-width fallback when chart is absent"
}
func (cis *chartInsightsSplit) UseWhen() string {
	return "Pair a chart/data visual with 2–6 narrative takeaways on the same slide; prefer comparison-2col when both sides are text, card-grid when there is no single dominant chart"
}
func (cis *chartInsightsSplit) NotWhen() string {
	return "The slide has no insights to narrate (use a plain chart slide) or the focal content is a single big number (use stat-hero)"
}
func (cis *chartInsightsSplit) Version() int      { return 1 }
func (cis *chartInsightsSplit) CellsHint() string { return "1 + 1" }
func (cis *chartInsightsSplit) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "data-display",
		NarrativeRole: []string{"evidence", "conclude"},
		PairsWith:     []string{"kpi-3up", "comparison-2col", "pull-quote"},
		ComposesWith:  []string{"pull-quote", "stat-hero"},
		RoleOnSlide:   nil,
		DensityClass:  "medium",
		AccentWeight:  "normal",
		DataVisual:    true,
	}
}

func (cis *chartInsightsSplit) SupportsCallout() bool        { return true }
func (cis *chartInsightsSplit) SupportsInlineMarkdown() bool { return true }

func (cis *chartInsightsSplit) ExemplarValues() any {
	return &ChartInsightsSplitValues{
		Chart: &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{
						"name":   "Revenue",
						"values": []any{120, 145, 170, 210},
					},
				},
			},
		},
		Unit:          "$M",
		Headline:      &ChartInsightsHeadline{Value: "+75%", Label: "revenue growth Q1 to Q4"},
		InsightsTitle: "Key Insights",
		Insights: []string{
			"Enterprise adoption drove most growth.",
			"Q4 spike reflects the EMEA launch.",
			"Pipeline points to further FY26 gains.",
		},
		SoWhat: "Fund the EMEA sales build-out now to keep the growth rate.",
		Source: "Source: Internal finance (FY25)",
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ChartInsightsSplitValues holds the data for a chart-insights-split pattern.
//
// Chart is optional. When omitted, the pattern renders the insights column
// full-width and emits a CHART_PLACEHOLDER_EMPTY warning so callers can flag
// the missing chart in fit reports.
type ChartInsightsSplitValues struct {
	Chart         *types.DiagramSpec `json:"chart,omitempty"`          // Optional chart/diagram spec rendered in the left panel
	InsightsTitle string             `json:"insights_title,omitempty"` // Label above the bullet list (default "Key Insights")
	Insights      []string           `json:"insights"`                 // 1–6 bullet-list takeaways
	Source        string             `json:"source,omitempty"`         // Optional source / footnote rendered below the left panel

	Headline   *ChartInsightsHeadline `json:"headline,omitempty"`    // Big number + label at the top of the insights column
	SoWhat     string                 `json:"so_what,omitempty"`     // Takeaway band at the bottom of the insights column
	ChartLabel string                 `json:"chart_label,omitempty"` // Caption above the chart (series name / units); derived when omitted
	Unit       string                 `json:"unit,omitempty"`        // Unit appended to the derived caption, e.g. "$M" → "Revenue ($M)"
}

// ChartInsightsHeadline is the headline figure of the insights column.
type ChartInsightsHeadline struct {
	Value string `json:"value"`           // e.g. "+75%", "$210M"
	Label string `json:"label,omitempty"` // e.g. "revenue growth FY25"
}

// Budgets for the so-what extensions.
const (
	cisHeadlineValueMax = 12
	cisHeadlineLabelMax = 60
	cisSoWhatMax        = 160
	cisChartLabelMax    = 60
	cisUnitMax          = 12
	cisDataLabelMaxPts  = 16 // auto data labels only when the chart has at most this many points
	cisSourceRowPt      = 30 // source line row: one 12pt line plus insets
)

// UnmarshalJSON decodes the values and normalizes the {label: value} chart
// shorthand (the form chart_value accepts, e.g. {"Q1": 12, "Q2": 14}) into
// svggen's categories/series payload, preserving the author's key order.
// Without this the raw map reached svggen, which rejected every label as an
// unknown field and aborted generation (go-slide-creator-yzbo).
func (v *ChartInsightsSplitValues) UnmarshalJSON(b []byte) error {
	type plain ChartInsightsSplitValues
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*v = ChartInsightsSplitValues(p)
	if v.Chart == nil {
		return nil
	}
	var raw struct {
		Chart struct {
			Data json.RawMessage `json:"data"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(b, &raw); err == nil {
		v.Chart.NormalizeFlatChartData(types.JSONObjectKeyOrder(raw.Chart.Data))
	}
	return nil
}

// ChartInsightsSplitOverrides contains pattern-level overrides.
type ChartInsightsSplitOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	TitleSize      float64 `json:"title_size,omitempty"`      // Font size for insights_title (default 12)
	BulletSize     float64 `json:"bullet_size,omitempty"`     // Font size for insight bullets (default 12)
	SourceSize     float64 `json:"source_size,omitempty"`     // Font size for source line (default 9)
	ChartWidthPct  float64 `json:"chart_width_pct,omitempty"` // Left-panel width as a percentage of the grid (default 65; clamped 40–80)
	ShowDivider    *bool   `json:"show_divider,omitempty"`    // When false, omit the thin vertical accent divider (default true)
	DataLabels     *bool   `json:"data_labels,omitempty"`     // Force value labels on / off (default: on for bar/column/line/area charts with ≤16 points)
	HeadlineSize   float64 `json:"headline_size,omitempty"`   // Headline value font size (default 32)
	// TakeawayEmphasis styles the so-what band: "" (accent bar only),
	// "subtle" (5% neutral tint) or "strong" (solid accent).
	TakeawayEmphasis string `json:"takeaway_emphasis,omitempty"`
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (cis *chartInsightsSplit) NewValues() any       { return &ChartInsightsSplitValues{} }
func (cis *chartInsightsSplit) NewOverrides() any    { return &ChartInsightsSplitOverrides{} }
func (cis *chartInsightsSplit) NewCellOverride() any { return nil }

func (cis *chartInsightsSplit) Schema() *Schema {
	chartSchema := ObjectSchema(
		map[string]*Schema{
			"type":  StringSchema(60).WithDescription("Diagram type (bar_chart, line_chart, pie_chart, etc.) — passed directly to svggen"),
			"title": StringSchema(120).WithDescription("Optional chart title"),
			"data":  ObjectSchema(map[string]*Schema{}, nil).WithDescription("Diagram-specific data payload (categories + series, or type-specific shape). bar_chart also takes highlight: bars painted in accent1 (0-based category indices or names) while the rest are neutral dk1 at 38%; omit to accent the last bar of a time series, otherwise the largest; [] accents none"),
		},
		[]string{"type", "data"},
	).WithDescription("Optional chart/diagram rendered in the left panel; omit to render insights full-width")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"chart":          chartSchema,
			"insights_title": StringSchema(40).WithDescription("Label above the bullet list (default \"Key Insights\")").WithDefault("Key Insights"),
			"insights":       ArraySchema(StringSchema(160), 0, 6).WithDescription("Up to 6 narrative takeaway bullets; may be empty when so_what carries the sole insight. With a chart, average about 160 characters per insight up to 4 (122 at 5, 82 at 6); with a chart and a headline or so-what, 122/81/41/40/40 at 2/3/4/5/6. Break long unbroken runs"),
			"source":         StringSchema(120).WithDescription("Optional source/footnote below the chart; target about 91 characters with one insight and no headline/callout, 77 when the right column has more content, and none beside six insights plus a headline or so-what"),
			"headline": ObjectSchema(map[string]*Schema{
				"value": StringSchema(cisHeadlineValueMax).WithDescription("Headline figure, e.g. \"+75%\""),
				"label": StringSchema(cisHeadlineLabelMax).WithDescription("What the figure means; about 41 readable characters"),
			}, []string{"value"}).WithAdditionalProperties(false).WithDescription("Big accent number at the top of the insights column"),
			"so_what":     StringSchema(cisSoWhatMax).WithDescription("Implication / recommendation shown as the takeaway band under the insights (flush accent bar, bold dk1 text, no box)"),
			"chart_label": StringSchema(cisChartLabelMax).WithDescription("Caption above the chart (series + units); defaults to the single series name + unit"),
			"unit":        StringSchema(cisUnitMax).WithDescription("Unit for the derived chart caption, e.g. \"$M\""),
		},
		[]string{"insights"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":            StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent":   EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"title_size":        NumberSchema(6, 40).WithDescription("Font size for insights_title in points (default 12)"),
			"bullet_size":       NumberSchema(6, 40).WithDescription("Font size for insight bullets in points (default 12)"),
			"source_size":       NumberSchema(6, 24).WithDescription("Font size for source line in points (default 9)"),
			"chart_width_pct":   NumberSchema(40, 80).WithDescription("Width of the chart panel as a percentage of the grid (default 65)").WithDefault(65),
			"show_divider":      BooleanSchema().WithDescription("Render a thin vertical accent divider between panels (default true)"),
			"data_labels":       BooleanSchema().WithDescription("Value labels on the chart (default on for bar/column/line/area charts with ≤16 points)"),
			"headline_size":     NumberSchema(18, 60).WithDescription("Headline value font size in points (default 32)"),
			"takeaway_emphasis": TakeawayEmphasisSchema(),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":    valuesSchema,
			"overrides": overridesSchema,
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Left chart panel + right insights column with full-width fallback when chart is absent")
}

func (cis *chartInsightsSplit) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*ChartInsightsSplitValues)
	if !ok || v == nil {
		return fmt.Errorf("chart-insights-split: values must be *ChartInsightsSplitValues, got %T", values)
	}

	const name = "chart-insights-split"
	var errs []error

	if len(v.Insights) == 0 && strings.TrimSpace(v.SoWhat) == "" {
		errs = append(errs, errRequired(name, "values.insights or values.so_what"))
	}
	if len(v.Insights) > 6 {
		errs = append(errs, newValidationError(name, "values.insights", ErrCodeMaxItems,
			fmt.Sprintf("chart-insights-split: values.insights must contain at most 6 bullets, got %d", len(v.Insights)),
			ReduceItemsFix("values.insights", 6)))
	}
	for i, b := range v.Insights {
		path := fmt.Sprintf("values.insights[%d]", i)
		if b == "" {
			errs = append(errs, errRequired(name, path))
			continue
		}
		if runeLen(b) > 160 {
			errs = append(errs, errMaxLength(name, path, 160, runeLen(b)))
		}
	}

	if v.InsightsTitle != "" && runeLen(v.InsightsTitle) > 40 {
		errs = append(errs, errMaxLength(name, "values.insights_title", 40, runeLen(v.InsightsTitle)))
	}
	if v.Source != "" && runeLen(v.Source) > 120 {
		errs = append(errs, errMaxLength(name, "values.source", 120, runeLen(v.Source)))
	}

	errs = append(errs, validateChartInsightsExtras(v)...)
	if ovr, ok := overrides.(*ChartInsightsSplitOverrides); ok && ovr != nil {
		if err := validateTakeawayEmphasis(name, ovr.TakeawayEmphasis); err != nil {
			errs = append(errs, err)
		}
	}

	// Validate chart: if present, must declare a type and data.
	if v.Chart != nil {
		if v.Chart.Type == "" {
			errs = append(errs, errRequired(name, "values.chart.type"))
		}
		if len(v.Chart.Data) == 0 {
			errs = append(errs, errRequired(name, "values.chart.data"))
		}
	}

	return errors.Join(errs...)
}

// PostExpandWarnings emits a CHART_PLACEHOLDER_EMPTY warning when the pattern
// is expanded without a chart spec, so fit_report can surface a finding that
// tells the agent to either provide a chart or switch to an insights-only
// pattern. The warning string follows the structured convention used by
// compose warnings: "<CODE>: <message>".
func (cis *chartInsightsSplit) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ChartInsightsSplitValues)
	if !ok || v == nil {
		return nil
	}
	extras := v.Headline != nil || strings.TrimSpace(v.SoWhat) != ""
	warnings := cisInsightWarnings(v, extras)
	if w := cisColumnAreaWarning(ctx, v, overrides); w != "" {
		warnings = append(warnings, w)
	}
	if h := v.Headline; h != nil && runeLen(h.Label) > cisHeadlineLabelBudget {
		warnings = append(warnings, fmt.Sprintf("%s: chart-insights-split headline.label has %d characters; the headline holds about %d readable label characters — shorten the label", ErrCodeBodyTooLong, runeLen(h.Label), cisHeadlineLabelBudget))
	}
	if v.Chart != nil {
		budget := cisSourceBudget(len(v.Insights), extras)
		switch {
		case budget == 0 && strings.TrimSpace(v.Source) != "":
			warnings = append(warnings, fmt.Sprintf("%s: chart-insights-split source has %d characters; with %d insights plus a headline or so-what the column leaves no readable source line — move the source to slide notes or use fewer insights", ErrCodeBodyTooLong, runeLen(v.Source), len(v.Insights)))
		case budget > 0 && runeLen(v.Source) > budget:
			warnings = append(warnings, fmt.Sprintf("%s: chart-insights-split source has %d characters; the chart source line holds about %d readable characters with this insights column — shorten the source or move detail to slide notes", ErrCodeBodyTooLong, runeLen(v.Source), budget))
		}
		return warnings
	}
	return append(warnings,
		ErrCodeChartPlaceholderEmpty+
			": chart-insights-split rendered insights-only; provide a chart spec to fill the left panel")
}

// Insight budgets by bullet count (index 0 = one insight), measured against
// the written size (no run stored below its role floor) on every shipped
// template with every insight at the same length and every shape keeping the
// uniform 0.5 cm text margin (go-slide-creator-n1muf). Indexed
// [chart][extras]; extras = headline or so-what present.
var (
	cisInsightWordBudgets = [2][2][6]int{
		{{160, 160, 160, 160, 160, 160}, {160, 160, 160, 136, 132, 132}},
		{{160, 160, 160, 160, 122, 82}, {160, 122, 81, 41, 40, 40}},
	}
	cisInsightUnbrokenBudgets = [2][2][6]int{
		{{160, 160, 160, 160, 145, 72}, {160, 149, 72, 71, 68, 68}},
		{{160, 160, 104, 75, 50, 25}, {160, 50, 24, 22, 21, 21}},
	}
)

// cisColumnAreaWarning reports a stacked headline / so-what column whose
// insights do not fit this layout's content area even at the compact sizes
// and narrowest chart. The character budgets are measured against the
// shipped templates' full content areas; a narrow or short area (or a
// takeaway bar) holds fewer lines, and the writer would shrink the insights
// below the 12pt floor (go-slide-creator-bzh34).
func cisColumnAreaWarning(ctx ExpandContext, v *ChartInsightsSplitValues, overrides any) string {
	if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 || len(v.Insights) == 0 {
		return "" // no measured content area: the budgets above apply
	}
	ovr, _ := overrides.(*ChartInsightsSplitOverrides)
	if ovr == nil {
		ovr = &ChartInsightsSplitOverrides{}
	}
	title := v.InsightsTitle
	if title == "" {
		title = "Key Insights"
	}
	panel := buildInsightsPanel(title, v.Insights, "accent1", ResolveSize(ovr.TitleSize, 12.0), ResolveSize(ovr.BulletSize, 12.0))
	chartPct := clampPct(ovr.ChartWidthPct, sparseInsightsChartPct(v), 40.0, 80.0)
	_, _, short := buildInsightsColumn(ctx, v, ovr, panel, "accent1", chartPct)
	if short <= 0.5 {
		return ""
	}
	return fmt.Sprintf("%s: chart-insights-split insights column is about %.0fpt taller than this layout's content area holds beside the headline / so-what at readable sizes — drop the headline, so_what or source, use fewer or shorter insights, or drop the slide takeaway", ErrCodeBodyTooLong, math.Ceil(short))
}

// cisHeadlineLabelBudget is the readable headline label length.
const cisHeadlineLabelBudget = 41

// cisSourceBudget is the readable chart source length for an insight count:
// 91 beside a single insight with no headline or so-what, 77 otherwise, and no
// source line at all beside six insights plus a headline or so-what.
func cisSourceBudget(insights int, extras bool) int {
	switch {
	case insights == 1 && !extras:
		return 91
	case insights >= 6 && extras:
		return 0
	}
	return 77
}

func cisInsightWarnings(v *ChartInsightsSplitValues, extras bool) []string {
	n := len(v.Insights)
	if n < 1 || n > 6 {
		return nil
	}
	c, e := 0, 0
	if v.Chart != nil {
		c = 1
	}
	if extras {
		e = 1
	}
	words, wide := cisInsightWordBudgets[c][e][n-1], cisInsightUnbrokenBudgets[c][e][n-1]
	var warnings []string
	total := 0
	for i, insight := range v.Insights {
		total += runeLen(insight)
		longest := 0
		for _, word := range strings.Fields(insight) {
			longest = max(longest, runeLen(word))
		}
		if longest > wide {
			warnings = append(warnings, fmt.Sprintf("%s: chart-insights-split insights[%d] contains a %d-character unbroken word; %d insights hold about %d wide characters each in this layout — add word breaks or shorten the insight", ErrCodeBodyTooLong, i, longest, n, wide))
		}
	}
	if total > words*n {
		warnings = append(warnings, fmt.Sprintf("%s: chart-insights-split insights use %d characters across %d bullets; this layout holds about %d characters per insight on average — shorten insights or use fewer", ErrCodeBodyTooLong, total, n, words))
	}
	return warnings
}

func (cis *chartInsightsSplit) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*ChartInsightsSplitValues)
	if !ok {
		return nil, fmt.Errorf("chart-insights-split: values must be *ChartInsightsSplitValues, got %T", values)
	}
	ovr := &ChartInsightsSplitOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ChartInsightsSplitOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("chart-insights-split: overrides must be *ChartInsightsSplitOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	titleSize := ResolveSize(ovr.TitleSize, 12.0)
	bulletSize := ResolveSize(ovr.BulletSize, 12.0)
	sourceSize := ResolveSize(ovr.SourceSize, SourceNoteSizePt)

	insightsTitle := v.InsightsTitle
	if insightsTitle == "" {
		insightsTitle = "Key Insights"
	}

	// Build the insights panel, or a callout-only panel for a scalar insight.
	var insightsCell *jsonschema.GridCellInput
	if len(v.Insights) > 0 {
		insightsCell = buildInsightsPanel(insightsTitle, v.Insights, accent, titleSize, bulletSize)
	}
	// Compute the chart panel width as a fraction of the grid. The default
	// widens when the insights column has little to hold: a 35% column carrying
	// one short bullet leaves a large empty block under it while the chart is
	// squeezed into 55% of the slide (go-slide-creator-pyxn). A stacked
	// headline / so-what column may narrow the chart instead so its insights
	// fit this template's content area (go-slide-creator-bzh34).
	chartPct := clampPct(ovr.ChartWidthPct, sparseInsightsChartPct(v), 40.0, 80.0)
	insightsCell, chartPct, _ = buildInsightsColumn(ctx, v, ovr, insightsCell, accent, chartPct)

	// Full-width fallback: no chart → single insights cell spanning the grid.
	if v.Chart == nil {
		grid := &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(`1`),
			Gap:     8,
			Rows: []jsonschema.GridRowInput{
				{Cells: []*jsonschema.GridCellInput{insightsCell}},
			},
		}
		return grid, nil
	}

	insightsPct := 100.0 - chartPct

	// Chart panel: a Diagram cell rendered via svggen, with value labels and
	// a series / unit caption above it when available.
	chartCell := &jsonschema.GridCellInput{
		Diagram: chartWithDataLabels(v.Chart, ovr.DataLabels),
	}
	if label := chartCaption(v); label != "" {
		chartCell = buildChartWithCaption(ctx, chartCell, label)
	}

	// Optional thin vertical divider rendered as an accent bar on the insights
	// cell. A stacked headline / so-what column is already visually anchored
	// by its accent figure and callout bar (and accent bars on nested-grid
	// cells are not drawn), so the divider applies to the plain panel only.
	showDivider := ovr.ShowDivider == nil || *ovr.ShowDivider
	if showDivider && insightsCell.Grid == nil {
		insightsCell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    1,
		}
	}

	// Two-column row hosting chart (left) and insights (right).
	colsJSON, _ := json.Marshal([]float64{chartPct, insightsPct})

	rows := []jsonschema.GridRowInput{
		{
			Cells: []*jsonschema.GridCellInput{chartCell, insightsCell},
		},
	}

	// Optional source row below the chart panel. Implemented as a second row
	// whose left cell carries the source line and whose right cell is an empty
	// transparent placeholder — keeping the 65/35 column ratio intact.
	if v.Source != "" {
		sourceCell := buildSourceCell(v.Source, sourceSize)
		spacer := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
			},
		}
		// Pinned in points: the source renders at the 12pt shape-text floor,
		// which an 8%-of-grid row could not hold without autofit shrinking it.
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: cisSourceRowPt,
			MaxHeight: cisSourceRowPt,
			Cells:     []*jsonschema.GridCellInput{sourceCell, spacer},
		})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     8,
		Rows:    rows,
	}

	return grid, nil
}

// Vertical rhythm (in points) for the insights panel. The title reads as a
// header with extra separation below it, and each bullet gets breathing room
// so the right column does not render as a dense, hard-to-scan block.
const (
	insightsTitleSpaceAfterPt  = 8.0
	insightsBulletSpaceAfterPt = 6.0
)

// buildInsightsPanel constructs the right-column cell containing the
// accent-coloured title label and a bullet list of insights.
func buildInsightsPanel(title string, insights []string, accent string, titleSize, bulletSize float64) *jsonschema.GridCellInput {
	paras := []chartInsightsParagraph{
		{Content: title, Size: titleSize, Bold: true, Color: accent, Align: "l", SpaceAfter: insightsTitleSpaceAfterPt},
	}
	for i, ins := range insights {
		// Omit the trailing gap after the last bullet so the block stays
		// top-anchored without padding the panel bottom.
		spaceAfter := insightsBulletSpaceAfterPt
		if i == len(insights)-1 {
			spaceAfter = 0
		}
		paras = append(paras, chartInsightsParagraph{
			Content:    "• " + pptx.ConvertMarkdownEmphasis(ins),
			Size:       bulletSize,
			Color:      "dk1",
			Align:      "l",
			SpaceAfter: spaceAfter,
		})
	}

	textObj := chartInsightsText{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	textJSON, _ := json.Marshal(textObj)

	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}
}

// buildSourceCell constructs the small italic source line shown below the
// chart panel.
func buildSourceCell(source string, sourceSize float64) *jsonschema.GridCellInput {
	textObj := chartInsightsText{
		Paragraphs: []chartInsightsParagraph{
			{Content: SourceNoteText(source), Size: sourceSize, Italic: true, Color: SourceNoteScheme, Align: SourceNoteAlign},
		},
		Align:         SourceNoteAlign,
		VerticalAlign: "ctr",
	}
	textJSON, _ := json.Marshal(textObj)
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}
}

// cloneDiagramSpec returns a shallow copy of d safe for embedding in a grid
// cell. The Data map is reused (chart pipeline does not mutate it).
func cloneDiagramSpec(d *types.DiagramSpec) *types.DiagramSpec {
	if d == nil {
		return nil
	}
	cp := *d
	return &cp
}

// clampPct clamps v into [min, max], returning fallback when v <= 0.
func clampPct(v, fallback, min, max float64) float64 {
	if v <= 0 {
		v = fallback
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

// chartInsightsParagraph is a text paragraph for JSON marshalling.
type chartInsightsParagraph struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Italic     bool    `json:"italic,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
}

// chartInsightsText is the text object for JSON marshalling.
type chartInsightsText struct {
	Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
	// InsetTop, in points, is the uniform top margin plus a baseline nudge. It
	// is what lets two top-anchored cells at different type sizes share a
	// first baseline (go-slide-creator-kol0); it never goes below the uniform
	// margin. Zero (omitted) keeps the uniform margin.
	InsetTop float64 `json:"inset_top,omitempty"`
}

// chartInsightsDefaultPct is the standard chart panel width, and
// chartInsightsWidePct the width used when the insights column is sparse.
const (
	chartInsightsDefaultPct = 65.0
	chartInsightsWidePct    = 75.0
	// chartInsightsSparseBullets / Runes bound what counts as sparse: up to two
	// bullets of ordinary length, with none of the extras (headline, so-what)
	// that give the column a reason to be wide.
	chartInsightsSparseBullets = 2
	chartInsightsSparseRunes   = 140
)

// sparseInsightsChartPct returns the default chart width for these values: the
// standard split, or a wider chart when the insights column would be mostly
// empty. An explicit chart_width_pct override still wins — this only chooses
// the default.
func sparseInsightsChartPct(v *ChartInsightsSplitValues) float64 {
	if v == nil || v.Headline != nil || strings.TrimSpace(v.SoWhat) != "" {
		return chartInsightsDefaultPct
	}
	if len(v.Insights) == 0 || len(v.Insights) > chartInsightsSparseBullets {
		return chartInsightsDefaultPct
	}
	total := 0
	for _, b := range v.Insights {
		total += runeLen(b)
	}
	if total > chartInsightsSparseRunes {
		return chartInsightsDefaultPct
	}
	return chartInsightsWidePct
}

// validateChartInsightsExtras checks the so-what extensions.
func validateChartInsightsExtras(v *ChartInsightsSplitValues) []error {
	const name = "chart-insights-split"
	var errs []error
	if h := v.Headline; h != nil {
		switch {
		case strings.TrimSpace(h.Value) == "":
			errs = append(errs, errRequired(name, "values.headline.value"))
		case runeLen(h.Value) > cisHeadlineValueMax:
			errs = append(errs, errMaxLength(name, "values.headline.value", cisHeadlineValueMax, runeLen(h.Value)))
		}
		if runeLen(h.Label) > cisHeadlineLabelMax {
			errs = append(errs, errMaxLength(name, "values.headline.label", cisHeadlineLabelMax, runeLen(h.Label)))
		}
	}
	for _, f := range []struct {
		path string
		val  string
		max  int
	}{
		{"values.so_what", v.SoWhat, cisSoWhatMax},
		{"values.chart_label", v.ChartLabel, cisChartLabelMax},
		{"values.unit", v.Unit, cisUnitMax},
	} {
		if runeLen(f.val) > f.max {
			errs = append(errs, errMaxLength(name, f.path, f.max, runeLen(f.val)))
		}
	}
	return errs
}

// chartCaption returns the caption shown above the chart: the explicit
// chart_label, else — when the chart has no title of its own — the single
// series name plus unit ("Revenue ($M)"), or the unit alone for multi-series
// charts (their legend already names the series).
func chartCaption(v *ChartInsightsSplitValues) string {
	if v.ChartLabel != "" {
		return v.ChartLabel
	}
	if v.Chart == nil || v.Chart.Title != "" {
		return ""
	}
	names := chartSeriesNames(v.Chart)
	unit := strings.TrimSpace(v.Unit)
	switch {
	case len(names) == 1 && names[0] != "":
		if unit != "" && !strings.Contains(names[0], unit) {
			return names[0] + " (" + unit + ")"
		}
		return names[0]
	case unit != "":
		return "Values in " + unit
	}
	return ""
}

// chartSeriesNames returns the series names of a categories/series payload.
func chartSeriesNames(d *types.DiagramSpec) []string {
	raw, ok := d.Data["series"].([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for _, s := range raw {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		names = append(names, strings.TrimSpace(name))
	}
	return names
}

// dataLabelChartTypes are the chart types whose value labels read well.
var dataLabelChartTypes = map[string]bool{
	"bar_chart": true, "column_chart": true, "stacked_bar_chart": true, "grouped_bar_chart": true,
	"horizontal_bar_chart": true, "line_chart": true, "area_chart": true, "bar": true, "line": true,
}

// chartWithDataLabels returns a copy of the chart with value labels turned on
// (Style.ShowValues) when forced, or by default for label-friendly chart types
// with few points. The caller's spec and style are never mutated.
func chartWithDataLabels(d *types.DiagramSpec, force *bool) *types.DiagramSpec {
	cp := cloneDiagramSpec(d)
	if cp == nil {
		return nil
	}
	on := dataLabelsByDefault(cp)
	if raw, explicit := cp.Data["data_labels"]; explicit {
		// A bool is the author's on/off switch; an object means the author
		// configured labels in the payload, which svggen applies itself
		// (go-slide-creator-csclk.10).
		b, isBool := raw.(bool)
		on = isBool && b
	}
	if force != nil {
		on = *force
	}
	style := types.DiagramStyle{}
	if cp.Style != nil {
		style = *cp.Style
	}
	style.ShowValues = on || style.ShowValues && force == nil
	cp.Style = &style
	return cp
}

// dataLabelsByDefault reports whether value labels read cleanly: bar-type
// charts with at most cisDataLabelMaxPts points, and single-series line /
// area charts with at most 12 points (labels of crossing series collide).
func dataLabelsByDefault(d *types.DiagramSpec) bool {
	if !dataLabelChartTypes[d.Type] {
		return false
	}
	points := chartPointCount(d)
	switch d.Type {
	case "line_chart", "area_chart", "line":
		return len(chartSeriesNames(d)) == 1 && points <= 12
	default:
		return points <= cisDataLabelMaxPts
	}
}

// chartPointCount counts series values (categories × series).
func chartPointCount(d *types.DiagramSpec) int {
	raw, ok := d.Data["series"].([]any)
	if !ok {
		return 0
	}
	n := 0
	for _, s := range raw {
		if m, ok := s.(map[string]any); ok {
			if vals, ok := m["values"].([]any); ok {
				n += len(vals)
			}
		}
	}
	return n
}

// Heights (points) of the caption row above the chart.
const cisCaptionPt = 26.0

// buildChartWithCaption stacks a small bold caption above the chart cell.
func buildChartWithCaption(ctx ExpandContext, chart *jsonschema.GridCellInput, label string) *jsonschema.GridCellInput {
	textJSON, _ := json.Marshal(chartInsightsText{
		Paragraphs:    []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(label), Size: 12, Bold: true, Color: inkOnLight(ctx, "dk2", 4.5), Align: "l"}},
		Align:         "l",
		VerticalAlign: "b",
	})
	_, areaH := sizingAreaPt(ctx)
	capPct := pctOf(cisCaptionPt, areaH)
	return &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		RowGap:  2,
		Rows: []jsonschema.GridRowInput{
			{Height: capPct, Cells: []*jsonschema.GridCellInput{{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: textJSON}}}},
			{Height: 100 - capPct, Cells: []*jsonschema.GridCellInput{chart}},
		},
	}}
}

// cisColGapPt is the gap between the chart and the insights column, and
// between the top row and the source row.
const (
	cisColGapPt = 8.0
	// cisMinChartPct is the narrowest default chart panel a stacked column
	// may take room from when its insights would not fit.
	cisMinChartPct = 55.0
)

// buildInsightsColumn returns the insights cell unchanged when neither a
// headline nor a so-what is set; otherwise it stacks [headline] / insights /
// [so-what] in a nested grid. The headline and so-what rows are pinned at the
// height the writer needs for them (writtenFitHeightPt) and the insights
// panel takes the rest. When the insights would not fit this template's
// content area, the headline and callout step down to their compact sizes
// and then, unless chart_width_pct pins it, the chart panel narrows (to
// cisMinChartPct). Sized from percentages of an estimated area, the column
// was written with its insights autofit below the 12pt floor on templates
// with a shorter content area (go-slide-creator-bzh34). It returns the cell,
// the chart panel width it was sized for and how many points the column is
// short of (0 when it fits); when no candidate fits, the one closest to
// fitting is used.
func buildInsightsColumn(ctx ExpandContext, v *ChartInsightsSplitValues, ovr *ChartInsightsSplitOverrides, insights *jsonschema.GridCellInput, accent string, chartPct float64) (*jsonschema.GridCellInput, float64, float64) {
	hasHeadline := v.Headline != nil && strings.TrimSpace(v.Headline.Value) != ""
	hasSoWhat := strings.TrimSpace(v.SoWhat) != ""
	if insights == nil && hasSoWhat && !hasHeadline {
		// Callout-only semantic slides give the implication the whole right
		// panel instead of leaving an empty "Key Insights" row above it. It is
		// still the takeaway band, top-anchored at its measured height. The
		// panel cell hosts the band grid directly (one sub-grid inset).
		spec := cisTakeaway(v, accent, ovr.TakeawayEmphasis)
		bandW := cisInsightsColumnPt(ctx, v, ovr) - 2*SubGridInsetPt
		grid := TakeawayGrid(ctx, spec, bandW, 0, TakeawayBandHeightPt(ctx, spec, bandW))
		grid.VerticalAlign = "top"
		return &jsonschema.GridCellInput{Grid: grid}, chartPct, 0
	}
	if !hasHeadline && !hasSoWhat {
		return insights, chartPct, 0
	}
	// Candidate layouts in order of preference: the authored sizes at the
	// default width, then the compact sizes at narrowing chart widths.
	type candidate struct {
		pct     float64
		compact bool
	}
	candidates := []candidate{{chartPct, false}, {chartPct, true}}
	if v.Chart != nil && ovr.ChartWidthPct <= 0 {
		for p := chartPct - 5; p >= cisMinChartPct; p -= 5 {
			candidates = append(candidates, candidate{p, true})
		}
	}
	var best *jsonschema.GridCellInput
	bestPct, bestShort := chartPct, math.Inf(1)
	for _, c := range candidates {
		cell, short := stackInsightsColumn(ctx, v, ovr, insights, accent, c.pct, c.compact)
		if short <= 0 {
			return cell, c.pct, 0
		}
		if short < bestShort {
			best, bestPct, bestShort = cell, c.pct, short
		}
	}
	return best, bestPct, bestShort
}

// stackInsightsColumn builds the stacked column for one chart width and type
// step, reporting how many points the insights panel is short of beside the
// pinned rows (0 or less when it fits).
func stackInsightsColumn(ctx ExpandContext, v *ChartInsightsSplitValues, ovr *ChartInsightsSplitOverrides, insights *jsonschema.GridCellInput, accent string, chartPct float64, compact bool) (*jsonschema.GridCellInput, float64) {
	hasHeadline := v.Headline != nil && strings.TrimSpace(v.Headline.Value) != ""
	hasSoWhat := strings.TrimSpace(v.SoWhat) != ""
	areaW, areaH := sizingAreaPt(ctx)
	// The column's inner frame: the nested grid resolves inside the cell less
	// the sub-grid inset on every side.
	colW := (areaW-cisColGapPt)*(100-chartPct)/100 - 2*SubGridInsetPt
	if v.Chart == nil {
		colW = areaW - 2*SubGridInsetPt
	}
	colH := areaH - 2*SubGridInsetPt
	if v.Source != "" {
		colH -= cisSourceRowPt + cisColGapPt // source row + row gap
	}

	var rows []jsonschema.GridRowInput
	used := 0.0
	if hasHeadline {
		size := ResolveSize(ovr.HeadlineSize, 32)
		if ovr.HeadlineSize == 0 && (compact || len(v.Insights) >= 5) {
			size = 26
		}
		paras := []chartInsightsParagraph{{Content: v.Headline.Value, Size: size, Bold: true, Color: inkOnLight(ctx, accent, 3.0), Align: "l"}}
		sized := []sizedPara{{text: v.Headline.Value, sizePt: size, bold: true}}
		if v.Headline.Label != "" {
			paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(v.Headline.Label), Size: 12, Color: "dk1", Align: "l"})
			sized = append(sized, sizedPara{text: v.Headline.Label, sizePt: 12})
		}
		textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
		h := math.Max(sizedBlockHeightPt(ctx, sized, colW), writtenFitHeightPt(textJSON, colW, 0))
		rows = append(rows, jsonschema.GridRowInput{MinHeight: h, MaxHeight: h, Cells: []*jsonschema.GridCellInput{{
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: textJSON},
		}}})
		used += h
	}
	var soWhatRow *jsonschema.GridRowInput
	if hasSoWhat {
		// The so-what is the shared takeaway band (go-slide-creator-7b5o6):
		// a flush accent bar and bold dk1 text, no tinted box. Its row is
		// floored at the band's measured height in points; the compact step
		// sets it at 12pt beside a chart.
		spec := cisTakeaway(v, accent, ovr.TakeawayEmphasis)
		if compact && v.Chart != nil {
			spec.SizePt = 12
		}
		h := TakeawayRowHeightPt(ctx, spec, colW, cisColumnRowGapPt)
		row := TakeawayRow(ctx, spec, 1, colW, cisColumnRowGapPt)
		soWhatRow = &row
		used += h
	}
	need := 0.0
	if insights != nil {
		rows = append(rows, jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{insights}})
		if insights.Shape != nil {
			need = writtenFitHeightPt(insights.Shape.Text, colW, 0)
		}
	}
	if soWhatRow != nil {
		rows = append(rows, *soWhatRow)
	}
	short := used + need + cisColumnRowGapPt*float64(len(rows)-1) - colH
	return &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		RowGap:  cisColumnRowGapPt,
		Rows:    rows,
	}}, short
}

// cisInsightsColumnPt is the insights column's width: its share of the
// content width after the 8pt gap between the chart and the column (the
// whole width when there is no chart).
func cisInsightsColumnPt(ctx ExpandContext, v *ChartInsightsSplitValues, ovr *ChartInsightsSplitOverrides) float64 {
	areaW, _ := sizingAreaPt(ctx)
	if v.Chart == nil {
		return areaW
	}
	return (areaW - 8) * (100 - clampPct(ovr.ChartWidthPct, sparseInsightsChartPct(v), 40.0, 80.0)) / 100
}

// cisColumnRowGapPt is the row gap of the stacked insights column.
const cisColumnRowGapPt = 6.0

// cisTakeaway is the so-what as a takeaway band spec. The band sits in the
// narrow insights column, so it steps down with a dense column the way the
// bullets do; everything else is the shared component.
func cisTakeaway(v *ChartInsightsSplitValues, accent, emphasis string) TakeawaySpec {
	size := TakeawaySizePt
	if v.Chart != nil {
		size = 13
		if len(v.Insights) >= 5 {
			size = 12
		}
	}
	return TakeawaySpec{Text: pptx.ConvertMarkdownEmphasis(v.SoWhat), Accent: accent, Emphasis: emphasis, SizePt: size}
}
