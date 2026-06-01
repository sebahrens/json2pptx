package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// card-grid pattern — parameterized N×M titled cards (D5)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&cardGrid{})
}

type cardGrid struct{}

func (c *cardGrid) Name() string        { return "card-grid" }
func (c *cardGrid) Description() string { return "Parameterized N×M grid of titled cards" }
func (c *cardGrid) UseWhen() string {
	return "4-9 freeform titled cards with custom content per cell; prefer kpi-3up for exactly 3 KPIs, comparison-2col for 2-column tradeoffs, bmc-canvas for a 9-block business model"
}
func (c *cardGrid) NotWhen() string {
	return "Exactly 3 numeric KPIs (use kpi-3up), two-column pros/cons (use comparison-2col), standard BMC (use bmc-canvas), or a single hero metric (use stat-hero)"
}
func (c *cardGrid) Version() int { return 2 }
func (c *cardGrid) CellsHint() string { return "rows × cols" }
func (c *cardGrid) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "data-display",
		NarrativeRole: []string{"evidence"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (c *cardGrid) SupportsCallout() bool        { return true }
func (c *cardGrid) SupportsInlineMarkdown() bool { return true }

func (c *cardGrid) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 2, Rows: 2},
		{Columns: 3, Rows: 2},
		{Columns: 2, Rows: 3},
	}
}

func (c *cardGrid) ExemplarValues() any {
	return &CardGridValues{
		Columns: 3,
		Rows:    2,
		Cells: []CardGridCell{
			{Header: "Card 1", Body: "Description 1"},
			{Header: "Card 2", Body: "Description 2"},
			{Header: "Card 3", Body: "Description 3"},
			{Header: "Card 4", Body: "Description 4"},
			{Header: "Card 5", Body: "Description 5"},
			{Header: "Card 6", Body: "Description 6"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CardGridCell is a single card with a header and body.
// Supports string shorthand: "Header | Body" unmarshals to {header:"Header", body:"Body"}.
// The optional Icon field is used only with the "icon-card" style.
//
// The optional Secondary field embeds a small chart (sparkline / bar_chart /
// line_chart) rendered below the card's title+body via a composite cell. Only
// one secondary is allowed per card (enforced by the field being a single
// pointer rather than an array).
type CardGridCell struct {
	Header    string          `json:"header"`
	Body      string          `json:"body"`
	Icon      *IconRef        `json:"icon,omitempty"`      // Icon: bundled-name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object
	Secondary *SecondaryChart `json:"secondary,omitempty"` // Optional embedded chart (one per cell)
}

// UnmarshalJSON supports string shorthand "Header | Body" or object {header, body}.
func (c *CardGridCell) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) != 2 {
			return &ValidationError{
				Pattern: "card-grid",
				Path:    "cells[]",
				Code:    ErrCodeInvalidShape,
				Message: fmt.Sprintf("CardGridCell string must be \"Header | Body\", got %q", s),
				Fix:     ReshapeValueFix("cells[]", `string "Header | Body" or {"header": "...", "body": "..."}`, "Header | Body"),
			}
		}
		c.Header = parts[0]
		c.Body = parts[1]
		return nil
	}
	type alias CardGridCell
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return &ValidationError{
			Pattern: "card-grid",
			Path:    "cells[]",
			Code:    ErrCodeInvalidShape,
			Message: fmt.Sprintf("CardGridCell must be string \"Header | Body\" or {header, body}: %v", err),
			Fix:     ReshapeValueFix("cells[]", `string "Header | Body" or {"header": "...", "body": "..."}`, "Header | Body"),
		}
	}
	*c = CardGridCell(a)
	return nil
}

// CardGridValues is the values type for card-grid.
type CardGridValues struct {
	Columns int            `json:"columns"`
	Rows    int            `json:"rows"`
	Cells   []CardGridCell `json:"cells"`
}

