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
	Label string `json:"label"`
	Date  string `json:"date,omitempty"`
	Body  string `json:"body,omitempty"`
}

// TimelineHorizontalValues is the values type: 3–7 timeline stops.
type TimelineHorizontalValues = []TimelineStop

// TimelineHorizontalOverrides contains pattern-level overrides for timeline-horizontal.
type TimelineHorizontalOverrides struct {
	Accent    string  `json:"accent,omitempty"`
	LabelSize float64 `json:"label_size,omitempty"`
	DateSize  float64 `json:"date_size,omitempty"`
	BodySize  float64 `json:"body_size,omitempty"`
	Connector string  `json:"connector,omitempty"` // "arrow" or "line" (default: "arrow")
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
			"label": StringSchema(60).WithDescription("Stop label (e.g. \"Q1 2025\", \"Launch\")"),
			"date":  StringSchema(30).WithDescription("Optional date or time annotation"),
			"body":  StringSchema(200).WithDescription("Optional body text for the stop"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)


	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(stopSchema, 3, 7).WithDescription("3–7 timeline stops"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":     StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"label_size": NumberSchema(6, 120).WithDescription("Font size for stop labels in points"),
					"date_size":  NumberSchema(6, 120).WithDescription("Font size for dates in points"),
					"body_size":  NumberSchema(6, 120).WithDescription("Font size for body text in points"),
					"connector":  EnumSchema("arrow", "line").WithDescription("Connector style between stops (default: arrow)").WithDefault("arrow"),
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

	var errs []error

	// Enforce 3–7 range with sibling-pattern hints
	if len(*stops) < 3 {
		errs = append(errs, fmt.Errorf("timeline-horizontal: values must contain at least 3 stops, got %d (hint: use pattern icon-row for fewer items with icons)", len(*stops)))
	}
	if len(*stops) > 7 {
		errs = append(errs, fmt.Errorf("timeline-horizontal: values must contain at most 7 stops, got %d (hint: consider splitting across two slides)", len(*stops)))
	}

	// Per-stop validation
	for i, stop := range *stops {
		if stop.Label == "" {
			errs = append(errs, fmt.Errorf("timeline-horizontal: values[%d].label is required", i))
		} else if len(stop.Label) > 60 {
			errs = append(errs, fmt.Errorf("timeline-horizontal: values[%d].label exceeds maxLength 60 (%d chars)", i, len(stop.Label)))
		}
		if len(stop.Date) > 30 {
			errs = append(errs, fmt.Errorf("timeline-horizontal: values[%d].date exceeds maxLength 30 (%d chars)", i, len(stop.Date)))
		}
		if len(stop.Body) > 200 {
			errs = append(errs, fmt.Errorf("timeline-horizontal: values[%d].body exceeds maxLength 200 (%d chars)", i, len(stop.Body)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= len(*stops) {
			errs = append(errs, fmt.Errorf("timeline-horizontal: cell_overrides key %d out of range [0,%d]", idx, len(*stops)-1))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("timeline-horizontal: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("timeline-horizontal: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !cellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("timeline-horizontal: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
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

	accent := ResolveAccent(ovr.Accent)
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
