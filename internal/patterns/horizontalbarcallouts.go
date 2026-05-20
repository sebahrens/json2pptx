package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// horizontal-bar-with-callouts pattern — ranked horizontal bars (3-8 items)
// with per-bar insight callout column.
// ---------------------------------------------------------------------------
//
// Layout:
//   Two-column outer split: left 60% = bar area, right 40% = callout area.
//   N rows (one per bar) carry both the bar (left) and its callout (right).
//
//   Each bar row's left cell hosts a sub-grid with three columns
//     [labelPct, fillPct, restPct]
//   so the bar fill width is strictly proportional to (value / max_value).
//   labelPct is fixed (~22% of the left column) and right-aligns the bar label.
//
//   Each bar row's right cell carries the callout text with a thin left
//   accent bar (2pt), creating a visual bond between bar and insight.

func init() {
	Default().Register(&horizontalBarCallouts{})
}

type horizontalBarCallouts struct{}

// Pattern budgets (kept near top of file for easy tuning).
const (
	hbcMinBars     = 3
	hbcMaxBars     = 8
	hbcLabelMax    = 40
	hbcCalloutMax  = 200
	hbcUnitMax     = 8
	hbcLabelColPct = 22.0 // % of the left column reserved for the bar label
	hbcLeftColPct  = 60.0 // outer split: left bar area
	hbcRightColPct = 40.0 // outer split: right callout column
)

func (h *horizontalBarCallouts) Name() string { return "horizontal-bar-with-callouts" }
func (h *horizontalBarCallouts) Description() string {
	return "Ranked horizontal bars (3-8 items) on the left with one accent-bar-anchored insight callout per bar on the right"
}
func (h *horizontalBarCallouts) UseWhen() string {
	return "Ranked horizontal comparison where each item (vendor scorecard, opportunity sizing, driver list) needs its own short insight bound to that bar; prefer chart-insights-split when insights are shared, kpi-Nup when items are big-number metrics, comparison-2col when the slide is purely textual"
}
func (h *horizontalBarCallouts) NotWhen() string {
	return "All callouts are interchangeable (use chart-insights-split with a bar chart), items are big-number KPIs without per-item narrative (use kpi-Nup), bars represent time-series rather than ranked items (use a chart), or item count is below 3 / above 8"
}
func (h *horizontalBarCallouts) Version() int      { return 1 }
func (h *horizontalBarCallouts) CellsHint() string { return "3-8 (rows)" }
func (h *horizontalBarCallouts) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "data-display",
		NarrativeRole: []string{"evidence", "compare"},
		PairsWith:     []string{"chart-insights-split", "kpi-3up", "pull-quote"},
		ComposesWith:  nil,
		RoleOnSlide:   nil,
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}

func (h *horizontalBarCallouts) SupportsCallout() bool        { return true }
func (h *horizontalBarCallouts) SupportsInlineMarkdown() bool { return true }

