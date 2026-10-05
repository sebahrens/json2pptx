package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// before-after pattern — two-column From → To with full-height chevron separator
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&beforeAfter{})
}

type beforeAfter struct{}

// The panels carry the uniform 0.5 cm shape text margin on every edge.
const beforeAfterPanelInsetPt = defaultShapeInsetLRPt

func (b *beforeAfter) Name() string        { return "before-after" }
func (b *beforeAfter) Description() string { return "Two-column before/after with transition chevron" }
func (b *beforeAfter) UseWhen() string {
	return "Exactly two states showing transformation (before→after, current→future); prefer comparison-2col when comparing options without temporal change"
}
func (b *beforeAfter) NotWhen() string {
	return "Comparing two options without a temporal dimension (use comparison-2col), or more than two states (use process-flow or timeline-horizontal)"
}
func (b *beforeAfter) Version() int      { return 1 }
func (b *beforeAfter) CellsHint() string { return "2 + header" }
func (b *beforeAfter) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"compare"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (b *beforeAfter) SupportsCallout() bool        { return true }
func (b *beforeAfter) SupportsInlineMarkdown() bool { return true }

func (b *beforeAfter) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 2},
	}
}

func (b *beforeAfter) ExemplarValues() any {
	return &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Current State", Items: []string{"Manual process", "3-day turnaround", "Error-prone"}},
		After:  BeforeAfterColumn{Header: "Future State", Items: []string{"Automated", "Same-day", "99.9% accuracy"}},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// BeforeAfterColumn holds header + bullet items for one side.
type BeforeAfterColumn struct {
	Header string   `json:"header"`
	Items  []string `json:"items"`
}

// BeforeAfterValues holds the before and after columns.
type BeforeAfterValues struct {
	Before BeforeAfterColumn `json:"before"`
	After  BeforeAfterColumn `json:"after"`
}

// BeforeAfterOverrides is the standard text overrides plus the style and the
// optional emphasis of both before-after variants.
type BeforeAfterOverrides struct {
	TextOverrides
	// Style is "open" (default: each state is a heading over a rule and an
	// open bullet list — a neutral rule for the before state, the accent rule
	// for the after state) or "panels" (a header tile over a body tile per
	// state, the look before go-slide-creator-xvpu2).
	Style string `json:"style,omitempty"`
	// Emphasis fills one state's header with the solid accent in the open
	// style: "before" or "after". Omitted, nothing is filled.
	Emphasis string `json:"emphasis,omitempty"`
}

var (
	// beforeAfterStyles are the accepted overrides.style values.
	beforeAfterStyles = []string{"open", "panels"}
	// beforeAfterEmphases are the accepted overrides.emphasis values.
	beforeAfterEmphases = []string{"before", "after"}
)

// beforeAfterOpen reports whether a before-after variant renders open.
func beforeAfterOpen(ovr *BeforeAfterOverrides) bool {
	return ovr == nil || ovr.Style != "panels"
}

// beforeAfterRulePt is the rule under an open state's heading.
const beforeAfterRulePt = 1.5

// beforeAfterOverridesSchema is the overrides schema both variants share.
func beforeAfterOverridesSchema() *Schema {
	sch := textOverridesSchema()
	sch.raw.Properties["style"] = EnumSchema(beforeAfterStyles...).WithDescription("open (default: each state is a heading over a rule and an open bullet list; the after state carries the accent rule) or panels (a header tile over a body tile per state)").WithDefault("open")
	sch.raw.Properties["emphasis"] = EnumSchema(beforeAfterEmphases...).WithDescription("Open style: fill this state's header with the solid accent, the only solid fill (before or after)")
	return sch
}

// validateBeforeAfterOverrides checks the overrides both variants share.
func validateBeforeAfterOverrides(name string, overrides any) []error {
	ovr, ok := overrides.(*BeforeAfterOverrides)
	if !ok || ovr == nil {
		return nil
	}
	var errs []error
	if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
		errs = append(errs, err)
	}
	if ovr.Style != "" && !slices.Contains(beforeAfterStyles, ovr.Style) {
		errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, beforeAfterStyles))
	}
	if ovr.Emphasis != "" && !slices.Contains(beforeAfterEmphases, ovr.Emphasis) {
		errs = append(errs, errInvalidEnum(name, "overrides.emphasis", ovr.Emphasis, beforeAfterEmphases))
	}
	return errs
}

