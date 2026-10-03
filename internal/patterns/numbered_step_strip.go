package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// ---------------------------------------------------------------------------
// numbered-step-strip pattern — ordered steps WITHOUT flowchart/diamond
// semantics, with a built-in detail zone. Three render styles:
//
//   chevron     — 3-6 connected chevrons in a single row (optional description row)
//   stacked-box — 3-7 table-like rows: narrow colored number/tip column + no-border body
//   toc         — 3-7 high-polish numbered agenda rows (badge + title, optional body)
//
// stacked-box and toc steps may carry an optional icon, drawn in its own
// column between the number badge and the label.
//
// The scorecard principle applies: a compact primary lane carries the ordinal
// label, an optional detail zone carries the per-step explanation. Unlike
// process-flow, this pattern never emits decision diamonds — it is for ordered
// sequences and table-of-contents lists, not branching workflows.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&numberedStepStrip{})
}

type numberedStepStrip struct{}

func (n *numberedStepStrip) Name() string { return "numbered-step-strip" }
func (n *numberedStepStrip) Description() string {
	return "Ordered numbered steps without flowchart diamonds, in chevron / stacked-box / toc styles, each with an optional per-step detail zone"
}
func (n *numberedStepStrip) UseWhen() string {
	return "Ordered steps or a table of contents that need numbering and an optional short explanation per item but NOT decision branching (3-7 steps; chevron holds at most 6); use stacked-box for a scorecard look, chevron for a compact ribbon, toc to replace a plain agenda"
}
func (n *numberedStepStrip) NotWhen() string {
	return "The flow has decision points/branches (use process-flow with type:decision), steps belong to different actors (use swimlane), stops are calendar dates (use timeline-horizontal), or items are unordered categories (use icon-row or card-grid)"
}
func (n *numberedStepStrip) Version() int      { return 1 }
func (n *numberedStepStrip) CellsHint() string { return "3-7" }
func (n *numberedStepStrip) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"open", "frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "stat-hero"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (n *numberedStepStrip) SupportsCallout() bool        { return true }
func (n *numberedStepStrip) SupportsInlineMarkdown() bool { return true }

func (n *numberedStepStrip) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 1},
		{Columns: 4, Rows: 1},
		{Columns: 5, Rows: 1},
		{Columns: 6, Rows: 1},
	}
}

