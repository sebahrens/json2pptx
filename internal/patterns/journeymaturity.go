package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// journey-maturity-model pattern — N-stage horizontal maturity ladder with
// state labels, descriptions, and an optional "where we are" current marker
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&journeyMaturity{})
}

type journeyMaturity struct{}

func (jm *journeyMaturity) Name() string { return "journey-maturity-model" }
func (jm *journeyMaturity) Description() string {
	return "Ascending maturity staircase: 3-6 solid steps rising left to right in a tonal ladder of the accent, each with a big stage numeral and bold name, a 1-3 line description under it, and an optional 'where we are' marker on the current stage (the one solid-accent step)"
}
func (jm *journeyMaturity) UseWhen() string {
	return "Capability or digital maturity model with 3-6 named stages where progression matters and a single stage represents the current state; prefer value-chain when stages have no progression semantics, phase-roadmap when stages are time-anchored, and process-flow for short action steps without descriptions"
}
func (jm *journeyMaturity) NotWhen() string {
	return "Stages are time-anchored milestones (use phase-roadmap), the sequence has no progression semantics (use value-chain), steps are short actions without descriptions (use process-flow), or the ladder has fewer than 3 / more than 6 stages"
}
func (jm *journeyMaturity) Version() int      { return 1 }
func (jm *journeyMaturity) CellsHint() string { return "3-6" }
func (jm *journeyMaturity) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"value-chain", "phase-roadmap", "process-flow", "kpi-3up"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (jm *journeyMaturity) SupportsInlineMarkdown() bool { return true }