// BeforeAfterCellOverride is the shared per-cell override.
type BeforeAfterCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (b *beforeAfter) NewValues() any       { return &BeforeAfterValues{} }
func (b *beforeAfter) NewOverrides() any    { return &BeforeAfterOverrides{} }
func (b *beforeAfter) NewCellOverride() any { return &BeforeAfterCellOverride{} }

// PostExpandWarnings measures the headers and bullets against the template's
// content area when it is known (go-slide-creator-n1muf).
func (b *beforeAfter) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*BeforeAfterValues)
	if !ok || v == nil {
		return nil
	}
	return beforeAfterAreaWarning(ctx, v, overrides, beforeAfterFullVariant)
}

func (b *beforeAfter) Schema() *Schema {
	columnSchema := ObjectSchema(
		map[string]*Schema{
			"header": StringSchema(60).WithDescription("Column header"),
			"items":  ArraySchema(StringSchema(200), 1, 8).WithDescription("Bullet items (1-8)"),
		},
		[]string{"header", "items"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"before": columnSchema.WithDescription("Left column (current/before state)"),
			"after":  columnSchema.WithDescription("Right column (future/after state)"),
		},
		[]string{"before", "after"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      beforeAfterOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Two-column before/after with transition chevron")
}

func (b *beforeAfter) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*BeforeAfterValues)
	if !ok || vals == nil {
		return fmt.Errorf("before-after: values must be *BeforeAfterValues, got %T", values)
	}

	const name = "before-after"
	var errs []error

	errs = append(errs, validateBeforeAfterOverrides(name, overrides)...)

	// Validate before column
	if vals.Before.Header == "" {
		errs = append(errs, errRequired(name, "before.header"))
	} else if runeLen(vals.Before.Header) > 60 {
		errs = append(errs, errMaxLength(name, "before.header", 60, runeLen(vals.Before.Header)))
	}
	if len(vals.Before.Items) == 0 {
		errs = append(errs, errMinItems(name, "before.items", 1, 0, ""))
	}
	if len(vals.Before.Items) > 8 {
		errs = append(errs, errMaxItems(name, "before.items", 8, len(vals.Before.Items), ""))
	}
	for i, item := range vals.Before.Items {
		path := fmt.Sprintf("before.items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(item) > 200 {
			errs = append(errs, errMaxLength(name, path, 200, runeLen(item)))
		}
	}

	// Validate after column
	if vals.After.Header == "" {
		errs = append(errs, errRequired(name, "after.header"))
	} else if runeLen(vals.After.Header) > 60 {
		errs = append(errs, errMaxLength(name, "after.header", 60, runeLen(vals.After.Header)))
	}
	if len(vals.After.Items) == 0 {
		errs = append(errs, errMinItems(name, "after.items", 1, 0, ""))
	}
	if len(vals.After.Items) > 8 {
		errs = append(errs, errMaxItems(name, "after.items", 8, len(vals.After.Items), ""))
	}
	for i, item := range vals.After.Items {
		path := fmt.Sprintf("after.items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(item) > 200 {
			errs = append(errs, errMaxLength(name, path, 200, runeLen(item)))
		}
	}

	// Total cells: 2 headers + chevron + 2 body cells = 5
	totalCells := 5
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (b *beforeAfter) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*BeforeAfterValues)
	if !ok {
		return nil, fmt.Errorf("before-after: values must be *BeforeAfterValues, got %T", values)
	}
	ovr := &BeforeAfterOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*BeforeAfterOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("before-after: overrides must be *BeforeAfterOverrides, got %T", overrides)
		}
	}

	plan := beforeAfterLayout(ctx, vals, ovr, cellOverrides, beforeAfterFullVariant)
	cells := plan.cells

	// 3-column grid: [45%, 10%, 45%]
	colsJSON := json.RawMessage(`[45, 10, 45]`)

	// Content-sized rows (go-slide-creator-3i7c): the header band is ~1.2x
	// the header line height (not 25% of the slide) and the body row hugs the
	// longest bullet list; the grid centres the block vertically. Neither row
	// is below the written fit of its text (go-slide-creator-n1muf).
	grid := &jsonschema.ShapeGridInput{
		Columns:       colsJSON,
		Gap:           ctx.Gap(beforeAfterFullVariant.gapPt),
		VerticalAlign: GridVerticalAlignDefault,
		Rows:          plan.gridRows(),
	}
	if plan.rowGap != ctx.Gap(beforeAfterFullVariant.gapPt) {
		grid.RowGap = plan.rowGap
	}
	if beforeAfterOpen(ovr) {
		plan.ruleRows(grid)
		return grid, nil
	}
	// No stretch-to-fill (go-slide-creator-wntyw): the panels hug their
	// bullets and the block is middle-anchored in the body zone. Stretching
	// them to 60% of the zone left four bullets floating in a 200pt panel.
	// The padding that remains is split evenly above and below the list.
	for _, cell := range []*jsonschema.GridCellInput{cells.beforeBody, cells.afterBody} {
		cell.Shape.Text = withVerticalAlign(cell.Shape.Text, "ctr")
	}

	return grid, nil
}

