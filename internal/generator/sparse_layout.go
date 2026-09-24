package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// SparseLayoutInput describes visible grid content as an equivalent height
// relative to its bounds. The detector fires below 40% coverage.
type SparseLayoutInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "slides[0].shape_grid".
	Path string
	// BoundsHeightEMU is the allocated grid bounds height in EMU.
	BoundsHeightEMU int64
	// ContentHeightEMU is the estimated content height in EMU, or in area mode
	// visible area divided by grid width.
	ContentHeightEMU int64
	// AreaMeasured means ContentHeightEMU is visible area divided by bounds
	// width, not a literal text extent. Zero is a valid empty grid in this mode.
	AreaMeasured bool

	// PatternName is the pattern that produced the grid (empty for inline grids).
	// When set, the detector can recommend reshape_grid instead of grow_pattern.
	PatternName string
	// FilledSlots is the number of populated cells in the grid.
	FilledSlots int
	// GridRows is the current number of rows in the grid.
	GridRows int
	// GridCols is the current number of columns in the grid.
	GridCols int
}

// sparseLayoutThreshold is the minimum ratio of content/bounds below which
// the finding fires. Content occupying less than 40% of bounds is sparse.
const sparseLayoutThreshold = 0.40

// DetectSparseLayout checks whether a shape grid's content occupies less than
// 40% of the available bounds height. Returns nil when the grid is not sparse
// or when inputs are invalid.
func DetectSparseLayout(input SparseLayoutInput) *patterns.FitFinding {
	if input.BoundsHeightEMU <= 0 || input.ContentHeightEMU < 0 || input.ContentHeightEMU == 0 && !input.AreaMeasured {
		return nil
	}

	filledPct := float64(input.ContentHeightEMU) / float64(input.BoundsHeightEMU)
	if filledPct >= sparseLayoutThreshold {
		return nil
	}

	pat := "shape_grid"
	if input.PatternName != "" {
		pat = input.PatternName
	}

	fix := sparseLayoutFix(input, filledPct)
	message := fmt.Sprintf("content occupies %.0f%% of bounds height (%d / %d EMU) — slide is mostly empty", filledPct*100, input.ContentHeightEMU, input.BoundsHeightEMU)
	if input.AreaMeasured {
		message = fmt.Sprintf("visible content covers %.0f%% of grid area — slide is mostly empty", filledPct*100)
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: pat,
			Path:    input.Path,
			Code:    patterns.ErrCodeSparseLayout,
			Message: message,
			Fix:     fix,
		},
		Action: "review",
		Measured: &patterns.Extent{
			HeightEMU: input.ContentHeightEMU,
		},
		Allowed: &patterns.Extent{
			HeightEMU: input.BoundsHeightEMU,
		},
		OverflowRatio: filledPct,
	}
}

// sparseLayoutFix returns a reshape_grid fix when the input carries pattern
// context (filled slots + grid dimensions), otherwise falls back to grow_pattern.
func sparseLayoutFix(input SparseLayoutInput, filledPct float64) *patterns.FixSuggestion {
	if input.PatternName != "" && input.FilledSlots > 0 && input.GridRows > 0 && input.GridCols > 0 {
		rows, cols := optimalGridDimensions(input.FilledSlots)
		// Only suggest reshape if the dimensions actually differ.
		if rows != input.GridRows || cols != input.GridCols {
			return &patterns.FixSuggestion{
				Kind: "reshape_grid",
				Params: map[string]any{
					"filled_pct":     filledPct,
					"filled_slots":   input.FilledSlots,
					"current_rows":   input.GridRows,
					"current_cols":   input.GridCols,
					"rows":           rows,
					"columns":        cols,
					"bounds_height":  input.BoundsHeightEMU,
					"content_height": input.ContentHeightEMU,
				},
			}
		}
	}

	return &patterns.FixSuggestion{
		Kind: "grow_pattern",
		Params: map[string]any{
			"filled_pct":     filledPct,
			"bounds_height":  input.BoundsHeightEMU,
			"content_height": input.ContentHeightEMU,
		},
	}
}

// optimalGridDimensions computes the most balanced rows × columns for n items,
// preferring wider-than-tall layouts (more columns than rows) to better fill
// widescreen slides.
func optimalGridDimensions(n int) (rows, cols int) {
	if n <= 0 {
		return 1, 1
	}
	if n == 1 {
		return 1, 1
	}

	// Find the largest integer whose square ≤ n.
	bestRows := 1
	for r := 1; r*r <= n; r++ {
		bestRows = r
	}
	// Prefer wider layout: rows = bestRows, cols = ceil(n / bestRows).
	rows = bestRows
	cols = (n + bestRows - 1) / bestRows
	return rows, cols
}