func (n *numberedStepStrip) ExemplarValues() any {
	return &NumberedStepStripValues{
		Style: "stacked-box",
		Steps: []NumberedStepStripStep{
			{Label: "Discover", Body: "Map the current state and surface the constraints."},
			{Label: "Design", Body: "Shape the target operating model and the roadmap."},
			{Label: "Deliver", Body: "Stand up the capability and migrate the workload."},
			{Label: "Sustain", Body: "Embed the run model and track the value captured."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// numberedStepStripStyles enumerates the supported render styles.
const (
	numberedStepStripChevron    = "chevron"
	numberedStepStripStackedBox = "stacked-box"
	numberedStepStripTOC        = "toc"
)

// NumberedStepStripStep is a single ordered step.
type NumberedStepStripStep struct {
	Label       string `json:"label"`
	Body        string `json:"body,omitempty"`        // optional 1-3 line explanation (detail zone)
	Recommended bool   `json:"recommended,omitempty"` // highlight a selected step in stacked-box style
	Number      string `json:"number,omitempty"`      // optional ordinal override (e.g. "01", "A"); defaults to %02d
	TipColor    string `json:"tip_color,omitempty"`   // optional scheme color for this step's number/tip lane
	// Icon is drawn between the number badge and the label (stacked-box and
	// toc styles). Bundled-name string shorthand or {name|path|url|svg_data}.
	Icon *IconRef `json:"icon,omitempty"`
}

// Step-count limits: stacked-box and toc rows stack vertically and hold 7;
// chevrons share one row and hold 6.
const (
	numberedStepStripMaxSteps        = 7
	numberedStepStripMaxChevronSteps = 6
	// numberedStepIconMaxPt caps the rendered icon so it stays a glyph beside
	// the label rather than a picture filling a tall row.
	numberedStepIconMaxPt = 26.0
)

// NumberedStepStripValues holds the render style and the ordered steps.
type NumberedStepStripValues struct {
	Style string                  `json:"style,omitempty"` // chevron | stacked-box | toc (default stacked-box)
	Steps []NumberedStepStripStep `json:"steps"`
}

// NumberedStepStripOverrides is the standard text overrides plus the number
// lane treatment.
type NumberedStepStripOverrides struct {
	TextOverrides
	// Style is the stacked-box / toc number treatment (not the render style,
	// which is values.style): "tinted" (default: accent numerals on a neutral
	// lane, unfilled toc numerals) or "solid" (accent-filled number lanes and
	// toc badges with white numerals; legacy look).
	Style string `json:"style,omitempty"`
}

// numberedStepLaneStyles are the accepted overrides.style values.
var numberedStepLaneStyles = []string{"tinted", "solid"}

// numberedStepStripOverridesSchema is the text overrides schema plus style.
func numberedStepStripOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(numberedStepLaneStyles...).WithDescription("Number treatment for stacked-box and toc (the render style is values.style). tinted (default): accent numerals on a neutral-tint lane (stacked-box) or unfilled (toc), so the column of numbers does not read as a row of accent blocks. solid: accent-filled number lanes / badges with white numerals (legacy look)").WithDefault("tinted")
	return s
}

// NumberedStepStripCellOverride is the shared per-cell override; indexed by step.
type NumberedStepStripCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) NewValues() any       { return &NumberedStepStripValues{} }
func (n *numberedStepStrip) NewOverrides() any    { return &NumberedStepStripOverrides{} }
func (n *numberedStepStrip) NewCellOverride() any { return &NumberedStepStripCellOverride{} }

func (n *numberedStepStrip) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label":       StringSchema(60).WithDescription("Short ordinal step label. Five-step chevrons hold about 41 readable characters per label at default size and six-step chevrons about 16; all other supported styles/counts hold 60"),
			"body":        StringSchema(180).WithDescription("Optional 1-3 line explanation rendered in the detail zone; about 177 readable characters in a chevron detail zone; stacked-box rows hold about 117 at five steps, toc rows about 117 at four and 40 at five; six or seven stacked-box / toc rows hold no body"),
			"recommended": BooleanSchema().WithDescription("Highlight this row with an accent fill and Recommended badge (stacked-box style)"),
			"number":      StringSchema(6).WithDescription("Optional ordinal override (e.g. \"01\", \"A\"); defaults to the 1-based index"),
			"tip_color":   StringSchema(0).WithDescription("Optional scheme color for this step's number / tip lane (default: rotating accent)"),
			"icon":        IconRefSchema("Optional icon drawn between the number badge and the label (stacked-box and toc styles only; rejected for chevron). Bundled name string shorthand or {name|path|url|svg_data, fill?, alt?} object."),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"style": EnumSchema(numberedStepStripChevron, numberedStepStripStackedBox, numberedStepStripTOC).
				WithDescription("Render style: chevron (connected ribbon), stacked-box (scorecard rows), or toc (numbered agenda)").
				WithDefault(numberedStepStripStackedBox),
			"steps": ArraySchema(stepSchema, 3, numberedStepStripMaxSteps).WithDescription("Ordered steps: 3-7 for stacked-box and toc, 3-6 for chevron"),
		},
		[]string{"steps"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      numberedStepStripOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Ordered numbered steps without flowchart diamonds, in chevron / stacked-box / toc styles, each with an optional per-step detail zone")
}

func (n *numberedStepStrip) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*NumberedStepStripValues)
	if !ok || vals == nil {
		return fmt.Errorf("numbered-step-strip: values must be *NumberedStepStripValues, got %T", values)
	}

	const name = "numbered-step-strip"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*NumberedStepStripOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.Style != "" && !slices.Contains(numberedStepLaneStyles, ovr.Style) {
				errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, numberedStepLaneStyles))
			}
		}
	}

	if vals.Style != "" &&
		vals.Style != numberedStepStripChevron &&
		vals.Style != numberedStepStripStackedBox &&
		vals.Style != numberedStepStripTOC {
		errs = append(errs, newValidationError(name, "style", ErrCodeUnknownEnum,
			fmt.Sprintf("numbered-step-strip: style must be %q, %q, or %q, got %q",
				numberedStepStripChevron, numberedStepStripStackedBox, numberedStepStripTOC, vals.Style),
			UseOneOfFix("style", []string{numberedStepStripChevron, numberedStepStripStackedBox, numberedStepStripTOC})))
	}

	if len(vals.Steps) < 3 {
		errs = append(errs, errMinItems(name, "steps", 3, len(vals.Steps), ""))
	}
	if len(vals.Steps) > numberedStepStripMaxSteps {
		errs = append(errs, errMaxItems(name, "steps", numberedStepStripMaxSteps, len(vals.Steps), "(hint: split across two slides or use agenda for a longer list)"))
	} else if vals.Style == numberedStepStripChevron && len(vals.Steps) > numberedStepStripMaxChevronSteps {
		errs = append(errs, errMaxItems(name, "steps", numberedStepStripMaxChevronSteps, len(vals.Steps),
			"(hint: chevron style holds at most 6 steps; use style \"stacked-box\" or \"toc\" for 7)"))
	}

	for i, step := range vals.Steps {
		errs = append(errs, validateNumberedStep(name, vals.Style, i, step)...)
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// validateNumberedStep checks one step's fields against the strip's style.
func validateNumberedStep(name, style string, i int, step NumberedStepStripStep) []error {
	var errs []error
	if step.Recommended && style != "" && style != numberedStepStripStackedBox {
		errs = append(errs, newValidationError(name, fmt.Sprintf("steps[%d].recommended", i), ErrCodeInvalidShape,
			"recommended is supported only by stacked-box style", nil))
	}
	labelPath := fmt.Sprintf("steps[%d].label", i)
	if strings.TrimSpace(step.Label) == "" {
		errs = append(errs, errRequired(name, labelPath))
	} else if runeLen(step.Label) > 60 {
		errs = append(errs, errMaxLength(name, labelPath, 60, runeLen(step.Label)))
	}
	if runeLen(step.Body) > 180 {
		errs = append(errs, errMaxLength(name, fmt.Sprintf("steps[%d].body", i), 180, runeLen(step.Body)))
	}
	if runeLen(step.Number) > 6 {
		errs = append(errs, errMaxLength(name, fmt.Sprintf("steps[%d].number", i), 6, runeLen(step.Number)))
	}
	if step.Icon != nil && !step.Icon.IsEmpty() {
		iconPath := fmt.Sprintf("steps[%d].icon", i)
		if style == numberedStepStripChevron {
			errs = append(errs, newValidationError(name, iconPath, ErrCodeInvalidShape,
				"icon is supported only by stacked-box and toc styles", nil))
		} else {
			errs = append(errs, validateIconRef(name, iconPath, *step.Icon)...)
		}
	}
	return errs
}

// PostExpandWarnings reports the chevron labels that still wrap mid-word after
// the strip has given up all the notch depth it can and shrunk the label to the
// readable floor. At that point the label itself is too long for the number of
// steps, and only the author can fix it — by shortening the word or dropping a
// step (go-slide-creator-e97v).
func (n *numberedStepStrip) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	vals, ok := values.(*NumberedStepStripValues)
	if !ok || vals == nil {
		return nil
	}
	if vals.Style != numberedStepStripChevron {
		warnings := numberedStepRowWarnings(vals)
		ovr, _ := overrides.(*NumberedStepStripOverrides)
		if ovr == nil {
			ovr = &NumberedStepStripOverrides{}
		}
		style := numberedStepStripStackedBox
		if vals.Style == numberedStepStripTOC {
			style = numberedStepStripTOC
		}
		if w := n.numberedStepRowsWarning(ctx, vals, ovr, style); w != "" {
			warnings = append(warnings, w)
		}
		return warnings
	}
	var warnings []string
	for i, step := range vals.Steps {
		if budget, length := numberedStepChevronLabelBudget(len(vals.Steps)), runeLen(step.Label); budget > 0 && length > budget {
			warnings = append(warnings, fmt.Sprintf("%s: numbered-step-strip steps[%d].label is %d characters; a %d-step chevron holds about %d readable label characters — shorten the label or use fewer steps", ErrCodeBodyTooLong, i, length, len(vals.Steps), budget))
		}
		if length := runeLen(strings.TrimSpace(step.Body)); length > numberedStepChevronBodyBudget {
			warnings = append(warnings, fmt.Sprintf("%s: numbered-step-strip steps[%d].body is %d characters; a chevron detail zone holds about %d readable characters — shorten the body", ErrCodeBodyTooLong, i, length, numberedStepChevronBodyBudget))
		}
	}
	ovr, _ := overrides.(*NumberedStepStripOverrides)
	if ovr == nil {
		ovr = &NumberedStepStripOverrides{}
	}
	fit := fitChevronLabels(ctx, vals, chevronStripGeometry(ctx, len(vals.Steps)), ResolveSize(ovr.HeaderSize, sizeLabelPt))
	if len(fit.unfit) == 0 {
		return warnings
	}
	noun, verb, pronoun := "label", "does", "it"
	if len(fit.unfit) > 1 {
		noun, verb, pronoun = "labels", "do", "them"
	}
	// TEXT_EXCEEDS_SHAPE, not BODY_TOO_LONG: the strip has MEASURED that the
	// label cannot fit its chevron at the readable floor, so the mid-word break
	// is certain rather than predicted. That certainty is what earns the
	// blocking action the geometry detector's estimate cannot claim
	// (go-slide-creator-rxkt).
	return append(warnings, fmt.Sprintf(
		"%s: numbered-step-strip chevron %s %s %s not fit on one line at %d steps even at %.0fpt — the renderer breaks %s mid-word; shorten %s or use fewer steps",
		ErrCodeTextExceedsShape, noun, listFirstN(fit.unfit, 3), verb,
		len(vals.Steps), fit.labelPt, pronoun, pronoun))
}