// beforeAfterVariant is the geometry and default type of one before-after
// variant.
type beforeAfterVariant struct {
	name                 string
	headerSize, bodySize float64
	minHeaderSize        float64 // the header type floor stepped down to
	gapPt                float64
	heightPct            float64 // the block's share of the content area
	growHeight           bool    // the height cap gives way before type
}

var (
	beforeAfterFullVariant    = beforeAfterVariant{name: "before-after", headerSize: sizeHeaderPt, bodySize: scaleBodyPt, minHeaderSize: scaleSubheadPt, gapPt: 8, heightPct: 100}
	beforeAfterCompactVariant = beforeAfterVariant{name: "before-after-compact", headerSize: scaleSubheadPt, bodySize: scaleDenseBodyPt, minHeaderSize: scaleBodyPt, gapPt: 6, heightPct: 60, growHeight: true}
)

// beforeAfterMinRowGapPt is the row gap the header / body gap gives way to.
const beforeAfterMinRowGapPt = 4.0

// beforeAfterCells are the five cells of either variant, in cell-override
// order.
type beforeAfterCells struct {
	beforeHeader, chevron, afterHeader, beforeBody, afterBody *jsonschema.GridCellInput
	// beforeRule / afterRule are the rules under the open style's headings;
	// nil in the panels style.
	beforeRule, afterRule *jsonschema.GridCellInput
}

// beforeAfterPlan is the chosen layout: the cells at the chosen header size,
// the row heights, the row gap and the block's height share.
type beforeAfterPlan struct {
	cells               beforeAfterCells
	headerPt, bodyMaxPt float64
	bodyNeedPt          float64
	rowGap, heightPct   float64
	textW               float64
	need, avail         float64
	fits                bool
}

// gridRows is the header row and the body row. The header band is pinned at
// its height; the body row never drops below the written fit of its bullets.
// When the block cannot fit at all, the body takes whatever the header leaves
// (the writer shrinks it and BODY_TOO_LONG reports it) rather than squeezing
// the header too.
func (p beforeAfterPlan) gridRows() []jsonschema.GridRowInput {
	c := p.cells
	bodyMin := p.bodyNeedPt
	if !p.fits {
		bodyMin = 0
	}
	return []jsonschema.GridRowInput{
		{
			MinHeight: p.headerPt,
			MaxHeight: p.headerPt,
			Cells:     []*jsonschema.GridCellInput{c.beforeHeader, c.chevron, c.afterHeader},
		},
		{
			MinHeight: bodyMin,
			MaxHeight: p.bodyMaxPt,
			Cells:     []*jsonschema.GridCellInput{c.beforeBody, c.afterBody},
		},
	}
}

