package pipeline

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Threshold constants for structural smell detection.
const (
	// minTableGapPt is the minimum acceptable gap (in points) between sibling
	// rows that both contain tables. Below this, the tables appear visually stacked.
	minTableGapPt = 4.0

	// minDividerGapPt is the minimum acceptable gap (in points) between any
	// sibling shape rows. Below this, shapes appear crushed together.
	minDividerGapPt = 3.0

	// defaultGridGapPt is the default gap between rows/columns when none is specified.
	defaultGridGapPt = 8.0

	// minDividerHeightPct is the minimum height percentage of the slide that a
	// divider shape row should occupy. Rows shorter than this are flagged.
	minDividerHeightPct = 4.0
)

// hexColorPattern matches #RGB or #RRGGBB hex color strings.
var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// brandAllowlist contains hex values that are NOT considered "hex fills" for
// the mixed-fill-scheme check (black, white).
var brandAllowlist = map[string]bool{
	"#000000": true, "#000": true,
	"#ffffff": true, "#fff": true,
}

// DetectStructuralSmells runs all structural smell detectors on a single slide's
// shape_grid and returns any warnings as ValidationError values. The slideIdx is
// zero-based (matches JSON path indices).
func DetectStructuralSmells(grid *jsonschema.ShapeGridInput, slideIdx int) []*patterns.ValidationError {
	if grid == nil || len(grid.Rows) == 0 {
		return nil
	}

	var warnings []*patterns.ValidationError
	warnings = append(warnings, detectStackedTables(grid, slideIdx)...)
	warnings = append(warnings, detectDividerTooThin(grid, slideIdx)...)
	warnings = append(warnings, detectMixedFillScheme(grid, slideIdx)...)
	return warnings
}

// effectiveRowGap returns the effective row gap in points for the grid.
func effectiveRowGap(grid *jsonschema.ShapeGridInput) float64 {
	if grid.RowGap > 0 {
		return grid.RowGap
	}
	if grid.Gap > 0 {
		return grid.Gap
	}
	return defaultGridGapPt
}

// rowHasTable reports whether any cell in the row contains a table.
func rowHasTable(row jsonschema.GridRowInput) bool {
	for _, cell := range row.Cells {
		if cell != nil && cell.Table != nil {
			return true
		}
	}
	return false
}

// detectStackedTables flags consecutive rows that both contain tables when the
// computed gap between them is less than minTableGapPt.
func detectStackedTables(grid *jsonschema.ShapeGridInput, slideIdx int) []*patterns.ValidationError {
	gap := effectiveRowGap(grid)
	var warnings []*patterns.ValidationError

	for i := 0; i < len(grid.Rows)-1; i++ {
		if !rowHasTable(grid.Rows[i]) || !rowHasTable(grid.Rows[i+1]) {
			continue
		}
		if gap < minTableGapPt {
			path := fmt.Sprintf("slides[%d].shape_grid.rows[%d:%d]", slideIdx, i, i+1)
			warnings = append(warnings, &patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    path,
				Code:    patterns.ErrCodeStackedTables,
				Message: fmt.Sprintf("slide %d: rows %d and %d both contain tables with only %.1fpt gap (minimum %.1fpt)", slideIdx+1, i, i+1, gap, minTableGapPt),
				Fix: &patterns.FixSuggestion{
					Kind: "increase_gap",
					Params: map[string]any{
						"current_pt": gap,
						"minimum_pt": minTableGapPt,
					},
				},
			})
		}
	}

	return warnings
}

// detectDividerTooThin flags consecutive rows where the computed gap between
// them is less than minDividerGapPt, or where a row's height percentage is
// below minDividerHeightPct (indicating a near-invisible divider row).
func detectDividerTooThin(grid *jsonschema.ShapeGridInput, slideIdx int) []*patterns.ValidationError {
	gap := effectiveRowGap(grid)
	var warnings []*patterns.ValidationError

	// Check row gaps.
	if gap < minDividerGapPt && len(grid.Rows) > 1 {
		for i := 0; i < len(grid.Rows)-1; i++ {
			path := fmt.Sprintf("slides[%d].shape_grid.rows[%d:%d]", slideIdx, i, i+1)
			warnings = append(warnings, &patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    path,
				Code:    patterns.ErrCodeDividerTooThin,
				Message: fmt.Sprintf("slide %d: gap between rows %d and %d is %.1fpt (minimum %.1fpt)", slideIdx+1, i, i+1, gap, minDividerGapPt),
				Fix: &patterns.FixSuggestion{
					Kind: "increase_gap",
					Params: map[string]any{
						"current_pt": gap,
						"minimum_pt": minDividerGapPt,
					},
				},
			})
		}
	}

	// Check for rows with height < minDividerHeightPct (divider-like rows).
	for i, row := range grid.Rows {
		if row.Height > 0 && row.Height < minDividerHeightPct {
			path := fmt.Sprintf("slides[%d].shape_grid.rows[%d]", slideIdx, i)
			warnings = append(warnings, &patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    path,
				Code:    patterns.ErrCodeDividerTooThin,
				Message: fmt.Sprintf("slide %d: row %d height is %.1f%% of slide (minimum %.1f%%)", slideIdx+1, i, row.Height, minDividerHeightPct),
				Fix: &patterns.FixSuggestion{
					Kind: "increase_row_height",
					Params: map[string]any{
						"current_pct": row.Height,
						"minimum_pct": minDividerHeightPct,
					},
				},
			})
		}
	}

	return warnings
}

// detectMixedFillScheme flags slides where the shape_grid contains both
// non-allowlist hex fills AND semantic (scheme color) fills. Mixing the two
// makes the deck non-portable across templates.
func detectMixedFillScheme(grid *jsonschema.ShapeGridInput, slideIdx int) []*patterns.ValidationError {
	var hasHex, hasSemantic bool
	var hexExample, semanticExample string

	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Fill) == 0 {
				continue
			}
			color := extractFillColor(cell.Shape.Fill)
			if color == "" {
				continue
			}
			if hexColorPattern.MatchString(color) {
				if !brandAllowlist[strings.ToLower(color)] {
					if !hasHex {
						hexExample = color
					}
					hasHex = true
				}
			} else {
				if !hasSemantic {
					semanticExample = color
				}
				hasSemantic = true
			}
		}
	}

	if hasHex && hasSemantic {
		path := fmt.Sprintf("slides[%d].shape_grid", slideIdx)
		return []*patterns.ValidationError{{
			Pattern: "shape_grid",
			Path:    path,
			Code:    patterns.ErrCodeMixedFillScheme,
			Message: fmt.Sprintf("slide %d: shape_grid mixes hex fills (e.g. %s) and semantic fills (e.g. %s); use one scheme for template portability", slideIdx+1, hexExample, semanticExample),
			Fix: &patterns.FixSuggestion{
				Kind: "use_semantic_color",
				Params: map[string]any{
					"message": "replace all hex fill colors with scheme references (accent1, accent2, lt2, dk1, etc.)",
				},
			},
		}}
	}

	return nil
}

// extractFillColor parses a fill JSON value (string or object) and returns the color string.
func extractFillColor(raw json.RawMessage) string {
	// Try string form.
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// Try object form {"color": "..."}.
	var obj struct {
		Color string `json:"color"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Color
	}
	return ""
}
