package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// agenda pattern — numbered section list for table-of-contents slides
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&agenda{})
}

type agenda struct{}

func (a *agenda) Name() string        { return "agenda" }
func (a *agenda) Description() string { return "Numbered section list for agenda / table-of-contents slides" }
func (a *agenda) UseWhen() string     { return "Agenda, table of contents, or section overview" }
func (a *agenda) Version() int        { return 1 }
func (a *agenda) CellsHint() string   { return "2-10" }

func (a *agenda) ExemplarValues() any {
	v := AgendaValues{
		Items: []string{"Introduction", "Market Analysis", "Strategy", "Next Steps"},
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// AgendaValues holds the section titles for the agenda pattern.
type AgendaValues struct {
	Items []string `json:"items"` // Section titles in order
}

// AgendaOverrides contains pattern-level overrides for the agenda.
type AgendaOverrides struct {
	Accent         string `json:"accent,omitempty"`          // Accent scheme color for number badges
	SemanticAccent string `json:"semantic_accent,omitempty"` // Semantic accent role
	Highlight      int    `json:"highlight,omitempty"`       // 1-based index of item to highlight (0 = none)
	NumberSize     float64 `json:"number_size,omitempty"`    // Font size for number in points
	TitleSize      float64 `json:"title_size,omitempty"`     // Font size for title in points
}

// AgendaCellOverride is an alias for the shared CellOverride struct.
type AgendaCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (a *agenda) NewValues() any      { return &AgendaValues{} }
func (a *agenda) NewOverrides() any   { return &AgendaOverrides{} }
func (a *agenda) NewCellOverride() any { return &AgendaCellOverride{} }

func (a *agenda) Schema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"values": ObjectSchema(
				map[string]*Schema{
					"items": ArraySchema(StringSchema(100).WithDescription("Section title"), 2, 10).
						WithDescription("Section titles in order"),
				},
				[]string{"items"},
			).WithAdditionalProperties(false),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color for number badges (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role"),
					"highlight":       NumberSchema(0, 10).WithDescription("1-based index of item to highlight (0 = none)"),
					"number_size":     NumberSchema(6, 120).WithDescription("Font size for number in points"),
					"title_size":      NumberSchema(6, 120).WithDescription("Font size for title in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Numbered section list for agenda / table-of-contents slides")
}

func (a *agenda) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*AgendaValues)
	if !ok || v == nil {
		return fmt.Errorf("agenda: values must be *AgendaValues, got %T", values)
	}

	const name = "agenda"
	var errs []error

	if len(v.Items) < 2 {
		errs = append(errs, errMinItems(name, "items", 2, len(v.Items), ""))
	}
	if len(v.Items) > 10 {
		errs = append(errs, errMaxItems(name, "items", 10, len(v.Items), ""))
	}

	for i, item := range v.Items {
		path := fmt.Sprintf("items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(item) > 100 {
			errs = append(errs, errMaxLength(name, path, 100, len(item)))
		}
	}

	if overrides != nil {
		ovr, ovrOk := overrides.(*AgendaOverrides)
		if !ovrOk {
			errs = append(errs, fmt.Errorf("agenda: overrides must be *AgendaOverrides, got %T", overrides))
		} else if ovr.Highlight > len(v.Items) {
			errs = append(errs, fmt.Errorf("agenda: overrides.highlight (%d) exceeds item count (%d)", ovr.Highlight, len(v.Items)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(v.Items), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (a *agenda) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*AgendaValues)
	if !ok {
		return nil, fmt.Errorf("agenda: values must be *AgendaValues, got %T", values)
	}

	ovr := &AgendaOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*AgendaOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("agenda: overrides must be *AgendaOverrides, got %T", overrides)
		}
	}

	accent := ResolveAccent(ovr.Accent, ovr.SemanticAccent, ctx.Metadata)
	numberSize := ResolveSize(ovr.NumberSize, 20.0)
	titleSize := ResolveSize(ovr.TitleSize, 14.0)

	rows := make([]jsonschema.GridRowInput, len(v.Items))
	for i, title := range v.Items {
		num := fmt.Sprintf("%02d", i+1)
		isHighlighted := ovr.Highlight > 0 && ovr.Highlight == i+1

		// Number badge cell
		numberFill := accent
		numberColor := "lt1"
		if !isHighlighted && ovr.Highlight > 0 {
			// Dim non-highlighted items
			numberFill = "lt2"
			numberColor = "dk1"
		}

		numberText := buildAgendaTextContent(num, numberSize, true, numberColor, "ctr")
		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, numberFill)),
				Text:     numberText,
			},
		}

		// Title cell
		titleColor := "dk1"
		titleBold := false
		if isHighlighted {
			titleBold = true
		} else if ovr.Highlight > 0 {
			titleColor = "dk2" // dim non-highlighted
		}

		titleText := buildAgendaTitleContent(title, titleSize, titleBold, titleColor)
		titleCell := &jsonschema.GridCellInput{
			ColSpan: 5,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     titleText,
			},
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			if cellOvr, coOk := co.(*AgendaCellOverride); coOk {
				if cellOvr.AccentBar {
					titleCell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    4,
					}
				}
			}
		}

		rows[i] = jsonschema.GridRowInput{
			AutoHeight: true,
			Cells:      []*jsonschema.GridCellInput{numberCell, titleCell},
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[1, 5]`),
		Gap:     8,
		Rows:    rows,
	}

	return grid, nil
}

// buildAgendaTextContent creates a centered text object for the number badge.
func buildAgendaTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
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

// buildAgendaTitleContent creates a left-aligned text object for the section title.
func buildAgendaTitleContent(content string, size float64, bold bool, color string) json.RawMessage {
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
			{Content: content, Size: size, Bold: bold, Color: color, Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
