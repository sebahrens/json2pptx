package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
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

func (ir *iconRow) Name() string { return "icon-row" }
func (ir *iconRow) Description() string {
	return "Horizontal row of 3-5 open icons, each over a caption and an optional one-line description"
}
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
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}

func (ir *iconRow) ExemplarValues() any {
	v := IconRowValues{
		{Icon: &IconRef{Name: "rocket"}, Caption: "Launch", Description: "Live in two pilot markets by June"},
		{Icon: &IconRef{Name: "trending-up"}, Caption: "Growth", Description: "Twelve markets by year end"},
		{Icon: &IconRef{Name: "currency-dollar"}, Caption: "Revenue", Description: "Break-even in the fourth quarter"},
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
	Icon        *IconRef        `json:"icon"`                  // Bundled icon name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object. Emoji glyphs are rejected.
	Caption     string          `json:"caption"`               // Short caption text
	Description string          `json:"description,omitempty"` // Optional one-line description under the caption
	Secondary   *SecondaryChart `json:"secondary,omitempty"`   // Optional embedded chart (one per item)
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
	// Style is "open" (default: the icon and its caption stand on the slide,
	// no container) or "tile" (each item in a neutral tile under an accent
	// rule, the look before go-slide-creator-hjqn2).
	Style string `json:"style,omitempty"`
}

// iconRowStyles are the accepted overrides.style values.
var iconRowStyles = []string{"open", "tile"}

// iconRowDescriptionMax bounds the optional description: one line under the
// caption on a five-item row is about this many characters.
const iconRowDescriptionMax = 80

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
				"icon":        IconRefSchema("Icon: bundled name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object. Emoji glyphs and unknown bundled names are rejected."),
				"caption":     StringSchema(60).WithDescription("Short caption text"),
				"description": StringSchema(iconRowDescriptionMax).WithDescription("Optional one-line description under the caption (about 40 characters stay on one line with five items)"),
				"secondary":   SecondaryChartSchema(),
			},
			[]string{"icon", "caption"},
		).WithAdditionalProperties(false),
	).WithDescription("Item: string \"icon | Caption\" or {icon, caption, description?, secondary?}")

	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(itemSchema, 3, 5).WithDescription("3–5 icon+caption pairs"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"icon_size":        NumberSchema(6, 120).WithDescription("Icon height in points for the open style (default: scaled to the content area, 40-88)"),
					"caption_size":     NumberSchema(6, 120).WithDescription("Font size for caption in points"),
					"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
					"style":            EnumSchema(iconRowStyles...).WithDescription("open (default: accent icons and captions on the slide, no container) or tile (each item in a neutral tile under an accent rule). Items with a secondary chart always render as tiles.").WithDefault("open"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Horizontal row of open icons over captions")
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
			if ovr.Style != "" && !slices.Contains(iconRowStyles, ovr.Style) {
				errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, iconRowStyles))
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
		if n := runeLen(item.Description); n > iconRowDescriptionMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].description", i), iconRowDescriptionMax, n))
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
	if iconRowOpen(*items, ovr) {
		return expandIconRowOpen(ctx, *items, ovr, cellOverrides, baseAccent), nil
	}
	// Tile style: the icon is an SVG overlay that sizes itself relative to its
	// tile, so icon_size is unused here. caption_size controls the caption.
	captionSize := ResolveSize(ovr.CaptionSize, scaleBodyPt)
	cellAccentMode := ovr.CellAccentMode

	gridCells := make([]*jsonschema.GridCellInput, len(*items))
	for i, item := range *items {
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)

		// SVG icon: caption-only text + icon overlay (same approach as kpi_parametric).
		// Validate has already rejected any icon that doesn't classify as a loadable
		// kind, so the loader is guaranteed to receive a bundled name, inline SVG,
		// data URI, URL, or file path.
		captionContent := buildIconRowTileText(item, captionSize)
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
			applyCellTextOverride(gc, cellOvr)
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
		Gap:     ctx.Gap(iconRowGapPt),
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

// iconRowOpen reports whether the row renders in the open style: the default,
// unless overrides.style is "tile" or an item carries a secondary chart (the
// chart needs the composite tile it has always rendered in).
func iconRowOpen(items IconRowValues, ovr *IconRowOverrides) bool {
	return ovr.Style != "tile" && !iconRowHasSecondary(items)
}

