package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// kpiNup — parametric adapter for kpi-Nup patterns (N = 2..6)
// ---------------------------------------------------------------------------

// KPINupConfig defines a parametric KPI variant.
type KPINupConfig struct {
	Count        int              // exact cell count (2..6)
	DensityClass string           // taxonomy density: "low", "medium", "high"
	Exemplars    []KPICell        // canonical example values
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
	return fmt.Sprintf("Exactly %d big-number KPIs with short captions; prefer stat-hero for a single dominant metric, card-grid when items need multi-line body text", k.cfg.Count)
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
		Category:      "data-display",
		NarrativeRole: []string{"evidence"},
		PairsWith:     []string{"process-flow", "comparison-2col", "card-grid"},
		ComposesWith:  []string{"stylish-panels", "pull-quote", "process-flow", "icon-row"},
		RoleOnSlide:   []string{"banner", "foundation"},
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

// KPINupValues is the values type for any kpi-Nup variant.
type KPINupValues = []KPICell

func (k *kpiNup) NewValues() any       { return &KPINupValues{} }
func (k *kpiNup) NewOverrides() any    { return &KPIOverrides{} }
func (k *kpiNup) NewCellOverride() any { return &KPICellOverride{} }

func (k *kpiNup) Schema() *Schema {
	n := k.cfg.Count
	return ObjectSchema(
		map[string]*Schema{
			"values":         ArraySchema(kpiCellSchema(), n, n).WithDescription(fmt.Sprintf("Exactly %d KPI cells", n)),
			"overrides":      kpiOverridesSchema(),
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

	// Validate cell_accent_mode
	var accentModeErr error
	if overrides != nil {
		if ovr, ok := overrides.(*KPIOverrides); ok {
			accentModeErr = ValidateCellAccentMode(name, ovr.CellAccentMode)
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

func (k *kpiNup) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	name := k.Name()
	n := k.cfg.Count

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
	bigSize := resolveKPIBigSize(ovr)
	smallSize := resolveKPISmallSize(ovr)
	cellAccentMode := ovr.CellAccentMode

	gridCells := make([]*jsonschema.GridCellInput, n)
	for i, cell := range *cells {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		textContent := buildKPITextContent(cell.Big, bigSize, cell.Small, smallSize)
		fillJSON := json.RawMessage(fmt.Sprintf(`"%s"`, accent))

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     fillJSON,
			Text:     textContent,
		}
		if cell.Icon != nil {
			if icon := cell.Icon.Resolve(accent, "left"); icon != nil {
				shape.Icon = icon
			}
		}

		gc := &jsonschema.GridCellInput{
			Shape: shape,
		}

		// Apply cell overrides (D15)
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*KPICellOverride)
			if !coOk {
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
		Columns: colsJSON,
		Gap:     12,
		Rows: []jsonschema.GridRowInput{
			{Cells: gridCells},
		},
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
