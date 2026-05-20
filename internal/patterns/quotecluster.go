package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// quote-cluster pattern — structured 3-col grid of stakeholder quote bubbles
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&quoteCluster{})
}

type quoteCluster struct{}

func (q *quoteCluster) Name() string { return "quote-cluster" }
func (q *quoteCluster) Description() string {
	return "Structured 3-column grid of stakeholder quote bubbles (3–8 quotes, each with attribution)"
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

// QuoteClusterItem is one quote bubble: quote text + attribution (name, title).
type QuoteClusterItem struct {
	Text  string `json:"text"`            // The quote text (italic)
	Name  string `json:"name"`            // Speaker's name (bold)
	Title string `json:"title,omitempty"` // Optional role / title rendered after the name
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
}

// QuoteClusterCellOverride is the shared per-cell override, indexed by quote.
type QuoteClusterCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	quoteClusterMinQuotes   = 3
	quoteClusterMaxQuotes   = 8
	quoteClusterColumns     = 3
	quoteClusterTextMax     = 240
	quoteClusterNameMax     = 60
	quoteClusterTitleMax    = 80
)

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (q *quoteCluster) NewValues() any       { return &QuoteClusterValues{} }
func (q *quoteCluster) NewOverrides() any    { return &QuoteClusterOverrides{} }
func (q *quoteCluster) NewCellOverride() any { return &QuoteClusterCellOverride{} }

func (q *quoteCluster) Schema() *Schema {
	quoteSchema := ObjectSchema(
		map[string]*Schema{
			"text":  StringSchema(quoteClusterTextMax).WithDescription("Quote text (italic, ~10pt)"),
			"name":  StringSchema(quoteClusterNameMax).WithDescription("Speaker name (bold)"),
			"title": StringSchema(quoteClusterTitleMax).WithDescription("Optional role or title rendered next to the name"),
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
			"accent":          StringSchema(0).WithDescription("Accent scheme color used for the speaker name (default accent1)").WithDefault("accent1"),
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
	}).WithDescription("Structured 3-column grid of stakeholder quote bubbles with alternating tinted fills")
}

func (q *quoteCluster) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*QuoteClusterValues)
	if !ok || v == nil {
		return fmt.Errorf("quote-cluster: values must be *QuoteClusterValues, got %T", values)
	}

	const name = "quote-cluster"
	var errs []error

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
		} else if len(qt.Text) > quoteClusterTextMax {
			errs = append(errs, errMaxLength(name, textPath, quoteClusterTextMax, len(qt.Text)))
		}
		namePath := fmt.Sprintf("quotes[%d].name", i)
		if strings.TrimSpace(qt.Name) == "" {
			errs = append(errs, errRequired(name, namePath))
		} else if len(qt.Name) > quoteClusterNameMax {
			errs = append(errs, errMaxLength(name, namePath, quoteClusterNameMax, len(qt.Name)))
		}
		if len(qt.Title) > quoteClusterTitleMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("quotes[%d].title", i), quoteClusterTitleMax, len(qt.Title)))
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
	quoteSize := ResolveSize(ovr.QuoteSize, 10.0)
	nameSize := ResolveSize(ovr.NameSize, 9.0)
	titleSize := ResolveSize(ovr.TitleSize, 8.0)

	// Lay out quotes left-to-right into 3-column rows. Short final rows stay
	// left-aligned: any unused right-hand columns become empty filler cells so
	// the grid keeps a consistent column count.
	totalRows := (len(v.Quotes) + quoteClusterColumns - 1) / quoteClusterColumns
	rows := make([]jsonschema.GridRowInput, 0, totalRows)

	for r := 0; r < totalRows; r++ {
		cells := make([]*jsonschema.GridCellInput, quoteClusterColumns)
		for c := 0; c < quoteClusterColumns; c++ {
			idx := r*quoteClusterColumns + c
			if idx >= len(v.Quotes) {
				cells[c] = buildQuoteClusterEmptyCell()
				continue
			}
			qt := v.Quotes[idx]
			fill := ctx.ResolveSurface("subtle", "lt1")
			if idx%2 == 1 {
				fill = ctx.ResolveSurface("paper", "lt2")
			}
			cell := &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "roundRect",
					Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, fill)),
					Text:     buildQuoteClusterBubbleText(qt, quoteSize, nameSize, titleSize, accent),
				},
			}
			if co, coOk := cellOverrides[idx]; coOk {
				if cellOvr, ok2 := co.(*QuoteClusterCellOverride); ok2 && cellOvr.AccentBar {
					cell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    3,
					}
				}
			}
			cells[c] = cell
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: cells})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, quoteClusterColumns)),
		Gap:     10,
		RowGap:  10,
		Rows:    rows,
	}
	return grid, nil
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

func buildQuoteClusterEmptyCell() *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
		},
	}
}
