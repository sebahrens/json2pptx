package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
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
		{Label: "Phase 1", Date: "Q1 2025", Body: "Planning"},
		{Label: "Phase 2", Date: "Q2 2025", Body: "Development"},
		{Label: "Phase 3", Date: "Q3 2025", Body: "Launch"},
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
	EndDate string `json:"end_date,omitempty"` // Only used in gantt style
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
	// The budgets assume a typical content area; with the template's own area
	// the rows are also measured against it (go-slide-creator-n1muf).
	if len(warnings) > 0 {
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
		grid, err := th.expandGantt(ctx, v, ovr, nil)
		if err != nil {
			return ""
		}
		if fit := timelineGanttFitRows(ctx, grid.Rows); !fit.fits {
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
		if fit := measureTimelineDots(ctx, *v, ovr, nil, "accent1"); fit.stopNeedPt > fit.stopAvailPt+1 {
			return fmt.Sprintf("%s: timeline-horizontal stops need %.0fpt below the axis at the readable minimum but the content area leaves about %.0fpt — shorten the longest labels or bodies, or use fewer stops", ErrCodeBodyTooLong, fit.stopNeedPt, fit.stopAvailPt)
		}
	}
	return ""
}

func timelineChevronDateSize(ovr *TimelineHorizontalOverrides) float64 {
	if ovr == nil {
		return shapegrid.MinTextSizePt
	}
	return shapegrid.EffectiveTextSizePt(ResolveSize(ovr.DateSize, 9))
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
	labelSize = 12
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
			"end_date": StringSchema(30).WithDescription("End date for gantt style (creates a range bar from date to end_date); date and end_date together hold about 32 readable characters"),
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
					"style":           EnumSchema("dots", "chevron", "gantt").WithDescription("Visual style: dots (default: horizontal axis with accent dots, dates above, label/body below), chevron (connected arrow shapes with gradient), gantt (horizontal range bars)").WithDefault("dots"),
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
	if fit.stopNeedPt > fit.modelStopPt {
		// The row is held at its written fit (go-slide-creator-n1muf).
		stopRow.MinHeight = math.Min(fit.stopNeedPt, fit.stopRowPt)
	}
	rows = append(rows, stopRow)

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, n)),
		ColGap:        timelineDotsColGapPt,
		RowGap:        timelineDotsRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// timelineDotsFit is the measured layout of a dots-style timeline.
type timelineDotsFit struct {
	labelSize   float64
	dateCells   []*jsonschema.GridCellInput
	stopCells   []*jsonschema.GridCellInput
	dateRowPt   float64 // 0 when no stop has a date
	modelStopPt float64 // the theme-font estimate, capped at timelineStopMaxHeightFrac
	stopNeedPt  float64 // written fit of the tallest stop cell
	stopAvailPt float64 // height left below the axis
	stopRowPt   float64 // the stop row height handed to the grid
}

