package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
//   ▌optional takeaway-band callout (accent bar + bold text)
//
// The value column is fixed-width and right-aligned so the numbers line up on
// their right edge — the typographic convention for a stat stack. One item may
// be highlighted: its row is tinted with the accent and carries an accent bar,
// and the value ink is measured against the tint rather than assumed.
//
// Every value shares one type size on the 40 / 36 / 32 / 28 / 24pt ladder: the
// largest step at which every value fits its column on one line (a stat that
// wraps reads as two numbers) and the list fits its area. The value column
// widens for a long value, and the rows give up their vertical text margin,
// before a value goes under 24pt — it never does: a list that cannot hold
// 24pt values is reported (go-slide-creator-1vmsk). Item rows are uniform in
// height — a stack whose rows differ looks ragged — and sized from the tallest
// measured label + detail.

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
	// metricListAutoMaxValuePct is how far the value column widens by itself
	// so the longest value stays on one line at the size the list holds.
	metricListAutoMaxValuePct = 40.0

	// metricListColGapPt is near zero on purpose: a highlighted row tints both
	// its cells, and a real column gap would show as a white seam through the
	// tint. The visual gutter comes from the uniform text margin instead
	// (defaultShapeInsetLRPt on each side of the boundary). A gap of 0
	// reads as "unset" and resolves to shapegrid's 8pt default, so it is
	// stated as a hair above zero.
	metricListColGapPt    = 0.1
	metricListRowGapPt    = 2.0
	metricListRulePt      = 0.75
	metricListBarPt       = 4.0 // highlight accent bar width
	metricListMinFillFrac = 0.68

	// metricListAuthoredMinValuePt is the floor an authored
	// overrides.value_size still shrinks to so its longest value stays on one
	// line. Default values never go under the ladder's 24pt floor
	// (metricListScales): under it the number no longer leads its label.
	metricListAuthoredMinValuePt = 16.0
	// metricListTightInsetPt is the top / bottom text margin of a list that
	// only fits without the uniform 0.5 cm one: the rows drop their spacing
	// before the values leave the ladder.
	metricListTightInsetPt = 4.0
	// metricListValue36Pt / metricListValue32Pt are the ladder's steps
	// between the 40pt KPI step and the 28pt display step.
	metricListValue36Pt = 36.0
	metricListValue32Pt = 32.0
)

func (m *metricList) Name() string { return "metric-list" }
func (m *metricList) Description() string {
	return "Vertical 'by the numbers' stack of 3-7 metrics: big right-aligned accent value beside a bold label and optional detail line, hairline rules between rows, optional highlighted row and bottom takeaway-band callout"
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

// MetricListValues holds the 3-7 metrics and the optional takeaway callout.
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
	// TakeawayEmphasis styles the callout band: "" (accent bar only),
	// "subtle" (5% neutral tint) or "strong" (solid accent).
	TakeawayEmphasis string `json:"takeaway_emphasis,omitempty"`
}

// MetricListCellOverride is the shared per-cell override, indexed by item.
type MetricListCellOverride = CellOverride

func (m *metricList) NewValues() any       { return &MetricListValues{} }
func (m *metricList) NewOverrides() any    { return &MetricListOverrides{} }
func (m *metricList) NewCellOverride() any { return &MetricListCellOverride{} }

