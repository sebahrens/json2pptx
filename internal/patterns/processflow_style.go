package patterns

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Restrained process-flow styling (go-slide-creator-xb06p). Four or five
// saturated accent slabs joined by black arrows read as a wall of colour; a
// consulting flow keeps the steps neutral, puts the accent on the connectors,
// and spends one solid accent fill on the step that matters — a highlighted
// step, or the flow's only decision.

// processFlowDecisionLinePt is the accent outline of a decision diamond that
// does not take the solid fill.
const processFlowDecisionLinePt = ProcessFlowDecisionLinePt

// The tinted process-flow look, shared with the native process_flow diagram
// (internal/generator) so a flow reads the same whichever route draws it
// (go-slide-creator-6shxx): steps on a neutral dk1 tint with no outline,
// decisions outlined in the accent, connectors in the accent on a 2pt line
// with the large arrowhead.
const (
	// ProcessFlowStepTintPct is the neutral step fill: dk1 at this ink
	// coverage (lumMod pct, lumOff 100-pct).
	ProcessFlowStepTintPct = NeutralTint8
	// ProcessFlowDecisionLinePt is a decision diamond's accent outline.
	ProcessFlowDecisionLinePt = 1.5
	// ProcessFlowConnectorLinePt and ProcessFlowConnectorHead are the
	// connector's line width and arrowhead size.
	ProcessFlowConnectorLinePt = processFlowConnectorLinePt
	ProcessFlowConnectorHead   = processFlowConnectorHead
	// ProcessFlowConnectorMinPt is the shortest connector either route draws.
	ProcessFlowConnectorMinPt = processFlowConnectorMinPt
)

// processFlowDiamondInsetPt is the text margin inside a decision diamond. The
// preset's text rectangle is already the inner half of the shape; the uniform
// 0.5 cm margin on top of it left a content-sized diamond no room for a
// two-word question.
const processFlowDiamondInsetPt = 3.0

// Arrow steps (go-slide-creator-fx48s). The rightArrow preset's text rectangle
// is its shaft, and the preset's shaft is half the shape's height: with the
// uniform 0.5 cm margin above and below, a 60pt step had 2pt left for its
// label and every renderer shrank it to about 3pt. The step is drawn as a
// block arrow instead — a shaft of processFlowArrowShaftAdj of the height
// under a head half the height long — its label keeps processFlowArrowInsetPt
// above and below, and the row is sized so the shaft holds the label at the
// flow's type size (processFlowWrittenNeedPt).
const (
	// processFlowArrowShaftAdj is the shaft height (rightArrow adj1, 1/100000
	// of the shape height).
	processFlowArrowShaftAdj = 70000
	// processFlowArrowHeadAdj is the head length (rightArrow adj2, 1/100000
	// of the shape's shorter side).
	processFlowArrowHeadAdj = 50000
	// processFlowArrowInsetPt is the label's margin above and below, inside
	// the shaft.
	processFlowArrowInsetPt = 3.0
)

// withTextInsets sets all four text insets of a {paragraphs} payload.
func withTextInsets(text json.RawMessage, pt float64) json.RawMessage {
	return withTextInsetSides(text, pt, "inset_left", "inset_top", "inset_right", "inset_bottom")
}

// withTextInsetSides sets the named text insets of a {paragraphs} payload.
func withTextInsetSides(text json.RawMessage, pt float64, sides ...string) json.RawMessage {
	var obj map[string]any
	if json.Unmarshal(text, &obj) != nil || obj == nil {
		return text
	}
	for _, k := range sides {
		obj[k] = pt
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return text
	}
	return out
}

// processFlowOverridesSchema is the text overrides minus header_size, plus
// the step style.
func processFlowOverridesSchema() *Schema {
	s := textOverridesSchemaWithout("header_size")
	s.raw.Properties["style"] = EnumSchema(processFlowStyles...).WithDescription("chevrons (default): steps are interlocking arrows in a light accent tint, a pentagon first and chevrons tucked under the point before them, with no connector lines; the solid accent fills only the highlighted step (steps[].highlight) or, when none is highlighted, a flow's single decision — further decisions get an accent outline. tinted: the flowchart look, neutral boxes joined by accent connector arrows. solid: boxes all filled with the accent, cell_accent_mode applies (legacy look)").WithDefault(processFlowStyleChevrons)
	return s
}

// processFlowStepSchema is one step: label, shape type and highlight flag.
func processFlowStepSchema(labelDesc string) *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"label":     StringSchema(80).WithDescription(labelDesc),
			"type":      EnumSchema("step", "decision", "chevron", "arrow").WithDescription("Shape type: step (an interlocking arrow; a rectangle in the tinted / solid styles), diamond (decision), chevron (free-standing, deep notch), or right-arrow (arrow)").WithDefault("step"),
			"highlight": BooleanSchema().WithDescription("Fill this step with the solid accent as the flow's one emphasised step (at most one step). Steps are otherwise a light tint").WithDefault(false),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)
}

