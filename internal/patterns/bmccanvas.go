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
	return "Standard Osterwalder 9-cell Business Model Canvas only; prefer card-grid for arbitrary card layouts, matrix-2x2 for 4-quadrant positioning"
}
func (b *bmcCanvas) NotWhen() string {
	return "Arbitrary card layouts not following Osterwalder's 9 blocks (use card-grid), or a 4-quadrant positioning exercise (use matrix-2x2)"
}
func (b *bmcCanvas) Version() int      { return 1 }
func (b *bmcCanvas) CellsHint() string { return "9" }
func (b *bmcCanvas) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"kpi-3up", "card-grid", "pull-quote"},
		DensityClass:  "high",
		AccentWeight:  "subtle",
	}
}

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
		RevenueStreams:    BMCCell{Header: "Revenue Streams", Bullets: []string{"Asset sale", "Subscription"}},
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
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	BulletSize     float64 `json:"bullet_size,omitempty"`
}

// BMCCanvasCellOverride is an alias for the shared CellOverride struct.
type BMCCanvasCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (b *bmcCanvas) NewValues() any       { return &BMCCanvasValues{} }
func (b *bmcCanvas) NewOverrides() any    { return &BMCCanvasOverrides{} }
func (b *bmcCanvas) NewCellOverride() any { return &BMCCanvasCellOverride{} }

// Calibrated with TestPatternBudgetProbe across all bundled templates. Each
// entry is the approximate characters per bullet when every bullet in that
// cell has the same length and the other eight cells contain short copy.
// Index zero is unused because a BMC cell requires at least one bullet.
var bmcBulletBudgets = [4][11]int{
	{0, 200, 200, 127, 102, 77, 52, 52, 52, 26, 26}, // tall narrow
	{0, 177, 77, 52, 26, 26, 26, 26, 0, 0, 0},       // short narrow
	{0, 200, 200, 183, 92, 92, 92, 92, 0, 0, 0},     // cost structure
	{0, 200, 181, 121, 58, 58, 58, 58, 0, 0, 0},     // revenue streams
}

type bmcNamedCell struct {
	name  string
	cell  BMCCell
	group int
}

func bmcNamedCells(v *BMCCanvasValues) []bmcNamedCell {
	return []bmcNamedCell{
		{"key_partners", v.KeyPartners, 0},
		{"key_activities", v.KeyActivities, 1},
		{"key_resources", v.KeyResources, 1},
		{"value_propositions", v.ValuePropositions, 0},
		{"customer_relations", v.CustomerRelations, 1},
		{"channels", v.Channels, 1},
		{"customer_segments", v.CustomerSegments, 0},
		{"cost_structure", v.CostStructure, 2},
		{"revenue_streams", v.RevenueStreams, 3},
	}
}

// PostExpandWarnings gives authors a cell-specific copy target. The schema
// retains the 200-character item maximum because spacious cells can use it.
func (b *bmcCanvas) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*BMCCanvasValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	for _, named := range bmcNamedCells(v) {
		count := len(named.cell.Bullets)
		if count < 1 || count > 10 { // validation reports invalid counts
			continue
		}
		budget := bmcBulletBudgets[named.group][count]
		if budget == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"%s: bmc-canvas %s.bullets has %d items; this cell holds at most 7 readable bullets — reduce the count",
				ErrCodeBodyTooLong, named.name, count))
			continue
		}
		for i, bullet := range named.cell.Bullets {
			if n := runeLen(bullet); n > budget {
				warnings = append(warnings, fmt.Sprintf(
					"%s: bmc-canvas %s.bullets[%d] is %d characters; with %d bullets this cell holds about %d characters per bullet before text shrinks below the readable minimum — shorten the bullet or reduce the count",
					ErrCodeBodyTooLong, named.name, i, n, count, budget))
			}
		}
	}
	return warnings
}

