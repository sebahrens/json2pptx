package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// pull-quote pattern — italic quote block with attribution
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&pullQuote{})
}

type pullQuote struct{}

func (pq *pullQuote) Name() string { return "pull-quote" }
func (pq *pullQuote) Description() string {
	return "Italic quote block with attribution and an optional headshot beside it"
}
func (pq *pullQuote) UseWhen() string {
	return "Emphasize a single quote or testimonial; prefer stat-hero when the focal point is a number, not words"
}
func (pq *pullQuote) NotWhen() string {
	return "The focal content is a number/metric (use stat-hero), or multiple quotes need comparison (use card-grid)"
}
func (pq *pullQuote) Version() int      { return 2 }
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
	Quote       string                     `json:"quote"`                 // The quote text
	Attribution string                     `json:"attribution"`           // Author/speaker name
	Role        string                     `json:"role,omitempty"`        // Optional role/title
	Image       *jsonschema.GridImageInput `json:"image,omitempty"`       // Optional headshot {path | url, alt}, cover-cropped into a column beside the quote
	AccentSide  string                     `json:"accent_side,omitempty"` // "left" (default), "right", or "none"
}

// PullQuoteOverrides contains pattern-level overrides for pull-quote.
type PullQuoteOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	QuoteSize      float64 `json:"quote_size,omitempty"`
	AttrSize       float64 `json:"attr_size,omitempty"`
	ImageSide      string  `json:"image_side,omitempty"`      // "left" (default) or "right"
	ImageWidthPct  float64 `json:"image_width_pct,omitempty"` // Headshot column width, 15-40% (default 25)
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

// ImageAssets exposes values.image so hosts resolve its path / url the way they
// do for shape_grid image cells (go-slide-creator-hdpq).
func (pq *pullQuote) ImageAssets(values any) []ImageAssetRef {
	v, ok := values.(*PullQuoteValues)
	if !ok || v == nil || v.Image == nil {
		return nil
	}
	return []ImageAssetRef{{Field: "image", Image: v.Image}}
}

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
					"image":       PhotoSchema("Optional headshot of the speaker, cover-cropped into a column beside the quote; narrows the quote column, so keep a photographed quote shorter (alt defaults to the attribution and role)", pullQuoteAltMaxChars),
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
					"image_side":      EnumSchema("left", "right").WithDescription("Which side the headshot sits on; ignored without values.image (default left)").WithDefault("left"),
					"image_width_pct": NumberSchema(pullQuoteImageMinPct, pullQuoteImageMaxPct).WithDescription("Headshot column width as a percentage of the grid (default 25)").WithDefault(pullQuoteImageDefaultPct),
				},
				nil,
			).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Italic quote block with attribution and an optional headshot beside it")
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

	errs = append(errs, validatePatternPhoto(name, "values.image", v.Image, pullQuoteAltMaxChars)...)
	if ovr, ovrOK := overrides.(*PullQuoteOverrides); ovrOK && ovr != nil {
		errs = append(errs, pullQuoteValidateImageOverrides(ovr)...)
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
	// pullQuoteImageDefaultPct / MinPct / MaxPct bound the optional headshot
	// column. The quote is the content here and the face is supporting
	// evidence, so the default is a quarter of the width — narrower than
	// image-text-split's 45%, where the picture is the subject
	// (go-slide-creator-hdpq).
	pullQuoteImageDefaultPct = 25.0
	pullQuoteImageMinPct     = 15
	pullQuoteImageMaxPct     = 40
	// pullQuoteAltMaxChars is the alt budget for the headshot.
	pullQuoteAltMaxChars = 200
)

