package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// agenda-with-images pattern — numbered accent numerals + title + image/quote
// per agenda row. Richer alternative to the plain `agenda` pattern.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&agendaWithImages{})
}

type agendaWithImages struct{}

func (a *agendaWithImages) Name() string { return "agenda-with-images" }
func (a *agendaWithImages) Description() string {
	return "Numbered agenda rows: 28pt serif accent numerals, bold titles with optional subtitles, one hairline rule between rows and a dashed image (or quote) placeholder per row"
}
func (a *agendaWithImages) UseWhen() string {
	return "Agenda or table-of-contents slide where each section needs a visual preview (image placeholder or pull quote) alongside the numbered title; prefer plain `agenda` when items are bare titles, card-grid when items need multi-line body text"
}
func (a *agendaWithImages) NotWhen() string {
	return "Items are bare titles only (use `agenda`), items are visual categories without numeric order (use `icon-row`), or items need dense multi-line bodies (use `card-grid`)"
}
func (a *agendaWithImages) Version() int      { return 1 }
func (a *agendaWithImages) CellsHint() string { return "3-6" }
func (a *agendaWithImages) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"open", "frame"},
		PairsWith:     []string{"scqa-summary", "stat-hero", "kpi-3up"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (a *agendaWithImages) ExemplarValues() any {
	return &AgendaWithImagesValues{
		// Four rows: the subtitle budget is zero at five or six rows, so a
		// five-row exemplar with subtitles was over its own budget and written
		// below the floor on the shorter content areas (go-slide-creator-k3eb3).
		Items: []AgendaWithImagesItem{
			{Title: "Executive Summary", Subtitle: "Situation, complication and our answer", ImageLabel: "Chart: Revenue trend"},
			{Title: "Market Analysis", Subtitle: "Size, growth and competitive position", ImageLabel: "Photo: Market scene"},
			{Title: "Strategic Options", Subtitle: "Three paths and our recommendation", ImageLabel: "Diagram: Option tree"},
			{Title: "Next Steps", Subtitle: "Decisions required from this meeting", ImageLabel: "Table: Decision log"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// AgendaWithImagesItem is a single agenda row.
type AgendaWithImagesItem struct {
	Number     int    `json:"number,omitempty"`      // 1-based ordinal. When 0, auto-assigned as i+1.
	Title      string `json:"title"`                 // Section title (bold)
	Subtitle   string `json:"subtitle,omitempty"`    // Optional descriptive subtitle
	ImageLabel string `json:"image_label,omitempty"` // Optional caption in the image placeholder; the column collapses only when NO item has one
}

// AgendaWithImagesValues holds the agenda rows (3-6 items).
type AgendaWithImagesValues struct {
	Items []AgendaWithImagesItem `json:"items"`
}

// AgendaWithImagesOverrides controls accent and typography.
type AgendaWithImagesOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	NumberSize     float64 `json:"number_size,omitempty"`      // Font size for the numeral (default 28; 18 in a solid badge)
	TitleSize      float64 `json:"title_size,omitempty"`       // Font size for row title (default 18, stepping to 14 and 12 when the rows need the height)
	SubtitleSize   float64 `json:"subtitle_size,omitempty"`    // Font size for subtitle (default 10)
	ImageLabelSize float64 `json:"image_label_size,omitempty"` // Font size for image placeholder caption (default 10)
	// Style is "numeral" (default: unfilled serif accent numerals standing on
	// the title's text line) or "solid" (accent-filled number squares with
	// white numerals; legacy look).
	Style string `json:"style,omitempty"`
}

// agendaWithImagesStyles are the accepted overrides.style values.
var agendaWithImagesStyles = []string{"numeral", "solid"}

// AgendaWithImagesCellOverride is the shared per-cell override, indexed by item.
type AgendaWithImagesCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (a *agendaWithImages) NewValues() any       { return &AgendaWithImagesValues{} }
func (a *agendaWithImages) NewOverrides() any    { return &AgendaWithImagesOverrides{} }
func (a *agendaWithImages) NewCellOverride() any { return &AgendaWithImagesCellOverride{} }

// The title and subtitle share one text cell. The budget was measured at
// default text sizes against the written size on every shipped template with
// the uniform 0.5 cm shape text margin (go-slide-creator-n1muf). Without image
// labels, the text cell expands into the image column. Five or six rows hold a
// title and no readable subtitle.
func agendaWithImagesSubtitleBudget(rows, titleChars int, withImages bool) int {
	switch {
	case rows >= 5:
		return 0
	case rows <= 3:
		return 160
	case !withImages:
		return 111
	case titleChars <= 65:
		return 65
	default:
		return 0
	}
}

// agendaWithImagesTitleBudget / agendaWithImagesLabelBudget are the readable
// title and image-label lengths at a row count with image labels, or 0 when
// the schema maximum holds (TestAgendaWithImagesBudgetProbe).
func agendaWithImagesTitleBudget(rows int, withImages bool) int {
	switch {
	case !withImages || rows < 5:
		return 0
	case rows == 5:
		return 65
	default:
		return 61
	}
}

func agendaWithImagesLabelBudget(rows int) int {
	switch {
	case rows < 5:
		return 0
	case rows == 5:
		return 41
	default:
		return 40
	}
}

func (a *agendaWithImages) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*AgendaWithImagesValues)
	if !ok || v == nil {
		return nil
	}
	warnings := agendaWithImagesBudgetWarnings(v)
	if len(warnings) > 0 || len(v.Items) == 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return warnings
	}
	// The budgets assume a typical content area; with the template's own area
	// the rows are measured against it (go-slide-creator-k3eb3).
	ovr, _ := overrides.(*AgendaWithImagesOverrides)
	if ovr == nil {
		ovr = &AgendaWithImagesOverrides{}
	}
	if _, _, total := agendaWithImagesFit(ctx, v, ovr); total > 0 {
		if _, areaH := sizingAreaPt(ctx); total > areaH+1 {
			warnings = append(warnings, fmt.Sprintf("%s: agenda-with-images rows need %.0fpt at readable sizes but the content area holds about %.0fpt — shorten titles or subtitles, omit subtitles, or use fewer rows", ErrCodeBodyTooLong, total, areaH))
		}
	}
	return warnings
}

// agendaWithImagesBudgetWarnings reports titles, subtitles and image labels
// longer than the measured character budgets.
func agendaWithImagesBudgetWarnings(v *AgendaWithImagesValues) []string {
	withImages := anyAgendaImageLabel(v.Items)
	var warnings []string
	for i, item := range v.Items {
		if tb := agendaWithImagesTitleBudget(len(v.Items), withImages); tb > 0 && runeLen(item.Title) > tb {
			warnings = append(warnings, fmt.Sprintf("%s: agenda-with-images items[%d].title is %d characters; a %d-row agenda with image labels holds about %d readable title characters — shorten the title or use fewer rows", ErrCodeBodyTooLong, i, runeLen(item.Title), len(v.Items), tb))
		}
		if lb := agendaWithImagesLabelBudget(len(v.Items)); lb > 0 && runeLen(item.ImageLabel) > lb {
			warnings = append(warnings, fmt.Sprintf("%s: agenda-with-images items[%d].image_label is %d characters; a %d-row agenda holds about %d readable image-label characters — shorten the label or use fewer rows", ErrCodeBodyTooLong, i, runeLen(item.ImageLabel), len(v.Items), lb))
		}
		budget := agendaWithImagesSubtitleBudget(len(v.Items), runeLen(item.Title), withImages)
		if n := runeLen(item.Subtitle); n > budget {
			if budget == 0 {
				warnings = append(warnings, fmt.Sprintf("%s: agenda-with-images items[%d].subtitle has %d characters; a %d-row agenda (image labels=%t, %d-character title) leaves no readable subtitle room — omit the subtitle, shorten the title, or use four rows or fewer", ErrCodeBodyTooLong, i, n, len(v.Items), withImages, runeLen(item.Title)))
			} else {
				warnings = append(warnings, fmt.Sprintf("%s: agenda-with-images items[%d].subtitle is %d characters; a %d-row agenda with image labels=%t and a %d-character title holds about %d subtitle characters before text shrinks below the readable minimum — shorten the subtitle or title, omit image labels, or use fewer rows", ErrCodeBodyTooLong, i, n, len(v.Items), withImages, runeLen(item.Title), budget))
			}
		}
	}
	return warnings
}

func (a *agendaWithImages) Schema() *Schema {
	itemSchema := ObjectSchema(
		map[string]*Schema{
			"number":      IntegerSchema(0, 999).WithDescription("1-based ordinal; auto-assigned 1..N when omitted (use 0 or omit to auto-assign)"),
			"title":       StringSchema(80).WithDescription("Section title (bold); with image labels, 5 rows hold about 65 characters and 6 rows about 61"),
			"subtitle":    StringSchema(160).WithDescription("Optional text below the title. About 160 readable characters with 3 rows; 4 rows hold about 111 without image labels and 65 with them (title up to 65); 5-6 rows hold no readable subtitle — omit it"),
			"image_label": StringSchema(60).WithDescription("Optional caption centred in the image placeholder (about 41 readable characters at 5 rows, 40 at 6); omit it on one row and that row still gets an empty placeholder, omit it on every row to collapse the image column"),
		},
		[]string{"title"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"items": ArraySchema(itemSchema, 3, 6).WithDescription("Agenda rows (3-6)"),
		},
		[]string{"items"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color for the numerals (default: the template's color_roles.primary_fill)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"number_size":      NumberSchema(6, 60).WithDescription("Font size for the numeral in points (default 28; 18 in a solid badge)"),
			"title_size":       NumberSchema(6, 60).WithDescription("Font size for row title in points (default 18, stepping to 14 and 12 when the rows need the height)"),
			"subtitle_size":    NumberSchema(6, 40).WithDescription("Font size for subtitle in points (default 10)"),
			"image_label_size": NumberSchema(6, 40).WithDescription("Font size for image placeholder caption in points (default 10)"),
			"style":            EnumSchema(agendaWithImagesStyles...).WithDescription("numeral (default): unfilled 28pt accent numerals in the heading font, so a column of agenda numbers does not read as a row of accent blocks. solid: accent-filled number squares with white numerals (legacy look)").WithDefault("numeral"),
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
	}).WithDescription("Numbered agenda rows with accent numerals (or solid number badges via overrides.style), bold titles, optional subtitles, hairline rules between rows and optional dashed image placeholders")
}

func (a *agendaWithImages) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*AgendaWithImagesValues)
	if !ok || v == nil {
		return fmt.Errorf("agenda-with-images: values must be *AgendaWithImagesValues, got %T", values)
	}

	const name = "agenda-with-images"
	var errs []error

	if len(v.Items) < 3 {
		errs = append(errs, errMinItems(name, "items", 3, len(v.Items), "(hint: use `agenda` for 2-item lists)"))
	}
	if len(v.Items) > 6 {
		errs = append(errs, errMaxItems(name, "items", 6, len(v.Items), "(hint: split the agenda across two slides or use plain `agenda` which supports up to 10)"))
	}

	for i, item := range v.Items {
		titlePath := fmt.Sprintf("items[%d].title", i)
		if strings.TrimSpace(item.Title) == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if runeLen(item.Title) > 80 {
			errs = append(errs, errMaxLength(name, titlePath, 80, runeLen(item.Title)))
		}
		if runeLen(item.Subtitle) > 160 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("items[%d].subtitle", i), 160, runeLen(item.Subtitle)))
		}
		if runeLen(item.ImageLabel) > 60 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("items[%d].image_label", i), 60, runeLen(item.ImageLabel)))
		}
		if item.Number < 0 {
			errs = append(errs, &ValidationError{
				Pattern: name,
				Path:    fmt.Sprintf("items[%d].number", i),
				Code:    "out_of_range",
				Message: fmt.Sprintf("agenda-with-images: items[%d].number must be >= 0 (0 = auto), got %d", i, item.Number),
			})
		}
	}

	if ovr, ok := overrides.(*AgendaWithImagesOverrides); ok && ovr != nil {
		if ovr.Style != "" && !slices.Contains(agendaWithImagesStyles, ovr.Style) {
			errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, agendaWithImagesStyles))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(v.Items), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (a *agendaWithImages) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*AgendaWithImagesValues)
	if !ok {
		return nil, fmt.Errorf("agenda-with-images: values must be *AgendaWithImagesValues, got %T", values)
	}
	ovr := &AgendaWithImagesOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*AgendaWithImagesOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("agenda-with-images: overrides must be *AgendaWithImagesOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	// An unfilled numeral is bold large text on the slide background: it
	// needs 3:1 against lt1, else it steps to dk2 / dk1.
	numeralInk := accentInkOnLight(ctx, accent, 3.0)
	solid := ovr.Style == "solid"
	numberSize := ResolveSize(ovr.NumberSize, agendaWithImagesNumberSize)
	if solid {
		numberSize = ResolveSize(ovr.NumberSize, scaleLeadPt)
	}
	subtitleSize := ResolveSize(ovr.SubtitleSize, scaleCaptionPt)
	imageLabelSize := ResolveSize(ovr.ImageLabelSize, scaleCaptionPt)

	// The image column is all-or-nothing: one row's label earns the column for
	// every row, and no labels at all mean no column (go-slide-creator-jodu).
	withImages := anyAgendaImageLabel(v.Items)

	// The badge is meant to be a numbered SQUARE, but it filled its cell, which
	// is 18% of the content width by whatever height the row took: 1.5:1 on a
	// three-item agenda and 2.4:1 on a five-item one, so it read as an accent
	// slab (go-slide-creator-tiarr). badgeWidthPct narrows it inside its cell
	// to the row's own height, centring the remainder.
	badgeWidthPct := agendaBadgeWidthPct(ctx, len(v.Items))
	_, areaH := sizingAreaPt(ctx)
	// A three-row semantic agenda used to occupy only 144pt of a roughly
	// 389pt content zone. Give the rows a substantial shared footprint while
	// retaining the 48pt readability floor for denser six-row agendas.
	rowCount := float64(len(v.Items))
	minRowPt := (areaH*agendaWithImagesMinFillFrac - (rowCount-1)*agendaRulePt - (2*rowCount-2)*ctx.Gap(agendaRowGapPt)) / rowCount
	minRowPt = max(minRowPt, 48)
	// Each row is floored at the written height of its own title cell; when
	// the rows would not fit at the default title size, the titles step to
	// the 12pt floor rather than being written shrunk below it
	// (go-slide-creator-k3eb3).
	titleSize, rowNeeds, _ := agendaWithImagesFit(ctx, v, ovr)

	// Build content rows interleaved with thin divider rows (one divider between
	// each pair of items, none above the first or below the last).
	rows := make([]jsonschema.GridRowInput, 0, len(v.Items)*2-1)
	ruleFill := fillTone{Color: "dk1", Alpha: agendaWithImagesRuleAlpha}.fillJSON()

	for i, item := range v.Items {
		num := item.Number
		if num == 0 {
			num = i + 1
		}

		// Number cell: an unfilled bold accent numeral by default; a column of
		// accent-filled squares read as a row of solid accent blocks
		// (go-slide-creator-fl11f). overrides.style "solid" restores the
		// accent-filled rounded square with a white numeral.
		var numberCell *jsonschema.GridCellInput
		if solid {
			numberCell = buildAgendaBadgeCell(
				fmt.Sprintf("%02d", num), accent, numberSize, badgeWidthPct)
		} else {
			numberCell = buildAgendaNumeralCell(fmt.Sprintf("%02d", num), numeralInk, numberSize)
		}

		// Title cell: title (+ optional subtitle paragraph).
		titleCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildAgendaWithImagesTitleText(item.Title, item.Subtitle, titleSize, subtitleSize),
			},
		}

		// Apply per-cell override (text keys and accent bar on the title cell).
		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*AgendaWithImagesCellOverride); ok2 {
				applyCellTextOverride(titleCell, cellOvr)
				if cellOvr.AccentBar {
					titleCell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    4,
					}
				}
			}
		}

		cells := []*jsonschema.GridCellInput{numberCell, titleCell}

		switch {
		case !withImages:
			// No row has a label, so there is no image column at all: span the
			// title across columns 1+2.
			titleCell.ColSpan = 2
		default:
			// The column exists, so every row gets a placeholder — a captioned
			// one where there is a label, an empty one where there is not. A row
			// that simply skipped its box punched a hole in the column and made
			// the whole grid read as ragged (go-slide-creator-jodu).
			// The slot is a dashed outline over a faint tint, the placeholder
			// vocabulary of image-text-split: a solid grey slab read as a
			// designed tile, four to a slide (go-slide-creator-ja6oy).
			imageShape := &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     fillTone{Color: "lt2", Alpha: agendaWithImagesSlotTintAlpha}.fillJSON(),
				Line:     json.RawMessage(agendaWithImagesSlotLine),
			}
			if label := strings.TrimSpace(item.ImageLabel); label != "" {
				imageShape.Text = buildAgendaWithImagesImageLabelText(item.ImageLabel, imageLabelSize)
			}
			cells = append(cells, &jsonschema.GridCellInput{Shape: imageShape})
		}

		rows = append(rows, jsonschema.GridRowInput{
			AutoHeight: true,
			MinHeight:  max(minRowPt, rowNeeds[i]),
			Cells:      cells,
		})

		// One hairline rule between two rows (none after the last item), the
		// weight and ink of the plain agenda's. It stops at the text column:
		// run under the placeholders it doubled their edges
		// (go-slide-creator-ja6oy).
		if i < len(v.Items)-1 {
			rule := &jsonschema.GridCellInput{
				ColSpan: 3,
				Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ruleFill, Line: noLine},
			}
			cells := []*jsonschema.GridCellInput{rule}
			if withImages {
				rule.ColSpan = 2
				cells = append(cells, &jsonschema.GridCellInput{})
			}
			rows = append(rows, jsonschema.GridRowInput{MinHeight: agendaRulePt, MaxHeight: agendaRulePt, Cells: cells})
		}
	}

	// 18% / 48% / 32% from the layout spec, expressed as fractional units. A
	// numeral needs no badge column: it stands on the title's text line in a
	// column its own width, and the title takes the rest.
	columns := agendaWithImagesBadgeColumns
	if !solid {
		columns = agendaWithImagesNumeralColumns
	}
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(columns),
		Gap:     ctx.Gap(8),
		RowGap:  ctx.Gap(4),
		Rows:    rows,
	}

	return grid, nil
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type agendaWithImagesParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type agendaWithImagesTextObj struct {
	Paragraphs    []agendaWithImagesParagraph `json:"paragraphs"`
	Align         string                      `json:"align"`
	VerticalAlign string                      `json:"vertical_align"`
}

