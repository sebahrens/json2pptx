package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

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
func (pq *pullQuote) UseWhen() string {
	return "Emphasize a single quote or testimonial; prefer stat-hero when the focal point is a number, not words"
}
func (pq *pullQuote) NotWhen() string {
	return "The focal content is a number/metric (use stat-hero), or multiple quotes need comparison (use card-grid)"
}
func (pq *pullQuote) Version() int      { return 1 }
func (pq *pullQuote) CellsHint() string { return "1" }
func (pq *pullQuote) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "hero",
		NarrativeRole: []string{"open", "conclude"},
		PairsWith:     []string{"kpi-3up", "card-grid", "before-after"},
		ComposesWith:  []string{"stylish-panels", "kpi-3up", "card-grid", "icon-row", "arch-stack"},
		RoleOnSlide:   []string{"callout", "banner"},
		DensityClass:  "low",
		AccentWeight:  "subtle",
	}
}

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
	Quote       string `json:"quote"`                 // The quote text
	Attribution string `json:"attribution"`           // Author/speaker name
	Role        string `json:"role,omitempty"`        // Optional role/title
	AccentSide  string `json:"accent_side,omitempty"` // "left" (default), "right", or "none"
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

func (pq *pullQuote) NewValues() any       { return &PullQuoteValues{} }
func (pq *pullQuote) NewOverrides() any    { return &PullQuoteOverrides{} }
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
	} else if runeLen(v.Quote) > 500 {
		errs = append(errs, errMaxLength(name, "values.quote", 500, runeLen(v.Quote)))
	}

	if v.Attribution == "" {
		errs = append(errs, errRequired(name, "values.attribution"))
	} else if runeLen(v.Attribution) > 60 {
		errs = append(errs, errMaxLength(name, "values.attribution", 60, runeLen(v.Attribution)))
	}

	if v.Role != "" && runeLen(v.Role) > 60 {
		errs = append(errs, errMaxLength(name, "values.role", 60, runeLen(v.Role)))
	}

	if v.AccentSide != "" && v.AccentSide != "left" && v.AccentSide != "right" && v.AccentSide != "none" {
		errs = append(errs, newValidationError(name, "values.accent_side", ErrCodeUnknownEnum,
			fmt.Sprintf("pull-quote: values.accent_side must be \"left\", \"right\", or \"none\", got %q", v.AccentSide),
			UseOneOfFix("values.accent_side", []string{"left", "right", "none"})))
	}

	return errors.Join(errs...)
}

