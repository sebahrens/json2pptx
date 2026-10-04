package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// kpiNup — parametric adapter for kpi-Nup patterns (N = 2..6)
// ---------------------------------------------------------------------------

// KPINupConfig defines a parametric KPI variant.
type KPINupConfig struct {
	Count        int       // exact cell count (2..6)
	DensityClass string    // taxonomy density: "low", "medium", "high"
	Exemplars    []KPICell // canonical example values
}

// kpiNup is a parametric Pattern implementation for KPI grids of N cells.
type kpiNup struct {
	cfg KPINupConfig
}

// NewKPINup creates a Pattern for a kpi-Nup variant.
func NewKPINup(cfg KPINupConfig) Pattern {
	return &kpiNup{cfg: cfg}
}

func (k *kpiNup) Name() string {
	return fmt.Sprintf("kpi-%dup", k.cfg.Count)
}

func (k *kpiNup) Description() string {
	return fmt.Sprintf("%s big-number KPI cards with short captions", numberWord(k.cfg.Count))
}

func (k *kpiNup) UseWhen() string {
	return fmt.Sprintf("Exactly %d big-number KPIs with short captions; prefer stat-hero for a single dominant metric, card-grid when items need multi-line body text, metric-list when 3–7 numbers read top to bottom each with a label and detail line", k.cfg.Count)
}

func (k *kpiNup) NotWhen() string {
	return "Items need multi-line descriptions (use card-grid), a single metric should dominate (use stat-hero), or items are not numeric KPIs (use icon-row)"
}

func (k *kpiNup) Version() int { return 2 }

func (k *kpiNup) CellsHint() string { return strconv.Itoa(k.cfg.Count) }

func (k *kpiNup) Taxonomy() PatternTaxonomy {
	density := k.cfg.DensityClass
	if density == "" {
		density = "medium"
	}
	return PatternTaxonomy{
		Category:           "data-display",
		NarrativeRole:      []string{"evidence"},
		PairsWith:          []string{"process-flow", "comparison-2col", "card-grid"},
		ComposesWith:       []string{"stylish-panels", "pull-quote", "process-flow", "icon-row"},
		RoleOnSlide:        []string{"banner", "foundation"},
		DensityClass:       density,
		AccentWeight:       "strong",
		SparseThresholdPct: 15,
	}
}

func (k *kpiNup) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: k.cfg.Count, Rows: 1},
	}
}

func (k *kpiNup) ExemplarValues() any {
	v := make([]KPICell, len(k.cfg.Exemplars))
	copy(v, k.cfg.Exemplars)
	vals := KPINupValues(v)
	return &vals
}

func (k *kpiNup) NewValues() any       { return &KPINupValues{} }
func (k *kpiNup) NewOverrides() any    { return &KPIOverrides{} }
func (k *kpiNup) NewCellOverride() any { return &KPICellOverride{} }

