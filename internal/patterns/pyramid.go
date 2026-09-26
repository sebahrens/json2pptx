package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// pyramid pattern — 3-5 stacked trapezoids representing a hierarchy
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&pyramid{})
}

type pyramid struct{}

func (p *pyramid) Name() string        { return "pyramid" }
func (p *pyramid) Description() string { return "Stacked trapezoid hierarchy (3-5 tiers)" }
func (p *pyramid) UseWhen() string {
	return "Hierarchy that narrows visually top-to-bottom (3-5 tiers, Maslow-style); prefer arch-stack when layers are equal-width technology tiers, process-flow when tiers are sequential"
}
func (p *pyramid) NotWhen() string {
	return "Layers are equal-width technology tiers (use arch-stack), layers are sequential steps (use process-flow), or more than 5 levels needed (use card-grid)"
}
func (p *pyramid) Version() int      { return 1 }
func (p *pyramid) CellsHint() string { return "3-5" }
func (p *pyramid) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"card-grid", "kpi-3up", "icon-row"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (p *pyramid) SupportsCallout() bool        { return true }
func (p *pyramid) SupportsInlineMarkdown() bool { return true }

func (p *pyramid) ExemplarValues() any {
	return &PyramidValues{
		Tiers: []string{"Strategy", "Tactics", "Operations", "Execution", "Measurement"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PyramidValues holds the tier labels for the pyramid pattern.
type PyramidValues struct {
	Tiers []string `json:"tiers"` // Top to bottom (narrowest to widest)
}

// PyramidOverrides is the standard text overrides. header_size is not
// supported (tier labels are body text); cell_accent_mode colours each tier.
type PyramidOverrides = TextOverrides

// PyramidCellOverride is the shared per-cell override.
type PyramidCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *pyramid) NewValues() any       { return &PyramidValues{} }
func (p *pyramid) NewOverrides() any    { return &PyramidOverrides{} }
func (p *pyramid) NewCellOverride() any { return &PyramidCellOverride{} }

func (p *pyramid) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*PyramidValues)
	if !ok || v == nil || len(v.Tiers) < 4 {
		return nil
	}
	budget := 116
	if len(v.Tiers) >= 5 {
		budget = 70
	}
	longest := 0
	for _, word := range strings.Fields(v.Tiers[0]) {
		longest = max(longest, runeLen(word))
	}
	if longest <= budget {
		return nil
	}
	return []string{fmt.Sprintf("%s: pyramid tiers[0] contains a %d-character unbroken word; the top tier of a %d-tier pyramid holds about %d wide characters — add word breaks, shorten the label, or use fewer tiers", ErrCodeBodyTooLong, longest, len(v.Tiers), budget)}
}

func (p *pyramid) Schema() *Schema {
	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"tiers": ArraySchema(StringSchema(120), 3, 5).WithDescription("Tier labels, top (narrowest) to bottom (widest); top tier with 4/5 tiers holds about 116/70 wide unbroken characters; add word breaks for longer copy"),
		},
		[]string{"tiers"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchemaWithout("header_size"),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Stacked trapezoid hierarchy (3-5 tiers)")
}

