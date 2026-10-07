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
// quote-cluster pattern — structured 3-col grid of stakeholder quotes
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&quoteCluster{})
}

type quoteCluster struct{}

func (q *quoteCluster) Name() string { return "quote-cluster" }
func (q *quoteCluster) Description() string {
	return "Structured 3-column grid of 3–8 attributed stakeholder quotes: open quotes under a quote mark, speech bubbles or tiles as styles, one optionally highlighted"
}
func (q *quoteCluster) UseWhen() string {
	return "Voice-of-customer or stakeholder research slide showing 3–8 short quotes from different people; prefer pull-quote when only one quote is the focal point, card-grid for non-quote text cards"
}
func (q *quoteCluster) NotWhen() string {
	return "Single quote (use pull-quote), more than 8 quotes (split across slides), or items are non-quote feature cards (use card-grid)"
}
func (q *quoteCluster) Version() int      { return 1 }
func (q *quoteCluster) CellsHint() string { return "3-8" }
func (q *quoteCluster) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"evidence", "frame"},
		PairsWith:     []string{"stat-hero", "kpi-3up", "card-grid"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (q *quoteCluster) ExemplarValues() any {
	return &QuoteClusterValues{
		Quotes: []QuoteClusterItem{
			{Text: "The new platform cut our cycle time in half.", Name: "J. Lin", Title: "Head of Operations"},
			{Text: "Our analysts finally trust the numbers they see.", Name: "P. Reyes", Title: "Director of Finance"},
			{Text: "Adoption was easier than any tool we have rolled out.", Name: "K. Müller", Title: "Chief Technology Officer"},
			{Text: "We are catching issues weeks earlier than before.", Name: "S. Patel", Title: "VP Customer Success"},
			{Text: "Reporting that used to take days is now a click.", Name: "M. Tanaka", Title: "Regional Controller"},
			{Text: "The team's data fluency has stepped up across the board.", Name: "A. Okafor", Title: "Chief Data Officer"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// QuoteClusterItem is one quote: quote text + attribution (name, title).
type QuoteClusterItem struct {
	Text  string `json:"text"`            // The quote text (italic)
	Name  string `json:"name"`            // Speaker's name (bold)
	Title string `json:"title,omitempty"` // Optional role / title rendered after the name
	// Highlight emphasises this quote (at most one): it is then the only
	// accent-coloured element of the cluster.
	Highlight bool `json:"highlight,omitempty"`
}

// QuoteClusterValues holds the cluster of quote bubbles (3–8 quotes).
type QuoteClusterValues struct {
	Quotes []QuoteClusterItem `json:"quotes"`
}

// QuoteClusterOverrides controls accents and per-zone font sizes.
type QuoteClusterOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	QuoteSize      float64 `json:"quote_size,omitempty"` // Default: open 14 (12 when dense), bubble / tile 10
	NameSize       float64 `json:"name_size,omitempty"`  // Default: open 12, bubble / tile 9
	TitleSize      float64 `json:"title_size,omitempty"` // Default 8
	// Style is "open" (default: unfilled quotes under a quote mark, the
	// attribution beneath), "bubble" (speech-bubble shapes with the
	// attribution under the tail) or "tile" (the look before
	// go-slide-creator-5cie9: tinted tiles with accent names).
	Style string `json:"style,omitempty"`
}

// quoteClusterStyles are the accepted overrides.style values.
var quoteClusterStyles = []string{"open", "bubble", "tile"}

// QuoteClusterCellOverride is the shared per-cell override, indexed by quote.
type QuoteClusterCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	quoteClusterMinQuotes = 3
	quoteClusterMaxQuotes = 8
	quoteClusterColumns   = 3
	quoteClusterTextMax   = 240
	quoteClusterNameMax   = 60
	quoteClusterTitleMax  = 80
)

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (q *quoteCluster) NewValues() any       { return &QuoteClusterValues{} }
func (q *quoteCluster) NewOverrides() any    { return &QuoteClusterOverrides{} }
func (q *quoteCluster) NewCellOverride() any { return &QuoteClusterCellOverride{} }

func (q *quoteCluster) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*QuoteClusterValues)
	if !ok || v == nil || len(v.Quotes) <= 3 {
		return nil
	}
	height, ok := q.budgetHeight(ctx, v, overrides)
	if !ok {
		return nil
	}
	// Reference payloads measured against the written size on every shipped
	// template with the uniform 0.5 cm shape text margin
	// (go-slide-creator-n1muf): with 4-6 quotes, 161 text characters beside
	// maximal attributions; with 7-8, 81 beside a 20-character name and a
	// 27-character title (maximal attributions leave no readable quote).
	targets := []QuoteClusterItem{
		{Text: quoteClusterBudgetCopy(161), Name: quoteClusterBudgetCopy(60), Title: quoteClusterBudgetCopy(80)},
	}
	if len(v.Quotes) > 6 {
		targets = []QuoteClusterItem{
			{Text: quoteClusterBudgetCopy(81), Name: quoteClusterBudgetCopy(20), Title: quoteClusterBudgetCopy(27)},
		}
	}
	limit := 0.0
	for _, target := range targets {
		reference := &QuoteClusterValues{Quotes: make([]QuoteClusterItem, len(v.Quotes))}
		for i := range reference.Quotes {
			reference.Quotes[i] = target
		}
		referenceHeight, ok := q.budgetHeight(ctx, reference, overrides)
		if !ok {
			return nil
		}
		limit = max(limit, referenceHeight)
	}
	if height <= limit {
		return nil
	}
	return []string{fmt.Sprintf("%s: quote-cluster quotes.text/name/title exceed the measured three-row copy budget; with 4–6 quotes keep quote text near 161 characters beside maximal attributions; with 7–8, near 81 beside a name of about 20 and a title of about 27 characters — shorten copy or split the quotes across slides", ErrCodeBodyTooLong)}
}

// budgetHeight is the height the copy budget is measured in: the cluster's
// rows at its densest setting. The open style sets short copy larger, so its
// height at the setting it chose says nothing about the copy it could hold.
func (q *quoteCluster) budgetHeight(ctx ExpandContext, v *QuoteClusterValues, overrides any) (float64, bool) {
	ovr, _ := overrides.(*QuoteClusterOverrides)
	if ovr == nil {
		ovr = &QuoteClusterOverrides{}
	}
	if ovr.Style == "" || ovr.Style == "open" {
		sc := quoteClusterOpenNoMark
		sc.quote = ResolveSize(ovr.QuoteSize, sc.quote)
		sc.attribution = ResolveSize(ovr.NameSize, sc.attribution)
		return quoteClusterHeight(q.expandOpenAt(ctx, v, nil, "accent1", false, sc)), true
	}
	grid, err := q.Expand(ctx, v, overrides, nil)
	if err != nil {
		return 0, false
	}
	return quoteClusterHeight(grid), true
}

func quoteClusterBudgetCopy(length int) string {
	return strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
}

func quoteClusterHeight(grid *jsonschema.ShapeGridInput) float64 {
	height := float64(len(grid.Rows)-1) * grid.RowGap
	for _, row := range grid.Rows {
		height += row.MaxHeight
	}
	return height
}

func (q *quoteCluster) Schema() *Schema {
	quoteSchema := ObjectSchema(
		map[string]*Schema{
			"text":      StringSchema(quoteClusterTextMax).WithDescription("Quote text (italic; 14pt in the open style, 12pt when the cluster is dense); with 4-6 quotes about 161 characters beside maximal names/titles; with 7-8 quotes about 81 beside a name of about 20 and a title of about 27 characters"),
			"name":      StringSchema(quoteClusterNameMax).WithDescription("Speaker name (bold)"),
			"title":     StringSchema(quoteClusterTitleMax).WithDescription("Optional role or title rendered next to the name"),
			"highlight": BooleanSchema().WithDescription("Emphasise this quote (at most one); it is then the only accent-coloured element"),
		},
		[]string{"text", "name"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"quotes": ArraySchema(quoteSchema, quoteClusterMinQuotes, quoteClusterMaxQuotes).WithDescription("Stakeholder quotes (3–8). Quotes auto-flow into a 3-column grid; the last row left-aligns when not full."),
		},
		[]string{"quotes"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color for the quote marks and the highlighted quote (default accent1)").WithDefault("accent1"),
			"style":           EnumSchema(quoteClusterStyles...).WithDescription("open (default: unfilled quotes under a quote mark, attribution beneath), bubble (speech-bubble shapes, attribution under the tail) or tile (tinted tiles with accent names)").WithDefault("open"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"quote_size":      NumberSchema(6, 40).WithDescription("Font size for quote text in points (default: open 14, stepping to 12 when the quotes need the height; bubble / tile 10)"),
			"name_size":       NumberSchema(6, 40).WithDescription("Font size for the attribution in points (default: open 12; bubble / tile 9)"),
			"title_size":      NumberSchema(6, 40).WithDescription("Font size for speaker title in points (default 8)"),
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
	}).WithDescription("Structured 3-column grid of attributed stakeholder quotes: open by default, speech bubbles or tiles as styles")
}

func (q *quoteCluster) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*QuoteClusterValues)
	if !ok || v == nil {
		return fmt.Errorf("quote-cluster: values must be *QuoteClusterValues, got %T", values)
	}

	const name = "quote-cluster"
	var errs []error

	if ovr, ok := overrides.(*QuoteClusterOverrides); ok && ovr != nil && ovr.Style != "" && !slices.Contains(quoteClusterStyles, ovr.Style) {
		errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, quoteClusterStyles))
	}
	errs = append(errs, singleHighlightErrors(name, "quote", "quotes", len(v.Quotes),
		func(i int) bool { return v.Quotes[i].Highlight },
		func(i int) string { return fmt.Sprintf("quotes[%d].highlight", i) })...)

	if len(v.Quotes) < quoteClusterMinQuotes {
		errs = append(errs, errMinItems(name, "quotes", quoteClusterMinQuotes, len(v.Quotes),
			"(hint: use `pull-quote` for a single quote)"))
	}
	if len(v.Quotes) > quoteClusterMaxQuotes {
		errs = append(errs, errMaxItems(name, "quotes", quoteClusterMaxQuotes, len(v.Quotes),
			"(hint: split the quotes across two slides)"))
	}

	for i, qt := range v.Quotes {
		textPath := fmt.Sprintf("quotes[%d].text", i)
		if strings.TrimSpace(qt.Text) == "" {
			errs = append(errs, errRequired(name, textPath))
		} else if runeLen(qt.Text) > quoteClusterTextMax {
			errs = append(errs, errMaxLength(name, textPath, quoteClusterTextMax, runeLen(qt.Text)))
		}
		namePath := fmt.Sprintf("quotes[%d].name", i)
		if strings.TrimSpace(qt.Name) == "" {
			errs = append(errs, errRequired(name, namePath))
		} else if runeLen(qt.Name) > quoteClusterNameMax {
			errs = append(errs, errMaxLength(name, namePath, quoteClusterNameMax, runeLen(qt.Name)))
		}
		if runeLen(qt.Title) > quoteClusterTitleMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("quotes[%d].title", i), quoteClusterTitleMax, runeLen(qt.Title)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(v.Quotes), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (q *quoteCluster) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*QuoteClusterValues)
	if !ok {
		return nil, fmt.Errorf("quote-cluster: values must be *QuoteClusterValues, got %T", values)
	}
	ovr := &QuoteClusterOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*QuoteClusterOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("quote-cluster: overrides must be *QuoteClusterOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	quoteSize := ResolveSize(ovr.QuoteSize, scaleCaptionPt)
	nameSize := ResolveSize(ovr.NameSize, sizeDenseCaptionPt)
	titleSize := ResolveSize(ovr.TitleSize, sizeBadgePt)

	style := ovr.Style
	if style == "" {
		style = "open"
	}
	anyHighlight := false
	for _, qt := range v.Quotes {
		anyHighlight = anyHighlight || qt.Highlight
	}
	sizes := quoteClusterSizes{quote: quoteSize, name: nameSize, title: titleSize}
	if style == "open" {
		return q.expandOpen(ctx, v, ovr, cellOverrides, accent, anyHighlight), nil
	}

	// Lay out quotes left-to-right into 3-column rows. Short final rows stay
	// left-aligned: any unused right-hand columns become empty filler cells so
	// the grid keeps a consistent column count.
	// A bubble and the attribution under it are two shapes, each with its
	// own text margin: three rows of them do not fit a content area, so 7–8
	// bubbles take four columns and stay on two rows.
	columns := quoteClusterColumns
	if style == "bubble" && len(v.Quotes) > 2*quoteClusterColumns {
		columns = quoteClusterBubbleWideColumns
	}
	totalRows := (len(v.Quotes) + columns - 1) / columns
	rows := make([]jsonschema.GridRowInput, 0, totalRows)

	for r := 0; r < totalRows; r++ {
		cells := make([]*jsonschema.GridCellInput, columns)
		var quotes []QuoteClusterItem
		for c := 0; c < columns; c++ {
			idx := r*columns + c
			if idx >= len(v.Quotes) {
				cells[c] = buildQuoteClusterEmptyCell()
				continue
			}
			qt := v.Quotes[idx]
			quotes = append(quotes, qt)
			var cell *jsonschema.GridCellInput
			if style == "tile" {
				cell = quoteClusterTileCell(ctx, qt, r, sizes, accent)
			} else {
				// The bubble and its attribution are placed once the row
				// height is known; the override target is the bubble text.
				cell = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Text: buildQuoteClusterBubbleQuote(qt, quoteSize, "dk1")}}
			}
			if co, coOk := cellOverrides[idx]; coOk {
				if cellOvr, ok2 := co.(*QuoteClusterCellOverride); ok2 {
					applyCellTextOverride(cell, cellOvr)
					if cellOvr.AccentBar {
						cell.AccentBar = &jsonschema.AccentBarInput{
							Position: "left",
							Color:    accent,
							Width:    3,
						}
					}
				}
			}
			cells[c] = cell
		}
		switch style {
		case "tile":
			// Size the row to its tallest quote and centre the short ones. Without
			// this the rows stretch to fill the content area and every quote is as
			// tall as the longest one in its row, so "It just works." sat in the
			// top fifth of a tall tinted box (go-slide-creator-pr3g).
			rows = append(rows, contentSizedRow(ctx, cells, quoteClusterColumns))
		default:
			rows = append(rows, quoteClusterBubbleRow(ctx, cells, quotes, r, columns, sizes, accent))
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, columns)),
		Gap:           ctx.Gap(contentSizedRowGapPt),
		RowGap:        ctx.Gap(10),
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}
	return grid, nil
}