// measureTimelineDots builds the date and stop cells and sizes their rows.
// The stop row was pinned to the theme-font estimate, capped at 40% of the
// content height, and on the short template areas the documented budgets were
// written shrunk to 9–11.8pt. The row now grows to the written fit of its
// tallest cell at the real column width; when that leaves no room the default
// 14pt label steps to 12pt, and a stop that still does not fit is left to
// the measured BODY_TOO_LONG in PostExpandWarnings (go-slide-creator-n1muf).
func measureTimelineDots(ctx ExpandContext, stops TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any, accent string) timelineDotsFit {
	if ovr == nil {
		ovr = &TimelineHorizontalOverrides{}
	}
	dateSize := ResolveSize(ovr.DateSize, 12.0)
	bodySize := ResolveSize(ovr.BodySize, 12.0)
	n := len(stops)
	font := ctx.Theme.BodyFont
	contentW, contentH := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, n, timelineDotsColGapPt)
	textW := colW - 2*defaultShapeInsetLRPt
	pad := 2*defaultShapeInsetTBPt + 4

	fit := timelineDotsFit{dateCells: make([]*jsonschema.GridCellInput, n)}
	hasDates := false
	var dateH, dateNeed float64
	for i, stop := range stops {
		if stop.Date != "" {
			hasDates = true
		}
		text := buildTimelineDotsText([]timelineDotsPara{{stop.Date, dateSize, true, accent}}, "b")
		fit.dateCells[i] = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     text,
		}}
		dateH = math.Max(dateH, textBlockHeightPt(font, textW, textParagraph{text: stop.Date, size: dateSize, bold: true}))
		if stop.Date != "" {
			dateNeed = math.Max(dateNeed, writtenFitHeightPt(text, colW, 0))
		}
	}
	fit.stopAvailPt = contentH - timelineDotSizePt - timelineDotsRowGapPt
	if hasDates {
		fit.dateRowPt = math.Max(math.Round(dateH+pad), dateNeed)
		fit.stopAvailPt -= fit.dateRowPt + timelineDotsRowGapPt
	}
	fit.stopAvailPt = math.Floor(fit.stopAvailPt)

	build := func(labelSize float64) {
		fit.labelSize = labelSize
		fit.stopCells = make([]*jsonschema.GridCellInput, n)
		var labelH float64
		fit.stopNeedPt = 0
		for i, stop := range stops {
			paras := []timelineDotsPara{{stop.Label, labelSize, true, "dk1"}}
			if stop.Body != "" {
				paras = append(paras, timelineDotsPara{stop.Body, bodySize, false, "dk1"})
			}
			cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildTimelineDotsText(paras, "t"),
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
			fit.stopNeedPt = math.Max(fit.stopNeedPt, writtenFitHeightPt(cell.Shape.Text, colW, 0))
		}
		stopH := math.Min(labelH+pad, contentH*timelineStopMaxHeightFrac)
		fit.modelStopPt = math.Round(math.Max(stopH, labelSize*contentLineHeight+pad))
	}
	build(ResolveSize(ovr.LabelSize, 14.0))
	if fit.stopNeedPt > fit.stopAvailPt && ovr.LabelSize == 0 {
		build(shapegrid.MinTextSizePt)
	}
	fit.stopRowPt = fit.modelStopPt
	if fit.stopNeedPt > fit.modelStopPt {
		fit.stopRowPt = math.Max(fit.modelStopPt, math.Min(fit.stopNeedPt, fit.stopAvailPt))
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
	// timelineStopMaxHeightFrac caps the label/body zone under each dot.
	timelineStopMaxHeightFrac = 0.4
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

// buildTimelineDotsText renders centred paragraphs anchored at vAlign.
func buildTimelineDotsText(paras []timelineDotsPara, vAlign string) json.RawMessage {
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
	data, _ := json.Marshal(struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{Paragraphs: out, Align: "ctr", VerticalAlign: vAlign})
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
	colW := equalColumnWidthPt(contentW, len(cells), timelineChevronResolvedGapPt)
	fit := timelineRowFit{capPt: math.Round(contentH * timelineChevronMaxHeightFrac), availPt: math.Floor(contentH - dateRowPt - timelineChevronResolvedGapPt)}
	for _, c := range cells {
		if c != nil && c.Shape != nil {
			fit.needPt = math.Max(fit.needPt, writtenFitHeightPt(c.Shape.Text, colW, 0))
		}
	}
	fit.rowPt = fit.capPt
	if fit.needPt > fit.capPt {
		fit.rowPt = math.Max(fit.capPt, math.Min(fit.needPt, fit.availPt))
	}
	return fit
}

// expandGantt renders horizontal bars representing date ranges.
// Label left-aligned in bar, date range shown as bar width hint.
func (th *timelineHorizontal) expandGantt(ctx ExpandContext, stops *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.LabelSize, 11.0)
	dateSize := ResolveSize(ovr.DateSize, 9.0)

	n := len(*stops)
	// Bars are capped (go-slide-creator-7km8) so 3 stops do not become three
	// slide-height slabs; the grid centres the stack vertically.
	_, contentH := contentAreaPt(ctx)
	ganttBarMaxPt := math.Round(math.Max(contentH*0.12, labelSize*contentLineHeight*2+2*defaultShapeInsetTBPt))

	// Each stop becomes a row with: label cell (col 1) + bar cell (col 2)
	rows := make([]jsonschema.GridRowInput, n)
	for i, stop := range *stops {
		// Label cell
		labelText := json.RawMessage(fmt.Sprintf(
			`{"paragraphs":[{"content":%q,"size":%g,"bold":true,"align":"right"}],"align":"right","vertical_align":"ctr"}`,
			stop.Label, labelSize,
		))
		labelShape := &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     labelText,
		}

		// Date range text inside bar
		dateLabel := stop.Date
		if stop.EndDate != "" {
			dateLabel = stop.Date + " → " + stop.EndDate
		}
		// Tint gradient per row, and the text colour that reads on this row's
		// own tint rather than a hardcoded lt1.
		tone := chevronGradientTone(accent, i, n)
		barText := json.RawMessage(fmt.Sprintf(
			`{"paragraphs":[{"content":%q,"size":%g,"color":%q,"align":"left"}],"align":"left","vertical_align":"ctr"}`,
			dateLabel, dateSize, timelineGradientTextColor(ctx, tone),
		))

		barShape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     tone.fillJSON(),
			Text:     barText,
		}

		barCell := &jsonschema.GridCellInput{Shape: barShape}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*TimelineHorizontalCellOverride)
			if coOk {
				applyCellTextOverride(barCell, cellOvr)
			}
			if coOk && cellOvr.AccentBar {
				barCell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
		}

		rows[i] = jsonschema.GridRowInput{
			Cells: []*jsonschema.GridCellInput{
				{Shape: labelShape},
				barCell,
			},
			MaxHeight: ganttBarMaxPt,
		}
	}

	// A bar whose label wraps is held at its written fit instead of being
	// squeezed to the cap; one-line rows keep giving way, since the writer
	// clamps a short shape's margin so one line always fits. When the wrapped
	// rows do not fit, the label column widens before they are left to shrink
	// (go-slide-creator-n1muf).
	fit := timelineGanttFitRows(ctx, rows)
	if fit.fits {
		for i := range rows {
			if fit.needs[i] > 0 {
				rows[i].MinHeight = fit.needs[i]
				rows[i].MaxHeight = math.Max(rows[i].MaxHeight, fit.needs[i])
			}
		}
	}
	cols := fmt.Sprintf(`[%g, %g]`, fit.labelPct, 100-fit.labelPct)

	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(cols),
		Gap:           timelineGanttGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