func buildAgendaWithImagesBadgeText(num string, size float64) json.RawMessage {
	return buildAgendaWithImagesNumberText(num, size, "lt1")
}

// buildAgendaWithImagesNumberText is the centred bold number in the given ink.
func buildAgendaWithImagesNumberText(num string, size float64, ink string) json.RawMessage {
	textObj := agendaWithImagesTextObj{
		Paragraphs: []agendaWithImagesParagraph{
			{Content: num, Size: size, Bold: true, Color: ink, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildAgendaWithImagesTitleText(title, subtitle string, titleSize, subtitleSize float64) json.RawMessage {
	paras := []agendaWithImagesParagraph{
		{Content: title, Size: titleSize, Bold: true, Color: "dk1", Align: "l"},
	}
	if strings.TrimSpace(subtitle) != "" {
		paras = append(paras, agendaWithImagesParagraph{
			Content: subtitle, Size: subtitleSize, Color: "dk1", Align: "l",
		})
	}
	textObj := agendaWithImagesTextObj{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// agendaWithImagesFit picks the title size (the default 18pt, stepping to
// 14pt and the 12pt floor when the rows would not otherwise fit the content
// area or a title that sat on one line would wrap; an authored title_size is
// kept) and returns each row's written-fit height at that size and the height
// all rows need together with dividers and gaps.
func agendaWithImagesFit(ctx ExpandContext, v *AgendaWithImagesValues, ovr *AgendaWithImagesOverrides) (float64, []float64, float64) {
	areaW, areaH := sizingAreaPt(ctx)
	const units = 1.8 + 4.8 + 3.2
	gridGapPt := ctx.Gap(8)
	unitW := (areaW - 2*gridGapPt) / units
	titleW := 4.8 * unitW
	if !anyAgendaImageLabel(v.Items) {
		titleW = 8*unitW + gridGapPt // the title spans the image column
	}
	n := float64(len(v.Items))
	fixed := (n-1)*agendaRulePt + (2*n-2)*ctx.Gap(agendaRowGapPt)
	subtitleSize := ResolveSize(ovr.SubtitleSize, scaleCaptionPt)
	sizes := []float64{ovr.TitleSize}
	if ovr.TitleSize == 0 {
		sizes = agendaWithImagesTitleSizes
	}
	textW := titleW - 2*defaultShapeInsetLRPt
	var needs []float64
	total := 0.0
	for si, size := range sizes {
		needs = make([]float64, len(v.Items))
		total = fixed
		wraps := false
		for i, item := range v.Items {
			needs[i] = writtenFitHeightPt(ctx.themeFonts(), buildAgendaWithImagesTitleText(item.Title, item.Subtitle, size, subtitleSize), titleW, 0)
			total += needs[i]
			// A larger step is not taken at the price of a second title
			// line: the title wraps only where it would at the floor too.
			wraps = wraps || (measuredLines(item.Title, ctx.Theme.BodyFont, true, size, textW*(1-cardGridOpenWrapSlack)) > 1 &&
				measuredLines(item.Title, ctx.Theme.BodyFont, true, agendaMinTitlePt, textW*(1-cardGridOpenWrapSlack)) == 1)
		}
		if total <= areaH && (!wraps || si == len(sizes)-1) {
			return size, needs, total
		}
	}
	return sizes[len(sizes)-1], needs, total
}

// agendaMinTitlePt is the title floor the default size steps down to.
const agendaMinTitlePt = scaleBodyPt

// agendaWithImagesTitleSizes are the default title steps, largest first.
var agendaWithImagesTitleSizes = []float64{scaleLeadPt, scaleSubheadPt, agendaMinTitlePt}

const (
	// agendaWithImagesMinFillFrac is the share of the content height the
	// rows take at least: the image slots need the height, and an agenda is
	// the slide's whole content.
	agendaWithImagesMinFillFrac = 0.85
	// agendaWithImagesNumberSize is the default numeral: the 28pt display
	// step of the plain agenda, in the same heading face.
	agendaWithImagesNumberSize = agendaNumberSize
	// agendaWithImagesRuleAlpha is the dk1 opacity of the rule between rows,
	// the plain agenda's.
	agendaWithImagesRuleAlpha = 30.0
	// agendaWithImagesSlotTintAlpha / agendaWithImagesSlotLine draw an image
	// slot: a faint tint under the 20% "filled" threshold inside a dashed
	// border, which reads as a wireframe slot and not as a content block. The
	// border is a mid neutral (dk1, "Lighter 50%"): up to six slots stack in
	// the column, and six full-ink dashed boxes outweighed the agenda.
	agendaWithImagesSlotTintAlpha = 15.0
	agendaWithImagesSlotLine      = `{"color":"dk1","lumMod":50000,"lumOff":50000,"width":1,"dash":"dash"}`
	// agendaWithImagesBadgeColumns / agendaWithImagesNumeralColumns are the
	// column weights with a solid badge and with a numeral.
	agendaWithImagesBadgeColumns   = `[1.8, 4.8, 3.2]`
	agendaWithImagesNumeralColumns = `[0.9, 5.7, 3.2]`
)

// anyAgendaImageLabel reports whether at least one item carries an image label,
// which is what earns the deck an image column at all.
func anyAgendaImageLabel(items []AgendaWithImagesItem) bool {
	for _, it := range items {
		if strings.TrimSpace(it.ImageLabel) != "" {
			return true
		}
	}
	return false
}

func buildAgendaWithImagesImageLabelText(label string, size float64) json.RawMessage {
	textObj := agendaWithImagesTextObj{
		Paragraphs: []agendaWithImagesParagraph{
			{Content: label, Size: size, Color: "dk2", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// agendaBadgeColumnFrac is the badge column's share of the grid's column units
// (1.8 of 1.8+4.8+3.2), used to compare the badge cell's width against the
// row's height.
const agendaBadgeColumnFrac = 1.8 / (1.8 + 4.8 + 3.2)

// agendaBadgeWidthPct returns how much of the number cell's WIDTH the badge
// should occupy so it renders square. 100 means "fill the cell".
//
// Only the width is narrowed. The badge column is 18% of the content width, so
// a row would have to be taller than that for the badge to be too TALL — at
// this pattern's minimum of three items the rows are already shorter than the
// column is wide, and they only get shorter as items are added. A height
// branch would be unreachable.
//
// The row height is estimated the way the grid divides it — n content rows plus
// n-1 hairline rule rows at agendaRulePt each, with RowGap between every pair —
// so the estimate and the layout cannot drift apart silently.
//
// ACCURACY IS CAPPED BY go-slide-creator-byr2b: on the generate path
// ExpandContext carries no LayoutBounds, so contentAreaPt reports the full-slide
// default (864x389pt) rather than the template's real content zone (828x308pt
// on midnight-blue). The badge therefore comes out about 20% wider than square
// there — measurably better than filling the cell, and exactly square once
// byr2b lands, with no change needed here.
func agendaBadgeWidthPct(ctx ExpandContext, n int) float64 {
	if n < 1 {
		return 100
	}
	w, h := contentAreaPt(ctx)
	colWidth := w * agendaBadgeColumnFrac
	if colWidth <= 0 || h <= 0 {
		return 100
	}
	dividers := float64(n-1) * agendaRulePt
	gaps := float64(2*n-2) * ctx.Gap(agendaRowGapPt)
	rowHeight := (h - dividers - gaps) / float64(n)
	if rowHeight <= 0 || rowHeight >= colWidth {
		return 100
	}
	// No lower floor: the badge is square or it is not, and a floor set to keep
	// it "substantial" simply reinstates the slab on the densest agendas — a
	// 35% floor made a six-item badge 53x42pt. The row height already shrinks
	// with the item count, which is the same budget the number's own font size
	// answers to.
	return rowHeight / colWidth * 100
}

// agendaRowGapPt mirrors the row gap of the grid the expansion builds.
const agendaRowGapPt = 4.0

// buildAgendaBadgeCell centres the badge in its cell at the given share of the
// cell's width, so it renders square rather than filling a cell whose
// proportions are the row's, not the badge's. At 100 it is the cell itself,
// with no nesting.
func buildAgendaBadgeCell(label, accent string, size, widthPct float64) *jsonschema.GridCellInput {
	badge := &jsonschema.ShapeSpecInput{
		Geometry: "roundRect",
		Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
		Text:     buildAgendaWithImagesBadgeText(label, size),
	}
	if widthPct >= 100 {
		return &jsonschema.GridCellInput{Shape: badge}
	}
	gutter := (100 - widthPct) / 2
	cols, _ := json.Marshal([]float64{gutter, widthPct, gutter})
	return &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(cols),
		Gap:     0.01,
		RowGap:  0.01,
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{{}, {Shape: badge}, {}},
		}},
	}}
}

// buildAgendaNumeralCell is the restrained default number cell: an unfilled,
// unoutlined numeral in the accent ink and the heading face, left-aligned so
// it stands on the title's text line as the plain agenda's does.
func buildAgendaNumeralCell(label, ink string, size float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Line:     noLine,
		Text:     agendaText(agendaParagraph{Content: label, Size: size, Color: ink, Font: agendaNumberFont}),
	}}
}
