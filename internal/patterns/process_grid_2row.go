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
// process-grid-2row pattern — two parallel tracks of phase boxes with a dk2
// row-label column on the left. Each row carries an equal number of phases.
// Optional column_headers add a header row with an accent underline above
// both tracks; optional outcomes add a row of accent pills beneath them.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&processGrid2Row{})
}

type processGrid2Row struct{}

func (p *processGrid2Row) Name() string { return "process-grid-2row" }
func (p *processGrid2Row) Description() string {
	return "Two parallel process tracks: dk2 row-label column on the left, then N equal-width phase boxes per row"
}
func (p *processGrid2Row) UseWhen() string {
	return "Double-track processes where two parallel workstreams share the same N phase columns (e.g., Design / Production, Strategy / Execution); prefer process-flow for a single linear track, swimlane when steps are owned by distinct actors with potentially different step counts"
}
func (p *processGrid2Row) NotWhen() string {
	return "A single linear sequence (use process-flow), more than two parallel tracks or unequal step counts per track (use swimlane), or tracks need phase descriptions beyond a short label (use roadmap-phased)"
}
func (p *processGrid2Row) Version() int      { return 1 }
func (p *processGrid2Row) CellsHint() string { return "2 × (3-6)" }
func (p *processGrid2Row) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "process-flow"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (p *processGrid2Row) SupportsCallout() bool        { return true }
func (p *processGrid2Row) SupportsInlineMarkdown() bool { return true }

func (p *processGrid2Row) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 4, Rows: 2},
		{Columns: 5, Rows: 2},
		{Columns: 6, Rows: 2},
		{Columns: 7, Rows: 2},
	}
}

