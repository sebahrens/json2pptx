package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// value-chain pattern — horizontal sequence of 4-10 steps with label + description
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&valueChain{})
}

type valueChain struct{}

func (vc *valueChain) Name() string { return "value-chain" }
func (vc *valueChain) Description() string {
	return "Horizontal value chain: equal-width step columns each with a label box on top and a description below; supports per-step highlight"
}
func (vc *valueChain) UseWhen() string {
	return "Porter-style value chain or supply/operations sequence of 4-10 steps where each step needs a short label and a 1-3 line description; prefer process-flow for 3-8 action steps without descriptions, timeline-horizontal when stops are date-based"
}
func (vc *valueChain) NotWhen() string {
	return "Steps have no description beyond the label (use process-flow), stops are calendar milestones (use timeline-horizontal), steps belong to different actors (use swimlane), or chain has fewer than 4 / more than 10 steps"
}
func (vc *valueChain) Version() int      { return 1 }
func (vc *valueChain) CellsHint() string { return "4-10" }
func (vc *valueChain) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "stylish-panels"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (vc *valueChain) SupportsCallout() bool        { return true }
func (vc *valueChain) SupportsInlineMarkdown() bool { return true }

func (vc *valueChain) ExemplarValues() any {
	return &ValueChainValues{
		Steps: []ValueChainStep{
			{Label: "Extraction", Description: "Mining raw materials and managing EPC contracts."},
			{Label: "Processing", Description: "Refining ore into intermediate inputs."},
			{Label: "Manufacturing", Description: "Converting inputs into finished goods.", Highlight: true},
			{Label: "Distribution", Description: "Moving product through wholesale channels."},
			{Label: "Retail", Description: "Reaching end customers via partner stores."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ValueChainStep is a single step in the chain: a short label and a 1-3 line
// description, with an optional highlight flag.
type ValueChainStep struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Highlight   bool   `json:"highlight,omitempty"`
}

// ValueChainValues holds the ordered steps for the value chain (4-10 items)
// plus an optional highlight color (defaults to accent2).
type ValueChainValues struct {
	Steps          []ValueChainStep `json:"steps"`
	HighlightColor string           `json:"highlight_color,omitempty"`
}

// ValueChainOverrides is the standard text overrides plus the step style.
type ValueChainOverrides struct {
	TextOverrides
	// Style is "arrows" (default: the label row is a pentagon followed by
	// interlocking chevrons) or "boxes" (rectangular labels joined by small
	// connector arrows — the look before go-slide-creator-gm4q9).
	Style string `json:"style,omitempty"`
}

// The accepted overrides.style values.
const (
	valueChainStyleArrows = "arrows"
	valueChainStyleBoxes  = "boxes"
)

var valueChainStyles = []string{valueChainStyleArrows, valueChainStyleBoxes}

// valueChainOverridesSchema is the text overrides plus the step style.
func valueChainOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(valueChainStyles...).WithDescription("arrows (default): step labels are interlocking arrows — a pentagon, then chevrons whose tails tuck under the point before them; labels wrap at spaces inside the arrow, and a word no arrow can hold is reported as TEXT_EXCEEDS_SHAPE. boxes: rectangular one-line labels joined by small connector arrows (the earlier look)").WithDefault(valueChainStyleArrows)
	return s
}

// ValueChainCellOverride is the shared per-cell override; indexed by step.
type ValueChainCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (vc *valueChain) NewValues() any       { return &ValueChainValues{} }
func (vc *valueChain) NewOverrides() any    { return &ValueChainOverrides{} }
func (vc *valueChain) NewCellOverride() any { return &ValueChainCellOverride{} }

func (vc *valueChain) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label":       StringSchema(40).WithDescription("Short step label (1-3 words)"),
			"description": StringSchema(180).WithDescription("1-3 line description rendered below the label; at 7/8/9/10 steps about 152/141/140/72 characters, and keep unbroken runs near 167/142/123/108/72 characters at 6/7/8/9/10 steps or add word breaks"),
			"highlight":   BooleanSchema().WithDescription("When true, the label row uses the highlight color (default: the first accent that stands out from the neutral step fill, normally accent1) instead of the neutral dk1 16% tint"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"steps":           ArraySchema(stepSchema, 4, 10).WithDescription("Value-chain steps left-to-right (4-10)"),
			"highlight_color": StringSchema(0).WithDescription("Scheme color used to fill highlighted label rows. Omit it: the default is chosen by measured contrast against the step fill for this template (the first accent clearing 1.6:1), because a fixed slot is invisible on some palettes. An authored colour is honoured, and reported as LOW_CONTRAST_HIGHLIGHT when it does not read as a highlight"),
		},
		[]string{"steps"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      valueChainOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Horizontal value chain of 4-10 equal-width step columns, each with an interlocking arrow label and a description, with per-step highlight support")
}

func (vc *valueChain) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ValueChainValues)
	if !ok || vals == nil {
		return fmt.Errorf("value-chain: values must be *ValueChainValues, got %T", values)
	}

	const name = "value-chain"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*ValueChainOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.Style != "" && !slices.Contains(valueChainStyles, ovr.Style) {
				errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, valueChainStyles))
			}
		}
	}

	if len(vals.Steps) < 4 {
		errs = append(errs, errMinItems(name, "steps", 4, len(vals.Steps), "(hint: use process-flow for 3-8 action steps without descriptions)"))
	}
	if len(vals.Steps) > 10 {
		errs = append(errs, errMaxItems(name, "steps", 10, len(vals.Steps), "(hint: split the chain across two slides)"))
	}

	for i, step := range vals.Steps {
		labelPath := fmt.Sprintf("steps[%d].label", i)
		if strings.TrimSpace(step.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if runeLen(step.Label) > 40 {
			errs = append(errs, errMaxLength(name, labelPath, 40, runeLen(step.Label)))
		}
		if runeLen(step.Description) > 180 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("steps[%d].description", i), 180, runeLen(step.Description)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (vc *valueChain) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ValueChainValues)
	if !ok {
		return nil, fmt.Errorf("value-chain: values must be *ValueChainValues, got %T", values)
	}
	ovr := &ValueChainOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ValueChainOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("value-chain: overrides must be *ValueChainOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	arrows := ovr.Style != valueChainStyleBoxes
	var arrowFit valueChainArrowFit
	var labelSize float64
	if arrows {
		arrowFit = fitValueChainArrows(ctx, vals.Steps, ResolveSize(ovr.HeaderSize, scaleBodyPt))
		labelSize = arrowFit.labelPt
	} else {
		labelSize, _ = fitValueChainLabels(ctx, vals.Steps, ResolveSize(ovr.HeaderSize, scaleBodyPt))
	}
	descSize := ResolveSize(ovr.BodySize, sizeDenseCaptionPt)
	cellAccentMode := ovr.CellAccentMode

	highlightColor := resolveValueChainHighlight(ctx, vals.HighlightColor, baseAccent)

	n := len(vals.Steps)

	labelCells := make([]*jsonschema.GridCellInput, n)
	descCells := make([]*jsonschema.GridCellInput, n)
	for i, step := range vals.Steps {
		tone := valueChainStepTone(ctx, baseAccent)
		if step.Highlight {
			tone = fillTone{Color: highlightColor}
		}
		// Resolved accent governs the connector and per-cell override accent bar,
		// but does NOT override the label fill — that semantic is reserved for
		// highlight vs. neutral-step contrast per the layout spec.
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)

		// Ink by measured contrast at the label's own size: lt1 when it clears
		// WCAG AA (3:1 for large bold labels), else the theme's dark ink.
		// A highlight accent lt1 misses is shaded until it reads rather than
		// taking black type (go-slide-creator-v9tup).
		ink := tonalInk(ctx, tone)
		if step.Highlight {
			tone, ink = accentFillAndInk(ctx, tone, TextContrastThreshold(labelSize, true))
		}
		labelText := buildValueChainLabelText(pptx.ConvertMarkdownEmphasis(step.Label), labelSize, ink)
		labelCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     tone.fillJSON(),
				Text:     labelText,
			},
		}
		if arrows {
			// A pentagon opens the chain and every later step is a chevron
			// whose tail tucks under the point before it. The preset's text
			// rectangle already stops short of the point and the notch, so
			// the label keeps only a small margin inside it.
			labelCell.Shape.Geometry = arrowFit.geometry(i)
			labelCell.Shape.Text = withTextInsets(labelText, valueChainArrowInsetPt)
			labelCell.BleedLeft = arrowFit.bleedPt(i)
		}

		descContent := strings.TrimSpace(step.Description)
		var descShape *jsonschema.ShapeSpecInput
		if descContent == "" {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"bg1"`),
			}
		} else {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"bg1"`),
				Text:     buildValueChainDescriptionText(pptx.ConvertMarkdownEmphasis(descContent), descSize),
			}
		}
		descCell := &jsonschema.GridCellInput{Shape: descShape}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*ValueChainCellOverride); ok2 {
				applyCellTextOverride(labelCell, cellOvr)
				if cellOvr.AccentBar {
					labelCell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    4,
					}
				}
			}
		}

		labelCells[i] = labelCell
		descCells[i] = descCell
	}

	colsJSON, _ := json.Marshal(n)

	gap := ctx.Gap(valueChainGapPt)
	labelRow := jsonschema.GridRowInput{
		Height:    25,
		Cells:     labelCells,
		Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: baseAccent, Width: 1.5},
	}
	if arrows {
		// The arrows say "next" themselves, so no connector is drawn, and the
		// row is as tall as its tallest label needs — not a quarter of the
		// slide (go-slide-creator-gm4q9).
		gap = valueChainArrowGapPt
		rowH := arrowFit.rowHeightPt(ctx.themeFonts(), labelCells)
		for i, c := range labelCells {
			c.Shape.Adjustments = map[string]int64{"adj": arrowFit.adj(i, rowH)}
		}
		labelRow = jsonschema.GridRowInput{MinHeight: rowH, MaxHeight: rowH, Cells: labelCells}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     gap,
		RowGap:  ctx.Gap(4),
		Rows: []jsonschema.GridRowInput{
			labelRow,
			{
				// Size the description row to its own text. Uncapped it took
				// the remaining 75% of the content area and centred the
				// descriptions inside it, so a full-width empty stripe ran
				// between the step boxes and their descriptions and the bottom
				// third of the slide was blank (go-slide-creator-pr3g).
				MaxHeight: valueChainDescRowHeightPt(ctx, descCells, n, gap),
				Cells:     descCells,
			},
		},
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

// valueChainDescriptionMax is the schema maximum for a step description.
const valueChainDescriptionMax = 180

// valueChainDescriptionBudgets returns the worded and unbroken-word
// description budgets for a step count.
func valueChainDescriptionBudgets(steps int) (words, wide int) {
	switch {
	case steps <= 5:
		return valueChainDescriptionMax, valueChainDescriptionMax
	case steps == 6:
		return valueChainDescriptionMax, 167
	case steps == 7:
		return 152, 142
	case steps == 8:
		return 141, 123
	case steps == 9:
		return 140, 108
	default:
		return 72, 72
	}
}

// valueChainGapPt is the gap between step columns.
const valueChainGapPt = 8.0

// valueChainMinLabelPt is the floor a step label may shrink to. It is the
// renderer's own floor, not a taste judgement: the shape_grid renderer raises
// any authored size below shapegrid.MinTextSizePt back to 12pt, so a fit that
// promised 9pt would be silently overridden and the label would break anyway —
// which is exactly what a first cut at this did (go-slide-creator-vo0j1).
const valueChainMinLabelPt = shapegrid.MinTextSizePt

// fitValueChainLabels shrinks the step-label size until every label fits its
// own column on one line, and reports the labels that still cannot.
//
// The size was fixed at 12pt regardless of the step count, so a ten-step chain
// rendered "Manufactur / ing" and "Decommissi / oning end- / of-life" — the
// bundled examples/value-chain.json shipped it. One shared size keeps the row
// even; the floor is where the pattern stops and the author has to act
// (go-slide-creator-vo0j1).
func fitValueChainLabels(ctx ExpandContext, steps []ValueChainStep, labelPt float64) (float64, []string) {
	contentW, _ := contentAreaPt(ctx)
	textW := equalColumnWidthPt(contentW, len(steps), ctx.Gap(valueChainGapPt)) - 2*defaultShapeInsetLRPt
	if textW <= 0 || len(steps) == 0 {
		return labelPt, nil
	}
	font := ctx.Theme.BodyFont

	size := labelPt
	for _, step := range steps {
		if s := fitSingleLineSize(step.Label, font, true, size, valueChainMinLabelPt, textW); s < size {
			size = s
		}
	}
	var unfit []string
	for _, step := range steps {
		if strings.TrimSpace(step.Label) == "" {
			continue
		}
		if measuredLines(step.Label, font, true, size, textW) > 1 {
			unfit = append(unfit, step.Label)
		}
	}
	return size, unfit
}

// Arrow-style geometry (go-slide-creator-gm4q9).
const (
	// valueChainArrowGapPt is the column gap of the arrow style, and the
	// width of the slanted gap between one arrow's point and the next arrow's
	// notch. It is a hairline between interlocking shapes, so it is not
	// scaled with the template gutter.
	valueChainArrowGapPt = 4.0
	// valueChainArrowInsetPt is the label margin inside the arrow's own text
	// rectangle, which already excludes the point and the notch.
	valueChainArrowInsetPt = 4.0
	// valueChainArrowAspect, MinHPt and MaxHPt size the arrow row from the
	// step width: a wide step gets a taller arrow, a ten-step chain a
	// shallower one, and a label that wraps grows the row past the cap.
	valueChainArrowAspect = 0.33
	valueChainArrowMinHPt = 36.0
	valueChainArrowMaxHPt = 66.0
	// valueChainNotchFrac is the deepest point / notch as a share of the
	// arrow height; valueChainMinNotchPt is the shallowest the fit goes to
	// before the label shrinks. Below it the shape reads as a box.
	valueChainNotchFrac  = 0.32
	valueChainMinNotchPt = 6.0
)

// valueChainArrowFit is the geometry the arrow row renders at: one point
// depth and one label size for the whole chain.
type valueChainArrowFit struct {
	colWPt  float64  // step column width
	rowHPt  float64  // arrow height before any label needs more
	notchPt float64  // depth of every point and notch
	labelPt float64  // label size
	unfit   []string // labels with a word no arrow can hold on one line
}

// geometry is step i's preset: a pentagon first, chevrons after it.
func (f valueChainArrowFit) geometry(i int) string {
	if i == 0 {
		return "homePlate"
	}
	return "chevron"
}

// bleedPt is how far step i's shape reaches left of its column: far enough
// for its notch to sit valueChainArrowGapPt off the point before it.
func (f valueChainArrowFit) bleedPt(i int) float64 {
	if i == 0 {
		return 0
	}
	return f.notchPt
}

// textRectPt is the width of step i's preset text rectangle
// (pptx.PresetTextRectSize): a pentagon gives up half its point, a chevron
// its point and its notch.
func (f valueChainArrowFit) textRectPt(i int) float64 {
	if i == 0 {
		return f.colWPt - f.notchPt/2
	}
	return f.colWPt + f.bleedPt(i) - 2*f.notchPt
}

// adj is step i's preset adjustment for a row rowHPt tall: the point depth
// as a share (x100000) of the shape's shorter side.
func (f valueChainArrowFit) adj(i int, rowHPt float64) int64 {
	short := math.Min(f.colWPt+f.bleedPt(i), rowHPt)
	if short <= 0 {
		return 0
	}
	return int64(math.Round(f.notchPt / short * 100000))
}

// rowHeightPt is the arrow row's height: the base height, or the written fit
// of the tallest label inside its own text rectangle when a label wraps.
func (f valueChainArrowFit) rowHeightPt(fonts pptx.ThemeFonts, cells []*jsonschema.GridCellInput) float64 {
	h := f.rowHPt
	for i, c := range cells {
		if c == nil || c.Shape == nil {
			continue
		}
		h = math.Max(h, writtenFitHeightPt(fonts, c.Shape.Text, f.textRectPt(i), 0))
	}
	return math.Ceil(h)
}

// unfitLabels returns the labels with a word wider than their arrow's text
// rectangle at the fit's point depth and label size — the words a renderer
// breaks mid-word. The word is measured as the writer measures it
// (pptx.WordLineNeedEMU), against the bare rectangle: the writer gives a
// word the label margin back before it lets it break
// (pptx.EffectiveTextInsets).
func (f valueChainArrowFit) unfitLabels(steps []ValueChainStep, font string) []string {
	var unfit []string
	for i, step := range steps {
		availPt := f.textRectPt(i)
		for _, word := range strings.Fields(step.Label) {
			if utf8.RuneCountInString(word) < 2 {
				continue
			}
			need, ok := pptx.WordLineNeedEMU(word, font, f.labelPt, true, 0)
			if !ok {
				if measuredLines(word, font, true, f.labelPt, availPt) <= 1 {
					continue
				}
			} else if float64(need) <= availPt*sizingEMUPerPt {
				continue
			}
			unfit = append(unfit, step.Label)
			break
		}
	}
	return unfit
}

// fitValueChainArrows finds the deepest point and the largest label size at
// which every label word stays whole inside its arrow, and reports the
// labels that cannot. The point gives way first — the arrow reads the same a
// little blunter — and the label only shrinks to the readable floor.
func fitValueChainArrows(ctx ExpandContext, steps []ValueChainStep, labelPt float64) valueChainArrowFit {
	contentW, _ := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, len(steps), valueChainArrowGapPt)
	rowH := clampPt(math.Round(colW*valueChainArrowAspect), valueChainArrowMinHPt, valueChainArrowMaxHPt)
	fit := valueChainArrowFit{colWPt: colW, rowHPt: rowH, labelPt: labelPt}
	font := ctx.Theme.BodyFont

	deepest := math.Max(math.Round(rowH*valueChainNotchFrac), valueChainMinNotchPt)
	floor := math.Min(labelPt, valueChainMinLabelPt)
	for size := labelPt; size >= floor; size-- {
		fit.labelPt = size
		for d := deepest; d >= valueChainMinNotchPt; d-- {
			fit.notchPt = d
			if len(fit.unfitLabels(steps, font)) == 0 {
				return fit
			}
		}
	}
	fit.unfit = fit.unfitLabels(steps, font)
	return fit
}