// Readable targets measured by TestNumberedStepBudgetProbe against the
// written size (no run stored below its role floor) on every shipped template
// at default sizes (go-slide-creator-n1muf), with every shape keeping the
// uniform 0.5 cm text margin: five- and six-step chevron labels, any chevron
// detail body, and stacked-box / toc bodies from four (toc) or five
// (stacked-box) rows up. Six or seven stacked rows hold a label and no body.
const (
	numberedStepFiveChevronLabelBudget = 41
	numberedStepSixChevronLabelBudget  = 16
	numberedStepChevronBodyBudget      = 177
	numberedStepFiveStackedBodyBudget  = 117
	numberedStepFourTOCBodyBudget      = 117
	numberedStepFiveTOCBodyBudget      = 40
)

// numberedStepChevronLabelBudget is the readable label length of a chevron at
// a step count, or 0 when the schema maximum holds.
func numberedStepChevronLabelBudget(steps int) int {
	switch steps {
	case 5:
		return numberedStepFiveChevronLabelBudget
	case 6:
		return numberedStepSixChevronLabelBudget
	}
	return 0
}

// numberedStepRowBodyBudget is the readable body length per step of a
// stacked-box / toc strip at a step count: -1 when the schema maximum holds,
// 0 when the rows hold no body at all.
func numberedStepRowBodyBudget(style string, steps int) int {
	switch {
	case steps >= numberedStepStripMaxSteps-1:
		return 0
	case style == numberedStepStripTOC && steps == 5:
		return numberedStepFiveTOCBodyBudget
	case style == numberedStepStripTOC && steps == 4:
		return numberedStepFourTOCBodyBudget
	case steps == 5:
		return numberedStepFiveStackedBodyBudget
	}
	return -1
}

// numberedStepRowWarnings reports stacked-box / toc bodies past the readable
// target for the step count.
func numberedStepRowWarnings(vals *NumberedStepStripValues) []string {
	style := numberedStepStripStackedBox
	if vals.Style == numberedStepStripTOC {
		style = numberedStepStripTOC
	}
	budget := numberedStepRowBodyBudget(style, len(vals.Steps))
	if budget < 0 {
		return nil
	}
	var warnings []string
	for i, step := range vals.Steps {
		length := runeLen(strings.TrimSpace(step.Body))
		switch {
		case length == 0 || length <= budget:
		case budget == 0:
			warnings = append(warnings, fmt.Sprintf("%s: numbered-step-strip steps[%d].body is %d characters; %d %s rows hold a label and no readable body — drop the bodies or use fewer steps", ErrCodeBodyTooLong, i, length, len(vals.Steps), style))
		default:
			warnings = append(warnings, fmt.Sprintf("%s: numbered-step-strip steps[%d].body is %d characters; %d %s rows hold about %d readable body characters per step — shorten the body or use fewer steps", ErrCodeBodyTooLong, i, length, len(vals.Steps), style, budget))
		}
	}
	return warnings
}

func (n *numberedStepStrip) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*NumberedStepStripValues)
	if !ok {
		return nil, fmt.Errorf("numbered-step-strip: values must be *NumberedStepStripValues, got %T", values)
	}
	ovr := &NumberedStepStripOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*NumberedStepStripOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("numbered-step-strip: overrides must be *NumberedStepStripOverrides, got %T", overrides)
		}
	}

	style := vals.Style
	if style == "" {
		style = numberedStepStripStackedBox
	}

	switch style {
	case numberedStepStripChevron:
		return n.expandChevron(ctx, vals, ovr, cellOverrides), nil
	case numberedStepStripTOC:
		return n.expandTOC(ctx, vals, ovr, cellOverrides), nil
	default:
		return n.expandStackedBox(ctx, vals, ovr, cellOverrides), nil
	}
}

// hasBody reports whether any step carries detail-zone text.
func (n *numberedStepStrip) hasBody(vals *NumberedStepStripValues) bool {
	for _, s := range vals.Steps {
		if strings.TrimSpace(s.Body) != "" {
			return true
		}
	}
	return false
}

// stepNumber returns the explicit ordinal override or the zero-padded index.
func stepNumber(step NumberedStepStripStep, idx int) string {
	if strings.TrimSpace(step.Number) != "" {
		return step.Number
	}
	return fmt.Sprintf("%02d", idx+1)
}

// ---------------------------------------------------------------------------
// chevron style — single row of connected chevrons; optional description row.
// When no step carries body text the strip is capped at ~35% content height.
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) expandChevron(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.HeaderSize, sizeLabelPt)
	descSize := ResolveSize(ovr.BodySize, chevronDescDefaultSize)
	cellAccentMode := ovr.CellAccentMode

	withBody := n.hasBody(vals)
	count := len(vals.Steps)

	geo := chevronStripGeometry(ctx, count)
	fit := fitChevronLabels(ctx, vals, geo, labelSize)
	labelSize = fit.labelPt

	chevronCells := make([]*jsonschema.GridCellInput, count)
	descCells := make([]*jsonschema.GridCellInput, count)
	maxDescLines := 0
	for i, step := range vals.Steps {
		fill := step.TipColor
		if fill == "" {
			fill = ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		}
		textColor := readableTextOn(ctx, fillTone{Color: fill}, "lt1")
		text := buildChevronLabelText(stepNumber(step, i), pptx.ConvertMarkdownEmphasis(step.Label), labelSize, textColor)
		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "chevron",
				Fill:        json.RawMessage(fmt.Sprintf(`"%s"`, fill)),
				Text:        text,
				Adjustments: map[string]int64{"adj": fit.adj},
			},
		}
		applyNumberedStepOverride(cell, cellOverrides, i, baseAccent)
		chevronCells[i] = cell

		body := strings.TrimSpace(step.Body)
		descShape := &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}
		if body != "" {
			descShape.Text = buildChevronDescText(pptx.ConvertMarkdownEmphasis(body), descSize)
			if lines := estimateWrappedLines(body, descSize, geo.stepWPt-2*chevronDescInsetPt); lines > maxDescLines {
				maxDescLines = lines
			}
		}
		descCells[i] = &jsonschema.GridCellInput{Shape: descShape}
	}

	colsJSON, _ := json.Marshal(count)

	// Height budget: the chevron row is capped so every chevron is at least
	// twice as wide as it is tall, and the detail row is sized to its text so
	// descriptions sit directly under the chevrons rather than floating in a
	// flex row that fills the rest of the slide.
	totalPt := geo.chevHPt
	chevRow := jsonschema.GridRowInput{Cells: chevronCells}
	rows := []jsonschema.GridRowInput{chevRow}
	if withBody {
		descHPt := float64(maxDescLines)*descSize*chevronLineHeight + 2*chevronDescInsetPt
		totalPt = geo.chevHPt + ctx.Gap(chevronRowGapPt) + descHPt
		rows[0].Height = geo.chevHPt / totalPt * 100
		rows = append(rows, jsonschema.GridRowInput{Cells: descCells})
	}
	heightPct := totalPt / geo.contentHPt * 100
	if heightPct > 100 {
		// Content taller than the area: fall back to a proportional split so
		// the chevrons keep their capped share and descriptions get the rest.
		heightPct = 100
		if withBody {
			rows[0].Height = math.Min(geo.chevHPt/geo.contentHPt*100, 40)
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     0,
		RowGap:  ctx.Gap(chevronRowGapPt),
		Rows:    rows,
		Bounds:  &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 100, Height: math.Round(heightPct*10) / 10},
	}
	return grid
}

