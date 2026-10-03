package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// kpi-inline — horizontal inline KPIs, height-capped for supporting context
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&kpiInline{})
}

type kpiInline struct{}

func (k *kpiInline) Name() string { return "kpi-inline" }
func (k *kpiInline) Description() string {
	return "Horizontal inline KPI bar, height-capped for supporting context"
}
func (k *kpiInline) UseWhen() string {
	return "2-6 KPIs as a compact supporting bar (not the hero); prefer kpi-Nup when KPIs are the main slide content, stat-hero for a single dominant metric"
}
func (k *kpiInline) NotWhen() string {
	return "KPIs are the primary slide content (use kpi-Nup), a single metric should dominate (use stat-hero), or items need multi-line descriptions (use card-grid)"
}
func (k *kpiInline) Version() int      { return 2 }
func (k *kpiInline) CellsHint() string { return "2-6" }
func (k *kpiInline) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "data-display",
		NarrativeRole:      []string{"evidence"},
		PairsWith:          []string{"process-flow", "before-after", "pull-quote", "card-grid"},
		DensityClass:       "low",
		AccentWeight:       "normal",
		SparseThresholdPct: 25,
	}
}

func (k *kpiInline) ExemplarValues() any {
	vals := KPINupValues{
		{Big: "$4.2M", Small: "ARR"},
		{Big: "127%", Small: "NRR"},
		{Big: "12d", Small: "Sales cycle"},
	}
	return &vals
}

func (k *kpiInline) NewValues() any    { return &KPINupValues{} }
func (k *kpiInline) NewOverrides() any { return &KPIInlineOverrides{} }

// KPIInlineOverrides is the KPI overrides plus the bar style.
type KPIInlineOverrides struct {
	KPIOverrides
	// Style is "open" (value and caption on the canvas between hairline
	// dividers; the default for plain cells), "tinted" (neutral cells with
	// dark text under a thin accent rule, like the kpi-Nup tiles; the default
	// when a cell has an icon or the bar sets semantic_accent or a
	// non-uniform cell_accent_mode) or "solid" (every KPI a solid accent
	// block; legacy look).
	Style string `json:"style,omitempty"`
}

// kpiInlineStyles are the accepted overrides.style values.
var kpiInlineStyles = []string{kpiStyleOpen, kpiStyleTinted, kpiStyleSolid}

// kpiInlineOverridesSchema is the KPI overrides schema with the bar's styles.
func kpiInlineOverridesSchema() *Schema {
	return kpiOverridesSchema(EnumSchema(kpiInlineStyles...).WithDescription("open: value and caption on the canvas between hairline dividers, so a supporting bar does not outshout the slide. tinted: neutral cells with dark text under a thin accent rule, matching the kpi-Nup tiles. solid: every KPI a solid accent block (legacy look). Default open for plain value + caption cells, tinted when a cell has an icon or the bar sets semantic_accent or a non-uniform cell_accent_mode"))
}
func (k *kpiInline) NewCellOverride() any { return &KPICellOverride{} }

// Measured by TestKPIInlineBudgetProbe across every shipped template at
// default sizes, every shape keeping the uniform 0.5 cm text margin. The icon
// consumes horizontal space and a delta line vertical space; the caption keeps
// what is left at the readable floor.
func kpiInlineCaptionBudget(cells, _, subChars int, icon bool) int {
	sub := subChars > 0
	switch {
	case icon && cells == 5 && sub:
		return 16
	case icon && cells == 5:
		return 31
	case icon && cells >= 6 && sub:
		return 11
	case icon && cells >= 6:
		return 21
	case cells <= 3:
		return 40
	case cells == 4 && sub:
		return 35
	case cells == 4:
		return 40
	case cells == 5 && sub:
		return 25
	case cells == 5:
		return 40
	case sub:
		return 20
	default:
		return 38
	}
}

func (k *kpiInline) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*KPINupValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	for i, cell := range *v {
		icon := cell.Icon != nil && !cell.Icon.IsEmpty()
		budget := kpiInlineCaptionBudget(len(*v), runeLen(cell.Big), runeLen(cell.Sub), icon)
		if budget == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: kpi-inline values[%d] combines an icon, %d-character number and %d-character delta in a %d-KPI bar; no readable caption fits — shorten the number, omit the delta/icon, or use fewer KPIs", ErrCodeBodyTooLong, i, runeLen(cell.Big), runeLen(cell.Sub), len(*v)))
		} else if n := runeLen(cell.Small); n > budget {
			warnings = append(warnings, fmt.Sprintf("%s: kpi-inline values[%d].small is %d characters; a %d-KPI bar with icon=%t, %d-character number and %d-character delta holds about %d caption characters — shorten the caption or simplify the cell", ErrCodeBodyTooLong, i, n, len(*v), icon, runeLen(cell.Big), runeLen(cell.Sub), budget))
		}
	}
	return warnings
}

