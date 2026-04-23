package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

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
// Types
// ---------------------------------------------------------------------------

// Kpi3upCell is a single KPI cell: a big number and a short caption.
type Kpi3upCell struct {
	Big   string `json:"big"`
	Small string `json:"small"`
}

// Kpi3upValues is the values type: exactly 3 KPI cells.
type Kpi3upValues = []Kpi3upCell

// Kpi3upOverrides contains pattern-level overrides for kpi-3up.
type Kpi3upOverrides struct {
	Accent   string  `json:"accent,omitempty"`
	BigSize  float64 `json:"big_size,omitempty"`
	SmallSize float64 `json:"small_size,omitempty"`
}

// Kpi3upCellOverride contains per-cell overrides for kpi-3up.
type Kpi3upCellOverride struct {
	AccentBar     bool    `json:"accent_bar,omitempty"`
	Emphasis      string  `json:"emphasis,omitempty"`
	Align         string  `json:"align,omitempty"`
	VerticalAlign string  `json:"vertical_align,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	Color         string  `json:"color,omitempty"`
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (k *kpi3up) NewValues() any      { return &Kpi3upValues{} }
func (k *kpi3up) NewOverrides() any   { return &Kpi3upOverrides{} }
func (k *kpi3up) NewCellOverride() any { return &Kpi3upCellOverride{} }

// kpi3upCellOverrideAllowed is the whitelist of per-cell override keys (D15).
var kpi3upCellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

func (k *kpi3up) Schema() *Schema {
	cellSchema := ObjectSchema(
		map[string]*Schema{
			"big":   StringSchema(8).WithDescription("The big number (e.g. \"$4.2M\")"),
			"small": StringSchema(40).WithDescription("Short caption (e.g. \"ARR\")"),
		},
		[]string{"big", "small"},
	).WithAdditionalProperties(false)

	cellOverrideSchema := ObjectSchema(
		map[string]*Schema{
			"accent_bar":     BooleanSchema().WithDescription("Show accent bar decoration"),
			"emphasis":       EnumSchema("bold", "italic", "bold-italic").WithDescription("Text emphasis"),
			"align":          EnumSchema("l", "ctr", "r").WithDescription("Horizontal alignment"),
			"vertical_align": EnumSchema("t", "ctr", "b").WithDescription("Vertical alignment"),
			"font_size":      NumberSchema(6, 120).WithDescription("Font size in points"),
			"color":          StringSchema(0).WithDescription("Text color (scheme ref, e.g. \"dk1\")"),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(cellSchema, 3, 3).WithDescription("Exactly 3 KPI cells"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":     StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"big_size":   NumberSchema(6, 120).WithDescription("Font size for big number in points"),
					"small_size": NumberSchema(6, 120).WithDescription("Font size for small caption in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": ObjectSchema(
				map[string]*Schema{
					"0": cellOverrideSchema,
					"1": cellOverrideSchema,
					"2": cellOverrideSchema,
				},
				nil,
			).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDescription("Three big-number KPI cards with short captions")
}

func (k *kpi3up) Validate(values, overrides any, cellOverrides map[int]any) error {
	cells, ok := values.(*Kpi3upValues)
	if !ok || cells == nil {
		return fmt.Errorf("kpi-3up: values must be []Kpi3upCell, got %T", values)
	}

	var errs []error

	// D4: exact count with sibling hint
	if len(*cells) != 3 {
		hint := ""
		if len(*cells) == 4 {
			hint = " (hint: use pattern kpi-4up for 4 KPIs)"
		}
		errs = append(errs, fmt.Errorf("kpi-3up: values must contain exactly 3 cells, got %d%s", len(*cells), hint))
	}

	// Per-cell validation
	for i, cell := range *cells {
		if cell.Big == "" {
			errs = append(errs, fmt.Errorf("kpi-3up: values[%d].big is required", i))
		} else if len(cell.Big) > 8 {
			errs = append(errs, fmt.Errorf("kpi-3up: values[%d].big exceeds maxLength 8 (%d chars)", i, len(cell.Big)))
		}
		if cell.Small == "" {
			errs = append(errs, fmt.Errorf("kpi-3up: values[%d].small is required", i))
		} else if len(cell.Small) > 40 {
			errs = append(errs, fmt.Errorf("kpi-3up: values[%d].small exceeds maxLength 40 (%d chars)", i, len(cell.Small)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= 3 {
			errs = append(errs, fmt.Errorf("kpi-3up: cell_overrides key %d out of range [0,2]", idx))
			continue
		}
		// Check for unknown keys by marshalling and unmarshalling to a map
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("kpi-3up: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("kpi-3up: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !kpi3upCellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("kpi-3up: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

func (k *kpi3up) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	cells, ok := values.(*Kpi3upValues)
	if !ok {
		return nil, fmt.Errorf("kpi-3up: values must be *Kpi3upValues, got %T", values)
	}
	ovr := &Kpi3upOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*Kpi3upOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("kpi-3up: overrides must be *Kpi3upOverrides, got %T", overrides)
		}
	}

	accent := "accent1"
	if ovr.Accent != "" {
		accent = ovr.Accent
	}
	bigSize := 36.0
	if ovr.BigSize > 0 {
		bigSize = ovr.BigSize
	}
	smallSize := 14.0
	if ovr.SmallSize > 0 {
		smallSize = ovr.SmallSize
	}

	// Build 3 cells — one per KPI
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
			cellOvr, coOk := co.(*Kpi3upCellOverride)
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

	// Emit columns as number
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
// Uses the paragraphs form expected by shapegrid.ResolveTextInput.
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