func (p *pyramid) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*PyramidValues)
	if !ok || vals == nil {
		return fmt.Errorf("pyramid: values must be *PyramidValues, got %T", values)
	}

	const name = "pyramid"
	var errs []error

	if ovr, ok := overrides.(*PyramidOverrides); ok && ovr != nil {
		if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
			errs = append(errs, err)
		}
		errs = append(errs, rejectUnusedTextOverrides(name, ovr, "header_size")...)
	}

	if len(vals.Tiers) < 3 {
		errs = append(errs, errMinItems(name, "tiers", 3, len(vals.Tiers), ""))
	}
	if len(vals.Tiers) > 5 {
		errs = append(errs, errMaxItems(name, "tiers", 5, len(vals.Tiers), ""))
	}

	for i, tier := range vals.Tiers {
		path := fmt.Sprintf("tiers[%d]", i)
		if tier == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(tier) > 120 {
			errs = append(errs, errMaxLength(name, path, 120, runeLen(tier)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Tiers), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (p *pyramid) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*PyramidValues)
	if !ok {
		return nil, fmt.Errorf("pyramid: values must be *PyramidValues, got %T", values)
	}
	ovr := &PyramidOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*PyramidOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("pyramid: overrides must be *PyramidOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	bodySize := ResolveSize(ovr.BodySize, 14.0)
	n := len(vals.Tiers)
	if n == 0 {
		return nil, fmt.Errorf("pyramid: at least one tier is required")
	}

	// Geometry: a symmetric column grid [side×(n-1), centre, side×(n-1)].
	// Tier i (0 = top) starts at column n-1-i and spans 2i+1 columns, so each
	// tier is exactly one side-column wider on each edge than the tier above
	// it — widths grow linearly top→bottom and the bottom tier spans the full
	// grid. The top tier keeps pyramidTopWidthPct of the width regardless of
	// tier count.
	cols := pyramidColumns(n)
	colsJSON, _ := json.Marshal(cols)
	adj := pyramidTrapezoidAdj(ctx, n, cols[0])

	rows := make([]jsonschema.GridRowInput, 0, n)
	for i, tier := range vals.Tiers {
		// cell_accent_mode picks each tier's accent (go-slide-creator-s1uvj.41);
		// the text colour below is chosen against that tier's own fill.
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)

		// Gradient the fill: top tier uses the full accent, lower tiers
		// lighten. Text colour is picked per tier against the effective
		// (alpha-composited) fill so light bottom tiers get dark text.
		alpha := 100 - i*15
		tone := fillTone{Color: accent, Alpha: float64(alpha)}
		textColor := readableTextOn(ctx, tone, pyramidFallbackTextColor(alpha))
		fill := json.RawMessage(fmt.Sprintf(`{"color":"%s","alpha":%d}`, accent, alpha))

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "trapezoid",
			Fill:     fill,
			Text:     buildPyramidTextContent(tier, bodySize, textColor),
		}
		if adj > 0 {
			shape.Adjustments = map[string]int64{"adj": adj}
		}
		cell := &jsonschema.GridCellInput{
			ColSpan: 2*i + 1,
			Shape:   shape,
		}

		// Apply cell overrides
		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*PyramidCellOverride); ok2 {
				applyCellTextOverride(cell, cellOvr)
				if cellOvr.AccentBar {
					cell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    4,
					}
				}
			}
		}

		// Leading empty cells advance the column cursor to the tier's
		// first column; trailing columns are simply left unfilled.
		rowCells := make([]*jsonschema.GridCellInput, 0, n-i)
		for k := 0; k < n-1-i; k++ {
			rowCells = append(rowCells, &jsonschema.GridCellInput{})
		}
		rowCells = append(rowCells, cell)
		rows = append(rows, jsonschema.GridRowInput{Cells: rowCells})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  pyramidColGapPt,
		RowGap:  pyramidRowGapPt,
		Rows:    rows,
	}

	return grid, nil
}

const (
	// pyramidTopWidthPct is the share of the grid width the top tier spans.
	pyramidTopWidthPct = 34.0
	// pyramidColGapPt is a hairline column gap: the pyramid's side columns
	// are purely geometric, so real gaps would only distort the tier widths.
	pyramidColGapPt = 0.01
	// pyramidRowGapPt separates the stacked tiers.
	pyramidRowGapPt = 4.0
)

// pyramidColumns returns the 2n-1 column percentages for an n-tier pyramid.
func pyramidColumns(n int) []float64 {
	if n <= 1 {
		return []float64{100}
	}
	side := (100 - pyramidTopWidthPct) / float64(2*(n-1))
	var cols []float64
	for k := 0; k < n-1; k++ {
		cols = append(cols, side)
	}
	cols = append(cols, pyramidTopWidthPct)
	for k := 0; k < n-1; k++ {
		cols = append(cols, side)
	}
	return cols
}

// pyramidTrapezoidAdj returns the trapezoid "adj" value that makes each
// tier's slanted edges line up with the tiers above and below: the
// horizontal inset of each side must equal one side column. OOXML measures
// adj against min(width, height) of the shape, which for pyramid tiers is the
// row height. Returns 0 (keep the preset default) when the layout size is
// unknown, e.g. in unit tests without a template.
func pyramidTrapezoidAdj(ctx ExpandContext, n int, sidePct float64) int64 {
	w, h := expandContentSize(ctx)
	if w <= 0 || h <= 0 || n <= 1 {
		return 0
	}
	rowGapEMU := int64(pyramidRowGapPt * 12700)
	rowH := float64(h-int64(n-1)*rowGapEMU) / float64(n)
	if rowH <= 0 {
		return 0
	}
	inset := float64(w) * sidePct / 100
	return int64(inset / rowH * 100000)
}

// expandContentSize returns the best available estimate of the grid's
// content-area size in EMU: the explicit layout bounds when the caller set
// them, otherwise the shape-grid default bounds for the slide size (the
// generate path expands patterns before the final bounds are known).
func expandContentSize(ctx ExpandContext) (w, h int64) {
	if ctx.LayoutBounds.Width > 0 && ctx.LayoutBounds.Height > 0 {
		return ctx.LayoutBounds.Width, ctx.LayoutBounds.Height
	}
	if ctx.SlideWidth > 0 && ctx.SlideHeight > 0 {
		db := shapegrid.DefaultBounds(ctx.SlideWidth, ctx.SlideHeight)
		return db.CX, db.CY
	}
	return 0, 0
}

// pyramidFallbackTextColor is used only when the theme cannot be resolved:
// tiers at or above 70% accent opacity keep light text, lighter tiers flip
// to dark.
func pyramidFallbackTextColor(alpha int) string {
	if alpha >= 70 {
		return "lt1"
	}
	return "dk1"
}

func buildPyramidTextContent(content string, size float64, color string) json.RawMessage {
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
			{Content: content, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