// valueChainDescRowHeightPt is the height the description row needs for its
// tallest description at the step column width.
func valueChainDescRowHeightPt(ctx ExpandContext, cells []*jsonschema.GridCellInput, cols int, gapPt float64) float64 {
	contentW, _ := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, cols, gapPt)
	textW := colW - 2*defaultShapeInsetLRPt
	if textW <= 0 {
		return 0
	}
	font := ctx.Theme.BodyFont
	h := 0.0
	for _, c := range cells {
		if c == nil || c.Shape == nil {
			continue
		}
		h = math.Max(h, shapeTextHeightPt(font, c.Shape.Text, textW))
	}
	if h == 0 {
		return 0
	}
	// The row shares one autofit shrink, so every description must fit
	// unscaled by the writer's own measure, not only the theme-font model.
	h = math.Round(h + 2*defaultShapeInsetTBPt)
	for _, c := range cells {
		if c != nil && c.Shape != nil {
			h = math.Max(h, writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, colW, h))
		}
	}
	return h
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type valueChainParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type valueChainTextObj struct {
	Paragraphs    []valueChainParagraph `json:"paragraphs"`
	Align         string                `json:"align"`
	VerticalAlign string                `json:"vertical_align"`
}

// valueChainHighlightCandidates is the order the default highlight is chosen
// in: the brand accent first, so the highlighted step reads as "this one" in
// the template's own primary colour, and the rest only as the measurement
// forces it. dk2 is the last resort — a dark step is always distinct from the
// light neutral chain and is still in palette.
var valueChainHighlightCandidates = []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "dk2"}