// ruleRows turns the two-row grid into the open style's three: the rules under
// the headings take the gap the header and body tiles had between them (two
// half gaps and the rule are as tall as that gap), so the block keeps its
// height and its measured budgets. The chevron spans all three rows.
func (p beforeAfterPlan) ruleRows(grid *jsonschema.ShapeGridInput) {
	c := p.cells
	if c.beforeRule == nil || c.afterRule == nil || len(grid.Rows) != 2 {
		return
	}
	colGap := grid.Gap
	grid.Gap, grid.ColGap = 0, colGap
	grid.RowGap = math.Max((p.rowGap-beforeAfterRulePt)/2, 0.01)
	c.chevron.RowSpan = 3
	grid.Rows = []jsonschema.GridRowInput{
		grid.Rows[0],
		{MinHeight: beforeAfterRulePt, MaxHeight: beforeAfterRulePt, Cells: []*jsonschema.GridCellInput{c.beforeRule, c.afterRule}},
		grid.Rows[1],
	}
}

// rowTextNeedPt is the written-fit height a row needs for text in a shape
// widthPt wide. writtenFitHeightPt returns its minPt (0) for text that still
// shrinks after its longest probe; such text needs more than any content area
// holds, so it reports rowTextBeyondAreaPt instead of nothing.
func rowTextNeedPt(fonts pptx.ThemeFonts, text json.RawMessage, widthPt float64) float64 {
	if len(text) == 0 {
		return 0
	}
	if h := writtenFitHeightPt(fonts, text, widthPt, 0); h > 0 {
		return h
	}
	return rowTextBeyondAreaPt
}

// labelRowNeedPt is the row height text in a shape widthPt wide needs once a
// renderer's face runs shapegrid.RenderFaceSlack wider than the measured one.
// need is its rowTextNeedPt at widthPt. Text that wraps here already has the
// lines the engine measured for it and keeps need; text on one line that the
// wider face would wrap takes the height of the wrapped lines, so the wrap
// costs a line and the renderer has no reason to shrink the cell alone.
func labelRowNeedPt(fonts pptx.ThemeFonts, text json.RawMessage, widthPt, need float64) float64 {
	inner := widthPt - 2*defaultShapeInsetLRPt
	if inner <= 0 || need >= rowTextBeyondAreaPt {
		return need
	}
	// One line per paragraph is what the text needs in a shape of any width.
	if rowTextNeedPt(fonts, text, labelUnwrappedWidthPt) < need {
		return need
	}
	return math.Max(need, rowTextNeedPt(fonts, text, inner/shapegrid.RenderFaceSlack+2*defaultShapeInsetLRPt))
}

// labelHoldsLine reports whether text sits on one line per paragraph in a
// shape widthPt wide, and still does once a renderer's face runs
// shapegrid.RenderFaceSlack wider.
func labelHoldsLine(fonts pptx.ThemeFonts, text json.RawMessage, widthPt float64) bool {
	inner := widthPt - 2*defaultShapeInsetLRPt
	if inner <= 0 {
		return false
	}
	return rowTextNeedPt(fonts, text, inner/shapegrid.RenderFaceSlack+2*defaultShapeInsetLRPt) <= rowTextNeedPt(fonts, text, labelUnwrappedWidthPt)
}

// labelUnwrappedWidthPt is a shape width no label wraps in.
const labelUnwrappedWidthPt = 20000.0

// writtenFitsAt reports whether the writer stores text in a widthPt × heightPt
// shape without a shrink.
func writtenFitsAt(fonts pptx.ThemeFonts, text json.RawMessage, widthPt, heightPt float64) bool {
	tb, err := shapegrid.ResolveTextInput(text)
	if err != nil || tb == nil {
		return true
	}
	tb.ThemeFonts = fonts
	return pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(widthPt * sizingEMUPerPt), CY: int64(heightPt * sizingEMUPerPt)})
}

// rowTextBeyondAreaPt stands in for the need of text taller than any slide.
const rowTextBeyondAreaPt = 1000.0

// readableNeedPhrase states a measured height need for a BODY_TOO_LONG
// message, without quoting the rowTextBeyondAreaPt stand-in as a number.
func readableNeedPhrase(needPt float64) string {
	if needPt >= rowTextBeyondAreaPt {
		return "more than a slide holds"
	}
	return fmt.Sprintf("about %.0fpt", needPt)
}

