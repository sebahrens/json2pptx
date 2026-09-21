package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// agenda-with-images pattern — numbered accent squares + title + image/quote
// per agenda row. Richer alternative to the plain `agenda` pattern.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&agendaWithImages{})
}

type agendaWithImages struct{}

func (a *agendaWithImages) Name() string { return "agenda-with-images" }
func (a *agendaWithImages) Description() string {
	return "Numbered accent squares + title/subtitle + image (or quote) placeholder per agenda row"
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
		Items: []AgendaWithImagesItem{
			{Title: "Executive Summary", Subtitle: "Situation, complication and our answer", ImageLabel: "Chart: Revenue trend"},
			{Title: "Market Analysis", Subtitle: "Size, growth and competitive position", ImageLabel: "Photo: Market scene"},
			{Title: "Strategic Options", Subtitle: "Three paths and our recommendation", ImageLabel: "Diagram: Option tree"},
			{Title: "Implementation Plan", Subtitle: "Phased rollout over 12 months", ImageLabel: "Photo: Project team"},
			{Title: "Next Steps", Subtitle: "Decisions required from this meeting"},
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
	NumberSize     float64 `json:"number_size,omitempty"`      // Font size for number badge (default 18)
	TitleSize      float64 `json:"title_size,omitempty"`       // Font size for row title (default 14)
	SubtitleSize   float64 `json:"subtitle_size,omitempty"`    // Font size for subtitle (default 10)
	ImageLabelSize float64 `json:"image_label_size,omitempty"` // Font size for image placeholder caption (default 10)
}

// AgendaWithImagesCellOverride is the shared per-cell override, indexed by item.
type AgendaWithImagesCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (a *agendaWithImages) NewValues() any       { return &AgendaWithImagesValues{} }
func (a *agendaWithImages) NewOverrides() any    { return &AgendaWithImagesOverrides{} }
func (a *agendaWithImages) NewCellOverride() any { return &AgendaWithImagesCellOverride{} }

// The title and subtitle share one text cell. The budget was measured at
// default text sizes across all four bundled templates. Without image labels,
// the text cell expands into the image column and retains its schema maximum.
func agendaWithImagesSubtitleBudget(rows, titleChars int, withImages bool) int {
	if !withImages || rows <= 4 {
		return 160
	}
	if titleChars <= 65 {
		return 150
	}
	if rows == 5 {
		return 75
	}
	return 0
}

func (a *agendaWithImages) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*AgendaWithImagesValues)
	if !ok || v == nil {
		return nil
	}
	withImages := anyAgendaImageLabel(v.Items)
	var warnings []string
	for i, item := range v.Items {
		budget := agendaWithImagesSubtitleBudget(len(v.Items), runeLen(item.Title), withImages)
		if n := runeLen(item.Subtitle); n > budget {
			if budget == 0 {
				warnings = append(warnings, fmt.Sprintf("%s: agenda-with-images items[%d].subtitle has %d characters; a %d-row agenda with image labels and a %d-character title leaves no readable subtitle room — shorten the title to about 65 characters, omit the subtitle, or use fewer rows", ErrCodeBodyTooLong, i, n, len(v.Items), runeLen(item.Title)))
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
			"title":       StringSchema(80).WithDescription("Section title (bold); with 5-6 image rows, keep it to about 65 characters when the subtitle exceeds 75 characters"),
			"subtitle":    StringSchema(160).WithDescription("Optional text below the title. About 160 readable characters with 3-4 rows or no image labels. With 5-6 image rows and a title up to 65 characters, use about 150. With a longer title, use about 75 at 5 rows or omit the subtitle at 6 rows"),
			"image_label": StringSchema(60).WithDescription("Optional caption centred in the image placeholder; omit it on one row and that row still gets an empty placeholder, omit it on every row to collapse the image column"),
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
			"accent":           StringSchema(0).WithDescription("Accent scheme color for number badges (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"number_size":      NumberSchema(6, 60).WithDescription("Font size for number badge in points (default 18)"),
			"title_size":       NumberSchema(6, 60).WithDescription("Font size for row title in points (default 14)"),
			"subtitle_size":    NumberSchema(6, 40).WithDescription("Font size for subtitle in points (default 10)"),
			"image_label_size": NumberSchema(6, 40).WithDescription("Font size for image placeholder caption in points (default 10)"),
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
	}).WithDescription("Numbered agenda rows with accent number badges, titles, optional subtitles, and optional image placeholders or pull-quotes")
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
	numberSize := ResolveSize(ovr.NumberSize, 18.0)
	titleSize := ResolveSize(ovr.TitleSize, 14.0)
	subtitleSize := ResolveSize(ovr.SubtitleSize, 10.0)
	imageLabelSize := ResolveSize(ovr.ImageLabelSize, 10.0)

	// The image column is all-or-nothing: one row's label earns the column for
	// every row, and no labels at all mean no column (go-slide-creator-jodu).
	withImages := anyAgendaImageLabel(v.Items)

	// The badge is meant to be a numbered SQUARE, but it filled its cell, which
	// is 18% of the content width by whatever height the row took: 1.5:1 on a
	// three-item agenda and 2.4:1 on a five-item one, so it read as an accent
	// slab (go-slide-creator-tiarr). badgeWidthPct narrows it inside its cell
	// to the row's own height, centring the remainder.
	badgeWidthPct := agendaBadgeWidthPct(ctx, len(v.Items))

	// Build content rows interleaved with thin divider rows (one divider between
	// each pair of items, none above the first or below the last).
	rows := make([]jsonschema.GridRowInput, 0, len(v.Items)*2-1)

	for i, item := range v.Items {
		num := item.Number
		if num == 0 {
			num = i + 1
		}

		// Number badge cell: accent-filled rounded square with white number.
		numberCell := buildAgendaBadgeCell(
			fmt.Sprintf("%02d", num), accent, numberSize, badgeWidthPct)

		// Title cell: title (+ optional subtitle paragraph).
		titleCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildAgendaWithImagesTitleText(item.Title, item.Subtitle, titleSize, subtitleSize),
			},
		}

		// Apply per-cell override (accent bar on the title cell).
		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*AgendaWithImagesCellOverride); ok2 && cellOvr.AccentBar {
				titleCell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
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
			imageShape := &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt2"`),
			}
			if label := strings.TrimSpace(item.ImageLabel); label != "" {
				imageShape.Text = buildAgendaWithImagesImageLabelText(item.ImageLabel, imageLabelSize)
			}
			cells = append(cells, &jsonschema.GridCellInput{Shape: imageShape})
		}

		rows = append(rows, jsonschema.GridRowInput{
			AutoHeight: true,
			MinHeight:  48,
			Cells:      cells,
		})

		// Divider row between content rows (not after the last item).
		if i < len(v.Items)-1 {
			rows = append(rows, jsonschema.GridRowInput{
				Height: 1,
				Cells: []*jsonschema.GridCellInput{
					{
						ColSpan: 3,
						Shape: &jsonschema.ShapeSpecInput{
							Geometry: "rect",
							Fill:     json.RawMessage(`"lt2"`),
						},
					},
				},
			})
		}
	}

	grid := &jsonschema.ShapeGridInput{
		// 18% / 48% / 32% from the layout spec, expressed as fractional units.
		Columns: json.RawMessage(`[1.8, 4.8, 3.2]`),
		Gap:     8,
		RowGap:  4,
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
	textObj := agendaWithImagesTextObj{
		Paragraphs: []agendaWithImagesParagraph{
			{Content: num, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
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
// n-1 hairline divider rows at agendaDividerHeightPct each, with RowGap between
// every pair — so the estimate and the layout cannot drift apart silently.
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
	dividers := float64(n-1) * h * agendaDividerHeightPct / 100
	gaps := float64(2*n-2) * agendaRowGapPt
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

const (
	// agendaDividerHeightPct / agendaRowGapPt mirror the grid the expansion
	// builds.
	agendaDividerHeightPct = 1.0
	agendaRowGapPt         = 4.0
)

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