func (m *metricList) Schema() *Schema {
	itemSchema := ObjectSchema(
		map[string]*Schema{
			"value":     StringSchema(metricListValueMax).WithDescription("The number, short (≤12 chars): \"3.8x\", \"~30%\", \"$4.2M\", \"16→33%\". Every value shares one size on the 40/36/32/28/24pt ladder, the largest at which the longest value fits its column on one line"),
			"label":     StringSchema(metricListLabelMax).WithDescription("What the number measures, one line (≤60 chars)"),
			"detail":    StringSchema(metricListDetailMax).WithDescription("Optional smaller context line under the label (≤120 chars). 3 items hold the full 120 at default sizes, 4 items about 98, 5 items about 40; with 6-7 items omit detail lines"),
			"highlight": BooleanSchema().WithDescription("Emphasise this row with an accent-tinted band and accent bar; at most one item"),
		},
		[]string{"value", "label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"items":   ArraySchema(itemSchema, metricListMinItems, metricListMaxItems).WithDescription("3-7 metrics, top to bottom"),
			"callout": StringSchema(metricListCalloutMax).WithDescription("Optional so-what rendered as the takeaway band under the list (≤140 chars): flush accent bar, bold dk1 text, no box; overrides.takeaway_emphasis tints or fills it"),
		},
		[]string{"items"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":            StringSchema(0).WithDescription("Accent scheme color for the values, highlight and callout (default accent1)").WithDefault("accent1"),
			"semantic_accent":   EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"value_size":        NumberSchema(16, 72).WithDescription("Value font size in points (default: the largest of 40/36/32/28/24 at which every value fits its column on one line and the list fits its area — never under 24; an authored size still shrinks, down to 16, so the longest value stays on one line)"),
			"label_size":        NumberSchema(12, 32).WithDescription("Label font size in points (default 18 stepping down to 14 as items are added); the detail line is 4pt smaller, never below 12"),
			"value_width_pct":   NumberSchema(metricListMinValuePct, metricListMaxValuePct).WithDescription("Width of the right-aligned value column as a percentage of the pattern width (default 28, widening up to 40 for a long value)"),
			"cell_accent_mode":  EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-row accent rotation for the values"),
			"takeaway_emphasis": TakeawayEmphasisSchema(),
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
	}).WithDescription("Metric list: 3-7 rows of a big right-aligned accent value beside a label and optional detail, hairline rules between rows, optional highlighted row and takeaway-band callout")
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
	if err := validateTakeawayEmphasis(name, ovr.TakeawayEmphasis); err != nil {
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
	rowGapPt                         float64 // metricListRowGapPt on the template grid
	tight                            bool    // rows use metricListTightInsetPt
	fits                             bool    // the list fits its area at this scale
}

// insetTB is the rows' top / bottom text margin: nil for the uniform shape
// margin, metricListTightInsetPt on a tight list.
func (l metricListLayout) insetTB() *float64 {
	if !l.tight {
		return nil
	}
	pt := metricListTightInsetPt
	return &pt
}

func (l metricListLayout) natural(n int) float64 {
	rows := n + (n - 1) // item rows + rules
	h := float64(n)*l.rowPt + float64(n-1)*metricListRulePt
	if l.calloutPt > 0 {
		rows++
		h += l.calloutPt
	}
	return h + float64(rows-1)*l.rowGapPt
}

// minimal is natural without the callout band's air (the spacer above it, the
// sub-grid insets and its rounding slack), which the grid squeezes before any
// text row is written shrunk.
func (l metricListLayout) minimal(n int) float64 {
	h := l.natural(n)
	if l.calloutPt > 0 {
		h -= TakeawaySpacerPt(l.rowGapPt) + 2*SubGridInsetPt + takeawaySafetyPt
	}
	return h
}

// metricListScales is the default type scale, largest first: value, label.
//
// Values are display figures on the documented 40 / 36 / 32 / 28 / 24pt
// ladder and never leave it: the last two steps keep the 24pt value and take
// the label from subhead to body, so a tall list gives up label size, then
// row spacing, and is reported when even that does not fit — the number has
// to lead its label (go-slide-creator-1vmsk). It once stepped 40 / 28 / 18,
// and four exemplar rows under a title drew 18pt values beside 12pt labels.
var metricListScales = [][2]float64{
	{scaleKPIPt, scaleLeadPt},
	{metricListValue36Pt, scaleLeadPt},
	{metricListValue32Pt, scaleSubheadPt},
	{scaleDisplayPt, scaleSubheadPt},
	{sizeFigurePt, scaleSubheadPt},
	{sizeFigurePt, scaleBodyPt},
}

// metricListDenseFrom is the first ladder step a list of six or seven rows
// tries: the 40 and 36pt steps never fit one.
const metricListDenseFrom = 2

func metricListDetailSize(label float64) float64 { return math.Max(12, label-4) }

// layoutMetricList picks the largest type scale at which every value fits its
// column on one line and the list fits the content area, and measures every
// row at it.
func layoutMetricList(ctx ExpandContext, vals *MetricListValues, ovr *MetricListOverrides) metricListLayout {
	areaW, areaH := sizingAreaPt(ctx)
	n := len(vals.Items)

	scales := metricListScales
	if n >= 6 {
		scales = scales[metricListDenseFrom:]
	}
	// A list with no detail lines gives the label the whole row: set it 2pt
	// larger, settled onto the scale (a 12pt label becomes 14pt), so the
	// stack does not read as a column of small captions.
	labelBump := 2.0
	for _, it := range vals.Items {
		if strings.TrimSpace(it.Detail) != "" {
			labelBump = 0
			break
		}
	}
	authored := ovr.ValueSize > 0 || ovr.LabelSize > 0
	if authored {
		scales = [][2]float64{{ResolveSize(ovr.ValueSize, scales[0][0]), ResolveSize(ovr.LabelSize, snapPt(scales[0][1]+labelBump))}}
		labelBump = 0
	}
	measure := func(sc [2]float64, tight bool) metricListLayout {
		label := sc[1]
		if labelBump > 0 {
			label = snapPt(label + labelBump)
		}
		lay := measureMetricList(ctx, vals, ovr, sc[0], label, areaW, tight)
		// The air above the callout band gives way before the type does.
		lay.fits = lay.minimal(n) <= areaH
		return lay
	}

	var lay metricListLayout
	for i, sc := range scales {
		lay = measure(sc, false)
		// A default step whose longest value would wrap is not taken: the
		// next step down is tried instead of shrinking the value between
		// steps. The last step keeps its wrap for the finding.
		if lay.fits && (authored || len(lay.unfitValues) == 0 || i == len(scales)-1) {
			return lay
		}
	}
	// Nothing fits at the uniform row margin: the smallest scale gives up
	// the rows' vertical text margin. A list that overflows even so keeps
	// this layout and PostExpandWarnings reports it.
	return measure(scales[len(scales)-1], true)
}

// metricListValuePct returns the value column's share: the authored one, or
// the default widened — up to metricListAutoMaxValuePct — until every value
// fits on one line at valueSize.
func metricListValuePct(ctx ExpandContext, vals *MetricListValues, ovr *MetricListOverrides, valueSize, areaW float64) float64 {
	if ovr.ValueWidthPct > 0 {
		return clampPt(ovr.ValueWidthPct, metricListMinValuePct, metricListMaxValuePct)
	}
	font := ctx.Theme.BodyFont
	usableW := areaW - metricListColGapPt
	for pct := metricListDefaultValuePct; pct < metricListAutoMaxValuePct; pct++ {
		textW := usableW*pct/100 - 2*defaultShapeInsetLRPt
		fits := true
		for _, it := range vals.Items {
			if strings.TrimSpace(it.Value) != "" && measuredLines(it.Value, font, true, valueSize, textW) > 1 {
				fits = false
				break
			}
		}
		if fits {
			return pct
		}
	}
	return metricListAutoMaxValuePct
}

func measureMetricList(ctx ExpandContext, vals *MetricListValues, ovr *MetricListOverrides, valueSize, labelSize, areaW float64, tight bool) metricListLayout {
	valuePct := metricListValuePct(ctx, vals, ovr, valueSize, areaW)
	cols := []float64{valuePct, 100 - valuePct}
	usableW := areaW - metricListColGapPt
	valueColW := usableW * cols[0] / 100
	textColW := usableW * cols[1] / 100
	lay := metricListLayout{cols: cols, labelSize: labelSize, detailSize: metricListDetailSize(labelSize), rowGapPt: ctx.Gap(metricListRowGapPt), tight: tight}

	font := ctx.Theme.BodyFont
	// The highlight bar sits inside the value cell's left margin.
	valueTextW := valueColW - 2*defaultShapeInsetLRPt
	size := valueSize
	if ovr.ValueSize > 0 {
		// An authored size is not a ladder step: it shrinks continuously so
		// its longest value stays on one line.
		for _, it := range vals.Items {
			if s := fitSingleLineSize(it.Value, font, true, size, metricListAuthoredMinValuePt, valueTextW); s < size {
				size = s
			}
		}
	}
	lay.valueSize = size
	for i, it := range vals.Items {
		if strings.TrimSpace(it.Value) != "" && measuredLines(it.Value, font, true, size, valueTextW) > 1 {
			lay.unfitValues = append(lay.unfitValues, i)
		}
	}

	insetPt := sizingInsetTBPt
	if tight {
		insetPt = metricListTightInsetPt
	}
	textFrameW := textColW
	row := size*sizingLineSpacing + 2*insetPt
	for _, it := range vals.Items {
		if !tight {
			paras := []sizedPara{{text: it.Label, sizePt: labelSize, bold: true, spaceAfterPt: 2}}
			if strings.TrimSpace(it.Detail) != "" {
				paras = append(paras, sizedPara{text: it.Detail, sizePt: lay.detailSize})
			}
			row = math.Max(row, sizedBlockHeightPt(ctx, paras, textFrameW))
		}
		// The row is never below what the writer needs to store the text
		// unshrunk at the real column widths (go-slide-creator-k3eb3).
		row = math.Max(row, writtenFitHeightPt(ctx.themeFonts(), metricListTextJSON(it, labelSize, lay.detailSize, "dk2", "dk1", lay.insetTB()), textFrameW, 0))
		row = math.Max(row, writtenFitHeightPt(ctx.themeFonts(), metricListValueJSON(it.Value, size, "dk1", lay.insetTB()), valueColW, 0))
	}
	lay.rowPt = math.Ceil(row)
	if strings.TrimSpace(vals.Callout) != "" {
		lay.calloutPt = TakeawayRowHeightPt(ctx, metricListTakeaway(vals.Callout, "", ovr.TakeawayEmphasis), areaW, ctx.Gap(metricListRowGapPt))
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

// insetText is a paragraphs cell text object. It carries no horizontal
// inset_* fields: every pattern shape keeps the uniform shape text margin.
type insetText struct {
	Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
	// InsetTop / InsetBottom are set only by a metric-list too tall for the
	// uniform margin.
	InsetTop    *float64 `json:"inset_top,omitempty"`
	InsetBottom *float64 `json:"inset_bottom,omitempty"`
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
			Text:     metricListValueJSON(it.Value, lay.valueSize, valueInk, lay.insetTB()),
		}}
		// The big value is the item's primary text (D15 text keys).
		applyCellTextOverride(valueCell, co)
		if it.Highlight || (co != nil && co.AccentBar) {
			valueCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: metricListBarPt}
		}

		textCell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     fill,
			Line:     line,
			Text:     metricListTextJSON(it, lay.labelSize, lay.detailSize, rowLabelInk, detailInk, lay.insetTB()),
		}}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: lay.rowPt, MaxHeight: lay.rowPt, Cells: []*jsonschema.GridCellInput{valueCell, textCell}})
		itemRow = append(itemRow, true)
	}

	if lay.calloutPt > 0 {
		// The so-what is the shared takeaway band, not a solid accent banner
		// (go-slide-creator-7b5o6).
		areaW, _ := sizingAreaPt(ctx)
		rows = append(rows, TakeawayRow(ctx, metricListTakeaway(vals.Callout, baseAccent, ovr.TakeawayEmphasis), 2, areaW, ctx.Gap(metricListRowGapPt)))
		itemRow = append(itemRow, false)
	}

	colsJSON, _ := json.Marshal(lay.cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  metricListColGapPt,
		RowGap:  ctx.Gap(metricListRowGapPt),
		Rows:    rows,
	}
	fillCappedRows(ctx, grid.Rows, grid.RowGap, metricListMinFillFrac, func(i int) bool { return itemRow[i] })
	return grid, nil
}