// beforeAfterLayout sizes both rows to the written fit of their text at the
// real column width (go-slide-creator-n1muf). When the block does not fit its
// share of the content area, the row gap gives way first, then (compact) the
// 60% height cap, then the header steps down to its floor; an authored header
// size is kept. A step is taken only when it makes the block fit, so an
// overflowing payload keeps the default layout (and reports BODY_TOO_LONG
// when the content area is known).
func beforeAfterLayout(ctx ExpandContext, vals *BeforeAfterValues, ovr *BeforeAfterOverrides, cellOverrides map[int]any, v beforeAfterVariant) beforeAfterPlan {
	headerSize := ResolveSize(ovr.HeaderSize, v.headerSize)
	type step struct{ header, gap, pct float64 }
	steps := []step{{headerSize, ctx.Gap(v.gapPt), v.heightPct}, {headerSize, ctx.Gap(beforeAfterMinRowGapPt), v.heightPct}}
	if v.growHeight {
		steps = append(steps, step{headerSize, ctx.Gap(beforeAfterMinRowGapPt), 100})
	}
	if ovr.HeaderSize == 0 && v.minHeaderSize < headerSize {
		steps = append(steps, step{v.minHeaderSize, ctx.Gap(beforeAfterMinRowGapPt), steps[len(steps)-1].pct})
	}
	var first beforeAfterPlan
	for i, st := range steps {
		plan := measureBeforeAfter(ctx, vals, ovr, cellOverrides, v, st.header, st.gap, st.pct)
		if plan.fits {
			return plan
		}
		if i == 0 {
			first = plan
		}
	}
	return first
}

// measureBeforeAfter builds the cells at one header size and measures the
// rows: the theme-font model the variants always used, floored at the written
// fit of every header and bullet list at the real column width.
func measureBeforeAfter(ctx ExpandContext, vals *BeforeAfterValues, ovr *BeforeAfterOverrides, cellOverrides map[int]any, v beforeAfterVariant, headerSize, rowGap, heightPct float64) beforeAfterPlan {
	cells := buildBeforeAfterCells(ctx, vals, ovr, cellOverrides, headerSize, ResolveSize(ovr.BodySize, v.bodySize))
	headerPt, bodyPt := beforeAfterFullRowHeights(ctx, cells.beforeHeader.Shape.Text, cells.afterHeader.Shape.Text, cells.beforeBody.Shape.Text, cells.afterBody.Shape.Text, ctx.Gap(v.gapPt))
	areaW, areaH := sizingAreaPt(ctx)
	colW := (areaW - 2*ctx.Gap(v.gapPt)) * 0.45
	plan := beforeAfterPlan{cells: cells, rowGap: rowGap, heightPct: heightPct, textW: colW}
	// The header band keeps the model's height when the writer stores both
	// headers unshrunk there (it clamps a one-line band's margin); otherwise
	// it grows to their written fit.
	if !writtenFitsAt(ctx.themeFonts(), cells.beforeHeader.Shape.Text, colW, headerPt) || !writtenFitsAt(ctx.themeFonts(), cells.afterHeader.Shape.Text, colW, headerPt) {
		headerPt = math.Max(headerPt, math.Max(rowTextNeedPt(ctx.themeFonts(), cells.beforeHeader.Shape.Text, colW), rowTextNeedPt(ctx.themeFonts(), cells.afterHeader.Shape.Text, colW)))
	}
	plan.headerPt = headerPt
	plan.bodyNeedPt = math.Max(rowTextNeedPt(ctx.themeFonts(), cells.beforeBody.Shape.Text, colW), rowTextNeedPt(ctx.themeFonts(), cells.afterBody.Shape.Text, colW))
	plan.bodyMaxPt = math.Max(bodyPt, plan.bodyNeedPt)
	plan.avail = areaH * heightPct / 100
	plan.need = plan.headerPt + rowGap + plan.bodyNeedPt
	plan.fits = plan.need <= plan.avail
	return plan
}

