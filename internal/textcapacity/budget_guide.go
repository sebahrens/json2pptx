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
	TargetDensity  TargetDensity         `json:"target_density"`
	Configurations []BudgetConfiguration `json:"configurations"`
}

// TargetDensity documents the global density thresholds.
type TargetDensity struct {
	MinPct   int `json:"min_pct"`
	IdealPct int `json:"ideal_pct"`
	MaxPct   int `json:"max_pct"`
}

// BudgetConfiguration is a single grid size with computed character budgets.
type BudgetConfiguration struct {
	Columns        int `json:"columns"`
	Rows           int `json:"rows"`
	BodyMaxChars   int `json:"body_max_chars"`
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

	vAlign, _ := shapegrid.ParseVerticalAlign(grid.VerticalAlign)
	sgGrid := &shapegrid.Grid{
		Bounds:    bounds,
		TypeScale: grid.TypeScale,
		Columns:   colWidths,
		Rows:      sgRows,
		ColGap:    colGap,
		RowGap:    rowGap,
		VAlign:    vAlign,
		// A budget guide describes the pattern alone on a slide: a sparse
		// configuration is measured as the composition policy renders it
		// (stepped type, grown rows), like generation (go-slide-creator-yhzxt).
		Compose:       grid.Bounds == nil,
		KeepTextSizes: grid.KeepTextSizes,
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
		if cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil {
			continue
		}
		// A layer that carries no text is drawing (a ring segment), not a
		// text box whose size bounds the grid's copy.
		if cell.Layer && len(cell.ShapeSpec.Text) == 0 {
			continue
		}
		paras, body := extractCellParagraphs(cell)
		w, h := effectiveTextRect(cell.Bounds, cell.TextInsets, body)
		bodyPt := max(defaultBodyFontPt, dominantFontPt(paras))
		hb := computeTextAreaBudget(w, h, max(defaultHeaderFontPt, bodyPt))
		bb := computeTextAreaBudget(w, h, bodyPt)
		headerBudgets = append(headerBudgets, hb.MaxChars)
		bodyBudgets = append(bodyBudgets, bb.MaxChars)
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
			if c == nil || c.Shape == nil && len(c.Layers) == 0 {
				continue // zero Cell = empty
			}
			cells[j] = shapegrid.Cell{
				ColSpan:   c.ColSpan,
				RowSpan:   c.RowSpan,
				MaxHeight: c.MaxHeight,
				Fit:       shapegrid.FitMode(c.Fit),
				Shape:     convertShape(c.Shape),
			}
			for _, l := range c.Layers {
				cells[j].Layers = append(cells[j].Layers, shapegrid.Layer{
					Frame: shapegrid.LayerFrame{X: l.Frame.X, Y: l.Frame.Y, W: l.Frame.W, H: l.Frame.H},
					Shape: convertShape(l.Shape),
					Name:  l.Name,
				})
			}
		}
		rows[i] = shapegrid.Row{
			Cells:      cells,
			Height:     r.Height,
			AutoHeight: r.AutoHeight,
			Flex:       r.Flex,
			MinHeight:  r.MinHeight,
			MaxHeight:  r.MaxHeight,
		}
	}
	return rows
}

// convertShape converts a shape DTO for capacity measurement; nil stays nil.
func convertShape(s *jsonschema.ShapeSpecInput) *shapegrid.ShapeSpec {
	if s == nil {
		return nil
	}
	return &shapegrid.ShapeSpec{
		Geometry:    s.Geometry,
		TypeScale:   s.TypeScale,
		Fill:        s.Fill,
		Line:        s.Line,
		Text:        s.Text,
		Rotation:    s.Rotation,
		Adjustments: s.Adjustments,
		FlipH:       s.FlipH,
	}
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
