package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// hero-detail pattern — big hero stat at top with supporting detail cards below
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&heroDetail{})
}

type heroDetail struct{}

func (hd *heroDetail) Name() string        { return "hero-detail" }
func (hd *heroDetail) Description() string { return "Big hero statistic with 2-4 supporting detail cards below" }
func (hd *heroDetail) UseWhen() string {
	return "One dominant metric plus 2-4 supporting detail bullets; prefer stat-hero when no details are needed, kpi-3up when all metrics have equal weight"
}
func (hd *heroDetail) NotWhen() string {
	return "No supporting details (use stat-hero), all items have equal weight (use kpi-3up or kpi-4up), or content is a quote (use pull-quote)"
}
func (hd *heroDetail) Version() int    { return 1 }
func (hd *heroDetail) CellsHint() string { return "1 + 2-4" }
func (hd *heroDetail) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "hero",
		NarrativeRole: []string{"evidence"},
		PairsWith:     []string{"kpi-3up", "chart", "process-flow"},
		DensityClass:  "low",
		AccentWeight:  "strong",
	}
}

func (hd *heroDetail) SupportsCallout() bool        { return true }
func (hd *heroDetail) SupportsInlineMarkdown() bool { return true }

func (hd *heroDetail) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 2, Rows: 2},
		{Columns: 3, Rows: 2},
		{Columns: 4, Rows: 2},
	}
}

