package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// pull-quote pattern — italic quote block with attribution
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&pullQuote{})
}

type pullQuote struct{}

func (pq *pullQuote) Name() string        { return "pull-quote" }
func (pq *pullQuote) Description() string { return "Italic quote block with attribution" }
func (pq *pullQuote) UseWhen() string     { return "Emphasize a single quote or testimonial" }
func (pq *pullQuote) Version() int        { return 1 }
func (pq *pullQuote) CellsHint() string   { return "1" }

func (pq *pullQuote) SupportsCallout() bool { return false }

func (pq *pullQuote) ExemplarValues() any {
	v := PullQuoteValues{
		Quote:       "The best way to predict the future is to invent it.",
		Attribution: "Alan Kay",
		Role:        "Computer Scientist",
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PullQuoteValues holds the data for a pull-quote pattern.
type PullQuoteValues struct {
	Quote       string `json:"quote"`                  // The quote text
	Attribution string `json:"attribution"`            // Author/speaker name
	Role        string `json:"role,omitempty"`         // Optional role/title
	AccentSide  string `json:"accent_side,omitempty"`  // "left" (default), "right", or "none"
}

// PullQuoteOverrides contains pattern-level overrides for pull-quote.
type PullQuoteOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	QuoteSize      float64 `json:"quote_size,omitempty"`
	AttrSize       float64 `json:"attr_size,omitempty"`
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (pq *pullQuote) NewValues() any      { return &PullQuoteValues{} }
func (pq *pullQuote) NewOverrides() any   { return &PullQuoteOverrides{} }
func (pq *pullQuote) NewCellOverride() any { return nil }

func (pq *pullQuote) Schema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"values": ObjectSchema(
				map[string]*Schema{
					"quote":       StringSchema(500).WithDescription("The quote text"),
					"attribution": StringSchema(60).WithDescription("Author/speaker name"),
					"role":        StringSchema(60).WithDescription("Optional role or title"),
					"accent_side": EnumSchema("left", "right", "none").WithDescription("Side for accent rule (default \"left\")").WithDefault("left"),
				},
				[]string{"quote", "attribution"},
			).WithAdditionalProperties(false),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"quote_size":      NumberSchema(20, 80).WithDescription("Font size for quote text in points (default 36)"),
					"attr_size":       NumberSchema(6, 40).WithDescription("Font size for attribution in points (default 14)"),
				},
				nil,
			).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Italic quote block with attribution")
}

func (pq *pullQuote) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*PullQuoteValues)
	if !ok || v == nil {
		return fmt.Errorf("pull-quote: values must be *PullQuoteValues, got %T", values)
	}

	const name = "pull-quote"
	var errs []error

	if v.Quote == "" {
		errs = append(errs, errRequired(name, "values.quote"))
	} else if len(v.Quote) > 500 {
		errs = append(errs, errMaxLength(name, "values.quote", 500, len(v.Quote)))
	}

	if v.Attribution == "" {
		errs = append(errs, errRequired(name, "values.attribution"))
	} else if len(v.Attribution) > 60 {
		errs = append(errs, errMaxLength(name, "values.attribution", 60, len(v.Attribution)))
	}

	if v.Role != "" && len(v.Role) > 60 {
		errs = append(errs, errMaxLength(name, "values.role", 60, len(v.Role)))
	}

	if v.AccentSide != "" && v.AccentSide != "left" && v.AccentSide != "right" && v.AccentSide != "none" {
		errs = append(errs, newValidationError(name, "values.accent_side", ErrCodeUnknownEnum,
			fmt.Sprintf("pull-quote: values.accent_side must be \"left\", \"right\", or \"none\", got %q", v.AccentSide),
			"set accent_side to \"left\", \"right\", or \"none\""))
	}

	return errors.Join(errs...)
}

func (pq *pullQuote) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*PullQuoteValues)
	if !ok {
		return nil, fmt.Errorf("pull-quote: values must be *PullQuoteValues, got %T", values)
	}
	ovr := &PullQuoteOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*PullQuoteOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("pull-quote: overrides must be *PullQuoteOverrides, got %T", overrides)
		}
	}

	accent := ResolveAccent(ovr.Accent, ovr.SemanticAccent, ctx.Metadata)
	quoteSize := ResolveSize(ovr.QuoteSize, 36.0)
	attrSize := ResolveSize(ovr.AttrSize, 14.0)

	// Build attribution line
	attrLine := "— " + v.Attribution
	if v.Role != "" {
		attrLine += ", " + v.Role
	}

	// Build paragraphs
	paragraphs := []pullQuoteParagraph{
		{Content: "\u201C" + v.Quote + "\u201D", Size: quoteSize, Italic: true, Color: "dk1", Align: "ctr"},
		{Content: attrLine, Size: attrSize, Color: "dk1", Align: "ctr"},
	}

	textObj := pullQuoteText{
		Paragraphs:    paragraphs,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	textJSON, _ := json.Marshal(textObj)

	cell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}

	// Add accent bar if requested
	accentSide := v.AccentSide
	if accentSide == "" {
		accentSide = "left"
	}
	if accentSide != "none" {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: accentSide,
			Color:    accent,
			Width:    6,
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []jsonschema.GridRowInput{
			{Cells: []*jsonschema.GridCellInput{cell}},
		},
	}

	return grid, nil
}

// pullQuoteParagraph is a text paragraph for JSON marshalling.
type pullQuoteParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Italic  bool    `json:"italic,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

// pullQuoteText is the text object for JSON marshalling.
type pullQuoteText struct {
	Paragraphs    []pullQuoteParagraph `json:"paragraphs"`
	Align         string               `json:"align"`
	VerticalAlign string               `json:"vertical_align"`
}
