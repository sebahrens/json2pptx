package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// ---------------------------------------------------------------------------
// next-steps pattern — the closing slide a consulting deck ends on: numbered
// action rows (action · owner · date) and a "Decisions requested" band.
// ---------------------------------------------------------------------------
//
// Layout (go-slide-creator-7lzdh, design review C6):
//
//        Action                                   Owner        Date
//   ─────────────────────────────────────────────────────────────────
//   01   Confirm the pilot scope with the COO     J. Smith     15 Oct
//   ─────────────────────────────────────────────────────────────────
//   02   Stand up the data workstream             A. Lee       Nov
//
//   ▌ Decisions requested
//   ▌ • Approve the €2.4M phase-1 budget
//
// A partner replaces "Thank you" with this. The rows share the agenda's rule
// language (serif accent numerals, 0.5pt rules, no tiles). The decisions band
// is the takeaway treatment: a left accent rule, bold text, no outline and no
// tinted fill. Owner and date columns are dropped when no action carries one.

func init() {
	Default().Register(&nextSteps{})
}

type nextSteps struct{}

// Pattern budgets.
const (
	nextStepsMinActions    = 2
	nextStepsMaxActions    = 6
	nextStepsMaxDecisions  = 3
	nextStepsActionMax     = 90
	nextStepsOwnerMax      = 30
	nextStepsDateMax       = 20
	nextStepsDecisionMax   = 120
	nextStepsLabelMax      = 40
	nextStepsDefaultLabel  = "Decisions requested"
	nextStepsRulePt        = 0.5
	nextStepsRowGapPt      = 2.0
	nextStepsBandGapPt     = 14.0
	nextStepsMinBandGapPt  = 6.0
	nextStepsBandBarPt     = 3.0
	nextStepsOwnerPct      = 22.0
	nextStepsDatePct       = 15.0
	nextStepsMinFillFrac   = 0.55
	nextStepsHeaderAlpha   = 60.0
	nextStepsNumberFont    = "+mj-lt"
	nextStepsHeaderSizePt  = tokens.TypeScaleBodyPt
	nextStepsBandLabelSize = sizeLabelPt
)

// nextStepsScales steps numeral / action / meta sizes down only when the
// default rows do not fit the content area. The last step sets the actions
// and decisions at the 12pt floor: the pattern picks a readable size itself
// rather than handing the writer rows it can only fit by autofit shrink
// (go-slide-creator-k3eb3).
//
// Every size is a scale step (go-slide-creator-vmdfm).
var nextStepsScales = [][3]float64{{scaleLeadPt, scaleSubheadPt, scaleSubheadPt}, {scaleLeadPt, scaleSubheadPt, scaleBodyPt}, {scaleSubheadPt, scaleBodyPt, scaleBodyPt}}

