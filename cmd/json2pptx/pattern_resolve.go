package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
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
	// The block is read by the one decoder every surface uses — the pattern
	// lookup, the inspection of the raw payload against the pattern's own
	// decoder, the typed unmarshal, the pattern's Validate and the callout
	// check (deckinput.DecodePattern, go-slide-creator-iqknz).
	decoded, err := deckinput.DecodePattern(p, reg)
	if err != nil {
		return nil, nil, patternBlockError(err)
	}
	pat, mode := decoded.Pattern, decoded.TypeScale
	values, overrides, cellOverrides := decoded.Values, decoded.Overrides, decoded.CellOverrides

	// Size content against the rectangle it will actually render into. Author
	// bounds used to replace only grid.Bounds after expansion, so card heights
	// and text fit had already been chosen from the full content area.
	expandCtx := patternAccentContext(ctx, p)
	if b, relative := resolvePatternBounds(p); b != nil {
		expandCtx.LayoutBounds = patternExpansionBounds(ctx, b, relative)
	}

	// Expand
	grid, err := pat.Expand(reserveCalloutBand(expandCtx, p.Callout), values, overrides, cellOverrides)
	if err != nil {
		return nil, nil, patternExpandError(p.Name, err)
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
	applyPatternVerticalAlign(p, grid)
	// Peer cards on light accents become a tint + accent rule instead of a
	// wall of solid colour; runs before the ink fix so the text darkens.
	patterns.SoftenPeerFills(expandCtx, p.Name, grid)
	// Keep the template's accents: swap unreadable pattern lt1 text for a
	// readable theme ink instead of recolouring fills black.
	patterns.ApplyReadableInk(expandCtx, grid)

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
		grid = appendCalloutRow(grid, p.Callout, expandCtx)
	}
	stampPatternTypeScale(grid, mode)
	stampPatternMeasureFonts(grid, jsonschema.MeasureFonts{Major: expandCtx.Theme.TitleFont, Minor: expandCtx.Theme.BodyFont})

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
		stampPatternSource(grid, patternSourcePrefix+pat.Name(), 0)
	}

	slog.Info("pattern expanded",
		slog.String("pattern", p.Name),
		slog.Int("version", pat.Version()),
	)

	return grid, warnings, nil
}

// patternExpandError wraps a failed Pattern.Expand. A pattern that measures
// its content against its area and refuses it (a kpi-Nup row in a region too
// short for a value over its caption, go-slide-creator-uj9zq) reports a
// located finding like Validate does, so it reaches the caller with the
// pattern's path instead of as prose.
func patternExpandError(name string, err error) error {
	if ves := patternValidationFindings(err); len(ves) > 0 {
		return newPatternInputError(name, rootPatternFindingPaths(ves))
	}
	return cliPatternError("pattern %q: expand failed: %w", name, err)
}

// Stamp a pattern's resolved policy on each shape as well as the grid. Compose
// flattens segment grids and can discard their grid-level fields; per-shape
// stamps preserve different overrides on adjacent segments. A shape the pattern
// itself pinned (peer labels kept at one size, go-slide-creator-n83ml) keeps
// its own policy.
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
			if cell.Shape != nil && cell.Shape.TypeScale == "" {
				cell.Shape.TypeScale = mode
			}
			for _, layer := range cell.Layers {
				if layer.Shape != nil && layer.Shape.TypeScale == "" {
					layer.Shape.TypeScale = mode
				}
			}
			if cell.Composite != nil && cell.Composite.Text != nil {
				cell.Composite.Text.TypeScale = mode
			}
			stampPatternTypeScale(cell.Grid, mode)
		}
	}
}

