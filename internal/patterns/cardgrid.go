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
func (c *cardGrid) Version() int { return 1 }
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
type CardGridCell struct {
	Header string `json:"header"`
	Body   string `json:"body"`
	Icon   string `json:"icon,omitempty"` // Emoji or short glyph for icon-card style
}

// UnmarshalJSON supports string shorthand "Header | Body" or object {header, body}.
func (c *CardGridCell) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("CardGridCell string must be \"Header | Body\", got %q", s)
		}
		c.Header = parts[0]
		c.Body = parts[1]
		return nil
	}
	type alias CardGridCell
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("CardGridCell must be string \"Header | Body\" or {header, body}: %w", err)
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

// CardGridOverrides extends TextOverrides with a Style field for visual variants.
type CardGridOverrides struct {
	TextOverrides
	Style string `json:"style,omitempty"` // "filled" (default), "accent-stripe", "numbered-badge", "icon-card", "tinted"
}

// validCardGridStyles enumerates the allowed style values.
var validCardGridStyles = map[string]bool{
	"":               true, // default = filled
	"filled":         true,
	"accent-stripe":  true,
	"numbered-badge": true,
	"icon-card":      true,
	"tinted":         true,
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
				"header": StringSchema(80).WithDescription("Card header/title"),
				"body":   StringSchema(300).WithDescription("Card body content"),
				"icon":   StringSchema(20).WithDescription("Emoji or short glyph (used with icon-card style)"),
			},
			[]string{"header", "body"},
		).WithAdditionalProperties(false),
	).WithDescription("Card cell: string \"Header | Body\" or {header, body, icon?}")

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
			"style":            EnumSchema("filled", "accent-stripe", "numbered-badge", "icon-card", "tinted").WithDescription("Visual style: filled (default solid accent cards), accent-stripe (left accent bar on light cards), numbered-badge (circled number badges), icon-card (emoji/glyph badge), tinted (alternating lt1/lt2 backgrounds)").WithDefault("filled"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
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
			gc := c.expandCell(ctx, cell, cellIdx, style, accent, headerSize, bodySize)

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
func (c *cardGrid) expandCell(ctx ExpandContext, cell CardGridCell, idx int, style, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
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
	default: // "filled"
		gc = c.expandFilled(cell, accent, headerSize, bodySize)
	}
	// Add SVG icon overlay when a bundled icon name is provided.
	if cell.Icon != "" && gc.Shape != nil && gc.Shape.Icon == nil {
		gc.Shape.Icon = &jsonschema.IconInput{
			Name:     cell.Icon,
			Fill:     accent,
			Position: "top",
		}
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

// expandIconCard: emoji/glyph badge above header text on light card with top accent bar.
func (c *cardGrid) expandIconCard(cell CardGridCell, accent string, headerSize, bodySize float64) *jsonschema.GridCellInput {
	icon := cell.Icon
	if icon == "" {
		icon = "\u2022" // bullet as fallback
	}
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     buildIconCardTextContent(icon, cell.Header, headerSize, cell.Body, bodySize, accent),
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
			Message: fmt.Sprintf("card-grid: overrides.style must be one of filled, accent-stripe, numbered-badge, icon-card, tinted; got %q", ovr.Style),
		})
	}
	if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
		errs = append(errs, err)
	}
	return errs
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

// buildIconCardTextContent renders an icon glyph, header, and body.
func buildIconCardTextContent(icon, header string, headerSize float64, body string, bodySize float64, accent string) json.RawMessage {
	iconSize := headerSize * 1.5
	if iconSize > 36 {
		iconSize = 36
	}
	return marshalTextObj(cardTextObj{
		Paragraphs: []cardParagraph{
			{Content: icon, Size: iconSize, Bold: false, Color: accent, Align: "l"},
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
