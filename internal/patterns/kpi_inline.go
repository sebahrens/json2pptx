package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
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

func (k *kpiInline) NewValues() any       { return &KPINupValues{} }
func (k *kpiInline) NewOverrides() any    { return &KPIOverrides{} }
func (k *kpiInline) NewCellOverride() any { return &KPICellOverride{} }

// Measured by TestKPIInlineBudgetProbe across all four bundled templates at
// default sizes. The icon consumes horizontal space; number and delta lines
// then determine whether the caption can stay at the readable floor.
func kpiInlineCaptionBudget(cells, bigChars, subChars int, icon bool) int {
	if !icon || cells <= 4 {
		return 40
	}
	if cells == 5 {
		if bigChars >= 8 {
			if subChars > 0 {
				return 0
			}
			return 16
		}
		if subChars > 0 {
			return 31
		}
		return 40
	}
	if bigChars >= 6 {
		if subChars > 0 {
			return 0
		}
		return 11
	}
	if subChars == 0 {
		return 31
	}
	if subChars >= 11 {
		return 11
	}
	return 21
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
			"values":         ArraySchema(kpiCellSchema(), 2, 6).WithDescription("2-6 KPI cells in a compact bar. Caption budget without icons: 40 chars. With icons at 5 KPIs: 40 if number <=7 chars/no delta, 31 with delta, or 16 for an 8-char number/no delta; an 8-char number plus delta leaves no readable caption. At 6 KPIs with icons: number <=5 chars holds 31 without delta, 21 with a 1-10 char delta, 11 with an 11-12 char delta; number >=6 chars holds 11 without delta and no readable caption with delta."),
			"overrides":      kpiOverridesSchema(),
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

	if overrides != nil {
		if ovr, ok := overrides.(*KPIOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
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
		} else if runeLen(cell.Big) > 8 {
			errs = append(errs, errMaxLength(name, bigPath, 8, runeLen(cell.Big)))
		}
		smallPath := fmt.Sprintf("values[%d].small", i)
		if cell.Small == "" {
			errs = append(errs, errRequired(name, smallPath))
		} else if runeLen(cell.Small) > 40 {
			errs = append(errs, errMaxLength(name, smallPath, 40, runeLen(cell.Small)))
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

func (k *kpiInline) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	const name = "kpi-inline"

	cells, ok := values.(*KPINupValues)
	if !ok {
		return nil, fmt.Errorf("%s: values must be *[]KPICell, got %T", name, values)
	}
	ovr := &KPIOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*KPIOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("%s: overrides must be *KPIOverrides, got %T", name, overrides)
		}
	}

	baseAccent := resolveKPIAccent(ovr, ctx)
	// Smaller sizes for inline variant
	bigSize := ResolveSize(ovr.BigSize, 24.0)
	smallSize := ResolveSize(ovr.SmallSize, 11.0)
	cellAccentMode := ovr.CellAccentMode

	n := len(*cells)
	gridCells := make([]*jsonschema.GridCellInput, n)
	for i, cell := range *cells {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		textContent := buildKPITextContent(cell.Big, bigSize, cell.Small, smallSize, cell.Sub)
		fillJSON := json.RawMessage(fmt.Sprintf(`"%s"`, accent))

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     fillJSON,
			Text:     textContent,
		}
		if cell.Icon != nil {
			if icon := cell.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), "left"); icon != nil {
				shape.Icon = icon
			}
		}

		gc := &jsonschema.GridCellInput{
			Shape: shape,
		}

		if co, coOk := cellOverrides[i]; coOk {
			cellOvr, ok2 := co.(*KPICellOverride)
			if !ok2 {
				continue
			}
			if cellOvr.AccentBar {
				gc.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
			shape.Text = applyKPICellTextOverrides(shape.Text, cellOvr)
		}

		gridCells[i] = gc
	}

	colsJSON := json.RawMessage(strconv.Itoa(n))
	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: 25,
		},
		Columns: colsJSON,
		Gap:     10,
		Rows: []jsonschema.GridRowInput{
			{Cells: gridCells},
		},
	}

	return grid, nil
}
