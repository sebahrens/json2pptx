package patterns

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// kpi-3up pattern — three big-number KPIs with short captions
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&kpi3up{})
}

type kpi3up struct{}

func (k *kpi3up) Name() string        { return "kpi-3up" }
func (k *kpi3up) Description() string { return "Three big-number KPI cards with short captions" }
func (k *kpi3up) UseWhen() string     { return "Three big-number KPIs with short captions" }
func (k *kpi3up) Version() int        { return 1 }

// ---------------------------------------------------------------------------
// Types — aliases to shared KPI types for backward compatibility
// ---------------------------------------------------------------------------

// Kpi3upCell is a single KPI cell: a big number and a short caption.
type Kpi3upCell = KPICell

// Kpi3upValues is the values type: exactly 3 KPI cells.
type Kpi3upValues = []KPICell

// Kpi3upOverrides contains pattern-level overrides for kpi-3up.
type Kpi3upOverrides = KPIOverrides

// Kpi3upCellOverride contains per-cell overrides for kpi-3up.
type Kpi3upCellOverride = KPICellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (k *kpi3up) NewValues() any       { return &Kpi3upValues{} }
func (k *kpi3up) NewOverrides() any    { return &Kpi3upOverrides{} }
func (k *kpi3up) NewCellOverride() any { return &Kpi3upCellOverride{} }

func (k *kpi3up) Schema() *Schema {
	cellOverride := kpiCellOverrideSchema()
	cellOverrideProps := make(map[string]*Schema, 3)
	for i := range 3 {
		cellOverrideProps[strconv.Itoa(i)] = cellOverride
	}

	return ObjectSchema(
		map[string]*Schema{
			"values":         ArraySchema(kpiCellSchema(), 3, 3).WithDescription("Exactly 3 KPI cells"),
			"overrides":      kpiOverridesSchema(),
			"cell_overrides": ObjectSchema(cellOverrideProps, nil).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDescription("Three big-number KPI cards with short captions")
}

func (k *kpi3up) Validate(values, overrides any, cellOverrides map[int]any) error {
	cells, ok := values.(*Kpi3upValues)
	if !ok || cells == nil {
		return fmt.Errorf("kpi-3up: values must be []KPICell, got %T", values)
	}
	return validateKPICells("kpi-3up", *cells, 3, "kpi-4up", cellOverrides)
}

func (k *kpi3up) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	cells, ok := values.(*Kpi3upValues)
	if !ok {
		return nil, fmt.Errorf("kpi-3up: values must be *Kpi3upValues, got %T", values)
	}
	ovr := &KPIOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*KPIOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("kpi-3up: overrides must be *KPIOverrides, got %T", overrides)
		}
	}

	accent := resolveKPIAccent(ovr)
	bigSize := resolveKPIBigSize(ovr)
	smallSize := resolveKPISmallSize(ovr)

	gridCells := make([]*jsonschema.GridCellInput, 3)
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

		// Apply cell overrides
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
		}

		gridCells[i] = gc
	}

	colsJSON := json.RawMessage(`3`)
	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		Gap:     12,
		Rows: []jsonschema.GridRowInput{
			{Cells: gridCells},
		},
	}

	return grid, nil
}

// buildKPITextContent creates a JSON text object with paragraphs for a KPI cell.
func buildKPITextContent(big string, bigSize float64, small string, smallSize float64) json.RawMessage {
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
			{Content: big, Size: bigSize, Bold: true, Color: "lt1", Align: "ctr"},
			{Content: small, Size: smallSize, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
