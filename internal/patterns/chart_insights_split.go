package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// chart-insights-split pattern — left chart panel + right insights column
// ---------------------------------------------------------------------------
//
// Layout:
//   65/35 column split.
//     Left  (65%): chart diagram rendered via svggen.
//     Right (35%): 'Key Insights' label + bullet list of takeaways.
//   When chart is omitted: insights column expands to 100% width and the
//   pattern emits a CHART_PLACEHOLDER_EMPTY warning so agents know they
//   chose a chart-bearing pattern without providing a chart.

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
		InsightsTitle: "Key Insights",
		Insights: []string{
			"Revenue grew 75% YoY driven by enterprise adoption.",
			"Q4 spike reflects the EMEA market launch.",
			"Pipeline coverage indicates further acceleration in FY26.",
		},
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
		if len(b) > 160 {
			errs = append(errs, errMaxLength(name, path, 160, len(b)))
		}
	}

	if v.InsightsTitle != "" && len(v.InsightsTitle) > 40 {
		errs = append(errs, errMaxLength(name, "values.insights_title", 40, len(v.InsightsTitle)))
	}
	if v.Source != "" && len(v.Source) > 120 {
		errs = append(errs, errMaxLength(name, "values.source", 120, len(v.Source)))
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

	// Build the insights panel (always present).
	insightsCell := buildInsightsPanel(insightsTitle, v.Insights, accent, titleSize, bulletSize)

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

	// Compute the chart panel width as a fraction of the grid.
	chartPct := clampPct(ovr.ChartWidthPct, 65.0, 40.0, 80.0)
	insightsPct := 100.0 - chartPct

	// Chart panel: a Diagram cell rendered via svggen.
	chartCell := &jsonschema.GridCellInput{
		Diagram: cloneDiagramSpec(v.Chart),
	}

	// Optional thin vertical divider rendered as an accent bar on the insights cell.
	showDivider := ovr.ShowDivider == nil || *ovr.ShowDivider
	if showDivider {
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
		rows = append(rows, jsonschema.GridRowInput{
			Height: 8,
			Cells:  []*jsonschema.GridCellInput{sourceCell, spacer},
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
}