const (
	// chevronAdj is the chevron point/notch depth as a fraction (×100000) of
	// the shape height. The OOXML default (50000) on a near-square chevron
	// swallows the label; 30% keeps a clear arrow with room for text.
	chevronAdj = 30000
	// chevronMinAdj is the shallowest notch fitChevronLabels will go to. Past
	// this the shape reads as a rectangle with a dent rather than an arrow, so
	// a label that still does not fit shrinks instead (go-slide-creator-e97v).
	chevronMinAdj = 12000
	// chevronAdjStep is the granularity of the notch search.
	chevronAdjStep = 3000
	// chevronMinLabelPt is the readable floor for a chevron label. It matches
	// shapegrid.MinTextSizePt: below it the renderer's own readability policy
	// reports the text, so there is nothing to gain by going smaller.
	chevronMinLabelPt = 12.0
	// chevronColGapPt is the column gap the renderer puts between chevrons.
	// The pattern asks for 0, but a zero gap reads as "unset" in the grid DTO
	// and resolves to shapegrid's 8pt default, so each chevron is 8pt narrower
	// than its share of the content area. Measuring against the share instead
	// of the shape is how a label that "fits" still wrapped.
	chevronColGapPt = 8.0
	// chevronFitSafetyFrac is the share of the available width a label has to
	// fit inside. The measurement runs on whatever font this machine has
	// (Liberation Sans substitutes for a missing template font) while the
	// renderer uses its own, so a label measured at 99% of the width still
	// broke mid-word. A blunter notch is a far cheaper loss than "Qualific /
	// ation".
	chevronFitSafetyFrac = 0.90
	// chevronDescDefaultSize is the default detail-zone text size (pt).
	chevronDescDefaultSize = scaleBodyPt
	// chevronDescInsetPt is the detail zone's side margin: the uniform shape
	// text inset.
	chevronDescInsetPt = defaultShapeInsetLRPt
	chevronRowGapPt    = 6.0
	chevronLineHeight  = 1.25
	// chevronMaxAspectH caps chevron height at half its width (width >= 2x height).
	chevronMaxAspectH = 0.5
	// chevronMaxHeightFrac caps the chevron row at this share of the content height.
	chevronMaxHeightFrac = 0.3
	// chevronMinHeightPt keeps a two-line (number + label) chevron legible:
	// an 11pt number and a 13pt label at 1.2 line height plus the uniform
	// top and bottom shape margin.
	chevronMinHeightPt = 58.0
)

// chevronGeometry is the estimated per-step geometry of a chevron strip.
type chevronGeometry struct {
	stepWPt    float64 // width of one chevron column
	chevHPt    float64 // chevron row height
	notchPt    float64 // depth of the tail notch (= depth of the point)
	contentHPt float64 // content-area height the grid bounds are relative to
}

// chevronStripGeometry estimates chevron sizes from the content area so the
// row height, notch depth and text insets can be fixed at expand time.
func chevronStripGeometry(ctx ExpandContext, count int) chevronGeometry {
	w, h := expandContentSize(ctx)
	if w <= 0 || h <= 0 {
		db := shapegrid.DefaultBounds(12192000, 6858000)
		w, h = db.CX, db.CY
	}
	if count < 1 {
		count = 1
	}
	const emuPerPt = 12700.0
	// Subtract the gaps the renderer puts between the chevrons: the step is
	// what one chevron actually gets, not its share of the content area.
	stepW := (float64(w)/emuPerPt - ctx.Gap(chevronColGapPt)*float64(count-1)) / float64(count)
	contentH := float64(h) / emuPerPt
	chevH := math.Min(stepW*chevronMaxAspectH, contentH*chevronMaxHeightFrac)
	if chevH < chevronMinHeightPt {
		chevH = math.Min(chevronMinHeightPt, stepW*chevronMaxAspectH)
	}
	// OOXML chevron: notch/point depth = adj × min(w, h); h is the minimum
	// because of the aspect cap above.
	notch := float64(chevronAdj) / 100000 * math.Min(chevH, stepW)
	return chevronGeometry{stepWPt: stepW, chevHPt: chevH, notchPt: notch, contentHPt: contentH}
}

// chevronLabelFit is the notch depth and label size one strip renders at.
//
// The notch inset and the chevron width were fixed independently: at 6 steps a
// chevron is 131pt wide and the 30% notch takes 45pt of it, so a 13pt label had
// 86pt to live in and "Qualification" wrapped to "Qualific / ation" — with no
// finding, because two lines still fit inside the shape (go-slide-creator-e97v).
// Reconciling them means giving up notch depth first (the arrow reads the same
// a little blunter) and label size only after that.
type chevronLabelFit struct {
	adj     int64    // chevron adjustment value, ×100000 of the shorter side
	labelPt float64  // label size that keeps every label on one line
	unfit   []string // labels that still wrap at the readable floor
}

// fitChevronLabels finds the deepest notch and the largest label size at which
// every step label stays on one line, and reports the labels that cannot.
func fitChevronLabels(ctx ExpandContext, vals *NumberedStepStripValues, geo chevronGeometry, labelPt float64) chevronLabelFit {
	font := ctx.Theme.BodyFont
	labels := chevronLabels(vals)

	// Deepest notch first: the arrow stays as pointed as the labels allow.
	for adj := int64(chevronAdj); adj >= chevronMinAdj; adj -= chevronAdjStep {
		if chevronLabelsFitOneLine(labels, font, labelPt, chevronLabelWidthPt(geo, adj)) {
			return chevronLabelFit{adj: adj, labelPt: labelPt}
		}
	}

	// Even the shallowest notch is not enough, so the label shrinks — never
	// below the readable floor, where the label has to get shorter instead.
	avail := chevronLabelWidthPt(geo, chevronMinAdj)
	size := labelPt
	for _, label := range labels {
		if s := fitSingleLineSize(label, font, true, size, chevronMinLabelPt, avail); s < size {
			size = s
		}
	}
	fit := chevronLabelFit{adj: chevronMinAdj, labelPt: size}
	for _, label := range labels {
		if measuredLines(label, font, true, size, avail) > 1 {
			fit.unfit = append(fit.unfit, label)
		}
	}
	return fit
}