func (b *bmcCanvas) Schema() *Schema {
	cellSchema := func(description, budget string) *Schema {
		return ObjectSchema(
			map[string]*Schema{
				"header":  StringSchema(60).WithDescription("Cell header (e.g. \"Key Partners\")"),
				"bullets": ArraySchema(StringSchema(200), 1, 10).WithDescription("Bullet points; approximate readable characters per bullet by count: " + budget),
			},
			[]string{"header", "bullets"},
		).WithAdditionalProperties(false).WithDescription(description)
	}
	const tall = "1-2: 200; 3: 127; 4: 102; 5: 77; 6-8: 52; 9-10: 26"
	const short = "1: 177; 2: 77; 3: 52; 4-7: 26; use at most 7 bullets"
	const cost = "1-2: 200; 3: 183; 4-7: 92; use at most 7 bullets"
	const revenue = "1: 200; 2: 181; 3: 121; 4-7: 58; use at most 7 bullets"

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"key_partners":       RefSchema("tallCell").WithDescription("Key Partners — who are our key partners and suppliers?"),
			"key_activities":     RefSchema("shortCell").WithDescription("Key Activities — what key activities does our value proposition require?"),
			"key_resources":      RefSchema("shortCell").WithDescription("Key Resources — what key resources does our value proposition require?"),
			"value_propositions": RefSchema("tallCell").WithDescription("Value Propositions — what value do we deliver to the customer?"),
			"customer_relations": RefSchema("shortCell").WithDescription("Customer Relationships — what type of relationship does each segment expect?"),
			"channels":           RefSchema("shortCell").WithDescription("Channels — through which channels do our segments want to be reached?"),
			"customer_segments":  RefSchema("tallCell").WithDescription("Customer Segments — for whom are we creating value?"),
			"cost_structure":     RefSchema("costCell").WithDescription("Cost Structure — what are the most important costs inherent in our model?"),
			"revenue_streams":    RefSchema("revenueCell").WithDescription("Revenue Streams — for what value are customers willing to pay?"),
		},
		[]string{"key_partners", "key_activities", "key_resources", "value_propositions",
			"customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values": valuesSchema,
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"header_size":     NumberSchema(6, 120).WithDescription("Font size for cell headers in points"),
					"bullet_size":     NumberSchema(6, 120).WithDescription("Font size for bullet text in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
		"tallCell":     cellSchema("Tall narrow BMC cell", tall),
		"shortCell":    cellSchema("Short narrow BMC cell", short),
		"costCell":     cellSchema("Wide cost-structure cell", cost),
		"revenueCell":  cellSchema("Wide revenue-streams cell", revenue),
	}).WithDescription("Formal 9-cell Business Model Canvas (Osterwalder)")
}

func (b *bmcCanvas) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*BMCCanvasValues)
	if !ok || vals == nil {
		return fmt.Errorf("bmc-canvas: values must be *BMCCanvasValues, got %T", values)
	}

	const name = "bmc-canvas"
	var errs []error

	// Validate each of the 9 cells
	cells := []struct {
		cellName string
		cell     BMCCell
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
		headerPath := c.cellName + ".header"
		if c.cell.Header == "" {
			errs = append(errs, errRequired(name, headerPath))
		} else if runeLen(c.cell.Header) > 60 {
			errs = append(errs, errMaxLength(name, headerPath, 60, runeLen(c.cell.Header)))
		}

		bulletsPath := c.cellName + ".bullets"
		if len(c.cell.Bullets) == 0 {
			errs = append(errs, newValidationError(name, bulletsPath, ErrCodeMinItems,
				fmt.Sprintf("bmc-canvas: %s.bullets must have at least 1 item (hint: use card-grid for cells without bullet lists)", c.cellName),
				AddItemsFix(bulletsPath, 1)))
		} else if len(c.cell.Bullets) > 10 {
			errs = append(errs, newValidationError(name, bulletsPath, ErrCodeMaxItems,
				fmt.Sprintf("bmc-canvas: %s.bullets exceeds maximum 10 items (%d items)", c.cellName, len(c.cell.Bullets)),
				ReduceItemsFix(bulletsPath, 10)))
		}
		for i, bullet := range c.cell.Bullets {
			bulletPath := fmt.Sprintf("%s.bullets[%d]", c.cellName, i)
			if bullet == "" {
				errs = append(errs, errEmptyValue(name, bulletPath))
			} else if runeLen(bullet) > 200 {
				errs = append(errs, errMaxLength(name, bulletPath, 200, runeLen(bullet)))
			}
		}
	}

	// Validate cell_overrides: indices 0-8 only
	const totalCells = 9
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, "(hint: "+bmcCellIndexHint()+")"); coErr != nil {
		errs = append(errs, coErr)
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

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 11.0)
	bulletSize := ResolveSize(ovr.BulletSize, 9.0)

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
		vals.RevenueStreams,    // 8
	}

	// Default cell border: subtle dk1-tinted stroke so the 9-cell canvas
	// reads as a structured grid (canonical Osterwalder look) rather than
	// floating panels. Authors can override via cell_overrides if desired.
	const bmcCellBorderJSON = `{"color":"dk1","width":0.75,"lumMod":25000,"lumOff":75000}`

	makeCell := func(idx int, cell BMCCell, colSpan, rowSpan int) *jsonschema.GridCellInput {
		gc := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Line:     json.RawMessage(bmcCellBorderJSON),
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
	applyCellTextOverride(cell, cellOvr)
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