func (hd *heroDetail) ExemplarValues() any {
	return &HeroDetailValues{
		Hero: HeroDetailHero{
			Value: "$2.4B",
			Label: "Addressable AI consulting market by FY27",
		},
		Details: []HeroDetailItem{
			{Title: "Market Growth", Body: "42% CAGR driven by enterprise adoption"},
			{Title: "Key Segment", Body: "Financial services leads with 35% share"},
			{Title: "Outlook", Body: "Projected to reach $5.1B by FY30"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// HeroDetailHero holds the hero stat data (top row).
type HeroDetailHero struct {
	Value   string `json:"value"`             // The big number (e.g. "$2.4B", "99.9%")
	Label   string `json:"label"`             // One-line context beneath the number
	Context string `json:"context,omitempty"` // Optional subtext
}

// HeroDetailItem is a single detail card (bottom row).
type HeroDetailItem struct {
	Icon  string `json:"icon,omitempty"`  // Emoji or bundled icon name
	Title string `json:"title"`          // Card title
	Body  string `json:"body,omitempty"` // Card body text
}

// HeroDetailValues holds the data for a hero-detail pattern.
type HeroDetailValues struct {
	Hero    HeroDetailHero   `json:"hero"`
	Details []HeroDetailItem `json:"details"`
}

// HeroDetailOverrides contains pattern-level overrides for hero-detail.
type HeroDetailOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeroSize       float64 `json:"hero_size,omitempty"`    // Font size for the big number (default 80)
	LabelSize      float64 `json:"label_size,omitempty"`   // Font size for the label (default 16)
	HeaderSize     float64 `json:"header_size,omitempty"`  // Font size for detail titles (default 14)
	DetailSize     float64 `json:"detail_size,omitempty"`  // Font size for detail body text (default 11)
	Style          string  `json:"style,omitempty"`        // "cards" (default) or "minimal"
}

// HeroDetailCellOverride is an alias for the shared CellOverride struct.
type HeroDetailCellOverride = CellOverride

// validHeroDetailStyles enumerates the allowed style values.
var validHeroDetailStyles = map[string]bool{
	"":        true, // default = cards
	"cards":   true,
	"minimal": true,
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (hd *heroDetail) NewValues() any      { return &HeroDetailValues{} }
func (hd *heroDetail) NewOverrides() any   { return &HeroDetailOverrides{} }
func (hd *heroDetail) NewCellOverride() any { return &HeroDetailCellOverride{} }

func (hd *heroDetail) Schema() *Schema {
	heroSchema := ObjectSchema(
		map[string]*Schema{
			"value":   StringSchema(20).WithDescription("The big number (e.g. \"$2.4B\", \"99.9%\")"),
			"label":   StringSchema(80).WithDescription("One-line label beneath the number"),
			"context": StringSchema(120).WithDescription("Optional subtext line"),
		},
		[]string{"value", "label"},
	).WithAdditionalProperties(false)

	detailSchema := ObjectSchema(
		map[string]*Schema{
			"icon":  StringSchema(60).WithDescription("Emoji or bundled icon name"),
			"title": StringSchema(60).WithDescription("Detail card title"),
			"body":  StringSchema(200).WithDescription("Detail card body text"),
		},
		[]string{"title"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"hero":    heroSchema.WithDescription("The hero statistic (top row)"),
			"details": ArraySchema(detailSchema, 2, 4).WithDescription("Supporting detail cards (bottom row, 2-4 items)"),
		},
		[]string{"hero", "details"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"hero_size":       NumberSchema(40, 200).WithDescription("Font size for the big number in points (default 80)"),
			"label_size":      NumberSchema(6, 60).WithDescription("Font size for the label in points (default 16)"),
			"header_size":     NumberSchema(6, 60).WithDescription("Font size for detail titles in points (default 14)"),
			"detail_size":     NumberSchema(6, 40).WithDescription("Font size for detail body text in points (default 11)"),
			"style":           EnumSchema("cards", "minimal").WithDescription("Visual style: cards (default, accent-filled detail cards) or minimal (light background with accent headers)").WithDefault("cards"),
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
	}).WithDescription("Big hero statistic with 2-4 supporting detail cards below")
}

func (hd *heroDetail) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*HeroDetailValues)
	if !ok || v == nil {
		return fmt.Errorf("hero-detail: values must be *HeroDetailValues, got %T", values)
	}

	const name = "hero-detail"
	var errs []error

	// Hero validation
	if v.Hero.Value == "" {
		errs = append(errs, errRequired(name, "hero.value"))
	} else if len(v.Hero.Value) > 20 {
		errs = append(errs, errMaxLength(name, "hero.value", 20, len(v.Hero.Value)))
	}

	if v.Hero.Label == "" {
		errs = append(errs, errRequired(name, "hero.label"))
	} else if len(v.Hero.Label) > 80 {
		errs = append(errs, errMaxLength(name, "hero.label", 80, len(v.Hero.Label)))
	}

	if v.Hero.Context != "" && len(v.Hero.Context) > 120 {
		errs = append(errs, errMaxLength(name, "hero.context", 120, len(v.Hero.Context)))
	}

	// Details validation
	if len(v.Details) < 2 || len(v.Details) > 4 {
		errs = append(errs, errOutOfRange(name, "details count", 2, 4, len(v.Details)))
	}

	for i, d := range v.Details {
		titlePath := fmt.Sprintf("details[%d].title", i)
		if d.Title == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if len(d.Title) > 60 {
			errs = append(errs, errMaxLength(name, titlePath, 60, len(d.Title)))
		}
		if d.Body != "" && len(d.Body) > 200 {
			bodyPath := fmt.Sprintf("details[%d].body", i)
			errs = append(errs, errMaxLength(name, bodyPath, 200, len(d.Body)))
		}
	}

	// Validate style override
	if overrides != nil {
		if ovr, ok := overrides.(*HeroDetailOverrides); ok {
			if ovr.Style != "" && !validHeroDetailStyles[ovr.Style] {
				errs = append(errs, &ValidationError{
					Pattern: name,
					Path:    "overrides.style",
					Code:    "invalid_enum",
					Message: fmt.Sprintf("hero-detail: overrides.style must be one of cards, minimal; got %q", ovr.Style),
				})
			}
		}
	}

	// Validate cell_overrides keys — total cells = 1 (hero) + len(details)
	totalCells := 1 + len(v.Details)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (hd *heroDetail) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*HeroDetailValues)
	if !ok {
		return nil, fmt.Errorf("hero-detail: values must be *HeroDetailValues, got %T", values)
	}
	ovr := &HeroDetailOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*HeroDetailOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("hero-detail: overrides must be *HeroDetailOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	heroSize := ResolveSize(ovr.HeroSize, 80.0)
	labelSize := ResolveSize(ovr.LabelSize, 16.0)
	headerSize := ResolveSize(ovr.HeaderSize, 14.0)
	detailSize := ResolveSize(ovr.DetailSize, 11.0)
	style := ovr.Style
	if style == "" {
		style = "cards"
	}

	// Row 1: Hero stat (single cell spanning all columns, ~40% height)
	heroCell := hd.buildHeroCell(v.Hero, accent, heroSize, labelSize)
	heroCell.ColSpan = len(v.Details)

	heroRow := jsonschema.GridRowInput{
		Height: 40,
		Cells:  []*jsonschema.GridCellInput{heroCell},
	}

	// Row 2: Detail cards (N columns, ~60% height)
	detailCells := make([]*jsonschema.GridCellInput, len(v.Details))
	for i, d := range v.Details {
		switch style {
		case "minimal":
			detailCells[i] = hd.buildMinimalDetailCell(d, accent, headerSize, detailSize)
		default: // "cards"
			detailCells[i] = hd.buildCardDetailCell(d, accent, headerSize, detailSize)
		}
		// Apply cell overrides (detail cells are 1-indexed since cell 0 is the hero)
		if co, ok := cellOverrides[i+1]; ok {
			cellOvr, coOk := co.(*HeroDetailCellOverride)
			if coOk && cellOvr.AccentBar {
				detailCells[i].AccentBar = &jsonschema.AccentBarInput{
					Position: "top",
					Color:    accent,
					Width:    3,
				}
			}
		}
	}

	detailRow := jsonschema.GridRowInput{
		Cells: detailCells,
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, len(v.Details))),
		Gap:     10,
		Rows: []jsonschema.GridRowInput{
			heroRow,
			detailRow,
		},
	}

	return grid, nil
}