func (n *nextSteps) Name() string { return "next-steps" }
func (n *nextSteps) Description() string {
	return "Closing next-steps slide: 2-6 numbered action rows (action / owner / date) separated by rules, plus an optional 'Decisions requested' band with a left accent rule"
}
func (n *nextSteps) UseWhen() string {
	return "The deck's closing slide: what happens next, who owns each action and by when, and the decisions the audience is asked to take; use it instead of a 'Thank you' closer"
}
func (n *nextSteps) NotWhen() string {
	return "The steps are a process to explain rather than actions to assign (use numbered-step-strip or process-flow), the plan is a dated multi-phase schedule (use phase-roadmap), or the slide weighs options rather than assigning actions (use table-highlight / the decision kind)"
}
func (n *nextSteps) Version() int      { return 1 }
func (n *nextSteps) CellsHint() string { return "2-6 (actions) + 0-3 decisions" }
func (n *nextSteps) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"conclude"},
		PairsWith:     []string{"exec-summary", "phase-roadmap", "table-highlight"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (n *nextSteps) SupportsInlineMarkdown() bool { return true }

func (n *nextSteps) ExemplarValues() any {
	return &NextStepsValues{
		Actions: []NextStepsAction{
			{Action: "Confirm pilot scope and success metrics with the COO", Owner: "J. Smith", Date: "15 Oct"},
			{Action: "Stand up the data workstream and secure access", Owner: "A. Lee", Date: "31 Oct"},
			{Action: "Present the phase-1 business case to the board", Owner: "CFO office", Date: "Nov board"},
		},
		Decisions: []string{"Approve the €2.4M phase-1 budget", "Nominate an executive sponsor for the pilot"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// NextStepsAction is one action row.
type NextStepsAction struct {
	Action string `json:"action"`
	Owner  string `json:"owner,omitempty"`
	Date   string `json:"date,omitempty"`
}

// NextStepsValues holds the action rows and the decisions requested.
type NextStepsValues struct {
	Actions        []NextStepsAction `json:"actions"`
	Decisions      []string          `json:"decisions,omitempty"`
	DecisionsLabel string            `json:"decisions_label,omitempty"`
}

// NextStepsOverrides are the pattern-level knobs.
type NextStepsOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	ActionSize     float64 `json:"action_size,omitempty"`
}

// NextStepsCellOverride is the shared per-cell override, indexed by action.
type NextStepsCellOverride = CellOverride

func (n *nextSteps) NewValues() any       { return &NextStepsValues{} }
func (n *nextSteps) NewOverrides() any    { return &NextStepsOverrides{} }
func (n *nextSteps) NewCellOverride() any { return &NextStepsCellOverride{} }

func (n *nextSteps) Schema() *Schema {
	actionSchema := ObjectSchema(
		map[string]*Schema{
			"action": StringSchema(nextStepsActionMax).WithDescription("What will be done, as an imperative (≤90 chars): \"Confirm pilot scope with the COO\""),
			"owner":  StringSchema(nextStepsOwnerMax).WithDescription("Who owns it (≤30 chars); the owner column is dropped when no action has one"),
			"date":   StringSchema(nextStepsDateMax).WithDescription("When it is due (≤20 chars): \"15 Oct\", \"Q1 2027\"; the date column is dropped when no action has one"),
		},
		[]string{"action"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"actions":         ArraySchema(actionSchema, nextStepsMinActions, nextStepsMaxActions).WithDescription("2-6 actions, in the order they happen"),
			"decisions":       ArraySchema(StringSchema(nextStepsDecisionMax).WithDescription("One decision the audience is asked to take (≤120 chars)"), 0, nextStepsMaxDecisions).WithDescription("0-3 decisions requested, rendered in a band under the actions with a left accent rule (no outline, no fill)"),
			"decisions_label": StringSchema(nextStepsLabelMax).WithDescription("Band label (default \"Decisions requested\")"),
		},
		[]string{"actions"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color for the numerals and the band rule (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"action_size":     NumberSchema(12, 24).WithDescription("Action text size in points (default 16, stepping to 14 when six long actions do not fit); owner and date are 2pt smaller"),
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
	}).WithDescription("Next steps: 2-6 numbered action rows (action / owner / date) separated by rules, plus an optional 'Decisions requested' band")
}

func (n *nextSteps) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*NextStepsValues)
	if !ok || vals == nil {
		return fmt.Errorf("next-steps: values must be *NextStepsValues, got %T", values)
	}
	const name = "next-steps"
	var errs []error
	if overrides != nil {
		ovr, ok := overrides.(*NextStepsOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("next-steps: overrides must be *NextStepsOverrides, got %T", overrides))
		} else if ovr.ActionSize != 0 && (ovr.ActionSize < 12 || ovr.ActionSize > 24) {
			errs = append(errs, errOutOfRange(name, "overrides.action_size", 12, 24, int(ovr.ActionSize)))
		}
	}
	if len(vals.Actions) < nextStepsMinActions {
		errs = append(errs, errMinItems(name, "actions", nextStepsMinActions, len(vals.Actions), "(hint: a single action belongs in the takeaway of the previous slide)"))
	}
	if len(vals.Actions) > nextStepsMaxActions {
		errs = append(errs, errMaxItems(name, "actions", nextStepsMaxActions, len(vals.Actions), "(hint: keep the six that need an owner now; move the rest to an appendix roadmap)"))
	}
	for i, a := range vals.Actions {
		for _, f := range []struct {
			field, text string
			max         int
			required    bool
		}{
			{"action", a.Action, nextStepsActionMax, true},
			{"owner", a.Owner, nextStepsOwnerMax, false},
			{"date", a.Date, nextStepsDateMax, false},
		} {
			path := fmt.Sprintf("actions[%d].%s", i, f.field)
			switch {
			case f.required && strings.TrimSpace(f.text) == "":
				errs = append(errs, errRequired(name, path))
			case runeLen(f.text) > f.max:
				errs = append(errs, errMaxLength(name, path, f.max, runeLen(f.text)))
			}
		}
	}
	if len(vals.Decisions) > nextStepsMaxDecisions {
		errs = append(errs, errMaxItems(name, "decisions", nextStepsMaxDecisions, len(vals.Decisions), "(hint: ask for the decisions that block the first action)"))
	}
	for i, d := range vals.Decisions {
		path := fmt.Sprintf("decisions[%d]", i)
		switch {
		case strings.TrimSpace(d) == "":
			errs = append(errs, errRequired(name, path))
		case runeLen(d) > nextStepsDecisionMax:
			errs = append(errs, errMaxLength(name, path, nextStepsDecisionMax, runeLen(d)))
		}
	}
	if runeLen(vals.DecisionsLabel) > nextStepsLabelMax {
		errs = append(errs, errMaxLength(name, "decisions_label", nextStepsLabelMax, runeLen(vals.DecisionsLabel)))
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Actions), "(index = action)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

// nextStepsLayout is the measured geometry at one type scale.
type nextStepsLayout struct {
	cols                                    []float64
	hasOwner, hasDate                       bool
	numberSize, actionSize, metaSize        float64
	headerPt                                float64
	rowPt                                   []float64
	bandPt                                  float64 // 0 = no decisions band
	bandGapPt                               float64 // space above the band
	rowGapPt                                float64 // nextStepsRowGapPt on the template grid
	decisionSize                            float64
	numberCol, actionCol, ownerCol, dateCol int
}

func (l nextStepsLayout) natural() float64 {
	n := len(l.rowPt)
	h := l.headerPt + float64(n)*nextStepsRulePt
	rows := 1 + 2*n
	for _, r := range l.rowPt {
		h += r
	}
	if l.bandPt > 0 {
		h += l.bandGapPt + l.bandPt
		rows += 2
	}
	return h + float64(rows-1)*l.rowGapPt
}

func layoutNextSteps(ctx ExpandContext, vals *NextStepsValues, ovr *NextStepsOverrides) nextStepsLayout {
	_, areaH := sizingAreaPt(ctx)
	scales := nextStepsScales
	if ovr.ActionSize > 0 {
		scales = [][3]float64{{scales[0][0], ovr.ActionSize, math.Max(scaleBodyPt, ovr.ActionSize-2)}}
	}
	fit := func(sc [3]float64) (nextStepsLayout, bool) {
		lay := measureNextSteps(ctx, vals, sc[0], sc[1], sc[2])
		if lay.natural() <= areaH {
			return lay, true
		}
		// Too tall at this step: the space above the band gives way before
		// the type steps down.
		if lay.bandPt > 0 {
			lay.bandGapPt = ctx.Gap(nextStepsMinBandGapPt)
		}
		return lay, lay.natural() <= areaH
	}
	var over nextStepsLayout
	for i, sc := range scales {
		lay, ok := fit(sc)
		if ok {
			return lay
		}
		// The floor step is taken only when it makes the list fit: content
		// that overflows even there keeps the larger step, since the writer
		// shrinks it either way and a smaller start only ends smaller.
		if i < len(scales)-1 || len(scales) < 3 {
			over = lay
		}
	}
	return over
}

func measureNextSteps(ctx ExpandContext, vals *NextStepsValues, numberSize, actionSize, metaSize float64) nextStepsLayout {
	areaW, _ := sizingAreaPt(ctx)
	lay := nextStepsLayout{numberSize: numberSize, actionSize: actionSize, metaSize: metaSize, decisionSize: actionSize, bandGapPt: ctx.Gap(nextStepsBandGapPt), rowGapPt: ctx.Gap(nextStepsRowGapPt),
		ownerCol: -1, dateCol: -1}
	for _, a := range vals.Actions {
		lay.hasOwner = lay.hasOwner || strings.TrimSpace(a.Owner) != ""
		lay.hasDate = lay.hasDate || strings.TrimSpace(a.Date) != ""
	}
	numberPct := math.Min(15, math.Max(5, math.Ceil((numberSize*1.3+2*defaultShapeInsetLRPt)/areaW*100)))
	actionPct := 100 - numberPct
	lay.cols = []float64{numberPct}
	lay.numberCol, lay.actionCol = 0, 1
	if lay.hasOwner {
		actionPct -= nextStepsOwnerPct
	}
	if lay.hasDate {
		actionPct -= nextStepsDatePct
	}
	lay.cols = append(lay.cols, actionPct)
	if lay.hasOwner {
		lay.ownerCol = len(lay.cols)
		lay.cols = append(lay.cols, nextStepsOwnerPct)
	}
	if lay.hasDate {
		lay.dateCol = len(lay.cols)
		lay.cols = append(lay.cols, nextStepsDatePct)
	}

	// Every row is sized to the written fit of the cells it will hold, at
	// their real column widths: the pattern's own metric model alone let the
	// writer store rows shrunk below the floor (go-slide-creator-k3eb3).
	lay.headerPt = math.Ceil(math.Max(nextStepsHeaderSizePt*sizingLineSpacing+2*sizingInsetTBPt,
		writtenFitHeightPt(nextStepsHeaderCell("Action", 100).Shape.Text, areaW*actionPct/100, 0)))
	numberRow := numberSize*sizingLineSpacing + 2*sizingInsetTBPt
	for i, a := range vals.Actions {
		h := math.Max(numberRow, writtenFitHeightPt(nextStepsNumberCell(i, numberSize, "dk1").Shape.Text, areaW*numberPct/100, 0))
		h = math.Max(h, sizedBlockHeightPt(ctx, []sizedPara{{text: a.Action, sizePt: actionSize}}, areaW*actionPct/100))
		h = math.Max(h, writtenFitHeightPt(nextStepsActionCell(a.Action, actionSize).Shape.Text, areaW*actionPct/100, 0))
		if lay.hasOwner {
			h = math.Max(h, sizedBlockHeightPt(ctx, []sizedPara{{text: a.Owner, sizePt: metaSize}}, areaW*nextStepsOwnerPct/100))
			h = math.Max(h, writtenFitHeightPt(nextStepsOwnerCell(a.Owner, metaSize).Shape.Text, areaW*nextStepsOwnerPct/100, 0))
		}
		if lay.hasDate {
			h = math.Max(h, sizedBlockHeightPt(ctx, []sizedPara{{text: a.Date, sizePt: metaSize}}, areaW*nextStepsDatePct/100))
			h = math.Max(h, writtenFitHeightPt(nextStepsDateCell(a.Date, metaSize).Shape.Text, areaW*nextStepsDatePct/100, 0))
		}
		lay.rowPt = append(lay.rowPt, math.Ceil(h))
	}
	if decisions := nonEmptyStrings(vals.Decisions); len(decisions) > 0 {
		paras := []sizedPara{{text: nextStepsLabel(vals), sizePt: nextStepsBandLabelSize, bold: true, spaceAfterPt: 4}}
		for _, d := range decisions {
			paras = append(paras, sizedPara{text: d, sizePt: lay.decisionSize, bold: true, spaceAfterPt: 2, bullet: true})
		}
		band := nextStepsBandCell(vals, lay.decisionSize, "dk1")
		lay.bandPt = math.Ceil(math.Max(sizedBlockHeightPt(ctx, paras, areaW-nextStepsBandBarPt),
			writtenFitHeightPt(band.Shape.Text, areaW-nextStepsBandBarPt, 0)))
	}
	return lay
}

// The row cells below are shared by measureNextSteps and Expand, so a row is
// sized on exactly the text it is written with.

func nextStepsHeaderCell(label string, alpha float64) *jsonschema.GridCellInput {
	return nextStepsTextCell("b", nextStepsParagraph{Content: label, Size: nextStepsHeaderSizePt, Bold: true, Color: "dk1", Alpha: alpha})
}

func nextStepsNumberCell(i int, size float64, ink string) *jsonschema.GridCellInput {
	return nextStepsTextCell("ctr", nextStepsParagraph{Content: fmt.Sprintf("%02d", i+1), Size: size, Color: ink, Font: nextStepsNumberFont})
}

func nextStepsActionCell(action string, size float64) *jsonschema.GridCellInput {
	return nextStepsTextCell("ctr", nextStepsParagraph{Content: pptx.ConvertMarkdownEmphasis(action), Size: size, Color: "dk1"})
}

func nextStepsOwnerCell(owner string, size float64) *jsonschema.GridCellInput {
	return nextStepsTextCell("ctr", nextStepsParagraph{Content: owner, Size: size, Color: "dk1"})
}

func nextStepsDateCell(date string, size float64) *jsonschema.GridCellInput {
	return nextStepsTextCell("ctr", nextStepsParagraph{Content: date, Size: size, Bold: true, Color: "dk1"})
}

// nextStepsBandCell is the "Decisions requested" band text: label + bullets.
func nextStepsBandCell(vals *NextStepsValues, size float64, labelInk string) *jsonschema.GridCellInput {
	paras := []nextStepsParagraph{{Content: nextStepsLabel(vals), Size: nextStepsBandLabelSize, Bold: true, Color: labelInk, SpaceAfter: 4}}
	for _, d := range nonEmptyStrings(vals.Decisions) {
		paras = append(paras, nextStepsParagraph{Content: pptx.ConvertMarkdownEmphasis(d), Size: size, Bold: true, Color: "dk1", SpaceAfter: 2, Bullet: true})
	}
	return nextStepsTextCell("ctr", paras...)
}

func nextStepsLabel(vals *NextStepsValues) string {
	if l := strings.TrimSpace(vals.DecisionsLabel); l != "" {
		return l
	}
	return nextStepsDefaultLabel
}

func nonEmptyStrings(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// nextStepsParagraph is one run of next-steps text; the header labels carry
// an opacity and the numerals the heading font.
type nextStepsParagraph struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	Font       string  `json:"font,omitempty"`
	Alpha      float64 `json:"alpha,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
	Bullet     bool    `json:"bullet,omitempty"`
}

func nextStepsTextCell(vAlign string, paras ...nextStepsParagraph) *jsonschema.GridCellInput {
	for i := range paras {
		paras[i].Align = "l"
	}
	text, _ := json.Marshal(struct {
		Paragraphs    []nextStepsParagraph `json:"paragraphs"`
		Align         string               `json:"align"`
		VerticalAlign string               `json:"vertical_align"`
	}{paras, "l", vAlign})
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Line:     json.RawMessage(`"none"`),
		Text:     text,
	}}
}

func (n *nextSteps) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*NextStepsValues)
	if !ok || vals == nil {
		return nil, fmt.Errorf("next-steps: values must be *NextStepsValues, got %T", values)
	}
	ovr := &NextStepsOverrides{}
	if overrides != nil {
		var ovrOk bool
		if ovr, ovrOk = overrides.(*NextStepsOverrides); !ovrOk {
			return nil, fmt.Errorf("next-steps: overrides must be *NextStepsOverrides, got %T", overrides)
		}
	}
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := layoutNextSteps(ctx, vals, ovr)
	nCols := len(lay.cols)
	ruleFill := fillTone{Color: "dk1", Alpha: 30}.fillJSON()
	rule := func() jsonschema.GridRowInput {
		return jsonschema.GridRowInput{
			MinHeight: nextStepsRulePt, MaxHeight: nextStepsRulePt,
			Cells: []*jsonschema.GridCellInput{{
				ColSpan: nCols,
				Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ruleFill, Line: json.RawMessage(`"none"`)},
			}},
		}
	}

	// Column headers: small, bold, dk1 at 60% — present but quieter than
	// the rows they label.
	headerAlpha := readableDimAlpha(ctx, "dk1", nextStepsHeaderAlpha, 4.5)
	numberInk := accentInkOnLight(ctx, accent, 3.0)
	header := func(label string) *jsonschema.GridCellInput {
		return nextStepsHeaderCell(label, headerAlpha)
	}
	headerCells := []*jsonschema.GridCellInput{{}, header("Action")}
	if lay.hasOwner {
		headerCells = append(headerCells, header("Owner"))
	}
	if lay.hasDate {
		headerCells = append(headerCells, header("Date"))
	}
	rows := []jsonschema.GridRowInput{{MinHeight: lay.headerPt, MaxHeight: lay.headerPt, Cells: headerCells}}
	itemRow := []bool{false}

	for i, a := range vals.Actions {
		rows = append(rows, rule())
		itemRow = append(itemRow, false)
		cells := []*jsonschema.GridCellInput{
			nextStepsNumberCell(i, lay.numberSize, numberInk),
			nextStepsActionCell(a.Action, lay.actionSize),
		}
		if co, ok := cellOverrides[i].(*NextStepsCellOverride); ok && co != nil {
			applyCellTextOverride(cells[1], co)
			if co.AccentBar {
				cells[1].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
			}
		}
		if lay.hasOwner {
			cells = append(cells, nextStepsOwnerCell(a.Owner, lay.metaSize))
		}
		if lay.hasDate {
			cells = append(cells, nextStepsDateCell(a.Date, lay.metaSize))
		}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: lay.rowPt[i], MaxHeight: lay.rowPt[i], Cells: cells})
		itemRow = append(itemRow, true)
	}

	if lay.bandPt > 0 {
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: lay.bandGapPt, MaxHeight: lay.bandGapPt,
			Cells: []*jsonschema.GridCellInput{{ColSpan: nCols}},
		})
		itemRow = append(itemRow, false)
		band := nextStepsBandCell(vals, lay.decisionSize, accentInkOnLight(ctx, accent, 4.5))
		band.ColSpan = nCols
		band.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: nextStepsBandBarPt}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: lay.bandPt, MaxHeight: lay.bandPt, Cells: []*jsonschema.GridCellInput{band}})
		itemRow = append(itemRow, false)
	}

	_, areaH := sizingAreaPt(ctx)
	if lay.natural() <= areaH {
		fillCappedRows(ctx, rows, ctx.Gap(nextStepsRowGapPt), nextStepsMinFillFrac, func(i int) bool { return itemRow[i] })
	}

	colsJSON, _ := json.Marshal(lay.cols)
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        0.1,
		RowGap:        ctx.Gap(nextStepsRowGapPt),
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// PostExpandWarnings reports a slide the rows cannot fit even at the smaller
// type scale.
func (n *nextSteps) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*NextStepsValues)
	if !ok || v == nil || len(v.Actions) == 0 {
		return nil
	}
	ovr, _ := overrides.(*NextStepsOverrides)
	if ovr == nil {
		ovr = &NextStepsOverrides{}
	}
	lay := layoutNextSteps(ctx, v, ovr)
	_, areaH := sizingAreaPt(ctx)
	if need := lay.natural(); need > areaH+1 {
		return []string{fmt.Sprintf(
			"%s: next-steps needs %.0fpt at the smallest type scale but the content area holds about %.0fpt — shorten the actions to one line each, drop to the decisions that block the first action, or use fewer actions",
			ErrCodeBodyTooLong, need, areaH)}
	}
	return nil
}
