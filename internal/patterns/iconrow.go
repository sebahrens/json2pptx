package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
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
		ComposesWith:  []string{"stylish-panels", "pull-quote", "kpi-3up"},
		RoleOnSlide:   []string{"foundation", "banner"},
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
//
// The optional Secondary field embeds a small chart (sparkline / bar_chart /
// line_chart) rendered below the caption via a composite cell. Only one
// secondary is allowed per item (enforced by the field being a single pointer
// rather than an array).
type IconRowItem struct {
	Icon      string          `json:"icon"`                // Bundled icon name, inline SVG, data: URI, https URL, or local file path. Emoji glyphs are rejected.
	Caption   string          `json:"caption"`             // Short caption text
	Secondary *SecondaryChart `json:"secondary,omitempty"` // Optional embedded chart (one per item)
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
				"icon":      StringSchema(0).WithDescription("Bundled icon name (e.g. \"rocket\"), inline SVG, data: URI, https URL, or local file path. Emoji glyphs and unknown names are rejected."),
				"caption":   StringSchema(60).WithDescription("Short caption text"),
				"secondary": SecondaryChartSchema(),
			},
			[]string{"icon", "caption"},
		).WithAdditionalProperties(false),
	).WithDescription("Item: string \"icon | Caption\" or {icon, caption, secondary?}")


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
		} else if svggen.ClassifyIcon(item.Icon) == svggen.IconKindEmpty {
			errs = append(errs, &ValidationError{
				Pattern: name,
				Path:    iconPath,
				Code:    ErrCodeInvalidShape,
				Message: fmt.Sprintf("%s: %s must be a bundled icon name (e.g. \"rocket\"), inline SVG, data: URI, https URL, or local image path; emoji glyphs and unknown names are rejected, got %q", name, iconPath, item.Icon),
			})
		}
		captionPath := fmt.Sprintf("values[%d].caption", i)
		if item.Caption == "" {
			errs = append(errs, errRequired(name, captionPath))
		} else if len(item.Caption) > 60 {
			errs = append(errs, errMaxLength(name, captionPath, 60, len(item.Caption)))
		}
		if item.Secondary != nil {
			errs = append(errs, validateSecondaryChart(name, fmt.Sprintf("values[%d].secondary", i), item.Secondary)...)
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
	// icon_size override is retained on the schema for backward compatibility but
	// is unused now that icons render as an SVG overlay that sizes itself relative
	// to the cell. caption_size still controls the caption font size.
	captionSize := ResolveSize(ovr.CaptionSize, 12.0)
	cellAccentMode := ovr.CellAccentMode

	gridCells := make([]*jsonschema.GridCellInput, len(*items))
	for i, item := range *items {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)

		// SVG icon: caption-only text + icon overlay (same approach as kpi_parametric).
		// Validate has already rejected any icon that doesn't classify as a loadable
		// kind, so the loader is guaranteed to receive a bundled name, inline SVG,
		// data URI, URL, or file path.
		captionContent := buildIconRowCaptionOnly(item.Caption, captionSize)
		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     captionContent,
			Icon: &jsonschema.IconInput{
				Name:     item.Icon,
				Fill:     accent,
				Position: "top",
			},
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

		// When a secondary chart is attached, convert the cell to a composite
		// stack so the existing icon+caption shape is rendered on top and the
		// chart below.
		if item.Secondary != nil {
			gc = wrapCellWithSecondary(gc, item.Secondary, accent)
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
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