// PostExpandWarnings reports what the stack measured and could not fix: a
// value that wraps even at the 24pt floor in the widest column, and a list too
// tall for its area with 24pt values and no row spacing left to give.
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
	if need := lay.minimal(len(v.Items)); need > areaH+1 {
		out = append(out, fmt.Sprintf(
			"%s: metric-list: %d rows need %.0fpt with %.0fpt values and %.0fpt labels at the tightest row spacing, but the area holds about %.0fpt — the values stay on the 24-40pt ladder, so the list overflows instead of shrinking; use fewer rows, shorten or drop the detail lines (4 rows hold about 98 detail characters each, 5 rows about 40, 6-7 rows none), or drop the callout",
			ErrCodeBodyTooLong, len(v.Items), need, lay.valueSize, lay.labelSize, areaH))
	}
	return out
}

// metricListValueJSON is the value cell text; measureMetricList sizes the row
// on the same text Expand writes.
func metricListValueJSON(value string, size float64, ink string, insetTB *float64) json.RawMessage {
	return insetText{
		Paragraphs: []chartInsightsParagraph{{Content: value, Size: size, Bold: true, Color: ink, Align: "r"}},
		Align:      "r", VerticalAlign: "ctr", InsetTop: insetTB, InsetBottom: insetTB,
	}.json()
}

// metricListTextJSON is the label + detail cell text.
func metricListTextJSON(it MetricListItem, labelSize, detailSize float64, labelInk, detailInk string, insetTB *float64) json.RawMessage {
	paras := []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(it.Label), Size: labelSize, Bold: true, Color: labelInk, Align: "l", SpaceAfter: 2}}
	if strings.TrimSpace(it.Detail) != "" {
		paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(it.Detail), Size: detailSize, Color: detailInk, Align: "l"})
	}
	return insetText{Paragraphs: paras, Align: "l", VerticalAlign: "ctr", InsetTop: insetTB, InsetBottom: insetTB}.json()
}

// metricListTakeaway is the callout as a takeaway band spec.
func metricListTakeaway(callout, accent, emphasis string) TakeawaySpec {
	return TakeawaySpec{Text: pptx.ConvertMarkdownEmphasis(callout), Accent: accent, Emphasis: emphasis}
}
