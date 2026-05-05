package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// GridOccupancyInput describes a shape grid's slot usage for underfilled /
// overcrowded detection.
type GridOccupancyInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "slides[0].shape_grid".
	Path string
	// PatternName is the pattern that produced the grid (empty for inline grids).
	PatternName string
	// FilledSlots is the number of non-nil cells in the grid.
	FilledSlots int
	// TotalSlots is rows * columns (the grid capacity).
	TotalSlots int
	// RecommendedMax is the pattern's recommended maximum cell count (0 = no limit).
	RecommendedMax int
}

// underfillThreshold: below 50% filled triggers the finding.
const underfillThreshold = 0.50

// DetectPatternUnderfilled fires when a pattern grid has less than 50% of its
// slots populated. Returns nil when the grid is sufficiently filled or inputs
// are invalid.
func DetectPatternUnderfilled(input GridOccupancyInput) *patterns.FitFinding {
	if input.TotalSlots <= 0 || input.FilledSlots <= 0 {
		return nil
	}

	filledPct := float64(input.FilledSlots) / float64(input.TotalSlots)
	if filledPct >= underfillThreshold {
		return nil
	}

	patName := input.PatternName
	if patName == "" {
		patName = "shape_grid"
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patName,
			Path:    input.Path,
			Code:    patterns.ErrCodePatternUnderfilled,
			Message: fmt.Sprintf(
				"%s: %d of %d slots filled (%.0f%%) — grid is underpopulated",
				patName, input.FilledSlots, input.TotalSlots, filledPct*100,
			),
			Fix: &patterns.FixSuggestion{
				Kind: "swap_pattern",
				Params: map[string]any{
					"filled_pct":   filledPct,
					"filled_slots": input.FilledSlots,
					"total_slots":  input.TotalSlots,
					"reason":       "reshape_grid",
				},
			},
		},
		Action:        "review",
		OverflowRatio: filledPct,
	}
}

// DetectPatternOvercrowded fires when a pattern grid exceeds the recommended
// maximum cell count. Returns nil when within limits or when no limit is set.
func DetectPatternOvercrowded(input GridOccupancyInput) *patterns.FitFinding {
	if input.RecommendedMax <= 0 || input.FilledSlots <= input.RecommendedMax {
		return nil
	}

	patName := input.PatternName
	if patName == "" {
		patName = "shape_grid"
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patName,
			Path:    input.Path,
			Code:    patterns.ErrCodePatternOvercrowded,
			Message: fmt.Sprintf(
				"%s: %d cells exceeds recommended max of %d — consider splitting",
				patName, input.FilledSlots, input.RecommendedMax,
			),
			Fix: &patterns.FixSuggestion{
				Kind: "split_pattern",
				Params: map[string]any{
					"filled_slots":    input.FilledSlots,
					"recommended_max": input.RecommendedMax,
				},
			},
		},
		Action:        "review",
		OverflowRatio: float64(input.FilledSlots) / float64(input.RecommendedMax),
	}
}