// quoteClusterSizes are the three type sizes of a quote.
type quoteClusterSizes struct{ quote, name, title float64 }

// quoteClusterColWidthPt is the width of one of columns quote columns.
func quoteClusterColWidthPt(ctx ExpandContext, columns int) float64 {
	// The conservative area: a column measured wider than it renders would
	// size its row below the written fit of its text.
	areaW, _ := sizingAreaPt(ctx)
	return equalColumnWidthPt(areaW, columns, ctx.Gap(contentSizedRowGapPt))
}

// quoteClusterTileCell is the tile style: a tinted rounded tile holding the
// quote in quotation marks, the name in the accent and the title.
func quoteClusterTileCell(ctx ExpandContext, qt QuoteClusterItem, row int, sizes quoteClusterSizes, accent string) *jsonschema.GridCellInput {
	// Rows alternate two neutral steps (4% / 8%), never outlined
	// white beside filled grey (go-slide-creator-pgdkp).
	fillA, fillB := surfacePairJSON(ctx)
	fill := fillA
	if row%2 == 1 {
		fill = fillB
	}
	cell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     fill,
			Line:     noLine,
			Text:     buildQuoteClusterBubbleText(qt, sizes.quote, sizes.name, sizes.title, accent),
		},
	}
	if qt.Highlight {
		tone, ink := accentFillAndInk(ctx, fillTone{Color: accent}, 4.5)
		cell.Shape.Fill = tone.fillJSON()
		cell.Shape.Text = buildQuoteClusterInkText(qt, sizes, ink)
	}
	return cell
}