func (p *processGrid2Row) ExemplarValues() any {
	return &ProcessGrid2RowValues{
		Row1Label:  "DESIGN PROCESS",
		Row1Phases: []string{"DESIGN", "EDIT", "ASSETS", "UX / UI"},
		Row2Label:  "PRODUCTION",
		Row2Phases: []string{"PROTOTYPE", "DEVELOP", "USER TESTING", "RELEASE"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ProcessGrid2RowValues holds the row labels, phase labels, and per-row colors
// for the process-grid-2row pattern. Both rows must have the same number of
// phases.
type ProcessGrid2RowValues struct {
	Row1Label  string   `json:"row1_label"`
	Row1Phases []string `json:"row1_phases"`
	Row1Color  string   `json:"row1_color,omitempty"`
	Row2Label  string   `json:"row2_label"`
	Row2Phases []string `json:"row2_phases"`
	Row2Color  string   `json:"row2_color,omitempty"`
	// ColumnHeaders names each phase column (one per phase) in a header row
	// with an accent underline above both tracks.
	ColumnHeaders []string `json:"column_headers,omitempty"`
	// Outcomes gives each phase column a result, rendered as a row of accent
	// pills beneath the tracks.
	Outcomes []string `json:"outcomes,omitempty"`
}

// Column header / outcome limits and geometry.
const (
	processGrid2RowHeaderMaxChars  = 24
	processGrid2RowOutcomeMaxChars = 32
	processGrid2RowLabelColPct     = 12.0
	processGrid2RowGapPt           = 6.0
	processGrid2RowUnderlinePt     = 3.0
	// processGrid2RowMeasureSafety narrows the measured width so a header or
	// outcome that nearly fills its column is sized for a second line rather
	// than squeezed when the renderer's font runs wider than the metrics.
	processGrid2RowMeasureSafety = 0.85
)

// ProcessGrid2RowOverrides is the standard text overrides.
type ProcessGrid2RowOverrides = TextOverrides

// ProcessGrid2RowCellOverride is the shared per-cell override; indexed
// row-major as: row1_label, row1_phase[0..N-1], row2_label, row2_phase[0..N-1],
// then column_headers[0..N-1] and outcomes[0..N-1] when present (appended so
// the track indices never move).
type ProcessGrid2RowCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *processGrid2Row) NewValues() any       { return &ProcessGrid2RowValues{} }
func (p *processGrid2Row) NewOverrides() any    { return &ProcessGrid2RowOverrides{} }
func (p *processGrid2Row) NewCellOverride() any { return &ProcessGrid2RowCellOverride{} }

func (p *processGrid2Row) Schema() *Schema {
	phasesSchema := ArraySchema(StringSchema(40), 3, 6).
		WithDescription("Phase labels for this row (3-6 short labels; both rows must have the same count)")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"row1_label":  StringSchema(40).WithDescription("Label for the top row (rendered in the dk2 left column)"),
			"row1_phases": phasesSchema,
			"row1_color":  StringSchema(0).WithDescription("Scheme color for the top-row phase boxes (default accent1)").WithDefault("accent1"),
			"row2_label":  StringSchema(40).WithDescription("Label for the bottom row (rendered in the dk2 left column)"),
			"row2_phases": phasesSchema,
			"row2_color":  StringSchema(0).WithDescription("Scheme color for the bottom-row phase boxes; defaults to a darker tone of row1_color so the two tracks read as parallel rather than unrelated").WithDefault(""),
			"column_headers": ArraySchema(StringSchema(processGrid2RowHeaderMaxChars), 3, 6).
				WithDescription("Optional per-column headers (one per phase; length must equal the phase count) rendered as a header row with an accent underline above both tracks"),
			"outcomes": ArraySchema(StringSchema(processGrid2RowOutcomeMaxChars), 3, 6).
				WithDescription("Optional per-column outcomes (one per phase; length must equal the phase count) rendered as a row of accent pills under the tracks, e.g. \"5-10% wallet share\""),
		},
		[]string{"row1_label", "row1_phases", "row2_label", "row2_phases"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Two parallel process tracks with a dk2 row-label column on the left and N equal-width phase boxes per row")
}

func (p *processGrid2Row) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ProcessGrid2RowValues)
	if !ok || vals == nil {
		return fmt.Errorf("process-grid-2row: values must be *ProcessGrid2RowValues, got %T", values)
	}

	const name = "process-grid-2row"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*ProcessGrid2RowOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if strings.TrimSpace(vals.Row1Label) == "" {
		errs = append(errs, errRequired(name, "row1_label"))
	} else if runeLen(vals.Row1Label) > 40 {
		errs = append(errs, errMaxLength(name, "row1_label", 40, runeLen(vals.Row1Label)))
	}
	if strings.TrimSpace(vals.Row2Label) == "" {
		errs = append(errs, errRequired(name, "row2_label"))
	} else if runeLen(vals.Row2Label) > 40 {
		errs = append(errs, errMaxLength(name, "row2_label", 40, runeLen(vals.Row2Label)))
	}

	if len(vals.Row1Phases) < 3 {
		errs = append(errs, errMinItems(name, "row1_phases", 3, len(vals.Row1Phases), ""))
	}
	if len(vals.Row1Phases) > 6 {
		errs = append(errs, errMaxItems(name, "row1_phases", 6, len(vals.Row1Phases), ""))
	}
	if len(vals.Row2Phases) < 3 {
		errs = append(errs, errMinItems(name, "row2_phases", 3, len(vals.Row2Phases), ""))
	}
	if len(vals.Row2Phases) > 6 {
		errs = append(errs, errMaxItems(name, "row2_phases", 6, len(vals.Row2Phases), ""))
	}

	if len(vals.Row1Phases) != len(vals.Row2Phases) {
		errs = append(errs, newValidationError(name, "row2_phases", ErrCodeCountMismatch,
			fmt.Sprintf("process-grid-2row: row1_phases and row2_phases must have the same length (row1 has %d, row2 has %d)", len(vals.Row1Phases), len(vals.Row2Phases)),
			ResizeListFix("row2_phases", len(vals.Row1Phases))))
	}

	for i, phase := range vals.Row1Phases {
		path := fmt.Sprintf("row1_phases[%d]", i)
		if strings.TrimSpace(phase) == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(phase) > 40 {
			errs = append(errs, errMaxLength(name, path, 40, runeLen(phase)))
		}
	}
	for i, phase := range vals.Row2Phases {
		path := fmt.Sprintf("row2_phases[%d]", i)
		if strings.TrimSpace(phase) == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(phase) > 40 {
			errs = append(errs, errMaxLength(name, path, 40, runeLen(phase)))
		}
	}

	errs = append(errs, validateProcessGrid2RowColumnList(name, "column_headers", vals.ColumnHeaders, processGrid2RowHeaderMaxChars, vals)...)
	errs = append(errs, validateProcessGrid2RowColumnList(name, "outcomes", vals.Outcomes, processGrid2RowOutcomeMaxChars, vals)...)

	// Total cells: 2 row labels + row1_phases + row2_phases (+ headers + outcomes).
	totalCells := 2 + len(vals.Row1Phases) + len(vals.Row2Phases) + len(vals.ColumnHeaders) + len(vals.Outcomes)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// validateProcessGrid2RowColumnList checks an optional one-per-phase-column
// list (column_headers / outcomes): it must match both tracks' phase count,
// and each entry must be non-blank and within limit.
func validateProcessGrid2RowColumnList(name, field string, items []string, limit int, vals *ProcessGrid2RowValues) []error {
	if len(items) == 0 {
		return nil
	}
	var errs []error
	n := len(vals.Row1Phases)
	if len(items) != n || len(vals.Row2Phases) != n {
		errs = append(errs, newValidationError(name, field, ErrCodeCountMismatch,
			fmt.Sprintf("process-grid-2row: %s needs one entry per phase column — row1_phases and row2_phases must have the same length and %s must match it (row1 has %d, row2 has %d, %s has %d)",
				field, field, n, len(vals.Row2Phases), field, len(items)),
			ResizeListFix(field, n)))
	}
	for i, item := range items {
		path := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(item) == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(item) > limit {
			errs = append(errs, errMaxLength(name, path, limit, runeLen(item)))
		}
	}
	return errs
}

func (p *processGrid2Row) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ProcessGrid2RowValues)
	if !ok {
		return nil, fmt.Errorf("process-grid-2row: values must be *ProcessGrid2RowValues, got %T", values)
	}
	ovr := &ProcessGrid2RowOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ProcessGrid2RowOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("process-grid-2row: overrides must be *ProcessGrid2RowOverrides, got %T", overrides)
		}
	}

	// Resolve the base accent for cell-override accent bars; per-row fills come
	// from row1_color / row2_color directly, not from the resolved accent.
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.HeaderSize, 14.0)
	phaseSize := ResolveSize(ovr.BodySize, 12.0)

	row1Color := vals.Row1Color
	if row1Color == "" {
		row1Color = "accent1"
	}
	// row2 defaults to a darker tone of row1, not accent3: the two rows are
	// parallel tracks of one process, and a second brand hue says they are two
	// different things (go-slide-creator-at7ij).
	row2Color := vals.Row2Color
	usePeerToneForRow2 := row2Color == ""

	n := len(vals.Row1Phases)
	if n == 0 || n != len(vals.Row2Phases) {
		// Validate would have caught this; bail out defensively.
		return nil, fmt.Errorf("process-grid-2row: row1_phases (%d) and row2_phases (%d) must be non-empty and equal length", n, len(vals.Row2Phases))
	}

	// Column widths: row-label column ~12%, phase columns split the rest.
	numCols := 1 + n
	cols := make([]float64, numCols)
	cols[0] = processGrid2RowLabelColPct
	phaseWidth := (100 - processGrid2RowLabelColPct) / float64(n)
	for i := 1; i < numCols; i++ {
		cols[i] = phaseWidth
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0

	row1Cells := make([]*jsonschema.GridCellInput, numCols)
	row1Cells[0] = buildProcessGrid2RowLabelCell(ctx, vals.Row1Label, labelSize)
	applyProcessGrid2RowOverride(row1Cells[0], cellOverrides, cellIdx, baseAccent)
	cellIdx++
	for i, phase := range vals.Row1Phases {
		row1Cells[1+i] = buildProcessGrid2RowPhaseCell(ctx, phase, row1Color, phaseSize)
		applyProcessGrid2RowOverride(row1Cells[1+i], cellOverrides, cellIdx, baseAccent)
		cellIdx++
	}

	row2Cells := make([]*jsonschema.GridCellInput, numCols)
	row2Cells[0] = buildProcessGrid2RowLabelCell(ctx, vals.Row2Label, labelSize)
	applyProcessGrid2RowOverride(row2Cells[0], cellOverrides, cellIdx, baseAccent)
	cellIdx++
	for i, phase := range vals.Row2Phases {
		row2Cells[1+i] = buildProcessGrid2RowPhaseTintedCell(ctx, phase, row1Color, row2Color, usePeerToneForRow2, phaseSize)
		applyProcessGrid2RowOverride(row2Cells[1+i], cellOverrides, cellIdx, baseAccent)
		cellIdx++
	}

	rows := []jsonschema.GridRowInput{
		{Cells: row1Cells},
		{Cells: row2Cells},
	}

	// Optional header and outcome rows: content-sized, so the two tracks keep
	// flexing over whatever height is left. Their cell_overrides indices come
	// after the tracks so existing indices stay stable.
	contentW, _ := contentAreaPt(ctx)
	colTextW := contentW*phaseWidth/100 - processGrid2RowGapPt - 2*defaultShapeInsetLRPt
	if len(vals.ColumnHeaders) == n {
		headerRow := buildProcessGrid2RowHeaderRow(ctx, vals.ColumnHeaders, baseAccent, math.Max(phaseSize, labelSize-2), colTextW)
		for i := range vals.ColumnHeaders {
			applyProcessGrid2RowOverride(headerRow.Cells[1+i], cellOverrides, cellIdx, baseAccent)
			cellIdx++
		}
		rows = append([]jsonschema.GridRowInput{headerRow}, rows...)
	}
	if len(vals.Outcomes) == n {
		outcomeRow := buildProcessGrid2RowOutcomeRow(ctx, vals.Outcomes, baseAccent, phaseSize, colTextW)
		for i := range vals.Outcomes {
			applyProcessGrid2RowOverride(outcomeRow.Cells[1+i], cellOverrides, cellIdx, baseAccent)
			cellIdx++
		}
		rows = append(rows, outcomeRow)
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     processGrid2RowGapPt,
		RowGap:  processGrid2RowGapPt,
		Rows:    rows,
	}

	return grid, nil
}