const (
	// timelineGanttGapPt separates the label column from the bars, and the rows.
	timelineGanttGapPt = 8.0
	// timelineGanttLabelPct is the label column's default share of the grid,
	// and timelineGanttLabelMaxPct the widest it grows to hold wrapped labels.
	timelineGanttLabelPct    = 30.0
	timelineGanttLabelMaxPct = 45.0
)

// timelineGanttFit is the written-fit measurement of a gantt stack.
type timelineGanttFit struct {
	labelPct float64
	needs    []float64 // written fit of a row whose text wraps; 0 for one-line rows
	totalPt  float64   // the stack's minimum height: wrapped rows at their fit, one-line rows at one line
	availPt  float64
	fits     bool
}

// timelineGanttFitRows measures the stack at the default label column and,
// while the wrapped rows do not fit, at wider ones in 5-point steps. With no
// width that fits it reports the default column.
func timelineGanttFitRows(ctx ExpandContext, rows []jsonschema.GridRowInput) timelineGanttFit {
	first := timelineGanttRowFit(ctx, rows, timelineGanttLabelPct)
	for pct := timelineGanttLabelPct + 5; !first.fits && pct <= timelineGanttLabelMaxPct; pct += 5 {
		if fit := timelineGanttRowFit(ctx, rows, pct); fit.fits {
			return fit
		}
	}
	return first
}

// timelineGanttRowFit measures each gantt row's label and bar text at their
// real column widths for a label column labelPct of the grid.
func timelineGanttRowFit(ctx ExpandContext, rows []jsonschema.GridRowInput, labelPct float64) timelineGanttFit {
	contentW, contentH := contentAreaPt(ctx)
	// A substituted template face draws wider than its stand-in, and a label
	// that only just fits one line wraps in the renderer — where the clamped
	// margin of a one-line row leaves no room for the second line, so
	// LibreOffice shrinks it. Rows are measured at the atomic-token width.
	font := ctx.Theme.BodyFont
	widths := []float64{
		textfit.AtomicTokenWidthPt(font, (contentW-timelineGanttGapPt)*labelPct/100),
		textfit.AtomicTokenWidthPt(font, (contentW-timelineGanttGapPt)*(100-labelPct)/100),
	}
	fit := timelineGanttFit{labelPct: labelPct, needs: make([]float64, len(rows)), availPt: contentH - float64(max(len(rows)-1, 0))*timelineGanttGapPt}
	for i, row := range rows {
		oneLine := 0.0
		for j, c := range row.Cells {
			if c == nil || c.Shape == nil || j >= len(widths) {
				continue
			}
			tb, err := shapegrid.ResolveTextInput(c.Shape.Text)
			if err != nil || tb == nil {
				continue
			}
			line := largestRunPt(tb) * contentLineHeight
			oneLine = math.Max(oneLine, line)
			if need := writtenFitHeightPt(c.Shape.Text, widths[j], 0); need > math.Ceil(2*defaultShapeInsetTBPt+line) {
				fit.needs[i] = math.Max(fit.needs[i], need)
			}
		}
		fit.totalPt += math.Max(fit.needs[i], oneLine)
	}
	fit.fits = fit.totalPt <= fit.availPt+1
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
