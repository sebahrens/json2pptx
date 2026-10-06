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
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// ---------------------------------------------------------------------------
// process-grid-2row pattern — two parallel tracks of phases behind a row
// label. Each row carries an equal number of phases. The default "lanes"
// style (process_grid_2row_lanes.go) draws each track as a pentagon label
// pointing into interlocking chevrons in tints of one accent; "tinted" and
// "solid" are the earlier box grids in this file. Optional column_headers
// name the phase columns above both tracks; optional outcomes give each
// column a result beneath them.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&processGrid2Row{})
}

type processGrid2Row struct{}

func (p *processGrid2Row) Name() string { return "process-grid-2row" }
func (p *processGrid2Row) Description() string {
	return "Two parallel process lanes: a row-label pentagon pointing into N interlocking phase chevrons per row, in tints of one accent"
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
		Row1Label:  "Design",
		Row1Phases: []string{"Research", "Concept", "Prototype", "Handoff"},
		Row2Label:  "Production",
		Row2Phases: []string{"Plan", "Build", "Test", "Release"},
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
	// processGrid2RowLabelColMaxPct is how wide the row-label column may grow
	// to keep a label word on one line before the label shrinks.
	processGrid2RowLabelColMaxPct = 22.0
	processGrid2RowGapPt          = 6.0
	processGrid2RowUnderlinePt    = 3.0
	// processGrid2RowMeasureSafety narrows the measured width so a header or
	// outcome that nearly fills its column is sized for a second line rather
	// than squeezed when the renderer's font runs wider than the metrics.
	processGrid2RowMeasureSafety = 0.85
)

// ProcessGrid2RowOverrides is the standard text overrides plus the phase box
// treatment.
type ProcessGrid2RowOverrides struct {
	TextOverrides
	// Style is "lanes" (default: a pentagon row label pointing into
	// interlocking phase chevrons, in tints of the track colour), "tinted"
	// (neutral-tint phase boxes with dark text under a thin rule in the track
	// colour, beside a dark label block) or "solid" (phase boxes filled with
	// the track colour).
	Style string `json:"style,omitempty"`
}

// processGrid2RowStyles are the accepted overrides.style values.
var processGrid2RowStyles = []string{processGrid2RowStyleLanes, processGrid2RowStyleTinted, processGrid2RowStyleSolid}

