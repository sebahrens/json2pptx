package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// process-flow pattern — left-to-right rectangles and diamonds with arrows
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&processFlow{})
}

type processFlow struct{}

func (p *processFlow) Name() string { return "process-flow" }
func (p *processFlow) Description() string {
	return "Left-to-right process flow with steps and decision points"
}
func (p *processFlow) UseWhen() string {
	return "Sequential steps in a single-lane workflow (3-8 steps); prefer swimlane when multiple actors own different steps, timeline-horizontal when stops are date-based"
}
func (p *processFlow) NotWhen() string {
	return "Steps belong to different actors/roles (use swimlane), stops are calendar-based milestones (use timeline-horizontal), or items are unordered (use icon-row or card-grid)"
}
func (p *processFlow) Version() int      { return 1 }
func (p *processFlow) CellsHint() string { return "3-8" }
func (p *processFlow) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "before-after"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (p *processFlow) SupportsCallout() bool        { return true }
func (p *processFlow) SupportsInlineMarkdown() bool { return true }

func (p *processFlow) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 1},
		{Columns: 4, Rows: 1},
		{Columns: 5, Rows: 1},
		{Columns: 6, Rows: 1},
		{Columns: 7, Rows: 1},
		{Columns: 8, Rows: 1},
	}
}

func (p *processFlow) ExemplarValues() any {
	return &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "Request", Type: "step"},
			{Label: "Review", Type: "decision"},
			{Label: "Approve", Type: "step"},
			{Label: "Deploy", Type: "step"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// processFlowDefaultFontPt returns the default step-label font size, shrinking
// it as the step count grows so 3-8 short labels stay on one line inside the
// progressively narrower boxes instead of wrapping awkwardly. n<=4 keeps the
// historical 12pt default; an explicit body_size override always wins via
// ResolveSize. Kept in lockstep between process-flow and process-flow-compact.
func processFlowDefaultFontPt(n int) float64 {
	switch {
	case n >= 7:
		return 9
	case n == 6:
		return 10
	case n == 5:
		return 11
	default:
		return 12
	}
}

// ProcessFlowStep is a single step in the process flow.
type ProcessFlowStep struct {
	Label string `json:"label"`
	Type  string `json:"type,omitempty"` // "step" (default), "decision", "chevron", or "arrow"
}

// ProcessFlowValues holds the steps for the process flow.
type ProcessFlowValues struct {
	Steps []ProcessFlowStep `json:"steps"`
}

// ProcessFlowOverrides is the standard text overrides. header_size is not
// supported (step labels are body text) and is rejected by Validate.
type ProcessFlowOverrides = TextOverrides

// ProcessFlowCellOverride is the shared per-cell override.
type ProcessFlowCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *processFlow) NewValues() any       { return &ProcessFlowValues{} }
func (p *processFlow) NewOverrides() any    { return &ProcessFlowOverrides{} }
func (p *processFlow) NewCellOverride() any { return &ProcessFlowCellOverride{} }

func processFlowLabelBudget(steps int, pointed bool) (wordLike, unbroken int) {
	if pointed {
		switch steps {
		case 5:
			return 76, 45
		case 6:
			return 42, 28
		case 7:
			return 30, 16
		case 8:
			return 17, 13
		default:
			return 80, 80
		}
	}
	if steps == 7 {
		return 80, 76
	}
	if steps >= 8 {
		return 80, 64
	}
	return 80, 80
}

func (p *processFlow) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*ProcessFlowValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	for i, step := range v.Steps {
		pointed := step.Type == "chevron" || step.Type == "arrow"
		wordBudget, unbrokenBudget := processFlowLabelBudget(len(v.Steps), pointed)
		longest := 0
		for _, word := range strings.Fields(step.Label) {
			longest = max(longest, runeLen(word))
		}
		if runeLen(step.Label) > wordBudget || longest > unbrokenBudget {
			warnings = append(warnings, fmt.Sprintf("%s: process-flow steps[%d].label has %d characters (longest unbroken run %d); this %d-step %s holds about %d word-like or %d wide unbroken characters — shorten the label, add word breaks, or use fewer steps", ErrCodeBodyTooLong, i, runeLen(step.Label), longest, len(v.Steps), step.Type, wordBudget, unbrokenBudget))
		}
	}
	return warnings
}

func (p *processFlow) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label": StringSchema(80).WithDescription("Step label text; chevron/arrow labels tighten to about 76/42/30/17 word-like characters at 5/6/7/8 steps, less for wide unbroken text"),
			"type":  EnumSchema("step", "decision", "chevron", "arrow").WithDescription("Shape type: rectangle (step), diamond (decision), chevron, or right-arrow (arrow)").WithDefault("step"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"steps": ArraySchema(stepSchema, 3, 8).WithDescription("Process steps left-to-right (3-8)"),
		},
		[]string{"steps"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchemaWithout("header_size"),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Left-to-right process flow with steps and decision points")
}

