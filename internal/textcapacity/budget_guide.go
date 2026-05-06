package textcapacity

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// TextBudgetGuide is the top-level text_budget_guide block emitted by show_pattern.
type TextBudgetGuide struct {
	TargetDensity  TargetDensity          `json:"target_density"`
	Configurations []BudgetConfiguration  `json:"configurations"`
}

// TargetDensity documents the global density thresholds.
type TargetDensity struct {
	MinPct   int `json:"min_pct"`
	IdealPct int `json:"ideal_pct"`
	MaxPct   int `json:"max_pct"`
}

// BudgetConfiguration is a single grid size with computed character budgets.
type BudgetConfiguration struct {
	Columns       int `json:"columns"`
	Rows          int `json:"rows"`
	BodyMaxChars  int `json:"body_max_chars"`
	HeaderMaxChars int `json:"header_max_chars"`
}

// DefaultTargetDensity returns the global density thresholds (60/85/110).
func DefaultTargetDensity() TargetDensity {
	return TargetDensity{MinPct: 60, IdealPct: 85, MaxPct: 110}
}

// GridBudgetConfig describes a single columns×rows configuration to compute.
type GridBudgetConfig struct {
	Columns int
	Rows    int
}

// ComputeBudgetGuide generates text budgets for the given grid configurations
// by expanding a synthetic grid and measuring cell capacity. The expandFn
// produces the ShapeGridInput for a given (columns, rows) configuration.
// layoutBounds provides the content area dimensions in EMU.
func ComputeBudgetGuide(
	configs []GridBudgetConfig,
	expandFn func(cols, rows int) (*jsonschema.ShapeGridInput, error),
	layoutBounds pptx.RectEmu,
	slideWidth, slideHeight int64,
) *TextBudgetGuide {
	if len(configs) == 0 {
		return nil
	}

	guide := &TextBudgetGuide{
		TargetDensity:  DefaultTargetDensity(),
		Configurations: make([]BudgetConfiguration, 0, len(configs)),
	}

	for _, cfg := range configs {
		grid, err := expandFn(cfg.Columns, cfg.Rows)
		if err != nil || grid == nil {
			continue
		}

		bc := computeConfigBudget(grid, cfg.Columns, cfg.Rows, layoutBounds, slideWidth, slideHeight)
		if bc != nil {
			guide.Configurations = append(guide.Configurations, *bc)
		}
	}

	if len(guide.Configurations) == 0 {
		return nil
	}
	return guide
}

// Default font sizes for budget computation (matching common pattern defaults).
const (
	defaultHeaderFontPt = 16.0
	defaultBodyFontPt   = 12.0
)

// computeConfigBudget resolves a single grid configuration and computes
// representative header and body character budgets from the cell dimensions.
func computeConfigBudget(
	grid *jsonschema.ShapeGridInput,
	cols, rows int,
	layoutBounds pptx.RectEmu,
	slideWidth, slideHeight int64,
) *BudgetConfiguration {
	// Parse columns
	colWidths, err := resolveColumns(grid.Columns, cols)
	if err != nil {
		return nil
	}

	// Resolve gaps
	colGap := grid.ColGap
	if colGap == 0 {
		colGap = grid.Gap
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}

	// Convert rows
	sgRows := convertRows(grid.Rows)

	bounds := layoutBounds
	if grid.Bounds != nil {
		bounds = shapegrid.BoundsFromPercentages(
			grid.Bounds.X, grid.Bounds.Y,
			grid.Bounds.Width, grid.Bounds.Height,
			slideWidth, slideHeight,
		)
	}

	sgGrid := &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    sgRows,
		ColGap:  colGap,
		RowGap:  rowGap,
	}

	if vErr := shapegrid.Validate(sgGrid); vErr != nil {
		return nil
	}

	alloc := pptx.NewShapeIDAllocator(nil)
	result, err := shapegrid.Resolve(sgGrid, alloc)
	if err != nil || result == nil || len(result.Cells) == 0 {
		return nil
	}

	// Compute header and body budgets from cell dimensions at standard font
	// sizes. We use the minimum across all cells to give agents a safe ceiling.
	var headerBudgets, bodyBudgets []int
	for _, cell := range result.Cells {
		w := cell.CellBounds.CX
		h := cell.CellBounds.CY
		if w <= 0 || h <= 0 {
			continue
		}
		hb := computeBudget(w, h, defaultHeaderFontPt)
		bb := computeBudget(w, h, defaultBodyFontPt)
		if hb.MaxChars > 0 {
			headerBudgets = append(headerBudgets, hb.MaxChars)
		}
		if bb.MaxChars > 0 {
			bodyBudgets = append(bodyBudgets, bb.MaxChars)
		}
	}

	if len(bodyBudgets) == 0 {
		return nil
	}

	return &BudgetConfiguration{
		Columns:        cols,
		Rows:           rows,
		BodyMaxChars:   minInts(bodyBudgets),
		HeaderMaxChars: minInts(headerBudgets),
	}
}

// resolveColumns parses the grid columns field into percentage widths.
func resolveColumns(raw json.RawMessage, fallbackCount int) ([]float64, error) {
	if len(raw) == 0 {
		// Equal-width columns
		w := 100.0 / float64(fallbackCount)
		cols := make([]float64, fallbackCount)
		for i := range cols {
			cols[i] = w
		}
		return cols, nil
	}

	// Try integer (equal-width columns)
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		if n < 1 {
			return nil, fmt.Errorf("columns must be >= 1, got %d", n)
		}
		w := 100.0 / float64(n)
		cols := make([]float64, n)
		for i := range cols {
			cols[i] = w
		}
		return cols, nil
	}

	// Try array of floats
	var arr []float64
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("columns must be integer or array: %w", err)
	}
	return arr, nil
}

// convertRows converts jsonschema GridRowInput to shapegrid.Row.
func convertRows(inputRows []jsonschema.GridRowInput) []shapegrid.Row {
	rows := make([]shapegrid.Row, len(inputRows))
	for i, r := range inputRows {
		cells := make([]shapegrid.Cell, len(r.Cells))
		for j, c := range r.Cells {
			if c == nil || c.Shape == nil {
				continue // zero Cell = empty
			}
			cells[j] = shapegrid.Cell{
				ColSpan: c.ColSpan,
				Shape: &shapegrid.ShapeSpec{
					Geometry: c.Shape.Geometry,
					Fill:     c.Shape.Fill,
					Line:     c.Shape.Line,
					Text:     c.Shape.Text,
					Rotation: c.Shape.Rotation,
				},
			}
		}
		rows[i] = shapegrid.Row{
			Cells:     cells,
			Height:    r.Height,
			MaxHeight: r.MaxHeight,
		}
	}
	return rows
}

func minInts(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