// processGrid2RowOverridesSchema is the text overrides schema plus style.
func processGrid2RowOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(processGrid2RowStyles...).WithDescription("lanes (default): each track is a process lane, a pentagon row label pointing into interlocking phase chevrons, in lighter tints of one accent (first lane: solid accent label over Lighter 60% phases; second lane: Lighter 40% label over Lighter 80% phases); lanes are content-sized, column_headers are bold text over the lanes and outcomes bold accent text under them. tinted: the earlier box grid, neutral-tint phase boxes under a thin rule in the track colour beside a dark label block, filling the area. solid: that grid with phase boxes filled with the track colour").WithDefault(processGrid2RowStyleLanes)
	return s
}

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
			"row1_label":  StringSchema(40).WithDescription("Label for the top row (the pentagon that leads its lane; one or two words read best)"),
			"row1_phases": phasesSchema,
			"row1_color":  StringSchema(0).WithDescription("Scheme color both lanes are tinted from (style tinted: the phase rules; solid: box fills); default: the template's color_roles.primary_fill").WithDefault("accent1"),
			"row2_label":  StringSchema(40).WithDescription("Label for the bottom row (the pentagon that leads its lane)"),
			"row2_phases": phasesSchema,
			"row2_color":  StringSchema(0).WithDescription("Scheme color for the bottom lane; default: lighter tints of row1_color, so the two tracks read as parallel rather than unrelated (style solid: a darker tone of it). When set, the bottom lane takes the top lane's depths in this color").WithDefault(""),
			"column_headers": ArraySchema(StringSchema(processGrid2RowHeaderMaxChars), 3, 6).
				WithDescription("Optional per-column headers (one per phase; length must equal the phase count) rendered as bold headers over both tracks (style tinted / solid: over an accent underline)"),
			"outcomes": ArraySchema(StringSchema(processGrid2RowOutcomeMaxChars), 3, 6).
				WithDescription("Optional per-column outcomes (one per phase; length must equal the phase count) rendered as bold accent text under the tracks (style tinted / solid: tinted pills), e.g. \"5-10% wallet share\""),
		},
		[]string{"row1_label", "row1_phases", "row2_label", "row2_phases"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      processGrid2RowOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Two parallel process lanes: a row-label pentagon pointing into N interlocking phase chevrons per row")
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
			if ovr.Style != "" && !slices.Contains(processGrid2RowStyles, ovr.Style) {
				errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, processGrid2RowStyles))
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

	if processGrid2RowStyle(ovr) == processGrid2RowStyleLanes {
		return p.expandLanes(ctx, vals, ovr, cellOverrides)
	}

	// Resolve the base accent for cell-override accent bars; per-row fills come
	// from row1_color / row2_color directly, not from the resolved accent.
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	phaseSize := ResolveSize(ovr.BodySize, scaleBodyPt)

	// The row labels sit in the narrow first column: widen it, then shrink
	// one shared size, until no label word breaks mid-word ("PRODUCTI / ON",
	// go-slide-creator-csclk.113 / b7qqg.15). The rows are then measured at
	// their written fit, stepping the label to the 12pt floor when the
	// tracks would not otherwise fit (go-slide-creator-n1muf).
	lay := layoutProcessGrid2Row(ctx, vals, ovr)
	lf := lay.label
	labelSize := lay.labelSize
	rowLabelSize, labelColPct := lf.sizePt, lf.colPct

	row1Color := vals.Row1Color
	if row1Color == "" {
		row1Color = ctx.DefaultAccent()
	}
	// row2 defaults to a darker tone of row1, not accent3: the two rows are
	// parallel tracks of one process, and a second brand hue says they are two
	// different things (go-slide-creator-at7ij).
	row2Color := vals.Row2Color
	usePeerToneForRow2 := row2Color == ""
	// Tinted by default: two rows of solid accent phase boxes were a wall of
	// colour (go-slide-creator-fl11f); the track colour marks each box with
	// a thin top rule instead. overrides.style "solid" restores the fills.
	solid := ovr.Style == processGrid2RowStyleSolid

	n := len(vals.Row1Phases)
	if n == 0 || n != len(vals.Row2Phases) {
		// Validate would have caught this; bail out defensively.
		return nil, fmt.Errorf("process-grid-2row: row1_phases (%d) and row2_phases (%d) must be non-empty and equal length", n, len(vals.Row2Phases))
	}

	// Column widths: row-label column ~12%, phase columns split the rest.
	numCols := 1 + n
	cols := make([]float64, numCols)
	cols[0] = labelColPct
	phaseWidth := (100 - labelColPct) / float64(n)
	for i := 1; i < numCols; i++ {
		cols[i] = phaseWidth
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0

	row1Cells := make([]*jsonschema.GridCellInput, numCols)
	row1Cells[0] = buildProcessGrid2RowLabelCell(ctx, vals.Row1Label, rowLabelSize)
	applyProcessGrid2RowOverride(row1Cells[0], cellOverrides, cellIdx, baseAccent)
	cellIdx++
	// Under column headers the header underline already tops the first
	// track; a second rule beneath it would read as a doubled line.
	row1Rule := row1Color
	if len(vals.ColumnHeaders) == n {
		row1Rule = ""
	}
	for i, phase := range vals.Row1Phases {
		if solid {
			row1Cells[1+i] = buildProcessGrid2RowPhaseCell(ctx, phase, row1Color, phaseSize)
		} else {
			row1Cells[1+i] = buildProcessGrid2RowPhaseRuledCell(ctx, phase, row1Rule, NeutralTint8, phaseSize)
		}
		applyProcessGrid2RowOverride(row1Cells[1+i], cellOverrides, cellIdx, baseAccent)
		cellIdx++
	}

	row2Cells := make([]*jsonschema.GridCellInput, numCols)
	row2Cells[0] = buildProcessGrid2RowLabelCell(ctx, vals.Row2Label, rowLabelSize)
	applyProcessGrid2RowOverride(row2Cells[0], cellOverrides, cellIdx, baseAccent)
	cellIdx++
	for i, phase := range vals.Row2Phases {
		switch {
		case solid:
			row2Cells[1+i] = buildProcessGrid2RowPhaseTintedCell(ctx, phase, row1Color, row2Color, usePeerToneForRow2, phaseSize)
		case usePeerToneForRow2:
			row2Cells[1+i] = buildProcessGrid2RowPhaseRuledCell(ctx, phase, row1Color, NeutralTint4, phaseSize)
		default:
			row2Cells[1+i] = buildProcessGrid2RowPhaseRuledCell(ctx, phase, row2Color, NeutralTint4, phaseSize)
		}
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
	colTextW := contentW*phaseWidth/100 - ctx.Gap(processGrid2RowGapPt) - 2*defaultShapeInsetLRPt
	// Track rows flex over the height the optional rows leave, each floored
	// at the written fit of its tallest cell.
	floorFlexRowsAtNeeds(rows, lay.trackNeeds[:], lay.trackAvailPt)
	if len(vals.ColumnHeaders) == n {
		headerRow := buildProcessGrid2RowHeaderRow(ctx, vals.ColumnHeaders, baseAccent, math.Max(phaseSize, labelSize-2), colTextW)
		headerRow.MinHeight = math.Max(headerRow.MinHeight, lay.headerPt)
		headerRow.MaxHeight = headerRow.MinHeight
		for i := range vals.ColumnHeaders {
			applyProcessGrid2RowOverride(headerRow.Cells[1+i], cellOverrides, cellIdx, baseAccent)
			cellIdx++
		}
		rows = append([]jsonschema.GridRowInput{headerRow}, rows...)
	}
	if len(vals.Outcomes) == n {
		outcomeRow := buildProcessGrid2RowOutcomeRow(ctx, vals.Outcomes, baseAccent, phaseSize, colTextW)
		outcomeRow.MinHeight = math.Max(outcomeRow.MinHeight, lay.outcomePt)
		outcomeRow.MaxHeight = outcomeRow.MinHeight
		for i := range vals.Outcomes {
			applyProcessGrid2RowOverride(outcomeRow.Cells[1+i], cellOverrides, cellIdx, baseAccent)
			cellIdx++
		}
		rows = append(rows, outcomeRow)
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     ctx.Gap(processGrid2RowGapPt),
		RowGap:  ctx.Gap(processGrid2RowGapPt),
		Rows:    rows,
	}

	return grid, nil
}

// processGrid2RowLabelFit is the measured row-label column: its width (percent
// of the grid), the shared label size, and the label words that still cannot
// fit one line at the floor.
type processGrid2RowLabelFit struct {
	colPct float64
	sizePt float64
	unfit  []string
}

// fitProcessGrid2RowLabels sizes the row-label column so every label word
// renders on one line. The readable size wins over the phase columns' width:
// each size from the requested one down to the renderer's floor first tries
// widening the column (12% → processGrid2RowLabelColMaxPct), and only a word
// that fits no column at that size costs a point. A word that fits no column
// even at the floor leaves the column at its default 12%. Words are measured bold in
// the theme body font against the atomic-token width, so a substituted face
// still keeps "PRODUCTION" whole (go-slide-creator-b7qqg.15).
func fitProcessGrid2RowLabels(ctx ExpandContext, vals *ProcessGrid2RowValues, labelSize float64) processGrid2RowLabelFit {
	var words []string
	for _, label := range []string{vals.Row1Label, vals.Row2Label} {
		words = append(words, strings.Fields(label)...)
	}
	areaW, _ := contentAreaPt(ctx)
	font := ctx.Theme.BodyFont
	textW := func(colPct float64) float64 {
		return math.Max(areaW*colPct/100-ctx.Gap(processGrid2RowGapPt)-2*defaultShapeInsetLRPt, 1)
	}
	fits := func(size, colPct float64) bool {
		w := textfit.AtomicTokenWidthPt(font, textW(colPct))
		for _, word := range words {
			if measuredLines(word, font, true, size, w) > 1 {
				return false
			}
		}
		return true
	}
	floor := math.Min(labelSize, shapegrid.MinTextSizePt)
	for size := labelSize; size >= floor; size-- {
		for colPct := processGrid2RowLabelColPct; colPct <= processGrid2RowLabelColMaxPct; colPct++ {
			if fits(size, colPct) {
				return processGrid2RowLabelFit{colPct: colPct, sizePt: size}
			}
		}
	}
	// No column width saves the label, so widening would only take room from
	// the phase columns: keep the default column and report the words.
	fit := processGrid2RowLabelFit{colPct: processGrid2RowLabelColPct, sizePt: floor}
	w := textfit.AtomicTokenWidthPt(font, textW(processGrid2RowLabelColMaxPct))
	for _, word := range words {
		if measuredLines(word, font, true, floor, w) > 1 {
			fit.unfit = append(fit.unfit, word)
		}
	}
	return fit
}

// processGrid2RowLayout is the measured sizing of the whole grid: the label
// column fit, the requested label size it was fitted from, the header and
// outcome row heights (0 when absent), the written-fit height each track
// needs, and the height the two tracks share.
type processGrid2RowLayout struct {
	label               processGrid2RowLabelFit
	labelSize           float64
	headerPt, outcomePt float64
	trackNeeds          [2]float64
	trackAvailPt        float64
}

// layoutProcessGrid2Row measures every row at the width the grid gives its
// cells, as the writer will write them (writtenFitHeightPt): the header and
// outcome rows are never shorter than their written fit, and each track is
// floored at the written fit of its tallest cell. When the tracks would not
// fit the height the optional rows leave, the default 14pt row label (and
// the column headers sized from it) step to the 12pt floor; an authored
// header_size is kept (go-slide-creator-n1muf).
func layoutProcessGrid2Row(ctx ExpandContext, vals *ProcessGrid2RowValues, ovr *ProcessGrid2RowOverrides) processGrid2RowLayout {
	phaseSize := ResolveSize(ovr.BodySize, scaleBodyPt)
	sizes := []float64{ResolveSize(ovr.HeaderSize, scaleSubheadPt)}
	if ovr.HeaderSize == 0 {
		sizes = append(sizes, shapegrid.MinTextSizePt)
	}
	n := len(vals.Row1Phases)
	areaW, areaH := sizingAreaPt(ctx)
	contentW, _ := contentAreaPt(ctx)
	var lay processGrid2RowLayout
	for _, size := range sizes {
		lay = processGrid2RowLayout{labelSize: size, label: fitProcessGrid2RowLabels(ctx, vals, size)}
		if n == 0 || n != len(vals.Row2Phases) {
			return lay
		}
		usable := areaW - float64(n)*ctx.Gap(processGrid2RowGapPt)
		labelW := usable * lay.label.colPct / 100
		phaseW := usable * (100 - lay.label.colPct) / 100 / float64(n)
		colTextW := contentW*(100-lay.label.colPct)/float64(n)/100 - ctx.Gap(processGrid2RowGapPt) - 2*defaultShapeInsetLRPt
		fixed := ctx.Gap(processGrid2RowGapPt) // between the two tracks
		if len(vals.ColumnHeaders) == n {
			row := buildProcessGrid2RowHeaderRow(ctx, vals.ColumnHeaders, "accent1", math.Max(phaseSize, size-2), colTextW)
			lay.headerPt = row.MinHeight
			for _, c := range row.Cells[1:] {
				lay.headerPt = math.Max(lay.headerPt, math.Ceil(writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, phaseW, 0)+processGrid2RowUnderlinePt+2))
			}
			fixed += lay.headerPt + ctx.Gap(processGrid2RowGapPt)
		}
		if len(vals.Outcomes) == n {
			row := buildProcessGrid2RowOutcomeRow(ctx, vals.Outcomes, "accent1", phaseSize, colTextW)
			lay.outcomePt = row.MinHeight
			for _, c := range row.Cells[1:] {
				lay.outcomePt = math.Max(lay.outcomePt, math.Ceil(writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, phaseW, 0)+8))
			}
			fixed += lay.outcomePt + ctx.Gap(processGrid2RowGapPt)
		}
		lay.trackAvailPt = areaH - fixed
		for r, track := range [2]struct {
			label  string
			phases []string
		}{{vals.Row1Label, vals.Row1Phases}, {vals.Row2Label, vals.Row2Phases}} {
			need := writtenFitHeightPt(ctx.themeFonts(), buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(track.label), lay.label.sizePt, true, "lt1"), labelW, 0)
			for _, phase := range track.phases {
				need = math.Max(need, writtenFitHeightPt(ctx.themeFonts(), buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(phase), phaseSize, true, "lt1"), phaseW, 0))
			}
			lay.trackNeeds[r] = need
		}
		if lay.trackNeeds[0]+lay.trackNeeds[1] <= lay.trackAvailPt {
			break
		}
	}
	return lay
}

