package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// metric-list pattern — a vertical "by the numbers" stack of 3-7 rows, each a
// big accent value beside a bold label and an optional detail line.
// ---------------------------------------------------------------------------
//
// Layout (one row per metric, hairline rules between rows):
//
//           3.8x │ Higher TSR by AI leaders
//                │ Leaders compound returns across the cycle
//   ───────────────────────────────────────────────────────
//          ~30%  │ Generated real AI value
//   ───────────────────────────────────────────────────────
//        16→33%  │ Agentic value share expected to double by 2028
//   ███████████ optional full-width accent callout banner ███████████
//
// The value column is fixed-width and right-aligned so the numbers line up on
// their right edge — the typographic convention for a stat stack. One item may
// be highlighted: its row is tinted with the accent and carries an accent bar,
// and the value ink is measured against the tint rather than assumed.
//
// Every value shares one type size, shrunk until the longest value fits its
// column on one line (a stat that wraps reads as two numbers). Item rows are
// uniform in height — a stack whose rows differ looks ragged — and sized from
// the tallest measured label + detail.

func init() {
	Default().Register(&metricList{})
}

type metricList struct{}

// Pattern budgets.
const (
	metricListMinItems   = 3
	metricListMaxItems   = 7
	metricListValueMax   = 12
	metricListLabelMax   = 60
	metricListDetailMax  = 120
	metricListCalloutMax = 140

	metricListDefaultValuePct = 28.0
	metricListMinValuePct     = 15.0
	metricListMaxValuePct     = 50.0

	// metricListColGapPt is near zero on purpose: a highlighted row tints both
	// its cells, and a real column gap would show as a white seam through the
	// tint. The visual gutter comes from the text insets instead. A gap of 0
	// reads as "unset" and resolves to shapegrid's 8pt default, so it is
	// stated as a hair above zero.
	metricListColGapPt    = 0.1
	metricListRowGapPt    = 2.0
	metricListRulePt      = 0.75
	metricListGutterPt    = 12.0 // inset on each side of the value / text boundary
	metricListOuterPt     = 10.0 // inset on the outer edges
	metricListBarPt       = 4.0  // highlight accent bar width
	metricListMinValuePt  = 16.0
	metricListMinFillFrac = 0.68
)

func (m *metricList) Name() string { return "metric-list" }
func (m *metricList) Description() string {
	return "Vertical 'by the numbers' stack of 3-7 metrics: big right-aligned accent value beside a bold label and optional detail line, hairline rules between rows, optional highlighted row and bottom callout banner"
}
func (m *metricList) UseWhen() string {
	return "3–7 headline numbers read top-to-bottom as a list, each needing a one-line label and an optional detail line (stat stack / 'by the numbers' / proof-point column); prefer kpi-3up..kpi-6up for 2–6 equally weighted KPI cards side by side, stat-hero or hero-detail when one number dominates, exec-summary when the rows are sentences rather than numbers, labeled-rows when each row is a keyword label with body text"
}
func (m *metricList) NotWhen() string {
	return "Metrics should sit side by side as cards (use kpi-Nup), one number dominates (use stat-hero or hero-detail), rows are conclusions without a number (use exec-summary) or keyword-labelled paragraphs (use labeled-rows), the values are a ranked series to compare visually (use horizontal-bar-with-callouts or a bar chart), or there are fewer than 3 / more than 7 metrics"
}
func (m *metricList) Version() int      { return 1 }
func (m *metricList) CellsHint() string { return "3-7 (rows)" }
func (m *metricList) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "data-display",
		NarrativeRole: []string{"evidence", "conclude"},
		PairsWith:     []string{"exec-summary", "chart-insights-split", "labeled-rows"},
		DensityClass:  "medium",
		AccentWeight:  "strong",
	}
}

func (m *metricList) SupportsInlineMarkdown() bool { return true }