// CardGridOverrides extends TextOverrides with a Style field for visual variants
// plus generic surface overrides (card_fill, line_color, line_width, border) that
// apply on top of any style. The surface overrides let a caller paint cards with a
// pale brand surface (e.g. card_fill "#FFF5ED") and control the card border without
// hardcoding any template-specific color in the engine.
type CardGridOverrides struct {
	TextOverrides
	Style string `json:"style,omitempty"` // "filled" (default), "accent-stripe", "numbered-badge", "icon-card", "tinted", "soft-card"
	// CardFill overrides every card's fill with a caller-supplied hex (e.g. "#FFF5ED")
	// or scheme color name. Applies across all styles.
	CardFill string `json:"card_fill,omitempty"`
	// LineColor sets an explicit card border color (hex or scheme name). Takes
	// precedence over Border when set.
	LineColor string `json:"line_color,omitempty"`
	// LineWidth sets the card border width in points (0–12). Takes precedence over
	// Border when set; defaults to 1pt when LineColor is set without a width.
	LineWidth float64 `json:"line_width,omitempty"`
	// Border is a convenience border keyword: "none" (explicit no border), "subtle"
	// (thin dk1 hairline), or "accent" (1pt accent-colored border). Ignored when
	// LineColor/LineWidth are set.
	Border string `json:"border,omitempty"`
}

// validCardGridStyles enumerates the allowed style values.
var validCardGridStyles = map[string]bool{
	"":               true, // default = filled
	"filled":         true,
	"accent-stripe":  true,
	"numbered-badge": true,
	"icon-card":      true,
	"tinted":         true,
	"soft-card":      true,
}

// validCardGridBorders enumerates the allowed border keyword values.
var validCardGridBorders = map[string]bool{
	"":       true,
	"none":   true,
	"subtle": true,
	"accent": true,
}

// CardGridCellOverride is an alias for the shared CellOverride struct.
type CardGridCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (c *cardGrid) NewValues() any      { return &CardGridValues{} }
func (c *cardGrid) NewOverrides() any   { return &CardGridOverrides{} }
func (c *cardGrid) NewCellOverride() any { return &CardGridCellOverride{} }