// buildProcessGrid2RowHeaderRow is the optional column-header row: bold
// headers in a readable ink over an accent underline, above both tracks. The
// first cell (over the row-label column) is empty.
func buildProcessGrid2RowHeaderRow(ctx ExpandContext, headers []string, accent string, size, textW float64) jsonschema.GridRowInput {
	ink := inkOnLight(ctx, "dk2", 4.5)
	h := size * contentLineHeight
	cells := make([]*jsonschema.GridCellInput, 1+len(headers))
	cells[0] = &jsonschema.GridCellInput{}
	for i, header := range headers {
		h = math.Max(h, textBlockHeightPt(ctx.Theme.BodyFont, textW*processGrid2RowMeasureSafety, textParagraph{text: header, size: size, bold: true}))
		cells[1+i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(header), size, true, ink),
			},
			AccentBar: &jsonschema.AccentBarInput{Position: "bottom", Color: accent, Width: processGrid2RowUnderlinePt},
		}
	}
	rowPt := math.Round(h + 2*defaultShapeInsetTBPt + processGrid2RowUnderlinePt + 2)
	return jsonschema.GridRowInput{MinHeight: rowPt, MaxHeight: rowPt, Cells: cells}
}

// buildProcessGrid2RowOutcomeRow is the optional outcome row: one pill per
// phase column on the accent's light tint (so it reads as a result, distinct
// from the solid track boxes above), text colour measured against the tint.
// The first cell (under the row-label column) is empty.
func buildProcessGrid2RowOutcomeRow(ctx ExpandContext, outcomes []string, accent string, size, textW float64) jsonschema.GridRowInput {
	tone := inactiveTintTone(accent)
	ink := readableTextOn(ctx, tone, "dk1")
	h := size * contentLineHeight
	cells := make([]*jsonschema.GridCellInput, 1+len(outcomes))
	cells[0] = &jsonschema.GridCellInput{}
	for i, outcome := range outcomes {
		h = math.Max(h, textBlockHeightPt(ctx.Theme.BodyFont, (textW-8)*processGrid2RowMeasureSafety, textParagraph{text: outcome, size: size, bold: true}))
		cells[1+i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "roundRect",
				Adjustments: map[string]int64{"adj": 35000}, // rounded pill ends
				Fill:        tone.fillJSON(),
				Text:        buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(outcome), size, true, ink),
			},
		}
	}
	rowPt := math.Round(h + 2*defaultShapeInsetTBPt + 8)
	return jsonschema.GridRowInput{MinHeight: rowPt, MaxHeight: rowPt, Cells: cells}
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