// PostExpandWarnings reports row-label words that break mid-word even in the
// widest label column at the readable floor: the renderer will split them
// ("PRODUCTI / ON"), and only a shorter label fixes that. With the template's
// content area known it also reports tracks whose written fit does not fit
// the height the header and outcome rows leave (go-slide-creator-n1muf).
func (p *processGrid2Row) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	vals, ok := values.(*ProcessGrid2RowValues)
	if !ok || vals == nil {
		return nil
	}
	ovr, _ := overrides.(*ProcessGrid2RowOverrides)
	if ovr == nil {
		ovr = &ProcessGrid2RowOverrides{}
	}
	if processGrid2RowStyle(ovr) == processGrid2RowStyleLanes {
		return p.lanesWarnings(ctx, vals, ovr)
	}
	lay := layoutProcessGrid2Row(ctx, vals, ovr)
	var warnings []string
	if fit := lay.label; len(fit.unfit) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%s: process-grid-2row row label word(s) %s cannot fit one line of the label column (even widened to %.0f%%) at %.0fpt — the renderer breaks them mid-word; shorten or abbreviate row1_label / row2_label",
			ErrCodeTextExceedsShape, listFirstN(fit.unfit, 3), processGrid2RowLabelColMaxPct, fit.sizePt))
	}
	if ctx.LayoutBounds.Width > 0 && ctx.LayoutBounds.Height > 0 {
		if need := lay.trackNeeds[0] + lay.trackNeeds[1]; need > lay.trackAvailPt+1 {
			warnings = append(warnings, fmt.Sprintf(
				"%s: process-grid-2row tracks need %.0fpt at readable sizes but the content area leaves about %.0fpt for them — shorten the row labels or phase labels, drop column_headers or outcomes, or use fewer phases",
				ErrCodeBodyTooLong, need, math.Max(lay.trackAvailPt, 0)))
		}
	}
	return warnings
}