// beforeAfterShrinks reports whether the writer would store a shrink for a
// header or bullet list of plan.
func beforeAfterShrinks(ctx ExpandContext, plan beforeAfterPlan) bool {
	// Past the fit the header keeps its band and the body takes the rest.
	bodyH := math.Min(plan.bodyNeedPt, plan.avail-plan.rowGap-plan.headerPt)
	c := plan.cells
	for _, cell := range []struct {
		text json.RawMessage
		h    float64
	}{
		{c.beforeHeader.Shape.Text, plan.headerPt}, {c.afterHeader.Shape.Text, plan.headerPt},
		{c.beforeBody.Shape.Text, bodyH}, {c.afterBody.Shape.Text, bodyH},
	} {
		tb, err := shapegrid.ResolveTextInput(cell.text)
		if err != nil || tb == nil {
			continue
		}
		tb.ThemeFonts = ctx.themeFonts()
		if !pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(plan.textW * sizingEMUPerPt), CY: int64(cell.h * sizingEMUPerPt)}) {
			return true
		}
	}
	return false
}

// beforeAfterAreaWarning is the measured BODY_TOO_LONG for either variant
// when the template's content area is known and the rows do not fit it.
func beforeAfterAreaWarning(ctx ExpandContext, vals *BeforeAfterValues, overrides any, v beforeAfterVariant) []string {
	if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return nil
	}
	ovr, _ := overrides.(*BeforeAfterOverrides)
	if ovr == nil {
		ovr = &BeforeAfterOverrides{}
	}
	plan := beforeAfterLayout(ctx, vals, ovr, nil, v)
	if plan.fits || !beforeAfterShrinks(ctx, plan) {
		return nil
	}
	return []string{fmt.Sprintf("%s: %s headers and bullets need %s at readable sizes but the content area holds about %.0fpt — shorten or merge bullets, use fewer bullets, or split the slide", ErrCodeBodyTooLong, v.name, readableNeedPhrase(plan.need), plan.avail)}
}

// buildBeforeAfterCells builds the header, chevron and body cells of either
// variant and applies the cell overrides.
func buildBeforeAfterCells(ctx ExpandContext, vals *BeforeAfterValues, ovr *BeforeAfterOverrides, cellOverrides map[int]any, headerSize, bodySize float64) beforeAfterCells {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	cellAccentMode := ovr.CellAccentMode

	beforeAccent := ctx.ResolveCellAccent(baseAccent, 0, cellAccentMode)
	afterAccent := ctx.ResolveCellAccent(baseAccent, 1, cellAccentMode)

	var c beforeAfterCells
	// Header row: Before header | compact transition chevron | After header.
	c.beforeHeader = &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, beforeAccent)),
			Text:     buildBeforeAfterTextContent(vals.Before.Header, headerSize, true, "lt1", "l"),
		},
	}

	c.chevron = beforeAfterChevronCell(baseAccent)
	applyBeforeAfterCellOverride(c.chevron, cellOverrides, 1, baseAccent)

	c.afterHeader = &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, afterAccent)),
			Text:     buildBeforeAfterTextContent(vals.After.Header, headerSize, true, "lt1", "l"),
		},
	}

	// Body row: before items | after items on neutral panels. The chevron
	// reserves the middle column through its row span, so no separate spacer
	// is needed.
	c.beforeBody = &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     beforeAfterPanelTone(beforeAccent).fillJSON(),
			Text:     buildBeforeAfterBulletContent(vals.Before.Items, bodySize),
		},
	}

	c.afterBody = &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     beforeAfterPanelTone(afterAccent).fillJSON(),
			Text:     buildBeforeAfterBulletContent(vals.After.Items, bodySize),
		},
	}
	if beforeAfterOpen(ovr) {
		openBeforeAfterCells(ctx, &c, vals, ovr, headerSize, beforeAccent, afterAccent)
	}
	applyBeforeAfterCellOverride(c.beforeHeader, cellOverrides, 0, beforeAccent)
	applyBeforeAfterCellOverride(c.afterHeader, cellOverrides, 2, afterAccent)
	applyBeforeAfterCellOverride(c.beforeBody, cellOverrides, 3, beforeAccent)
	applyBeforeAfterCellOverride(c.afterBody, cellOverrides, 4, afterAccent)
	return c
}

// beforeAfterNeutralRule is the before state's rule: the state being left
// behind is set in neutral ink, and the accent marks where the slide is going.
var beforeAfterNeutralRule = fillTone{Color: "dk1", Alpha: 40}

