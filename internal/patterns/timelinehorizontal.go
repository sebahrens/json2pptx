package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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
func (th *timelineHorizontal) UseWhen() string     { return "Linear timeline with stops" }
func (th *timelineHorizontal) Version() int        { return 1 }
func (th *timelineHorizontal) CellsHint() string   { return "3-7" }

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
	Connector      string  `json:"connector,omitempty"` // "arrow" or "line" (default: "arrow")
	Style          string  `json:"style,omitempty"`     // "dots" (default), "chevron", or "gantt"
}

// TimelineHorizontalCellOverride is an alias for the shared CellOverride struct.
type TimelineHorizontalCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (th *timelineHorizontal) NewValues() any      { return &TimelineHorizontalValues{} }
func (th *timelineHorizontal) NewOverrides() any   { return &TimelineHorizontalOverrides{} }
func (th *timelineHorizontal) NewCellOverride() any { return &TimelineHorizontalCellOverride{} }


func (th *timelineHorizontal) Schema() *Schema {
	stopSchema := ObjectSchema(
		map[string]*Schema{
			"label":    StringSchema(60).WithDescription("Stop label (e.g. \"Q1 2025\", \"Launch\")"),
			"date":     StringSchema(30).WithDescription("Optional date or time annotation"),
			"end_date": StringSchema(30).WithDescription("End date for gantt style (creates a range bar from date to end_date)"),
			"body":     StringSchema(200).WithDescription("Optional body text for the stop"),
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
					"date_size":       NumberSchema(6, 120).WithDescription("Font size for dates in points"),
					"body_size":       NumberSchema(6, 120).WithDescription("Font size for body text in points"),
					"connector":       EnumSchema("arrow", "line").WithDescription("Connector style between stops (default: arrow)").WithDefault("arrow"),
					"style":           EnumSchema("dots", "chevron", "gantt").WithDescription("Visual style: dots (default rounded rectangles), chevron (connected arrow shapes with gradient), gantt (horizontal range bars)").WithDefault("dots"),
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
			"provide at least 3 stops in values"))
	}
	if len(*stops) > 7 {
		errs = append(errs, newValidationError(name, "values", ErrCodeMaxItems,
			fmt.Sprintf("timeline-horizontal: values must contain at most 7 stops, got %d (hint: consider splitting across two slides)", len(*stops)),
			"reduce values to at most 7 stops"))
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
		} else if len(stop.Label) > 60 {
			errs = append(errs, errMaxLength(name, labelPath, 60, len(stop.Label)))
		}
		if len(stop.Date) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].date", i), 30, len(stop.Date)))
		}
		if len(stop.EndDate) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].end_date", i), 30, len(stop.EndDate)))
		}
		if stop.EndDate != "" && style != "gantt" {
			errs = append(errs, newValidationError(name, fmt.Sprintf("values[%d].end_date", i), ErrCodeUnknownEnum,
				"end_date is only valid with style \"gantt\"",
				"remove end_date or set overrides.style to \"gantt\""))
		}
		if len(stop.Body) > 200 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("values[%d].body", i), 200, len(stop.Body)))
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
	labelSize := ResolveSize(ovr.LabelSize, 14.0)
	dateSize := ResolveSize(ovr.DateSize, 10.0)
	bodySize := ResolveSize(ovr.BodySize, 10.0)
	connectorStyle := "arrow"
	if ovr.Connector != "" {
		connectorStyle = ovr.Connector
	}

	n := len(*stops)
	gridCells := make([]*jsonschema.GridCellInput, n)
	for i, stop := range *stops {
		textContent := buildTimelineStopTextContent(stop, labelSize, dateSize, bodySize)

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     textContent,
		}

		gc := &jsonschema.GridCellInput{
			Shape: shape,
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*TimelineHorizontalCellOverride)
			if !coOk {
				continue
			}
			if cellOvr.AccentBar {
				gc.AccentBar = &jsonschema.AccentBarInput{
					Position: "top",
					Color:    accent,
					Width:    4,
				}
			}
		}

		gridCells[i] = gc
	}

	connector := &jsonschema.ConnectorSpecInput{
		Style: connectorStyle,
		Color: accent,
		Width: 2.0,
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:     16,
		Rows: []jsonschema.GridRowInput{
			{
				Cells:     gridCells,
				Connector: connector,
			},
		},
	}

	return grid, nil
}

