package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// PatternInput is the JSON schema for pattern-based slides.
// Defined in internal/deckinput; aliased here so package-main call sites are unchanged.
type PatternInput = deckinput.PatternInput

// expandPattern looks up the named pattern in the registry, unmarshals the
// typed Values/Overrides/CellOverrides, validates, and expands to a
// ShapeGridInput. Returns the expanded grid, any warnings, and an error.
func expandPattern(p *PatternInput, ctx patterns.ExpandContext, reg *patterns.Registry) (*jsonschema.ShapeGridInput, []string, error) {
	pat, ok := reg.Get(p.Name)
	if !ok {
		return nil, nil, unknownPatternInputError(reg, p.Name)
	}
	mode, cleanOverrides, _ := patterns.SplitTypeScaleOverride(p.Name, p.Overrides)
	if mode == "" {
		mode = p.DefaultTypeScale
	}

	// Inspect the raw payload against the pattern's own decoder BEFORE anything
	// else reads it (go-slide-creator-20jm). Two failures are invisible further
	// down: a shape encoding/json rejects (whose error names Go types), and a key
	// the decoder silently discards — {"columns": …} on comparison-2col reported
	// "rows must contain at least 1 row" and never mentioned columns. Both become
	// per-field findings here, so refusing names the field and the edit.
	if inputErrs := patterns.InspectPatternInput(pat, p.Values, p.Overrides, p.CellOverrides); len(inputErrs) > 0 {
		return nil, nil, newPatternInputError(p.Name, rootPatternFindingPaths(inputErrs))
	}

	// Unmarshal values
	values := pat.NewValues()
	if err := json.Unmarshal(p.Values, values); err != nil {
		return nil, nil, fmt.Errorf("pattern %q: invalid values: %w", p.Name, err)
	}

	// Unmarshal overrides
	var overrides any
	if len(cleanOverrides) > 0 {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal(cleanOverrides, overrides); err != nil {
				return nil, nil, fmt.Errorf("pattern %q: invalid overrides: %w", p.Name, err)
			}
		}
	}

	// Unmarshal cell_overrides: string keys → int keys
	var cellOverrides map[int]any
	if len(p.CellOverrides) > 0 {
		cellOverrides = make(map[int]any, len(p.CellOverrides))
		for key, raw := range p.CellOverrides {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return nil, nil, fmt.Errorf("pattern %q: cell_overrides key %q is not an integer", p.Name, key)
			}
			co := pat.NewCellOverride()
			if co == nil {
				return nil, nil, fmt.Errorf("pattern %q: does not support cell_overrides", p.Name)
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return nil, nil, fmt.Errorf("pattern %q: invalid cell_overrides[%d]: %w", p.Name, idx, err)
			}
			cellOverrides[idx] = co
		}
	}

	// Validate. The pattern's own findings are already *patterns.ValidationError
	// with a path; they are rewrapped rather than %w-formatted so the per-field
	// structure survives into the response instead of being newline-joined into
	// one message (go-slide-creator-20jm).
	if err := pat.Validate(values, overrides, cellOverrides); err != nil {
		if ves := patternValidationFindings(err); len(ves) > 0 {
			return nil, nil, newPatternInputError(p.Name, rootPatternFindingPaths(ves))
		}
		return nil, nil, fmt.Errorf("pattern %q: validation failed: %w", p.Name, err)
	}

	// Pre-expand callout support check (D18): fail before Expand if pattern
	// does not support callout — this keeps validate and expand parity (0kyd).
	if p.Callout != nil {
		cs, ok := pat.(patterns.CalloutSupport)
		if !ok || !cs.SupportsCallout() {
			return nil, nil, patterns.ErrCalloutUnsupportedFor(p.Name, reg.CalloutSupportedPatterns())
		}
	}

	// Size content against the rectangle it will actually render into. Author
	// bounds used to replace only grid.Bounds after expansion, so card heights
	// and text fit had already been chosen from the full content area.
	expandCtx := patternAccentContext(ctx, p)
	if b, relative := resolvePatternBounds(p); b != nil {
		expandCtx.LayoutBounds = patternExpansionBounds(ctx, b, relative)
	}

	// Expand
	grid, err := pat.Expand(expandCtx, values, overrides, cellOverrides)
	if err != nil {
		return nil, nil, fmt.Errorf("pattern %q: expand failed: %w", p.Name, err)
	}
	if grid.Bounds != nil {
		grid.BoundsRelativeToContentArea = true
	}
	// Patterns place their content-sized block in the middle of the content
	// area by default (go-slide-creator-7km8): rows capped by max_height and
	// height-capped pattern bounds are centred instead of stretching or
	// leaving the lower half of the slide empty. A pattern may opt out by
	// setting vertical_align itself ("top" / "stretch").
	patterns.ApplyGridDefaults(grid)

	// Apply bounds_override: explicit bounds or max_height_pct convenience alias.
	// This constrains the grid to a sub-region of the layout area, which also
	// corrects density math (cell_budgets uses grid.Bounds when present).
	// A user-positioned block is kept where the user put it: max_height_pct
	// stays top-anchored and rows fill the user's bounds (stretch). The shapegrid
	// stretch constant is the empty string, so vertical_align is omitted in JSON.
	if b, relativeToContentArea := resolvePatternBounds(p); b != nil {
		grid.Bounds = b
		grid.BoundsRelativeToContentArea = relativeToContentArea
		grid.VerticalAlign = string(shapegrid.VAlignStretch)
	}

	// Post-expand callout decorator (D18): append full-width callout row
	if p.Callout != nil {
		grid = appendCalloutRow(grid, p.Callout)
	}
	stampPatternTypeScale(grid, mode)

	// Optional PostExpandWarner interface: patterns can surface structured
	// warning strings (e.g. CHART_PLACEHOLDER_EMPTY) describing known-degraded
	// states after a successful expansion. Downstream fit-report consumers
	// parse the leading "<CODE>: " prefix into FitFindings.
	warnings := collectExpansionWarnings(pat, expandCtx, p, values, overrides)

	// Stamp the grid with its provenance. It is what lets an agent feed an
	// expansion straight back into generate_presentation: the expanders emit
	// explicit font sizes, which constrained design mode refuses from an
	// author, so without the stamp expand_pattern output was preview-only and
	// the documented expand -> inspect -> tweak -> generate loop dead-ended on
	// violations the agent had no way to remove (go-slide-creator-c3po).
	if grid != nil {
		grid.Source = patternSourcePrefix + pat.Name()
	}

	slog.Info("pattern expanded",
		slog.String("pattern", p.Name),
		slog.Int("version", pat.Version()),
	)

	return grid, warnings, nil
}

