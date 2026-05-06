package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// layoutSuggestion is a single alternative layout recommendation emitted when
// density is suboptimal across cells. Agents use these to pre-flight pattern
// swaps without guessing.
type layoutSuggestion struct {
	Pattern   string         `json:"pattern"`
	Overrides map[string]any `json:"overrides,omitempty"`
	Reason    string         `json:"reason"`
}

// suggestAlternativeLayouts examines cell budgets for a pattern and returns
// layout suggestions when density is consistently suboptimal. Returns nil
// when density is optimal or mixed (no clear recommendation).
//
// Rules (initial seed — expand with usage):
//   - card-grid where all populated cells <50% density → smaller grid or kpi-Nup
//   - card-grid where all populated cells >120% density → larger grid or body_and_bullets
//   - Other patterns: suggest lower/higher density class siblings from taxonomy
func suggestAlternativeLayouts(patternName string, budgets []cellBudgetEntry, reg *patterns.Registry) []layoutSuggestion {
	if len(budgets) == 0 || reg == nil {
		return nil
	}

	// Count populated cells and classify them
	var populated, underfilled, overflowing int
	for _, b := range budgets {
		if b.ActualChars == 0 {
			continue
		}
		populated++
		if b.DensityPct < 50 {
			underfilled++
		}
		if b.DensityPct > 120 {
			overflowing++
		}
	}

	// Need at least one populated cell, and ALL populated cells must be
	// consistently suboptimal for us to make a recommendation.
	if populated == 0 {
		return nil
	}

	allUnderfilled := underfilled == populated
	allOverflowing := overflowing == populated

	if !allUnderfilled && !allOverflowing {
		return nil
	}

	var suggestions []layoutSuggestion

	if patternName == "card-grid" {
		suggestions = suggestForCardGrid(budgets, populated, allUnderfilled, reg)
	} else {
		suggestions = suggestByDensityClass(patternName, allUnderfilled, reg)
	}

	return suggestions
}

// suggestForCardGrid handles card-grid-specific suggestions using grid
// dimension awareness.
func suggestForCardGrid(budgets []cellBudgetEntry, populated int, allUnderfilled bool, reg *patterns.Registry) []layoutSuggestion {
	// Infer current grid dimensions from budget row/col indices
	maxRow, maxCol := 0, 0
	for _, b := range budgets {
		if b.Row > maxRow {
			maxRow = b.Row
		}
		if b.Col > maxCol {
			maxCol = b.Col
		}
	}
	rows := maxRow + 1
	cols := maxCol + 1

	var suggestions []layoutSuggestion

	if allUnderfilled {
		// Content is sparse — suggest fewer/larger cells
		if rows > 1 && cols > 1 {
			// Suggest a smaller grid
			newRows := rows
			newCols := cols
			if cols > rows {
				newCols = cols - 1
			} else {
				newRows = rows - 1
			}
			suggestions = append(suggestions, layoutSuggestion{
				Pattern:   "card-grid",
				Overrides: map[string]any{"columns": newCols, "rows": newRows},
				Reason:    "smaller cells better match short content",
			})
		}
		// Suggest KPI if populated count matches a registered kpi-Nup
		kpiName := kpiNameForCount(populated, reg)
		if kpiName != "" {
			suggestions = append(suggestions, layoutSuggestion{
				Pattern: kpiName,
				Reason:  "fewer items, larger emphasis per item",
			})
		}
	} else {
		// All overflowing — suggest more/larger cells
		newRows := rows
		newCols := cols
		if cols <= rows {
			newCols = cols + 1
		} else {
			newRows = rows + 1
		}
		suggestions = append(suggestions, layoutSuggestion{
			Pattern:   "card-grid",
			Overrides: map[string]any{"columns": newCols, "rows": newRows},
			Reason:    "larger cells accommodate longer content",
		})
	}

	return suggestions
}

// suggestByDensityClass suggests patterns with a different density class
// when all cells are consistently suboptimal.
func suggestByDensityClass(patternName string, allUnderfilled bool, reg *patterns.Registry) []layoutSuggestion {
	pat, ok := reg.Get(patternName)
	if !ok {
		return nil
	}
	currentDensity := pat.Taxonomy().DensityClass
	pairsWith := pat.Taxonomy().PairsWith

	// Build a set of sibling patterns from pairs_with that have the desired
	// density direction.
	var targetDensity string
	if allUnderfilled {
		targetDensity = lowerDensity(currentDensity)
	} else {
		targetDensity = higherDensity(currentDensity)
	}

	if targetDensity == "" {
		return nil // already at the extreme
	}

	var suggestions []layoutSuggestion
	for _, sibling := range pairsWith {
		sib, ok := reg.Get(sibling)
		if !ok {
			continue
		}
		if sib.Taxonomy().DensityClass == targetDensity {
			reason := "lower density layout for sparse content"
			if !allUnderfilled {
				reason = "higher density layout for dense content"
			}
			suggestions = append(suggestions, layoutSuggestion{
				Pattern: sib.Name(),
				Reason:  reason,
			})
		}
	}

	return suggestions
}

// kpiNameForCount returns the kpi-Nup pattern name if one exists for the given
// cell count, or empty string if none is registered.
func kpiNameForCount(count int, reg *patterns.Registry) string {
	if count < 2 || count > 6 {
		return ""
	}
	name := kpiPatternName(count)
	if _, ok := reg.Get(name); ok {
		return name
	}
	return ""
}

// kpiPatternName returns the pattern name for a kpi-Nup variant.
func kpiPatternName(n int) string {
	return fmt.Sprintf("kpi-%dup", n)
}

// lowerDensity returns the next lower density class, or "" if already at minimum.
func lowerDensity(d string) string {
	switch d {
	case "high":
		return "medium"
	case "medium":
		return "low"
	default:
		return ""
	}
}

// higherDensity returns the next higher density class, or "" if already at maximum.
func higherDensity(d string) string {
	switch d {
	case "low":
		return "medium"
	case "medium":
		return "high"
	default:
		return ""
	}
}