// listFirstN quotes at most n entries, noting how many were left out.
func listFirstN(items []string, n int) string {
	if len(items) <= n {
		return strconv.Quote(strings.Join(items, ", "))
	}
	return fmt.Sprintf("%s and %d more", strconv.Quote(strings.Join(items[:n], ", ")), len(items)-n)
}

// chevronLabels returns the non-empty step labels, as rendered.
func chevronLabels(vals *NumberedStepStripValues) []string {
	if vals == nil {
		return nil
	}
	labels := make([]string, 0, len(vals.Steps))
	for _, step := range vals.Steps {
		if label := strings.TrimSpace(step.Label); label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

// chevronNotchPt is the depth of the tail notch (and of the point) at a given
// adjustment value: the OOXML chevron measures it against the shorter side.
func chevronNotchPt(geo chevronGeometry, adj int64) float64 {
	return float64(adj) / 100000 * math.Min(geo.chevHPt, geo.stepWPt)
}

// chevronLabelWidthPt is the width a label is measured against.
//
// A renderer lays text out inside the chevron's own text rectangle, which is
// already pulled in past the point and the notch; the uniform shape text
// margin then stacks on top of that on both sides. Measuring against the shape
// width alone said a 13pt "Qualification" fitted, and LibreOffice broke it at
// "Qualific / ation" (go-slide-creator-e97v).
func chevronLabelWidthPt(geo chevronGeometry, adj int64) float64 {
	notch := chevronNotchPt(geo, adj)
	return (geo.stepWPt - 2*notch - 2*defaultShapeInsetLRPt) * chevronFitSafetyFrac
}

// chevronLabelsFitOneLine reports whether every label renders on a single line
// at sizePt within widthPt.
func chevronLabelsFitOneLine(labels []string, font string, sizePt, widthPt float64) bool {
	if widthPt <= 0 {
		return false
	}
	for _, label := range labels {
		if measuredLines(label, font, true, sizePt, widthPt) > 1 {
			return false
		}
	}
	return true
}

// estimateWrappedLines estimates how many lines text wraps to at sizePt in a
// box widthPt wide (average glyph advance ≈ 0.5 em).
func estimateWrappedLines(text string, sizePt, widthPt float64) int {
	if widthPt <= 0 || sizePt <= 0 {
		return 1
	}
	perLine := int(widthPt / (sizePt * 0.5))
	if perLine < 1 {
		perLine = 1
	}
	lines := 0
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		cur := 0
		paraLines := 1
		for _, w := range words {
			l := len([]rune(w))
			switch {
			case cur == 0:
				cur = l
			case cur+1+l <= perLine:
				cur += 1 + l
			default:
				paraLines++
				cur = l
			}
			for cur > perLine {
				paraLines++
				cur -= perLine
			}
		}
		lines += paraLines
	}
	if lines < 1 {
		lines = 1
	}
	return lines
}

// buildChevronLabelText renders the number + label inside a chevron. The
// preset's text rectangle clears the tail notch and the point, and the
// uniform shape text margin sits inside it.
func buildChevronLabelText(number, label string, size float64, color string) json.RawMessage {
	obj := numberedStepTextObj{
		Paragraphs: []numberedStepParagraph{
			{Content: number, Size: size - 2, Bold: true, Color: color, Align: "ctr"},
			{Content: label, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

// buildChevronDescText renders a detail-zone description, top-anchored so it
// sits directly under its chevron.
func buildChevronDescText(body string, size float64) json.RawMessage {
	obj := numberedStepTextObj{
		Paragraphs: []numberedStepParagraph{
			{Content: body, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "t",
	}
	data, _ := json.Marshal(obj)
	return data
}

// ---------------------------------------------------------------------------
// stacked-box style — table-like rows: narrow colored number/tip column on the
// left + no-border body column on the right (the scorecard look).
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) expandStackedBox(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	spec := numberedStepDetailSpec(ctx, vals, ovr, stackedBoxRowSpec(numberedStepsHaveIcons(vals)).withGutter(ctx), sizeLabelPt)
	lay := layoutNumberedStepRows(ctx, spec, sizeLabelPt, func(labelPt float64) []jsonschema.GridRowInput {
		return n.stackedBoxRows(ctx, vals, ovr, cellOverrides, labelPt, spec.detailBeside)
	})
	return lay.grid(spec)
}

// stackedBoxRowSpec is the stacked-box column geometry: number lane, optional
// icon column, body column.
func stackedBoxRowSpec(withIcons bool) numberedStepRowSpec {
	if withIcons {
		return numberedStepRowSpec{cols: `[1, 0.7, 8]`, weights: []float64{1, 0.7, 8}, colGapPt: stackedBoxColGapPt}
	}
	return numberedStepRowSpec{cols: `[1, 8]`, weights: []float64{1, 8}, colGapPt: stackedBoxColGapPt}
}

// stackedBoxRows builds the stacked-box rows with the step labels at labelSize.
func (n *numberedStepStrip) stackedBoxRows(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any, labelSize float64, detailBeside bool) []jsonschema.GridRowInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	numberSize := ResolveSize(ovr.HeaderSize, scaleLeadPt)
	bodySize := ResolveSize(ovr.BodySize, scaleCaptionPt)
	cellAccentMode := ovr.CellAccentMode
	solid := ovr.Style == "solid"

	withIcons := numberedStepsHaveIcons(vals)
	rows := make([]jsonschema.GridRowInput, len(vals.Steps))
	for i, step := range vals.Steps {
		tip := step.TipColor
		if tip == "" {
			tip = ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		}

		// The number lane is a neutral tint carrying an accent numeral by
		// default; a stack of solid accent lanes was a wall of colour
		// (go-slide-creator-fl11f). overrides.style "solid" restores it.
		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     neutralFillJSON(NeutralTint4),
				Line:     noLine,
				Text:     buildNumberedStepNumberText(stepNumber(step, i), numberSize, accentInkOnTone(ctx, tip, neutralTone(NeutralTint4), 3.0)),
			},
		}
		if solid {
			numberCell.Shape.Fill = json.RawMessage(fmt.Sprintf(`"%s"`, tip))
			numberCell.Shape.Line = nil
			numberCell.Shape.Text = buildNumberedStepNumberText(stepNumber(step, i), numberSize, "lt1")
		}

		bodyCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text: buildNumberedStepStackedBody(
					pptx.ConvertMarkdownEmphasis(step.Label), labelSize,
					pptx.ConvertMarkdownEmphasis(strings.TrimSpace(step.Body)), bodySize),
			},
		}
		if step.Recommended {
			tone, ink := accentFillAndInk(ctx, fillTone{Color: baseAccent}, 4.5)
			bodyCell.Shape.Fill = tone.fillJSON()
			bodyCell.Shape.Text = buildNumberedStepRecommendedBody(
				pptx.ConvertMarkdownEmphasis(step.Label), labelSize,
				pptx.ConvertMarkdownEmphasis(strings.TrimSpace(step.Body)), bodySize, ink)
		}
		applyNumberedStepOverride(bodyCell, cellOverrides, i, baseAccent)

		cells := []*jsonschema.GridCellInput{numberCell}
		if withIcons {
			cells = append(cells, numberedStepIconCell(ctx, step, tip))
		}
		cells = append(cells, bodyCell)
		if detailBeside {
			cells = append(cells, numberedStepDetailCell(bodyCell, step, labelSize, bodySize, cellOverrides, i))
		}
		rows[i] = jsonschema.GridRowInput{Cells: cells}
	}
	return rows
}

const (
	stackedBoxColGapPt    = 8.0
	tocColGapPt           = 10.0
	stackedBoxRowGapMaxPt = 6.0
	stackedBoxRowGapMinPt = 2.0
)

// numberedStepRowSpec is the column geometry of a stacked-box / toc strip.
type numberedStepRowSpec struct {
	cols     string
	weights  []float64
	colGapPt float64
	// detailBeside puts each step's body in a detail column beside its label
	// instead of on a line under it (numberedStepDetailSpec).
	detailBeside bool
}

// numberedStepLabelShare is the label column's share of the label + detail
// width in the detail-beside layout: the details start a third of the way
// across the slide, so one-line details reach its right half.
const numberedStepLabelShare = 0.36

// numberedStepDetailGrowth is the size the one-line test allows for: the
// composition policy may step a sparse strip's text up once.
const numberedStepDetailGrowth = 1.2

// numberedStepDetailSpec decides where the step bodies go. A body under its
// label is right for rows of text that reach across the slide; when every
// label and every body is one short line, the stacked rows end before
// mid-slide and the right half stays empty (go-slide-creator-yhzxt). Such a
// strip is a three-part row — number, label, detail — so the bodies move to a
// detail column of their own and the row uses its width. It is taken only
// when every label and every body stays on one line in its column (with room
// for the placement policy's type step) and no step carries the recommended
// tile; otherwise spec is returned unchanged.
func numberedStepDetailSpec(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, spec numberedStepRowSpec, labelPt float64) numberedStepRowSpec {
	bodies := 0
	for _, step := range vals.Steps {
		if step.Recommended {
			return spec
		}
		if strings.TrimSpace(step.Body) != "" {
			bodies++
		}
	}
	if bodies == 0 || len(spec.weights) < 2 {
		return spec
	}
	last := len(spec.weights) - 1
	areaW, _ := contentAreaPt(ctx)
	bodyColW := spec.colWidthsPt(areaW)[last] - spec.colGapPt
	if bodyColW <= 0 {
		return spec
	}
	inset := 2 * float64(pptx.ShapeTextInsetEMU) / 12700
	bodyPt := ResolveSize(ovr.BodySize, scaleCaptionPt)
	font := ctx.Theme.BodyFont
	lineWidthPt := func(text string, sizePt float64, bold bool) float64 {
		w, err := textfit.MeasureStyledLineWidth(text, font, sizePt*numberedStepDetailGrowth, bold)
		if err != nil {
			return math.Inf(1)
		}
		return float64(w) / 12700
	}
	labelNeed := 0.0
	for _, step := range vals.Steps {
		labelNeed = math.Max(labelNeed, lineWidthPt(step.Label, labelPt, true))
	}
	// 1.1: the renderer's face may draw wider than the one measured.
	share := numberedStepLabelShare
	if (labelNeed*1.1+inset)/bodyColW > share {
		return spec
	}
	detailW := bodyColW*(1-share) - inset
	for _, step := range vals.Steps {
		if body := strings.TrimSpace(step.Body); body != "" && lineWidthPt(body, bodyPt, false)*1.05 > detailW {
			return spec
		}
	}

	beside := spec
	beside.detailBeside = true
	beside.weights = append(append([]float64{}, spec.weights[:last]...), spec.weights[last]*share, spec.weights[last]*(1-share))
	parts := make([]string, len(beside.weights))
	for i, w := range beside.weights {
		beside.weights[i] = math.Round(w*100) / 100
		parts[i] = strconv.FormatFloat(beside.weights[i], 'f', -1, 64)
	}
	beside.cols = "[" + strings.Join(parts, ", ") + "]"
	return beside
}

// numberedStepDetailCell splits a step's stacked label + body cell for the
// detail-beside layout: labelCell keeps the label alone and the returned cell
// carries the body. The step's text override applies to both.
func numberedStepDetailCell(labelCell *jsonschema.GridCellInput, step NumberedStepStripStep, labelSize, bodySize float64, cellOverrides map[int]any, idx int) *jsonschema.GridCellInput {
	labelCell.Shape.Text = buildNumberedStepStackedBody(pptx.ConvertMarkdownEmphasis(step.Label), labelSize, "", bodySize)
	body := strings.TrimSpace(step.Body)
	if body == "" {
		return &jsonschema.GridCellInput{}
	}
	text, _ := json.Marshal(numberedStepTextObj{
		Paragraphs:    []numberedStepParagraph{{Content: pptx.ConvertMarkdownEmphasis(body), Size: bodySize, Color: "dk2", Align: "l"}},
		Align:         "l",
		VerticalAlign: "ctr",
	})
	cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: text}}
	if co, ok := cellOverrides[idx].(*NumberedStepStripCellOverride); ok {
		applyCellTextOverride(cell, co)
	}
	return cell
}