// Stamp a pattern's resolved policy on each shape as well as the grid. Compose
// flattens segment grids and can discard their grid-level fields; per-shape
// stamps preserve different overrides on adjacent segments.
func stampPatternTypeScale(grid *jsonschema.ShapeGridInput, mode string) {
	if grid == nil || mode == "" {
		return
	}
	grid.TypeScale = mode
	for i := range grid.Rows {
		for _, cell := range grid.Rows[i].Cells {
			if cell == nil {
				continue
			}
			if cell.Shape != nil {
				cell.Shape.TypeScale = mode
			}
			if cell.Composite != nil && cell.Composite.Text != nil {
				cell.Composite.Text.TypeScale = mode
			}
			stampPatternTypeScale(cell.Grid, mode)
		}
	}
}

func patternAccentContext(ctx patterns.ExpandContext, p *PatternInput) patterns.ExpandContext {
	var authored struct {
		Accent         string `json:"accent"`
		SemanticAccent string `json:"semantic_accent"`
	}
	_ = json.Unmarshal(p.Overrides, &authored)
	semanticResolved := false
	if ctx.Metadata != nil {
		semanticResolved = ctx.Metadata.SemanticAccents[authored.SemanticAccent] != ""
	}
	if !semanticResolved {
		semanticResolved = ctx.Theme.SemanticAccents[authored.SemanticAccent] != ""
	}
	ctx.AutoAccent = authored.Accent == "" && !semanticResolved
	if ctx.AccentStrategy != patterns.AccentStrategyRotate {
		return ctx
	}
	// Canonical JSON is stable across whitespace and object-key order, unlike
	// deck position. Repeated identical patterns intentionally share a colour.
	var canonical any
	if err := json.Unmarshal(p.Values, &canonical); err == nil {
		if data, err := json.Marshal(canonical); err == nil {
			ctx.RotationKey = p.Name + ":" + string(data)
		}
	}
	return ctx
}