// validateProcessFlowStyle checks the overrides and highlight flags shared by
// process-flow and process-flow-compact.
func validateProcessFlowStyle(name string, steps []ProcessFlowStep, overrides any) []error {
	var errs []error
	if ovr, ok := overrides.(*ProcessFlowOverrides); ok && ovr != nil {
		if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
			errs = append(errs, err)
		}
		// Step labels are body text; there is no header for header_size
		// to size (go-slide-creator-s1uvj.41).
		errs = append(errs, rejectUnusedTextOverrides(name, &ovr.TextOverrides, "header_size")...)
		if ovr.Style != "" && !slices.Contains(processFlowStyles, ovr.Style) {
			errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, processFlowStyles))
		}
	}
	highlighted := 0
	for i, st := range steps {
		if !st.Highlight {
			continue
		}
		highlighted++
		if highlighted > 1 {
			path := fmt.Sprintf("steps[%d].highlight", i)
			errs = append(errs, newValidationError(name, path, ErrCodeOutOfRange,
				fmt.Sprintf("%s: %s: at most one step may set highlight — a second highlight dilutes the first; keep the one step the slide is about", name, path),
				RemoveFieldFix(path)))
		}
	}
	return errs
}

// buildProcessFlowCells builds the step cells shared by process-flow and
// process-flow-compact. look is the flow's style and, for the default chevron
// look, the geometry its plain steps are drawn at (processflow_chevrons.go).
func buildProcessFlowCells(ctx ExpandContext, steps []ProcessFlowStep, ovr *ProcessFlowOverrides, cellOverrides map[int]any, bodySize float64, look processFlowLook) []*jsonschema.GridCellInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	solid := ovr.Style == processFlowStyleSolid
	emphasis := processFlowEmphasisIndex(steps)
	// The default look tints its steps with the accent; the flowchart look
	// keeps them neutral.
	stepTone, stepInk := neutralTone(ProcessFlowStepTintPct), "dk1"
	if look.chevrons {
		stepTone = inactiveTintTone(baseAccent)
		stepInk = readableTextOn(ctx, stepTone, "dk1")
	}
	cells := make([]*jsonschema.GridCellInput, len(steps))
	number := 0
	for i, step := range steps {
		accent := baseAccent
		if solid {
			accent = ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		}
		geometry := "roundRect"
		pointed, interlocked := false, false
		switch step.Type {
		case "decision":
			geometry = "diamond"
		case "chevron":
			geometry = "chevron"
			pointed = true
		case "arrow":
			geometry = "rightArrow"
			pointed = true
		default:
			if look.chevrons {
				geometry = look.stepGeometry(i)
				interlocked = true
			}
		}
		if step.Type != "decision" {
			number++
		}

		fill, ink, line := stepTone.fillJSON(), stepInk, noLine
		emphasised := solid || i == emphasis
		switch {
		case emphasised:
			fill, ink, line = json.RawMessage(fmt.Sprintf(`"%s"`, accent)), "lt1", nil
		case step.Type == "decision":
			data, _ := json.Marshal(map[string]any{"color": accent, "width": processFlowDecisionLinePt})
			line = data
		}

		label := pptx.ConvertMarkdownEmphasis(step.Label)
		text := buildProcessFlowTextContent(label, bodySize, ink)
		if pointed {
			text = buildProcessFlowPointedText(label, bodySize, ink)
		}
		if geometry == "diamond" {
			text = withTextInsets(text, processFlowDiamondInsetPt)
		}
		if interlocked {
			numeralInk := ink
			if !emphasised {
				numeralInk = processFlowNumeralInk(ctx, stepTone, accent, ink)
			}
			text = look.stepText(label, bodySize, ink, number, numeralInk)
		}

		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: geometry,
				Fill:     fill,
				Line:     line,
				Text:     text,
			},
		}
		switch {
		case interlocked:
			cell.BleedLeft = look.bleedPt(i)
		case geometry == "chevron":
			cell.Shape.Adjustments = map[string]int64{"adj": chevronAdj}
		case geometry == "rightArrow":
			cell.Shape.Adjustments = map[string]int64{"adj1": processFlowArrowShaftAdj, "adj2": processFlowArrowHeadAdj}
			cell.Shape.Text = withTextInsetSides(text, processFlowArrowInsetPt, "inset_top", "inset_bottom")
		}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*ProcessFlowCellOverride); ok2 {
				applyCellTextOverride(cell, cellOvr)
				if cellOvr.AccentBar {
					cell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    4,
					}
				}
			}
		}
		cells[i] = cell
	}
	return cells
}

// processFlowEmphasisIndex is the step that takes the solid accent in the
// tinted style: the highlighted step, else the flow's only decision, else
// none (-1).
func processFlowEmphasisIndex(steps []ProcessFlowStep) int {
	decision, decisions := -1, 0
	for i, st := range steps {
		if st.Highlight {
			return i
		}
		if st.Type == "decision" {
			decision = i
			decisions++
		}
	}
	if decisions == 1 {
		return decision
	}
	return -1
}

// processFlowConnector is the arrow between steps, drawn in the accent (it
// was dk1, the one black connector in the pattern family) on a 2pt line with
// the large arrowhead, so the direction reads at presentation size.
func processFlowConnector(ctx ExpandContext, ovr *ProcessFlowOverrides) *jsonschema.ConnectorSpecInput {
	if processFlowChevronStyle(ovr) {
		// The interlocking arrows say which step follows which.
		return nil
	}
	return &jsonschema.ConnectorSpecInput{
		Style: "arrow",
		Color: ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent),
		Width: processFlowConnectorLinePt,
		Head:  processFlowConnectorHead,
	}
}
