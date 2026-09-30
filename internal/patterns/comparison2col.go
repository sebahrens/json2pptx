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
// comparison-2col pattern — two-column compare (pros/cons, before/after)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&comparison2col{})
}

type comparison2col struct{}

func (c *comparison2col) Name() string        { return "comparison-2col" }
func (c *comparison2col) Description() string { return "Two-column comparison with optional headers" }
func (c *comparison2col) UseWhen() string {
	return "Two options evaluated side-by-side (pros/cons, option A vs B); prefer before-after when showing temporal transformation, card-grid when comparing more than 2 items"
}
func (c *comparison2col) NotWhen() string {
	return "Comparing a temporal before→after state (use before-after), more than 2 items (use card-grid), or positioned on two axes (use matrix-2x2)"
}
func (c *comparison2col) Version() int      { return 1 }
func (c *comparison2col) CellsHint() string { return "2 + header" }
func (c *comparison2col) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"compare", "evidence"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (c *comparison2col) SupportsCallout() bool        { return true }
func (c *comparison2col) SupportsInlineMarkdown() bool { return true }

func (c *comparison2col) ExemplarValues() any {
	return &Comparison2colValues{
		Headers: [2]string{"Pros", "Cons"},
		Rows: []Comparison2colRow{
			{Left: "Fast", Right: "Expensive"},
			{Left: "Reliable", Right: "Complex"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Comparison2colRow is a single row with a left and right cell.
// Supports string shorthand: "Left | Right" unmarshals to {left:"Left", right:"Right"}.
type Comparison2colRow struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

// UnmarshalJSON supports string shorthand "Left | Right" or object {left, right}.
func (r *Comparison2colRow) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) != 2 {
			return &ValidationError{
				Pattern: "comparison-2col",
				Path:    "rows[]",
				Code:    ErrCodeInvalidShape,
				Message: fmt.Sprintf("Comparison2colRow string must be \"Left | Right\", got %q", s),
				Fix:     ReshapeValueFix("rows[]", `string "Left | Right" or {"left": "...", "right": "..."}`, "left_value | right_value"),
			}
		}
		r.Left = parts[0]
		r.Right = parts[1]
		return nil
	}
	type alias Comparison2colRow
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return &ValidationError{
			Pattern: "comparison-2col",
			Path:    "rows[]",
			Code:    ErrCodeInvalidShape,
			Message: fmt.Sprintf("Comparison2colRow must be string \"Left | Right\" or {left, right}: %v", err),
			Fix:     ReshapeValueFix("rows[]", `string "Left | Right" or {"left": "...", "right": "..."}`, "left_value | right_value"),
		}
	}
	*r = Comparison2colRow(a)
	return nil
}

// Comparison2colValues is the values type for comparison-2col.
//
// Headers can be specified two ways (both optional):
//   - New: "headers": ["Left", "Right"]   (preferred, saves tokens)
//   - Legacy: "header_left": "Left", "header_right": "Right"
//
// If both are present, "headers" takes precedence. Internally the struct
// normalises to HeaderLeft / HeaderRight for Expand and Validate.
type Comparison2colValues struct {
	Headers     [2]string           `json:"headers,omitempty"`
	HeaderLeft  string              `json:"header_left,omitempty"`
	HeaderRight string              `json:"header_right,omitempty"`
	Rows        []Comparison2colRow `json:"rows"`
}

// UnmarshalJSON supports both "headers" array and legacy "header_left"/"header_right".
// If "headers" is present, it takes precedence and populates HeaderLeft/HeaderRight.
func (v *Comparison2colValues) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid recursion.
	type alias Comparison2colValues
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	// Normalise: headers array → HeaderLeft/HeaderRight
	if a.Headers != [2]string{} {
		a.HeaderLeft = a.Headers[0]
		a.HeaderRight = a.Headers[1]
	} else if a.HeaderLeft != "" || a.HeaderRight != "" {
		// Back-fill Headers from legacy fields so MarshalJSON is consistent.
		a.Headers = [2]string{a.HeaderLeft, a.HeaderRight}
	}

	*v = Comparison2colValues(a)
	return nil
}