func (k *kpiNup) Schema() *Schema {
	n := k.cfg.Count
	return ObjectSchema(
		map[string]*Schema{
			"values":         ArraySchema(kpiCellSchema(kpiNupBigMaxChars, kpiComparatorMaxChars), n, n).WithDescription(fmt.Sprintf("Exactly %d KPI cells; metric values have a 12-character hard maximum and a measured fit warning when they cannot stay on one line", n)),
			"overrides":      kpiOverridesSchema(EnumSchema(kpiNupStyles...).WithDescription("open: value and caption on the canvas between hairline dividers. tiles: each KPI in a tinted card under an accent rule. Default open for plain value + caption cells, tiles when a cell has an icon or the row sets semantic_accent or a non-uniform cell_accent_mode")),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription(k.Description())
}

func (k *kpiNup) Validate(values, overrides any, cellOverrides map[int]any) error {
	name := k.Name()
	cells, ok := values.(*KPINupValues)
	if !ok || cells == nil {
		return fmt.Errorf("%s: values must be []KPICell, got %T", name, values)
	}

	// Validate cell_accent_mode and style
	var accentModeErr error
	if overrides != nil {
		if ovr, ok := overrides.(*KPIOverrides); ok && ovr != nil {
			accentModeErr = ValidateCellAccentMode(name, ovr.CellAccentMode)
			if ovr.Style != "" && !slices.Contains(kpiNupStyles, ovr.Style) {
				accentModeErr = errors.Join(accentModeErr, errInvalidEnum(name, "overrides.style", ovr.Style, kpiNupStyles))
			}
		}
	}

	// Find the nearest sibling for swap hints.
	siblingHint := kpiSiblingHint(k.cfg.Count, len(*cells))
	cellErr := validateKPICells(name, *cells, k.cfg.Count, siblingHint, cellOverrides)

	if accentModeErr != nil {
		return errors.Join(accentModeErr, cellErr)
	}
	return cellErr
}

// nupOverrides returns the typed overrides (never nil) and whether the value
// had the right type.
func nupOverrides(overrides any) (*KPIOverrides, bool) {
	if overrides == nil {
		return &KPIOverrides{}, true
	}
	ovr, ok := overrides.(*KPIOverrides)
	if !ok {
		return &KPIOverrides{}, false
	}
	if ovr == nil {
		return &KPIOverrides{}, true
	}
	return ovr, true
}

// layout measures the row in the style it will be drawn in: an open strip
// has a hairline gap between its cells, tiles the card gap.
func (k *kpiNup) layout(ctx ExpandContext, cells []KPICell, ovr *KPIOverrides) (kpiRowLayout, bool) {
	open := ovr.Style == kpiStyleOpen || (ovr.Style == "" && kpiDefaultOpen(cells, ovr))
	gap := ctx.Gap(kpiCardGapPt)
	if open {
		gap = ctx.Gap(kpiOpenGapPt)
	}
	return layoutKPIRow(ctx, cells, ovr, gap), open
}

// PostExpandWarnings applies the same hard-ceiling/soft-fit distinction as
// card-grid. A legal metric may still be too wide for the chosen template,
// density, icon position, or font at the readable 16pt floor.
func (k *kpiNup) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	cells, ok := values.(*KPINupValues)
	if !ok || cells == nil || len(*cells) == 0 {
		return nil
	}
	ovr, _ := nupOverrides(overrides)
	lay, _ := k.layout(ctx, *cells, ovr)
	var warnings []string
	for i, cell := range *cells {
		width := lay.geo.valueWidthPt(cell.Icon, lay.iconPos)
		// The same atomic-token width kpiFitBigSize fits against: a value that
		// only fits edge-to-edge in a stand-in face renders as "$4.2" / "M"
		// (go-slide-creator-b7qqg.14).
		if !kpiValueFits(cell.Big, ctx.Theme.BodyFont, lay.bigSize, width) {
			warnings = append(warnings, fmt.Sprintf("%s: %s values[%d].big cannot fit on one line at the %.0fpt effective size in a %.0fpt-wide card, which holds about %d characters like these — shorten the metric, move/remove its icon, or use fewer KPI cards", ErrCodeBodyTooLong, k.Name(), i, lay.bigSize, width, kpiValueMaxChars(cell.Big, ctx.Theme.BodyFont, lay.bigSize, width)))
		}
	}
	return warnings
}

// kpiValueMaxChars is how many characters of value fit one line at sizePt:
// the longest prefix that does, so the budget is in the value's own glyphs
// (digits and capitals run wider than body text).
func kpiValueMaxChars(value, font string, sizePt, widthPt float64) int {
	runes := []rune(strings.TrimSpace(value))
	for n := len(runes) - 1; n > 0; n-- {
		if kpiValueFits(strings.TrimSpace(string(runes[:n])), font, sizePt, widthPt) {
			return n
		}
	}
	return 1
}

func (k *kpiNup) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	name := k.Name()
	n := k.cfg.Count

	cells, ok := values.(*KPINupValues)
	if !ok {
		return nil, fmt.Errorf("%s: values must be *[]KPICell, got %T", name, values)
	}
	ovr, ovrOk := nupOverrides(overrides)
	if !ovrOk {
		return nil, fmt.Errorf("%s: overrides must be *KPIOverrides, got %T", name, overrides)
	}

	baseAccent := resolveKPIAccent(ovr, ctx)
	cellAccentMode := ovr.CellAccentMode

	lay, open := k.layout(ctx, *cells, ovr)
	if !lay.fits {
		// A row whose area cannot hold a value over its caption is refused:
		// written anyway, the lines shrink into each other
		// (go-slide-creator-uj9zq).
		return nil, errKPIRowTooTall(name, len(*cells), lay)
	}

	gridCells := make([]*jsonschema.GridCellInput, n)
	for i, cell := range *cells {
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		text := lay.texts[i]

		var gc *jsonschema.GridCellInput
		if open {
			text.valueInk, text.ink = kpiOpenInks(ctx, accent)
			gc = kpiOpenCell(ctx, i, text.json())
		} else {
			gc = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     text.json(),
			}}
		}
		shape := gc.Shape
		if lay.pinned {
			// A value with no room to grow on its line keeps its size, and its
			// siblings with it: the grid's type step wrote "EUR 48.2M" a step
			// larger than the size it was fitted at, where it broke at the
			// space (go-slide-creator-kjrxx).
			shape.TypeScale = peerTextTypeScale
		}
		if cell.Icon != nil {
			if icon := cell.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), lay.iconPos); icon != nil {
				icon.Scale = lay.iconScales[i]
				shape.Icon = icon
			}
		}

		// Apply cell overrides (D15)
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*KPICellOverride)
			if !coOk {
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

	gap := ctx.Gap(kpiCardGapPt)
	if open {
		gap = ctx.Gap(kpiOpenGapPt)
	}
	colsJSON := json.RawMessage(strconv.Itoa(n))
	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		Gap:     gap,
		Rows: []jsonschema.GridRowInput{
			{
				Cells:     gridCells,
				MaxHeight: math.Round(lay.rowPt),
			},
		},
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// numberWord returns a capitalized English word for small numbers.
func numberWord(n int) string {
	switch n {
	case 2:
		return "Two"
	case 3:
		return "Three"
	case 4:
		return "Four"
	case 5:
		return "Five"
	case 6:
		return "Six"
	default:
		return strconv.Itoa(n)
	}
}

// kpiSiblingHint picks the best sibling pattern name for a swap suggestion
// when the user provides the wrong number of cells.
func kpiSiblingHint(expectedCount, actualCount int) string {
	if actualCount >= 2 && actualCount <= 6 && actualCount != expectedCount {
		return fmt.Sprintf("kpi-%dup", actualCount)
	}
	return ""
}
