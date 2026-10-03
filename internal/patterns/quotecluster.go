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
	QuoteSize      float64 `json:"quote_size,omitempty"` // Default 10
	NameSize       float64 `json:"name_size,omitempty"`  // Default 9
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
	grid, err := q.Expand(ctx, values, overrides, nil)
	if err != nil {
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
		referenceGrid, err := q.Expand(ctx, reference, overrides, nil)
		if err != nil {
			return nil
		}
		limit = max(limit, quoteClusterHeight(referenceGrid))
	}
	if quoteClusterHeight(grid) <= limit {
		return nil
	}
	return []string{fmt.Sprintf("%s: quote-cluster quotes.text/name/title exceed the measured three-row copy budget; with 4–6 quotes keep quote text near 161 characters beside maximal attributions; with 7–8, near 81 beside a name of about 20 and a title of about 27 characters — shorten copy or split the quotes across slides", ErrCodeBodyTooLong)}
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
			"text":      StringSchema(quoteClusterTextMax).WithDescription("Quote text (italic, ~10pt); with 4-6 quotes about 161 characters beside maximal names/titles; with 7-8 quotes about 81 beside a name of about 20 and a title of about 27 characters"),
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
			"quote_size":      NumberSchema(6, 40).WithDescription("Font size for quote text in points (default 10)"),
			"name_size":       NumberSchema(6, 40).WithDescription("Font size for speaker name in points (default 9)"),
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
	// markSize stays zero (no mark line) outside the open style.
	var markSize float64
	if style == "open" {
		markSize = quoteClusterOpenMarkSize(ctx, v.Quotes, sizes)
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
			switch style {
			case "tile":
				cell = quoteClusterTileCell(ctx, qt, r, sizes, accent)
			case "bubble":
				// The bubble and its attribution are placed once the row
				// height is known; the override target is the bubble text.
				cell = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Text: buildQuoteClusterBubbleQuote(qt, quoteSize, "dk1")}}
			default:
				cell = quoteClusterOpenCell(ctx, qt, sizes, markSize, accent, anyHighlight)
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
		case "bubble":
			rows = append(rows, quoteClusterBubbleRow(ctx, cells, quotes, r, columns, sizes, accent))
		default:
			rows = append(rows, quoteClusterOpenRow(ctx, cells))
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

// Open style (go-slide-creator-5cie9): a quote is text on the slide under an
// opening quote mark, with its attribution directly beneath — no tile.
const (
	// quoteClusterMark is the opening mark set above each open quote.
	quoteClusterMark = "\u201C"
	// quoteClusterMutedMarkAlpha dims the marks of the quotes that are not
	// highlighted, so the highlighted quote keeps the only accent.
	quoteClusterMutedMarkAlpha = 35.0
)

// quoteClusterMarkSteps are the opening mark's sizes, largest first: display
// type, then the lead step. Past both, the mark gives its line back and the
// quote is set in quotation marks instead (quoteClusterOpenMarkSize returns 0).
var quoteClusterMarkSteps = []float64{scaleDisplayPt, scaleLeadPt}

// quoteClusterOpenMarkSize picks the largest mark the cluster has room for:
// the mark is the first thing to give way when the quotes need the height, so
// an open cluster never holds less copy than the tiles it replaced.
func quoteClusterOpenMarkSize(ctx ExpandContext, quotes []QuoteClusterItem, sizes quoteClusterSizes) float64 {
	_, areaH := sizingAreaPt(ctx)
	rows := (len(quotes) + quoteClusterColumns - 1) / quoteClusterColumns
	avail := areaH - float64(rows-1)*ctx.Gap(10)
	for _, size := range quoteClusterMarkSteps {
		total := 0.0
		for r := 0; r < rows; r++ {
			var cells []*jsonschema.GridCellInput
			for i := r * quoteClusterColumns; i < min((r+1)*quoteClusterColumns, len(quotes)); i++ {
				cells = append(cells, quoteClusterOpenCell(ctx, quotes[i], sizes, size, "accent1", false))
			}
			total += quoteClusterOpenRow(ctx, cells).MaxHeight
		}
		if total <= avail {
			return size
		}
	}
	return 0
}

// quoteClusterOpenCell is one open quote. Every mark takes the accent unless
// one quote is highlighted: then only that quote keeps it (on an accent tint
// band) and the other marks go neutral.
func quoteClusterOpenCell(ctx ExpandContext, qt QuoteClusterItem, sizes quoteClusterSizes, markSize float64, accent string, anyHighlight bool) *jsonschema.GridCellInput {
	fill := json.RawMessage(`"none"`)
	surface := fillTone{Color: "lt1"}
	if qt.Highlight {
		surface = inactiveTintTone(accent)
		fill = surface.fillJSON()
	}
	mark := quoteClusterParagraph{Content: quoteClusterMark, Size: markSize, Bold: true, Align: "l"}
	if anyHighlight && !qt.Highlight {
		mark.Color, mark.Alpha = "dk1", readableDimAlpha(ctx, "dk1", quoteClusterMutedMarkAlpha, 4.5)
	} else {
		mark.Color = inkOnFill(ctx, accent, surface, 3.0)
	}
	quote := qt.Text
	if markSize <= 0 {
		// No room for the mark's own line: the quotation marks move into
		// the text.
		quote = "“" + qt.Text + "”"
	}
	ink := "dk1"
	if qt.Highlight {
		ink = readableTextOn(ctx, surface, "dk1")
	}
	// The attribution is one line — bold name, then the title — so an open
	// quote is no taller than the tile it replaces.
	attribution := "<b>" + qt.Name + "</b>"
	if title := strings.TrimSpace(qt.Title); title != "" {
		attribution += ", " + title
	}
	paras := []quoteClusterParagraph{
		{Content: quote, Size: sizes.quote, Italic: true, Color: ink, Align: "l"},
		{Content: attribution, Size: sizes.name, Color: ink, Align: "l"},
	}
	if markSize > 0 {
		paras = append([]quoteClusterParagraph{mark}, paras...)
	}
	data, _ := json.Marshal(quoteClusterTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: fill, Line: noLine, Text: data}}
}

// quoteClusterOpenRow sizes an open row to the written fit of its tallest
// quote: no card padding, since there is no card.
func quoteClusterOpenRow(ctx ExpandContext, cells []*jsonschema.GridCellInput) jsonschema.GridRowInput {
	colW := quoteClusterColWidthPt(ctx, quoteClusterColumns)
	need := 0.0
	for _, c := range cells {
		if c != nil && c.Shape != nil && len(c.Shape.Text) > 0 {
			need = math.Max(need, rowTextNeedPt(ctx.themeFonts(), c.Shape.Text, colW))
		}
	}
	return jsonschema.GridRowInput{Cells: cells, MaxHeight: math.Ceil(need)}
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
