package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// waterfall-bridge pattern — horizontal P&L / driver-decomposition bridge.
//
// Layout:
//   N columns (3-10), one per component. Each column's "bar" floats vertically
//   so that the bar's top/bottom edges align with the running cumulative total
//   at that step, creating the classic waterfall/bridge silhouette.
//
//   Outer grid: 2 rows.
//     Row 1 — bar area: N column cells, each holding a vertical sub-grid
//             [top-spacer, bridge line, bar, bridge line, bottom-spacer] on a
//             [gutter, bar, gutter] split (see buildWaterfallColumnGrid). Unused rows collapse to
//             zero height. The bar carries the value label unless it is too thin,
//             in which case the label sits in the adjacent spacer.
//     Row 2 — labels: N column cells with the component name beneath each bar.
//
//   Column types:
//     "total"    — bar runs from 0 to value (e.g. opening revenue, EBITDA).
//     "delta"    — floating bar from prev_running to prev_running + value;
//                  positive = accent (positive) fill; negative = accent2 / red.
//     "subtotal" — bar runs from 0 to running total; value auto-computed when
//                  omitted. Uses a distinct fill so the eye picks it out.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&waterfallBridge{})
}

type waterfallBridge struct{}

// Pattern budgets.
const (
	wbMinColumns = 3
	wbMaxColumns = 10
	wbLabelMax   = 40
	wbUnitMax    = 8
)

// Column type tokens.
const (
	wbTypeTotal    = "total"
	wbTypeDelta    = "delta"
	wbTypeSubtotal = "subtotal"
)

