package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
func (ir *iconRow) UseWhen() string {
	return "3-6 short labeled icons in a single row; prefer process-flow when steps have sequence, card-grid when items need multi-line body text"
}
func (ir *iconRow) NotWhen() string {
	return "Items are sequential steps (use process-flow), items need body text beyond a caption (use card-grid), or content is a single metric (use stat-hero)"
}
func (ir *iconRow) Version() int      { return 2 }
func (ir *iconRow) CellsHint() string { return "3-5" }
func (ir *iconRow) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "data-display",
		NarrativeRole:      []string{"evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "process-flow"},
		ComposesWith:       []string{"stylish-panels", "pull-quote", "kpi-3up"},
		RoleOnSlide:        []string{"foundation", "banner"},
		DensityClass:       "low",
		AccentWeight:       "strong",
		SparseThresholdPct: 15,
	}
}

func (ir *iconRow) ExemplarValues() any {
	v := IconRowValues{
		{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
		{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth"},
		{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue"},
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
	Icon      *IconRef        `json:"icon"`                // Bundled icon name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object. Emoji glyphs are rejected.
	Caption   string          `json:"caption"`             // Short caption text
	Secondary *SecondaryChart `json:"secondary,omitempty"` // Optional embedded chart (one per item)
}

// UnmarshalJSON supports string shorthand "Caption" or "icon | Caption", or object {icon, caption}.
// The icon segment of the shorthand is classified via parseIconString so URLs,
// inline SVG, file paths, and bundled names all route to the correct IconRef
// field.
func (item *IconRowItem) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if parts := strings.SplitN(s, " | ", 2); len(parts) == 2 {
			ref := parseIconString(parts[0])
			item.Icon = &ref
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

func (ir *iconRow) NewValues() any       { return &IconRowValues{} }
func (ir *iconRow) NewOverrides() any    { return &IconRowOverrides{} }
func (ir *iconRow) NewCellOverride() any { return &IconRowCellOverride{} }

func (ir *iconRow) Schema() *Schema {
	itemSchema := OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Caption\" or \"icon | Caption\""),
		ObjectSchema(
			map[string]*Schema{
				"icon":      IconRefSchema("Icon: bundled name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object. Emoji glyphs and unknown bundled names are rejected."),
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
		if item.Icon == nil || item.Icon.IsEmpty() {
			errs = append(errs, errRequired(name, iconPath))
		} else {
			errs = append(errs, validateIconRef(name, iconPath, *item.Icon)...)
		}
		captionPath := fmt.Sprintf("values[%d].caption", i)
		if item.Caption == "" {
			errs = append(errs, errRequired(name, captionPath))
		} else if runeLen(item.Caption) > 60 {
			errs = append(errs, errMaxLength(name, captionPath, 60, runeLen(item.Caption)))
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
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)

		// SVG icon: caption-only text + icon overlay (same approach as kpi_parametric).
		// Validate has already rejected any icon that doesn't classify as a loadable
		// kind, so the loader is guaranteed to receive a bundled name, inline SVG,
		// data URI, URL, or file path.
		captionContent := buildIconRowCaptionOnly(item.Caption, captionSize)
		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     captionContent,
		}
		if item.Icon != nil {
			shape.Icon = item.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), "top")
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
		Gap:     iconRowGapPt,
		Rows: []jsonschema.GridRowInput{
			{Cells: gridCells},
		},
	}

	// Size the row to the icon and its caption, and centre it. A single row
	// with no max_height stretches to the whole content zone, so an icon and
	// a five-word caption floated in a card three times taller than its
	// content (go-slide-creator-tee7). A cell carrying a secondary chart is
	// left to fill the zone: the chart needs the height, and capping the row
	// would squash it.
	if !iconRowHasSecondary(*items) {
		grid.Rows[0].MaxHeight = math.Round(iconRowMaxHeightPt(ctx, gridCells))
		grid.VerticalAlign = GridVerticalAlignDefault
	}

	return grid, nil
}

const (
	// iconRowGapPt is the gap between icon cards.
	iconRowGapPt = 12.0
	// iconRowMaxHeightFrac caps the row at this share of the content area,
	// matching the KPI cards' cap: past it the row stops reading as a strip.
	iconRowMaxHeightFrac = 0.45
	// iconRowMinHeightPt keeps a card tall enough for a recognisable icon over
	// one line of caption.
	iconRowMinHeightPt = 96.0
)

// iconRowHasSecondary reports whether any item attaches a secondary chart, in
// which case the cell becomes a composite stack that needs the full zone.
func iconRowHasSecondary(items IconRowValues) bool {
	for _, item := range items {
		if item.Secondary != nil {
			return true
		}
	}
	return false
}

// iconRowMaxHeightPt is the height one icon card needs for its top icon and
// its caption, capped at a share of the content area.
//
// contentCardHeightPt solves the circularity the icon creates — the renderer
// sizes a "top" overlay from the card's own height — so the height comes from
// the same helper card-grid and hero-detail use.
func iconRowMaxHeightPt(ctx ExpandContext, cells []*jsonschema.GridCellInput) float64 {
	n := len(cells)
	areaW, areaH := sizingAreaPt(ctx)
	if n == 0 || areaW <= 0 || areaH <= 0 {
		return 0
	}
	cardW := equalColumnWidthPt(areaW, n, iconRowGapPt)
	textW := cardW - 2*defaultShapeInsetLRPt
	if textW <= 0 {
		return 0
	}

	font := ctx.Theme.BodyFont
	need := 0.0
	for _, c := range cells {
		if c == nil || c.Shape == nil {
			continue
		}
		textH := shapeTextHeightPt(font, c.Shape.Text, textW)
		need = math.Max(need, contentCardHeightPt(textH, cardW, c.Shape.Icon != nil))
	}
	return clampPt(need, iconRowMinHeightPt, areaH*iconRowMaxHeightFrac)
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