func (h *horizontalBarCallouts) ExemplarValues() any {
	return &HorizontalBarCalloutsValues{
		Unit: "%",
		Bars: []HorizontalBarCalloutsBar{
			{Label: "Vendor A", Value: 87, Callout: "Strongest on price; weakest on support SLA."},
			{Label: "Vendor B", Value: 72, Callout: "Preferred for enterprise; long implementation cycles."},
			{Label: "Vendor C", Value: 64, Callout: "Best APIs and ecosystem fit, mid-market focus."},
			{Label: "Vendor D", Value: 41, Callout: "Lagging on security certifications."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// HorizontalBarCalloutsBar is a single ranked bar: a short label, a numeric
// value, and a one-sentence insight callout shown to the right of the bar.
type HorizontalBarCalloutsBar struct {
	Label   string  `json:"label"`
	Value   float64 `json:"value"`
	Callout string  `json:"callout,omitempty"`
}

// HorizontalBarCalloutsValues holds the ordered bars plus optional scale and
// unit. MaxValue defaults to the maximum bar value when omitted. Unit is
// appended to value labels (e.g. "87%", "$4.2M").
type HorizontalBarCalloutsValues struct {
	Bars     []HorizontalBarCalloutsBar `json:"bars"`
	MaxValue float64                    `json:"max_value,omitempty"`
	Unit     string                     `json:"unit,omitempty"`
}

// HorizontalBarCalloutsOverrides is the standard text overrides plus pattern
// specifics. Accent governs both the bar fill and the callout accent bar.
type HorizontalBarCalloutsOverrides struct {
	TextOverrides
	ValueSize   float64 `json:"value_size,omitempty"`   // value label font size (default 12)
	CalloutSize float64 `json:"callout_size,omitempty"` // callout text size (default 10)
	LabelSize   float64 `json:"label_size,omitempty"`   // bar label font size (default 11)
}

// HorizontalBarCalloutsCellOverride is the standard per-cell override; indexed
// by bar.
type HorizontalBarCalloutsCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (h *horizontalBarCallouts) NewValues() any       { return &HorizontalBarCalloutsValues{} }
func (h *horizontalBarCallouts) NewOverrides() any    { return &HorizontalBarCalloutsOverrides{} }
func (h *horizontalBarCallouts) NewCellOverride() any { return &HorizontalBarCalloutsCellOverride{} }

func (h *horizontalBarCallouts) Schema() *Schema {
	barSchema := ObjectSchema(
		map[string]*Schema{
			"label":   StringSchema(hbcLabelMax).WithDescription("Short bar label (1-3 words)"),
			"value":   NumberSchema(0, 1e12).WithDescription("Numeric value rendered as a proportional bar"),
			"callout": StringSchema(hbcCalloutMax).WithDescription("One-sentence insight rendered next to the bar"),
		},
		[]string{"label", "value"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"bars":      ArraySchema(barSchema, hbcMinBars, hbcMaxBars).WithDescription("Ranked horizontal bars (3-8)"),
			"max_value": NumberSchema(0, 1e12).WithDescription("Scale ceiling; defaults to the maximum bar value"),
			"unit":      StringSchema(hbcUnitMax).WithDescription("Optional unit string appended to value labels (e.g. \"%\", \"M\")"),
		},
		[]string{"bars"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color governing bar fill and callout accent bar (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":     NumberSchema(6, 40).WithDescription("Bar label font size (default 11)"),
			"body_size":       NumberSchema(6, 40).WithDescription("Callout text font size (default 10)"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent rotation for callout accent bars"),
			"label_size":      NumberSchema(6, 40).WithDescription("Bar label font size (default 11) — overrides header_size for this pattern"),
			"value_size":      NumberSchema(6, 40).WithDescription("Value label font size (default 12)"),
			"callout_size":    NumberSchema(6, 40).WithDescription("Callout text font size (default 10) — overrides body_size for this pattern"),
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
	}).WithDescription("Ranked horizontal bars on the left with per-bar insight callouts on the right; bar widths are proportional to value/max_value")
}

func (h *horizontalBarCallouts) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*HorizontalBarCalloutsValues)
	if !ok || vals == nil {
		return fmt.Errorf("horizontal-bar-with-callouts: values must be *HorizontalBarCalloutsValues, got %T", values)
	}

	const name = "horizontal-bar-with-callouts"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*HorizontalBarCalloutsOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(vals.Bars) < hbcMinBars {
		errs = append(errs, errMinItems(name, "bars", hbcMinBars, len(vals.Bars), "(hint: use kpi-Nup for fewer than 3 ranked items)"))
	}
	if len(vals.Bars) > hbcMaxBars {
		errs = append(errs, errMaxItems(name, "bars", hbcMaxBars, len(vals.Bars), "(hint: split into two slides or switch to a chart)"))
	}

	for i, bar := range vals.Bars {
		labelPath := fmt.Sprintf("bars[%d].label", i)
		if strings.TrimSpace(bar.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if len(bar.Label) > hbcLabelMax {
			errs = append(errs, errMaxLength(name, labelPath, hbcLabelMax, len(bar.Label)))
		}
		if bar.Value < 0 {
			errs = append(errs, &ValidationError{
				Pattern: name,
				Path:    fmt.Sprintf("bars[%d].value", i),
				Code:    ErrCodeOutOfRange,
				Message: fmt.Sprintf("%s: bars[%d].value must be >= 0, got %g", name, i, bar.Value),
				Fix:     ProvideValueFix(fmt.Sprintf("bars[%d].value", i)),
			})
		}
		if len(bar.Callout) > hbcCalloutMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("bars[%d].callout", i), hbcCalloutMax, len(bar.Callout)))
		}
	}

	if vals.MaxValue < 0 {
		errs = append(errs, &ValidationError{
			Pattern: name,
			Path:    "max_value",
			Code:    ErrCodeOutOfRange,
			Message: fmt.Sprintf("%s: max_value must be >= 0, got %g", name, vals.MaxValue),
			Fix:     ProvideValueFix("max_value"),
		})
	}

	if len(vals.Unit) > hbcUnitMax {
		errs = append(errs, errMaxLength(name, "unit", hbcUnitMax, len(vals.Unit)))
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Bars), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (h *horizontalBarCallouts) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*HorizontalBarCalloutsValues)
	if !ok {
		return nil, fmt.Errorf("horizontal-bar-with-callouts: values must be *HorizontalBarCalloutsValues, got %T", values)
	}
	ovr := &HorizontalBarCalloutsOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*HorizontalBarCalloutsOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("horizontal-bar-with-callouts: overrides must be *HorizontalBarCalloutsOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	// label_size wins over header_size; callout_size wins over body_size.
	labelSize := ResolveSize(ovr.LabelSize, ResolveSize(ovr.HeaderSize, 11.0))
	calloutSize := ResolveSize(ovr.CalloutSize, ResolveSize(ovr.BodySize, 10.0))
	valueSize := ResolveSize(ovr.ValueSize, 12.0)

	// Resolve max_value: explicit > derived from bars > 1 (fallback to avoid div0).
	maxVal := vals.MaxValue
	if maxVal <= 0 {
		for _, b := range vals.Bars {
			if b.Value > maxVal {
				maxVal = b.Value
			}
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	n := len(vals.Bars)
	rows := make([]jsonschema.GridRowInput, n)

	for i, bar := range vals.Bars {
		// Per-bar accent (governs the callout accent bar; bar fill stays accent
		// for consistency unless cell_accent_mode is explicitly set).
		accent := ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)

		// Bar value clamped into [0, maxVal] so the sub-grid columns stay valid.
		clampedValue := bar.Value
		if clampedValue < 0 {
			clampedValue = 0
		}
		if clampedValue > maxVal {
			clampedValue = maxVal
		}
		fillFraction := clampedValue / maxVal
		// Reserve hbcLabelColPct for the label; split the remainder into
		// fillPct (proportional) and restPct (transparent placeholder).
		barAreaPct := 100.0 - hbcLabelColPct
		fillPct := barAreaPct * fillFraction
		restPct := barAreaPct - fillPct

		barCell := buildHorizontalBarRowCell(bar, accent, vals.Unit, labelSize, valueSize, fillPct, restPct)
		calloutCell := buildHorizontalBarCalloutCell(bar.Callout, calloutSize)

		// Cell-override accent bar always renders on the callout cell so the
		// visual link between bar and callout is preserved by default; users
		// who set accent_bar:false in cell_overrides suppress only that bar.
		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*HorizontalBarCalloutsCellOverride); ok2 && !cellOvr.AccentBar {
				calloutCell.AccentBar = nil
			}
		} else {
			calloutCell.AccentBar = &jsonschema.AccentBarInput{
				Position: "left",
				Color:    accent,
				Width:    2,
			}
		}

		rows[i] = jsonschema.GridRowInput{
			Cells: []*jsonschema.GridCellInput{barCell, calloutCell},
		}
	}

	colsJSON, _ := json.Marshal([]float64{hbcLeftColPct, hbcRightColPct})

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     8,
		RowGap:  6,
		Rows:    rows,
	}
	return grid, nil
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

// buildHorizontalBarRowCell constructs the left-column cell containing a
// three-column sub-grid: [label, fill, rest].
func buildHorizontalBarRowCell(bar HorizontalBarCalloutsBar, accent, unit string, labelSize, valueSize, fillPct, restPct float64) *jsonschema.GridCellInput {
	labelCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
			Text:     buildHorizontalBarLabelText(pptx.ConvertMarkdownEmphasis(bar.Label), labelSize),
		},
	}

	valueLabel := formatHorizontalBarValue(bar.Value, unit)
	fillCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     buildHorizontalBarValueText(valueLabel, valueSize),
		},
	}

	cells := []*jsonschema.GridCellInput{labelCell, fillCell}
	cols := []float64{hbcLabelColPct, fillPct}
	if restPct > 0.01 {
		restCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
			},
		}
		cells = append(cells, restCell)
		cols = append(cols, restPct)
	}

	subColsJSON, _ := json.Marshal(cols)
	subGrid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(subColsJSON),
		Gap:     2,
		Rows: []jsonschema.GridRowInput{
			{Cells: cells},
		},
	}

	return &jsonschema.GridCellInput{Grid: subGrid}
}

