package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/icons"
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
func (ir *iconRow) UseWhen() string {
	return "3-6 short labeled icons in a single row; prefer process-flow when steps have sequence, card-grid when items need multi-line body text"
}
func (ir *iconRow) NotWhen() string {
	return "Items are sequential steps (use process-flow), items need body text beyond a caption (use card-grid), or content is a single metric (use stat-hero)"
}
func (ir *iconRow) Version() int { return 1 }
func (ir *iconRow) CellsHint() string { return "3-5" }
func (ir *iconRow) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "data-display",
		NarrativeRole: []string{"evidence"},
		PairsWith:     []string{"kpi-3up", "card-grid", "process-flow"},
		DensityClass:       "low",
		AccentWeight:       "strong",
		SparseThresholdPct: 15,
	}
}

func (ir *iconRow) ExemplarValues() any {
	v := IconRowValues{
		{Icon: "rocket", Caption: "Launch"},
		{Icon: "trending-up", Caption: "Growth"},
		{Icon: "currency-dollar", Caption: "Revenue"},
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
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	IconSize       float64 `json:"icon_size,omitempty"`
	CaptionSize    float64 `json:"caption_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"` // uniform | alternate | progressive
	IconMode       string  `json:"icon_mode,omitempty"`        // text | svg | auto (default auto)
}

// IconRowCellOverride is an alias for the shared CellOverride struct.
type IconRowCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (ir *iconRow) NewValues() any      { return &IconRowValues{} }
func (ir *iconRow) NewOverrides() any   { return &IconRowOverrides{} }
func (ir *iconRow) NewCellOverride() any { return &IconRowCellOverride{} }


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
					"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"icon_size":        NumberSchema(6, 120).WithDescription("Font size for icon in points"),
					"caption_size":     NumberSchema(6, 120).WithDescription("Font size for caption in points"),
					"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
					"icon_mode":        EnumSchema("text", "svg", "auto").WithDescription("Icon rendering mode: text (emoji/glyph in text paragraph), svg (bundled SVG icon overlay), auto (SVG when icon name resolves to a bundled icon, text otherwise). Default: auto.").WithDefault("auto"),
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

	const name = "icon-row"
	var errs []error

	// Validate overrides
	if overrides != nil {
		if ovr, ok := overrides.(*IconRowOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.IconMode != "" {
				switch ovr.IconMode {
				case "text", "svg", "auto":
					// valid
				default:
					errs = append(errs, fmt.Errorf("%s: icon_mode must be \"text\", \"svg\", or \"auto\", got %q", name, ovr.IconMode))
				}
			}
		}
	}

	if len(*items) < 3 {
		errs = append(errs, errMinItems(name, "values", 3, len(*items), "(hint: use pattern kpi-3up for KPI-style cards)"))
	}
	if len(*items) > 5 {
		errs = append(errs, errMaxItems(name, "values", 5, len(*items), ""))
	}

	for i, item := range *items {
		iconPath := fmt.Sprintf("values[%d].icon", i)
		if item.Icon == "" {
			errs = append(errs, errRequired(name, iconPath))
		} else if len(item.Icon) > 20 {
			errs = append(errs, errMaxLength(name, iconPath, 20, len(item.Icon)))
		}
		captionPath := fmt.Sprintf("values[%d].caption", i)
		if item.Caption == "" {
			errs = append(errs, errRequired(name, captionPath))
		} else if len(item.Caption) > 60 {
			errs = append(errs, errMaxLength(name, captionPath, 60, len(item.Caption)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(*items), ""); coErr != nil {
		errs = append(errs, coErr)
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

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	iconSize := ResolveSize(ovr.IconSize, 28.0)
	captionSize := ResolveSize(ovr.CaptionSize, 12.0)
	cellAccentMode := ovr.CellAccentMode
	iconMode := ovr.IconMode
	if iconMode == "" {
		iconMode = "auto"
	}

	gridCells := make([]*jsonschema.GridCellInput, len(*items))
	for i, item := range *items {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)

		// Determine whether this icon should render as SVG or text.
		useSVG := iconMode == "svg" || (iconMode == "auto" && icons.Exists(item.Icon))

		var shape *jsonschema.ShapeSpecInput
		if useSVG {
			// SVG icon: caption-only text + icon overlay (same approach as kpi_parametric).
			captionContent := buildIconRowCaptionOnly(item.Caption, captionSize)
			shape = &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     captionContent,
				Icon: &jsonschema.IconInput{
					Name:     item.Icon,
					Fill:     accent,
					Position: "top",
				},
			}
		} else {
			// Text/emoji icon: render icon as a text paragraph.
			textContent := buildIconRowTextContent(item.Icon, iconSize, item.Caption, captionSize)
			shape = &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     textContent,
			}
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

// buildIconRowCaptionOnly creates a JSON text object with caption only (for SVG icon mode).
func buildIconRowCaptionOnly(caption string, captionSize float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: caption, Size: captionSize, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "bottom",
	}

	data, _ := json.Marshal(textObj)
	return data
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