// Open style (go-slide-creator-5cie9, -rxdkf): a quote is text on the slide
// under a large opening quote mark, and its attribution stands on its own
// line under a short rule — no tile. A row of quotes is two grid rows (the
// quote, then the rule and the attribution), so the marks of a row share a
// line at the top and its attributions a line at the bottom however long
// each quote runs.
const (
	// quoteClusterMark is the opening mark set above each open quote.
	quoteClusterMark = "“"
	// quoteClusterMarkFont is the theme's major (heading) font: the mark is
	// display type, a serif on a serif-headed template.
	quoteClusterMarkFont = "+mj-lt"
	// quoteClusterMutedMarkAlpha dims the marks of the quotes that are not
	// highlighted, so the highlighted quote keeps the only accent.
	quoteClusterMutedMarkAlpha = 35.0
	// quoteClusterOpenRule is the short rule an attribution stands under: a
	// line of its own holding four underscores, the one stroke every face
	// joins into a line. It is type, not a drawn shape, so it starts exactly
	// where the attribution under it does in every column (the engine moves
	// unfilled first-column text onto the title's text line, and only text)
	// and grows with it.
	quoteClusterOpenRule = "____"
	// The text margins of an open quote: above the mark, between the quote
	// and the rule, and under the attribution. They are also the padding of
	// the highlighted quote's tint panel.
	quoteClusterOpenTopPt        = 6.0
	quoteClusterOpenQuoteGapPt   = 4.0
	quoteClusterOpenAttrBottomPt = 6.0
	// quoteClusterOpenBandGapPt is the whitespace between two rows of quotes.
	quoteClusterOpenBandGapPt = 14.0
	// The densest setting (no mark line) gives the height back to the
	// quotes: no rule line, the margins and the band at these values.
	quoteClusterDenseMarginPt  = 3.0
	quoteClusterDenseBandGapPt = 6.0
	// quoteClusterOpenRowGapPt joins a quote and its attribution, and
	// quoteClusterOpenPanelLapPt is how far the highlighted attribution's
	// tint laps over its quote's: the panel is two cells and must read as
	// one, without a seam where they meet.
	quoteClusterOpenRowGapPt   = 0.01
	quoteClusterOpenPanelLapPt = 0.5
)

