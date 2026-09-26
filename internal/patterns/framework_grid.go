package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// framework-grid pattern — 2-6 dimension rows, each a bold row label on a
// tinted band followed by 1-4 small cards (accent title + short body).
//
//   [ People      ][ Leadership ][ Skills     ][ Incentives ]
//   [ Process     ][ Governance ][ Workflows  ]
//   [ Technology  ][ Platforms  ][ Data       ][ Tooling    ]
//
// The column count is the longest row's card count; shorter rows leave the
// trailing space empty so a card always sits under the same column heading.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&frameworkGrid{})
}

type frameworkGrid struct{}

const (
	fgName = "framework-grid"

	fgMinRows  = 2
	fgMaxRows  = 6
	fgMinCards = 1
	fgMaxCards = 4

	fgLabelMax = 40
	fgTitleMax = 40
	fgBodyMax  = 160

	fgDefaultLabelPct = 18.0
	fgMinLabelPct     = 10.0
	fgMaxLabelPct     = 35.0
	fgColGapPt        = 8.0
	fgRowGapPt        = 8.0
	fgCardPadPt       = 10.0
	fgMinRowPt        = 40.0
	fgFillFrac        = 0.80 // share of the content height the block aims for
	fgMaxStretch      = 1.8  // rows stop growing past this multiple of their content
)

func (p *frameworkGrid) Name() string { return fgName }
func (p *frameworkGrid) Description() string {
	return "Framework grid: 2-6 dimension rows, each a bold row label on a tinted band followed by 1-4 small cards (accent title + short body); shorter rows leave trailing space empty"
}
func (p *frameworkGrid) UseWhen() string {
	return "A framework whose rows are named dimensions (people / process / technology, levers by dimension, change-management building blocks) and whose cells are 1-4 short titled levers per dimension that need not line up as the same criteria; prefer card-grid for a flat set of tiles without row labels, table-highlight for options scored against shared criteria, stylish-panels for 3-5 pillars with bullet lists"
}
func (p *frameworkGrid) NotWhen() string {
	return "Cards form a flat catalog with no dimension labels (use card-grid), every row answers the same scored criteria (use table-highlight), items are rated on a scale (use capability-heatmap), each row is a sequence of phases (use process-grid-2row), items sit on two axes (use matrix-2x2), a single ordered chain of steps (use value-chain), or each group needs bullet lists (use stylish-panels)"
}
func (p *frameworkGrid) Version() int      { return 1 }
func (p *frameworkGrid) CellsHint() string { return "(2-6 rows) × (label + 1-4 cards)" }
func (p *frameworkGrid) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"phase-roadmap", "kpi-3up", "exec-summary"},
		DensityClass:       "high",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (p *frameworkGrid) SupportsInlineMarkdown() bool { return true }

func (p *frameworkGrid) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 4, Rows: 3},
		{Columns: 5, Rows: 4},
		{Columns: 5, Rows: 6},
	}
}

func (p *frameworkGrid) ExemplarValues() any {
	return &FrameworkGridValues{Rows: []FrameworkGridRow{
		{Label: "People", Cards: []FrameworkGridCard{
			{Title: "Leadership", Body: "Visible sponsor at every level"},
			{Title: "Skills", Body: "Role-based academies and coaching"},
			{Title: "Incentives", Body: "Goals tied to adoption"},
		}},
		{Label: "Process", Cards: []FrameworkGridCard{
			{Title: "Governance", Body: "Single change board, monthly cadence"},
			{Title: "Ways of working", Body: "Redesigned end-to-end workflows"},
		}},
		{Label: "Technology", Cards: []FrameworkGridCard{
			{Title: "Platforms", Body: "Shared data and AI platform"},
			{Title: "Tooling", Body: "Copilots in daily tools"},
			{Title: "Data", Body: "Owned, trusted, reusable"},
		}},
	}}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// FrameworkGridCard is one lever: a short title and an optional body.
type FrameworkGridCard struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// FrameworkGridRow is one dimension: a row label and its cards.
type FrameworkGridRow struct {
	Label string              `json:"label"`
	Cards []FrameworkGridCard `json:"cards"`
}

// FrameworkGridValues holds the dimension rows top to bottom.
type FrameworkGridValues struct {
	Rows []FrameworkGridRow `json:"rows"`
}

// FrameworkGridOverrides are the pattern-level overrides.
type FrameworkGridOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	LabelWidthPct  float64 `json:"label_width_pct,omitempty"`
	TitleSize      float64 `json:"title_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"` // uniform | alternate | progressive
}