// openBeforeAfterCells restyles the cells for the open style
// (go-slide-creator-xvpu2): each state is a bold heading over a rule and an
// open bullet list. The two states differ by their rules — neutral for
// before, the accent for after — not by filled boxes. overrides.emphasis
// fills one heading with the solid accent, the slide's only solid fill.
func openBeforeAfterCells(ctx ExpandContext, c *beforeAfterCells, vals *BeforeAfterValues, ovr *BeforeAfterOverrides, headerSize float64, beforeAccent, afterAccent string) {
	none := json.RawMessage(`"none"`)
	rule := func(fill json.RawMessage) *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: fill, Line: noLine}}
	}
	c.beforeRule = rule(beforeAfterNeutralRule.fillJSON())
	c.afterRule = rule(accentFillJSON(afterAccent))
	for _, st := range []struct {
		header *jsonschema.GridCellInput
		body   *jsonschema.GridCellInput
		text   string
		accent string
		name   string
	}{
		{c.beforeHeader, c.beforeBody, vals.Before.Header, beforeAccent, "before"},
		{c.afterHeader, c.afterBody, vals.After.Header, afterAccent, "after"},
	} {
		st.body.Shape.Fill, st.body.Shape.Line = none, noLine
		st.header.Shape.Fill, st.header.Shape.Line = none, noLine
		st.header.Shape.Text = buildBeforeAfterTextContent(st.text, headerSize, true, "dk1", "l")
		if ovr.Emphasis == st.name {
			tone, ink := accentFillAndInk(ctx, fillTone{Color: st.accent}, 4.5)
			st.header.Shape.Fill = tone.fillJSON()
			st.header.Shape.Text = buildBeforeAfterTextContent(st.text, headerSize, true, ink, "l")
		}
	}
}

// beforeAfterChevronPt is the transition chevron's size: a small marker
// centred in the gutter between the two panels, not a full-height block
// (go-slide-creator-7z5we).
const beforeAfterChevronPt = 24.0

// beforeAfterChevronCell is the compact transition chevron. It spans both
// rows so it centres on the whole block; max_height + fit "contain" hold it
// to a beforeAfterChevronPt square whatever the column width.
func beforeAfterChevronCell(accent string) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		RowSpan:   2,
		MaxHeight: beforeAfterChevronPt,
		Fit:       "contain",
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "chevron",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
		},
	}
}

// Size both before-after variants against their actual 0.5 cm text insets.
func beforeAfterFullRowHeights(ctx ExpandContext, beforeHeader, afterHeader, beforeBody, afterBody json.RawMessage, gapPt float64) (headerPt, bodyPt float64) {
	font := ctx.Theme.BodyFont
	w, _ := contentAreaPt(ctx)
	textW := (w-2*gapPt)*0.45 - 2*beforeAfterPanelInsetPt
	header := math.Max(shapeTextHeightPt(font, beforeHeader, textW), shapeTextHeightPt(font, afterHeader, textW))
	body := math.Max(shapeTextHeightPt(font, beforeBody, textW), shapeTextHeightPt(font, afterBody, textW))
	headerPt = math.Round(header + 2*beforeAfterPanelInsetPt)
	bodyPt = math.Round(body + 2*beforeAfterPanelInsetPt + cardPadPt)
	return headerPt, bodyPt
}

// Keep only a trace of the accent in the panel fill. The shape-grid renderer
// only emits tint modifiers for scheme colours, so pre-mix explicit hex
// overrides with white instead.
func beforeAfterPanelTone(string) fillTone {
	// A neutral surface, not a pale accent wash: the two panels are peers,
	// and accent tints are reserved for marking one highlighted band
	// (go-slide-creator-8xsj3).
	return neutralTone(NeutralTint4)
}

func buildBeforeAfterTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
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

func buildBeforeAfterBulletContent(items []string, size float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
		Bullet  bool    `json:"bullet,omitempty"`
	}

	paras := make([]paragraph, len(items))
	for i, item := range items {
		paras[i] = paragraph{
			Bullet:  true,
			Content: pptx.ConvertMarkdownEmphasis(item),
			Size:    size,
			Color:   "dk1",
			Align:   "l",
		}
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func applyBeforeAfterCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*BeforeAfterCellOverride)
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