func (w *waterfallBridge) Name() string { return "waterfall-bridge" }
func (w *waterfallBridge) Description() string {
	return "Waterfall / bridge bar chart of 3-10 columns showing how components sum to a total; floating delta bars with auto-computed subtotals"
}
func (w *waterfallBridge) UseWhen() string {
	return "P&L walk, cost-driver decomposition, gap-to-target analysis, or any composition where positive/negative components reconcile a start total to an end total in a single visual; prefer chart-insights-split when the chart needs paired commentary, kpi-Nup when only the end totals matter, and value-chain when the sequence is operational rather than additive"
}
func (w *waterfallBridge) NotWhen() string {
	return "All values are independent (use a chart or kpi-Nup), the sequence is not additive (use value-chain or process-flow), only the end total matters without component attribution (use stat-hero), or column count is below 3 / above 10"
}
func (w *waterfallBridge) Version() int      { return 1 }
func (w *waterfallBridge) CellsHint() string { return "3-10" }
func (w *waterfallBridge) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "data-display",
		NarrativeRole:      []string{"evidence", "compare"},
		PairsWith:          []string{"kpi-3up", "chart-insights-split", "stat-hero", "scqa-summary"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (w *waterfallBridge) SupportsInlineMarkdown() bool { return true }

func (w *waterfallBridge) ExemplarValues() any {
	return &WaterfallBridgeValues{
		Unit: "$m",
		Columns: []WaterfallBridgeColumn{
			{Label: "Revenue", Value: 120, Type: wbTypeTotal},
			{Label: "COGS", Value: -45, Type: wbTypeDelta},
			{Label: "Gross Profit", Type: wbTypeSubtotal},
			{Label: "OpEx", Value: -30, Type: wbTypeDelta},
			{Label: "EBITDA", Value: 45, Type: wbTypeTotal},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// WaterfallBridgeColumn is one bar in the bridge. Type controls how Value is
// interpreted and how the bar floats.
type WaterfallBridgeColumn struct {
	Label string  `json:"label"`
	Value float64 `json:"value,omitempty"` // ignored for subtotal when omitted
	Type  string  `json:"type"`            // "total" | "delta" | "subtotal"
}

// WaterfallBridgeValues holds the ordered columns and an optional unit suffix
// (e.g. "$m", "%") appended to value labels.
type WaterfallBridgeValues struct {
	Columns []WaterfallBridgeColumn `json:"columns"`
	Unit    string                  `json:"unit,omitempty"`
}

// WaterfallBridgeOverrides reuses the standard text overrides plus a negative
// accent for downward delta bars.
type WaterfallBridgeOverrides struct {
	TextOverrides
	NegativeAccent string  `json:"negative_accent,omitempty"` // default: template semantic_accents.negative, else "accent2"
	SubtotalAccent string  `json:"subtotal_accent,omitempty"` // default: template semantic_accents.neutral, else "accent3"
	ValueSize      float64 `json:"value_size,omitempty"`      // default 10
	LabelSize      float64 `json:"label_size,omitempty"`      // default 9
}

// WaterfallBridgeCellOverride is the shared per-cell override; indexed by column.
type WaterfallBridgeCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (w *waterfallBridge) NewValues() any       { return &WaterfallBridgeValues{} }
func (w *waterfallBridge) NewOverrides() any    { return &WaterfallBridgeOverrides{} }
func (w *waterfallBridge) NewCellOverride() any { return &WaterfallBridgeCellOverride{} }

func (w *waterfallBridge) Schema() *Schema {
	columnSchema := ObjectSchema(
		map[string]*Schema{
			"label": StringSchema(wbLabelMax).WithDescription("Short column label (1-3 words)"),
			"value": NumberSchema(-1e12, 1e12).WithDescription("Numeric value; sign determines direction for delta; omit for subtotal to auto-compute"),
			"type":  EnumSchema(wbTypeTotal, wbTypeDelta, wbTypeSubtotal).WithDescription("\"total\" (anchored bar from 0), \"delta\" (floating bar of length value), or \"subtotal\" (running total to date)"),
		},
		[]string{"label", "type"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"columns": ArraySchema(columnSchema, wbMinColumns, wbMaxColumns).WithDescription("Bridge columns left-to-right (3-10)"),
			"unit":    StringSchema(wbUnitMax).WithDescription("Optional unit for value labels (e.g. \"$m\", \"%\"); a leading currency symbol renders as a prefix (\"$m\" → $210m, −$41m)"),
		},
		[]string{"columns"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color for total bars and positive deltas (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":      NumberSchema(6, 40).WithDescription("Column label font size (default 9)"),
			"body_size":        NumberSchema(6, 40).WithDescription("Value label font size (default 10)"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent rotation for delta bars"),
			"negative_accent":  StringSchema(0).WithDescription("Accent scheme color used for negative delta bars (default: the template's declared semantic_accents.negative, else accent2)"),
			"subtotal_accent":  StringSchema(0).WithDescription("Accent scheme color used for subtotal bars (default: the template's declared semantic_accents.neutral, else accent3)"),
			"label_size":       NumberSchema(6, 40).WithDescription("Column label font size (default 9) — overrides header_size for this pattern"),
			"value_size":       NumberSchema(6, 40).WithDescription("Value label font size (default 10) — overrides body_size for this pattern"),
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
	}).WithDescription("Waterfall / bridge bar chart of 3-10 columns with total, delta, and subtotal types; floating delta bars and auto-computed subtotals")
}

func (w *waterfallBridge) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*WaterfallBridgeValues)
	if !ok || vals == nil {
		return fmt.Errorf("waterfall-bridge: values must be *WaterfallBridgeValues, got %T", values)
	}

	const name = "waterfall-bridge"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*WaterfallBridgeOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(vals.Columns) < wbMinColumns {
		errs = append(errs, errMinItems(name, "columns", wbMinColumns, len(vals.Columns), "(hint: use stat-hero for a single number or kpi-Nup for fewer than 3 components)"))
	}
	if len(vals.Columns) > wbMaxColumns {
		errs = append(errs, errMaxItems(name, "columns", wbMaxColumns, len(vals.Columns), "(hint: collapse minor deltas or split into two slides)"))
	}

	for i, col := range vals.Columns {
		labelPath := fmt.Sprintf("columns[%d].label", i)
		if strings.TrimSpace(col.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if len(col.Label) > wbLabelMax {
			errs = append(errs, errMaxLength(name, labelPath, wbLabelMax, len(col.Label)))
		}

		typePath := fmt.Sprintf("columns[%d].type", i)
		switch col.Type {
		case wbTypeTotal, wbTypeDelta, wbTypeSubtotal:
			// ok
		case "":
			errs = append(errs, errRequired(name, typePath))
		default:
			errs = append(errs, &ValidationError{
				Pattern: name,
				Path:    typePath,
				Code:    ErrCodeUnknownKey,
				Message: fmt.Sprintf("%s: %s must be one of \"total\", \"delta\", \"subtotal\"; got %q", name, typePath, col.Type),
				Fix:     UseOneOfFix(typePath, []string{wbTypeTotal, wbTypeDelta, wbTypeSubtotal}),
			})
		}
	}

	if len(vals.Unit) > wbUnitMax {
		errs = append(errs, errMaxLength(name, "unit", wbUnitMax, len(vals.Unit)))
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Columns), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// resolvedColumn carries the geometry data computed for one column.
type resolvedColumn struct {
	label      string
	typ        string
	value      float64 // the rendered value (auto-filled for subtotal)
	yStart     float64 // chart-space y at bar start
	yEnd       float64 // chart-space y at bar end
	isNegDelta bool
}

// resolveColumns walks the columns left-to-right, computing running total,
// each bar's chart-space y-range, and filling in auto-computed subtotals.
func resolveColumns(columns []WaterfallBridgeColumn) []resolvedColumn {
	out := make([]resolvedColumn, len(columns))
	running := 0.0
	for i, col := range columns {
		r := resolvedColumn{label: col.Label, typ: col.Type, value: col.Value}
		switch col.Type {
		case wbTypeTotal:
			r.yStart = 0
			r.yEnd = col.Value
			running = col.Value
		case wbTypeDelta:
			r.yStart = running
			r.yEnd = running + col.Value
			r.isNegDelta = col.Value < 0
			running += col.Value
		case wbTypeSubtotal:
			// Auto-compute when value omitted (zero) or when the supplied
			// value disagrees with the running total: trust the running total
			// (subtotals are derived, not authored).
			r.value = running
			r.yStart = 0
			r.yEnd = running
		}
		out[i] = r
	}
	return out
}

// chartRange computes [yMin, yMax] across all resolved columns, ensuring 0 is
// included (the baseline) so total bars anchor to the bottom.
func chartRange(cols []resolvedColumn) (yMin, yMax float64) {
	yMin = 0
	yMax = 0
	for _, c := range cols {
		if c.yStart < yMin {
			yMin = c.yStart
		}
		if c.yEnd < yMin {
			yMin = c.yEnd
		}
		if c.yStart > yMax {
			yMax = c.yStart
		}
		if c.yEnd > yMax {
			yMax = c.yEnd
		}
	}
	return yMin, yMax
}

func (w *waterfallBridge) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*WaterfallBridgeValues)
	if !ok {
		return nil, fmt.Errorf("waterfall-bridge: values must be *WaterfallBridgeValues, got %T", values)
	}
	ovr := &WaterfallBridgeOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*WaterfallBridgeOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("waterfall-bridge: overrides must be *WaterfallBridgeOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	// Negative and subtotal fills carry meaning, so they come from the
	// template's declared semantic_accents when it has them; the literals are
	// only the fallback for templates that declare none (go-slide-creator-noa7).
	negativeAccent := ovr.NegativeAccent
	if negativeAccent == "" {
		negativeAccent = ctx.SemanticAccentOr("negative", "accent2")
	}
	subtotalAccent := ovr.SubtotalAccent
	if subtotalAccent == "" {
		subtotalAccent = ctx.SemanticAccentOr("neutral", "accent3")
	}
	positiveAccent := ctx.SemanticAccentOr("positive", "")
	labelSize := ResolveSize(ovr.LabelSize, ResolveSize(ovr.HeaderSize, 9.0))
	valueSize := ResolveSize(ovr.ValueSize, ResolveSize(ovr.BodySize, 10.0))

	resolved := resolveColumns(vals.Columns)
	yMin, yMax := chartRange(resolved)
	scale := yMax - yMin
	if scale <= 0 {
		scale = 1
	}

	n := len(resolved)
	barAreaPt := waterfallBarAreaPt(ctx)
	barCells := make([]*jsonschema.GridCellInput, n)
	labelCells := make([]*jsonschema.GridCellInput, n)

	for i, col := range resolved {
		// Choose bar fill by type.
		fill := baseAccent
		switch col.typ {
		case wbTypeSubtotal:
			fill = subtotalAccent
		case wbTypeDelta:
			switch {
			case col.isNegDelta:
				fill = negativeAccent
			case positiveAccent != "" && ovr.CellAccentMode == "":
				// A positive delta is "good"; use the template's declared
				// positive accent unless the author asked for a per-cell
				// rotation, which they own.
				fill = positiveAccent
			default:
				fill = ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
			}
		case wbTypeTotal:
			fill = baseAccent
		}

		// Compute the bar's vertical placement as percentages of the bar-area row.
		barTopY := math.Max(col.yStart, col.yEnd)
		barBottomY := math.Min(col.yStart, col.yEnd)
		topPct := (yMax - barTopY) / scale * 100
		barPct := (barTopY - barBottomY) / scale * 100
		bottomPct := (barBottomY - yMin) / scale * 100

		// Tiny bars (zero-value totals or deltas) get a 2pct minimum so the
		// label still fits visually.
		if barPct < 2 {
			barPct = 2
			// Squeeze proportionally from the larger spacer.
			if topPct >= bottomPct {
				topPct = math.Max(0, topPct-2)
			} else {
				bottomPct = math.Max(0, bottomPct-2)
			}
		}
		// Renormalise to 100% (floating-point cleanup).
		total := topPct + barPct + bottomPct
		if total > 0 {
			topPct = topPct * 100 / total
			barPct = barPct * 100 / total
			bottomPct = bottomPct * 100 / total
		}

		valueText := formatWaterfallBridgeValue(col.value, vals.Unit, col.typ == wbTypeDelta)
		barCells[i] = &jsonschema.GridCellInput{
			Grid: buildWaterfallColumnGrid(ctx, wbColumnLayout{
				fill:      fill,
				valueText: valueText,
				valueSize: valueSize,
				topPct:    topPct,
				barPct:    barPct,
				bottomPct: bottomPct,
				labelUp:   !col.isNegDelta,
				// Bridge lines: the level this column is entered at (from the
				// previous column) and left at (towards the next one).
				inOnTop:  col.typ != wbTypeDelta || col.isNegDelta,
				outOnTop: !col.isNegDelta,
				hasIn:    i > 0,
				hasOut:   i < n-1,
				areaPt:   barAreaPt,
			}),
		}

		// Per-column override accent bar: render along the left edge of the
		// label cell so it doesn't fight the bar's own fill.
		labelCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
				Text:     buildWaterfallBridgeLabelText(pptx.ConvertMarkdownEmphasis(col.label), labelSize),
			},
		}
		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*WaterfallBridgeCellOverride); ok2 && cellOvr.AccentBar {
				labelCell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    baseAccent,
					Width:    2,
				}
			}
		}
		labelCells[i] = labelCell
	}

	colsJSON, _ := json.Marshal(n)

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  0.01, // columns carry their own inner gutter so bridge lines can cross it
		RowGap:  4,
		Rows: []jsonschema.GridRowInput{
			{Height: 85, Cells: barCells},
			{Height: 15, Cells: labelCells},
		},
	}
	return grid, nil
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type waterfallBridgeParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type waterfallBridgeTextObj struct {
	Paragraphs    []waterfallBridgeParagraph `json:"paragraphs"`
	Align         string                     `json:"align"`
	VerticalAlign string                     `json:"vertical_align"`
}