// buildHeroCell produces the hero stat cell for the top row.
func (hd *heroDetail) buildHeroCell(hero HeroDetailHero, accent string, heroSize, labelSize float64) *jsonschema.GridCellInput {
	paras := []heroDetailParagraph{
		{Content: hero.Value, Size: heroSize, Bold: true, Color: accent, Align: "ctr"},
		{Content: hero.Label, Size: labelSize, Color: "dk1", Align: "ctr"},
	}
	if hero.Context != "" {
		paras = append(paras, heroDetailParagraph{
			Content: hero.Context, Size: labelSize - 2, Color: "dk1", Align: "ctr",
		})
	}

	textObj := heroDetailText{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	textJSON, _ := json.Marshal(textObj)

	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}
}

// buildCardDetailCell produces an accent-filled detail card (default style).
func (hd *heroDetail) buildCardDetailCell(d HeroDetailItem, accent string, headerSize, detailSize float64) *jsonschema.GridCellInput {
	paras := []heroDetailParagraph{
		{Content: d.Title, Size: headerSize, Bold: true, Color: "lt1", Align: "l"},
	}
	if d.Body != "" {
		paras = append(paras, heroDetailParagraph{
			Content: pptx.ConvertMarkdownEmphasis(d.Body), Size: detailSize, Color: "lt1", Align: "l",
		})
	}

	textObj := heroDetailText{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	textJSON, _ := json.Marshal(textObj)

	gc := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     textJSON,
		},
	}

	// Add icon overlay when provided
	if d.Icon != "" {
		gc.Shape.Icon = &jsonschema.IconInput{
			Name:     d.Icon,
			Fill:     accent,
			Position: "top",
		}
	}

	return gc
}

// buildMinimalDetailCell produces a light-background detail card with accent header text.
func (hd *heroDetail) buildMinimalDetailCell(d HeroDetailItem, accent string, headerSize, detailSize float64) *jsonschema.GridCellInput {
	paras := []heroDetailParagraph{
		{Content: d.Title, Size: headerSize, Bold: true, Color: accent, Align: "l"},
	}
	if d.Body != "" {
		paras = append(paras, heroDetailParagraph{
			Content: pptx.ConvertMarkdownEmphasis(d.Body), Size: detailSize, Color: "dk1", Align: "l",
		})
	}

	textObj := heroDetailText{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	textJSON, _ := json.Marshal(textObj)

	gc := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"lt1"`),
			Text:     textJSON,
		},
		AccentBar: &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    3,
		},
	}

	if d.Icon != "" {
		gc.Shape.Icon = &jsonschema.IconInput{
			Name:     d.Icon,
			Fill:     accent,
			Position: "top",
		}
	}

	return gc
}

// heroDetailParagraph is a text paragraph for JSON marshalling.
type heroDetailParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

// heroDetailText is the text object for JSON marshalling.
type heroDetailText struct {
	Paragraphs    []heroDetailParagraph `json:"paragraphs"`
	Align         string               `json:"align"`
	VerticalAlign string               `json:"vertical_align"`
}