// stampPatternMeasureFonts records on every shape of an expanded grid the
// theme typefaces the pattern sized its text in, so the writer measures the
// stored autofit shrink in the same face (go-slide-creator-ohhb2). Per shape,
// like the type-scale stamp, because compose flattens segment grids. A shape
// already stamped (a nested cell pattern) keeps its own.
func stampPatternMeasureFonts(grid *jsonschema.ShapeGridInput, fonts jsonschema.MeasureFonts) {
	if grid == nil || fonts == (jsonschema.MeasureFonts{}) {
		return
	}
	for i := range grid.Rows {
		for _, cell := range grid.Rows[i].Cells {
			if cell == nil {
				continue
			}
			if cell.Shape != nil && cell.Shape.MeasureFonts == (jsonschema.MeasureFonts{}) {
				cell.Shape.MeasureFonts = fonts
			}
			for _, layer := range cell.Layers {
				if layer.Shape != nil && layer.Shape.MeasureFonts == (jsonschema.MeasureFonts{}) {
					layer.Shape.MeasureFonts = fonts
				}
			}
			stampPatternMeasureFonts(cell.Grid, fonts)
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
	// deck position. Repeated identical patterns intentionally share a colour,
	// and sibling families (every kpi-*) share one key so they never rotate
	// apart.
	if key, ok := patterns.SharedRotationKey(p.Name); ok {
		ctx.RotationKey = key
		return ctx
	}
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

// appendCalloutRow appends the callout as the shared takeaway band
// (patterns.TakeawayRows): the full-width dark neutral band with 14pt bold
// text in the ink measured against it, 16pt of air above — the look of every
// pattern-owned so-what (go-slide-creator-3a1rm, -fmmec). emphasis "bar" /
// "subtle" / "strong" pick the flush 3pt accent bar variants. It used to be a
// solid accent1 strip with white centred text — one of four different boxes
// the engine drew for the same job (go-slide-creator-7b5o6).
// Callout cells are NOT addressable via cell_overrides (D18).
func appendCalloutRow(grid *jsonschema.ShapeGridInput, callout *patterns.PatternCallout, ctx patterns.ExpandContext) *jsonschema.ShapeGridInput {
	// The band spans every column of the host grid. Counting the first row's
	// cells stopped it short wherever that row spans columns (a compose
	// envelope, driver-tree): the bar's text wrapped early there and a filled
	// band would end mid-slide.
	numCols := inferColumnCount(grid)
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}
	if rowGap == 0 {
		rowGap = 8 // shapegrid default
	}
	grid.Rows = append(grid.Rows, patterns.TakeawayRows(ctx, calloutTakeawaySpec(callout), numCols, calloutBandWidthPt(ctx), rowGap)...)
	return grid
}

// reserveCalloutBand returns ctx with the callout band's height taken out of
// the layout bounds. The band is appended after expansion at its measured
// height and does not shrink, so the pattern sizes its own rows for what is
// left. Unknown bounds (and no callout) leave ctx unchanged.
func reserveCalloutBand(ctx patterns.ExpandContext, callout *patterns.PatternCallout) patterns.ExpandContext {
	if callout == nil || ctx.LayoutBounds.Height <= 0 {
		return ctx
	}
	const hostRowGapPt = 8 // the shapegrid default; the pattern sets its own after expansion
	bandPt := patterns.TakeawayRowHeightPt(ctx, calloutTakeawaySpec(callout), calloutBandWidthPt(ctx), hostRowGapPt)
	reserve := int64((bandPt + hostRowGapPt) * 12700)
	ctx.LayoutBounds.Height = max(ctx.LayoutBounds.Height-reserve, ctx.LayoutBounds.Height/2)
	return ctx
}

// calloutTakeawaySpec maps the callout DTO onto the takeaway band: the dark
// neutral band by default, like the pattern-owned takeaways
// (go-slide-creator-fmmec). The band is always bold; "italic" / "bold-italic"
// set it italic, and "bar" / "subtle" / "strong" pick the accent bar, its
// tinted variant or the solid-accent band.
func calloutTakeawaySpec(callout *patterns.PatternCallout) patterns.TakeawaySpec {
	spec := patterns.TakeawaySpec{Text: callout.Text, Accent: callout.Accent}
	switch callout.Emphasis {
	case "italic", "bold-italic":
		spec.Italic = true
	case patterns.TakeawayEmphasisBar, patterns.TakeawayEmphasisSubtle, patterns.TakeawayEmphasisStrong:
		spec.Emphasis = callout.Emphasis
	}
	return spec
}

// calloutBandWidthPt is the width the callout band wraps at: the pattern's
// layout bounds, else the shape-grid default for the slide.
func calloutBandWidthPt(ctx patterns.ExpandContext) float64 {
	widthEMU := ctx.LayoutBounds.Width
	if widthEMU <= 0 {
		widthEMU = shapegrid.DefaultBounds(ctx.SlideWidth, ctx.SlideHeight).CX
	}
	return float64(widthEMU) / 12700
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
	return expandNestedCellPatternsInBounds(grid, ctx, b, reg, true)
}

// expandNestedCellPatternsInBounds gives each nested pattern its own cell's
// usable rectangle, matching the inset rectangle used by renderNestedSubGrids.
//
// slideBlock says grid is the slide's own block, which generation resolves
// under the composition policy (shapegrid.Grid.Compose): a sparse block's rows
// are grown with its type step, so the cells a nested pattern is expanded for
// are the ones it renders in (go-slide-creator-yhzxt). Grids nested in a cell
// are not composed.
func expandNestedCellPatternsInBounds(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, parentBounds pptx.RectEmu, reg *patterns.Registry, slideBlock bool) error {
	if !hasNestedCellPattern(grid) {
		return nil
	}
	rebalanceRegionsStack(grid, ctx, parentBounds)
	cellBounds, err := nestedPatternCellBounds(grid, ctx, parentBounds, slideBlock)
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
				if err := expandNestedCellPatternsInBounds(cell.Grid, ctx, cellBounds[[2]int{ri, ci}], reg, false); err != nil {
					// Keep the outer cell's coordinates on a finding raised
					// deeper down (a region's pattern sits under its heading
					// row's sub-grid).
					return prefixPatternFindingPaths(err, fmt.Sprintf("rows[%d].cells[%d].grid.", ri, ci))
				}
			}
		}
	}
	return nil
}