// FrameworkGridCellOverride is the shared per-cell override, indexed row by
// row: each row's label, then its cards.
type FrameworkGridCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *frameworkGrid) NewValues() any       { return &FrameworkGridValues{} }
func (p *frameworkGrid) NewOverrides() any    { return &FrameworkGridOverrides{} }
func (p *frameworkGrid) NewCellOverride() any { return &FrameworkGridCellOverride{} }

func (p *frameworkGrid) Schema() *Schema {
	cardSchema := ObjectSchema(map[string]*Schema{
		"title": StringSchema(fgTitleMax).WithDescription("Lever name, bold in the accent colour"),
		"body":  StringSchema(fgBodyMax).WithDescription("Optional one-line explanation; at 4 cards per row keep it near 60 characters"),
	}, []string{"title"}).WithAdditionalProperties(false)

	rowSchema := ObjectSchema(map[string]*Schema{
		"label": StringSchema(fgLabelMax).WithDescription("Dimension name shown bold in the left band"),
		"cards": ArraySchema(cardSchema, fgMinCards, fgMaxCards).WithDescription("1-4 cards; the longest row sets the column count and shorter rows leave trailing space empty"),
	}, []string{"label", "cards"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"rows": ArraySchema(rowSchema, fgMinRows, fgMaxRows).WithDescription("Dimension rows top to bottom (2-6)"),
	}, []string{"rows"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":           StringSchema(0).WithDescription("Accent scheme color for card titles and tints (default accent1)").WithDefault("accent1"),
		"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"label_width_pct":  NumberSchema(fgMinLabelPct, fgMaxLabelPct).WithDescription("Row-label column width as a percent of the grid (default 18)"),
		"title_size":       NumberSchema(6, 120).WithDescription("Card title and row label size in points (default 14)"),
		"body_size":        NumberSchema(6, 120).WithDescription("Card body size in points (default 12)"),
		"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-column accent variation: uniform (default), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":         valuesSchema,
		"overrides":      overridesSchema,
		"cell_overrides": CellOverridesSchema("cellOverride"),
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Framework grid: 2-6 labelled dimension rows of 1-4 titled lever cards")
}

func (p *frameworkGrid) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*FrameworkGridValues)
	if !ok || vals == nil {
		return fmt.Errorf("%s: values must be *FrameworkGridValues, got %T", fgName, values)
	}
	var errs []error

	if ovr, ok := overrides.(*FrameworkGridOverrides); ok && ovr != nil {
		if err := ValidateCellAccentMode(fgName, ovr.CellAccentMode); err != nil {
			errs = append(errs, err)
		}
		if ovr.LabelWidthPct != 0 && (ovr.LabelWidthPct < fgMinLabelPct || ovr.LabelWidthPct > fgMaxLabelPct) {
			errs = append(errs, newValidationError(fgName, "overrides.label_width_pct", ErrCodeOutOfRange,
				fmt.Sprintf("%s: overrides.label_width_pct must be %.0f–%.0f, got %g", fgName, fgMinLabelPct, fgMaxLabelPct, ovr.LabelWidthPct),
				ReplaceValueFix("overrides.label_width_pct", int(fgMinLabelPct), int(fgMaxLabelPct))))
		}
	}

	if len(vals.Rows) < fgMinRows {
		errs = append(errs, errMinItems(fgName, "rows", fgMinRows, len(vals.Rows), "(hint: a single row of cards is a card-grid)"))
	}
	if len(vals.Rows) > fgMaxRows {
		errs = append(errs, errMaxItems(fgName, "rows", fgMaxRows, len(vals.Rows), "(hint: merge dimensions or split the framework across two slides)"))
	}
	total := 0
	for i, row := range vals.Rows {
		errs = appendRequiredMax(errs, fgName, fmt.Sprintf("rows[%d].label", i), row.Label, fgLabelMax, true)
		cardsPath := fmt.Sprintf("rows[%d].cards", i)
		if len(row.Cards) < fgMinCards {
			errs = append(errs, errMinItems(fgName, cardsPath, fgMinCards, len(row.Cards), ""))
		}
		if len(row.Cards) > fgMaxCards {
			errs = append(errs, errMaxItems(fgName, cardsPath, fgMaxCards, len(row.Cards), "(hint: group levers or add a row)"))
		}
		total += 1 + len(row.Cards)
		for j, card := range row.Cards {
			errs = appendRequiredMax(errs, fgName, fmt.Sprintf("%s[%d].title", cardsPath, j), card.Title, fgTitleMax, true)
			errs = appendRequiredMax(errs, fgName, fmt.Sprintf("%s[%d].body", cardsPath, j), card.Body, fgBodyMax, false)
		}
	}

	if coErr := validateCellOverrideKeys(fgName, cellOverrides, total, "(indices: row by row — each row's label, then its cards)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Geometry
// ---------------------------------------------------------------------------

type fgLayout struct {
	cols       int
	labelPct   float64
	labelTextW float64
	cardTextW  float64
	titlePt    float64
	bodyPt     float64
	contentHPt float64 // tallest row's content-sized height
	rowHPt     float64 // row height after the fill
	neededHPt  float64
	areaHPt    float64
	tallestRow int
}

func fgResolveOverrides(overrides any) *FrameworkGridOverrides {
	if ovr, ok := overrides.(*FrameworkGridOverrides); ok && ovr != nil {
		return ovr
	}
	return &FrameworkGridOverrides{}
}

func fgMeasure(ctx ExpandContext, v *FrameworkGridValues, ovr *FrameworkGridOverrides) fgLayout {
	font := ctx.Theme.BodyFont
	contentW, contentH := contentAreaPt(ctx)
	l := fgLayout{areaHPt: contentH}
	for _, row := range v.Rows {
		l.cols = max(l.cols, len(row.Cards))
	}
	l.cols = max(l.cols, 1)
	l.labelPct = fgDefaultLabelPct
	if ovr.LabelWidthPct > 0 {
		l.labelPct = clampPt(ovr.LabelWidthPct, fgMinLabelPct, fgMaxLabelPct)
	}
	gridW := contentW - fgColGapPt*float64(l.cols)
	l.labelTextW = math.Max(gridW*l.labelPct/100-2*defaultShapeInsetLRPt, 1)
	cardW := gridW * (100 - l.labelPct) / 100 / float64(l.cols)
	l.cardTextW = math.Max(cardW-2*defaultShapeInsetLRPt, 1)
	l.titlePt = shapegrid.EffectiveTextSizePt(ResolveSize(ovr.TitleSize, 14))
	l.bodyPt = shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, 12))

	for i, row := range v.Rows {
		h := textBlockHeightPt(font, l.labelTextW, textParagraph{text: inlineMarkupRe.ReplaceAllString(row.Label, ""), size: l.titlePt, bold: true})
		for _, card := range row.Cards {
			h = math.Max(h, textBlockHeightPt(font, l.cardTextW,
				textParagraph{text: inlineMarkupRe.ReplaceAllString(card.Title, ""), size: l.titlePt, bold: true},
				textParagraph{text: inlineMarkupRe.ReplaceAllString(card.Body, ""), size: l.bodyPt}))
		}
		h = math.Round(math.Max(h+2*defaultShapeInsetTBPt+2*fgCardPadPt, fgMinRowPt))
		if h > l.contentHPt {
			l.contentHPt, l.tallestRow = h, i
		}
	}
	n := float64(len(v.Rows))
	gaps := fgRowGapPt * math.Max(n-1, 0)
	l.neededHPt = gaps + n*l.contentHPt

	// Every row takes the tallest row's height so the grid reads as a grid,
	// then grows toward the fill target without turning a short lever into a
	// tall empty tile.
	_, sizingH := sizingAreaPt(ctx)
	l.rowHPt = l.contentHPt
	if n > 0 {
		per := (sizingH*fgFillFrac - gaps) / n
		l.rowHPt = math.Round(clampPt(per, l.contentHPt, l.contentHPt*fgMaxStretch))
	}
	return l
}