func (k *kpiInline) Schema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"values":         ArraySchema(kpiCellSchema(kpiInlineBigMaxChars, 0), 2, 6).WithDescription("2-6 KPI cells in a compact bar. Metric values have a tighter 8-character maximum than full-size KPI cards. Caption budget without icons: 40 chars (38 at 6 KPIs); with a delta 35/25/20 at 4/5/6 KPIs. With icons: 31 at 5 KPIs (16 with a delta), 21 at 6 KPIs (11 with a delta)."),
			"overrides":      kpiInlineOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Horizontal inline KPI bar, height-capped at ~25% of content area")
}

func (k *kpiInline) Validate(values, overrides any, cellOverrides map[int]any) error {
	const name = "kpi-inline"
	cells, ok := values.(*KPINupValues)
	if !ok || cells == nil {
		return fmt.Errorf("%s: values must be []KPICell, got %T", name, values)
	}

	var errs []error

	if ovr, ok := overrides.(*KPIInlineOverrides); ok && ovr != nil {
		if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
			errs = append(errs, err)
		}
		if ovr.Style != "" && !slices.Contains(kpiInlineStyles, ovr.Style) {
			errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, kpiInlineStyles))
		}
	}

	if len(*cells) < 2 {
		errs = append(errs, errMinItems(name, "values", 2, len(*cells), ""))
	}
	if len(*cells) > 6 {
		errs = append(errs, errMaxItems(name, "values", 6, len(*cells), ""))
	}

	for i, cell := range *cells {
		bigPath := fmt.Sprintf("values[%d].big", i)
		if cell.Big == "" {
			errs = append(errs, errRequired(name, bigPath))
		} else if runeLen(cell.Big) > kpiInlineBigMaxChars {
			errs = append(errs, errMaxLength(name, bigPath, kpiInlineBigMaxChars, runeLen(cell.Big)))
		}
		smallPath := fmt.Sprintf("values[%d].small", i)
		if cell.Small == "" {
			errs = append(errs, errRequired(name, smallPath))
		} else if runeLen(cell.Small) > 40 {
			errs = append(errs, errMaxLength(name, smallPath, 40, runeLen(cell.Small)))
		}
		if subLength := runeLen(cell.Sub); subLength > kpiSubMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].sub", i), kpiSubMaxChars, subLength))
		}
		if cell.Comparator != "" {
			// The compact bar is height-capped: a comparator line would push
			// every caption below the readable floor.
			errs = append(errs, newValidationError(name, fmt.Sprintf("values[%d].comparator", i), ErrCodeUnknownKey,
				fmt.Sprintf("%s: values[%d].comparator: the height-capped kpi-inline bar has no comparator line — use a kpi-Nup card row for KPIs read against plan or prior year, or fold the reference into the delta (\"+4 vs plan\")", name, i),
				RemoveFieldFix(fmt.Sprintf("values[%d].comparator", i))))
		}
		if cell.Icon != nil {
			iconPath := fmt.Sprintf("values[%d].icon", i)
			errs = append(errs, validateIconRef(name, iconPath, *cell.Icon)...)
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(*cells), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// kpiInlineBandPct is the share of the content height the bar is capped at.
const kpiInlineBandPct = 25

// kpiInlineIconScale mirrors shapegrid's default overlay scale: the bar's
// left icon is that share of the cell's shorter side.
const kpiInlineIconScale = 0.6

// style resolves the bar's container style.
func (o *KPIInlineOverrides) style(cells []KPICell) string {
	switch {
	case o.Style != "":
		return o.Style
	case kpiDefaultOpen(cells, &o.KPIOverrides):
		return kpiStyleOpen
	}
	return kpiStyleTinted
}

func (k *kpiInline) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	const name = "kpi-inline"

	cells, ok := values.(*KPINupValues)
	if !ok {
		return nil, fmt.Errorf("%s: values must be *[]KPICell, got %T", name, values)
	}
	ovr := &KPIInlineOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*KPIInlineOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("%s: overrides must be *KPIInlineOverrides, got %T", name, overrides)
		}
	}

	baseAccent := resolveKPIAccent(&ovr.KPIOverrides, ctx)
	// Open by default: three saturated accent bars dominated a slide this
	// pattern only supports (go-slide-creator-061ag), and a row of tinted
	// tiles is still a row of cards (go-slide-creator-8zles).
	style := ovr.style(*cells)
	// Smaller sizes for inline variant
	bigSize := ResolveSize(ovr.BigSize, sizeFigurePt)
	smallSize := ResolveSize(ovr.SmallSize, scaleDenseBodyPt)
	cellAccentMode := ovr.CellAccentMode

	n := len(*cells)
	gap := ctx.Gap(10)
	if style == kpiStyleOpen {
		gap = ctx.Gap(kpiOpenGapPt)
	}
	areaW, areaH := contentAreaPt(ctx)
	cardW := equalColumnWidthPt(areaW, n, gap)
	bandPt := areaH * kpiInlineBandPct / 100
	// A left icon narrows the text beside it.
	widthOf := func(i int) float64 {
		w := cardW - 2*defaultShapeInsetLRPt
		if icon := (*cells)[i].Icon; icon != nil && !icon.IsEmpty() {
			w -= math.Min(kpiInlineIconScale*math.Min(cardW, bandPt), kpiLeftIconMaxWFrac*cardW) + 2*kpiIconGapPt
		}
		return w
	}
	pads := kpiCaptionPadLines(ctx.Theme.BodyFont, *cells, smallSize, widthOf)

	rowPt := 0.0
	gridCells := make([]*jsonschema.GridCellInput, n)
	for i, cell := range *cells {
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		// Compact bars reserve no empty delta line: the text is top-anchored,
		// so a cell without a delta just ends one line earlier.
		text := kpiText{
			big: cell.Big, bigSize: bigSize, small: cell.Small, smallSize: smallSize, padLines: pads[i],
			sub: cell.Sub, valueInk: "lt1", ink: "lt1",
		}

		var gc *jsonschema.GridCellInput
		switch style {
		case kpiStyleOpen:
			text.valueInk, text.ink = kpiOpenInks(ctx, accent)
			gc = kpiOpenCell(ctx, i, text.json())
		case kpiStyleSolid:
			gc = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     text.json(),
			}}
		default:
			gc = &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     neutralFillJSON(NeutralTint4),
					Line:     noLine,
					Text:     recolorTextInk(text.json(), "lt1", "dk1"),
				},
				AccentBar: &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: peerRuleWidthPt},
			}
		}
		shape := gc.Shape
		// The bar is as tall as its tallest cell needs to store its text
		// unshrunk, never more than the band.
		rowPt = math.Max(rowPt, writtenFitHeightPt(ctx.themeFonts(), shape.Text, widthOf(i)+2*defaultShapeInsetLRPt, 0))
		if cell.Icon != nil {
			if icon := cell.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), "left"); icon != nil {
				shape.Icon = icon
			}
		}

		if co, coOk := cellOverrides[i]; coOk {
			cellOvr, ok2 := co.(*KPICellOverride)
			if !ok2 {
				gridCells[i] = gc
				continue
			}
			if cellOvr.AccentBar {
				gc.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
			shape.Text = applyCellTextOverrideToText(shape.Text, cellOvr)
		}

		gridCells[i] = gc
	}

	row := jsonschema.GridRowInput{Cells: gridCells}
	bandPct := float64(kpiInlineBandPct)
	if rowPt > 0 {
		row.MaxHeight = math.Ceil(rowPt + kpiRowSlackPt)
		// On a short content area a quarter of the height does not hold a
		// value, a caption and a delta: the band grows to the row rather
		// than have the writer shrink the caption below its floor.
		if row.MaxHeight > bandPt && areaH > 0 {
			bandPct = math.Min(100, math.Ceil(row.MaxHeight/areaH*100))
		}
	}
	colsJSON := json.RawMessage(strconv.Itoa(n))
	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: bandPct,
		},
		Columns: colsJSON,
		Gap:     gap,
		Rows:    []jsonschema.GridRowInput{row},
		// The bar hangs from the top of its band, where it has always sat.
		VerticalAlign: "top",
	}

	return grid, nil
}