// expandChevron renders connected homePlate shapes with a gradient tint across the chain.
// Label inside chevron, date below in a second row.
func (th *timelineHorizontal) expandChevron(ctx ExpandContext, stops *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.LabelSize, 12.0)
	dateSize := ResolveSize(ovr.DateSize, 9.0)

	n := len(*stops)

	// Chevron row: homePlate shapes with gradient tint across the chain
	chevronCells := make([]*jsonschema.GridCellInput, n)
	for i, stop := range *stops {
		// Compute tint for gradient: first stop is darkest (shade), last is lightest (tint)
		fill := buildChevronGradientFill(accent, i, n)

		// Label (and optionally body) inside the chevron
		textContent := buildChevronTextContent(stop, labelSize)

		shape := &jsonschema.ShapeSpecInput{
			Geometry: "homePlate",
			Fill:     fill,
			Text:     textContent,
		}

		gc := &jsonschema.GridCellInput{
			Shape: shape,
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*TimelineHorizontalCellOverride)
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

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:     0,
		Rows: []jsonschema.GridRowInput{
			{Cells: chevronCells, Height: 60},
			{Cells: dateCells, Height: 24},
		},
	}

	return grid, nil
}

// expandGantt renders horizontal bars representing date ranges.
// Label left-aligned in bar, date range shown as bar width hint.
func (th *timelineHorizontal) expandGantt(ctx ExpandContext, stops *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.LabelSize, 11.0)
	dateSize := ResolveSize(ovr.DateSize, 9.0)

	n := len(*stops)

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
		barText := json.RawMessage(fmt.Sprintf(
			`{"paragraphs":[{"content":%q,"size":%g,"color":"lt1","align":"left"}],"align":"left","vertical_align":"ctr"}`,
			dateLabel, dateSize,
		))

		// Tint gradient per row
		fill := buildChevronGradientFill(accent, i, n)

		barShape := &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     fill,
			Text:     barText,
		}

		barCell := &jsonschema.GridCellInput{Shape: barShape}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*TimelineHorizontalCellOverride)
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
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[30, 70]`),
		Gap:     8,
		Rows:    rows,
	}

	return grid, nil
}

// buildChevronGradientFill produces a fill with gradient tint/shade across a chain.
// First item is darkest (shade 70000), last is lightest (tint 40000), middle interpolates.
func buildChevronGradientFill(accent string, index, total int) json.RawMessage {
	if total <= 1 {
		return json.RawMessage(fmt.Sprintf(`"%s"`, accent))
	}
	// Interpolate from shade=70000 (dark) at index 0 to tint=40000 (light) at index n-1.
	// Midpoint (ratio=0.5) is no modifier (plain accent).
	ratio := float64(index) / float64(total-1)
	if ratio < 0.5 {
		// Shade: 70000 at ratio=0, no shade at ratio=0.5
		shadeVal := int(70000 * (1.0 - 2.0*ratio))
		if shadeVal > 0 {
			return json.RawMessage(fmt.Sprintf(`{"color":%q,"shade":%d}`, accent, shadeVal))
		}
	} else if ratio > 0.5 {
		// Tint: no tint at ratio=0.5, 40000 at ratio=1.0
		tintVal := int(40000 * (2.0*ratio - 1.0))
		if tintVal > 0 {
			return json.RawMessage(fmt.Sprintf(`{"color":%q,"tint":%d}`, accent, tintVal))
		}
	}
	return json.RawMessage(fmt.Sprintf(`"%s"`, accent))
}

// buildChevronTextContent creates text for inside a chevron shape (label + optional body, no date).
func buildChevronTextContent(stop TimelineStop, labelSize float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	paras := []paragraph{
		{Content: stop.Label, Size: labelSize, Bold: true, Color: "lt1", Align: "ctr"},
	}
	if stop.Body != "" {
		paras = append(paras, paragraph{Content: stop.Body, Size: labelSize - 2, Color: "lt1", Align: "ctr"})
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

// buildTimelineStopTextContent creates a JSON text object with paragraphs for a timeline stop.
func buildTimelineStopTextContent(stop TimelineStop, labelSize, dateSize, bodySize float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	paras := []paragraph{
		{Content: stop.Label, Size: labelSize, Bold: true, Color: "lt1", Align: "ctr"},
	}
	if stop.Date != "" {
		paras = append(paras, paragraph{Content: stop.Date, Size: dateSize, Color: "lt1", Align: "ctr"})
	}
	if stop.Body != "" {
		paras = append(paras, paragraph{Content: stop.Body, Size: bodySize, Color: "lt1", Align: "ctr"})
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