// MarshalJSON emits the compact "headers" form when headers are present,
// omitting the legacy header_left/header_right fields.
func (v Comparison2colValues) MarshalJSON() ([]byte, error) {
	type compactForm struct {
		Headers [2]string           `json:"headers,omitempty"`
		Rows    []Comparison2colRow `json:"rows"`
	}
	if v.Headers != [2]string{} {
		return json.Marshal(compactForm{Headers: v.Headers, Rows: v.Rows})
	}
	// No headers at all — omit both fields.
	type rowsOnly struct {
		Rows []Comparison2colRow `json:"rows"`
	}
	return json.Marshal(rowsOnly{Rows: v.Rows})
}

// Comparison2colOverrides extends TextOverrides with an optional row_fill for
// body row background color (e.g. "lt2", "#F5F0E8").
type Comparison2colOverrides struct {
	TextOverrides
	// RowFill paints every body row the same background color (scheme name or
	// hex). When omitted, body rows are zebra-striped between two surface tints
	// as a grouping cue; setting row_fill disables striping.
	RowFill string `json:"row_fill,omitempty"`
	// Connectors reserves a narrow gutter between the columns and draws a
	// per-row accent connector (a small accent circle with a chevron, joined
	// to both cells by an accent rule) so each left cell reads as leading to
	// its right cell ("from → to" comparisons). Left cells gain a left accent
	// stripe and right cells an accent tint. Default false: unchanged layout.
	Connectors bool `json:"connectors,omitempty"`
}

// Comparison2colCellOverride is an alias for the shared CellOverride struct.
type Comparison2colCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (c *comparison2col) NewValues() any       { return &Comparison2colValues{} }
func (c *comparison2col) NewOverrides() any    { return &Comparison2colOverrides{} }
func (c *comparison2col) NewCellOverride() any { return &Comparison2colCellOverride{} }

// Measured by TestComparisonBudgetProbe against the written size (no run
// stored below its role floor) on every shipped template at default text
// sizes, every cell keeping the uniform 0.5 cm shape text margin
// (go-slide-creator-n1muf). A header row consumes the same height as one body
// row.
func comparisonBodyBudget(bodyRows int, headers bool) int {
	effectiveRows := bodyRows
	if headers {
		effectiveRows++
	}
	switch {
	case effectiveRows <= 3:
		return 200
	case effectiveRows == 4:
		return 193
	default:
		return 65
	}
}

// comparisonConnectorBudgetPct is the share of the plain two-column budget a
// cell keeps when overrides.connectors reserves the centre gutter: each text
// column narrows from 50% to comparisonConnectorColPct of the grid width.
const comparisonConnectorBudgetPct = 88

func (c *comparison2col) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*Comparison2colValues)
	if !ok || v == nil {
		return nil
	}
	headers := v.Headers != [2]string{} || v.HeaderLeft != "" || v.HeaderRight != ""
	budget := comparisonBodyBudget(len(v.Rows), headers)
	connectors := false
	if ovr, ok := overrides.(*Comparison2colOverrides); ok && ovr != nil && ovr.Connectors {
		connectors = true
		budget = budget * comparisonConnectorBudgetPct / 100
	}
	var warnings []string
	for i, row := range v.Rows {
		for _, field := range []struct{ name, text string }{{"left", row.Left}, {"right", row.Right}} {
			if n := runeLen(field.text); n > budget {
				warnings = append(warnings, fmt.Sprintf("%s: comparison-2col rows[%d].%s is %d characters; %d body rows with headers=%t%s hold about %d characters per cell before text shrinks below the readable minimum — shorten the cell or use fewer rows", ErrCodeBodyTooLong, i, field.name, n, len(v.Rows), headers, connectorNote(connectors), budget))
			}
		}
	}
	if len(warnings) > 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 || len(v.Rows) == 0 {
		return warnings
	}
	// The budgets assume a typical content area; with the template's own area
	// the rows are measured against it (go-slide-creator-n1muf).
	ovr, _ := overrides.(*Comparison2colOverrides)
	if ovr == nil {
		ovr = &Comparison2colOverrides{}
	}
	if plan := comparisonLayout(ctx, v, ovr, nil); !plan.fits && comparisonShrinks(plan) {
		warnings = append(warnings, fmt.Sprintf("%s: comparison-2col rows need %s at readable sizes but the content area holds about %.0fpt — shorten the cells, use fewer rows, or split the comparison", ErrCodeBodyTooLong, readableNeedPhrase(plan.total), plan.avail))
	}
	return warnings
}

func connectorNote(connectors bool) string {
	if connectors {
		return " and connectors"
	}
	return ""
}