// Pull-quote geometry (go-slide-creator-36ny).
//
// The quote and its attribution used to be consecutive paragraphs in ONE cell
// with no spacing, which cost three things at once: the attribution's baseline
// sat directly under the quote's descenders (on a long quote the two lines'
// ink actually crossed), the cell's autofit shrank the attribution along with
// the quote — down to ~7pt on a 500-character quote — and the accent rule ran
// the full height of a box whose bottom third was empty.
//
// They are now two rows of one grid: separate text bodies, so the quote's
// autofit cannot touch the attribution, with an explicit gap between them and
// both rows sized to their own content so the rule stops where the text does.
const (
	// pullQuoteGapFrac is the gap under the quote, as a fraction of the quote's
	// own size — ~0.6em, which reads as one blank line at any type scale.
	pullQuoteGapFrac = 0.6
	// pullQuoteGapMinPt / MaxPt keep that gap sane at extreme type scales.
	pullQuoteGapMinPt = 10.0
	pullQuoteGapMaxPt = 28.0
	// pullQuoteAttrMinPt is the floor for the attribution. Below it the
	// speaker's name is unreadable at the back of a room, and an unreadable
	// attribution is the same as no attribution.
	pullQuoteAttrMinPt = 11.0
	// pullQuoteMaxQuoteFrac caps the quote row so a very long quote still
	// leaves the attribution its row.
	pullQuoteMaxQuoteFrac = 0.8
	// pullQuoteRulePt is the thickness of the accent rule, and
	// pullQuoteRuleMinPct keeps it visible inside a small composed cell.
	pullQuoteRulePt     = 6.0
	pullQuoteRuleMinPct = 0.5
	// pullQuoteRuleGapPt is the space between the rule and the text.
	pullQuoteRuleGapPt = 14.0
	// pullQuoteRowGapPt is the gap between the quote row and the attribution
	// row. The gap under the quote's text is the quote cell's bottom inset, so
	// this stays small; the rule spans both rows, so it is not broken by it.
	pullQuoteRowGapPt = 1.0
)

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

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	quoteSize := ResolveSize(ovr.QuoteSize, 36.0)
	attrSize := math.Max(ResolveSize(ovr.AttrSize, 14.0), pullQuoteAttrMinPt)

	// Build attribution line
	attrLine := "\u2014 " + v.Attribution
	if v.Role != "" {
		attrLine += ", " + v.Role
	}
	quoteLine := "\u201C" + v.Quote + "\u201D"

	accentSide := v.AccentSide
	if accentSide == "" {
		accentSide = "left"
	}

	areaW, areaH := sizingAreaPt(ctx)
	textW := areaW
	rulePct := 0.0
	if accentSide != "none" {
		rulePct = math.Max(pctOf(pullQuoteRulePt, areaW), pullQuoteRuleMinPct)
		textW = areaW * (100 - rulePct) / 100
	}

	gapPt := math.Min(math.Max(quoteSize*pullQuoteGapFrac, pullQuoteGapMinPt), pullQuoteGapMaxPt)
	attrRowPt := sizedBlockHeightPt(ctx, []sizedPara{{text: attrLine, sizePt: attrSize}}, textW)
	quoteRowPt := sizedBlockHeightPt(ctx, []sizedPara{{text: quoteLine, sizePt: quoteSize}}, textW) + gapPt
	// A quote longer than its share of the area is left to the renderer's
	// autofit — but only the quote's own row shrinks, never the attribution.
	if capPt := areaH * pullQuoteMaxQuoteFrac; quoteRowPt > capPt {
		quoteRowPt = capPt
	}

	quoteCell := pullQuoteCell([]pullQuoteParagraph{
		{Content: quoteLine, Size: quoteSize, Italic: true, Color: "dk1", Align: "ctr"},
	}, "b", 0, gapPt)
	attrCell := pullQuoteCell([]pullQuoteParagraph{
		{Content: attrLine, Size: attrSize, Color: "dk1", Align: "ctr"},
	}, "t", 0, 0)

	// The accent rule is a column spanning both rows rather than a per-cell
	// accent bar: two bars would be broken apart by the row gap, and the rule
	// has to read as one mark against the whole block.
	columns := json.RawMessage(`1`)
	quoteCells := []*jsonschema.GridCellInput{quoteCell}
	attrCells := []*jsonschema.GridCellInput{attrCell}
	if accentSide != "none" {
		rule := &jsonschema.GridCellInput{
			RowSpan: 2,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(strconv.Quote(accent)),
			},
		}
		columns = json.RawMessage(fmt.Sprintf("[%.3f,%.3f]", rulePct, 100-rulePct))
		quoteCells = []*jsonschema.GridCellInput{rule, quoteCell}
		// The attribution row lists only its own cell: the resolver skips the
		// column the rule's row-span already occupies.
		attrCells = []*jsonschema.GridCellInput{attrCell}
		if accentSide == "right" {
			columns = json.RawMessage(fmt.Sprintf("[%.3f,%.3f]", 100-rulePct, rulePct))
			quoteCells = []*jsonschema.GridCellInput{quoteCell, rule}
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: columns,
		ColGap:  pullQuoteRuleGapPt,
		RowGap:  pullQuoteRowGapPt,
		Rows: []jsonschema.GridRowInput{
			{MaxHeight: quoteRowPt, Cells: quoteCells},
			{MinHeight: attrRowPt, MaxHeight: attrRowPt, Cells: attrCells},
		},
	}

	return grid, nil
}

// pullQuoteCell wraps paragraphs in a text cell with the given vertical anchor
// and text insets (points).
func pullQuoteCell(paras []pullQuoteParagraph, vAlign string, insetTop, insetBottom float64) *jsonschema.GridCellInput {
	textJSON, _ := json.Marshal(pullQuoteText{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: vAlign,
		InsetTop:      insetTop,
		InsetBottom:   insetBottom,
	})
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}
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
	InsetTop      float64              `json:"inset_top,omitempty"`
	InsetBottom   float64              `json:"inset_bottom,omitempty"`
}