func buildWaterfallBridgeValueTextAnchored(value string, size float64, color, anchor string) json.RawMessage {
	textObj := waterfallBridgeTextObj{
		Paragraphs: []waterfallBridgeParagraph{
			{Content: value, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: anchor,
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildWaterfallBridgeLabelText(label string, size float64) json.RawMessage {
	textObj := waterfallBridgeTextObj{
		Paragraphs: []waterfallBridgeParagraph{
			{Content: label, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// formatWaterfallBridgeValue renders a numeric value with optional unit suffix
// and a leading "+" / "-" sign for deltas so positive vs. negative direction is
// unambiguous in the label.
func formatWaterfallBridgeValue(v float64, unit string, signed bool) string {
	return FormatMagnitudeLabel(v, unit, signed)
}

// wbColumnLayout describes one waterfall column's vertical split (percent of
// the bar area) and bridge-line attachment.
type wbColumnLayout struct {
	fill                      string
	valueText                 string
	valueSize                 float64
	topPct, barPct, bottomPct float64
	labelUp                   bool // preferred side for an outside label
	inOnTop, outOnTop         bool // bridge levels sit on the bar's top edge?
	hasIn, hasOut             bool // bridge line from the previous / to the next column
	areaPt                    float64
}

const (
	// wbGutterPct is each column's inner gutter (left and right) — the gap
	// between bars. The bridge lines run across it.
	wbGutterPct = 7.0
	// wbBridgeLinePt is the bridge line thickness.
	wbBridgeLinePt = 0.75
)

// waterfallBarAreaPt estimates the bar-area row height in points (85% of the
// content height) for label-fit and line-thickness decisions.
func waterfallBarAreaPt(ctx ExpandContext) float64 {
	_, h := expandContentSize(ctx)
	if h <= 0 {
		h = shapegrid.DefaultBounds(12192000, 6858000).CY
	}
	return float64(h) / 12700 * 0.85
}

// waterfallLabelSide returns where a column's value label goes: "bar" when
// the bar can hold one line of value text, otherwise the adjacent spacer
// ("top" / "bottom", preferring l.labelUp) that has room.
func waterfallLabelSide(l wbColumnLayout, topPct, barPct, bottomPct float64) string {
	needPt := l.valueSize*1.5 + 4
	if l.areaPt <= 0 || l.areaPt*barPct/100 >= needPt {
		return "bar"
	}
	first, second := "top", "bottom"
	firstPct, secondPct := topPct, bottomPct
	if !l.labelUp {
		first, second = second, first
		firstPct, secondPct = secondPct, firstPct
	}
	switch {
	case l.areaPt*firstPct/100 >= needPt:
		return first
	case l.areaPt*secondPct/100 >= needPt:
		return second
	}
	return "bar"
}

// buildWaterfallColumnGrid builds a column's sub-grid:
//
//	[top spacer] [top bridge line] [bar] [bottom bridge line] [bottom spacer]
//
// on a [gutter, bar, gutter] column split. Bridge lines are hairline rows at
// the bar edge the running total enters / leaves at; the entering segment
// covers the left gutter and the leaving segment the right gutter, so
// adjacent columns' segments meet across the gap at the same level. A bar too
// thin to hold its value label gets the label in the adjacent spacer (above
// for totals and increases, below for decreases) in dark text.
func buildWaterfallColumnGrid(ctx ExpandContext, l wbColumnLayout) *jsonschema.ShapeGridInput {
	linePct := 0.0
	if l.areaPt > 0 {
		linePct = wbBridgeLinePt / l.areaPt * 100
	}
	topPct, barPct, bottomPct := l.topPct, l.barPct, l.bottomPct

	type edgeLine struct{ left, right bool }
	var top, bottom edgeLine
	if l.hasIn {
		if l.inOnTop {
			top.left = true
		} else {
			bottom.left = true
		}
	}
	if l.hasOut {
		if l.outOnTop {
			top.right = true
		} else {
			bottom.right = true
		}
	}
	// Carve line rows out of the adjacent spacer (or the bar when flush).
	take := func(spacer *float64) {
		if *spacer >= linePct+0.5 {
			*spacer -= linePct
		} else {
			barPct -= linePct
		}
	}
	if top.left || top.right {
		take(&topPct)
	}
	if bottom.left || bottom.right {
		take(&bottomPct)
	}

	labelSide := waterfallLabelSide(l, topPct, barPct, bottomPct)

	spacer := func(pct float64, side, anchor string) jsonschema.GridRowInput {
		shape := &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
		}
		if labelSide == side {
			shape.Text = buildWaterfallBridgeValueTextAnchored(l.valueText, l.valueSize, "dk1", anchor)
		}
		return jsonschema.GridRowInput{Height: pct, Cells: []*jsonschema.GridCellInput{{ColSpan: 3, Shape: shape}}}
	}
	lineRow := func(e edgeLine) jsonschema.GridRowInput {
		line := func(span int) *jsonschema.GridCellInput {
			return &jsonschema.GridCellInput{ColSpan: span, Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`{"color":"dk1","lumMod":50000,"lumOff":50000}`),
			}}
		}
		var cells []*jsonschema.GridCellInput
		switch {
		case e.left && e.right:
			cells = []*jsonschema.GridCellInput{line(3)}
		case e.left:
			cells = []*jsonschema.GridCellInput{line(2)}
		default:
			cells = []*jsonschema.GridCellInput{{}, line(2)}
		}
		return jsonschema.GridRowInput{Height: linePct, Cells: cells}
	}

	barShape := &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, l.fill)),
	}
	if labelSide == "bar" {
		barShape.Text = buildWaterfallBridgeValueTextAnchored(l.valueText, l.valueSize,
			readableTextOn(ctx, fillTone{Color: l.fill}, "lt1"), "ctr")
	}

	var rows []jsonschema.GridRowInput
	if topPct > 0.5 {
		rows = append(rows, spacer(topPct, "top", "b"))
	}
	if (top.left || top.right) && linePct > 0 {
		rows = append(rows, lineRow(top))
	}
	rows = append(rows, jsonschema.GridRowInput{
		Height: barPct,
		Cells:  []*jsonschema.GridCellInput{{}, {Shape: barShape}},
	})
	if (bottom.left || bottom.right) && linePct > 0 {
		rows = append(rows, lineRow(bottom))
	}
	if bottomPct > 0.5 {
		rows = append(rows, spacer(bottomPct, "bottom", "t"))
	}

	cols, _ := json.Marshal([]float64{wbGutterPct, 100 - 2*wbGutterPct, wbGutterPct})
	return &jsonschema.ShapeGridInput{
		Columns: cols,
		ColGap:  0.01,
		RowGap:  0.01,
		Rows:    rows,
	}
}
