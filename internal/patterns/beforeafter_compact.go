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

func (b *beforeAfterCompact) Name() string { return "before-after-compact" }
func (b *beforeAfterCompact) Description() string {
	return "Compact two-column before/after with transition chevron, height-capped"
}
func (b *beforeAfterCompact) UseWhen() string {
	return "Brief before→after with short bullet lists (1-4 items each) where the transformation is context, not the slide hero; prefer full before-after when items need more vertical space"
}
func (b *beforeAfterCompact) NotWhen() string {
	return "Each column has 5+ items needing full height (use before-after), comparing options without temporal change (use comparison-2col)"
}
func (b *beforeAfterCompact) Version() int      { return 1 }
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

// Each compact body column has a measured vertical budget. At default text
// sizes, a bullet up to 67/133/200 characters occupies about 1/2/3 wrapped
// lines. A header above 46 characters wraps and takes room from the body.
func beforeAfterCompactBulletLines(item string) int {
	switch n := runeLen(item); {
	case n <= 67:
		return 1
	case n <= 133:
		return 2
	default:
		return 3
	}
}

func beforeAfterCompactBodyLineBudget(before, after string) int {
	if runeLen(before) > 46 || runeLen(after) > 46 {
		return 10
	}
	return 12
}

func (b *beforeAfterCompact) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*BeforeAfterValues)
	if !ok || v == nil {
		return nil
	}
	budget := beforeAfterCompactBodyLineBudget(v.Before.Header, v.After.Header)
	var warnings []string
	for _, side := range []struct {
		name  string
		items []string
	}{
		{"before", v.Before.Items},
		{"after", v.After.Items},
	} {
		lines := 0
		for _, item := range side.items {
			lines += beforeAfterCompactBulletLines(item)
		}
		if lines > budget {
			warnings = append(warnings, fmt.Sprintf("%s: before-after-compact %s.items use about %d wrapped text lines across %d bullets; this compact column holds about %d lines with the chosen headers — shorten bullets above 133 or 67 characters, use fewer bullets, shorten the headers, or use before-after for more height", ErrCodeBodyTooLong, side.name, lines, len(side.items), budget))
		}
	}
	return warnings
}

func (b *beforeAfterCompact) Schema() *Schema {
	columnSchema := ObjectSchema(
		map[string]*Schema{
			"header": StringSchema(46).WithDescription("Short column header; use the full before-after variant for longer headers"),
			"items":  ArraySchema(StringSchema(133), 1, 4).WithDescription("Brief bullet items (1-4); use full before-after for longer copy or more items"),
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
	} else if runeLen(vals.Before.Header) > 46 {
		errs = append(errs, errMaxLength(name, "before.header", 46, runeLen(vals.Before.Header)))
	}
	if len(vals.Before.Items) == 0 {
		errs = append(errs, errMinItems(name, "before.items", 1, 0, ""))
	}
	if len(vals.Before.Items) > 4 {
		errs = append(errs, errMaxItems(name, "before.items", 4, len(vals.Before.Items), ""))
	}
	for i, item := range vals.Before.Items {
		path := fmt.Sprintf("before.items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(item) > 133 {
			errs = append(errs, errMaxLength(name, path, 133, runeLen(item)))
		}
	}

	// Validate after column
	if vals.After.Header == "" {
		errs = append(errs, errRequired(name, "after.header"))
	} else if runeLen(vals.After.Header) > 46 {
		errs = append(errs, errMaxLength(name, "after.header", 46, runeLen(vals.After.Header)))
	}
	if len(vals.After.Items) == 0 {
		errs = append(errs, errMinItems(name, "after.items", 1, 0, ""))
	}
	if len(vals.After.Items) > 4 {
		errs = append(errs, errMaxItems(name, "after.items", 4, len(vals.After.Items), ""))
	}
	for i, item := range vals.After.Items {
		path := fmt.Sprintf("after.items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(item) > 133 {
			errs = append(errs, errMaxLength(name, path, 133, runeLen(item)))
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

	// Header row: Before header | full-height chevron | After header.
	beforeHeader := withBeforeAfterPanelInsets(buildBeforeAfterTextContent(vals.Before.Header, headerSize, true, "lt1", "ctr"))
	afterHeader := withBeforeAfterPanelInsets(buildBeforeAfterTextContent(vals.After.Header, headerSize, true, "lt1", "ctr"))

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
		RowSpan: 2,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "chevron",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent)),
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

	// Body row: light accent-derived panels; the chevron owns the middle column.
	beforeBody := withBeforeAfterPanelInsets(buildBeforeAfterBulletContent(vals.Before.Items, bodySize))
	afterBody := withBeforeAfterPanelInsets(buildBeforeAfterBulletContent(vals.After.Items, bodySize))

	beforeBodyCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     beforeAfterPanelTone(beforeAccent).fillJSON(),
			Text:     beforeBody,
		},
	}
	applyBeforeAfterCellOverride(beforeBodyCell, cellOverrides, cellIdx, beforeAccent)
	cellIdx++

	afterBodyCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     beforeAfterPanelTone(afterAccent).fillJSON(),
			Text:     afterBody,
		},
	}
	applyBeforeAfterCellOverride(afterBodyCell, cellOverrides, cellIdx, afterAccent)

	colsJSON := json.RawMessage(`[45, 10, 45]`)

	// Compact means compact (go-slide-creator-3i7c): a header band sized to
	// the header text and a body row that hugs the bullets, the whole block
	// capped at 60% of the content area and centred there.
	headerPt, bodyPt := beforeAfterFullRowHeights(ctx, beforeHeader, afterHeader, beforeBody, afterBody, 6)
	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: 60,
		},
		Columns:       colsJSON,
		Gap:           6,
		VerticalAlign: GridVerticalAlignDefault,
		Rows: []jsonschema.GridRowInput{
			{
				MinHeight: headerPt,
				MaxHeight: headerPt,
				Cells:     []*jsonschema.GridCellInput{beforeHeaderCell, chevronCell, afterHeaderCell},
			},
			{
				MaxHeight: bodyPt,
				Cells:     []*jsonschema.GridCellInput{beforeBodyCell, afterBodyCell},
			},
		},
	}

	return grid, nil
}