// pullQuoteValidateImageOverrides checks the two headshot knobs.
func pullQuoteValidateImageOverrides(ovr *PullQuoteOverrides) []error {
	const name = "pull-quote"
	var errs []error
	if ovr.ImageSide != "" && ovr.ImageSide != "left" && ovr.ImageSide != "right" {
		errs = append(errs, newValidationError(name, "overrides.image_side", ErrCodeUnknownEnum,
			fmt.Sprintf("pull-quote: overrides.image_side must be \"left\" or \"right\", got %q", ovr.ImageSide),
			UseOneOfFix("overrides.image_side", []string{"left", "right"})))
	}
	if ovr.ImageWidthPct != 0 && (ovr.ImageWidthPct < pullQuoteImageMinPct || ovr.ImageWidthPct > pullQuoteImageMaxPct) {
		errs = append(errs, errOutOfRange(name, "overrides.image_width_pct", pullQuoteImageMinPct, pullQuoteImageMaxPct, int(ovr.ImageWidthPct)))
	}
	return errs
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

	// An optional headshot takes a column of its own, and the quote block is
	// measured against what is left — measuring against the full width would
	// set a type scale the narrower column cannot hold (go-slide-creator-hdpq).
	photoCell := patternPhotoCell(v.Image, pullQuoteImageAlt(v))
	imgPct := 0.0
	usableW := areaW
	if photoCell != nil {
		photoCell.RowSpan = 2
		imgPct = clampPct(ovr.ImageWidthPct, pullQuoteImageDefaultPct, pullQuoteImageMinPct, pullQuoteImageMaxPct)
		usableW = math.Max(usableW-pullQuoteRuleGapPt, 1)
	}

	rulePct := 0.0
	if accentSide != "none" {
		rulePct = math.Max(pctOf(pullQuoteRulePt, usableW), pullQuoteRuleMinPct)
		usableW = math.Max(usableW-pullQuoteRuleGapPt, 1)
	}
	textW := usableW * (100 - rulePct - imgPct) / 100

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

	// The accent rule and the headshot are columns spanning both rows, not
	// per-cell bars and not a nested grid: two bars would be broken apart by
	// the row gap, and nesting the quote hides it from the readability
	// preflight, which walks a slide's top-level cells (go-slide-creator-hdpq).
	var rule *jsonschema.GridCellInput
	if accentSide != "none" {
		rule = &jsonschema.GridCellInput{
			RowSpan: 2,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(strconv.Quote(accent)),
			},
		}
	}

	cols, quoteCells := pullQuoteColumns(pullQuoteColumnSpec{
		photo:      photoCell,
		imgPct:     imgPct,
		imageSide:  ovr.ImageSide,
		rule:       rule,
		rulePct:    rulePct,
		accentSide: accentSide,
		quote:      quoteCell,
		quotePct:   100 - rulePct - imgPct,
	})
	colsJSON, _ := json.Marshal(cols)

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  pullQuoteRuleGapPt,
		RowGap:  pullQuoteRowGapPt,
		Rows: []jsonschema.GridRowInput{
			{MaxHeight: quoteRowPt, Cells: quoteCells},
			// The attribution row lists only its own cell: the resolver skips
			// the columns the row-spanning rule and headshot already occupy.
			{MinHeight: attrRowPt, MaxHeight: attrRowPt, Cells: []*jsonschema.GridCellInput{attrCell}},
		},
	}
	return grid, nil
}

// pullQuoteColumnSpec is the set of columns one pull-quote grid can hold. A nil
// photo or rule drops that column.
type pullQuoteColumnSpec struct {
	photo      *jsonschema.GridCellInput
	imgPct     float64
	imageSide  string
	rule       *jsonschema.GridCellInput
	rulePct    float64
	accentSide string
	quote      *jsonschema.GridCellInput
	quotePct   float64
}

// pullQuoteColumns lays the headshot, accent rule and quote out left to right,
// each on the side it was asked for, and returns the column widths with the
// matching first-row cells.
func pullQuoteColumns(sp pullQuoteColumnSpec) ([]float64, []*jsonschema.GridCellInput) {
	var widths []float64
	var cells []*jsonschema.GridCellInput
	add := func(pct float64, cell *jsonschema.GridCellInput) {
		if cell == nil {
			return
		}
		widths = append(widths, pct)
		cells = append(cells, cell)
	}
	if sp.imageSide != "right" {
		add(sp.imgPct, sp.photo)
	}
	if sp.accentSide != "right" {
		add(sp.rulePct, sp.rule)
	}
	add(sp.quotePct, sp.quote)
	if sp.accentSide == "right" {
		add(sp.rulePct, sp.rule)
	}
	if sp.imageSide == "right" {
		add(sp.imgPct, sp.photo)
	}
	return widths, cells
}

// pullQuoteImageAlt describes the headshot for a reader who cannot see it: who
// is speaking, which is the only thing the picture adds to the quote.
func pullQuoteImageAlt(v *PullQuoteValues) string {
	attr := strings.TrimSpace(v.Attribution)
	role := strings.TrimSpace(v.Role)
	switch {
	case attr != "" && role != "":
		return attr + ", " + role
	case attr != "":
		return attr
	default:
		return "Photo of the person quoted"
	}
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