// Open icon-row geometry (go-slide-creator-hjqn2).
const (
	// iconRowOpenIconFrac is the icon height's share of the content-area
	// height, so the icons grow with the slide instead of staying a fixed
	// small glyph; iconRowOpenIconMinPt / MaxPt bound it.
	iconRowOpenIconFrac  = 0.26
	iconRowOpenIconMinPt = 40.0
	iconRowOpenIconMaxPt = 88.0
	// iconRowOpenIconWidthFrac keeps an icon inside its column on narrow rows.
	iconRowOpenIconWidthFrac = 0.5
	// iconRowOpenRowGapPt separates the icon row from the caption row; the
	// caption's own top margin does the rest.
	iconRowOpenRowGapPt = 2.0
	// iconRowOpenGapPt is the gutter between items.
	iconRowOpenGapPt = 16.0
)

// iconRowOpenIconPt is the icon height of the open style: icon_size when the
// author set one, else a share of the content-area height, bounded and never
// wider than half its column.
func iconRowOpenIconPt(ctx ExpandContext, n int, ovr *IconRowOverrides) float64 {
	areaW, areaH := sizingAreaPt(ctx)
	colW := equalColumnWidthPt(areaW, n, ctx.Gap(iconRowOpenGapPt))
	size := clampPt(areaH*iconRowOpenIconFrac, iconRowOpenIconMinPt, iconRowOpenIconMaxPt)
	if ovr.IconSize > 0 {
		size = ovr.IconSize
	}
	return math.Round(math.Min(size, math.Max(colW*iconRowOpenIconWidthFrac, 1)))
}

// iconRowOpenCaptionSize is the caption size of the open style: a bold
// subhead under a large icon unless the author set caption_size.
func iconRowOpenCaptionSize(ovr *IconRowOverrides) float64 {
	return ResolveSize(ovr.CaptionSize, scaleSubheadPt)
}

// expandIconRowOpen renders the items without containers: a row of accent
// icons standing on the slide, and under it a row of top-anchored captions
// (bold) with an optional muted description line. Both rows are one per item
// column, so every icon sits on one line and every caption starts on one
// baseline whatever its neighbours' length.
func expandIconRowOpen(ctx ExpandContext, items IconRowValues, ovr *IconRowOverrides, cellOverrides map[int]any, baseAccent string) *jsonschema.ShapeGridInput {
	n := len(items)
	captionSize := iconRowOpenCaptionSize(ovr)
	areaW, _ := sizingAreaPt(ctx)
	colW := equalColumnWidthPt(areaW, n, ctx.Gap(iconRowOpenGapPt))

	iconCells := make([]*jsonschema.GridCellInput, n)
	captionCells := make([]*jsonschema.GridCellInput, n)
	captionPt := 0.0
	for i, item := range items {
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		iconCells[i] = &jsonschema.GridCellInput{}
		if item.Icon != nil {
			iconCells[i].Icon = item.Icon.Resolve(iconFillOn(ctx, nil, accent), "")
			iconCells[i].Fit = "contain"
		}
		captionCells[i] = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     noLine,
			Text:     buildIconRowOpenText(item, captionSize),
		}}
		if co, ok := cellOverrides[i].(*IconRowCellOverride); ok {
			applyCellTextOverride(captionCells[i], co)
			if co.AccentBar {
				captionCells[i].AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: peerRuleWidthPt}
			}
		}
		captionPt = math.Max(captionPt, rowTextNeedPt(ctx.themeFonts(), captionCells[i].Shape.Text, colW))
	}

	iconPt := iconRowOpenIconPt(ctx, n, ovr)
	captionPt = math.Ceil(captionPt)
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, n)),
		ColGap:        ctx.Gap(iconRowOpenGapPt),
		RowGap:        iconRowOpenRowGapPt,
		VerticalAlign: GridVerticalAlignDefault,
		Rows: []jsonschema.GridRowInput{
			{MinHeight: iconPt, MaxHeight: iconPt, Cells: iconCells},
			{MinHeight: captionPt, MaxHeight: captionPt, Cells: captionCells},
		},
	}
}

// iconRowOpenNeedPt is the open row's height (icon, gap, tallest caption) and
// the content-area height it has to fit.
func iconRowOpenNeedPt(ctx ExpandContext, items IconRowValues, ovr *IconRowOverrides) (need, areaH float64) {
	grid := expandIconRowOpen(ctx, items, ovr, nil, "accent1")
	_, areaH = sizingAreaPt(ctx)
	return grid.Rows[0].MaxHeight + grid.RowGap + grid.Rows[1].MaxHeight, areaH
}