// processGrid2RowLabelFill is the row-label column fill: the template's dark
// brand colour (dk2) rather than dk1, which is pure black in most themes and
// reads off-brand next to the accent phase boxes.
const processGrid2RowLabelFill = "dk2"

func buildProcessGrid2RowLabelCell(ctx ExpandContext, label string, size float64) *jsonschema.GridCellInput {
	textColor := readableTextOn(ctx, fillTone{Color: processGrid2RowLabelFill}, "lt1")
	text := buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(label), size, true, textColor)
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, processGrid2RowLabelFill)),
			Text:     text,
		},
	}
}

// buildProcessGrid2RowPhaseTintedCell builds a row-2 phase box: a darker tone
// of row1's colour by default, or the authored row2_color verbatim.
func buildProcessGrid2RowPhaseTintedCell(ctx ExpandContext, phase, row1Color, row2Color string, usePeerTone bool, size float64) *jsonschema.GridCellInput {
	if !usePeerTone {
		return buildProcessGrid2RowPhaseCell(ctx, phase, row2Color, size)
	}
	tone := peerTone(row1Color)
	textColor := readableTextOn(ctx, tone, "lt1")
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     peerFillJSON(row1Color),
			Text:     buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(phase), size, true, textColor),
		},
	}
}

func buildProcessGrid2RowPhaseCell(ctx ExpandContext, phase, color string, size float64) *jsonschema.GridCellInput {
	textColor := readableTextOn(ctx, fillTone{Color: color}, "lt1")
	text := buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(phase), size, true, textColor)
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, color)),
			Text:     text,
		},
	}
}

func buildProcessGrid2RowTextContent(content string, size float64, bold bool, color string) json.RawMessage {
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
			{Content: content, Size: size, Bold: bold, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func applyProcessGrid2RowOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*ProcessGrid2RowCellOverride)
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