func (m *metricList) ExemplarValues() any {
	return &MetricListValues{
		Items: []MetricListItem{
			{Value: "3.8x", Label: "Higher TSR for AI leaders", Detail: "Leaders compound returns across the full cycle", Highlight: true},
			{Value: "~30%", Label: "Share of value already generated", Detail: "Up 2.2x from 2023"},
			{Value: "20x", Label: "More agentic solutions in production", Detail: "Leaders versus laggards"},
			{Value: "16→33%", Label: "Agentic share of AI value", Detail: "Expected to double by 2028"},
		},
		Callout: "The window is open because most firms have not moved beyond pilots",
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// MetricListItem is one row: a short value, a label and an optional detail.
type MetricListItem struct {
	Value     string `json:"value"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	Highlight bool   `json:"highlight,omitempty"`
}

// MetricListValues holds the 3-7 metrics and the optional callout banner.
type MetricListValues struct {
	Items   []MetricListItem `json:"items"`
	Callout string           `json:"callout,omitempty"`
}

// MetricListOverrides are the pattern-level knobs.
type MetricListOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	ValueSize      float64 `json:"value_size,omitempty"`
	LabelSize      float64 `json:"label_size,omitempty"`
	ValueWidthPct  float64 `json:"value_width_pct,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"`
}

// MetricListCellOverride is the shared per-cell override, indexed by item.
type MetricListCellOverride = CellOverride

func (m *metricList) NewValues() any       { return &MetricListValues{} }
func (m *metricList) NewOverrides() any    { return &MetricListOverrides{} }
func (m *metricList) NewCellOverride() any { return &MetricListCellOverride{} }

func (m *metricList) Schema() *Schema {
	itemSchema := ObjectSchema(
		map[string]*Schema{
			"value":     StringSchema(metricListValueMax).WithDescription("The number, short (≤12 chars): \"3.8x\", \"~30%\", \"$4.2M\", \"16→33%\". Every value shares one size, shrunk until the longest fits its column on one line"),
			"label":     StringSchema(metricListLabelMax).WithDescription("What the number measures, one line (≤60 chars)"),
			"detail":    StringSchema(metricListDetailMax).WithDescription("Optional smaller context line under the label (≤120 chars). 3-4 items hold the full 120 at default sizes, 5 items about 90; with 6-7 items omit detail lines"),
			"highlight": BooleanSchema().WithDescription("Emphasise this row with an accent-tinted band and accent bar; at most one item"),
		},
		[]string{"value", "label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"items":   ArraySchema(itemSchema, metricListMinItems, metricListMaxItems).WithDescription("3-7 metrics, top to bottom"),
			"callout": StringSchema(metricListCalloutMax).WithDescription("Optional so-what rendered as a full-width accent banner under the list (≤140 chars); its text colour is chosen by measured contrast"),
		},
		[]string{"items"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color for the values, highlight and callout (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"value_size":       NumberSchema(16, 72).WithDescription("Value font size in points (default: the largest of 40/36/32/28/24 that fits; still shrinks so the longest value stays on one line)"),
			"label_size":       NumberSchema(12, 32).WithDescription("Label font size in points (default 18 stepping down to 14 as items are added); the detail line is 4pt smaller, never below 12"),
			"value_width_pct":  NumberSchema(metricListMinValuePct, metricListMaxValuePct).WithDescription("Width of the right-aligned value column as a percentage of the pattern width (default 28)"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-row accent rotation for the values"),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      overridesSchema,
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Metric list: 3-7 rows of a big right-aligned accent value beside a label and optional detail, hairline rules between rows, optional highlighted row and callout banner")
}

func (m *metricList) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*MetricListValues)
	if !ok || vals == nil {
		return fmt.Errorf("metric-list: values must be *MetricListValues, got %T", values)
	}
	const name = "metric-list"
	var errs []error

	if overrides != nil {
		errs = append(errs, validateMetricListOverrides(overrides)...)
	}

	if len(vals.Items) < metricListMinItems {
		errs = append(errs, errMinItems(name, "items", metricListMinItems, len(vals.Items), "(hint: use stat-hero for one number or kpi-2up for two)"))
	}
	if len(vals.Items) > metricListMaxItems {
		errs = append(errs, errMaxItems(name, "items", metricListMaxItems, len(vals.Items), "(hint: keep the strongest 7 or split across two slides)"))
	}
	highlighted := 0
	for i, it := range vals.Items {
		for _, f := range []struct {
			field, text string
			max         int
			required    bool
		}{
			{"value", it.Value, metricListValueMax, true},
			{"label", it.Label, metricListLabelMax, true},
			{"detail", it.Detail, metricListDetailMax, false},
		} {
			path := fmt.Sprintf("items[%d].%s", i, f.field)
			switch {
			case f.required && strings.TrimSpace(f.text) == "":
				errs = append(errs, errRequired(name, path))
			case runeLen(f.text) > f.max:
				errs = append(errs, errMaxLength(name, path, f.max, runeLen(f.text)))
			}
		}
		if it.Highlight {
			highlighted++
			if highlighted == 2 {
				errs = append(errs, newValidationError(name, fmt.Sprintf("items[%d].highlight", i), ErrCodeOutOfRange,
					fmt.Sprintf("metric-list: items[%d].highlight: at most one item may set highlight — a second highlight dilutes the first; keep the one row the slide is about", i),
					RemoveFieldFix(fmt.Sprintf("items[%d].highlight", i))))
			}
		}
	}
	if runeLen(vals.Callout) > metricListCalloutMax {
		errs = append(errs, errMaxLength(name, "callout", metricListCalloutMax, runeLen(vals.Callout)))
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Items), "(index = item)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

// validateMetricListOverrides checks the pattern-level knobs.
func validateMetricListOverrides(overrides any) []error {
	const name = "metric-list"
	ovr, ok := overrides.(*MetricListOverrides)
	if !ok {
		return []error{fmt.Errorf("metric-list: overrides must be *MetricListOverrides, got %T", overrides)}
	}
	var errs []error
	if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
		errs = append(errs, err)
	}
	if ovr.ValueWidthPct != 0 && (ovr.ValueWidthPct < metricListMinValuePct || ovr.ValueWidthPct > metricListMaxValuePct) {
		errs = append(errs, errOutOfRange(name, "overrides.value_width_pct", int(metricListMinValuePct), int(metricListMaxValuePct), int(ovr.ValueWidthPct)))
	}
	if ovr.ValueSize != 0 && (ovr.ValueSize < 16 || ovr.ValueSize > 72) {
		errs = append(errs, errOutOfRange(name, "overrides.value_size", 16, 72, int(ovr.ValueSize)))
	}
	if ovr.LabelSize != 0 && (ovr.LabelSize < 12 || ovr.LabelSize > 32) {
		errs = append(errs, errOutOfRange(name, "overrides.label_size", 12, 32, int(ovr.LabelSize)))
	}
	return errs
}

// metricListLayout is the measured geometry at one type scale.
type metricListLayout struct {
	cols                             []float64
	valueSize, labelSize, detailSize float64
	rowPt                            float64 // uniform item row height
	calloutPt                        float64 // 0 = no callout
	unfitValues                      []int   // items whose value wraps even at the floor
}

func (l metricListLayout) natural(n int) float64 {
	rows := n + (n - 1) // item rows + rules
	h := float64(n)*l.rowPt + float64(n-1)*metricListRulePt
	if l.calloutPt > 0 {
		rows++
		h += l.calloutPt
	}
	return h + float64(rows-1)*metricListRowGapPt
}

// metricListScales is the default type scale, largest first: value, label.
var metricListScales = [][2]float64{{40, 18}, {36, 17}, {32, 16}, {28, 15}, {24, 14}, {22, 14}}

func metricListDetailSize(label float64) float64 { return math.Max(12, label-4) }

// layoutMetricList picks the largest type scale whose natural height fits the
// content area and measures every row at it.
func layoutMetricList(ctx ExpandContext, vals *MetricListValues, ovr *MetricListOverrides) metricListLayout {
	valuePct := metricListDefaultValuePct
	if ovr.ValueWidthPct > 0 {
		valuePct = clampPt(ovr.ValueWidthPct, metricListMinValuePct, metricListMaxValuePct)
	}
	cols := []float64{valuePct, 100 - valuePct}
	areaW, areaH := sizingAreaPt(ctx)

	scales := metricListScales
	if len(vals.Items) >= 6 {
		scales = scales[1:]
	}
	// A list with no detail lines gives the label the whole row: set it 2pt
	// larger so the stack does not read as a column of small captions.
	labelBump := 2.0
	for _, it := range vals.Items {
		if strings.TrimSpace(it.Detail) != "" {
			labelBump = 0
			break
		}
	}
	if ovr.ValueSize > 0 || ovr.LabelSize > 0 {
		scales = [][2]float64{{ResolveSize(ovr.ValueSize, scales[0][0]), ResolveSize(ovr.LabelSize, scales[0][1]+labelBump)}}
		labelBump = 0
	}
	var lay metricListLayout
	for _, sc := range scales {
		lay = measureMetricList(ctx, vals, cols, sc[0], sc[1]+labelBump, areaW)
		if lay.natural(len(vals.Items)) <= areaH {
			break
		}
	}
	return lay
}

func measureMetricList(ctx ExpandContext, vals *MetricListValues, cols []float64, valueSize, labelSize, areaW float64) metricListLayout {
	usableW := areaW - metricListColGapPt
	valueColW := usableW * cols[0] / 100
	textColW := usableW * cols[1] / 100
	lay := metricListLayout{cols: cols, labelSize: labelSize, detailSize: metricListDetailSize(labelSize)}

	// One shared value size: the largest that puts every value on one line.
	font := ctx.Theme.BodyFont
	valueTextW := valueColW - metricListOuterPt - metricListBarPt - metricListGutterPt
	size := valueSize
	for _, it := range vals.Items {
		if s := fitSingleLineSize(it.Value, font, true, size, metricListMinValuePt, valueTextW); s < size {
			size = s
		}
	}
	lay.valueSize = size
	for i, it := range vals.Items {
		if strings.TrimSpace(it.Value) != "" && measuredLines(it.Value, font, true, size, valueTextW) > 1 {
			lay.unfitValues = append(lay.unfitValues, i)
		}
	}

	// sizedBlockHeightPt assumes the default 7.2pt side insets; hand it the
	// width that leaves the text the same measure as the real insets do.
	textFrameW := textColW - (metricListGutterPt + metricListOuterPt) + 2*sizingInsetLRPt
	row := size*sizingLineSpacing + 2*sizingInsetTBPt
	for _, it := range vals.Items {
		paras := []sizedPara{{text: it.Label, sizePt: labelSize, bold: true, spaceAfterPt: 2}}
		if strings.TrimSpace(it.Detail) != "" {
			paras = append(paras, sizedPara{text: it.Detail, sizePt: lay.detailSize})
		}
		row = math.Max(row, sizedBlockHeightPt(ctx, paras, textFrameW))
	}
	lay.rowPt = math.Ceil(row)
	if strings.TrimSpace(vals.Callout) != "" {
		h := sizedBlockHeightPt(ctx, []sizedPara{{text: vals.Callout, sizePt: labelSize, bold: true}}, areaW-2*metricListOuterPt)
		lay.calloutPt = math.Ceil(math.Max(h, labelSize*sizingLineSpacing*2))
	}
	return lay
}

// metricListHighlightTone is the highlighted row's band: PowerPoint's
// "Lighter 80%" swatch of the accent. It is expressed with lumMod / lumOff
// rather than a tint because the band's outline must match it exactly, and a
// shape line honours lumMod / lumOff but not tint.
func metricListHighlightTone(accent string) fillTone { return inactiveTintTone(accent) }

// metricListBandLine outlines a highlighted cell in its own band colour. The
// two cells of a row sit a hair apart (metricListColGapPt), and without the
// outline that hairline of white shows as a seam through the band.
func metricListBandLine(tone fillTone) json.RawMessage {
	data, _ := json.Marshal(struct {
		Color  string  `json:"color"`
		Width  float64 `json:"width"`
		LumMod int     `json:"lumMod,omitempty"`
		LumOff int     `json:"lumOff,omitempty"`
	}{tone.Color, 0.75, tone.LumMod, tone.LumOff})
	return data
}

// insetText is a cell text object with per-side insets (points).
type insetText struct {
	Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
	InsetLeft     float64                  `json:"inset_left,omitempty"`
	InsetRight    float64                  `json:"inset_right,omitempty"`
	InsetTop      float64                  `json:"inset_top,omitempty"`
	InsetBottom   float64                  `json:"inset_bottom,omitempty"`
}

func (t insetText) json() json.RawMessage {
	data, _ := json.Marshal(t)
	return data
}

// hairlineRuleRow is a full-width 0.75pt rule row spanning cols columns.
func hairlineRuleRow(cols int) jsonschema.GridRowInput {
	return jsonschema.GridRowInput{
		MinHeight: metricListRulePt, MaxHeight: metricListRulePt,
		Cells: []*jsonschema.GridCellInput{{
			ColSpan: cols,
			Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: fillTone{Color: "dk1", Alpha: 30}.fillJSON()},
		}},
	}
}

// inkOnFill returns preferred when it clears minRatio against the effective
// fill, else the first theme ink that does. Without a theme it trusts
// preferred.
func inkOnFill(ctx ExpandContext, preferred string, tone fillTone, minRatio float64) string {
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return preferred
	}
	if c, cok := resolveThemeColor(ctx, preferred); cok && c.ContrastWith(fill) >= minRatio {
		return preferred
	}
	return readableInkOn(ctx, tone, preferred, svggen.WCAGAANormal)
}

func (m *metricList) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*MetricListValues)
	if !ok || vals == nil {
		return nil, fmt.Errorf("metric-list: values must be *MetricListValues, got %T", values)
	}
	ovr := &MetricListOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*MetricListOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("metric-list: overrides must be *MetricListOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := layoutMetricList(ctx, vals, ovr)
	labelInk := inkOnLight(ctx, "dk2", 4.5)

	var rows []jsonschema.GridRowInput
	var itemRow []bool
	for i, it := range vals.Items {
		if i > 0 {
			rows = append(rows, hairlineRuleRow(2))
			itemRow = append(itemRow, false)
		}
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		co, _ := cellOverrides[i].(*MetricListCellOverride)

		surface := fillTone{Color: "lt1"}
		fill := json.RawMessage(`"none"`)
		line := json.RawMessage(`"none"`)
		if it.Highlight {
			surface = metricListHighlightTone(accent)
			fill = surface.fillJSON()
			line = metricListBandLine(surface)
		}
		valueInk := inkOnFill(ctx, accent, surface, 3.0)
		rowLabelInk := labelInk
		detailInk := "dk1"
		if it.Highlight {
			rowLabelInk = inkOnFill(ctx, labelInk, surface, 4.5)
			detailInk = inkOnFill(ctx, "dk1", surface, 4.5)
		}
		if co != nil && co.Color != "" {
			valueInk = co.Color
		}

		valueCell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     fill,
			Line:     line,
			Text: insetText{
				Paragraphs: []chartInsightsParagraph{{
					Content: it.Value, Size: lay.valueSize, Bold: true, Color: valueInk, Align: "r",
				}},
				Align: "r", VerticalAlign: "ctr",
				InsetLeft: metricListOuterPt + metricListBarPt, InsetRight: metricListGutterPt,
			}.json(),
		}}
		// The big value is the item's primary text (D15 text keys).
		applyCellTextOverride(valueCell, co)
		if it.Highlight || (co != nil && co.AccentBar) {
			valueCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: metricListBarPt}
		}

		paras := []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(it.Label), Size: lay.labelSize, Bold: true, Color: rowLabelInk, Align: "l", SpaceAfter: 2}}
		if strings.TrimSpace(it.Detail) != "" {
			paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(it.Detail), Size: lay.detailSize, Color: detailInk, Align: "l"})
		}
		textCell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     fill,
			Line:     line,
			Text: insetText{
				Paragraphs: paras, Align: "l", VerticalAlign: "ctr",
				InsetLeft: metricListGutterPt, InsetRight: metricListOuterPt,
			}.json(),
		}}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: lay.rowPt, MaxHeight: lay.rowPt, Cells: []*jsonschema.GridCellInput{valueCell, textCell}})
		itemRow = append(itemRow, true)
	}

	if lay.calloutPt > 0 {
		tone := fillTone{Color: baseAccent}
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: lay.calloutPt, MaxHeight: lay.calloutPt,
			Cells: []*jsonschema.GridCellInput{{
				ColSpan: 2,
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(strconv.Quote(baseAccent)),
					Line:     json.RawMessage(`"none"`),
					Text: insetText{
						Paragraphs: []chartInsightsParagraph{{
							Content: pptx.ConvertMarkdownEmphasis(vals.Callout), Size: lay.labelSize, Bold: true,
							Color: readableTextOn(ctx, tone, "lt1"), Align: "ctr",
						}},
						Align: "ctr", VerticalAlign: "ctr",
						InsetLeft: metricListOuterPt, InsetRight: metricListOuterPt,
					}.json(),
				},
			}},
		})
		itemRow = append(itemRow, false)
	}

	colsJSON, _ := json.Marshal(lay.cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  metricListColGapPt,
		RowGap:  metricListRowGapPt,
		Rows:    rows,
	}
	fillCappedRows(ctx, grid.Rows, grid.RowGap, metricListMinFillFrac, func(i int) bool { return itemRow[i] })
	return grid, nil
}

// PostExpandWarnings reports what the stack measured and could not fix: a
// value that wraps even at the floor size, and a list too tall for the content
// area at the smallest type scale.
func (m *metricList) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*MetricListValues)
	if !ok || v == nil || len(v.Items) == 0 {
		return nil
	}
	ovr, _ := overrides.(*MetricListOverrides)
	if ovr == nil {
		ovr = &MetricListOverrides{}
	}
	lay := layoutMetricList(ctx, v, ovr)
	var out []string
	for _, i := range lay.unfitValues {
		out = append(out, fmt.Sprintf(
			"%s: metric-list items[%d].value %q does not fit the value column on one line even at %.0fpt — the renderer breaks the number; shorten it (\"$4.2M\", \"2-5x\") or raise overrides.value_width_pct",
			ErrCodeTextExceedsShape, i, v.Items[i].Value, lay.valueSize))
	}
	_, areaH := sizingAreaPt(ctx)
	if need := lay.natural(len(v.Items)); need > areaH+1 {
		out = append(out, fmt.Sprintf(
			"%s: metric-list items need %.0fpt at the smallest type scale but the content area holds about %.0fpt — 5 items hold about 90 detail characters each and 6-7 items none; shorten or drop the detail lines, drop the callout, or use fewer items",
			ErrCodeBodyTooLong, need, areaH))
	}
	return out
}