// quoteClusterOpenScale is one type setting of the open cluster.
type quoteClusterOpenScale struct{ mark, quote, attribution float64 }

// quoteClusterOpenScales are the settings an open cluster is tried at,
// largest first: a 36pt mark over 14pt quotes, then the mark and the quote
// give way a step at a time. The first the content area holds is taken. Past
// the last the mark gives its line back and the quote is set in quotation
// marks instead (quoteClusterOpenNoMark), so an open cluster never holds less
// copy than the tiles it replaced.
var quoteClusterOpenScales = []quoteClusterOpenScale{
	{sizeQuotePt, scaleSubheadPt, scaleBodyPt},
	{scaleDisplayPt, scaleSubheadPt, scaleBodyPt},
	{scaleDisplayPt, scaleBodyPt, scaleBodyPt},
	{scaleLeadPt, scaleBodyPt, scaleBodyPt},
}

// quoteClusterOpenNoMark is the densest open setting: no mark line.
var quoteClusterOpenNoMark = quoteClusterOpenScale{quote: scaleBodyPt, attribution: scaleBodyPt}

// expandOpen lays the cluster out in the open style at the largest of
// quoteClusterOpenScales its content area holds; authored sizes stand.
func (q *quoteCluster) expandOpen(ctx ExpandContext, v *QuoteClusterValues, ovr *QuoteClusterOverrides, cellOverrides map[int]any, accent string, anyHighlight bool) *jsonschema.ShapeGridInput {
	_, areaH := sizingAreaPt(ctx)
	for _, sc := range append(append([]quoteClusterOpenScale{}, quoteClusterOpenScales...), quoteClusterOpenNoMark) {
		sc.quote = ResolveSize(ovr.QuoteSize, sc.quote)
		sc.attribution = ResolveSize(ovr.NameSize, sc.attribution)
		grid := q.expandOpenAt(ctx, v, cellOverrides, accent, anyHighlight, sc)
		if sc.mark <= 0 || quoteClusterHeight(grid) <= areaH {
			return grid
		}
	}
	return nil
}