func (c *cardGrid) Schema() *Schema {
	cellSchema := OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Header | Body\""),
		ObjectSchema(
			map[string]*Schema{
				"header":    StringSchema(80).WithDescription("Card header/title"),
				"body":      StringSchema(300).WithDescription("Card body content"),
				"icon":      IconRefSchema("Optional icon: bundled name string (e.g. \"rocket\") or {name|path|url|svg_data, fill?, alt?, position?} object. Used with icon-card style; also rendered as overlay when set with other styles."),
				"secondary": SecondaryChartSchema(),
			},
			[]string{"header", "body"},
		).WithAdditionalProperties(false),
	).WithDescription("Card cell: string \"Header | Body\" or {header, body, icon?, secondary?}")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"columns": IntegerSchema(1, 5).WithDescription("Number of columns (1–5)"),
			"rows":    IntegerSchema(1, 5).WithDescription("Number of rows (1–5)"),
			"cells":   ArraySchema(cellSchema, 1, 25).WithDescription("Cards in row-major order (length must equal columns × rows)"),
		},
		[]string{"columns", "rows", "cells"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":      NumberSchema(6, 120).WithDescription("Font size for headers in points"),
			"body_size":        NumberSchema(6, 120).WithDescription("Font size for body text in points"),
			"style":            EnumSchema("filled", "accent-stripe", "numbered-badge", "icon-card", "tinted", "soft-card").WithDescription("Visual style: filled (default solid accent cards), accent-stripe (left accent bar on light cards), numbered-badge (circled number badges), icon-card (bundled SVG icon badge above header), tinted (alternating lt1/lt2 backgrounds), soft-card (single pale surface, dark text, no border)").WithDefault("filled"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
			"card_fill":        StringSchema(0).WithDescription("Override every card's fill with a hex color (e.g. \"#FFF5ED\") or scheme color name. Applies across all styles; pair with soft-card or accent-stripe for a pale surface."),
			"line_color":       StringSchema(0).WithDescription("Card border color as a hex value or scheme color name. Takes precedence over border when set."),
			"line_width":       NumberSchema(0, 12).WithDescription("Card border width in points (0–12). Defaults to 1 when line_color is set without a width."),
			"border":           EnumSchema("none", "subtle", "accent").WithDescription("Border keyword: none (explicit no border), subtle (thin dk1 hairline), accent (1pt accent-colored border). Ignored when line_color/line_width are set."),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      overridesSchema,
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Parameterized N×M grid of titled cards")
}

func (c *cardGrid) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*CardGridValues)
	if !ok || vals == nil {
		return fmt.Errorf("card-grid: values must be *CardGridValues, got %T", values)
	}

	const name = "card-grid"
	var errs []error

	// Validate overrides enums
	errs = append(errs, validateCardGridOverrides(name, overrides)...)

	// Columns range
	if vals.Columns < 1 || vals.Columns > 5 {
		errs = append(errs, errOutOfRange(name, "columns", 1, 5, vals.Columns))
	}
	// Rows range
	if vals.Rows < 1 || vals.Rows > 5 {
		errs = append(errs, errOutOfRange(name, "rows", 1, 5, vals.Rows))
	}

	// Cell count must equal columns × rows (D4: hard error, no truncation)
	expectedCells := vals.Columns * vals.Rows
	countMatches := expectedCells > 0 && len(vals.Cells) == expectedCells
	if expectedCells > 0 && !countMatches {
		e := errCountMismatch(name, "cells", expectedCells, len(vals.Cells), "")
		e.Message = fmt.Sprintf("card-grid: cells must contain exactly %d items (columns=%d × rows=%d), got %d",
			expectedCells, vals.Columns, vals.Rows, len(vals.Cells))
		errs = append(errs, e)

		// Reverse-recommend: suggest alternative patterns that accept the actual cell count.
		if swaps := SuggestSwap(Default(), name, len(vals.Cells), false); len(swaps) > 0 {
			errs = append(errs, ErrWrongPatternFor(name, len(vals.Cells), swaps))
		}
	}

	// When the cell count is valid but all headers look like KPI metrics,
	// suggest the more specific KPI pattern (fewer tokens, better semantics).
	if countMatches && len(vals.Cells) > 0 && cellsLookLikeKPIs(vals.Cells) {
		if swaps := SuggestSwap(Default(), name, len(vals.Cells), true); len(swaps) > 0 {
			errs = append(errs, ErrWrongPatternFor(name, len(vals.Cells), swaps))
		}
	}

	// Per-cell validation
	for i, cell := range vals.Cells {
		path := fmt.Sprintf("cells[%d].header", i)
		if cell.Header == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(cell.Header) > 80 {
			errs = append(errs, errMaxLength(name, path, 80, len(cell.Header)))
		}
		bodyPath := fmt.Sprintf("cells[%d].body", i)
		if cell.Body == "" {
			errs = append(errs, errRequired(name, bodyPath))
		} else if len(cell.Body) > 300 {
			errs = append(errs, errMaxLength(name, bodyPath, 300, len(cell.Body)))
		}
		if cell.Secondary != nil {
			errs = append(errs, validateSecondaryChart(name, fmt.Sprintf("cells[%d].secondary", i), cell.Secondary)...)
		}
		if cell.Icon != nil {
			errs = append(errs, validateIconRef(name, fmt.Sprintf("cells[%d].icon", i), *cell.Icon)...)
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Cells), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (c *cardGrid) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*CardGridValues)
	if !ok {
		return nil, fmt.Errorf("card-grid: values must be *CardGridValues, got %T", values)
	}
	ovr := &CardGridOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*CardGridOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("card-grid: overrides must be *CardGridOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 16.0)
	bodySize := ResolveSize(ovr.BodySize, 12.0)
	style := ovr.Style
	if style == "" {
		style = "filled"
	}
	cellAccentMode := ovr.CellAccentMode

	var rows []jsonschema.GridRowInput
	cellIdx := 0

	for r := 0; r < vals.Rows; r++ {
		gridCells := make([]*jsonschema.GridCellInput, vals.Columns)
		for col := 0; col < vals.Columns; col++ {
			cell := vals.Cells[cellIdx]
			accent := ResolveCellAccent(baseAccent, cellIdx, cellAccentMode)
			gc := c.expandCell(ctx, cell, cellIdx, style, accent, headerSize, bodySize, ovr)

			// Apply cell overrides
			if co, ok := cellOverrides[cellIdx]; ok {
				cellOvr, coOk := co.(*CardGridCellOverride)
				if coOk && cellOvr.AccentBar {
					gc.AccentBar = &jsonschema.AccentBarInput{
						Position: "top",
						Color:    accent,
						Width:    4,
					}
				}
			}

			gridCells[col] = gc
			cellIdx++
		}
		rows = append(rows, jsonschema.GridRowInput{
			Cells: gridCells,
		})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, vals.Columns)),
		Gap:     10,
		Rows:    rows,
	}

	return grid, nil
}

