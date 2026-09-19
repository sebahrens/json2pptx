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
//     - so_what: a tinted, accent-barred callout at the bottom of the column;
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
			"Enterprise adoption drove most of the growth.",
			"Q4 spike reflects the EMEA market launch.",
			"Pipeline coverage points to further gains in FY26.",
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
	SoWhat     string                 `json:"so_what,omitempty"`     // Tinted "So what" callout at the bottom of the insights column
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
			"data":  ObjectSchema(map[string]*Schema{}, nil).WithDescription("Diagram-specific data payload (categories + series, or type-specific shape)"),
		},
		[]string{"type", "data"},
	).WithDescription("Optional chart/diagram rendered in the left panel; omit to render insights full-width")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"chart":          chartSchema,
			"insights_title": StringSchema(40).WithDescription("Label above the bullet list (default \"Key Insights\")").WithDefault("Key Insights"),
			"insights":       ArraySchema(StringSchema(160), 1, 6).WithDescription("1–6 narrative takeaway bullets"),
			"source":         StringSchema(120).WithDescription("Optional source/footnote rendered below the left panel"),
			"headline": ObjectSchema(map[string]*Schema{
				"value": StringSchema(cisHeadlineValueMax).WithDescription("Headline figure, e.g. \"+75%\""),
				"label": StringSchema(cisHeadlineLabelMax).WithDescription("What the figure means"),
			}, []string{"value"}).WithAdditionalProperties(false).WithDescription("Big accent number at the top of the insights column"),
			"so_what":     StringSchema(cisSoWhatMax).WithDescription("Implication / recommendation shown as a tinted callout under the insights"),
			"chart_label": StringSchema(cisChartLabelMax).WithDescription("Caption above the chart (series + units); defaults to the single series name + unit"),
			"unit":        StringSchema(cisUnitMax).WithDescription("Unit for the derived chart caption, e.g. \"$M\""),
		},
		[]string{"insights"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"title_size":      NumberSchema(6, 40).WithDescription("Font size for insights_title in points (default 12)"),
			"bullet_size":     NumberSchema(6, 40).WithDescription("Font size for insight bullets in points (default 12)"),
			"source_size":     NumberSchema(6, 24).WithDescription("Font size for source line in points (default 9)"),
			"chart_width_pct": NumberSchema(40, 80).WithDescription("Width of the chart panel as a percentage of the grid (default 65)").WithDefault(65),
			"show_divider":    BooleanSchema().WithDescription("Render a thin vertical accent divider between panels (default true)"),
			"data_labels":     BooleanSchema().WithDescription("Value labels on the chart (default on for bar/column/line/area charts with ≤16 points)"),
			"headline_size":   NumberSchema(18, 60).WithDescription("Headline value font size in points (default 32)"),
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

	if len(v.Insights) == 0 {
		errs = append(errs, errRequired(name, "values.insights"))
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
	if v.Chart != nil {
		return nil
	}
	return []string{
		ErrCodeChartPlaceholderEmpty +
			": chart-insights-split rendered insights-only; provide a chart spec to fill the left panel",
	}
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
	sourceSize := ResolveSize(ovr.SourceSize, 9.0)

	insightsTitle := v.InsightsTitle
	if insightsTitle == "" {
		insightsTitle = "Key Insights"
	}

	// Build the insights panel (always present): the plain text cell, or a
	// stacked headline / insights / so-what column when the extras are used.
	insightsCell := buildInsightsPanel(insightsTitle, v.Insights, accent, titleSize, bulletSize)
	insightsCell = buildInsightsColumn(ctx, v, ovr, insightsCell, accent)

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

	// Compute the chart panel width as a fraction of the grid. The default
	// widens when the insights column has little to hold: a 35% column carrying
	// one short bullet leaves a large empty block under it while the chart is
	// squeezed into 55% of the slide (go-slide-creator-pyxn).
	chartPct := clampPct(ovr.ChartWidthPct, sparseInsightsChartPct(v), 40.0, 80.0)
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
			{Content: source, Size: sourceSize, Italic: true, Color: "dk1", Align: "l"},
		},
		Align:         "l",
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
	// InsetTop nudges the first line down, in points. It is what lets two
	// top-anchored cells at different type sizes share a first baseline
	// (go-slide-creator-kol0).
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
	if _, explicit := cp.Data["data_labels"]; explicit {
		on = false // the author configured labels in the payload
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

// buildInsightsColumn returns the insights cell unchanged when neither a
// headline nor a so-what is set; otherwise it stacks [headline] / insights /
// [so-what] in a nested grid sized from the measured headline and callout.
func buildInsightsColumn(ctx ExpandContext, v *ChartInsightsSplitValues, ovr *ChartInsightsSplitOverrides, insights *jsonschema.GridCellInput, accent string) *jsonschema.GridCellInput {
	hasHeadline := v.Headline != nil && strings.TrimSpace(v.Headline.Value) != ""
	hasSoWhat := strings.TrimSpace(v.SoWhat) != ""
	if !hasHeadline && !hasSoWhat {
		return insights
	}
	areaW, areaH := sizingAreaPt(ctx)
	colW := areaW * (100 - clampPct(ovr.ChartWidthPct, 65.0, 40.0, 80.0)) / 100
	if v.Chart == nil {
		colW = areaW
	}
	colH := areaH
	if v.Source != "" {
		colH -= cisSourceRowPt + 8 // source row + row gap
	}

	var rows []jsonschema.GridRowInput
	used := 0.0
	if hasHeadline {
		size := ResolveSize(ovr.HeadlineSize, 32)
		if ovr.HeadlineSize == 0 && len(v.Insights) >= 5 {
			size = 26
		}
		paras := []chartInsightsParagraph{{Content: v.Headline.Value, Size: size, Bold: true, Color: inkOnLight(ctx, accent, 3.0), Align: "l"}}
		sized := []sizedPara{{text: v.Headline.Value, sizePt: size, bold: true}}
		if v.Headline.Label != "" {
			paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(v.Headline.Label), Size: 12, Color: "dk1", Align: "l"})
			sized = append(sized, sizedPara{text: v.Headline.Label, sizePt: 12})
		}
		h := sizedBlockHeightPt(ctx, sized, colW)
		textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
		rows = append(rows, jsonschema.GridRowInput{Height: pctOf(h, colH), Cells: []*jsonschema.GridCellInput{{
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: textJSON},
		}}})
		used += pctOf(h, colH)
	}
	var soWhatRow *jsonschema.GridRowInput
	if hasSoWhat {
		tone := inactiveTintTone(accent)
		content := "<b>So what:</b> " + pptx.ConvertMarkdownEmphasis(v.SoWhat)
		soSize := 13.0
		if len(v.Insights) >= 5 {
			soSize = 12
		}
		h := sizedBlockHeightPt(ctx, []sizedPara{{text: content, sizePt: soSize}}, colW-8)
		textJSON, _ := json.Marshal(chartInsightsText{
			Paragraphs:    []chartInsightsParagraph{{Content: content, Size: soSize, Color: readableTextOn(ctx, tone, "dk1"), Align: "l"}},
			Align:         "l",
			VerticalAlign: "ctr",
		})
		soWhatRow = &jsonschema.GridRowInput{Height: pctOf(h, colH), Cells: []*jsonschema.GridCellInput{{
			Shape:     &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: tone.fillJSON(), Text: textJSON},
			AccentBar: &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 3},
		}}}
		used += pctOf(h, colH)
	}
	rows = append(rows, jsonschema.GridRowInput{Height: math.Max(100-used, 10), Cells: []*jsonschema.GridCellInput{insights}})
	if soWhatRow != nil {
		rows = append(rows, *soWhatRow)
	}
	return &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		RowGap:  6,
		Rows:    rows,
	}}
}