func collectExpansionWarnings(pat patterns.Pattern, ctx patterns.ExpandContext, p *PatternInput, values, overrides any) []string {
	var warnings []string
	if warner, ok := pat.(patterns.PostExpandWarner); ok {
		warnings = warner.PostExpandWarnings(postExpandWarningContext(ctx, p), values, overrides)
	}
	if ctx.AccentStrategy == patterns.AccentStrategyRotate && ctx.AutoAccent {
		if warning := ctx.RotationWarning(); warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func patternExpansionBounds(ctx patterns.ExpandContext, pct *GridBoundsInput, relativeToContent bool) patterns.LayoutBounds {
	content := &pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	if content.CX <= 0 || content.CY <= 0 {
		content = nil
	}
	grid := &ShapeGridInput{Bounds: pct, BoundsRelativeToContentArea: relativeToContent}
	bounds := resolveGridBounds(grid, content, ctx.ContentZone, ctx.SlideWidth, ctx.SlideHeight)
	return patterns.LayoutBounds{X: bounds.X, Y: bounds.Y, Width: bounds.CX, Height: bounds.CY}
}

// appendCalloutRow appends a full-width callout row to the expanded grid.
// The callout spans all columns and uses AutoHeight for text-driven sizing.
// Callout cells are NOT addressable via cell_overrides (D18).
func appendCalloutRow(grid *jsonschema.ShapeGridInput, callout *patterns.PatternCallout) *jsonschema.ShapeGridInput {
	// Determine column count from the grid
	numCols := 1
	if len(grid.Rows) > 0 {
		numCols = len(grid.Rows[0].Cells)
	}

	accent := "accent1"
	if callout.Accent != "" {
		accent = callout.Accent
	}

	bold := callout.Emphasis == "bold" || callout.Emphasis == "bold-italic"
	italic := callout.Emphasis == "italic" || callout.Emphasis == "bold-italic"

	textContent := buildCalloutTextContent(callout.Text, 14.0, bold, italic, "lt1", "ctr")

	calloutCell := &jsonschema.GridCellInput{
		ColSpan: numCols,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     jsonStringRaw(accent),
			Text:     textContent,
		},
	}

	calloutRow := jsonschema.GridRowInput{
		AutoHeight: true,
		Cells:      []*jsonschema.GridCellInput{calloutCell},
	}

	grid.Rows = append(grid.Rows, calloutRow)
	return grid
}

// buildCalloutTextContent creates a JSON text object for a callout cell.
func buildCalloutTextContent(content string, size float64, bold, italic bool, color, align string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Italic  bool    `json:"italic,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: bold, Italic: italic, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

// expandNestedCellPatterns walks every cell of grid (and any nested grids)
// and expands a cell-level Pattern into the cell's Grid field. After this
// pass, no cell still has a Pattern set; each formerly-pattern cell is
// represented as a Grid (a ShapeGridInput produced by pattern expansion).
//
// Mutual exclusion: a cell may not set both Pattern and Grid; that is rejected
// as a validation error. A cell that sets Pattern alongside Shape/Table/Icon/
// Image/Diagram/Composite is also rejected — a nested pattern occupies the
// whole cell rectangle and is incompatible with sibling content.
func expandNestedCellPatterns(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, reg *patterns.Registry) error {
	b := pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	if b.CX <= 0 || b.CY <= 0 {
		b = shapegrid.DefaultBounds(ctx.SlideWidth, ctx.SlideHeight)
	}
	return expandNestedCellPatternsInBounds(grid, ctx, b, reg)
}

// expandNestedCellPatternsInBounds gives each nested pattern its own cell's
// usable rectangle, matching the inset rectangle used by renderNestedSubGrids.
func expandNestedCellPatternsInBounds(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, parentBounds pptx.RectEmu, reg *patterns.Registry) error {
	if !hasNestedCellPattern(grid) {
		return nil
	}
	cellBounds, err := nestedPatternCellBounds(grid, ctx, parentBounds)
	if err != nil {
		return err
	}
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil {
				continue
			}
			if len(cell.Pattern) > 0 {
				if err := expandPatternInCell(cell, ctx, cellBounds[[2]int{ri, ci}], ri, ci, grid.TypeScale, reg); err != nil {
					return err
				}
			}
			if cell.Grid != nil {
				if err := expandNestedCellPatternsInBounds(cell.Grid, ctx, cellBounds[[2]int{ri, ci}], reg); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func expandPatternInCell(cell *jsonschema.GridCellInput, ctx patterns.ExpandContext, bounds pptx.RectEmu, ri, ci int, defaultTypeScale string, reg *patterns.Registry) error {
	if cell.Grid != nil {
		return fmt.Errorf("grid cell row %d col %d: 'pattern' and 'grid' are mutually exclusive", ri, ci)
	}
	if cell.Shape != nil || cell.Table != nil || cell.Icon != nil ||
		cell.Image != nil || cell.Diagram != nil || cell.Composite != nil {
		return fmt.Errorf("grid cell row %d col %d: nested 'pattern' is incompatible with sibling cell content (shape/table/icon/image/diagram/composite)", ri, ci)
	}
	var pi PatternInput
	if err := json.Unmarshal(cell.Pattern, &pi); err != nil {
		return fmt.Errorf("grid cell row %d col %d: invalid pattern: %w", ri, ci, err)
	}
	pi.DefaultTypeScale = defaultTypeScale
	ctx.LayoutBounds = patterns.LayoutBounds{X: bounds.X, Y: bounds.Y, Width: bounds.CX, Height: bounds.CY}
	expanded, _, err := expandPattern(&pi, ctx, reg)
	if err != nil {
		return fmt.Errorf("grid cell row %d col %d: %w", ri, ci,
			prefixPatternFindingPaths(err, fmt.Sprintf("rows[%d].cells[%d].pattern.", ri, ci)))
	}
	cell.Pattern = nil
	cell.Grid = expanded
	return nil
}

func nestedPatternCellBounds(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, parentBounds pptx.RectEmu) (map[[2]int]pptx.RectEmu, error) {
	bounds := resolveGridBounds(grid, &parentBounds, nil, ctx.SlideWidth, ctx.SlideHeight)
	cols, err := resolveColumnsDTO(grid.Columns, grid.Rows)
	if err != nil {
		return nil, err
	}
	rows := convertGridRows(grid.Rows)
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell != nil && len(cell.Pattern) > 0 {
				rows[ri].Cells[ci] = shapegrid.Cell{ColSpan: cell.ColSpan, RowSpan: cell.RowSpan, Group: cell.Group, Placeholder: true}
			}
		}
	}
	colGap, rowGap := grid.ColGap, grid.RowGap
	if colGap == 0 {
		colGap = grid.Gap
	}
	if rowGap == 0 {
		rowGap = grid.Gap
	}
	align, ok := shapegrid.ParseVerticalAlign(grid.VerticalAlign)
	if !ok {
		return nil, fmt.Errorf("shape_grid: invalid vertical_align %q", grid.VerticalAlign)
	}
	resolved, err := shapegrid.Resolve(&shapegrid.Grid{Bounds: bounds, Columns: cols, Rows: rows, ColGap: colGap, RowGap: rowGap, VAlign: align}, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		return nil, err
	}
	cellBounds := make(map[[2]int]pptx.RectEmu)
	for _, cell := range resolved.Cells {
		if cell.Kind == shapegrid.CellKindSubGrid {
			b := cell.Bounds
			inset := pptx.RectEmu{X: b.X + subGridInsetEMU, Y: b.Y + subGridInsetEMU, CX: b.CX - 2*subGridInsetEMU, CY: b.CY - 2*subGridInsetEMU}
			if inset.CX > 0 && inset.CY > 0 {
				b = inset
			}
			cellBounds[[2]int{cell.RowIdx, cell.ColIdx}] = b
		}
	}
	return cellBounds, nil
}

func hasNestedCellPattern(grid *jsonschema.ShapeGridInput) bool {
	if grid == nil {
		return false
	}
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil && (len(cell.Pattern) > 0 || hasNestedCellPattern(cell.Grid)) {
				return true
			}
		}
	}
	return false
}

// resolvePatternBounds returns a GridBoundsInput from PatternInput's bounds
// fields, applying the max_height_pct convenience alias. Returns nil if no
// bounds override was specified.
func resolvePatternBounds(p *PatternInput) (*jsonschema.GridBoundsInput, bool) {
	if p.Bounds != nil {
		// Explicit bounds take priority — use as-is.
		return p.Bounds, false
	}
	if p.MaxHeightPct > 0 && p.MaxHeightPct < 100 {
		// Convenience alias: constrain height while preserving full width.
		// X/Y/Width default to the layout content area (0,0 = top-left of layout).
		return &jsonschema.GridBoundsInput{
			X:      0,
			Y:      0,
			Width:  100,
			Height: p.MaxHeightPct,
		}, true
	}
	return nil, false
}