// expandOpenAt lays the open cluster out at one type setting.
func (q *quoteCluster) expandOpenAt(ctx ExpandContext, v *QuoteClusterValues, cellOverrides map[int]any, accent string, anyHighlight bool, sc quoteClusterOpenScale) *jsonschema.ShapeGridInput {
	columns := quoteClusterColumns
	colW := quoteClusterColWidthPt(ctx, columns)
	fonts := ctx.themeFonts()
	none := json.RawMessage(`"none"`)
	totalRows := (len(v.Quotes) + columns - 1) / columns
	var rows []jsonschema.GridRowInput
	for r := 0; r < totalRows; r++ {
		quotes := make([]*jsonschema.GridCellInput, columns)
		attrs := make([]*jsonschema.GridCellInput, columns)
		quoteH, attrH := 0.0, 0.0
		for c := 0; c < columns; c++ {
			idx := r*columns + c
			if idx >= len(v.Quotes) {
				quotes[c], attrs[c] = buildQuoteClusterEmptyCell(), buildQuoteClusterEmptyCell()
				continue
			}
			qt := v.Quotes[idx]
			fill, surface, ink := none, fillTone{Color: "lt1"}, "dk1"
			if qt.Highlight {
				surface = inactiveTintTone(accent)
				fill = surface.fillJSON()
				ink = readableTextOn(ctx, surface, "dk1")
			}
			quote := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect", Fill: fill, Line: noLine,
				Text: quoteClusterOpenQuoteText(ctx, qt, sc, accent, surface, ink, anyHighlight),
			}}
			attr := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect", Fill: fill, Line: noLine,
				Text: quoteClusterOpenAttributionText(qt, sc, ink),
			}}
			if qt.Highlight {
				attr.BleedTop = quoteClusterOpenPanelLapPt
			}
			if co, ok := cellOverrides[idx].(*QuoteClusterCellOverride); ok {
				applyCellTextOverride(quote, co)
				if co.AccentBar {
					quote.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 3}
				}
			}
			quoteH = math.Max(quoteH, rowTextNeedPt(fonts, quote.Shape.Text, colW))
			attrH = math.Max(attrH, rowTextNeedPt(fonts, attr.Shape.Text, colW))
			quotes[c], attrs[c] = quote, attr
		}

		if r > 0 {
			band := ctx.Gap(quoteClusterOpenBandGapPt)
			if sc.mark <= 0 {
				band = ctx.Gap(quoteClusterDenseBandGapPt)
			}
			rows = append(rows, jsonschema.GridRowInput{
				Cells:     []*jsonschema.GridCellInput{{ColSpan: columns}},
				MinHeight: band, MaxHeight: band,
			})
		}
		rows = append(rows,
			jsonschema.GridRowInput{Cells: quotes, MaxHeight: math.Ceil(quoteH)},
			jsonschema.GridRowInput{Cells: attrs, MaxHeight: math.Ceil(attrH)},
		)
	}
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, columns)),
		ColGap:        ctx.Gap(contentSizedRowGapPt),
		RowGap:        quoteClusterOpenRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}
}