// withGutter scales the column gap by the template grid's gutter (unchanged
// when no grid is declared), like every other pattern gap (go-slide-creator-5ms8c).
func (s numberedStepRowSpec) withGutter(ctx ExpandContext) numberedStepRowSpec {
	s.colGapPt = ctx.Gap(s.colGapPt)
	return s
}

// colWidthsPt returns each column's width at the content width.
func (s numberedStepRowSpec) colWidthsPt(areaW float64) []float64 {
	total := 0.0
	for _, w := range s.weights {
		total += w
	}
	avail := areaW - s.colGapPt*float64(len(s.weights)-1)
	out := make([]float64, len(s.weights))
	for i, w := range s.weights {
		out[i] = avail * w / total
	}
	return out
}

// numberedStepRowLayout is the sizing a stacked-box / toc strip renders at.
type numberedStepRowLayout struct {
	rows    []jsonschema.GridRowInput
	rowPt   []float64
	fullPt  []float64 // each row's fit with the full 0.5 cm margin
	gapPt   float64
	labelPt float64
	needPt  float64 // rows plus the narrowest gaps
	areaPt  float64
	fits    bool
}

func (l numberedStepRowLayout) grid(spec numberedStepRowSpec) *jsonschema.ShapeGridInput {
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(spec.cols),
		Gap:     spec.colGapPt,
		RowGap:  l.gapPt,
		Rows:    l.rows,
	}
}

