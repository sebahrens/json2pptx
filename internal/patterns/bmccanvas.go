package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// bmc-canvas pattern — formal 9-cell Business Model Canvas (D5)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&bmcCanvas{})
}

type bmcCanvas struct{}

func (b *bmcCanvas) Name() string { return "bmc-canvas" }
func (b *bmcCanvas) Description() string {
	return "Formal 9-cell Business Model Canvas (Osterwalder)"
}
func (b *bmcCanvas) UseWhen() string {
	return "Osterwalder BMC only; prefer card-grid for general cards"
}
func (b *bmcCanvas) Version() int      { return 1 }
func (b *bmcCanvas) CellsHint() string { return "9" }

func (b *bmcCanvas) ExemplarValues() any {
	return &BMCCanvasValues{
		KeyPartners:       BMCCell{Header: "Key Partners", Bullets: []string{"Supplier A", "Partner B"}},
		KeyActivities:     BMCCell{Header: "Key Activities", Bullets: []string{"Manufacturing", "Problem solving"}},
		KeyResources:      BMCCell{Header: "Key Resources", Bullets: []string{"Physical assets", "IP"}},
		ValuePropositions: BMCCell{Header: "Value Propositions", Bullets: []string{"Newness", "Performance", "Customization"}},
		CustomerRelations: BMCCell{Header: "Customer Relationships", Bullets: []string{"Personal assistance", "Self-service"}},
		Channels:          BMCCell{Header: "Channels", Bullets: []string{"Direct sales", "Web"}},
		CustomerSegments:  BMCCell{Header: "Customer Segments", Bullets: []string{"Mass market", "Niche market"}},
		CostStructure:     BMCCell{Header: "Cost Structure", Bullets: []string{"Fixed costs", "Variable costs"}},
		RevenueStreams:     BMCCell{Header: "Revenue Streams", Bullets: []string{"Asset sale", "Subscription"}},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// BMCCell is a single Business Model Canvas cell with a header and bullet list.
type BMCCell struct {
	Header  string   `json:"header"`
	Bullets []string `json:"bullets"`
}

// BMCCanvasValues is the values type for bmc-canvas.
type BMCCanvasValues struct {
	KeyPartners       BMCCell `json:"key_partners"`
	KeyActivities     BMCCell `json:"key_activities"`
	KeyResources      BMCCell `json:"key_resources"`
	ValuePropositions BMCCell `json:"value_propositions"`
	CustomerRelations BMCCell `json:"customer_relations"`
	Channels          BMCCell `json:"channels"`
	CustomerSegments  BMCCell `json:"customer_segments"`
	CostStructure     BMCCell `json:"cost_structure"`
	RevenueStreams    BMCCell `json:"revenue_streams"`
}

// BMCCanvasOverrides contains pattern-level overrides for bmc-canvas.
type BMCCanvasOverrides struct {
	Accent     string  `json:"accent,omitempty"`
	HeaderSize float64 `json:"header_size,omitempty"`
	BulletSize float64 `json:"bullet_size,omitempty"`
}

// BMCCanvasCellOverride contains per-cell overrides for bmc-canvas.
type BMCCanvasCellOverride struct {
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

func (b *bmcCanvas) NewValues() any      { return &BMCCanvasValues{} }
func (b *bmcCanvas) NewOverrides() any   { return &BMCCanvasOverrides{} }
func (b *bmcCanvas) NewCellOverride() any { return &BMCCanvasCellOverride{} }

// bmcCanvasCellOverrideAllowed is the whitelist of per-cell override keys (D15).
var bmcCanvasCellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}


func (b *bmcCanvas) Schema() *Schema {
	cellSchema := ObjectSchema(
		map[string]*Schema{
			"header":  StringSchema(60).WithDescription("Cell header (e.g. \"Key Partners\")"),
			"bullets": ArraySchema(StringSchema(200), 1, 10).WithDescription("Bullet points for this BMC section"),
		},
		[]string{"header", "bullets"},
	).WithAdditionalProperties(false)


	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"key_partners":       cellSchema.WithDescription("Key Partners — who are our key partners and suppliers?"),
			"key_activities":     cellSchema.WithDescription("Key Activities — what key activities does our value proposition require?"),
			"key_resources":      cellSchema.WithDescription("Key Resources — what key resources does our value proposition require?"),
			"value_propositions": cellSchema.WithDescription("Value Propositions — what value do we deliver to the customer?"),
			"customer_relations": cellSchema.WithDescription("Customer Relationships — what type of relationship does each segment expect?"),
			"channels":           cellSchema.WithDescription("Channels — through which channels do our segments want to be reached?"),
			"customer_segments":  cellSchema.WithDescription("Customer Segments — for whom are we creating value?"),
			"cost_structure":     cellSchema.WithDescription("Cost Structure — what are the most important costs inherent in our model?"),
			"revenue_streams":    cellSchema.WithDescription("Revenue Streams — for what value are customers willing to pay?"),
		},
		[]string{"key_partners", "key_activities", "key_resources", "value_propositions",
			"customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values": valuesSchema,
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":      StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"header_size": NumberSchema(6, 120).WithDescription("Font size for cell headers in points"),
					"bullet_size": NumberSchema(6, 120).WithDescription("Font size for bullet text in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Formal 9-cell Business Model Canvas (Osterwalder)")
}

func (b *bmcCanvas) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*BMCCanvasValues)
	if !ok || vals == nil {
		return fmt.Errorf("bmc-canvas: values must be *BMCCanvasValues, got %T", values)
	}

	var errs []error

	// Validate each of the 9 cells
	cells := []struct {
		name string
		cell BMCCell
	}{
		{"key_partners", vals.KeyPartners},
		{"key_activities", vals.KeyActivities},
		{"key_resources", vals.KeyResources},
		{"value_propositions", vals.ValuePropositions},
		{"customer_relations", vals.CustomerRelations},
		{"channels", vals.Channels},
		{"customer_segments", vals.CustomerSegments},
		{"cost_structure", vals.CostStructure},
		{"revenue_streams", vals.RevenueStreams},
	}

	for _, c := range cells {
		if c.cell.Header == "" {
			errs = append(errs, fmt.Errorf("bmc-canvas: %s.header is required", c.name))
		} else if len(c.cell.Header) > 60 {
			errs = append(errs, fmt.Errorf("bmc-canvas: %s.header exceeds maxLength 60 (%d chars)", c.name, len(c.cell.Header)))
		}

		if len(c.cell.Bullets) == 0 {
			errs = append(errs, fmt.Errorf("bmc-canvas: %s.bullets must have at least 1 item (hint: use card-grid for cells without bullet lists)", c.name))
		} else if len(c.cell.Bullets) > 10 {
			errs = append(errs, fmt.Errorf("bmc-canvas: %s.bullets exceeds maximum 10 items (%d items)", c.name, len(c.cell.Bullets)))
		}
		for i, bullet := range c.cell.Bullets {
			if bullet == "" {
				errs = append(errs, fmt.Errorf("bmc-canvas: %s.bullets[%d] must not be empty", c.name, i))
			} else if len(bullet) > 200 {
				errs = append(errs, fmt.Errorf("bmc-canvas: %s.bullets[%d] exceeds maxLength 200 (%d chars)", c.name, i, len(bullet)))
			}
		}
	}

	// Validate cell_overrides: indices 0-8 only
	const totalCells = 9
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= totalCells {
			errs = append(errs, fmt.Errorf("bmc-canvas: cell_overrides key %d out of range [0,%d] (hint: %s)",
				idx, totalCells-1, bmcCellIndexHint()))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("bmc-canvas: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("bmc-canvas: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !bmcCanvasCellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("bmc-canvas: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

func (b *bmcCanvas) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*BMCCanvasValues)
	if !ok {
		return nil, fmt.Errorf("bmc-canvas: values must be *BMCCanvasValues, got %T", values)
	}
	ovr := &BMCCanvasOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*BMCCanvasOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("bmc-canvas: overrides must be *BMCCanvasOverrides, got %T", overrides)
		}
	}

	accent := "accent1"
	if ovr.Accent != "" {
		accent = ovr.Accent
	}
	headerSize := 11.0
	if ovr.HeaderSize > 0 {
		headerSize = ovr.HeaderSize
	}
	bulletSize := 9.0
	if ovr.BulletSize > 0 {
		bulletSize = ovr.BulletSize
	}

	// BMC canonical layout (5 columns, 3 rows):
	//
	// Row 0: | Key Partners (rs=2) | Key Activities  | Value Props (rs=2) | Cust Relations  | Cust Segments (rs=2) |
	// Row 1: |                     | Key Resources   |                    | Channels         |                      |
	// Row 2: | Cost Structure (cs=3)                                     | Revenue Streams (cs=2)                  |
	//
	// Cell indices: 0=key_partners, 1=key_activities, 2=key_resources,
	//   3=value_propositions, 4=customer_relations, 5=channels,
	//   6=customer_segments, 7=cost_structure, 8=revenue_streams

	bmcCells := []BMCCell{
		vals.KeyPartners,       // 0
		vals.KeyActivities,     // 1
		vals.KeyResources,      // 2
		vals.ValuePropositions, // 3
		vals.CustomerRelations, // 4
		vals.Channels,          // 5
		vals.CustomerSegments,  // 6
		vals.CostStructure,     // 7
		vals.RevenueStreams,     // 8
	}

	makeCell := func(idx int, cell BMCCell, colSpan, rowSpan int) *jsonschema.GridCellInput {
		gc := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Text:     buildBMCCellContent(cell, headerSize, bulletSize, accent),
			},
		}
		if colSpan > 1 {
			gc.ColSpan = colSpan
		}
		if rowSpan > 1 {
			gc.RowSpan = rowSpan
		}
		applyBMCCellOverride(gc, cellOverrides, idx, accent)
		return gc
	}

	_ = bmcCells // referenced via makeCell

	rows := []jsonschema.GridRowInput{
		// Row 0: Key Partners (rs=2) | Key Activities | Value Props (rs=2) | Cust Relations | Cust Segments (rs=2)
		{
			Cells: []*jsonschema.GridCellInput{
				makeCell(0, bmcCells[0], 1, 2), // Key Partners
				makeCell(1, bmcCells[1], 1, 1), // Key Activities
				makeCell(3, bmcCells[3], 1, 2), // Value Propositions
				makeCell(4, bmcCells[4], 1, 1), // Customer Relations
				makeCell(6, bmcCells[6], 1, 2), // Customer Segments
			},
		},
		// Row 1: (Key Partners spans) | Key Resources | (Value Props spans) | Channels | (Cust Segments spans)
		{
			Cells: []*jsonschema.GridCellInput{
				makeCell(2, bmcCells[2], 1, 1), // Key Resources
				makeCell(5, bmcCells[5], 1, 1), // Channels
			},
		},
		// Row 2: Cost Structure (cs=3) | Revenue Streams (cs=2)
		{
			Cells: []*jsonschema.GridCellInput{
				makeCell(7, bmcCells[7], 3, 1), // Cost Structure
				makeCell(8, bmcCells[8], 2, 1), // Revenue Streams
			},
		},
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`5`),
		Gap:     4,
		Rows:    rows,
	}

	return grid, nil
}

// buildBMCCellContent creates a JSON text object with a bold header and bullet list.
func buildBMCCellContent(cell BMCCell, headerSize, bulletSize float64, accent string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	paras := []paragraph{
		{Content: cell.Header, Size: headerSize, Bold: true, Color: accent, Align: "l"},
	}
	for _, bullet := range cell.Bullets {
		paras = append(paras, paragraph{Content: "• " + bullet, Size: bulletSize, Color: "dk1", Align: "l"})
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

// applyBMCCellOverride applies cell_overrides for a given BMC cell index.
func applyBMCCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*BMCCanvasCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    4,
		}
	}
}

// bmcCellIndexHint returns a hint string mapping cell indices to names.
func bmcCellIndexHint() string {
	return "0=key_partners, 1=key_activities, 2=key_resources, 3=value_propositions, 4=customer_relations, 5=channels, 6=customer_segments, 7=cost_structure, 8=revenue_streams"
}
