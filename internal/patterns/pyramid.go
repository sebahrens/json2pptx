package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// pyramid pattern — 3-5 stacked trapezoids representing a hierarchy
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&pyramid{})
}

type pyramid struct{}

func (p *pyramid) Name() string        { return "pyramid" }
func (p *pyramid) Description() string { return "Stacked trapezoid hierarchy (3-5 tiers)" }
func (p *pyramid) UseWhen() string     { return "Hierarchy, layered model, Maslow-style pyramid" }
func (p *pyramid) Version() int        { return 1 }
func (p *pyramid) CellsHint() string { return "3-5" }
func (p *pyramid) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"card-grid", "kpi-3up", "icon-row"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (p *pyramid) SupportsCallout() bool        { return true }
func (p *pyramid) SupportsInlineMarkdown() bool { return true }

func (p *pyramid) ExemplarValues() any {
	return &PyramidValues{
		Tiers: []string{"Strategy", "Tactics", "Operations", "Execution", "Measurement"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PyramidValues holds the tier labels for the pyramid pattern.
type PyramidValues struct {
	Tiers []string `json:"tiers"` // Top to bottom (narrowest to widest)
}

// PyramidOverrides is the standard text overrides.
type PyramidOverrides = TextOverrides

// PyramidCellOverride is the shared per-cell override.
type PyramidCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *pyramid) NewValues() any      { return &PyramidValues{} }
func (p *pyramid) NewOverrides() any   { return &PyramidOverrides{} }
func (p *pyramid) NewCellOverride() any { return &PyramidCellOverride{} }

func (p *pyramid) Schema() *Schema {
	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"tiers": ArraySchema(StringSchema(120), 3, 5).WithDescription("Tier labels, top (narrowest) to bottom (widest)"),
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
	}).WithDescription("Stacked trapezoid hierarchy (3-5 tiers)")
}

func (p *pyramid) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*PyramidValues)
	if !ok || vals == nil {
		return fmt.Errorf("pyramid: values must be *PyramidValues, got %T", values)
	}

	const name = "pyramid"
	var errs []error

	if len(vals.Tiers) < 3 {
		errs = append(errs, errMinItems(name, "tiers", 3, len(vals.Tiers), ""))
	}
	if len(vals.Tiers) > 5 {
		errs = append(errs, errMaxItems(name, "tiers", 5, len(vals.Tiers), ""))
	}

	for i, tier := range vals.Tiers {
		path := fmt.Sprintf("tiers[%d]", i)
		if tier == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(tier) > 120 {
			errs = append(errs, errMaxLength(name, path, 120, len(tier)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Tiers), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (p *pyramid) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*PyramidValues)
	if !ok {
		return nil, fmt.Errorf("pyramid: values must be *PyramidValues, got %T", values)
	}
	ovr := &PyramidOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*PyramidOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("pyramid: overrides must be *PyramidOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	bodySize := ResolveSize(ovr.BodySize, 14.0)
	n := len(vals.Tiers)

	// Each tier is a row with a single cell. We use column widths to
	// simulate narrowing: row i has left padding + content + right padding.
	// The "trapezoid" effect is achieved via the geometry (use "trapezoid"
	// OOXML auto-shape) with centered text.
	var rows []jsonschema.GridRowInput
	for i, tier := range vals.Tiers {
		text := buildPyramidTextContent(tier, bodySize)

		// Gradient the fill: top tier uses accent, lower tiers lighten.
		// We use accent for all tiers but vary alpha for visual weight.
		fill := json.RawMessage(fmt.Sprintf(`{"color":"%s","alpha":%d}`, accent, 100-i*15))

		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "trapezoid",
				Fill:     fill,
				Text:     text,
			},
		}

		// Apply cell overrides
		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*PyramidCellOverride); ok2 && cellOvr.AccentBar {
				cell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
		}

		// Use column proportions to create narrowing effect: pad on sides.
		// Top tier: narrow center. Bottom tier: full width.
		padPct := float64(n-1-i) * 10 // 0% at bottom, up to 40% each side at top
		cols := []float64{padPct, 100 - 2*padPct, padPct}
		colsJSON, _ := json.Marshal(cols)

		row := jsonschema.GridRowInput{
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}},
				cell,
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}},
			},
		}
		_ = colsJSON // columns set per-row is not supported; use single grid columns

		rows = append(rows, row)
	}

	// The shape grid doesn't support per-row column widths, so we use a
	// single 3-column grid with all rows. The trapezoid geometry itself
	// creates the narrowing visual effect.
	colsJSON := json.RawMessage(`3`)
	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		Gap:     4,
		Rows:    rows,
	}

	return grid, nil
}

func buildPyramidTextContent(content string, size float64) json.RawMessage {
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
			{Content: content, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