// ---------------------------------------------------------------------------
// Fills and ink
// ---------------------------------------------------------------------------

// fgLabelTone is the row-label band: a light tint of the accent, one step
// deeper than the cards so the band reads as the row's heading.
func fgLabelTone(accent string) fillTone {
	return inactiveTintTone(accent)
}

// fgCardTone is the card surface: a pale wash of the column's accent.
func fgCardTone(accent string) fillTone {
	return paleAccentTone(accent)
}

// fgTitleInk keeps the card title in the accent when it reads on the card,
// and falls back to the measured theme ink otherwise — a pale accent on its
// own pale wash is invisible. The bar is the one the generator's contrast
// pass applies to the whole text body: it judges a shape by its SMALLEST run,
// so a title above a 12pt body needs 4.5:1, and only a title-only card at
// 14pt bold or more gets the 3:1 large-text bar. Picking a lower bar here
// just hands the choice to the render-time fixer, which swaps in a literal.
func fgTitleInk(ctx ExpandContext, accent string, card fillTone, sizePt float64, hasBody bool) string {
	minRatio := svggen.WCAGAANormal
	if !hasBody && sizePt >= 14 {
		minRatio = svggen.WCAGAALarge
	}
	fill, ok := effectiveFillColor(ctx, card)
	if !ok {
		return accent
	}
	if c, cok := resolveThemeColor(ctx, accent); cok && c.ContrastWith(fill) >= minRatio {
		return accent
	}
	return readableTextOn(ctx, card, "dk1")
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *frameworkGrid) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*FrameworkGridValues)
	if !ok || vals == nil {
		return nil, fmt.Errorf("%s: values must be *FrameworkGridValues, got %T", fgName, values)
	}
	if overrides != nil {
		if _, ok := overrides.(*FrameworkGridOverrides); !ok {
			return nil, fmt.Errorf("%s: overrides must be *FrameworkGridOverrides, got %T", fgName, overrides)
		}
	}
	if len(vals.Rows) == 0 {
		return nil, fmt.Errorf("%s: at least one row is required", fgName)
	}
	ovr := fgResolveOverrides(overrides)
	base := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	l := fgMeasure(ctx, vals, ovr)

	labelTone := fgLabelTone(base)
	labelInk := readableTextOn(ctx, labelTone, "dk2")

	rows := make([]jsonschema.GridRowInput, 0, len(vals.Rows))
	idx := 0
	insetTop := math.Round(defaultShapeInsetTBPt + fgCardPadPt + (l.rowHPt-l.contentHPt)/2)
	for _, row := range vals.Rows {
		labelText := patternTextObj{
			Paragraphs:    []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(row.Label), Size: l.titlePt, Bold: true, Color: labelInk, Align: "l"}},
			Align:         "l",
			VerticalAlign: "ctr",
		}
		// Label cell, then one cell per card column.
		label := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     labelTone.fillJSON(),
			Line:     json.RawMessage(`"none"`),
			Text:     labelText.json(),
		}}
		fgApplyCellOverride(label, cellOverrides, idx, base)
		cells := []*jsonschema.GridCellInput{label}
		idx++

		for j := 0; j < l.cols; j++ {
			if j >= len(row.Cards) {
				cells = append(cells, &jsonschema.GridCellInput{})
				continue
			}
			card := row.Cards[j]
			accent := ctx.ResolveCellAccent(base, j, ovr.CellAccentMode)
			tone := fgCardTone(accent)
			body := strings.TrimSpace(card.Body)
			paras := []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(card.Title), Size: l.titlePt, Bold: true, Color: fgTitleInk(ctx, accent, tone, l.titlePt, body != ""), Align: "l", SpaceAfter: 2}}
			if body != "" {
				paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(body), Size: l.bodyPt, Color: readableTextOn(ctx, tone, "dk1"), Align: "l"})
			}
			// Every card is top-anchored at the same inset, so titles line up
			// across the row; the inset centres the row's tallest card, which
			// splits the stretch evenly above and below it.
			text := patternTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "t", InsetTop: insetTop}.json()
			cell := &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     tone.fillJSON(),
					Line:     json.RawMessage(`"none"`),
					Text:     text,
				},
			}
			fgApplyCellOverride(cell, cellOverrides, idx, accent)
			cells = append(cells, cell)
			idx++
		}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: l.rowHPt, MaxHeight: l.rowHPt, Cells: cells})
	}

	cols := []float64{l.labelPct}
	for j := 0; j < l.cols; j++ {
		cols = append(cols, math.Round((100-l.labelPct)/float64(l.cols)*100)/100)
	}
	colsJSON, _ := json.Marshal(cols)
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        fgColGapPt,
		RowGap:        fgRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// fgApplyCellOverride applies the D15 per-cell override (accent_bar).
func fgApplyCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, ok := co.(*FrameworkGridCellOverride)
	if !ok {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
	}
}

// ---------------------------------------------------------------------------
// Post-expand warnings
// ---------------------------------------------------------------------------

// PostExpandWarnings reports a framework whose rows, each as tall as its
// tallest card, need more height than the content area has — every card
// then autofits smaller.
func (p *frameworkGrid) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*FrameworkGridValues)
	if !ok || v == nil || len(v.Rows) == 0 {
		return nil
	}
	l := fgMeasure(ctx, v, fgResolveOverrides(overrides))
	if l.areaHPt <= 0 || l.neededHPt <= l.areaHPt {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s: framework-grid rows[%d] sets a %.0fpt row height and every row takes it; %d rows need about %.0fpt but the content area holds %.0fpt, so every card shrinks — shorten the card bodies in rows[%d], drop a row, or split the framework",
		ErrCodeBodyTooLong, l.tallestRow, l.contentHPt, len(v.Rows), l.neededHPt, l.areaHPt, l.tallestRow)}
}