// expandCell produces a single GridCellInput based on the selected visual style.
func (c *cardGrid) expandCell(ctx ExpandContext, cell CardGridCell, idx int, style, accent string, headerSize, bodySize float64, ovr *CardGridOverrides) *jsonschema.GridCellInput {
	var gc *jsonschema.GridCellInput
	switch style {
	case "accent-stripe":
		gc = c.expandAccentStripe(cell, accent, headerSize, bodySize)
	case "numbered-badge":
		gc = c.expandNumberedBadge(cell, idx, accent, headerSize, bodySize)
	case "icon-card":
		gc = c.expandIconCard(cell, accent, headerSize, bodySize)
	case "tinted":
		gc = c.expandTinted(ctx, cell, idx, accent, headerSize, bodySize)
	case "soft-card":
		gc = c.expandSoftCard(ctx, cell, accent, headerSize, bodySize)
	default: // "filled"
		gc = c.expandFilled(cell, accent, headerSize, bodySize)
	}
	// Apply generic surface overrides (card_fill / border) on top of the style.
	applyCardGridSurfaceOverrides(gc, ovr, accent)
	// Add SVG icon overlay when a bundled icon name or rich icon spec is provided.
	if cell.Icon != nil && gc.Shape != nil && gc.Shape.Icon == nil {
		gc.Shape.Icon = cell.Icon.Resolve(accent, "top")
	}
	// When a secondary chart is attached, convert the cell to a composite
	// stack so the existing text shape is rendered on top and the chart below.
	if cell.Secondary != nil {
		gc = wrapCellWithSecondary(gc, cell.Secondary, accent)
	}
	return gc
}

// expandFilled is the original style: solid accent fill, white text.
func (c *cardGrid) expandFilled(cell CardGridCell, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     buildCardGridTextContent(cell.Header, headerSize, cell.Body, bodySize),
		},
	}
}

// expandAccentStripe: light card with a left-edge accent bar.
func (c *cardGrid) expandAccentStripe(cell CardGridCell, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     buildCardGridDarkTextContent(cell.Header, headerSize, cell.Body, bodySize, accent),
		},
		AccentBar: &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		},
	}
}

// expandNumberedBadge: extracts leading number from header into a badge paragraph.
func (c *cardGrid) expandNumberedBadge(cell CardGridCell, idx int, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	badge, header := extractNumberPrefix(cell.Header, idx+1)
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     buildNumberedBadgeTextContent(badge, header, headerSize, cell.Body, bodySize, accent),
		},
	}
}

// expandIconCard: bundled SVG icon badge above header text on a light card with
// a top accent bar. The icon overlay itself is attached by expandCell when
// cell.Icon resolves to a bundled icon; when no icon is supplied the card
// renders gracefully as header+body with the accent bar still visible.
func (c *cardGrid) expandIconCard(cell CardGridCell, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     buildCardGridDarkTextContent(cell.Header, headerSize, cell.Body, bodySize, accent),
		},
		AccentBar: &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    3,
		},
	}
}

// expandTinted: alternating surface tint backgrounds with dark text.
// Uses SurfaceTints from template metadata when available (subtle/paper roles),
// falling back to lt1/lt2 for templates without surface tint definitions.
func (c *cardGrid) expandTinted(ctx ExpandContext, cell CardGridCell, idx int, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	fill := ctx.ResolveSurface("subtle", "lt1")
	if idx%2 == 1 {
		fill = ctx.ResolveSurface("paper", "lt2")
	}
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, fill)),
			Text:     buildCardGridDarkTextContent(cell.Header, headerSize, cell.Body, bodySize, accent),
		},
	}
}

// expandSoftCard: a single pale surface card with dark text and no visible border.
// The default surface is resolved from template metadata (subtle role → lt1
// fallback); callers paint a brand surface (e.g. "#FFF5ED") via the card_fill
// override. The border line is explicitly suppressed so no theme-default outline
// leaks through.
func (c *cardGrid) expandSoftCard(ctx ExpandContext, cell CardGridCell, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	fill := ctx.ResolveSurface("subtle", "lt1")
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, fill)),
			Line:     json.RawMessage(`"none"`),
			Text:     buildCardGridDarkTextContent(cell.Header, headerSize, cell.Body, bodySize, accent),
		},
	}
}