// buildProcessGrid2RowHeaderRow is the optional column-header row: bold
// headers in a readable ink over an accent underline, above both tracks. The
// first cell (over the row-label column) is empty.
func buildProcessGrid2RowHeaderRow(ctx ExpandContext, headers []string, accent string, size, textW float64) jsonschema.GridRowInput {
	ink := inkOnLight(ctx, "dk2", 4.5)
	h := size * contentLineHeight
	// Leading empty cell over the row-label column, then one per header.
	cells := []*jsonschema.GridCellInput{{}}
	for _, header := range headers {
		h = math.Max(h, textBlockHeightPt(ctx.Theme.BodyFont, textW*processGrid2RowMeasureSafety, textParagraph{text: header, size: size, bold: true}))
		cells = append(cells, &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(header), size, true, ink),
			},
			AccentBar: &jsonschema.AccentBarInput{Position: "bottom", Color: accent, Width: processGrid2RowUnderlinePt},
		})
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
	// Leading empty cell under the row-label column, then one per outcome.
	cells := []*jsonschema.GridCellInput{{}}
	for _, outcome := range outcomes {
		h = math.Max(h, textBlockHeightPt(ctx.Theme.BodyFont, (textW-8)*processGrid2RowMeasureSafety, textParagraph{text: outcome, size: size, bold: true}))
		cells = append(cells, &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "roundRect",
				Adjustments: map[string]int64{"adj": 35000}, // rounded pill ends
				Fill:        tone.fillJSON(),
				Text:        buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(outcome), size, true, ink),
			},
		})
	}
	rowPt := math.Round(h + 2*defaultShapeInsetTBPt + 8)
	return jsonschema.GridRowInput{MinHeight: rowPt, MaxHeight: rowPt, Cells: cells}
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

