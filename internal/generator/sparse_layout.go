package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// SparseLayoutInput describes a shape grid's content vs available height for
// sparse layout detection. The detector fires when rendered content occupies
// less than 40% of the available bounds height.
type SparseLayoutInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "slides[0].shape_grid".
	Path string
	// BoundsHeightEMU is the allocated grid bounds height in EMU.
	BoundsHeightEMU int64
	// ContentHeightEMU is the estimated content height in EMU (sum of row
	// content heights plus inter-row gaps).
	ContentHeightEMU int64
}

// sparseLayoutThreshold is the minimum ratio of content/bounds below which
// the finding fires. Content occupying less than 40% of bounds is sparse.
const sparseLayoutThreshold = 0.40

// DetectSparseLayout checks whether a shape grid's content occupies less than
// 40% of the available bounds height. Returns nil when the grid is not sparse
// or when inputs are invalid.
func DetectSparseLayout(input SparseLayoutInput) *patterns.FitFinding {
	if input.BoundsHeightEMU <= 0 || input.ContentHeightEMU <= 0 {
		return nil
	}

	filledPct := float64(input.ContentHeightEMU) / float64(input.BoundsHeightEMU)
	if filledPct >= sparseLayoutThreshold {
		return nil
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "shape_grid",
			Path:    input.Path,
			Code:    patterns.ErrCodeSparseLayout,
			Message: fmt.Sprintf(
				"content occupies %.0f%% of bounds height (%d / %d EMU) — slide is mostly empty",
				filledPct*100, input.ContentHeightEMU, input.BoundsHeightEMU,
			),
			Fix: &patterns.FixSuggestion{
				Kind: "grow_pattern",
				Params: map[string]any{
					"filled_pct":     filledPct,
					"bounds_height":  input.BoundsHeightEMU,
					"content_height": input.ContentHeightEMU,
				},
			},
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