// quoteClusterOpenQuoteText is the mark and the quote of one open quote,
// top-anchored so the marks of a row share a line. Every mark takes the
// accent unless one quote is highlighted: then only that quote keeps it (on
// its accent tint panel) and the other marks go neutral.
func quoteClusterOpenQuoteText(ctx ExpandContext, qt QuoteClusterItem, sc quoteClusterOpenScale, accent string, surface fillTone, ink string, anyHighlight bool) json.RawMessage {
	quote := qt.Text
	if sc.mark <= 0 {
		// No room for the mark's own line: the quotation marks move into
		// the text.
		quote = "“" + qt.Text + "”"
	}
	paras := []quoteClusterParagraph{{Content: quote, Size: sc.quote, Italic: true, Color: ink, Align: "l"}}
	if sc.mark > 0 {
		mark := quoteClusterParagraph{Content: quoteClusterMark, Size: sc.mark, Bold: true, Align: "l", Font: quoteClusterMarkFont, Figure: true}
		if anyHighlight && !qt.Highlight {
			mark.Color, mark.Alpha = "dk1", readableDimAlpha(ctx, "dk1", quoteClusterMutedMarkAlpha, 4.5)
		} else {
			mark.Color = inkOnFill(ctx, accent, surface, 3.0)
		}
		paras = append([]quoteClusterParagraph{mark}, paras...)
	}
	data, _ := json.Marshal(quoteClusterTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
	if sc.mark <= 0 {
		return withTextInsetSides(withTextInsetSides(data, quoteClusterDenseMarginPt, "inset_top"), 0.01, "inset_bottom")
	}
	return withTextInsetSides(withTextInsetSides(data, quoteClusterOpenTopPt, "inset_top"), quoteClusterOpenQuoteGapPt, "inset_bottom")
}

// quoteClusterOpenAttributionText is the rule and the attribution under it:
// the bold name, then the title, on one line where they fit. The densest
// setting has no rule line.
func quoteClusterOpenAttributionText(qt QuoteClusterItem, sc quoteClusterOpenScale, ink string) json.RawMessage {
	size := sc.attribution
	attribution := "<b>" + qt.Name + "</b>"
	if title := strings.TrimSpace(qt.Title); title != "" {
		attribution += ", " + title
	}
	paras := []quoteClusterParagraph{{Content: attribution, Size: size, Color: ink, Align: "l"}}
	bottom := quoteClusterDenseMarginPt
	if sc.mark > 0 {
		paras = append([]quoteClusterParagraph{{Content: quoteClusterOpenRule, Size: size, Color: ink, Align: "l"}}, paras...)
		bottom = quoteClusterOpenAttrBottomPt
	}
	data, _ := json.Marshal(quoteClusterTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
	return withTextInsetSides(withTextInsetSides(data, 0.01, "inset_top"), bottom, "inset_bottom")
}

// Bubble style: a speech bubble holds the quote and its tail points at the
// attribution set beneath it.
const (
	// quoteClusterBubbleWideColumns is the column count for 7–8 bubbles.
	quoteClusterBubbleWideColumns = 4
	// quoteClusterTailPt is the length of the bubble's tail below its body.
	quoteClusterTailPt = 10.0
	// quoteClusterTailX places the tail tip left of the bubble's centre, as a
	// share of its width (preset adj1 units of 1/100000).
	quoteClusterTailX = -30000
)

// quoteClusterBubbleRow turns the row's placeholder cells into bubble +
// attribution stacks. Every bubble in the row takes the tallest quote's
// height so the attributions share a line.
func quoteClusterBubbleRow(ctx ExpandContext, cells []*jsonschema.GridCellInput, quotes []QuoteClusterItem, row, columns int, sizes quoteClusterSizes, accent string) jsonschema.GridRowInput {
	// Each stack is a nested grid, which resolves inside its cell by
	// SubGridInsetPt on every side.
	colW := quoteClusterColWidthPt(ctx, columns) - 2*SubGridInsetPt
	fonts := ctx.themeFonts()
	bubblePt, attrPt := 0.0, 0.0
	for i, qt := range quotes {
		bubblePt = math.Max(bubblePt, rowTextNeedPt(fonts, cells[i].Shape.Text, colW))
		attrPt = math.Max(attrPt, rowTextNeedPt(fonts, buildQuoteClusterAttribution(qt, sizes, "dk1"), colW))
	}
	bubblePt, attrPt = math.Ceil(bubblePt), math.Ceil(attrPt)
	fillA, fillB := surfacePairJSON(ctx)
	if row%2 == 1 {
		fillA = fillB
	}
	for i, qt := range quotes {
		fill := fillA
		text := cells[i].Shape.Text
		if qt.Highlight {
			tone, ink := accentFillAndInk(ctx, fillTone{Color: accent}, 4.5)
			fill = tone.fillJSON()
			text = recolorTextInk(text, "dk1", ink)
		}
		bubble := &jsonschema.GridCellInput{
			AccentBar: cells[i].AccentBar,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "wedgeRoundRectCallout",
				Fill:     fill,
				Line:     noLine,
				Text:     text,
				Adjustments: map[string]int64{
					"adj1": quoteClusterTailX,
					"adj2": 50000 + int64(math.Round(quoteClusterTailPt/bubblePt*100000)),
					"adj3": 16667,
				},
			},
		}
		attribution := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     noLine,
			Text:     buildQuoteClusterAttribution(qt, sizes, "dk1"),
		}}
		cells[i] = &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(`1`),
			ColGap:  0.01,
			RowGap:  quoteClusterTailPt,
			Rows: []jsonschema.GridRowInput{
				{MinHeight: bubblePt, MaxHeight: bubblePt, Cells: []*jsonschema.GridCellInput{bubble}},
				{MinHeight: attrPt, MaxHeight: attrPt, Cells: []*jsonschema.GridCellInput{attribution}},
			},
		}}
	}
	return jsonschema.GridRowInput{Cells: cells, MaxHeight: bubblePt + quoteClusterTailPt + attrPt + 2*SubGridInsetPt}
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type quoteClusterParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Italic  bool    `json:"italic,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
	// Alpha is the text opacity in percent (0 = opaque).
	Alpha      float64 `json:"alpha,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
	// Font is a typeface or theme-font reference ("+mj-lt"); empty is the
	// template's body font.
	Font string `json:"font,omitempty"`
	// Figure marks display type that keeps its size off the word steps.
	Figure bool `json:"figure,omitempty"`
}