// valueChainDefaultHighlight is the default for a context with no theme to
// measure against (unit tests, a pattern expanded without a template): the
// primary accent, the slide's one emphasised element (go-slide-creator-8xsj3).
const valueChainDefaultHighlight = "accent1"

// resolveValueChainHighlight picks the fill for a highlighted step. An authored
// highlight_color is always honoured — the author may know something the
// measurement does not — but the DEFAULT is chosen by measured contrast against
// the chain's own fill, because a fixed slot is only as visible as the gap
// between two slots in whatever template the deck lands on: accent2 on dk2 is
// 3.21 on midnight-blue and 1.48 on warm-coral, where the highlight vanished
// (go-slide-creator-ah5s).
func resolveValueChainHighlight(ctx ExpandContext, authored, baseAccent string) string {
	if authored != "" {
		return authored
	}
	// The template's primary fill leads, so the highlight is the same slot
	// every other pattern defaults to (go-slide-creator-2mia4).
	candidates := append([]string{ctx.DefaultAccent()}, valueChainHighlightCandidates...)
	// The base accent leads: the steps are its tint.
	step := valueChainStepTone(ctx, baseAccent)
	candidates = append([]string{baseAccent}, candidates...)
	if pick, ok := pickDistinctFill(ctx, step, valueChainHighlightMin, candidates...); ok {
		return pick
	}
	return valueChainDefaultHighlight
}