// applyCardGridSurfaceOverrides applies the generic card_fill / border overrides
// onto an already-styled card shape. It is a no-op when no surface overrides are
// set, preserving each style's native fill and border behavior.
func applyCardGridSurfaceOverrides(gc *jsonschema.GridCellInput, ovr *CardGridOverrides, accent string) {
	if gc == nil || gc.Shape == nil || ovr == nil {
		return
	}
	if ovr.CardFill != "" {
		gc.Shape.Fill = json.RawMessage(fmt.Sprintf("%q", ovr.CardFill))
	}
	if line := buildCardGridLineOverride(ovr, accent); line != nil {
		gc.Shape.Line = line
	}
}

// buildCardGridLineOverride resolves the card border from the line_color/line_width
// or border overrides, returning nil when neither is set (style default preserved).
func buildCardGridLineOverride(ovr *CardGridOverrides, accent string) json.RawMessage {
	// Explicit color/width take precedence over the border keyword.
	if ovr.LineColor != "" || ovr.LineWidth > 0 {
		color := ovr.LineColor
		if color == "" {
			color = "dk1"
		}
		width := ovr.LineWidth
		if width <= 0 {
			width = 1
		}
		return json.RawMessage(fmt.Sprintf(`{"color":%q,"width":%s}`, color, formatFloat(width)))
	}
	switch ovr.Border {
	case "none":
		return json.RawMessage(`"none"`)
	case "subtle":
		return json.RawMessage(`{"color":"dk1","width":0.5}`)
	case "accent":
		return json.RawMessage(fmt.Sprintf(`{"color":%q,"width":1}`, accent))
	}
	return nil
}

// formatFloat renders a float without a trailing ".0" so widths stay compact in
// the emitted JSON (e.g. 1 → "1", 0.75 → "0.75").
func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", f), "0"), ".")
}