func (c *comparison2col) Schema() *Schema {
	rowSchema := OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Left | Right\""),
		ObjectSchema(
			map[string]*Schema{
				"left":  StringSchema(200).WithDescription("Left cell; readable copy depends on row count and optional header row"),
				"right": StringSchema(200).WithDescription("Right cell; readable copy depends on row count and optional header row"),
			},
			[]string{"left", "right"},
		).WithAdditionalProperties(false),
	).WithDescription("Row: string \"Left | Right\" or {left, right}. Approximate chars per cell by body rows plus one if headers: 1-3: 200, 4: 193, 5-11: 65")

	headersSchema := ArraySchema(StringSchema(60), 2, 2).
		WithDescription("Column headers [left, right] (preferred over header_left/header_right)")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"headers":      headersSchema,
			"header_left":  StringSchema(60).WithDescription("Left column header (legacy, prefer headers)"),
			"header_right": StringSchema(60).WithDescription("Right column header (legacy, prefer headers)"),
			"rows":         ArraySchema(rowSchema, 1, 10).WithDescription("Comparison rows (1–10)"),
		},
		[]string{"rows"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":      NumberSchema(6, 120).WithDescription("Font size for headers in points"),
			"body_size":        NumberSchema(6, 120).WithDescription("Font size for body text in points"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
			"row_fill":         StringSchema(0).WithDescription("Uniform background fill for body rows (scheme name like 'lt2' or hex like '#F5F0E8'). Omit to zebra-stripe body rows between two surface tints as a grouping cue; setting this paints every body row the same color."),
			"connectors":       BooleanSchema().WithDescription("Reserve a narrow centre gutter and draw a per-row accent connector (circle + chevron joined to both cells) so each left cell reads as leading to its right cell. Left cells gain a left accent stripe, right cells an accent tint. Text columns narrow to 45% each, so per-cell budgets drop to about 88% of the plain values.").WithDefault(false),
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
	}).WithDescription("Two-column comparison with optional headers")
}

func (c *comparison2col) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*Comparison2colValues)
	if !ok || vals == nil {
		return fmt.Errorf("comparison-2col: values must be *Comparison2colValues, got %T", values)
	}

	const name = "comparison-2col"
	var errs []error

	// Validate cell_accent_mode
	if overrides != nil {
		if ovr, ok := overrides.(*Comparison2colOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Rows required and count check
	if len(vals.Rows) == 0 {
		errs = append(errs, newValidationError(name, "rows", ErrCodeMinItems,
			"comparison-2col: rows must contain at least 1 row",
			AddItemsFix("rows", 1)))
	}
	if len(vals.Rows) > 10 {
		errs = append(errs, newValidationError(name, "rows", ErrCodeMaxItems,
			fmt.Sprintf("comparison-2col: rows must contain at most 10 rows, got %d", len(vals.Rows)),
			ReduceItemsFix("rows", 10)))
	}

	// Per-row validation
	for i, row := range vals.Rows {
		leftPath := fmt.Sprintf("rows[%d].left", i)
		if row.Left == "" {
			errs = append(errs, errRequired(name, leftPath))
		} else if runeLen(row.Left) > 200 {
			errs = append(errs, errMaxLength(name, leftPath, 200, runeLen(row.Left)))
		}
		rightPath := fmt.Sprintf("rows[%d].right", i)
		if row.Right == "" {
			errs = append(errs, errRequired(name, rightPath))
		} else if runeLen(row.Right) > 200 {
			errs = append(errs, errMaxLength(name, rightPath, 200, runeLen(row.Right)))
		}
	}

	// Header length checks
	if runeLen(vals.HeaderLeft) > 60 {
		errs = append(errs, errMaxLength(name, "header_left", 60, runeLen(vals.HeaderLeft)))
	}
	if runeLen(vals.HeaderRight) > 60 {
		errs = append(errs, errMaxLength(name, "header_right", 60, runeLen(vals.HeaderRight)))
	}

	// Compute total cell count for cell_overrides validation
	totalCells := len(vals.Rows) * 2
	hasHeaders := vals.HeaderLeft != "" || vals.HeaderRight != ""
	if hasHeaders {
		totalCells += 2
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (c *comparison2col) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*Comparison2colValues)
	if !ok {
		return nil, fmt.Errorf("comparison-2col: values must be *Comparison2colValues, got %T", values)
	}
	ovr := &Comparison2colOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*Comparison2colOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("comparison-2col: overrides must be *Comparison2colOverrides, got %T", overrides)
		}
	}

	plan := comparisonLayout(ctx, vals, ovr, cellOverrides)
	plan.sizeRows()

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Gap:     comparisonGapPt,
		Rows:    plan.rows,
	}
	if plan.rowGap != comparisonGapPt {
		grid.RowGap = plan.rowGap
	}
	if ovr.Connectors {
		grid.Columns = json.RawMessage(fmt.Sprintf(`[%d, %d, %d]`, comparisonConnectorColPct, 100-2*comparisonConnectorColPct, comparisonConnectorColPct))
		grid.ColGap = comparisonConnectorColGap
	}

	return grid, nil
}