// layoutNumberedStepRows sizes stacked-box / toc rows against the content
// area the strip renders into — the zone left after the title, the footer and
// any takeaway / source band (go-slide-creator-ni71s). Rows used to be
// AutoHeight, estimated from newline counts, so when the block was taller than
// a zone shortened by chrome bands the grid scaled every row down and the
// writer stored 12–14pt labels at 50–96% autofit (6–11.5pt): generation
// refused the deck. Every row is now pinned at the writer's own fit of its
// cells (writtenFitHeightPt at the real column widths), and when the rows do
// not fit they give way in the written-fit order: air (the row gap 6 → 2pt),
// geometry (single-line rows drop to the writer's clamped one-line margin),
// then type (the label / title steps to 12pt). What still does not fit keeps
// the larger step and is reported by PostExpandWarnings as BODY_TOO_LONG.
func layoutNumberedStepRows(ctx ExpandContext, spec numberedStepRowSpec, fullLabelPt float64, build func(labelPt float64) []jsonschema.GridRowInput) numberedStepRowLayout {
	areaW, areaH := contentAreaPt(ctx)
	widths := spec.colWidthsPt(areaW)
	type attempt struct {
		labelPt float64
		tight   bool
	}
	attempts := []attempt{{fullLabelPt, false}, {fullLabelPt, true}}
	if fullLabelPt > scaleBodyPt {
		attempts = append(attempts, attempt{scaleBodyPt, false}, attempt{scaleBodyPt, true})
	}
	var first numberedStepRowLayout
	for i, a := range attempts {
		lay := measureNumberedStepRows(ctx.themeFonts(), build(a.labelPt), widths, a.tight)
		lay.labelPt, lay.areaPt = a.labelPt, areaH
		gaps := float64(max(len(lay.rows)-1, 0))
		rowsPt := lay.needPt
		lay.needPt = rowsPt + ctx.Gap(stackedBoxRowGapMinPt)*gaps
		lay.gapPt = ctx.Gap(stackedBoxRowGapMaxPt)
		if gaps > 0 && rowsPt+ctx.Gap(stackedBoxRowGapMaxPt)*gaps > areaH {
			lay.gapPt = math.Max(ctx.Gap(stackedBoxRowGapMinPt), math.Floor((areaH-rowsPt)/gaps))
		}
		lay.fits = lay.needPt <= areaH+0.5
		if lay.fits {
			if a.tight {
				growTightRows(&lay, areaH, gaps)
			}
			return lay
		}
		// The type step is taken only when it makes the strip fit: content
		// that overflows even there keeps the larger size, since the writer
		// shrinks it either way.
		if i == 1 {
			first = lay
		}
	}
	return first
}

// measureNumberedStepRows pins each row at the writer's fit of its tallest
// cell. tight measures single-line cells at the writer's clamped margin (one
// line always fits a shape that holds it) instead of the full 0.5 cm margin.
func measureNumberedStepRows(fonts pptx.ThemeFonts, rows []jsonschema.GridRowInput, widths []float64, tight bool) numberedStepRowLayout {
	lay := numberedStepRowLayout{rows: rows, rowPt: make([]float64, len(rows)), fullPt: make([]float64, len(rows))}
	for i := range rows {
		need, full := 0.0, 0.0
		for c, cell := range rows[i].Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 || c >= len(widths) {
				continue
			}
			h := rowTextNeedPt(fonts, cell.Shape.Text, widths[c])
			full = math.Max(full, h)
			if tight && h < rowTextBeyondAreaPt {
				h = writtenTightFitPt(fonts, cell.Shape.Text, widths[c], h)
			}
			need = math.Max(need, h)
		}
		lay.fullPt[i] = full
		if need >= rowTextBeyondAreaPt {
			need = rowTextBeyondAreaPt
		}
		lay.rowPt[i] = need
		lay.needPt += need
		if need > 0 && need < rowTextBeyondAreaPt {
			rows[i].MinHeight, rows[i].MaxHeight = need, need
		}
	}
	return lay
}

// growTightRows shares the slack a clamped-margin layout leaves equally
// among its rows, never past a row's full-margin fit, so rows that only fit
// with the writer's clamp get back as much air as the zone allows.
func growTightRows(lay *numberedStepRowLayout, areaH, gaps float64) {
	slack := areaH - lay.gapPt*gaps
	for _, h := range lay.rowPt {
		slack -= h
	}
	for slack > 0.5 {
		open := 0
		for i, h := range lay.rowPt {
			if h < lay.fullPt[i] {
				open++
			}
		}
		if open == 0 {
			break
		}
		share := slack / float64(open)
		for i, h := range lay.rowPt {
			if h >= lay.fullPt[i] {
				continue
			}
			grown := math.Min(lay.fullPt[i], h+share)
			slack -= grown - h
			lay.rowPt[i] = grown
		}
	}
	for i, h := range lay.rowPt {
		if h > 0 && h < rowTextBeyondAreaPt {
			h = math.Floor(h*10) / 10
			lay.rows[i].MinHeight, lay.rows[i].MaxHeight = h, h
		}
	}
}