func expandPatternInCell(cell *jsonschema.GridCellInput, ctx patterns.ExpandContext, bounds pptx.RectEmu, ri, ci int, defaultTypeScale string, reg *patterns.Registry) error {
	if cell.Grid != nil {
		return cliPatternError("grid cell row %d col %d: 'pattern' and 'grid' are mutually exclusive", ri, ci)
	}
	if cell.Shape != nil || cell.Table != nil || cell.Icon != nil ||
		cell.Image != nil || cell.Diagram != nil || cell.Composite != nil || len(cell.Layers) > 0 {
		return cliPatternError("grid cell row %d col %d: nested 'pattern' is incompatible with sibling cell content (shape/table/icon/image/diagram/composite/layers)", ri, ci)
	}
	var pi PatternInput
	if err := json.Unmarshal(cell.Pattern, &pi); err != nil {
		return cliPatternError("grid cell row %d col %d: invalid pattern: %w", ri, ci, err)
	}
	pi.DefaultTypeScale = defaultTypeScale
	ctx.LayoutBounds = patterns.LayoutBounds{X: bounds.X, Y: bounds.Y, Width: bounds.CX, Height: bounds.CY}
	expanded, _, err := expandPattern(&pi, ctx, reg)
	if err != nil {
		return cliPatternError("grid cell row %d col %d: %w", ri, ci,
			prefixPatternFindingPaths(err, fmt.Sprintf("rows[%d].cells[%d].pattern.", ri, ci)))
	}
	cell.Pattern = nil
	cell.Grid = expanded
	return nil
}

func nestedPatternCellBounds(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, parentBounds pptx.RectEmu, slideBlock bool) (map[[2]int]pptx.RectEmu, error) {
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
		return nil, cliCoded(diagnostics.CodeInvalidGrid, "shape_grid: invalid vertical_align %q", grid.VerticalAlign)
	}
	resolved, err := shapegrid.Resolve(&shapegrid.Grid{
		Bounds: bounds, TypeScale: grid.TypeScale, Columns: cols, Rows: rows, ColGap: colGap, RowGap: rowGap, VAlign: align,
		Compose: slideBlock && composesSlideBlock(grid), ComposeGrow: slideBlock && growsLoneRow(grid), ComposeBand: slideBlock && scalesToBand(grid), ComposeBandSquare: slideBlock && scalesToBand(grid) && bandScalesToSquare(grid), ComposeZoom: slideBlock && zoomsSlideBlock(grid), KeepTextSizes: grid.KeepTextSizes,
		CanvasScale: gridCanvasScale(grid, ctx.SlideWidth, ctx.SlideHeight),
	}, pptx.NewShapeIDAllocator(nil))
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

// patternBlockError is the decoder's refusal of a pattern block
// (*deckinput.PatternError) as this package reports it: a fault of the block
// as a whole is a PATTERN_ERROR located at the block by whoever knows where
// the block is; per-field findings travel as a patternInputError, one finding
// per field.
func patternBlockError(err error) error {
	var pe *deckinput.PatternError
	if !errors.As(err, &pe) {
		return err
	}
	if msg, ok := pe.IsBlockFault(); ok {
		return cliPatternError("%s", msg)
	}
	return newPatternInputError(pe.Pattern, pe.Findings)
}

// applyPatternVerticalAlign applies a slide pattern's (already validated)
// vertical_align over the expansion default (go-slide-creator-e17xy).
func applyPatternVerticalAlign(p *PatternInput, grid *jsonschema.ShapeGridInput) {
	switch p.VerticalAlign {
	case "":
	case "stretch":
		grid.VerticalAlign = string(shapegrid.VAlignStretch)
	default:
		grid.VerticalAlign = p.VerticalAlign
	}
}

// stampPatternSource marks grid, and every sub-grid the pattern nested in its
// cells, as the engine's own expansion of the named pattern. The nested grids
// are the pattern's too: they carry its explicit sizes, and they follow the
// canvas with it (gridCanvasScale). A nested grid that already names a source
// (a pattern expanded into a cell) keeps it.
func stampPatternSource(grid *jsonschema.ShapeGridInput, source string, depth int) {
	if grid == nil || depth > maxGeomNestingDepth {
		return
	}
	if grid.Source == "" || depth == 0 {
		grid.Source = source
	}
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil && cell.Grid != nil {
				stampPatternSource(cell.Grid, source, depth+1)
			}
		}
	}
}
