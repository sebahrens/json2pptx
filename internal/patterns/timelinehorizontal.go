package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// timeline-horizontal pattern — linear timeline with N stops (3–7)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&timelineHorizontal{})
}

type timelineHorizontal struct{}

func (th *timelineHorizontal) Name() string        { return "timeline-horizontal" }
func (th *timelineHorizontal) Description() string { return "Linear horizontal timeline with stops" }
func (th *timelineHorizontal) UseWhen() string {
	return "Linear sequence of 3-7 date-based or ordered milestones; prefer roadmap-phased when multiple parallel workstreams exist, process-flow when steps are actions not events"
}
func (th *timelineHorizontal) NotWhen() string {
	return "Multiple parallel workstreams across time (use roadmap-phased), steps are actions/decisions not milestones (use process-flow), or events belong to different actors (use swimlane)"
}
func (th *timelineHorizontal) Version() int      { return 1 }
func (th *timelineHorizontal) CellsHint() string { return "3-7" }
func (th *timelineHorizontal) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "roadmap-phased", "card-grid"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}

func (th *timelineHorizontal) ExemplarValues() any {
	v := TimelineHorizontalValues{
		{Label: "Discovery", Date: "Q1 2025", Body: "Baseline costs and interview 40 stakeholders"},
		{Label: "Pilot", Date: "Q2 2025", Body: "Run the new model in two regions"},
		{Label: "Scale-up", Date: "Q3 2025", Body: "Roll out to all twelve markets"},
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// TimelineStop is a single stop on the timeline: a label, optional date, optional body.
type TimelineStop struct {
	Label   string `json:"label"`
	Date    string `json:"date,omitempty"`
	EndDate string `json:"end_date,omitempty"` // Only used in gantt style: the bar ends at the end of this period
	Body    string `json:"body,omitempty"`
}

// TimelineHorizontalValues is the values type: 3–7 timeline stops.
type TimelineHorizontalValues = []TimelineStop

// TimelineHorizontalOverrides contains pattern-level overrides for timeline-horizontal.
type TimelineHorizontalOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	LabelSize      float64 `json:"label_size,omitempty"`
	DateSize       float64 `json:"date_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
	Style          string  `json:"style,omitempty"` // "dots" (default), "chevron", or "gantt"
}

// TimelineHorizontalCellOverride is an alias for the shared CellOverride struct.
type TimelineHorizontalCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (th *timelineHorizontal) NewValues() any       { return &TimelineHorizontalValues{} }
func (th *timelineHorizontal) NewOverrides() any    { return &TimelineHorizontalOverrides{} }
func (th *timelineHorizontal) NewCellOverride() any { return &TimelineHorizontalCellOverride{} }

// Measured against the written size (no run stored below its role floor) on
// every shipped template at default text sizes, every shape keeping the
// uniform 0.5 cm text margin (go-slide-creator-n1muf). Label wrapping consumes
// room in the same text shape as the body.
type timelineBudgetBand struct {
	maxLabel, body int
}

var timelineBodyBudgetBands = map[string]map[int][]timelineBudgetBand{
	"dots": {
		3: {{35, 200}, {60, 161}},
		4: {{20, 150}, {40, 118}, {60, 77}},
		5: {{15, 101}, {30, 81}, {45, 60}, {60, 40}},
		6: {{15, 75}, {20, 61}, {30, 45}, {60, 40}},
		7: {{10, 52}, {20, 42}, {40, 40}, {50, 15}, {60, 0}},
	},
}

// Gantt bars hold a shorter label, and the start / end dates share one row.
const (
	timelineGanttLabelBudget     = 36
	timelineGanttDateRangeBudget = 32
)

func timelineBodyBudget(style string, stops, labelChars int) int {
	bands := timelineBodyBudgetBands[style][stops]
	for _, band := range bands {
		if labelChars <= band.maxLabel {
			return band.body
		}
	}
	if len(bands) > 0 {
		return bands[len(bands)-1].body
	}
	return 200
}

func (th *timelineHorizontal) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*TimelineHorizontalValues)
	if !ok || v == nil {
		return nil
	}
	style := "dots"
	var ovr *TimelineHorizontalOverrides
	if o, ok := overrides.(*TimelineHorizontalOverrides); ok && o != nil {
		ovr = o
		if o.Style != "" {
			style = o.Style
		}
	}
	dateSize := timelineChevronDateSize(ovr)
	contentW, _ := contentAreaPt(ctx)
	dateTextW := math.Max(equalColumnWidthPt(contentW, len(*v), 0)-2*defaultShapeInsetLRPt, 1)
	var warnings []string
	for i, stop := range *v {
		if style == "gantt" {
			if strings.TrimSpace(stop.Body) != "" {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].body is not rendered in gantt style — move the detail into the label, choose dots or chevron style, or remove the body", ErrCodeContentDropped, i))
			}
			if n := runeLen(stop.Label); n > timelineGanttLabelBudget {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].label is %d characters; a gantt bar holds about %d readable label characters — shorten the label", ErrCodeBodyTooLong, i, n, timelineGanttLabelBudget))
			}
			if n := runeLen(stop.Date) + runeLen(stop.EndDate); stop.EndDate != "" && n > timelineGanttDateRangeBudget {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].date/end_date use %d characters; a gantt date range holds about %d readable characters — shorten the dates", ErrCodeBodyTooLong, i, n, timelineGanttDateRangeBudget))
			}
		} else if style == "chevron" {
			lines, capacity := timelineChevronBodyCapacity(ctx, *v, i, ovr)
			if lines > capacity {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].body needs %d lines but this %d-stop chevron holds %d below its label — shorten the body or label, use fewer stops, or choose dots style", ErrCodeBodyTooLong, i, lines, len(*v), capacity))
			}
		} else {
			budget := timelineBodyBudget(style, len(*v), runeLen(stop.Label))
			if n := runeLen(stop.Body); n > 0 && budget == 0 {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].body is %d characters; %d-stop %s style with a %d-character label holds no readable body — shorten the label, drop the body, or use fewer stops", ErrCodeBodyTooLong, i, n, len(*v), style, runeLen(stop.Label)))
			} else if n > budget {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].body is %d characters; %d-stop %s style with a %d-character label holds about %d readable body characters — shorten the body or label, or use fewer stops", ErrCodeBodyTooLong, i, n, len(*v), style, runeLen(stop.Label), budget))
			}
		}
		if style == "chevron" {
			if lines := measuredLines(stop.Date, ctx.Theme.BodyFont, false, dateSize, dateTextW); lines > 1 {
				warnings = append(warnings, fmt.Sprintf("%s: timeline-horizontal values[%d].date needs %d lines at %gpt but this %d-stop chevron date row holds 1 — shorten the date or use fewer stops", ErrCodeBodyTooLong, i, lines, dateSize, len(*v)))
			}
		}
	}
	// A gantt bar is placed by its dates; one that cannot be read is not
	// drawn to scale, and says so (go-slide-creator-o34er).
	budgetWarned := len(warnings) > 0
	warnings = append(warnings, timelineGanttDateWarnings(style, *v)...)
	// The budgets assume a typical content area; with the template's own area
	// the rows are also measured against it (go-slide-creator-n1muf).
	if budgetWarned {
		return warnings
	}
	if w := th.measuredWarning(ctx, v, ovr, style); w != "" {
		warnings = append(warnings, w)
	}
	return warnings
}

// measuredWarning reports a timeline whose text needs more height than the
// template's content area holds even with every row at its written fit and
// the type at the readable minimum. Without LayoutBounds it reports nothing.
func (th *timelineHorizontal) measuredWarning(ctx ExpandContext, v *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, style string) string {
	if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return ""
	}
	if ovr == nil {
		ovr = &TimelineHorizontalOverrides{}
	}
	switch style {
	case "gantt":
		if _, fit := th.ganttFit(ctx, *v, ovr, nil); !fit.fits {
			return fmt.Sprintf("%s: timeline-horizontal gantt rows need %.0fpt for their wrapped labels but the content area holds about %.0fpt — shorten the labels to one line or use fewer stops", ErrCodeBodyTooLong, fit.totalPt, fit.availPt)
		}
	case "chevron":
		grid, err := th.expandChevron(ctx, v, ovr, nil)
		if err != nil || len(grid.Rows) == 0 {
			return ""
		}
		if fit := timelineChevronRowFit(ctx, grid.Rows[0].Cells, timelineChevronDateRowPt(timelineChevronDateSize(ovr))); fit.needPt > fit.availPt+1 {
			return fmt.Sprintf("%s: timeline-horizontal chevrons need %.0fpt for their labels and bodies but the content area holds about %.0fpt — shorten the longest labels or bodies, use fewer stops, or choose dots style", ErrCodeBodyTooLong, fit.needPt, fit.availPt)
		}
	default:
		if fit := measureTimelineDots(ctx, *v, ovr, nil, "accent1"); !fit.fits() {
			return fmt.Sprintf("%s: timeline-horizontal stops need %.0fpt below the axis at the readable minimum but the content area leaves about %.0fpt — shorten the longest labels or bodies, or use fewer stops", ErrCodeBodyTooLong, fit.stopNeedPt, fit.stopAvailPt)
		}
	}
	return ""
}

func timelineChevronDateSize(ovr *TimelineHorizontalOverrides) float64 {
	if ovr == nil {
		return shapegrid.MinTextSizePt
	}
	return shapegrid.EffectiveTextSizePt(ResolveSize(ovr.DateSize, sizeDenseCaptionPt))
}

// timelineChevronBodyCapacity measures the actual line budget beneath one
// stop's label. The homePlate preset's pointed right quarter is not usable
// text width; measuring the full rectangular cell undercounts wrapping.
func timelineChevronBodyCapacity(ctx ExpandContext, stops TimelineHorizontalValues, i int, ovr *TimelineHorizontalOverrides) (lines, capacity int) {
	stop := stops[i]
	if stop.Body == "" {
		return 0, 0
	}
	labelSize, bodySize := timelineChevronTextSizes(ovr)
	contentW, contentH := contentAreaPt(ctx)
	textW := math.Max(equalColumnWidthPt(contentW, len(stops), 0)*0.75-2*defaultShapeInsetLRPt, 1)
	font := ctx.Theme.BodyFont
	labelLines := measuredLines(stop.Label, font, true, labelSize, textW)
	lines = measuredLines(stop.Body, font, false, bodySize, textW)
	rowH := math.Round(contentH * timelineChevronMaxHeightFrac)
	availableH := rowH - 2*defaultShapeInsetTBPt - 4 - float64(labelLines)*labelSize*contentLineHeight
	if availableH <= 0 {
		return lines, 0
	}
	capacity = int(math.Floor(availableH / (bodySize * contentLineHeight)))
	return lines, capacity
}

// timelineChevronTextSizes matches the sizes the shape-grid renderer emits.
// A small label cannot pull its body below the renderer's readable 12pt floor;
// an explicit body_size wins over the derived default.
func timelineChevronTextSizes(ovr *TimelineHorizontalOverrides) (labelSize, bodySize float64) {
	labelSize = scaleBodyPt
	if ovr != nil {
		labelSize = ResolveSize(ovr.LabelSize, labelSize)
		bodySize = ResolveSize(ovr.BodySize, labelSize-2)
	} else {
		bodySize = labelSize - 2
	}
	return shapegrid.EffectiveTextSizePt(labelSize), shapegrid.EffectiveTextSizePt(bodySize)
}

func (th *timelineHorizontal) Schema() *Schema {
	stopSchema := ObjectSchema(
		map[string]*Schema{
			"label":    StringSchema(60).WithDescription("Stop label (e.g. \"Q1 2025\", \"Launch\"); about 36 readable characters in gantt style"),
			"date":     StringSchema(30).WithDescription("Optional date or time annotation. Chevron dates have a one-line row at the effective font size (12pt minimum); BODY_TOO_LONG reports wrapping for the chosen template width. Dots and gantt retain the 30-character schema limit."),
			"end_date": StringSchema(30).WithDescription("End date for gantt style: the bar runs from the start of date to the end of end_date on a shared time axis (a stop without end_date is a diamond marker at its date). Write both as 2026-03-15, 2026-03, Mar 2026, Q1 2026, H1 2026 or 2026 — every stop with a year, or every stop without; a value that is not a date gets no bar and reports TIMELINE_DATE_UNPARSEABLE. date and end_date together hold about 32 readable characters"),
			"body":     StringSchema(200).WithDescription("Optional body for dots and chevron stops; gantt does not render body and emits CONTENT_DROPPED if set. Dots readable chars for short/long labels by stop count: 3: 200/161, 4: 150/77, 5: 101/40, 6: 75/40, 7: 52/0 (a 7-stop label over 50 characters leaves no body). Chevron body capacity is measured from its actual width, height, label wrapping, and font sizes; BODY_TOO_LONG reports the line limit for the chosen layout. Shorten descriptions or use fewer stops when warned."),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(stopSchema, 3, 7).WithDescription("3–7 timeline stops"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"label_size":      NumberSchema(6, 120).WithDescription("Font size for stop labels in points"),
					"date_size":       NumberSchema(6, 120).WithDescription("Font size for dates in points; chevron style uses the shape-grid 12pt rendering floor for row height and wrap warnings"),
					"body_size":       NumberSchema(6, 120).WithDescription("Font size for body text in points; chevron style honors this override and applies the shape-grid 12pt readable floor"),
					"style":           EnumSchema("dots", "chevron", "gantt").WithDescription("Visual style: dots (default: horizontal axis with accent dots, dates above, label/body below), chevron (connected arrow shapes with gradient), gantt (one row per stop on a labelled time axis: bars positioned and sized by date / end_date, diamond markers for stops without end_date)").WithDefault("dots"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Linear horizontal timeline with stops")
}

func (th *timelineHorizontal) Validate(values, overrides any, cellOverrides map[int]any) error {
	stops, ok := values.(*TimelineHorizontalValues)
	if !ok || stops == nil {
		return fmt.Errorf("timeline-horizontal: values must be []TimelineStop, got %T", values)
	}

	const name = "timeline-horizontal"
	var errs []error

	// Enforce 3–7 range with sibling-pattern hints
	if len(*stops) < 3 {
		errs = append(errs, newValidationError(name, "values", ErrCodeMinItems,
			fmt.Sprintf("timeline-horizontal: values must contain at least 3 stops, got %d (hint: use pattern icon-row for fewer items with icons)", len(*stops)),
			AddItemsFix("values", 3)))
	}
	if len(*stops) > 7 {
		errs = append(errs, newValidationError(name, "values", ErrCodeMaxItems,
			fmt.Sprintf("timeline-horizontal: values must contain at most 7 stops, got %d (hint: consider splitting across two slides)", len(*stops)),
			ReduceItemsFix("values", 7)))
	}

	// Determine style for context-sensitive validation
	style := "dots"
	if overrides != nil {
		if ovr, ok := overrides.(*TimelineHorizontalOverrides); ok && ovr.Style != "" {
			style = ovr.Style
		}
	}

	// Per-stop validation
	for i, stop := range *stops {
		labelPath := fmt.Sprintf("values[%d].label", i)
		if stop.Label == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if runeLen(stop.Label) > 60 {
			errs = append(errs, errMaxLength(name, labelPath, 60, runeLen(stop.Label)))
		}
		if runeLen(stop.Date) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].date", i), 30, runeLen(stop.Date)))
		}
		if runeLen(stop.EndDate) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].end_date", i), 30, runeLen(stop.EndDate)))
		}
		if stop.EndDate != "" && style != "gantt" {
			errs = append(errs, newValidationError(name, fmt.Sprintf("values[%d].end_date", i), ErrCodeUnknownEnum,
				"end_date is only valid with style \"gantt\"",
				RemoveFieldFix(fmt.Sprintf("values[%d].end_date", i))))
		}
		if runeLen(stop.Body) > 200 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].body", i), 200, runeLen(stop.Body)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(*stops), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (th *timelineHorizontal) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	stops, ok := values.(*TimelineHorizontalValues)
	if !ok {
		return nil, fmt.Errorf("timeline-horizontal: values must be *TimelineHorizontalValues, got %T", values)
	}
	ovr := &TimelineHorizontalOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*TimelineHorizontalOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("timeline-horizontal: overrides must be *TimelineHorizontalOverrides, got %T", overrides)
		}
	}

	style := ovr.Style
	if style == "" {
		style = "dots"
	}

	switch style {
	case "chevron":
		return th.expandChevron(ctx, stops, ovr, cellOverrides)
	case "gantt":
		return th.expandGantt(ctx, stops, ovr, cellOverrides)
	default:
		return th.expandDots(ctx, stops, ovr, cellOverrides)
	}
}

func (th *timelineHorizontal) expandDots(ctx ExpandContext, stops *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	fit := measureTimelineDots(ctx, *stops, ovr, cellOverrides, accent)
	n := len(*stops)

	// A real timeline (go-slide-creator-7km8): an optional date row above a
	// horizontal axis of accent dots joined by connector lines, with the stop
	// label + body below each dot. Every row is content-sized and the block
	// is centred vertically, instead of full-height filled pillars.
	dotCells := make([]*jsonschema.GridCellInput, n)
	for i := range dotCells {
		dotCells[i] = &jsonschema.GridCellInput{
			Fit: "contain",
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "ellipse",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Line:     json.RawMessage(`"none"`),
			},
		}
	}

	var rows []jsonschema.GridRowInput
	if fit.dateRowPt > 0 {
		rows = append(rows, jsonschema.GridRowInput{Cells: fit.dateCells, MinHeight: fit.dateRowPt, MaxHeight: fit.dateRowPt})
	}
	rows = append(rows, jsonschema.GridRowInput{
		Cells:     dotCells,
		MinHeight: timelineDotSizePt,
		MaxHeight: timelineDotSizePt,
		Connector: &jsonschema.ConnectorSpecInput{Style: "line", Color: accent, Width: timelineRulePt},
	})
	stopRow := jsonschema.GridRowInput{Cells: fit.stopCells, MaxHeight: fit.stopRowPt}
	if fit.stopNeedPt > fit.modelStopPt || fit.trim != timelineTrimNone {
		// The row is held at its written fit (go-slide-creator-n1muf). A
		// trimmed row that does not fit is scaled with the others rather than
		// given what they leave, which on a short area was nothing: the
		// labels were written into a zero-height box (go-slide-creator-wj8uz).
		stopRow.MinHeight = math.Min(fit.stopNeedPt, fit.stopRowPt)
	}
	rows = append(rows, stopRow)

	// A timeline that does not fit even trimmed is scaled as a whole, its row
	// gaps with its rows. Kept whole, the gaps took an area as short as they
	// are and the rows were laid out in nothing — no text written, so nothing
	// reported (go-slide-creator-wj8uz).
	rowGap := ctx.Gap(timelineDotsRowGapPt)
	if !fit.fits() {
		_, contentH := contentAreaPt(ctx)
		gaps := rowGap * float64(len(rows)-1)
		if total := fit.dateRowPt + timelineDotSizePt + fit.stopNeedPt + gaps; total > contentH {
			rowGap = math.Max(rowGap*math.Max(contentH, 0)/total, timelineDotsMinRowGapPt)
		}
	}

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, n)),
		ColGap:        ctx.Gap(timelineDotsColGapPt),
		RowGap:        rowGap,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// timelineDotsFit is the measured layout of a dots-style timeline.
type timelineDotsFit struct {
	labelSize   float64
	trim        timelineTrim
	dateCells   []*jsonschema.GridCellInput
	stopCells   []*jsonschema.GridCellInput
	dateRowPt   float64 // 0 when no stop has a date
	modelStopPt float64 // the theme-font estimate, capped at timelineStopMaxHeightFrac
	stopNeedPt  float64 // written fit of the tallest stop cell
	stopAvailPt float64 // height left below the axis
	stopRowPt   float64 // the stop row height handed to the grid
}

// fits reports whether the stops, written at their fit, fit below the date
// row and the axis.
func (f timelineDotsFit) fits() bool { return f.stopNeedPt <= f.stopAvailPt }

// timelineTrim is how much of the top and bottom text inset of the unfilled
// date and stop cells a stage of measureTimelineDots gives up.
type timelineTrim int

const (
	// timelineTrimNone keeps the inset all round, and 4pt more.
	timelineTrimNone timelineTrim = iota
	// timelineTrimOuter drops the inset above the date and below the stop
	// text: the rows hug their text and the space around the axis stays.
	timelineTrimOuter
	// timelineTrimAll drops every top and bottom inset: the date and the
	// label sit a row gap from their dot.
	timelineTrimAll
)

// measureTimelineDots builds the date and stop cells and sizes their rows.
// The stop row was pinned to the theme-font estimate, capped at 40% of the
// content height, and on the short template areas the documented budgets were
// written shrunk to 9–11.8pt. The row now grows to the written fit of its
// tallest cell at the real column width; when that leaves no room the default
// 14pt label steps to 12pt, and a stop that still does not fit is left to
// the measured BODY_TOO_LONG in PostExpandWarnings (go-slide-creator-n1muf).
//
// A short area (a regions cell, a compose segment) then gets the trimmed
// stages: the date row — 47pt of inset and padding around a 12pt date —
// shrinks to its line, and the stop row to its text. The stages run from
// roomiest to tightest and the first that fits wins, so more height never
// turns a fitting timeline into one that does not; the whole content area
// fits the first stage, so a full-slide timeline is unchanged
// (go-slide-creator-wj8uz).
func measureTimelineDots(ctx ExpandContext, stops TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any, accent string) timelineDotsFit {
	if ovr == nil {
		ovr = &TimelineHorizontalOverrides{}
	}
	label := ResolveSize(ovr.LabelSize, scaleSubheadPt)
	type stage struct {
		label float64
		trim  timelineTrim
	}
	stages := []stage{{label: label}}
	if ovr.LabelSize == 0 {
		label = shapegrid.MinTextSizePt
		stages = append(stages, stage{label: label})
	}
	stages = append(stages, stage{label, timelineTrimOuter}, stage{label, timelineTrimAll})
	// A sparse timeline — a few stops with one-line descriptions — promotes
	// its date and body to the label's size: three stops at 12pt under their
	// dots left most of the slide to the axis (go-slide-creator-yhzxt). The
	// promotion is taken only when the promoted block needs no more than
	// timelineSparseMaxFrac of the area, so a timeline that carries real copy,
	// or sits in a short cell, keeps the sizes its budgets are measured at.
	if ovr.LabelSize == 0 && ovr.BodySize == 0 && ovr.DateSize == 0 {
		promoted := *ovr
		promoted.BodySize, promoted.DateSize = scaleSubheadPt, scaleSubheadPt
		fit := measureTimelineDotsAt(ctx, stops, &promoted, cellOverrides, accent, scaleSubheadPt, timelineTrimNone)
		_, contentH := contentAreaPt(ctx)
		block := fit.dateRowPt + timelineDotSizePt + math.Max(fit.stopNeedPt, fit.modelStopPt) + 2*ctx.Gap(timelineDotsRowGapPt)
		if fit.fits() && block <= contentH*timelineSparseMaxFrac && timelineStopsHoldLeadStep(ctx, stops) {
			fit.stopRowPt = math.Max(fit.modelStopPt, fit.stopNeedPt)
			return fit
		}
	}
	var fit timelineDotsFit
	for _, st := range stages {
		if fit = measureTimelineDotsAt(ctx, stops, ovr, cellOverrides, accent, st.label, st.trim); fit.fits() {
			break
		}
	}
	fit.stopRowPt = fit.modelStopPt
	if fit.stopNeedPt > fit.modelStopPt {
		fit.stopRowPt = math.Max(fit.modelStopPt, math.Min(fit.stopNeedPt, fit.stopAvailPt))
	}
	return fit
}

// timelineStopsHoldLeadStep reports whether every date stays on one line and
// every label word stays whole in its stop column at the lead step — the size
// the placement policy may step a promoted timeline to. Narrow stop columns
// (many stops, a regions cell) keep the pattern's own sizes.
func timelineStopsHoldLeadStep(ctx ExpandContext, stops TimelineHorizontalValues) bool {
	contentW, _ := contentAreaPt(ctx)
	textW := equalColumnWidthPt(contentW, len(stops), ctx.Gap(timelineDotsColGapPt)) - 2*defaultShapeInsetLRPt
	font := ctx.Theme.BodyFont
	for _, stop := range stops {
		if stop.Date != "" && measuredLines(stop.Date, font, true, scaleLeadPt, textW*0.9) > 1 {
			return false
		}
		for _, word := range strings.Fields(stop.Label) {
			if measuredLines(word, font, true, scaleLeadPt, textW*0.9) > 1 {
				return false
			}
		}
		// A description short enough to be a label stays on one line: the
		// step refuses to wrap one (shapegrid.ComposeLabelMaxWords), and the
		// promoted timeline would then be left unstepped.
		if len(strings.Fields(stop.Body)) <= shapegrid.ComposeLabelMaxWords && measuredLines(stop.Body, font, false, scaleSubheadPt, textW) == 1 &&
			measuredLines(stop.Body, font, false, scaleLeadPt, textW*0.9) > 1 {
			return false
		}
	}
	return true
}

// measureTimelineDotsAt measures one stage of measureTimelineDots.
func measureTimelineDotsAt(ctx ExpandContext, stops TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any, accent string, labelSize float64, trim timelineTrim) timelineDotsFit {
	dateSize := ResolveSize(ovr.DateSize, scaleBodyPt)
	bodySize := ResolveSize(ovr.BodySize, scaleBodyPt)
	n := len(stops)
	font := ctx.Theme.BodyFont
	contentW, contentH := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, n, ctx.Gap(timelineDotsColGapPt))
	textW := colW - 2*defaultShapeInsetLRPt
	pad := 2*defaultShapeInsetTBPt + 4

	fit := timelineDotsFit{labelSize: labelSize, trim: trim, dateCells: make([]*jsonschema.GridCellInput, n)}
	hasDates := false
	var dateH, dateNeed float64
	for i, stop := range stops {
		if stop.Date != "" {
			hasDates = true
		}
		text := buildTimelineDotsText([]timelineDotsPara{{stop.Date, dateSize, true, accent}}, "b", trim >= timelineTrimOuter, trim == timelineTrimAll)
		fit.dateCells[i] = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     text,
		}}
		dateH = math.Max(dateH, textBlockHeightPt(font, textW, textParagraph{text: stop.Date, size: dateSize, bold: true}))
		if stop.Date != "" {
			dateNeed = math.Max(dateNeed, writtenFitHeightPt(ctx.themeFonts(), text, colW, 0))
		}
	}
	fit.stopAvailPt = contentH - timelineDotSizePt - ctx.Gap(timelineDotsRowGapPt)
	if hasDates {
		fit.dateRowPt = dateNeed
		if trim == timelineTrimNone {
			fit.dateRowPt = math.Max(math.Round(dateH+pad), dateNeed)
		}
		fit.stopAvailPt -= fit.dateRowPt + ctx.Gap(timelineDotsRowGapPt)
	}
	fit.stopAvailPt = math.Floor(fit.stopAvailPt)

	fit.stopCells = make([]*jsonschema.GridCellInput, n)
	var labelH float64
	for i, stop := range stops {
		paras := []timelineDotsPara{{stop.Label, labelSize, true, "dk1"}}
		if stop.Body != "" {
			paras = append(paras, timelineDotsPara{stop.Body, bodySize, false, "dk1"})
		}
		cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     buildTimelineDotsText(paras, "t", trim == timelineTrimAll, trim >= timelineTrimOuter),
		}}
		if co, ok := cellOverrides[i]; ok {
			if cellOvr, coOk := co.(*TimelineHorizontalCellOverride); coOk {
				applyCellTextOverride(cell, cellOvr)
				if cellOvr.AccentBar {
					cell.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: 4}
				}
			}
		}
		fit.stopCells[i] = cell
		labelH = math.Max(labelH, textBlockHeightPt(font, textW,
			textParagraph{text: stop.Label, size: labelSize, bold: true},
			textParagraph{text: stop.Body, size: bodySize}))
		fit.stopNeedPt = math.Max(fit.stopNeedPt, writtenFitHeightPt(ctx.themeFonts(), cell.Shape.Text, colW, 0))
	}
	if trim != timelineTrimNone {
		// A trimmed stop row is its written fit: no estimate, no padding.
		fit.modelStopPt = fit.stopNeedPt
	} else {
		stopH := math.Min(labelH+pad, contentH*timelineStopMaxHeightFrac)
		fit.modelStopPt = math.Round(math.Max(stopH, labelSize*contentLineHeight+pad))
	}
	return fit
}

const (
	// timelineRulePt is the thickness of a timeline / roadmap spine: a 3pt
	// rule, not a band (go-slide-creator-7z5we). phase-roadmap's timeline row
	// and the dots-style axis connectors both use it.
	timelineRulePt = 3.0
	// timelineDotSizePt is the axis-row height and therefore the dot diameter.
	timelineDotSizePt = 18.0
	// timelineDotsColGapPt separates stop columns in dots style.
	timelineDotsColGapPt = 16.0
	// timelineDotsRowGapPt separates the date, axis and stop rows.
	timelineDotsRowGapPt = 6.0
	// timelineDotsMinRowGapPt is the least a scaled row gap gets: a grid's
	// row gap of 0 means the shape-grid default.
	timelineDotsMinRowGapPt = 0.1
	// timelineStopMaxHeightFrac caps the label/body zone under each dot.
	timelineStopMaxHeightFrac = 0.4
	// timelineSparseMaxFrac is the share of the content height a dots
	// timeline may need at its promoted sizes and still count as sparse.
	timelineSparseMaxFrac = 0.6
	// timelineChevronMaxHeightFrac caps the chevron row in chevron style.
	timelineChevronMaxHeightFrac = 0.25
)

// timelineDotsPara is one paragraph of dots-style stop text.
type timelineDotsPara struct {
	content string
	size    float64
	bold    bool
	color   string
}

// buildTimelineDotsText renders centred paragraphs anchored at vAlign,
// without the top and / or bottom text inset when asked.
func buildTimelineDotsText(paras []timelineDotsPara, vAlign string, noTopInset, noBottomInset bool) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	out := make([]paragraph, 0, len(paras))
	for _, p := range paras {
		content := p.content
		if content == "" {
			content = " "
		}
		out = append(out, paragraph{Content: content, Size: p.size, Bold: p.bold, Color: p.color, Align: "ctr"})
	}
	obj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
		InsetTop      *float64    `json:"inset_top,omitempty"`
		InsetBottom   *float64    `json:"inset_bottom,omitempty"`
	}{Paragraphs: out, Align: "ctr", VerticalAlign: vAlign}
	zero := 0.0
	if noTopInset {
		obj.InsetTop = &zero
	}
	if noBottomInset {
		obj.InsetBottom = &zero
	}
	data, _ := json.Marshal(obj)
	return data
}

// expandChevron renders connected homePlate shapes with a gradient tint across the chain.
// Label inside chevron, date below in a second row.
func (th *timelineHorizontal) expandChevron(ctx ExpandContext, stops *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize, bodySize := timelineChevronTextSizes(ovr)
	dateSize := timelineChevronDateSize(ovr)

	n := len(*stops)

	// Chevron row: homePlate shapes with gradient tint across the chain
	chevronCells := make([]*jsonschema.GridCellInput, n)
	for i, stop := range *stops {
		// Compute tint for gradient: first stop is darkest (shade), last is lightest (tint)
		tone := chevronGradientTone(accent, i, n)

		// Label (and optionally body) inside the chevron, in whichever text
		// colour reads on this link's own tint.
		textContent := buildChevronTextContent(stop, labelSize, bodySize, timelineGradientTextColor(ctx, tone))

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "homePlate",
			Fill:     tone.fillJSON(),
			Text:     textContent,
		}

		gc := &jsonschema.GridCellInput{
			Shape: shape,
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*TimelineHorizontalCellOverride)
			if coOk {
				applyCellTextOverride(gc, cellOvr)
			}
			if coOk && cellOvr.AccentBar {
				gc.AccentBar = &jsonschema.AccentBarInput{
					Position: "top",
					Color:    accent,
					Width:    4,
				}
			}
		}

		chevronCells[i] = gc
	}

	// Date row: text labels below each chevron
	dateCells := make([]*jsonschema.GridCellInput, n)
	for i, stop := range *stops {
		dateText := stop.Date
		if dateText == "" {
			dateText = " "
		}
		textContent := json.RawMessage(fmt.Sprintf(
			`{"paragraphs":[{"content":%q,"size":%g,"align":"ctr"}],"align":"ctr","vertical_align":"top"}`,
			dateText, dateSize,
		))

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     textContent,
		}
		dateCells[i] = &jsonschema.GridCellInput{Shape: shape}
	}

	// Content-sized rows (go-slide-creator-7km8): the chevron row is capped
	// at timelineChevronMaxHeightFrac of the content height and the date row
	// hugs its text; the grid centres the block vertically. A chevron whose
	// text needs more than the cap grows to its written fit, up to the height
	// the date row leaves (go-slide-creator-n1muf).
	dateRowH := timelineChevronDateRowPt(dateSize)
	fit := timelineChevronRowFit(ctx, chevronCells, dateRowH)
	chevronRow := jsonschema.GridRowInput{Cells: chevronCells, MaxHeight: fit.rowPt}
	if fit.needPt > fit.capPt {
		chevronRow.MinHeight = fit.rowPt
	}
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:     0,
		Rows: []jsonschema.GridRowInput{
			chevronRow,
			{Cells: dateCells, MinHeight: dateRowH, MaxHeight: dateRowH},
		},
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

// timelineChevronResolvedGapPt is the gap a chevron grid's Gap 0 resolves to
// (the shape-grid default).
const timelineChevronResolvedGapPt = 8.0

// timelineChevronDateRowPt is the one-line date row below the chevrons.
func timelineChevronDateRowPt(dateSize float64) float64 {
	return math.Round(dateSize*contentLineHeight + 2*defaultShapeInsetTBPt + 4)
}

// timelineRowFit sizes a timeline row against the written fit of its cells.
type timelineRowFit struct {
	capPt   float64 // the pattern's own height cap for the row
	needPt  float64 // written fit of the tallest cell
	availPt float64 // the most the row can take
	rowPt   float64 // the height handed to the grid
}

// timelineChevronRowFit measures the chevron row: at least its cap, grown to
// the written fit of its tallest chevron (at the full cell width, the box the
// writer's autofit measures) but never past the height the date row leaves.
func timelineChevronRowFit(ctx ExpandContext, cells []*jsonschema.GridCellInput, dateRowPt float64) timelineRowFit {
	contentW, contentH := contentAreaPt(ctx)
	// The grid's Gap 0 resolves to the shape-grid default gap between the
	// links and above the date row.
	colW := equalColumnWidthPt(contentW, len(cells), ctx.Gap(timelineChevronResolvedGapPt))
	fit := timelineRowFit{capPt: math.Round(contentH * timelineChevronMaxHeightFrac), availPt: math.Floor(contentH - dateRowPt - ctx.Gap(timelineChevronResolvedGapPt))}
	for _, c := range cells {
		if c != nil && c.Shape != nil {
			fit.needPt = math.Max(fit.needPt, writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, colW, 0))
		}
	}
	fit.rowPt = fit.capPt
	if fit.needPt > fit.capPt {
		fit.rowPt = math.Max(fit.capPt, math.Min(fit.needPt, fit.availPt))
	}
	return fit
}

// chevronGradientTone produces the fill tone for one link of a gradient chain.
// First item is darkest (shade 70000), last is lightest (tint 40000), middle
// interpolates. It returns the tone rather than the JSON so the caller can ask
// which text colour reads on it: the text used to be hardcoded lt1, which on
// the lightest bar measured 1.5:1 (go-slide-creator-5qotm).
func chevronGradientTone(accent string, index, total int) fillTone {
	tone := fillTone{Color: accent}
	if total <= 1 {
		return tone
	}
	// Interpolate from shade=70000 (dark) at index 0 to tint=40000 (light) at
	// index n-1. Midpoint (ratio=0.5) is no modifier (plain accent).
	ratio := float64(index) / float64(total-1)
	switch {
	case ratio < 0.5:
		if shadeVal := int(70000 * (1.0 - 2.0*ratio)); shadeVal > 0 {
			tone.Shade = shadeVal
		}
	case ratio > 0.5:
		if tintVal := int(40000 * (2.0*ratio - 1.0)); tintVal > 0 {
			tone.Tint = tintVal
		}
	}
	return tone
}

// timelineGradientTextColor uses measured theme contrast when available. An
// expand_pattern call without a template has no theme to measure, but the
// gradient modifier still tells us tinted links are light surfaces. Choosing
// dark ink there prevents a portable expansion from baking in white-on-pale
// text before the eventual rendering template is known.
func timelineGradientTextColor(ctx ExpandContext, tone fillTone) string {
	fallback := "lt1"
	if tone.Tint > 0 {
		fallback = "dk2"
	}
	return readableTextOn(ctx, tone, fallback)
}

// buildChevronTextContent creates text for inside a chevron shape (label +
// optional body, no date), in the given text colour.
func buildChevronTextContent(stop TimelineStop, labelSize, bodySize float64, textColor string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	paras := []paragraph{
		{Content: stop.Label, Size: labelSize, Bold: true, Color: textColor, Align: "ctr"},
	}
	if stop.Body != "" {
		paras = append(paras, paragraph{Content: stop.Body, Size: bodySize, Color: textColor, Align: "ctr"})
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
