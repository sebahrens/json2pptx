package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// stylish-panels pattern — accent-banded panels with ribbon headers
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&stylishPanels{})
}

type stylishPanels struct{}

func (sp *stylishPanels) Name() string        { return "stylish-panels" }
func (sp *stylishPanels) Description() string { return "Accent-banded panels with ribbon headers for pillars, capabilities, or workstreams" }
func (sp *stylishPanels) UseWhen() string {
	return "3-5 titled content blocks with bullet lists, each representing a pillar, capability, or workstream; prefer card-grid when items need only header+body without bullets, icon-row when items are icon+caption pairs"
}
func (sp *stylishPanels) NotWhen() string {
	return "Items are icon+caption pairs (use icon-row), items need only header+body text (use card-grid), or content is a single metric (use stat-hero)"
}
func (sp *stylishPanels) Version() int    { return 1 }
func (sp *stylishPanels) CellsHint() string { return "3-5" }
func (sp *stylishPanels) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "strong",
	}
}
func (sp *stylishPanels) SupportsCallout() bool        { return true }
func (sp *stylishPanels) SupportsInlineMarkdown() bool { return true }

func (sp *stylishPanels) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 2},
		{Columns: 4, Rows: 2},
		{Columns: 5, Rows: 2},
	}
}

func (sp *stylishPanels) ExemplarValues() any {
	return &StylishPanelsValues{
		{Title: "Strategy", Body: []string{"Market analysis", "Competitive positioning", "Growth targets"}},
		{Title: "Execution", Body: []string{"Sprint planning", "Resource allocation", "Risk management"}},
		{Title: "Measurement", Body: []string{"KPI tracking", "Quarterly reviews", "Course correction"}},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// StylishPanelsItem is a single panel with a title and bullet list body.
type StylishPanelsItem struct {
	Title string   `json:"title"`
	Body  []string `json:"body"`
}

// UnmarshalJSON supports object {title, body} form.
func (item *StylishPanelsItem) UnmarshalJSON(data []byte) error {
	type alias StylishPanelsItem
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("StylishPanelsItem must be {title, body}: %w", err)
	}
	*item = StylishPanelsItem(a)
	return nil
}

// StylishPanelsValues is the values type: 3–5 panel items.
type StylishPanelsValues = []StylishPanelsItem

// StylishPanelsOverrides contains pattern-level overrides.
type StylishPanelsOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"` // uniform | alternate | progressive
}

// StylishPanelsCellOverride is an alias for the shared CellOverride struct.
type StylishPanelsCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (sp *stylishPanels) NewValues() any      { return &StylishPanelsValues{} }
func (sp *stylishPanels) NewOverrides() any   { return &StylishPanelsOverrides{} }
func (sp *stylishPanels) NewCellOverride() any { return &StylishPanelsCellOverride{} }

func (sp *stylishPanels) Schema() *Schema {
	itemSchema := ObjectSchema(
		map[string]*Schema{
			"title": StringSchema(80).WithDescription("Panel header title"),
			"body":  ArraySchema(StringSchema(200), 1, 8).WithDescription("Bullet list items for the panel body"),
		},
		[]string{"title", "body"},
	).WithAdditionalProperties(false).WithDescription("Panel with titled header and bulleted body")

	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(itemSchema, 3, 5).WithDescription("3–5 panel items"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent2)").WithDefault("accent2"),
					"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"header_size":      NumberSchema(6, 120).WithDescription("Font size for panel headers in points"),
					"body_size":        NumberSchema(6, 120).WithDescription("Font size for body bullet text in points"),
					"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default), alternate, progressive").WithDefault("uniform"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Accent-banded panels with ribbon headers for pillars, capabilities, or workstreams")
}

func (sp *stylishPanels) Validate(values, overrides any, cellOverrides map[int]any) error {
	items, ok := values.(*StylishPanelsValues)
	if !ok || items == nil {
		return fmt.Errorf("stylish-panels: values must be []StylishPanelsItem, got %T", values)
	}

	const name = "stylish-panels"
	var errs []error

	// Validate overrides
	if overrides != nil {
		if ovr, ok := overrides.(*StylishPanelsOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(*items) < 3 {
		errs = append(errs, errMinItems(name, "values", 3, len(*items), "(hint: use card-grid for fewer items)"))
	}
	if len(*items) > 5 {
		errs = append(errs, errMaxItems(name, "values", 5, len(*items), ""))
	}

	for i, item := range *items {
		titlePath := fmt.Sprintf("values[%d].title", i)
		if item.Title == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if len(item.Title) > 80 {
			errs = append(errs, errMaxLength(name, titlePath, 80, len(item.Title)))
		}
		bodyPath := fmt.Sprintf("values[%d].body", i)
		if len(item.Body) == 0 {
			errs = append(errs, errRequired(name, bodyPath))
		}
		if len(item.Body) > 8 {
			errs = append(errs, errMaxItems(name, bodyPath, 8, len(item.Body), ""))
		}
		for j, bullet := range item.Body {
			bulletPath := fmt.Sprintf("values[%d].body[%d]", i, j)
			if bullet == "" {
				errs = append(errs, errRequired(name, bulletPath))
			} else if len(bullet) > 200 {
				errs = append(errs, errMaxLength(name, bulletPath, 200, len(bullet)))
			}
		}
	}

	// Validate cell_overrides keys
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(*items), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (sp *stylishPanels) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	items, ok := values.(*StylishPanelsValues)
	if !ok {
		return nil, fmt.Errorf("stylish-panels: values must be *StylishPanelsValues, got %T", values)
	}
	ovr := &StylishPanelsOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*StylishPanelsOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("stylish-panels: overrides must be *StylishPanelsOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	if baseAccent == "" {
		baseAccent = "accent2"
	}
	headerSize := ResolveSize(ovr.HeaderSize, 16.0)
	bodySize := ResolveSize(ovr.BodySize, 14.0)
	cellAccentMode := ovr.CellAccentMode

	n := len(*items)

	// Row 1: accent header cells (short, colored band)
	headerCells := make([]*jsonschema.GridCellInput, n)
	for i, item := range *items {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		headerCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     buildStylishHeaderText(item.Title, headerSize),
			},
		}
	}

	// Row 2: body cells (light tinted, with bullet text)
	bodyCells := make([]*jsonschema.GridCellInput, n)
	for i, item := range *items {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		bodyCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Text:     buildStylishBodyText(item.Body, bodySize, accent),
			},
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*StylishPanelsCellOverride)
			if coOk && cellOvr.AccentBar {
				bodyCells[i].AccentBar = &jsonschema.AccentBarInput{
					Position: "top",
					Color:    accent,
					Width:    4,
				}
			}
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:     12,
		Rows: []jsonschema.GridRowInput{
			{Cells: headerCells, Height: 20},
			{Cells: bodyCells},
		},
	}

	return grid, nil
}

// buildStylishHeaderText creates bold centered white text for the accent header band.
func buildStylishHeaderText(title string, headerSize float64) json.RawMessage {
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
			{Content: title, Size: headerSize, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// buildStylishBodyText creates bullet-list text on a light background.
func buildStylishBodyText(bullets []string, bodySize float64, accent string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	paras := make([]paragraph, len(bullets))
	for i, b := range bullets {
		paras[i] = paragraph{
			Content: "• " + pptx.ConvertMarkdownEmphasis(b),
			Size:    bodySize,
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