// PostExpandWarnings reports an authored highlight_color that does not read as
// a highlight against the chain's own fill. The step still renders — the author
// asked for that colour — but nothing else would say that the slide's one
// semantic signal is invisible.
func (vc *valueChain) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ValueChainValues)
	if !ok || v == nil {
		return nil
	}
	var out []string

	// A label the chain cannot fit at its readable floor WILL break mid-word.
	// The strip measured it, so the finding blocks rather than advises
	// (go-slide-creator-vo0j1, go-slide-creator-rxkt).
	ovr, _ := overrides.(*ValueChainOverrides)
	if ovr == nil {
		ovr = &ValueChainOverrides{}
	}
	if ovr.Style != valueChainStyleBoxes {
		if fit := fitValueChainArrows(ctx, v.Steps, ResolveSize(ovr.HeaderSize, scaleBodyPt)); len(fit.unfit) > 0 {
			noun, verb, pronoun := "label", "has", "it"
			if len(fit.unfit) > 1 {
				noun, verb, pronoun = "labels", "have", "them"
			}
			out = append(out, fmt.Sprintf(
				"%s: value-chain step %s %s %s a word wider than the step arrow at %d steps even at %.0fpt — the renderer breaks %s mid-word; shorten %s or use fewer steps",
				ErrCodeTextExceedsShape, noun, listFirstN(fit.unfit, 3), verb, len(v.Steps), fit.labelPt, pronoun, pronoun))
		}
	} else if size, unfit := fitValueChainLabels(ctx, v.Steps, ResolveSize(ovr.HeaderSize, scaleBodyPt)); len(unfit) > 0 {
		noun, verb, pronoun := "label", "does", "it"
		if len(unfit) > 1 {
			noun, verb, pronoun = "labels", "do", "them"
		}
		out = append(out, fmt.Sprintf(
			"%s: value-chain step %s %s %s not fit on one line at %d steps even at %.0fpt — the renderer breaks %s mid-word; shorten %s or use fewer steps",
			ErrCodeTextExceedsShape, noun, listFirstN(unfit, 3), verb, len(v.Steps), size, pronoun, pronoun))
	}
	// Measured against the written size (no run stored below its role floor)
	// on every shipped template (go-slide-creator-n1muf): worded descriptions
	// and unbroken words each have a per-step-count budget.
	if words, wide := valueChainDescriptionBudgets(len(v.Steps)); words < valueChainDescriptionMax {
		for i, step := range v.Steps {
			longest := 0
			for _, word := range strings.Fields(step.Description) {
				longest = max(longest, runeLen(word))
			}
			switch {
			case longest > wide:
				out = append(out, fmt.Sprintf("%s: value-chain steps[%d].description contains a %d-character unbroken word; %d steps hold about %d wide characters per description — add a word break, shorten the copy, or use fewer steps", ErrCodeBodyTooLong, i, longest, len(v.Steps), wide))
			case runeLen(step.Description) > words:
				out = append(out, fmt.Sprintf("%s: value-chain steps[%d].description is %d characters; %d steps hold about %d readable characters per description — shorten the copy or use fewer steps", ErrCodeBodyTooLong, i, runeLen(step.Description), len(v.Steps), words))
			}
		}
	}

	if v.HighlightColor == "" {
		return out
	}
	highlighted := false
	for _, step := range v.Steps {
		if step.Highlight {
			highlighted = true
			break
		}
	}
	if !highlighted {
		return out
	}
	// Measured against the fill the steps are drawn in: the tint of the
	// accent the overrides resolve to.
	baseAccent := ctx.DefaultAccent()
	if ovr, isOvr := overrides.(*ValueChainOverrides); isOvr && ovr != nil {
		baseAccent = ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	}
	ratio, ok := fillContrast(ctx, valueChainStepTone(ctx, baseAccent), fillTone{Color: v.HighlightColor})
	if !ok || ratio >= valueChainHighlightMin {
		return out
	}
	return append(out, fmt.Sprintf(
		"%s: value-chain highlight_color %q reads at %.2f:1 against the step fill (%s) — below %.1f:1 the highlighted step is not distinguishable from its neighbours; omit highlight_color to let the engine pick an accent that clears the bar",
		ErrCodeLowContrastHighlight, v.HighlightColor, ratio, valueChainStepFillName(ctx, baseAccent), valueChainHighlightMin))
}