// comparisonGapPt is the default row and column gap.
const comparisonGapPt = 8.0

// comparisonPlan is one candidate layout: the rows built at a type size and
// row gap, each row's written-fit height and the height the rows share.
type comparisonPlan struct {
	rows   []jsonschema.GridRowInput
	needs  []float64
	avail  float64
	rowGap float64
	textW  float64
	fits   bool
	header bool    // rows[0] is the header row
	total  float64 // the rows' written-fit heights together
}

// sizeRows hands the rows their heights. Rows that fit keep equal flex rows
// unless one needs more than its share; then every row is floored at its
// need (floorFlexRowsAtNeeds). Rows that cannot fit share what the header
// band leaves in proportion to their needs, so the header keeps its band and
// the crowded rows take height from the sparse ones.
func (p comparisonPlan) sizeRows() {
	if p.fits {
		floorFlexRowsAtNeeds(p.rows, p.needs, p.avail)
		return
	}
	for i := range p.rows {
		if i == 0 && p.header {
			p.rows[i].MinHeight, p.rows[i].MaxHeight = math.Ceil(p.needs[i]), math.Ceil(p.needs[i])
			continue
		}
		p.rows[i].Flex = math.Max(math.Ceil(p.needs[i]), 1)
	}
}

// heights are the row heights sizeRows leads the grid to.
func (p comparisonPlan) heights() []float64 {
	out := append([]float64(nil), p.needs...)
	if p.fits {
		return out
	}
	avail, total, from := p.avail, 0.0, 0
	if p.header && len(out) > 0 {
		avail -= out[0]
		from = 1
	}
	for _, n := range out[from:] {
		total += n
	}
	for i := from; i < len(out) && total > 0; i++ {
		out[i] = math.Max(avail, 0) * out[i] / total
	}
	return out
}

// comparisonLayout sizes the rows to the written fit of their tallest cell at
// the real column width (go-slide-creator-n1muf). When the rows do not fit the
// content area at the default sizes, the row gap gives way first, then the
// type steps to the 12pt body / 14pt header floor; an authored size is kept.
// A step is taken only when it makes the rows fit, so overflowing payloads
// keep the default sizes (and report BODY_TOO_LONG when the area is known).
func comparisonLayout(ctx ExpandContext, vals *Comparison2colValues, ovr *Comparison2colOverrides, cellOverrides map[int]any) comparisonPlan {
	headerSize := ResolveSize(ovr.HeaderSize, 18.0)
	bodySize := ResolveSize(ovr.BodySize, 14.0)
	type step struct{ header, body, gap float64 }
	steps := []step{{headerSize, bodySize, comparisonGapPt}, {headerSize, bodySize, comparisonMinRowGapPt}}
	if ovr.HeaderSize == 0 || ovr.BodySize == 0 {
		st := step{headerSize, bodySize, comparisonMinRowGapPt}
		if ovr.HeaderSize == 0 {
			st.header = comparisonMinHeaderPt
		}
		if ovr.BodySize == 0 {
			st.body = comparisonMinBodyPt
		}
		steps = append(steps, st)
	}
	var first comparisonPlan
	for i, st := range steps {
		plan := comparisonMeasure(ctx, vals, ovr, cellOverrides, st.header, st.body, st.gap)
		if plan.fits {
			return plan
		}
		if i == 0 {
			first = plan
		}
	}
	return first
}

// Floors the comparison steps down to before it reports BODY_TOO_LONG.
const (
	comparisonMinRowGapPt = 4.0
	comparisonMinHeaderPt = 14.0
	comparisonMinBodyPt   = 12.0
)

