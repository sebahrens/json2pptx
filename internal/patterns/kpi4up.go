package patterns

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// kpi-4up pattern — four big-number KPIs with short captions
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&kpi4up{})
}

type kpi4up struct{}

func (k *kpi4up) Name() string        { return "kpi-4up" }
func (k *kpi4up) Description() string { return "Four big-number KPI cards with short captions" }
func (k *kpi4up) UseWhen() string     { return "Four big-number KPIs with short captions" }
func (k *kpi4up) Version() int        { return 1 }
func (k *kpi4up) CellsHint() string   { return "4" }

func (k *kpi4up) ExemplarValues() any {
	v := Kpi4upValues{
		{Big: "$4.2M", Small: "ARR"},
		{Big: "127%", Small: "NRR"},
		{Big: "12d", Small: "Sales cycle"},
		{Big: "98%", Small: "CSAT"},
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Kpi4upValues is the values type: exactly 4 KPI cells.
type Kpi4upValues = []KPICell

// Kpi4upOverrides contains pattern-level overrides for kpi-4up.
type Kpi4upOverrides = KPIOverrides

// Kpi4upCellOverride contains per-cell overrides for kpi-4up.
type Kpi4upCellOverride = KPICellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (k *kpi4up) NewValues() any       { return &Kpi4upValues{} }
func (k *kpi4up) NewOverrides() any    { return &Kpi4upOverrides{} }
func (k *kpi4up) NewCellOverride() any { return &Kpi4upCellOverride{} }

func (k *kpi4up) Schema() *Schema {
	cellOverride := kpiCellOverrideSchema()
	cellOverrideProps := make(map[string]*Schema, 4)
	for i := range 4 {
		cellOverrideProps[strconv.Itoa(i)] = cellOverride
	}

	return ObjectSchema(
		map[string]*Schema{
			"values":         ArraySchema(kpiCellSchema(), 4, 4).WithDescription("Exactly 4 KPI cells"),
			"overrides":      kpiOverridesSchema(),
			"cell_overrides": ObjectSchema(cellOverrideProps, nil).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDescription("Four big-number KPI cards with short captions")
}

func (k *kpi4up) Validate(values, overrides any, cellOverrides map[int]any) error {
	cells, ok := values.(*Kpi4upValues)
	if !ok || cells == nil {
		return fmt.Errorf("kpi-4up: values must be []KPICell, got %T", values)
	}
	return validateKPICells("kpi-4up", *cells, 4, "kpi-3up", cellOverrides)
}

func (k *kpi4up) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	cells, ok := values.(*Kpi4upValues)
	if !ok {
		return nil, fmt.Errorf("kpi-4up: values must be *Kpi4upValues, got %T", values)
	}
	ovr := &KPIOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*KPIOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("kpi-4up: overrides must be *KPIOverrides, got %T", overrides)
		}
	}

	accent := resolveKPIAccent(ovr)
	bigSize := resolveKPIBigSize(ovr)
	smallSize := resolveKPISmallSize(ovr)

	gridCells := make([]*jsonschema.GridCellInput, 4)
	for i, cell := range *cells {
		textContent := buildKPITextContent(cell.Big, bigSize, cell.Small, smallSize)
		fillJSON := json.RawMessage(fmt.Sprintf(`"%s"`, accent))

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     fillJSON,
			Text:     textContent,
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
			// Apply text-level overrides: emphasis, align, vertical_align, font_size, color
			shape.Text = applyKPICellTextOverrides(shape.Text, cellOvr)
		}

		gridCells[i] = gc
	}

	colsJSON := json.RawMessage(`4`)
	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		Gap:     12,
		Rows: []jsonschema.GridRowInput{
			{Cells: gridCells},
		},
	}

	return grid, nil
}