// valueChainStepTone is the default (non-highlighted) step fill: the content
// swatch of the tonal system, the accent's Lighter 80% (tonalContent). The
// steps ARE the content, so they take the accent's family rather than a row
// of mid-grey boxes (go-slide-creator-x5m8f); the one highlighted step is the
// solid accent.
func valueChainStepTone(ctx ExpandContext, accent string) fillTone {
	return tonalContent(ctx, accent)
}

// valueChainLabelFillName names valueChainStepTone in findings.
const valueChainLabelFillName = "the accent's Lighter 80% swatch"

// valueChainStepFillName names the step fill a finding measured against: the
// accent's swatch, or the neutral that stands in for it on a template whose
// accent tint collides.
func valueChainStepFillName(ctx ExpandContext, accent string) string {
	if tonalAccentCollides(ctx, accent) {
		return "dk1 at 16%"
	}
	return valueChainLabelFillName
}

// valueChainHighlightMin is the luminance contrast the highlight must clear
// against the step fill: the bar the tonal system sets between the solid
// accent and its own content swatch (tonalEmphasisMin), which every shipped
// template's primary accent clears.
const valueChainHighlightMin = tonalEmphasisMin

func buildValueChainLabelText(label string, size float64, color string) json.RawMessage {
	textObj := valueChainTextObj{
		Paragraphs: []valueChainParagraph{
			{Content: label, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildValueChainDescriptionText(description string, size float64) json.RawMessage {
	textObj := valueChainTextObj{
		Paragraphs: []valueChainParagraph{
			{Content: description, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}