type quoteClusterTextObj struct {
	Paragraphs    []quoteClusterParagraph `json:"paragraphs"`
	Align         string                  `json:"align"`
	VerticalAlign string                  `json:"vertical_align"`
}

func buildQuoteClusterBubbleText(qt QuoteClusterItem, quoteSize, nameSize, titleSize float64, accent string) json.RawMessage {
	paras := []quoteClusterParagraph{
		{Content: "“" + qt.Text + "”", Size: quoteSize, Italic: true, Color: "dk1", Align: "l"},
		{Content: qt.Name, Size: nameSize, Bold: true, Color: accent, Align: "l"},
	}
	if strings.TrimSpace(qt.Title) != "" {
		paras = append(paras, quoteClusterParagraph{
			Content: qt.Title, Size: titleSize, Color: "dk2", Align: "l",
		})
	}
	obj := quoteClusterTextObj{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	data, _ := json.Marshal(obj)
	return data
}

// buildQuoteClusterInkText is the tile text in one ink, for a highlighted
// tile on its accent fill.
func buildQuoteClusterInkText(qt QuoteClusterItem, sizes quoteClusterSizes, ink string) json.RawMessage {
	return recolorTextInk(recolorTextInk(buildQuoteClusterBubbleText(qt, sizes.quote, sizes.name, sizes.title, ink), "dk1", ink), "dk2", ink)
}

// buildQuoteClusterBubbleQuote is the quote inside a speech bubble.
func buildQuoteClusterBubbleQuote(qt QuoteClusterItem, quoteSize float64, ink string) json.RawMessage {
	data, _ := json.Marshal(quoteClusterTextObj{
		Paragraphs:    []quoteClusterParagraph{{Content: "“" + qt.Text + "”", Size: quoteSize, Italic: true, Color: ink, Align: "l"}},
		Align:         "l",
		VerticalAlign: "ctr",
	})
	return data
}

// buildQuoteClusterAttribution is the attribution set under a bubble: the bold
// name, then the title, on one line where they fit.
func buildQuoteClusterAttribution(qt QuoteClusterItem, sizes quoteClusterSizes, ink string) json.RawMessage {
	attribution := "<b>" + qt.Name + "</b>"
	if title := strings.TrimSpace(qt.Title); title != "" {
		attribution += ", " + title
	}
	paras := []quoteClusterParagraph{{Content: attribution, Size: sizes.name, Color: ink, Align: "l"}}
	data, _ := json.Marshal(quoteClusterTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
	return data
}

func buildQuoteClusterEmptyCell() *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
		},
	}
}
