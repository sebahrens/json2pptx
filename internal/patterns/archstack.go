package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// arch-stack pattern — labeled tiers with optional cross-cutting side rails
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&archStack{})
}

type archStack struct{}

func (a *archStack) Name() string        { return "arch-stack" }
func (a *archStack) Description() string { return "Architecture stack diagram with tiers and optional side rails" }
func (a *archStack) UseWhen() string     { return "Architecture layers, technology stack, layered model" }
func (a *archStack) Version() int        { return 1 }
func (a *archStack) CellsHint() string   { return "3-6 tiers + rails" }
func (a *archStack) SupportsCallout() bool        { return true }
func (a *archStack) SupportsInlineMarkdown() bool { return true }

func (a *archStack) ExemplarValues() any {
	return &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Presentation", Description: "React, Next.js"},
			{Label: "API Gateway", Description: "Kong, rate limiting"},
			{Label: "Business Logic", Description: "Go services"},
			{Label: "Data Layer", Description: "PostgreSQL, Redis"},
		},
		SideRails: []string{"Security", "Monitoring"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ArchStackTier represents one horizontal layer in the architecture stack.
type ArchStackTier struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// ArchStackValues holds tiers (top to bottom) and optional side rails.
type ArchStackValues struct {
	Tiers     []ArchStackTier `json:"tiers"`
	SideRails []string        `json:"side_rails,omitempty"` // Cross-cutting concerns shown as vertical bars
}

// ArchStackOverrides is the standard text overrides.
type ArchStackOverrides = TextOverrides

// ArchStackCellOverride is the shared per-cell override.
type ArchStackCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (a *archStack) NewValues() any      { return &ArchStackValues{} }
func (a *archStack) NewOverrides() any   { return &ArchStackOverrides{} }
func (a *archStack) NewCellOverride() any { return &ArchStackCellOverride{} }

func (a *archStack) Schema() *Schema {
	tierSchema := ObjectSchema(
		map[string]*Schema{
			"label":       StringSchema(60).WithDescription("Tier/layer name"),
			"description": StringSchema(120).WithDescription("Technologies or details for this tier"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"tiers":      ArraySchema(tierSchema, 3, 6).WithDescription("Architecture tiers, top to bottom"),
			"side_rails": ArraySchema(StringSchema(30), 0, 3).WithDescription("Cross-cutting concerns shown as vertical side bars (0-3)"),
		},
		[]string{"tiers"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Architecture stack diagram with tiers and optional side rails")
}

func (a *archStack) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ArchStackValues)
	if !ok || vals == nil {
		return fmt.Errorf("arch-stack: values must be *ArchStackValues, got %T", values)
	}

	const name = "arch-stack"
	var errs []error

	if len(vals.Tiers) < 3 {
		errs = append(errs, errMinItems(name, "tiers", 3, len(vals.Tiers), ""))
	}
	if len(vals.Tiers) > 6 {
		errs = append(errs, errMaxItems(name, "tiers", 6, len(vals.Tiers), ""))
	}

	for i, tier := range vals.Tiers {
		path := fmt.Sprintf("tiers[%d].label", i)
		if tier.Label == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(tier.Label) > 60 {
			errs = append(errs, errMaxLength(name, path, 60, len(tier.Label)))
		}
		if tier.Description != "" && len(tier.Description) > 120 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("tiers[%d].description", i), 120, len(tier.Description)))
		}
	}

	if len(vals.SideRails) > 3 {
		errs = append(errs, errMaxItems(name, "side_rails", 3, len(vals.SideRails), ""))
	}
	for i, rail := range vals.SideRails {
		if rail == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("side_rails[%d]", i)))
		} else if len(rail) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("side_rails[%d]", i), 30, len(rail)))
		}
	}

	// Total cells: tiers + side rails
	totalCells := len(vals.Tiers) + len(vals.SideRails)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (a *archStack) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ArchStackValues)
	if !ok {
		return nil, fmt.Errorf("arch-stack: values must be *ArchStackValues, got %T", values)
	}
	ovr := &ArchStackOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ArchStackOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("arch-stack: overrides must be *ArchStackOverrides, got %T", overrides)
		}
	}

	accent := ResolveAccent(ovr.Accent, ovr.SemanticAccent, ctx.Metadata)
	headerSize := ResolveSize(ovr.HeaderSize, 14.0)
	bodySize := ResolveSize(ovr.BodySize, 11.0)

	hasSideRails := len(vals.SideRails) > 0
	numRails := len(vals.SideRails)

	// Grid layout: tier column + side rail columns
	// If side rails: [80%, rail1%, rail2%, ...]
	numCols := 1 + numRails
	cols := make([]float64, numCols)
	if hasSideRails {
		railWidth := 12.0
		cols[0] = 100 - float64(numRails)*railWidth
		for i := 1; i <= numRails; i++ {
			cols[i] = railWidth
		}
	} else {
		cols[0] = 100
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0
	var rows []jsonschema.GridRowInput

	// Tier rows
	for i, tier := range vals.Tiers {
		cells := make([]*jsonschema.GridCellInput, numCols)

		// Build tier text: label + optional description
		var text json.RawMessage
		if tier.Description != "" {
			text = buildArchStackTierContent(
				pptx.ConvertMarkdownEmphasis(tier.Label), headerSize,
				pptx.ConvertMarkdownEmphasis(tier.Description), bodySize,
			)
		} else {
			text = buildArchStackSimpleContent(pptx.ConvertMarkdownEmphasis(tier.Label), headerSize)
		}

		// Vary fill slightly per tier for visual distinction
		alpha := 100 - i*10
		if alpha < 40 {
			alpha = 40
		}
		fill := json.RawMessage(fmt.Sprintf(`{"color":"%s","alpha":%d}`, accent, alpha))

		cells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     fill,
				Text:     text,
			},
		}
		applyArchStackOverride(cells[0], cellOverrides, cellIdx, accent)
		cellIdx++

		// Side rail cells: span all rows for visual continuity (we fill them on each row)
		for j := 0; j < numRails; j++ {
			if i == 0 {
				// First row: render the side rail label
				railText := buildArchStackSimpleContent(vals.SideRails[j], 10.0)
				cells[j+1] = &jsonschema.GridCellInput{
					RowSpan: len(vals.Tiers),
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"lt2"`),
						Text:     railText,
					},
				}
				applyArchStackOverride(cells[j+1], cellOverrides, len(vals.Tiers)+j, accent)
			} else {
				// Subsequent rows: nil cell (covered by rowspan)
				cells[j+1] = nil
			}
		}

		rows = append(rows, jsonschema.GridRowInput{Cells: cells})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     4,
		RowGap:  4,
		Rows:    rows,
	}

	return grid, nil
}

func buildArchStackTierContent(label string, labelSize float64, desc string, descSize float64) json.RawMessage {
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
			{Content: label, Size: labelSize, Bold: true, Color: "lt1", Align: "ctr"},
			{Content: desc, Size: descSize, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func buildArchStackSimpleContent(content string, size float64) json.RawMessage {
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
			{Content: content, Size: size, Bold: true, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func applyArchStackOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	if cell == nil {
		return
	}
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*ArchStackCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