// buildHorizontalBarCalloutCell constructs the right-column callout cell.
func buildHorizontalBarCalloutCell(callout string, calloutSize float64) *jsonschema.GridCellInput {
	if strings.TrimSpace(callout) == "" {
		return &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
			},
		}
	}
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`{"color": "lt1", "alpha": 0}`),
			Text:     buildHorizontalBarCalloutText(pptx.ConvertMarkdownEmphasis(callout), calloutSize),
		},
	}
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type horizontalBarParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type horizontalBarTextObj struct {
	Paragraphs    []horizontalBarParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
}

func buildHorizontalBarLabelText(label string, size float64) json.RawMessage {
	textObj := horizontalBarTextObj{
		Paragraphs: []horizontalBarParagraph{
			{Content: label, Size: size, Bold: true, Color: "dk1", Align: "r"},
		},
		Align:         "r",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildHorizontalBarValueText(value string, size float64) json.RawMessage {
	textObj := horizontalBarTextObj{
		Paragraphs: []horizontalBarParagraph{
			{Content: value, Size: size, Bold: true, Color: "lt1", Align: "r"},
		},
		Align:         "r",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildHorizontalBarCalloutText(text string, size float64) json.RawMessage {
	textObj := horizontalBarTextObj{
		Paragraphs: []horizontalBarParagraph{
			{Content: text, Size: size, Color: "dk1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// formatHorizontalBarValue renders a numeric value with an optional unit
// suffix. Integers render without a decimal; non-integers use minimal
// precision.
func formatHorizontalBarValue(v float64, unit string) string {
	var s string
	if v == float64(int64(v)) {
		s = strconv.FormatInt(int64(v), 10)
	} else {
		s = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return s + unit
}