func (p *processFlow) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ProcessFlowValues)
	if !ok || vals == nil {
		return fmt.Errorf("process-flow: values must be *ProcessFlowValues, got %T", values)
	}

	const name = "process-flow"
	var errs []error

	// Validate cell_accent_mode
	if overrides != nil {
		if ovr, ok := overrides.(*ProcessFlowOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			// Step labels are body text; there is no header for header_size
			// to size (go-slide-creator-s1uvj.41).
			errs = append(errs, rejectUnusedTextOverrides(name, ovr, "header_size")...)
		}
	}

	if len(vals.Steps) < 3 {
		errs = append(errs, errMinItems(name, "steps", 3, len(vals.Steps), ""))
	}
	if len(vals.Steps) > 8 {
		errs = append(errs, errMaxItems(name, "steps", 8, len(vals.Steps), ""))
	}

	for i, step := range vals.Steps {
		path := fmt.Sprintf("steps[%d].label", i)
		if step.Label == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(step.Label) > 80 {
			errs = append(errs, errMaxLength(name, path, 80, runeLen(step.Label)))
		}
		if step.Type != "" && step.Type != "step" && step.Type != "decision" && step.Type != "chevron" && step.Type != "arrow" {
			errs = append(errs, newValidationError(name, fmt.Sprintf("steps[%d].type", i), ErrCodeUnknownEnum,
				fmt.Sprintf("process-flow: steps[%d].type must be \"step\", \"decision\", \"chevron\", or \"arrow\", got %q", i, step.Type),
				UseOneOfFix(fmt.Sprintf("steps[%d].type", i), []string{"step", "decision", "chevron", "arrow"})))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (p *processFlow) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ProcessFlowValues)
	if !ok {
		return nil, fmt.Errorf("process-flow: values must be *ProcessFlowValues, got %T", values)
	}
	ovr := &ProcessFlowOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ProcessFlowOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("process-flow: overrides must be *ProcessFlowOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	bodySize := ResolveSize(ovr.BodySize, processFlowFullFontPt(vals.Steps))
	cellAccentMode := ovr.CellAccentMode

	cells := make([]*jsonschema.GridCellInput, len(vals.Steps))
	for i, step := range vals.Steps {
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		geometry := "roundRect"
		pointed := false
		switch step.Type {
		case "decision":
			geometry = "diamond"
		case "chevron":
			geometry = "chevron"
			pointed = true
		case "arrow":
			geometry = "rightArrow"
			pointed = true
		}

		text := buildProcessFlowTextContent(pptx.ConvertMarkdownEmphasis(step.Label), bodySize)
		if pointed {
			text = buildProcessFlowPointedText(pptx.ConvertMarkdownEmphasis(step.Label), bodySize, chevronTextPadPt)
		}

		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: geometry,
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     text,
			},
		}
		if pointed {
			cell.Shape.Adjustments = map[string]int64{"adj": chevronAdj}
		}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*ProcessFlowCellOverride); ok2 && cellOvr.AccentBar {
				cell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
		}

		cells[i] = cell
	}

	colsJSON, _ := json.Marshal(len(vals.Steps))

	// Steps are capped at processFlowMaxHeightFrac of the content height
	// (go-slide-creator-7km8) instead of stretching into full-height pillars
	// with needle-thin diamonds; the grid centres the row vertically.
	pointedRow := allStepsPointed(vals.Steps)
	_, rowHeight := processFlowCellSize(ctx, len(vals.Steps), pointedRow)
	row := jsonschema.GridRowInput{
		Cells:     cells,
		Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: "dk1", Width: 1.5},
		MaxHeight: rowHeight,
	}
	// Chevrons point at the next step; an arrow drawn between them is a second
	// statement of the same thing, and it was being drawn straight through the
	// notch (go-slide-creator-czk4).
	if pointedRow {
		row.Connector = nil
	}

	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		Gap:           processFlowGapPt,
		Rows:          []jsonschema.GridRowInput{row},
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

// processFlowMaxHeightFrac caps process-flow steps at this share of the
// content height.
const processFlowMaxHeightFrac = 0.45

// processFlowFullFontPt uses the taller full-size row to promote short labels.
// Dense labels keep the conservative scale; compact flows retain their own
// 9–12pt scale via processFlowDefaultFontPt.
func processFlowFullFontPt(steps []ProcessFlowStep) float64 {
	base := processFlowDefaultFontPt(len(steps))
	for _, step := range steps {
		if runeLen(step.Label) > 22 {
			return base
		}
	}
	switch {
	case len(steps) <= 4:
		return 16
	case len(steps) <= 6:
		return 13
	default:
		return base
	}
}

// processFlowGapPt is the gap between steps, in points. The notch calculation
// reads it, so the two cannot drift.
const processFlowGapPt = 12.0

// processFlowCellSize is one step's width and the row's height in points.
// A row of pointed steps is additionally capped to half its step width: the
// notch is a fraction of the SHORTER side, so a tall chevron eats its own
// label — at four steps an uncapped row left 110pt of text width in a 198pt
// shape, and even "Board sign-off" broke mid-word (go-slide-creator-czk4).
func processFlowCellSize(ctx ExpandContext, steps int, pointed bool) (width, height float64) {
	if steps < 1 {
		steps = 1
	}
	contentW, contentH := contentAreaPt(ctx)
	width = (contentW - processFlowGapPt*float64(steps-1)) / float64(steps)
	height = math.Round(contentH * processFlowMaxHeightFrac)
	if pointed {
		height = math.Min(height, math.Round(width*chevronMaxAspectH))
	}
	return width, height
}

// allStepsPointed reports whether every step draws its own direction — a
// chevron or a right arrow — so a connector between them says nothing new.
func allStepsPointed(steps []ProcessFlowStep) bool {
	for _, s := range steps {
		if s.Type != "chevron" && s.Type != "arrow" {
			return false
		}
	}
	return len(steps) > 0
}

// buildProcessFlowPointedText adds padding inside the preset's text rectangle.
// Chevron and rightArrow presets already reserve their point/notch width in
// that rectangle; adding the notch again as bodyPr insets leaves almost no
// room for text in narrow steps.
func buildProcessFlowPointedText(content string, size, insetPt float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
		InsetLeft     float64     `json:"inset_left"`
		InsetRight    float64     `json:"inset_right"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
		InsetLeft:     insetPt,
		InsetRight:    insetPt,
	}

	data, _ := json.Marshal(textObj)
	return data
}

func buildProcessFlowTextContent(content string, size float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
