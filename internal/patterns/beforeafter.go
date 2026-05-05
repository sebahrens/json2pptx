package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// before-after pattern — two-column From → To with chevron separator
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&beforeAfter{})
}

type beforeAfter struct{}

func (b *beforeAfter) Name() string        { return "before-after" }
func (b *beforeAfter) Description() string { return "Two-column before/after with transition chevron" }
func (b *beforeAfter) UseWhen() string     { return "Before/after, current vs future state, from→to" }
func (b *beforeAfter) Version() int        { return 1 }
func (b *beforeAfter) CellsHint() string { return "2 + header" }
func (b *beforeAfter) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"compare"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (b *beforeAfter) SupportsCallout() bool        { return true }
func (b *beforeAfter) SupportsInlineMarkdown() bool { return true }

func (b *beforeAfter) ExemplarValues() any {
	return &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Current State", Items: []string{"Manual process", "3-day turnaround", "Error-prone"}},
		After:  BeforeAfterColumn{Header: "Future State", Items: []string{"Automated", "Same-day", "99.9% accuracy"}},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// BeforeAfterColumn holds header + bullet items for one side.
type BeforeAfterColumn struct {
	Header string   `json:"header"`
	Items  []string `json:"items"`
}

// BeforeAfterValues holds the before and after columns.
type BeforeAfterValues struct {
	Before BeforeAfterColumn `json:"before"`
	After  BeforeAfterColumn `json:"after"`
}

// BeforeAfterOverrides is the standard text overrides.
type BeforeAfterOverrides = TextOverrides

// BeforeAfterCellOverride is the shared per-cell override.
type BeforeAfterCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (b *beforeAfter) NewValues() any      { return &BeforeAfterValues{} }
func (b *beforeAfter) NewOverrides() any   { return &BeforeAfterOverrides{} }
func (b *beforeAfter) NewCellOverride() any { return &BeforeAfterCellOverride{} }

func (b *beforeAfter) Schema() *Schema {
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
	}).WithDescription("Two-column before/after with transition chevron")
}

func (b *beforeAfter) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*BeforeAfterValues)
	if !ok || vals == nil {
		return fmt.Errorf("before-after: values must be *BeforeAfterValues, got %T", values)
	}

	const name = "before-after"
	var errs []error

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

	// Total cells: 2 headers + chevron + 2 body cells = 5
	totalCells := 5
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (b *beforeAfter) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*BeforeAfterValues)
	if !ok {
		return nil, fmt.Errorf("before-after: values must be *BeforeAfterValues, got %T", values)
	}
	ovr := &BeforeAfterOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*BeforeAfterOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("before-after: overrides must be *BeforeAfterOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 16.0)
	bodySize := ResolveSize(ovr.BodySize, 12.0)

	cellIdx := 0

	// Header row: Before header | chevron | After header
	beforeHeader := buildBeforeAfterTextContent(vals.Before.Header, headerSize, true, "lt1", "ctr")
	afterHeader := buildBeforeAfterTextContent(vals.After.Header, headerSize, true, "lt1", "ctr")
	chevronText := buildBeforeAfterTextContent("→", 24.0, true, "lt1", "ctr")

	beforeHeaderCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     beforeHeader,
		},
	}
	applyBeforeAfterCellOverride(beforeHeaderCell, cellOverrides, cellIdx, accent)
	cellIdx++

	chevronCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "chevron",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     chevronText,
		},
	}
	applyBeforeAfterCellOverride(chevronCell, cellOverrides, cellIdx, accent)
	cellIdx++

	afterHeaderCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     afterHeader,
		},
	}
	applyBeforeAfterCellOverride(afterHeaderCell, cellOverrides, cellIdx, accent)
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
	applyBeforeAfterCellOverride(beforeBodyCell, cellOverrides, cellIdx, accent)
	cellIdx++

	afterBodyCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     afterBody,
		},
	}
	applyBeforeAfterCellOverride(afterBodyCell, cellOverrides, cellIdx, accent)

	// 3-column grid: [45%, 10%, 45%]
	colsJSON := json.RawMessage(`[45, 10, 45]`)

	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		Gap:     8,
		Rows: []jsonschema.GridRowInput{
			{
				Height: 25,
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

func buildBeforeAfterTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
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
			{Content: content, Size: size, Bold: bold, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func buildBeforeAfterBulletContent(items []string, size float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	paras := make([]paragraph, len(items))
	for i, item := range items {
		paras[i] = paragraph{
			Content: "• " + pptx.ConvertMarkdownEmphasis(item),
			Size:    size,
			Color:   "dk1",
			Align:   "l",
		}
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

func applyBeforeAfterCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*BeforeAfterCellOverride)
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