// validateCardGridOverrides checks card-grid-specific override enums.
func validateCardGridOverrides(name string, overrides any) []error {
	if overrides == nil {
		return nil
	}
	ovr, ok := overrides.(*CardGridOverrides)
	if !ok {
		return nil
	}
	var errs []error
	if ovr.Style != "" && !validCardGridStyles[ovr.Style] {
		errs = append(errs, &ValidationError{
			Pattern: name,
			Path:    "overrides.style",
			Code:    "invalid_enum",
			Message: fmt.Sprintf("card-grid: overrides.style must be one of filled, accent-stripe, numbered-badge, icon-card, tinted, soft-card; got %q", ovr.Style),
		})
	}
	if ovr.CardFill != "" && !isCardGridColor(ovr.CardFill) {
		errs = append(errs, &ValidationError{
			Pattern: name,
			Path:    "overrides.card_fill",
			Code:    "invalid_color",
			Message: fmt.Sprintf("card-grid: overrides.card_fill must be a hex color (e.g. \"#FFF5ED\") or scheme color name; got %q", ovr.CardFill),
		})
	}
	if ovr.LineColor != "" && !isCardGridColor(ovr.LineColor) {
		errs = append(errs, &ValidationError{
			Pattern: name,
			Path:    "overrides.line_color",
			Code:    "invalid_color",
			Message: fmt.Sprintf("card-grid: overrides.line_color must be a hex color (e.g. \"#888888\") or scheme color name; got %q", ovr.LineColor),
		})
	}
	if ovr.LineWidth < 0 || ovr.LineWidth > 12 {
		errs = append(errs, &ValidationError{
			Pattern: name,
			Path:    "overrides.line_width",
			Code:    "out_of_range",
			Message: fmt.Sprintf("card-grid: overrides.line_width must be between 0 and 12 points; got %g", ovr.LineWidth),
		})
	}
	if ovr.Border != "" && !validCardGridBorders[ovr.Border] {
		errs = append(errs, &ValidationError{
			Pattern: name,
			Path:    "overrides.border",
			Code:    "invalid_enum",
			Message: fmt.Sprintf("card-grid: overrides.border must be one of none, subtle, accent; got %q", ovr.Border),
		})
	}
	if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// isCardGridColor reports whether s is an accepted color value for the card-grid
// surface overrides: a scheme color name (accent1, lt1, dk1, …) or a 3-/6-digit
// hex string (with or without a leading "#").
func isCardGridColor(s string) bool {
	if s == "" {
		return false
	}
	if pptx.SchemeColorNames[s] {
		return true
	}
	hex := strings.TrimPrefix(s, "#")
	if len(hex) != 3 && len(hex) != 6 {
		return false
	}
	for _, ch := range hex {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Text content builders
// ---------------------------------------------------------------------------

type cardParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type cardTextObj struct {
	Paragraphs    []cardParagraph `json:"paragraphs"`
	Align         string          `json:"align"`
	VerticalAlign string          `json:"vertical_align"`
}

func marshalTextObj(obj cardTextObj) json.RawMessage {
	data, _ := json.Marshal(obj)
	return data
}

// buildCardGridTextContent creates a JSON text object with header + body paragraphs
// using light text (for dark/accent backgrounds).
func buildCardGridTextContent(header string, headerSize float64, body string, bodySize float64) json.RawMessage {
	return marshalTextObj(cardTextObj{
		Paragraphs: []cardParagraph{
			{Content: header, Size: headerSize, Bold: true, Color: "lt1", Align: "l"},
			{Content: pptx.ConvertMarkdownEmphasis(body), Size: bodySize, Color: "lt1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "t",
	})
}

// buildCardGridDarkTextContent creates text for light backgrounds: accent-colored header, dk1 body.
func buildCardGridDarkTextContent(header string, headerSize float64, body string, bodySize float64, accent string) json.RawMessage {
	return marshalTextObj(cardTextObj{
		Paragraphs: []cardParagraph{
			{Content: header, Size: headerSize, Bold: true, Color: accent, Align: "l"},
			{Content: pptx.ConvertMarkdownEmphasis(body), Size: bodySize, Color: "dk1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "t",
	})
}

// buildNumberedBadgeTextContent renders a large number badge, header, and body.
func buildNumberedBadgeTextContent(badge, header string, headerSize float64, body string, bodySize float64, accent string) json.RawMessage {
	badgeSize := headerSize * 1.5
	if badgeSize > 36 {
		badgeSize = 36
	}
	return marshalTextObj(cardTextObj{
		Paragraphs: []cardParagraph{
			{Content: badge, Size: badgeSize, Bold: true, Color: accent, Align: "l"},
			{Content: header, Size: headerSize, Bold: true, Color: "dk1", Align: "l"},
			{Content: pptx.ConvertMarkdownEmphasis(body), Size: bodySize, Color: "dk1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "t",
	})
}

// extractNumberPrefix extracts a leading "N. " or "N) " prefix from header text.
// If no prefix is found, the fallback number is used (1-based cell index).
func extractNumberPrefix(header string, fallback int) (badge string, remainder string) {
	// Try "1. Header" or "1) Header" patterns
	for i, ch := range header {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if i > 0 && (ch == '.' || ch == ')') {
			rest := header[i+1:]
			rest = strings.TrimLeft(rest, " ")
			if rest != "" {
				return header[:i], rest
			}
		}
		break
	}
	return fmt.Sprintf("%d", fallback), header
}

// cellsLookLikeKPIs returns true when every cell's header looks like a short
// metric value (e.g. "$4.2M", "127%", "12d"). The heuristic is: header ≤ 8
// chars and contains at least one digit.
func cellsLookLikeKPIs(cells []CardGridCell) bool {
	for _, cell := range cells {
		if !looksLikeMetric(cell.Header) {
			return false
		}
	}
	return true
}

// looksLikeMetric returns true when s resembles a KPI big number: short (≤ 8
// chars), starts with a digit or currency/sign prefix followed by a digit, and
// digits outnumber letters. Examples: "$4.2M", "127%", "12d", "99.9%", "+15%".
func looksLikeMetric(s string) bool {
	if len(s) == 0 || len(s) > 8 {
		return false
	}
	runes := []rune(s)
	// First rune must be a digit or a common metric prefix ($, €, £, +, -, ~).
	first := runes[0]
	metricPrefix := first == '$' || first == '€' || first == '£' ||
		first == '+' || first == '-' || first == '~'
	if !unicode.IsDigit(first) && !metricPrefix {
		return false
	}
	// Count digits vs letters — metrics are digit-heavy.
	var digits, letters int
	for _, ch := range runes {
		if unicode.IsDigit(ch) {
			digits++
		} else if unicode.IsLetter(ch) {
			letters++
		}
	}
	return digits > 0 && digits >= letters
}
