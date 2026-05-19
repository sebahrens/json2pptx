package pipeline

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
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

	// maxAccentHuesPerSlide is the maximum number of distinct accent hues
	// (accent1..accent6) that may appear on a single slide before the
	// composition reads as "many colors competing" rather than "one focused
	// argument". Above this, the validator emits an `accent_overload`
	// finding. Two hues lets a slide draw a paired comparison (current
	// vs. proposed, before vs. after) without losing focus.
	maxAccentHuesPerSlide = 2
)

// accentColorPattern matches "accent1".."accent6" semantic color names,
// optionally suffixed by tint modifiers the renderer accepts (e.g.,
// `accent1` proper or just the bare scheme name).
var accentColorPattern = regexp.MustCompile(`^accent[1-6]$`)

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
	warnings = append(warnings, detectAccentOverload(grid, slideIdx)...)
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
			path := slidepath.GridRowRange(slideIdx, i, i+1)
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
			path := slidepath.GridRowRange(slideIdx, i, i+1)
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
			path := slidepath.GridRow(slideIdx, i)
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
		path := slidepath.ShapeGrid(slideIdx)
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

// detectAccentOverload flags slides whose shape_grid uses more than
// maxAccentHuesPerSlide distinct accent semantic fills (accent1..accent6).
// Multiple accent hues on one slide reads as visual noise — the audience
// cannot tell which item is the focus. The "one accent per slide" rule
// allows a second accent for paired comparisons but blocks three or more.
//
// Hex fills are ignored here; mixed hex+semantic combinations are caught
// by detectMixedFillScheme. Cells whose fill is a tint/shade object
// referencing an accent (e.g. `{"color": "accent1", "lumMod": 75000}`)
// count toward the same accent hue as the bare name.
func detectAccentOverload(grid *jsonschema.ShapeGridInput, slideIdx int) []*patterns.ValidationError {
	hues := make(map[string]struct{})
	walkAccentFills(grid, func(name string) {
		if accentColorPattern.MatchString(name) {
			hues[name] = struct{}{}
		}
	})

	if len(hues) <= maxAccentHuesPerSlide {
		return nil
	}

	names := make([]string, 0, len(hues))
	for n := range hues {
		names = append(names, n)
	}
	// Stable order so the message is deterministic for snapshot tests.
	sortStrings(names)

	path := slidepath.ShapeGrid(slideIdx)
	return []*patterns.ValidationError{{
		Pattern: "shape_grid",
		Path:    path,
		Code:    patterns.ErrCodeAccentOverload,
		Message: fmt.Sprintf(
			"slide %d: shape_grid uses %d distinct accent hues (%s); max %d — pick one base accent and use cell_accent_mode for within-slide variety",
			slideIdx+1, len(hues), strings.Join(names, ", "), maxAccentHuesPerSlide),
		Fix: &patterns.FixSuggestion{
			Kind: "consolidate_accents",
			Params: map[string]any{
				"accents_used": names,
				"max_accents":  maxAccentHuesPerSlide,
				"guidance":     "keep at most two accent hues per slide; use cell_accent_mode (alternate/progressive) for grids that need item differentiation",
			},
		},
	}}
}

// walkAccentFills calls visit for every fill color found on a shape_grid
// cell's shape. Object-form fills (with tint/shade modifiers) yield their
// base `color` value, so tinted accents count as the same hue as the bare
// scheme name.
func walkAccentFills(grid *jsonschema.ShapeGridInput, visit func(name string)) {
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Fill) == 0 {
				continue
			}
			color := extractFillColor(cell.Shape.Fill)
			if color != "" {
				visit(color)
			}
		}
	}
}

// sortStrings sorts a slice in place. Small helper to avoid importing sort
// in a hot file already crowded with imports.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// DetectTableDensity checks a single table against TDR density rules and returns
// warnings as ValidationError values. The path is the JSON path prefix for the
// table (e.g. "slides[0].content[1]").
func DetectTableDensity(table *jsonschema.TableInput, path string) []*patterns.ValidationError {
	if table == nil || len(table.Headers) == 0 {
		return nil
	}

	numCols := len(table.Headers)
	logicalRows := table.LogicalRowCount()

	var warnings []*patterns.ValidationError

	if logicalRows > jsonschema.TDRMaxRows {
		warnings = append(warnings, &patterns.ValidationError{
			Pattern: "table",
			Path:    path,
			Code:    patterns.ErrCodeDensityExceeded,
			Message: fmt.Sprintf("table has %d logical rows (max %d); consider splitting across slides", logicalRows, jsonschema.TDRMaxRows),
			Fix: &patterns.FixSuggestion{
				Kind:   "split_at_row",
				Params: map[string]any{"row": len(table.Rows) / 2, "logical_rows": logicalRows, "max_rows": jsonschema.TDRMaxRows},
			},
		})
	}

	if numCols > jsonschema.TDRMaxCols {
		warnings = append(warnings, &patterns.ValidationError{
			Pattern: "table",
			Path:    path,
			Code:    patterns.ErrCodeDensityExceeded,
			Message: fmt.Sprintf("table has %d columns (max %d); consider removing columns or splitting", numCols, jsonschema.TDRMaxCols),
			Fix: &patterns.FixSuggestion{
				Kind:   "reduce_columns",
				Params: map[string]any{"columns": numCols, "max_columns": jsonschema.TDRMaxCols},
			},
		})
	}

	return warnings
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