func (jm *journeyMaturity) ExemplarValues() any {
	return &JourneyMaturityValues{
		Stages: []JourneyMaturityStage{
			{Label: "Initial", Description: "Ad hoc processes; no formal practices in place."},
			{Label: "Developing", Description: "Repeatable practices; informal governance."},
			{Label: "Defined", Description: "Standardised practices; documented playbooks.", Current: true},
			{Label: "Managed", Description: "Quantitatively measured outcomes; continuous review."},
			{Label: "Optimising", Description: "Continuous improvement embedded across the organisation."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// JourneyMaturityStage is a single stage on the maturity ladder: an optional
// number badge, a short label, a 1-3 line description, and an optional
// `current` flag that marks the stage as the present state.
type JourneyMaturityStage struct {
	Number      int    `json:"number,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Current     bool   `json:"current,omitempty"`
}

// JourneyMaturityValues holds the ordered stages on the ladder (3-6 items).
type JourneyMaturityValues struct {
	Stages []JourneyMaturityStage `json:"stages"`
}

// JourneyMaturityOverrides is the standard text overrides plus the ladder
// style.
type JourneyMaturityOverrides struct {
	TextOverrides
	// Style is "staircase" (default: one solid step per stage, rising in a
	// tonal ladder of the accent), "columns" (a header box over a description
	// box per stage, each column starting higher than the last — the default
	// before go-slide-creator-an4ao) or "flat" (equal boxes in one row joined
	// by small arrows — the look before go-slide-creator-j8t7o).
	Style string `json:"style,omitempty"`
}

// The accepted overrides.style values.
const (
	journeyMaturityStyleStaircase = "staircase"
	journeyMaturityStyleColumns   = "columns"
	journeyMaturityStyleFlat      = "flat"
)

var journeyMaturityStyles = []string{journeyMaturityStyleStaircase, journeyMaturityStyleColumns, journeyMaturityStyleFlat}

// journeyMaturityOverridesSchema is the text overrides plus the ladder style.
func journeyMaturityOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(journeyMaturityStyles...).WithDescription("staircase (default): one solid step per stage on a shared floor, each a rise taller than the last, filled as a tonal ladder of the accent's lighter swatches up to the solid accent on the current stage (the last stage when none is current; stages ahead of it stay palest); stage numeral and bold name on the step, descriptions unboxed beneath at one size, 'We are here' label over a solid pointer on the current step. columns: a header box over a description box per stage, each column starting higher, outlined marker beneath (the earlier default). flat: equal header and description boxes in one row joined by small arrows").WithDefault(journeyMaturityStyleStaircase)
	return s
}

// JourneyMaturityCellOverride is the shared per-cell override; indexed by stage.
type JourneyMaturityCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (jm *journeyMaturity) NewValues() any       { return &JourneyMaturityValues{} }
func (jm *journeyMaturity) NewOverrides() any    { return &JourneyMaturityOverrides{} }
func (jm *journeyMaturity) NewCellOverride() any { return &JourneyMaturityCellOverride{} }

func (jm *journeyMaturity) Schema() *Schema {
	stageSchema := ObjectSchema(
		map[string]*Schema{
			"number":      IntegerSchema(1, 9).WithDescription("Optional stage number (defaults to 1..N by position)"),
			"label":       StringSchema(40).WithDescription("Short stage label (1-3 words); at 6 stages keep unbroken runs near 34 characters"),
			"description": StringSchema(180).WithDescription("1-3 line description rendered below the label; about 162 readable characters at 5 stages and 122 at 6; at 4/5/6 stages keep wide unbroken runs near 146/115/95 characters or add word breaks"),
			"current":     BooleanSchema().WithDescription("When true, marks this stage as the present state: it takes the solid accent and a 'We are here' marker (set it on one stage)"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"stages": ArraySchema(stageSchema, 3, 6).WithDescription("Maturity stages left-to-right (3-6)"),
		},
		[]string{"stages"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      journeyMaturityOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Ascending maturity staircase of 3-6 solid steps in an accent tonal ladder, each with a stage numeral, name, description, and optional current-stage marker")
}

func (jm *journeyMaturity) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*JourneyMaturityValues)
	if !ok || vals == nil {
		return fmt.Errorf("journey-maturity-model: values must be *JourneyMaturityValues, got %T", values)
	}

	const name = "journey-maturity-model"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*JourneyMaturityOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.Style != "" && !slices.Contains(journeyMaturityStyles, ovr.Style) {
				errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, journeyMaturityStyles))
			}
		}
	}

	if len(vals.Stages) < 3 {
		errs = append(errs, errMinItems(name, "stages", 3, len(vals.Stages), "(hint: use value-chain when there is no progression semantic)"))
	}
	if len(vals.Stages) > 6 {
		errs = append(errs, errMaxItems(name, "stages", 6, len(vals.Stages), "(hint: collapse adjacent stages or split into two slides)"))
	}

	for i, stage := range vals.Stages {
		labelPath := fmt.Sprintf("stages[%d].label", i)
		if strings.TrimSpace(stage.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if runeLen(stage.Label) > 40 {
			errs = append(errs, errMaxLength(name, labelPath, 40, runeLen(stage.Label)))
		}
		if runeLen(stage.Description) > 180 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("stages[%d].description", i), 180, runeLen(stage.Description)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Stages), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// PostExpandWarnings emits a "MULTIPLE_CURRENT_STAGES" advisory when more than
// one stage is flagged as current, since the visual marker only makes sense for
// a single present state.
func (jm *journeyMaturity) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	vals, ok := values.(*JourneyMaturityValues)
	if !ok || vals == nil {
		return nil
	}
	var warnings []string
	// Budgets measured against the written size on every shipped template,
	// every shape keeping the uniform 0.5 cm text margin
	// (go-slide-creator-n1muf): unbroken runs from four stages, worded copy
	// from five, and unbroken labels at six.
	if len(vals.Stages) >= 4 {
		wide := map[int]int{4: 146, 5: 115}[len(vals.Stages)]
		words := map[int]int{5: 162}[len(vals.Stages)]
		labelWide := 0
		if len(vals.Stages) >= 6 {
			wide, words, labelWide = 95, 122, 34
		}
		for i, stage := range vals.Stages {
			longest := 0
			for _, word := range strings.Fields(stage.Description) {
				longest = max(longest, runeLen(word))
			}
			switch {
			case longest > wide:
				warnings = append(warnings, fmt.Sprintf("%s: journey-maturity-model stages[%d].description contains a %d-character unbroken word; %d stages hold about %d wide characters per description — add a word break, shorten the copy, or use fewer stages", ErrCodeBodyTooLong, i, longest, len(vals.Stages), wide))
			case words > 0 && runeLen(stage.Description) > words:
				warnings = append(warnings, fmt.Sprintf("%s: journey-maturity-model stages[%d].description is %d characters; %d stages hold about %d readable characters per description — shorten the copy or use fewer stages", ErrCodeBodyTooLong, i, runeLen(stage.Description), len(vals.Stages), words))
			}
			labelLongest := 0
			for _, word := range strings.Fields(stage.Label) {
				labelLongest = max(labelLongest, runeLen(word))
			}
			if labelWide > 0 && labelLongest > labelWide {
				warnings = append(warnings, fmt.Sprintf("%s: journey-maturity-model stages[%d].label contains a %d-character unbroken word; %d stages hold about %d wide label characters — add a word break or shorten the label", ErrCodeBodyTooLong, i, labelLongest, len(vals.Stages), labelWide))
			}
		}
	}
	count := 0
	for _, stage := range vals.Stages {
		if stage.Current {
			count++
		}
	}
	if count <= 1 {
		return warnings
	}
	return append(warnings, fmt.Sprintf("MULTIPLE_CURRENT_STAGES: %d stages flagged as current; only one 'where we are' marker is rendered meaningfully", count))
}

func (jm *journeyMaturity) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*JourneyMaturityValues)
	if !ok {
		return nil, fmt.Errorf("journey-maturity-model: values must be *JourneyMaturityValues, got %T", values)
	}
	ovr := &JourneyMaturityOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*JourneyMaturityOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("journey-maturity-model: overrides must be *JourneyMaturityOverrides, got %T", overrides)
		}
	}

	if ovr.Style == "" || ovr.Style == journeyMaturityStyleStaircase {
		return journeyMaturitySteps(ctx, vals, ovr, cellOverrides), nil
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.HeaderSize, scaleBodyPt)
	descSize := ResolveSize(ovr.BodySize, sizeDenseCaptionPt)
	cellAccentMode := ovr.CellAccentMode

	bodyFill := surfaceFillJSON(ctx, "subtle", NeutralTint4)
	uniform := cellAccentMode == "" || cellAccentMode == CellAccentUniform

	n := len(vals.Stages)

	headerCells := make([]*jsonschema.GridCellInput, n)
	bodyCells := make([]*jsonschema.GridCellInput, n)
	markerCells := make([]*jsonschema.GridCellInput, n)

	for i, stage := range vals.Stages {
		number := stage.Number
		if number <= 0 {
			number = i + 1
		}

		// Header fill: current stage uses the resolved base accent; others
		// follow the cell-accent-mode (default uniform = base accent). When
		// the user picks cell_accent_mode=progressive or alternate this gives
		// per-stage variation while still emphasising the current stage by
		// matching the base accent.
		//
		// In the default uniform mode only the current stage keeps the solid
		// accent; the other stages are neutral 16% steps, so the slide has one
		// emphasised element instead of a row of identical accent blocks
		// (go-slide-creator-8xsj3). ApplyReadableInk darkens their text.
		headerFill := json.RawMessage(fmt.Sprintf(`"%s"`, ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)))
		if uniform {
			headerFill = neutralFillJSON(NeutralTint16)
		}
		if stage.Current {
			headerFill = json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent))
		}

		labelText := buildJourneyMaturityHeaderText(number, pptx.ConvertMarkdownEmphasis(stage.Label), labelSize)
		headerCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     headerFill,
				Text:     labelText,
			},
		}
		// The current header's accent fill is its emphasis. A left accent bar
		// on it rendered outside the cell, in the column gap over the
		// previous stage's arrow (go-slide-creator-8gbmn).

		descContent := strings.TrimSpace(stage.Description)
		var descShape *jsonschema.ShapeSpecInput
		if descContent == "" {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     bodyFill,
			}
		} else {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     bodyFill,
				Text:     buildJourneyMaturityDescriptionText(pptx.ConvertMarkdownEmphasis(descContent), descSize),
			}
		}
		bodyCell := &jsonschema.GridCellInput{Shape: descShape}

		// Marker row: only the current stage renders a "We are here" callout
		// in the resolved base accent — an up-arrow callout whose arrow
		// points at the current stage column. The shape is never rotated:
		// a rotated shape rotates its text too (the old rot=180 triangle
		// rendered the label upside down). Other cells render an empty rect
		// so the grid stays well-formed.
		//
		// The callout is outlined in the base accent so the accent-filled
		// header stays the slide's one solid emphasis (go-slide-creator-fl11f),
		// and the other cells are unfilled rects: bg1 painted white tiles on
		// a tinted slide background (go-slide-creator-8gbmn).
		var markerShape *jsonschema.ShapeSpecInput
		if stage.Current {
			markerLine, _ := json.Marshal(map[string]any{"color": baseAccent, "width": journeyMaturityMarkerLinePt})
			markerShape = &jsonschema.ShapeSpecInput{
				Geometry: journeyMaturityMarkerGeometry,
				Fill:     json.RawMessage(`"none"`),
				Line:     markerLine,
				Text:     buildJourneyMaturityMarkerText("We are here", descSize, inkOnLight(ctx, baseAccent, 4.5)),
			}
		} else {
			markerShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
			}
		}
		markerCells[i] = &jsonschema.GridCellInput{Shape: markerShape}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*JourneyMaturityCellOverride); ok2 {
				applyCellTextOverride(headerCell, cellOvr)
				if cellOvr.AccentBar {
					headerCell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    baseAccent,
						Width:    4,
					}
				}
			}
		}

		headerCells[i] = headerCell
		bodyCells[i] = bodyCell
	}

	colsJSON, _ := json.Marshal(n)

	if ovr.Style == journeyMaturityStyleColumns {
		return journeyMaturityStaircase(ctx, colsJSON, headerCells, bodyCells, markerCells), nil
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     ctx.Gap(8),
		RowGap:  ctx.Gap(4),
		Rows: []jsonschema.GridRowInput{
			{
				Height:    30,
				Cells:     headerCells,
				Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: baseAccent, Width: 1.5},
			},
			{
				Height: 50,
				Cells:  bodyCells,
			},
			{
				Height: 20,
				Cells:  markerCells,
			},
		},
	}

	return grid, nil
}

// "columns" style geometry (go-slide-creator-j8t7o).
const (
	// journeyMaturityStairGapPt is the column gap; journeyMaturityStairRowGapPt
	// the gap under a header and above the marker.
	journeyMaturityStairGapPt    = 8.0
	journeyMaturityStairRowGapPt = 4.0
	// journeyMaturityMinRisePt is the smallest step up from one stage to the
	// next that still reads as a staircase; journeyMaturityMaxRisePt and
	// journeyMaturityMaxRiseFrac (the whole staircase's share of the content
	// height) cap it.
	journeyMaturityMinRisePt   = 10.0
	journeyMaturityMaxRisePt   = 44.0
	journeyMaturityMaxRiseFrac = 0.42
	// journeyMaturityHeaderMinPt is a one-line header's height.
	journeyMaturityHeaderMinPt = 40.0
	// journeyMaturityBaseMinPt is the least the first stage's description
	// column is tall, so a stage without copy still reads as a step.
	journeyMaturityBaseMinPt = 48.0
	// journeyMaturityMarkerPt is the "We are here" callout's height and
	// journeyMaturityMarkerInsetPt its text margin.
	journeyMaturityMarkerPt      = 46.0
	journeyMaturityMarkerInsetPt = 2.0
)

// journeyMaturityRisePt is the step up from one stage to the next: as tall as
// the content area allows up to the cap, and smaller when the descriptions
// need the height — stage i's column is i rises taller than the first, so it
// is the early stages' copy that limits the staircase. needs are the
// description heights per stage and fixedPt the header, marker and gaps.
func journeyMaturityRisePt(areaHPt, fixedPt float64, needs []float64) float64 {
	n := len(needs)
	if n < 2 {
		return 0
	}
	rise := math.Floor(math.Min(journeyMaturityMaxRisePt, areaHPt*journeyMaturityMaxRiseFrac/float64(n-1)))
	for ; rise > journeyMaturityMinRisePt; rise-- {
		base := areaHPt - fixedPt - float64(n-1)*rise
		fits := true
		for i, need := range needs {
			if need > base+float64(i)*rise {
				fits = false
				break
			}
		}
		if fits {
			return rise
		}
	}
	return journeyMaturityMinRisePt
}

// journeyMaturityStaircase lays the "columns" style's stage cells out as an
// ascending staircase. The grid keeps the flat style's three rows — headers,
// descriptions, markers — so cell paths and overlay anchors address the same
// (row, column) in both styles; the steps are made inside the rows. The
// header row is as tall as the whole staircase and stage i's header is inset
// to its own step; stage i's description reaches up from the description row
// to the underside of its header, and every description ends on the row's
// baseline.
func journeyMaturityStaircase(ctx ExpandContext, colsJSON []byte, headers, bodies, markers []*jsonschema.GridCellInput) *jsonschema.ShapeGridInput {
	n := len(headers)
	contentW, contentH := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, n, ctx.Gap(journeyMaturityStairGapPt))
	fonts := ctx.themeFonts()

	headerH := journeyMaturityHeaderMinPt
	needs := make([]float64, n)
	for i := range headers {
		headerH = math.Max(headerH, writtenFitHeightPt(fonts, headers[i].Shape.Text, colW, 0))
		// The description hangs from its header instead of floating in the
		// middle of a column that is taller for every later stage.
		if len(bodies[i].Shape.Text) > 0 {
			bodies[i].Shape.Text = withVerticalAlign(bodies[i].Shape.Text, "t")
			needs[i] = writtenFitHeightPt(fonts, bodies[i].Shape.Text, colW, 0)
		}
	}
	hasMarker := false
	for _, m := range markers {
		if m.Shape != nil && m.Shape.Geometry == journeyMaturityMarkerGeometry {
			hasMarker = true
			// The callout's own text box is the lower part of the shape; the
			// uniform margin on top of that leaves a 46pt marker no line.
			m.Shape.Text = withTextInsets(m.Shape.Text, journeyMaturityMarkerInsetPt)
		}
	}
	fixed := headerH + journeyMaturityStairRowGapPt
	if hasMarker {
		fixed += journeyMaturityMarkerPt + journeyMaturityStairRowGapPt
	}
	rise := journeyMaturityRisePt(contentH, fixed, needs)

	// The description row is as tall as the copy needs — stage i's column is
	// i rises taller than the first — so the columns end under the longest
	// description instead of running to the bottom of the slide as tall,
	// mostly empty blocks.
	base := journeyMaturityBaseMinPt
	for i, need := range needs {
		base = math.Max(base, need-float64(i)*rise)
	}
	base = math.Ceil(base)

	for i := 0; i < n; i++ {
		headers[i].InsetTop = float64(n-1-i) * rise
		headers[i].InsetBottom = float64(i) * rise
		bodies[i].BleedTop = float64(i) * rise
	}
	stairH := float64(n-1)*rise + headerH
	rows := []jsonschema.GridRowInput{
		{MinHeight: stairH, MaxHeight: stairH, Cells: headers},
		{MaxHeight: base, Cells: bodies},
	}
	if hasMarker {
		rows = append(rows, jsonschema.GridRowInput{MinHeight: journeyMaturityMarkerPt, MaxHeight: journeyMaturityMarkerPt, Cells: markers})
	}

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		Gap:           ctx.Gap(journeyMaturityStairGapPt),
		RowGap:        journeyMaturityStairRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}
}

// journeyMaturityMarkerLinePt is the accent outline of the "We are here"
// callout.
const journeyMaturityMarkerLinePt = 1.5

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type journeyMaturityParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type journeyMaturityTextObj struct {
	Paragraphs    []journeyMaturityParagraph `json:"paragraphs"`
	Align         string                     `json:"align"`
	VerticalAlign string                     `json:"vertical_align"`
}

func buildJourneyMaturityHeaderText(number int, label string, size float64) json.RawMessage {
	textObj := journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: fmt.Sprintf("%d. %s", number, label), Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildJourneyMaturityDescriptionText(description string, size float64) json.RawMessage {
	textObj := journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: description, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// journeyMaturityMarkerGeometry is an upright callout box with an arrow
// pointing up at the current stage (never rotated, so its label reads upright).
const journeyMaturityMarkerGeometry = "upArrowCallout"

func buildJourneyMaturityMarkerText(label string, size float64, color string) json.RawMessage {
	textObj := journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: label, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}