// comparisonMeasure builds the rows at one type size and row gap and measures
// each row's written fit at the real text-column width.
func comparisonMeasure(ctx ExpandContext, vals *Comparison2colValues, ovr *Comparison2colOverrides, cellOverrides map[int]any, headerSize, bodySize, rowGap float64) comparisonPlan {
	areaW, areaH := sizingAreaPt(ctx)
	textW := (areaW - comparisonGapPt) / 2
	if ovr.Connectors {
		textW = (areaW - 2*comparisonConnectorColGap) * comparisonConnectorColPct / 100
	}
	rows := buildComparison2colRows(ctx, vals, ovr, cellOverrides, headerSize, bodySize)
	plan := comparisonPlan{rows: rows, rowGap: rowGap, textW: textW, header: vals.HeaderLeft != "" || vals.HeaderRight != ""}
	plan.avail = areaH - float64(len(rows)-1)*rowGap
	total := 0.0
	for _, r := range rows {
		need := 0.0
		for _, cell := range r.Cells {
			if cell != nil && cell.Shape != nil && len(cell.Shape.Text) > 0 {
				need = max(need, rowTextNeedPt(cell.Shape.Text, textW))
			}
		}
		plan.needs = append(plan.needs, need)
		total += need
	}
	plan.fits = total <= plan.avail
	plan.total = total
	if !plan.fits {
		// Rows past the fit share the area in proportion to their needs; a
		// need beyond the whole area counts as the area.
		for i := range plan.needs {
			plan.needs[i] = math.Min(plan.needs[i], plan.avail)
		}
	}
	return plan
}

// comparisonShrinks reports whether the writer would store a shrink for some
// cell of plan at the heights sizeRows leads to. A one-line cell in a short
// row can still be written whole (the writer clamps a degenerate shape's
// margin), so this is measured rather than read off the needs.
func comparisonShrinks(plan comparisonPlan) bool {
	heights := plan.heights()
	for i, r := range plan.rows {
		h := heights[i]
		for _, cell := range r.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
				continue
			}
			tb, err := shapegrid.ResolveTextInput(cell.Shape.Text)
			if err != nil || tb == nil {
				continue
			}
			if !pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(plan.textW * sizingEMUPerPt), CY: int64(h * sizingEMUPerPt)}) {
				return true
			}
		}
	}
	return false
}