// writtenTightFitPt is the smallest whole-point height at which the writer
// stores text with no shrink, counting the margin clamp it applies to a shape
// too short for one line plus the full margin (pptx.EffectiveTextInsets).
// fitPt is a height known to fit.
func writtenTightFitPt(fonts pptx.ThemeFonts, text json.RawMessage, widthPt, fitPt float64) float64 {
	lo, hi := 0.0, math.Ceil(fitPt)
	if !writtenFitsAt(fonts, text, widthPt, hi) {
		return fitPt
	}
	for hi-lo > 1 {
		mid := math.Floor((lo + hi) / 2)
		if writtenFitsAt(fonts, text, widthPt, mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// numberedStepRowsWarning reports a stacked-box / toc strip whose rows do not
// fit the measured content area even after the gap, margin and type give way.
func (n *numberedStepStrip) numberedStepRowsWarning(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, style string) string {
	if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return ""
	}
	var lay numberedStepRowLayout
	if style == numberedStepStripTOC {
		spec := numberedStepDetailSpec(ctx, vals, ovr, tocRowSpec(numberedStepsHaveIcons(vals)).withGutter(ctx), scaleSubheadPt)
		lay = layoutNumberedStepRows(ctx, spec, scaleSubheadPt, func(labelPt float64) []jsonschema.GridRowInput {
			return n.tocRows(ctx, vals, ovr, nil, labelPt, spec.detailBeside)
		})
	} else {
		spec := numberedStepDetailSpec(ctx, vals, ovr, stackedBoxRowSpec(numberedStepsHaveIcons(vals)).withGutter(ctx), sizeLabelPt)
		lay = layoutNumberedStepRows(ctx, spec, sizeLabelPt, func(labelPt float64) []jsonschema.GridRowInput {
			return n.stackedBoxRows(ctx, vals, ovr, nil, labelPt, spec.detailBeside)
		})
	}
	if lay.fits {
		return ""
	}
	return fmt.Sprintf("%s: numbered-step-strip %d %s rows need %s at 12pt with 2pt gaps but the content area holds about %.0fpt (a takeaway or source band shortens it) — drop or shorten the step bodies, use fewer steps, or drop the takeaway / source",
		ErrCodeBodyTooLong, len(vals.Steps), style, readableNeedPhrase(lay.needPt), lay.areaPt)
}

// numberedStepsHaveIcons reports whether any step carries an icon; the icon
// column is all-or-nothing so labels stay left-aligned down the list.
func numberedStepsHaveIcons(vals *NumberedStepStripValues) bool {
	for _, s := range vals.Steps {
		if s.Icon != nil && !s.Icon.IsEmpty() {
			return true
		}
	}
	return false
}

// numberedStepIconCell is the icon column cell for one step: the icon drawn
// in the step's tip colour (or a readable ink when that colour is too pale on
// the slide), capped at numberedStepIconMaxPt and centred on the row. A step
// without an icon gets an empty cell so the column stays aligned.
func numberedStepIconCell(ctx ExpandContext, step NumberedStepStripStep, tip string) *jsonschema.GridCellInput {
	if step.Icon == nil || step.Icon.IsEmpty() {
		return &jsonschema.GridCellInput{}
	}
	return &jsonschema.GridCellInput{
		MaxHeight: numberedStepIconMaxPt,
		Icon:      step.Icon.Resolve(iconFillOn(ctx, json.RawMessage(`"none"`), tip), ""),
	}
}

// ---------------------------------------------------------------------------
// toc style — high-polish numbered agenda: a rounded accent number badge and a
// no-fill title (optionally with a body line). Can replace a plain agenda.
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) expandTOC(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	spec := numberedStepDetailSpec(ctx, vals, ovr, tocRowSpec(numberedStepsHaveIcons(vals)).withGutter(ctx), scaleSubheadPt)
	lay := layoutNumberedStepRows(ctx, spec, scaleSubheadPt, func(labelPt float64) []jsonschema.GridRowInput {
		return n.tocRows(ctx, vals, ovr, cellOverrides, labelPt, spec.detailBeside)
	})
	return lay.grid(spec)
}

// tocRowSpec is the toc column geometry: number, optional icon, title.
func tocRowSpec(withIcons bool) numberedStepRowSpec {
	if withIcons {
		return numberedStepRowSpec{cols: `[1, 0.6, 6]`, weights: []float64{1, 0.6, 6}, colGapPt: tocColGapPt}
	}
	return numberedStepRowSpec{cols: `[1, 6]`, weights: []float64{1, 6}, colGapPt: tocColGapPt}
}

// tocRows builds the toc rows with the step titles at titleSize.
func (n *numberedStepStrip) tocRows(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any, titleSize float64, detailBeside bool) []jsonschema.GridRowInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	numberSize := ResolveSize(ovr.HeaderSize, sizeLeadPlusPt)
	bodySize := ResolveSize(ovr.BodySize, scaleCaptionPt)
	cellAccentMode := ovr.CellAccentMode
	solid := ovr.Style == "solid"

	withIcons := numberedStepsHaveIcons(vals)
	rows := make([]jsonschema.GridRowInput, len(vals.Steps))
	for i, step := range vals.Steps {
		badge := step.TipColor
		if badge == "" {
			badge = ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		}

		// An unfilled accent numeral by default (go-slide-creator-fl11f);
		// overrides.style "solid" restores the accent badge.
		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Line:     noLine,
				Text:     buildNumberedStepNumberText(stepNumber(step, i), numberSize, accentInkOnLight(ctx, badge, 3.0)),
			},
		}
		if solid {
			numberCell.Shape.Geometry = "roundRect"
			numberCell.Shape.Fill = json.RawMessage(fmt.Sprintf(`"%s"`, badge))
			numberCell.Shape.Line = nil
			numberCell.Shape.Text = buildNumberedStepNumberText(stepNumber(step, i), numberSize, "lt1")
		}

		titleCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text: buildNumberedStepStackedBody(
					pptx.ConvertMarkdownEmphasis(step.Label), titleSize,
					pptx.ConvertMarkdownEmphasis(strings.TrimSpace(step.Body)), bodySize),
			},
		}
		applyNumberedStepOverride(titleCell, cellOverrides, i, baseAccent)

		cells := []*jsonschema.GridCellInput{numberCell}
		if withIcons {
			cells = append(cells, numberedStepIconCell(ctx, step, badge))
		}
		cells = append(cells, titleCell)
		if detailBeside {
			cells = append(cells, numberedStepDetailCell(titleCell, step, titleSize, bodySize, cellOverrides, i))
		}
		rows[i] = jsonschema.GridRowInput{Cells: cells}
	}
	return rows
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type numberedStepParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type numberedStepTextObj struct {
	Paragraphs    []numberedStepParagraph `json:"paragraphs"`
	Align         string                  `json:"align"`
	VerticalAlign string                  `json:"vertical_align"`
}

// buildNumberedStepNumberText renders a single centered ordinal, used in the
// number / tip lane of stacked-box and toc styles.
func buildNumberedStepNumberText(number string, size float64, color string) json.RawMessage {
	obj := numberedStepTextObj{
		Paragraphs: []numberedStepParagraph{
			{Content: number, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

// buildNumberedStepStackedBody renders a bold label and, when present, a body
// line beneath it — the left-aligned body column for stacked-box and toc.
func buildNumberedStepStackedBody(label string, labelSize float64, body string, bodySize float64) json.RawMessage {
	paras := []numberedStepParagraph{
		{Content: label, Size: labelSize, Bold: true, Color: "dk1", Align: "l"},
	}
	if body != "" {
		paras = append(paras, numberedStepParagraph{Content: body, Size: bodySize, Color: "dk2", Align: "l"})
	}
	obj := numberedStepTextObj{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

func buildNumberedStepRecommendedBody(label string, labelSize float64, body string, bodySize float64, ink string) json.RawMessage {
	paras := []numberedStepParagraph{
		{Content: "RECOMMENDED", Size: sizeBadgePt, Bold: true, Color: ink, Align: "l"},
		{Content: label, Size: labelSize, Bold: true, Color: ink, Align: "l"},
	}
	if body != "" {
		paras = append(paras, numberedStepParagraph{Content: body, Size: bodySize, Color: ink, Align: "l"})
	}
	data, _ := json.Marshal(numberedStepTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "ctr"})
	return data
}

// applyNumberedStepOverride applies the shared per-cell accent-bar override.
func applyNumberedStepOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*NumberedStepStripCellOverride)
	if !coOk {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