// buildIconRowOpenText is the open style's caption cell: a bold centred
// caption and, when given, a description line in the body size.
func buildIconRowOpenText(item IconRowItem, captionSize float64) json.RawMessage {
	paras := []chartInsightsParagraph{{Content: item.Caption, Size: captionSize, Bold: true, Color: "dk1", Align: "ctr"}}
	if d := strings.TrimSpace(item.Description); d != "" {
		paras[0].SpaceAfter = 2
		paras = append(paras, chartInsightsParagraph{Content: d, Size: scaleBodyPt, Color: "dk1", Align: "ctr"})
	}
	return insetText{Paragraphs: paras, Align: "ctr", VerticalAlign: "t"}.json()
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
//
// The caption height is the larger of the theme-font model and the written
// fit of the caption cell the writer measures (writtenFitHeightPt): the model
// alone let five 60-character captions be written at 7–10pt on the shipped
// content areas (go-slide-creator-n1muf). When the card needs more than the
// strip cap, the cap gives way up to the whole content area before the
// caption is shrunk; what still does not fit is reported by
// PostExpandWarnings as BODY_TOO_LONG.
func iconRowMaxHeightPt(ctx ExpandContext, cells []*jsonschema.GridCellInput) float64 {
	need, areaH := iconRowCardNeedPt(ctx, cells)
	if need <= 0 {
		return 0
	}
	if need > areaH*iconRowMaxHeightFrac {
		return clampPt(need, iconRowMinHeightPt, areaH)
	}
	return clampPt(need, iconRowMinHeightPt, areaH*iconRowMaxHeightFrac)
}

// iconRowCardNeedPt is the tallest card's content height (icon zone, caption
// at its written fit, padding and insets) and the content-area height.
func iconRowCardNeedPt(ctx ExpandContext, cells []*jsonschema.GridCellInput) (need, areaH float64) {
	n := len(cells)
	areaW, areaH := sizingAreaPt(ctx)
	if n == 0 || areaW <= 0 || areaH <= 0 {
		return 0, areaH
	}
	cardW := equalColumnWidthPt(areaW, n, ctx.Gap(iconRowGapPt))
	textW := cardW - 2*defaultShapeInsetLRPt
	if textW <= 0 {
		return 0, areaH
	}

	font := ctx.Theme.BodyFont
	for _, c := range cells {
		if c == nil || c.Shape == nil {
			continue
		}
		textH := math.Max(shapeTextHeightPt(font, c.Shape.Text, textW),
			writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, cardW, 0)-2*defaultShapeInsetTBPt)
		need = math.Max(need, contentCardHeightPt(textH, cardW, c.Shape.Icon != nil))
	}
	return need, areaH
}

// PostExpandWarnings reports captions whose cards need more height than the
// template's content area holds, so the writer would shrink them below the
// readable floor (go-slide-creator-n1muf). Without a known content area the
// schema's 60-character caption limit is the contract.
func (ir *iconRow) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	items, ok := values.(*IconRowValues)
	if !ok || items == nil || len(*items) == 0 || iconRowHasSecondary(*items) ||
		ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return nil
	}
	ovr, _ := overrides.(*IconRowOverrides)
	if ovr == nil {
		ovr = &IconRowOverrides{}
	}
	if iconRowOpen(*items, ovr) {
		need, areaH := iconRowOpenNeedPt(ctx, *items, ovr)
		if need <= areaH+1 {
			return nil
		}
		return []string{fmt.Sprintf("%s: icon-row icons and captions need %.0fpt at readable sizes but the content area holds about %.0fpt — shorten the captions or descriptions, or use fewer items", ErrCodeBodyTooLong, need, areaH)}
	}
	captionSize := ResolveSize(ovr.CaptionSize, scaleBodyPt)
	cells := make([]*jsonschema.GridCellInput, len(*items))
	for i, item := range *items {
		cells[i] = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Text: buildIconRowTileText(item, captionSize)}}
		if item.Icon != nil && !item.Icon.IsEmpty() {
			cells[i].Shape.Icon = &jsonschema.IconInput{Position: "top"}
		}
	}
	need, areaH := iconRowCardNeedPt(ctx, cells)
	if need <= areaH+1 {
		return nil
	}
	return []string{fmt.Sprintf("%s: icon-row captions need %.0fpt cards at readable sizes but the content area holds about %.0fpt — shorten the captions or use fewer items", ErrCodeBodyTooLong, need, areaH)}
}

// buildIconRowTileText is the tile style's text: the caption, and the optional
// description as a second line.
func buildIconRowTileText(item IconRowItem, captionSize float64) json.RawMessage {
	if strings.TrimSpace(item.Description) == "" {
		return buildIconRowCaptionOnly(item.Caption, captionSize)
	}
	paras := []chartInsightsParagraph{
		{Content: item.Caption, Size: captionSize, Bold: true, Color: "lt1", Align: "ctr"},
		{Content: strings.TrimSpace(item.Description), Size: captionSize, Color: "lt1", Align: "ctr"},
	}
	return insetText{Paragraphs: paras, Align: "ctr", VerticalAlign: "ctr"}.json()
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