// buildComparison2colRows builds the header row (when headers are given) and
// one row per comparison at the given type sizes.
func buildComparison2colRows(ctx ExpandContext, vals *Comparison2colValues, ovr *Comparison2colOverrides, cellOverrides map[int]any, headerSize, bodySize float64) []jsonschema.GridRowInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	cellAccentMode := ovr.CellAccentMode

	// Body-row fills. When no explicit row_fill is given, alternate body rows
	// between two surface tints (zebra striping). The banding is a grouping cue:
	// it makes each left/right pair read as one unit and keeps sparse 3-row
	// comparisons from looking like floating, ungrouped cells. An explicit
	// row_fill overrides striping and paints every body row the same color.
	//
	// The two bands are the template's declared subtle / paper surfaces, or
	// the dk1 neutral steps (4% / 8%) where a role is undeclared or is the
	// page colour; no row is outlined (go-slide-creator-pgdkp).
	striped := ovr.RowFill == ""
	stripeFillA, stripeFillB := surfacePairJSON(ctx) // even / odd body rows
	uniformFill := json.RawMessage(fmt.Sprintf(`"%s"`, ovr.RowFill))
	if striped && ovr.Connectors {
		// Connector rows are already grouped by the connector rule, so the
		// left column takes one neutral (non-white) surface instead of zebra
		// bands; the right column carries the accent tint.
		striped = false
		uniformFill = stripeFillA
	}

	hasHeaders := vals.HeaderLeft != "" || vals.HeaderRight != ""
	cellIdx := 0 // running cell index for cell_overrides

	var rows []jsonschema.GridRowInput

	// Header row (optional)
	if hasHeaders {
		leftAccent := ctx.ResolveCellAccent(baseAccent, 0, cellAccentMode)
		rightAccent := ctx.ResolveCellAccent(baseAccent, 1, cellAccentMode)

		leftHeader := buildComparison2colTextContent(vals.HeaderLeft, headerSize, true, "lt1", "ctr")
		rightHeader := buildComparison2colTextContent(vals.HeaderRight, headerSize, true, "lt1", "ctr")

		leftCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, leftAccent)),
				Text:     leftHeader,
			},
		}
		applyComparison2colCellOverride(leftCell, cellOverrides, cellIdx, leftAccent)
		cellIdx++

		rightCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, rightAccent)),
				Text:     rightHeader,
			},
		}
		applyComparison2colCellOverride(rightCell, cellOverrides, cellIdx, rightAccent)
		cellIdx++

		headerCells := []*jsonschema.GridCellInput{leftCell, rightCell}
		if ovr.Connectors {
			headerCells = []*jsonschema.GridCellInput{leftCell, {}, rightCell}
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: headerCells})
	}

	// Connector mode: right cells sit on an accent tint (their text colour is
	// measured against the tinted fill) and left cells gain an accent stripe.
	rightTint := inactiveTintTone(baseAccent)
	rightTintJSON := rightTint.fillJSON()
	rightTextColor := readableTextOn(ctx, rightTint, "dk1")

	// Body rows — apply inline markdown emphasis (**bold**, *italic*)
	for bi, row := range vals.Rows {
		// Rows alternate two tints and carry no outline: outlined-white
		// beside filled-grey read as a rendering bug (go-slide-creator-pgdkp).
		// An authored row_fill is kept as written.
		rowFillJSON := uniformFill
		if striped {
			rowFillJSON = stripeFillA
			if bi%2 == 1 {
				rowFillJSON = stripeFillB
			}
		}
		rowLine := noLine

		leftText := buildComparison2colTextContent(pptx.ConvertMarkdownEmphasis(row.Left), bodySize, false, "dk1", "l")
		rightColor := "dk1"
		if ovr.Connectors {
			rightColor = rightTextColor
		}
		rightText := buildComparison2colTextContent(pptx.ConvertMarkdownEmphasis(row.Right), bodySize, false, rightColor, "l")

		leftCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     rowFillJSON,
				Line:     rowLine,
				Text:     leftText,
			},
		}
		applyComparison2colCellOverride(leftCell, cellOverrides, cellIdx, baseAccent)
		cellIdx++

		rightCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     rowFillJSON,
				Line:     rowLine,
				Text:     rightText,
			},
		}
		if ovr.Connectors {
			leftCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: baseAccent, Width: 4}
			rightCell.Shape.Fill = rightTintJSON
			rightCell.Shape.Line = nil
		}
		applyComparison2colCellOverride(rightCell, cellOverrides, cellIdx, baseAccent)
		cellIdx++

		if !ovr.Connectors {
			rows = append(rows, jsonschema.GridRowInput{
				Cells: []*jsonschema.GridCellInput{leftCell, rightCell},
			})
			continue
		}
		rows = append(rows, jsonschema.GridRowInput{
			Cells:     []*jsonschema.GridCellInput{leftCell, comparison2colConnectorCell(ctx, baseAccent), rightCell},
			Connector: &jsonschema.ConnectorSpecInput{Style: "line", Color: baseAccent, Width: 1.5},
		})
	}

	return rows
}

// Connector-mode geometry: each text column keeps comparisonConnectorColPct of
// the grid width and the centre gutter takes the rest. The column gap is kept
// small so the accent rule visibly joins each cell to the connector badge.
const (
	comparisonConnectorColPct    = 45
	comparisonConnectorColGap    = 2
	comparisonConnectorBadgeSize = 24.0 // pt; the badge is a circle this tall, centred on its row
)

// comparison2colConnectorCell is the gutter cell of one connector row: a small
// accent circle carrying a chevron, vertically centred on the row. The row's
// line connector joins it to the left and right cells.
func comparison2colConnectorCell(ctx ExpandContext, accent string) *jsonschema.GridCellInput {
	fill := json.RawMessage(fmt.Sprintf(`"%s"`, accent))
	return &jsonschema.GridCellInput{
		MaxHeight: comparisonConnectorBadgeSize,
		Fit:       "contain",
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "ellipse",
			Fill:     fill,
			Icon: &jsonschema.IconInput{
				Name:  "chevron-right",
				Fill:  iconFillOn(ctx, fill, accent),
				Scale: 0.7,
			},
		},
	}
}

// buildComparison2colTextContent creates a JSON text object for a comparison cell.
func buildComparison2colTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: bold, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

// applyComparison2colCellOverride applies cell_overrides for a given cell index.
func applyComparison2colCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*Comparison2colCellOverride)
	if !coOk {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
