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
	ImageLabel string `json:"image_label,omitempty"` // Optional label shown in the image placeholder; when empty the right zone collapses
}

// AgendaWithImagesValues holds the agenda rows (3-6 items).
type AgendaWithImagesValues struct {
	Items []AgendaWithImagesItem `json:"items"`
}

// AgendaWithImagesOverrides controls accent and typography.
type AgendaWithImagesOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	NumberSize     float64 `json:"number_size,omitempty"`   // Font size for number badge (default 18)
	TitleSize      float64 `json:"title_size,omitempty"`    // Font size for row title (default 14)
	SubtitleSize   float64 `json:"subtitle_size,omitempty"` // Font size for subtitle (default 10)
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

func (a *agendaWithImages) Schema() *Schema {
	itemSchema := ObjectSchema(
		map[string]*Schema{
			"number":      IntegerSchema(0, 999).WithDescription("1-based ordinal; auto-assigned 1..N when omitted (use 0 or omit to auto-assign)"),
			"title":       StringSchema(80).WithDescription("Section title (bold)"),
			"subtitle":    StringSchema(160).WithDescription("Optional descriptive subtitle rendered below the title"),
			"image_label": StringSchema(60).WithDescription("Optional label centred in the image placeholder; omit to collapse the right zone"),
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
		} else if len(item.Title) > 80 {
			errs = append(errs, errMaxLength(name, titlePath, 80, len(item.Title)))
		}
		if len(item.Subtitle) > 160 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("items[%d].subtitle", i), 160, len(item.Subtitle)))
		}
		if len(item.ImageLabel) > 60 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("items[%d].image_label", i), 60, len(item.ImageLabel)))
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

	// Build content rows interleaved with thin divider rows (one divider between
	// each pair of items, none above the first or below the last).
	rows := make([]jsonschema.GridRowInput, 0, len(v.Items)*2-1)

	for i, item := range v.Items {
		num := item.Number
		if num == 0 {
			num = i + 1
		}

		// Number badge cell: accent-filled rounded square with white number.
		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     buildAgendaWithImagesBadgeText(fmt.Sprintf("%02d", num), numberSize),
			},
		}

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

		if strings.TrimSpace(item.ImageLabel) != "" {
			// Right zone: light-grey placeholder rectangle with centred label.
			imageCell := &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"lt2"`),
					Text:     buildAgendaWithImagesImageLabelText(item.ImageLabel, imageLabelSize),
				},
			}
			cells = append(cells, imageCell)
		} else {
			// Collapse the right zone by spanning the title across columns 1+2.
			titleCell.ColSpan = 2
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