// buildProcessGrid2RowLabelCell builds the row-label column cell. Its fill is
// the template's dark brand colour (dk2), or dk1 at 60% when dk2 is black: a
// solid black block is the heavy "structure" fill the design review rejected
// (go-slide-creator-8xsj3). Ink is measured on the fill actually painted.
func buildProcessGrid2RowLabelCell(ctx ExpandContext, label string, size float64) *jsonschema.GridCellInput {
	tone := structuralDarkTone(ctx)
	textColor := readableTextOn(ctx, tone, "lt1")
	text := buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(label), size, true, textColor)
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     tone.fillJSON(),
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

// buildProcessGrid2RowPhaseRuledCell is the restrained phase box: a neutral
// tint with dark bold text under a thin rule in the track colour (none when
// color is empty). The second track takes the lighter tint step so the
// parallel rows stay distinguishable (go-slide-creator-at7ij).
func buildProcessGrid2RowPhaseRuledCell(ctx ExpandContext, phase, color string, tint int, size float64) *jsonschema.GridCellInput {
	cell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     neutralFillJSON(tint),
			Line:     noLine,
			Text:     buildProcessGrid2RowTextContent(pptx.ConvertMarkdownEmphasis(phase), size, true, readableTextOn(ctx, neutralTone(tint), "dk1")),
		},
	}
	if color != "" {
		cell.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: color, Width: peerRuleWidthPt}
	}
	return cell
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
