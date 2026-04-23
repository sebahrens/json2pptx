package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// icon-row pattern — horizontal row of icon+caption pairs
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&iconRow{})
}

type iconRow struct{}

func (ir *iconRow) Name() string        { return "icon-row" }
func (ir *iconRow) Description() string { return "Horizontal row of icon+caption pairs" }
func (ir *iconRow) UseWhen() string     { return "Icon + caption row" }
func (ir *iconRow) Version() int        { return 1 }
func (ir *iconRow) CellsHint() string   { return "3-5" }

func (ir *iconRow) ExemplarValues() any {
	v := IconRowValues{
		{Icon: "🚀", Caption: "Launch"},
		{Icon: "📈", Caption: "Growth"},
		{Icon: "💰", Caption: "Revenue"},
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// IconRowItem is a single icon+caption pair.
// Supports string shorthand: "Caption" or "icon | Caption".
type IconRowItem struct {
	Icon    string `json:"icon"`    // Icon name or hex glyph
	Caption string `json:"caption"` // Short caption text
}

// UnmarshalJSON supports string shorthand "Caption" or "icon | Caption", or object {icon, caption}.
func (item *IconRowItem) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if parts := strings.SplitN(s, " | ", 2); len(parts) == 2 {
			item.Icon = parts[0]
			item.Caption = parts[1]
		} else {
			item.Caption = s
		}
		return nil
	}
	type alias IconRowItem
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("IconRowItem must be string \"icon | Caption\" or {icon, caption}: %w", err)
	}
	*item = IconRowItem(a)
	return nil
}

// IconRowValues is the values type: 3–5 icon+caption pairs.
type IconRowValues = []IconRowItem

// IconRowOverrides contains pattern-level overrides for icon-row.
type IconRowOverrides struct {
	Accent      string  `json:"accent,omitempty"`
	IconSize    float64 `json:"icon_size,omitempty"`
	CaptionSize float64 `json:"caption_size,omitempty"`
}

// IconRowCellOverride contains per-cell overrides for icon-row.
type IconRowCellOverride struct {
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

func (ir *iconRow) NewValues() any      { return &IconRowValues{} }
func (ir *iconRow) NewOverrides() any   { return &IconRowOverrides{} }
func (ir *iconRow) NewCellOverride() any { return &IconRowCellOverride{} }

// iconRowCellOverrideAllowed is the whitelist of per-cell override keys (D15).
var iconRowCellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

func (ir *iconRow) Schema() *Schema {
	itemSchema := OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Caption\" or \"icon | Caption\""),
		ObjectSchema(
			map[string]*Schema{
				"icon":    StringSchema(20).WithDescription("Icon name or hex glyph (e.g. \"🚀\" or \"rocket\")"),
				"caption": StringSchema(60).WithDescription("Short caption text"),
			},
			[]string{"icon", "caption"},
		).WithAdditionalProperties(false),
	).WithDescription("Item: string \"icon | Caption\" or {icon, caption}")


	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(itemSchema, 3, 5).WithDescription("3–5 icon+caption pairs"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":       StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"icon_size":    NumberSchema(6, 120).WithDescription("Font size for icon in points"),
					"caption_size": NumberSchema(6, 120).WithDescription("Font size for caption in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Horizontal row of icon+caption pairs")
}

func (ir *iconRow) Validate(values, overrides any, cellOverrides map[int]any) error {
	items, ok := values.(*IconRowValues)
	if !ok || items == nil {
		return fmt.Errorf("icon-row: values must be []IconRowItem, got %T", values)
	}

	var errs []error

	if len(*items) < 3 {
		errs = append(errs, fmt.Errorf("icon-row: values must contain at least 3 items, got %d (hint: use pattern kpi-3up for KPI-style cards)", len(*items)))
	}
	if len(*items) > 5 {
		errs = append(errs, fmt.Errorf("icon-row: values must contain at most 5 items, got %d", len(*items)))
	}

	for i, item := range *items {
		if item.Icon == "" {
			errs = append(errs, fmt.Errorf("icon-row: values[%d].icon is required", i))
		} else if len(item.Icon) > 20 {
			errs = append(errs, fmt.Errorf("icon-row: values[%d].icon exceeds maxLength 20 (%d chars)", i, len(item.Icon)))
		}
		if item.Caption == "" {
			errs = append(errs, fmt.Errorf("icon-row: values[%d].caption is required", i))
		} else if len(item.Caption) > 60 {
			errs = append(errs, fmt.Errorf("icon-row: values[%d].caption exceeds maxLength 60 (%d chars)", i, len(item.Caption)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= len(*items) {
			errs = append(errs, fmt.Errorf("icon-row: cell_overrides key %d out of range [0,%d]", idx, len(*items)-1))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("icon-row: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("icon-row: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !iconRowCellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("icon-row: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

func (ir *iconRow) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	items, ok := values.(*IconRowValues)
	if !ok {
		return nil, fmt.Errorf("icon-row: values must be *IconRowValues, got %T", values)
	}
	ovr := &IconRowOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*IconRowOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("icon-row: overrides must be *IconRowOverrides, got %T", overrides)
		}
	}

	accent := "accent1"
	if ovr.Accent != "" {
		accent = ovr.Accent
	}
	iconSize := 28.0
	if ovr.IconSize > 0 {
		iconSize = ovr.IconSize
	}
	captionSize := 12.0
	if ovr.CaptionSize > 0 {
		captionSize = ovr.CaptionSize
	}

	gridCells := make([]*jsonschema.GridCellInput, len(*items))
	for i, item := range *items {
		textContent := buildIconRowTextContent(item.Icon, iconSize, item.Caption, captionSize)

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     textContent,
		}

		gc := &jsonschema.GridCellInput{
			Shape: shape,
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*IconRowCellOverride)
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

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, len(*items))),
		Gap:     12,
		Rows: []jsonschema.GridRowInput{
			{Cells: gridCells},
		},
	}

	return grid, nil
}

// buildIconRowTextContent creates a JSON text object with icon and caption paragraphs.
func buildIconRowTextContent(icon string, iconSize float64, caption string, captionSize float64) json.RawMessage {
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
			{Content: icon, Size: iconSize, Bold: false, Color: "lt1", Align: "ctr"},
			{Content: caption, Size: captionSize, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
