package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// before-after-compact — height-capped variant of before-after
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&beforeAfterCompact{})
}

type beforeAfterCompact struct{}

func (b *beforeAfterCompact) Name() string        { return "before-after-compact" }
func (b *beforeAfterCompact) Description() string { return "Compact two-column before/after with transition chevron, height-capped" }
func (b *beforeAfterCompact) UseWhen() string {
	return "Brief before→after with short bullet lists (1-4 items each) where the transformation is context, not the slide hero; prefer full before-after when items need more vertical space"
}
func (b *beforeAfterCompact) NotWhen() string {
	return "Each column has 5+ items needing full height (use before-after), comparing options without temporal change (use comparison-2col)"
}
func (b *beforeAfterCompact) Version() int     { return 1 }
func (b *beforeAfterCompact) CellsHint() string { return "2 + header" }
func (b *beforeAfterCompact) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"compare"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		DensityClass:  "low",
		AccentWeight:  "normal",
	}
}
func (b *beforeAfterCompact) SupportsCallout() bool        { return true }
func (b *beforeAfterCompact) SupportsInlineMarkdown() bool { return true }

func (b *beforeAfterCompact) ExemplarValues() any {
	return &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Current", Items: []string{"Manual", "Slow"}},
		After:  BeforeAfterColumn{Header: "Future", Items: []string{"Automated", "Fast"}},
	}
}

// Reuse types from before-after.
func (b *beforeAfterCompact) NewValues() any       { return &BeforeAfterValues{} }
func (b *beforeAfterCompact) NewOverrides() any    { return &BeforeAfterOverrides{} }
func (b *beforeAfterCompact) NewCellOverride() any { return &BeforeAfterCellOverride{} }

func (b *beforeAfterCompact) Schema() *Schema {
	columnSchema := ObjectSchema(
		map[string]*Schema{
			"header": StringSchema(60).WithDescription("Column header"),
			"items":  ArraySchema(StringSchema(200), 1, 8).WithDescription("Bullet items (1-8)"),
		},
		[]string{"header", "items"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"before": columnSchema.WithDescription("Left column (current/before state)"),
			"after":  columnSchema.WithDescription("Right column (future/after state)"),
		},
		[]string{"before", "after"},
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
	}).WithDescription("Compact two-column before/after with transition chevron, height-capped at ~60% of content area")
}

func (b *beforeAfterCompact) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*BeforeAfterValues)
	if !ok || vals == nil {
		return fmt.Errorf("before-after-compact: values must be *BeforeAfterValues, got %T", values)
	}

	const name = "before-after-compact"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*BeforeAfterOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Validate before column
	if vals.Before.Header == "" {
		errs = append(errs, errRequired(name, "before.header"))
	} else if len(vals.Before.Header) > 60 {
		errs = append(errs, errMaxLength(name, "before.header", 60, len(vals.Before.Header)))
	}
	if len(vals.Before.Items) == 0 {
		errs = append(errs, errMinItems(name, "before.items", 1, 0, ""))
	}
	if len(vals.Before.Items) > 8 {
		errs = append(errs, errMaxItems(name, "before.items", 8, len(vals.Before.Items), ""))
	}
	for i, item := range vals.Before.Items {
		path := fmt.Sprintf("before.items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(item) > 200 {
			errs = append(errs, errMaxLength(name, path, 200, len(item)))
		}
	}

	// Validate after column
	if vals.After.Header == "" {
		errs = append(errs, errRequired(name, "after.header"))
	} else if len(vals.After.Header) > 60 {
		errs = append(errs, errMaxLength(name, "after.header", 60, len(vals.After.Header)))
	}
	if len(vals.After.Items) == 0 {
		errs = append(errs, errMinItems(name, "after.items", 1, 0, ""))
	}
	if len(vals.After.Items) > 8 {
		errs = append(errs, errMaxItems(name, "after.items", 8, len(vals.After.Items), ""))
	}
	for i, item := range vals.After.Items {
		path := fmt.Sprintf("after.items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(item) > 200 {
			errs = append(errs, errMaxLength(name, path, 200, len(item)))
		}
	}

	totalCells := 5
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (b *beforeAfterCompact) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*BeforeAfterValues)
	if !ok {
		return nil, fmt.Errorf("before-after-compact: values must be *BeforeAfterValues, got %T", values)
	}
	ovr := &BeforeAfterOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*BeforeAfterOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("before-after-compact: overrides must be *BeforeAfterOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 14.0)
	bodySize := ResolveSize(ovr.BodySize, 11.0)
	cellAccentMode := ovr.CellAccentMode

	beforeAccent := ResolveCellAccent(baseAccent, 0, cellAccentMode)
	afterAccent := ResolveCellAccent(baseAccent, 1, cellAccentMode)

	cellIdx := 0

	// Header row: Before header | chevron | After header
	beforeHeader := buildBeforeAfterTextContent(vals.Before.Header, headerSize, true, "lt1", "ctr")
	afterHeader := buildBeforeAfterTextContent(vals.After.Header, headerSize, true, "lt1", "ctr")
	chevronText := buildBeforeAfterTextContent("→", 20.0, true, "lt1", "ctr")

	beforeHeaderCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, beforeAccent)),
			Text:     beforeHeader,
		},
	}
	applyBeforeAfterCellOverride(beforeHeaderCell, cellOverrides, cellIdx, beforeAccent)
	cellIdx++

	chevronCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "chevron",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent)),
			Text:     chevronText,
		},
	}
	applyBeforeAfterCellOverride(chevronCell, cellOverrides, cellIdx, baseAccent)
	cellIdx++

	afterHeaderCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, afterAccent)),
			Text:     afterHeader,
		},
	}
	applyBeforeAfterCellOverride(afterHeaderCell, cellOverrides, cellIdx, afterAccent)
	cellIdx++

	// Body row: before items | spacer | after items
	beforeBody := buildBeforeAfterBulletContent(vals.Before.Items, bodySize)
	afterBody := buildBeforeAfterBulletContent(vals.After.Items, bodySize)

	beforeBodyCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     beforeBody,
		},
	}
	applyBeforeAfterCellOverride(beforeBodyCell, cellOverrides, cellIdx, beforeAccent)
	cellIdx++

	afterBodyCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     afterBody,
		},
	}
	applyBeforeAfterCellOverride(afterBodyCell, cellOverrides, cellIdx, afterAccent)

	colsJSON := json.RawMessage(`[45, 10, 45]`)

	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: 60,
		},
		Columns: colsJSON,
		Gap:     6,
		Rows: []jsonschema.GridRowInput{
			{
				Height: 30,
				Cells:  []*jsonschema.GridCellInput{beforeHeaderCell, chevronCell, afterHeaderCell},
			},
			{
				Cells: []*jsonschema.GridCellInput{
					beforeBodyCell,
					{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}},
					afterBodyCell,
				},
			},
		},
	}

	return grid, nil
}
