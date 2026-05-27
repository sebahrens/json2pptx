package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
	"github.com/sebahrens/json2pptx/svggen/icons"
)

// ShapeGridResult holds the output of resolveShapeGrid: both the raw XML
// fragments ready for injection and the resolved cell metadata (bounds, IDs,
// specs) for downstream processing such as icon insertion or validation.
type ShapeGridResult struct {
	Shapes       [][]byte                 // Raw <p:sp>/<p:graphicFrame> XML fragments
	Cells        []shapegrid.ResolvedCell // Resolved cell metadata with absolute coordinates
	IconInserts  []generator.IconInsert   // Icon cells requiring media registration in the generator
	ImageInserts []generator.ImageInsert  // Image cells requiring media registration in the generator
	RowOverflows []shapegrid.RowOverflow  // Rows whose content exceeded max_height
	Warnings     []string                 // Quality warnings (e.g. complex diagram in narrow cell)
	FitFindings  []patterns.FitFinding    // Structured findings for visual grid cells (diagram, icon, image)
}

// GridDiagramContext provides template-level context for rendering diagram cells
// in a shape_grid. Without this context, grid diagrams render with default colors
// instead of inheriting the template's theme palette.
type GridDiagramContext struct {
	ThemeColors []types.ThemeColor // Template theme colors for chart styling
	DataPalette []string           // Ordered hex palette for chart series (from TemplateMetadata)
	FontFamily  string             // Template body font, injected when a diagram omits style.font_family
	SlideNum    int                // 1-based slide number for warning messages
}

// virtualLayoutResult holds the result of virtual layout resolution.
type virtualLayoutResult struct {
	LayoutID string                 // Selected base layout ID
	Bounds   pptx.RectEmu           // Computed grid bounds from placeholder metadata
	Zone     *shapegrid.ContentZone // Template-derived safe content area (nil if unavailable)
}

// resolveVirtualLayout selects a base layout for shape_grid slides and computes
// grid bounds from the layout's placeholder metadata.
//
// Selection priority:
//  1. Canonical Blank layout with a title placeholder
//  2. Canonical Blank+Title layout (synthesized)
//  3. Any layout with a body/content placeholder (bounds = body placeholder)
//
// Selection is driven by the canonical layout type assigned during template
// parsing (types.CanonicalLayoutType), so generation and preflight share one
// source of truth instead of matching ad-hoc classification tags.
//
// Returns nil if no suitable layout is found.
func resolveVirtualLayout(layouts []types.LayoutMetadata, slideWidth, slideHeight int64) *virtualLayoutResult {
	// Priority 1 & 2: find blank or blank-title layout with title placeholder
	var blankLayout, blankTitleLayout *types.LayoutMetadata
	for i := range layouts {
		switch template.EffectiveCanonicalType(&layouts[i]) {
		case types.CanonicalLayoutBlankTitle:
			blankTitleLayout = &layouts[i]
		case types.CanonicalLayoutBlank:
			// Check if it has a title placeholder
			for _, ph := range layouts[i].Placeholders {
				if ph.Type == types.PlaceholderTitle {
					blankLayout = &layouts[i]
					break
				}
			}
		}
	}

	// Try blank with title first, then blank-title
	if chosen := pickBlankLayout(blankLayout, blankTitleLayout, slideWidth, slideHeight); chosen != nil {
		return chosen
	}

	// Priority 3: any layout with a body/content placeholder. Always derive a
	// ContentZone from that layout's title/footer placeholders so downstream
	// clamping (ClampBoundsToZone / DefaultBoundsFromZone) protects title and
	// footer chrome. Without a Zone here, callers fall back to generic
	// full-slide DefaultBounds and content overlaps the title/footer
	// (go-slide-creator-ihmo).
	for i := range layouts {
		content, ok := firstBodyOrContentBounds(&layouts[i])
		if !ok {
			continue
		}
		zone := fallbackContentZone(&layouts[i], content, slideWidth, slideHeight)
		return &virtualLayoutResult{
			LayoutID: layouts[i].ID,
			Bounds:   shapegrid.BoundsFromPlaceholder(content),
			Zone:     &zone,
		}
	}

	return nil
}

// firstBodyOrContentBounds returns the bounds of the first body or content
// placeholder in layout (in placeholder order). The bool is false when the
// layout has neither.
func firstBodyOrContentBounds(layout *types.LayoutMetadata) (pptx.RectEmu, bool) {
	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
			return pptx.RectEmu{
				X:  ph.Bounds.X,
				Y:  ph.Bounds.Y,
				CX: ph.Bounds.Width,
				CY: ph.Bounds.Height,
			}, true
		}
	}
	return pptx.RectEmu{}, false
}

// fallbackContentZone derives a ContentZone for the priority-3 body/content
// fallback layout. TitleBottom comes from the layout's title placeholder
// (falling back to the content top), FooterTop from the first utility/footer
// placeholder (falling back to a minimum bottom margin), and the horizontal
// extent from the content placeholder. This guarantees resolveVirtualLayout
// never returns a result with a nil Zone, so chrome-protection clamping runs
// on every shape_grid slide.
func fallbackContentZone(layout *types.LayoutMetadata, content pptx.RectEmu, slideWidth, slideHeight int64) shapegrid.ContentZone {
	sw := slideWidth
	if sw <= 0 {
		sw = shapegrid.DefaultSlideWidthEMU
	}
	sh := slideHeight
	if sh <= 0 {
		sh = shapegrid.DefaultSlideHeightEMU
	}

	titleBottom := content.Y
	footerTop := sh - shapegrid.MinBottomMarginEMU
	hasFooter := false
	for _, ph := range layout.Placeholders {
		switch ph.Type {
		case types.PlaceholderTitle:
			titleBottom = ph.Bounds.Y + ph.Bounds.Height
		case types.PlaceholderOther:
			if !hasFooter {
				footerTop = ph.Bounds.Y
				hasFooter = true
			}
		}
	}

	return shapegrid.ContentZone{
		TitleBottom: titleBottom,
		FooterTop:   footerTop,
		LeftMargin:  content.X,
		RightEdge:   content.X + content.CX,
		SlideWidth:  sw,
		SlideHeight: sh,
	}
}

// pickBlankLayout tries the blank (with title) layout first, then blank-title.
// Both use BoundsFromTitleAndFooter with a 9pt gap.
func pickBlankLayout(blank, blankTitle *types.LayoutMetadata, slideWidth, slideHeight int64) *virtualLayoutResult {
	candidates := []*types.LayoutMetadata{blank, blankTitle}
	for _, layout := range candidates {
		if layout == nil {
			continue
		}
		var titleRect pptx.RectEmu
		var footerRect pptx.RectEmu
		hasTitle := false
		hasFooter := false

		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderTitle {
				titleRect = pptx.RectEmu{X: ph.Bounds.X, Y: ph.Bounds.Y, CX: ph.Bounds.Width, CY: ph.Bounds.Height}
				hasTitle = true
			}
			if ph.Type == types.PlaceholderOther && !hasFooter {
				// Use the first utility placeholder (footer/slide number) as bottom boundary
				footerRect = pptx.RectEmu{X: ph.Bounds.X, Y: ph.Bounds.Y, CX: ph.Bounds.Width, CY: ph.Bounds.Height}
				hasFooter = true
			}
		}

		if !hasTitle {
			continue
		}

		if !hasFooter {
			// No footer — reserve minimum bottom margin for visual clearance
			sh := slideHeight
			if sh <= 0 {
				sh = shapegrid.DefaultSlideHeightEMU
			}
			footerRect = pptx.RectEmu{
				X:  titleRect.X,
				Y:  sh - shapegrid.MinBottomMarginEMU,
				CX: titleRect.CX,
				CY: 0,
			}
		}

		const gapPt = 9.0 // standard gap between title/footer and grid area

		// Compute ContentZone from actual template geometry
		sw := slideWidth
		if sw <= 0 {
			sw = shapegrid.DefaultSlideWidthEMU
		}
		sh := slideHeight
		if sh <= 0 {
			sh = shapegrid.DefaultSlideHeightEMU
		}
		rightEdge := sw - titleRect.X // symmetric margin
		if rightEdge < titleRect.X+titleRect.CX {
			rightEdge = titleRect.X + titleRect.CX
		}
		zone := &shapegrid.ContentZone{
			TitleBottom: titleRect.Y + titleRect.CY,
			FooterTop:   footerRect.Y,
			LeftMargin:  titleRect.X,
			RightEdge:   rightEdge,
			SlideWidth:  sw,
			SlideHeight: sh,
		}

		return &virtualLayoutResult{
			LayoutID: layout.ID,
			Bounds:   shapegrid.DefaultBoundsFromZone(*zone, gapPt),
			Zone:     zone,
		}
	}
	return nil
}

// findLayoutByID returns a pointer to the layout with the given ID, or nil if
// no layout matches (or id is empty).
func findLayoutByID(layouts []types.LayoutMetadata, id string) *types.LayoutMetadata {
	if id == "" {
		return nil
	}
	for i := range layouts {
		if layouts[i].ID == id {
			return &layouts[i]
		}
	}
	return nil
}

// contentZoneFromLayout derives a ContentZone directly from a concrete layout's
// own placeholders. Used for shape_grid slides that carry an explicit (or
// auto-selected) layout_id: the safe content area (title bottom, footer top,
// horizontal margins) must come from THAT layout, not from resolveVirtualLayout's
// blank/blank-title pick, which can hand back title/footer bounds from an
// unrelated layout (go-slide-creator-j15r).
//
// When the layout exposes a body/content placeholder, the horizontal extent and
// title/footer derivation reuse fallbackContentZone (the same logic as
// resolveVirtualLayout's priority-3 path). A title-only layout falls back to
// symmetric margins around the title placeholder. Returns nil when the layout is
// nil or has neither a title nor a body/content placeholder to anchor the zone,
// so the caller can fall back to virtual resolution.
func contentZoneFromLayout(layout *types.LayoutMetadata, slideWidth, slideHeight int64) *shapegrid.ContentZone {
	if layout == nil {
		return nil
	}
	if content, ok := firstBodyOrContentBounds(layout); ok {
		zone := fallbackContentZone(layout, content, slideWidth, slideHeight)
		return &zone
	}
	return titleOnlyContentZone(layout, slideWidth, slideHeight)
}

// titleOnlyContentZone derives a ContentZone for a concrete layout that has a
// title placeholder but no body/content placeholder. The horizontal extent uses
// symmetric margins around the title; FooterTop comes from the first
// utility/footer placeholder, falling back to the minimum bottom margin. Returns
// nil when the layout has no title placeholder.
func titleOnlyContentZone(layout *types.LayoutMetadata, slideWidth, slideHeight int64) *shapegrid.ContentZone {
	sw := slideWidth
	if sw <= 0 {
		sw = shapegrid.DefaultSlideWidthEMU
	}
	sh := slideHeight
	if sh <= 0 {
		sh = shapegrid.DefaultSlideHeightEMU
	}

	var title *pptx.RectEmu
	footerTop := sh - shapegrid.MinBottomMarginEMU
	hasFooter := false
	for i := range layout.Placeholders {
		ph := &layout.Placeholders[i]
		switch ph.Type {
		case types.PlaceholderTitle:
			if title == nil {
				title = &pptx.RectEmu{X: ph.Bounds.X, Y: ph.Bounds.Y, CX: ph.Bounds.Width, CY: ph.Bounds.Height}
			}
		case types.PlaceholderOther:
			if !hasFooter {
				footerTop = ph.Bounds.Y
				hasFooter = true
			}
		}
	}
	if title == nil {
		return nil
	}

	rightEdge := sw - title.X // symmetric margin
	if rightEdge < title.X+title.CX {
		rightEdge = title.X + title.CX
	}
	return &shapegrid.ContentZone{
		TitleBottom: title.Y + title.CY,
		FooterTop:   footerTop,
		LeftMargin:  title.X,
		RightEdge:   rightEdge,
		SlideWidth:  sw,
		SlideHeight: sh,
	}
}

// needsVirtualLayout returns true if the slide should use virtual layout resolution
// (has shape_grid and either no layout_id or a blank/virtual slide type).
func needsVirtualLayout(slide SlideInput) bool {
	if slide.ShapeGrid == nil {
		return false
	}
	st := strings.ToLower(slide.SlideType)
	return slide.LayoutID == "" || st == "blank" || st == "virtual"
}

// GridGeometry is the layout-aware geometry resolved for a shape_grid slide:
// the chrome-safe content zone plus any virtual-layout bounds override. It is
// the single source of truth shared by generation (json_mode.go), preflight
// (fit_findings_collect.go), and preview (mcp_preview.go) so all three evaluate
// shape_grid cells against the SAME coordinates. Before this was shared,
// preflight resolved grid bounds via generic defaults and could not detect the
// title/footer overlap class that generation's zone clamping produces
// (go-slide-creator-s1rd).
type GridGeometry struct {
	// Zone is the template-derived safe content area (nil when no layout
	// geometry could be resolved).
	Zone *shapegrid.ContentZone
	// OverrideBounds are virtual-layout-derived grid bounds (nil for slides
	// resolved against a concrete layout; the grid then derives bounds from
	// the zone or its own explicit bounds).
	OverrideBounds *pptx.RectEmu
	// LayoutID is the base layout selected by virtual resolution (empty for
	// concrete-layout slides).
	LayoutID string
	// VirtualUsed reports whether virtual layout resolution drove the result.
	VirtualUsed bool
}

// resolveGridGeometry resolves the ContentZone and any bounds override for a
// shape_grid slide using the SAME priority rules as generation:
//
//   - Blank / virtual slides go through resolveVirtualLayout, which selects a
//     base layout, derives a chrome-safe zone, and supplies override bounds.
//   - Slides bound to a concrete layout take their zone from THAT layout's
//     placeholders (contentZoneFromLayout), falling back to the virtual zone
//     for chrome protection when the concrete layout has no usable geometry.
//
// Returns the zero value when no layouts are available (the grid then falls
// back to generic default bounds, matching resolveShapeGrid). This mirrors the
// switch previously inlined in json_mode.go so generation and preflight cannot
// drift.
func resolveGridGeometry(slide SlideInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) GridGeometry {
	var g GridGeometry
	if len(layouts) == 0 {
		return g
	}
	switch {
	case needsVirtualLayout(slide):
		if vl := resolveVirtualLayout(layouts, slideWidth, slideHeight); vl != nil {
			g.Zone = vl.Zone
			b := vl.Bounds
			g.OverrideBounds = &b
			g.LayoutID = vl.LayoutID
			g.VirtualUsed = true
		}
	default:
		g.Zone = contentZoneFromLayout(findLayoutByID(layouts, slide.LayoutID), slideWidth, slideHeight)
		if g.Zone == nil {
			// Concrete layout had no usable title/body/footer geometry — fall
			// back to the virtual zone for chrome protection.
			if vl := resolveVirtualLayout(layouts, slideWidth, slideHeight); vl != nil {
				g.Zone = vl.Zone
			}
		}
	}
	return g
}

// resolveGridBounds computes the absolute grid bounds for a ShapeGridInput
// using the precedence shared by generation and preflight:
//
//	explicit input.Bounds (clamped to zone) > overrideBounds > zone default > generic default
//
// With overrideBounds == nil and zone == nil this reduces to the legacy
// "explicit bounds or DefaultBounds" behavior, so callers that have no layout
// geometry get identical results to before.
func resolveGridBounds(input *ShapeGridInput, overrideBounds *pptx.RectEmu, zone *shapegrid.ContentZone, slideWidth, slideHeight int64) pptx.RectEmu {
	switch {
	case input.Bounds != nil:
		bounds := shapegrid.BoundsFromPercentages(input.Bounds.X, input.Bounds.Y, input.Bounds.Width, input.Bounds.Height, slideWidth, slideHeight)
		// Clamp explicit bounds against ContentZone to prevent overlapping chrome.
		if zone != nil {
			bounds = shapegrid.ClampBoundsToZone(bounds, *zone)
		}
		return bounds
	case overrideBounds != nil:
		return *overrideBounds
	case zone != nil:
		return shapegrid.DefaultBoundsFromZone(*zone, 9.0)
	default:
		return shapegrid.DefaultBounds(slideWidth, slideHeight)
	}
}

// resolveShapeGrid converts a ShapeGridInput into a ShapeGridResult containing
// both raw XML fragments and resolved cell metadata.
// If overrideBounds is non-nil, it is used instead of DefaultBounds or input.Bounds.
// If zone is non-nil, explicit input.Bounds are clamped against it to prevent content
// from overlapping title or footer chrome.
// slideWidth and slideHeight are the template's actual slide dimensions in EMU (0 = use 16:9 defaults).
// diagCtx provides template theme colors for diagram cells (nil = no theme injection).
func resolveShapeGrid(input *ShapeGridInput, alloc *pptx.ShapeIDAllocator, overrideBounds *pptx.RectEmu, zone *shapegrid.ContentZone, slideWidth, slideHeight int64, diagCtx *GridDiagramContext) (*ShapeGridResult, error) {
	if input == nil || len(input.Rows) == 0 {
		return nil, nil
	}

	// Convert JSON DTO columns to []float64
	colWidths, err := resolveColumnsDTO(input.Columns, input.Rows)
	if err != nil {
		return nil, err
	}

	// Resolve bounds: explicit input.Bounds > overrideBounds > zone default >
	// generic default. Shared with preflight/preview via resolveGridBounds so
	// all consumers evaluate against the same geometry (go-slide-creator-s1rd).
	bounds := resolveGridBounds(input, overrideBounds, zone, slideWidth, slideHeight)

	// Resolve gaps
	colGap := input.ColGap
	if colGap == 0 {
		colGap = input.Gap
	}
	rowGap := input.RowGap
	if rowGap == 0 {
		rowGap = input.Gap
	}

	// Convert DTO rows to shapegrid.Row
	rows := convertGridRows(input.Rows)

	grid := &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    rows,
		ColGap:  colGap,
		RowGap:  rowGap,
	}

	// Validate grid structure before rendering (catches overlaps, span errors, etc.)
	if vErr := shapegrid.Validate(grid); vErr != nil {
		return nil, fmt.Errorf("shape_grid validation: %w", vErr)
	}

	// Resolve grid into cells with absolute coordinates
	result, err := shapegrid.Resolve(grid, alloc)
	if err != nil {
		return nil, err
	}

	// Derive 0-based slide index from diagCtx.SlideNum (1-based).
	slideIdx := 0
	if diagCtx != nil {
		slideIdx = diagCtx.SlideNum - 1
	}

	// Generate XML fragments, icon inserts, and image inserts from resolved cells
	out, err := generateGridOutput(result, alloc, diagCtx, slideIdx)
	if err != nil {
		return nil, err
	}

	// Recursively render any nested sub-grids in this grid. Cells with
	// Placeholder=true produce CellKindSubGrid ResolvedCells whose bounds
	// define the sub-grid's render rectangle. The accompanying DTO cell
	// (input.Rows[r].Cells[c]) supplies the nested ShapeGridInput.
	if err := renderNestedSubGrids(input, out, alloc, slideWidth, slideHeight, diagCtx); err != nil {
		return nil, err
	}
	return out, nil
}

// subGridInsetEMU is the inset applied to nested sub-grid bounds so the
// nested grid does not visually butt up against the parent cell edges. Keeps
// nested patterns visually distinct from neighbouring siblings.
const subGridInsetEMU int64 = 50800 // 4pt = 4 * 12700

// renderNestedSubGrids walks resolved cells, locates CellKindSubGrid
// placeholders, and recursively resolves their accompanying sub-grids using
// the placeholder's bounds (with a small inset). Resulting shapes/icons are
// appended to out so the caller receives a single unified render result.
func renderNestedSubGrids(input *ShapeGridInput, out *ShapeGridResult, alloc *pptx.ShapeIDAllocator, slideWidth, slideHeight int64, diagCtx *GridDiagramContext) error {
	if input == nil || out == nil {
		return nil
	}
	for _, rc := range out.Cells {
		if rc.Kind != shapegrid.CellKindSubGrid {
			continue
		}
		if rc.RowIdx < 0 || rc.RowIdx >= len(input.Rows) {
			continue
		}
		row := input.Rows[rc.RowIdx]
		if rc.ColIdx < 0 || rc.ColIdx >= len(row.Cells) {
			continue
		}
		src := row.Cells[rc.ColIdx]
		if src == nil || src.Grid == nil {
			continue
		}
		inset := pptx.RectEmu{
			X:  rc.Bounds.X + subGridInsetEMU,
			Y:  rc.Bounds.Y + subGridInsetEMU,
			CX: rc.Bounds.CX - 2*subGridInsetEMU,
			CY: rc.Bounds.CY - 2*subGridInsetEMU,
		}
		if inset.CX <= 0 || inset.CY <= 0 {
			// Cell is too small for inset; fall back to raw bounds.
			inset = rc.Bounds
		}
		sub, err := resolveShapeGrid(src.Grid, alloc, &inset, nil, slideWidth, slideHeight, diagCtx)
		if err != nil {
			return fmt.Errorf("nested sub-grid at row %d col %d: %w", rc.RowIdx, rc.ColIdx, err)
		}
		if sub == nil {
			continue
		}
		out.Shapes = append(out.Shapes, sub.Shapes...)
		out.IconInserts = append(out.IconInserts, sub.IconInserts...)
		out.ImageInserts = append(out.ImageInserts, sub.ImageInserts...)
		out.RowOverflows = append(out.RowOverflows, sub.RowOverflows...)
		out.Warnings = append(out.Warnings, sub.Warnings...)
		out.FitFindings = append(out.FitFindings, sub.FitFindings...)
		// Expose nested resolved cells on the parent result so consumers
		// (e.g. overlay anchor_cell, fit_findings) can introspect them. The
		// nested cells keep their own RowIdx/ColIdx within the sub-grid; the
		// parent's CellKindSubGrid placeholder remains as the anchor point
		// for the outer cell coordinate.
		out.Cells = append(out.Cells, sub.Cells...)
	}
	return nil
}

// convertGridRows converts DTO GridRowInput slices into shapegrid.Row domain objects.
func convertGridRows(inputRows []GridRowInput) []shapegrid.Row {
	rows := make([]shapegrid.Row, len(inputRows))
	for i, r := range inputRows {
		cells := make([]shapegrid.Cell, len(r.Cells))
		for j, c := range r.Cells {
			if c == nil {
				cells[j] = shapegrid.Cell{}
				continue
			}
			// Cells hosting a nested sub-grid become bounds-only placeholders.
			// The parent resolver allocates their rectangle; the caller
			// recursively renders the sub-grid using those bounds.
			if c.Grid != nil {
				cells[j] = shapegrid.Cell{
					ColSpan:     c.ColSpan,
					RowSpan:     c.RowSpan,
					Group:       c.Group,
					Placeholder: true,
				}
				continue
			}
			if c.Shape == nil && c.Table == nil && c.Icon == nil && c.Image == nil && c.Diagram == nil && c.Composite == nil {
				cells[j] = shapegrid.Cell{}
				continue
			}
			cells[j] = convertGridCell(c)
		}
		var connSpec *shapegrid.ConnectorSpec
		if r.Connector != nil {
			connSpec = &shapegrid.ConnectorSpec{
				Style: r.Connector.Style,
				Color: r.Connector.Color,
				Width: r.Connector.Width,
				Dash:  r.Connector.Dash,
			}
		}
		rows[i] = shapegrid.Row{
			Height:     r.Height,
			AutoHeight: r.AutoHeight,
			Flex:       r.Flex,
			MinHeight:  r.MinHeight,
			MaxHeight:  r.MaxHeight,
			Cells:      cells,
			Connector:  connSpec,
		}
	}
	return rows
}

// convertGridCell converts a single GridCellInput DTO into a shapegrid.Cell.
func convertGridCell(c *GridCellInput) shapegrid.Cell {
	cell := shapegrid.Cell{
		ColSpan: c.ColSpan,
		RowSpan: c.RowSpan,
		Fit:     shapegrid.FitMode(c.Fit),
		Group:   c.Group,
	}
	if c.Shape != nil {
		cell.Shape = &shapegrid.ShapeSpec{
			Geometry:    c.Shape.Geometry,
			Fill:        c.Shape.Fill,
			Line:        c.Shape.Line,
			Text:        c.Shape.Text,
			Rotation:    c.Shape.Rotation,
			Adjustments: c.Shape.Adjustments,
		}
	}
	if c.Table != nil {
		cell.TableSpec = c.Table.ToTableSpec()
	}
	if c.Icon != nil {
		cell.Icon = &shapegrid.IconSpec{
			Name:     c.Icon.Name,
			Path:     c.Icon.Path,
			SVGData:  c.Icon.SVGData,
			Alt:      c.Icon.Alt,
			Fill:     c.Icon.Fill,
			Position: c.Icon.Position,
			Scale:    c.Icon.Scale,
		}
	}
	// Support icon nested inside shape (e.g. {"shape": {"fill": "accent1", "icon": {"name": "shield"}}})
	if c.Shape != nil && c.Shape.Icon != nil && cell.Icon == nil {
		cell.Icon = &shapegrid.IconSpec{
			Name:     c.Shape.Icon.Name,
			Path:     c.Shape.Icon.Path,
			SVGData:  c.Shape.Icon.SVGData,
			Alt:      c.Shape.Icon.Alt,
			Fill:     c.Shape.Icon.Fill,
			Position: c.Shape.Icon.Position,
			Scale:    c.Shape.Icon.Scale,
		}
	}
	if c.Image != nil {
		imgSpec := &shapegrid.ImageSpec{
			Path: c.Image.Path,
			Alt:  c.Image.Alt,
		}
		if c.Image.Overlay != nil {
			imgSpec.Overlay = &shapegrid.OverlaySpec{
				Color: c.Image.Overlay.Color,
				Alpha: c.Image.Overlay.Alpha,
			}
		}
		if c.Image.Text != nil {
			imgSpec.Text = &shapegrid.ImageText{
				Content:       c.Image.Text.Content,
				Size:          c.Image.Text.Size,
				Bold:          c.Image.Text.Bold,
				Color:         c.Image.Text.Color,
				Align:         c.Image.Text.Align,
				VerticalAlign: c.Image.Text.VerticalAlign,
				Font:          c.Image.Text.Font,
			}
		}
		cell.Image = imgSpec
	}
	if c.Diagram != nil {
		cell.DiagramSpec = c.Diagram
	}
	if c.Composite != nil {
		comp := &shapegrid.CompositeSpec{
			SubDiagram: c.Composite.SubDiagram,
			Split:      shapegrid.CompositeSplit(c.Composite.Split),
			Ratio:      c.Composite.Ratio,
		}
		if c.Composite.Text != nil {
			comp.Text = &shapegrid.ShapeSpec{
				Geometry:    c.Composite.Text.Geometry,
				Fill:        c.Composite.Text.Fill,
				Line:        c.Composite.Text.Line,
				Text:        c.Composite.Text.Text,
				Rotation:    c.Composite.Text.Rotation,
				Adjustments: c.Composite.Text.Adjustments,
			}
		}
		cell.Composite = comp
	}
	if c.AccentBar != nil {
		cell.AccentBar = &shapegrid.AccentBarSpec{
			Position: c.AccentBar.Position,
			Color:    c.AccentBar.Color,
			Width:    c.AccentBar.Width,
		}
	}
	return cell
}

// collectDiagramCellFindings emits render-time fit findings for a resolved
// diagram cell: narrow-cell legibility and cell/SVG aspect mismatch/conflict.
// Legibility and conflict checks use the post-fit Bounds — the frame the diagram
// is sized into and placed at by generateDiagramCellInserts — so the findings
// reflect what is actually rendered (including any cell.fit adjustment). The
// aspect-mismatch check additionally receives the original (pre-fit) CellBounds
// so its evidence can distinguish an authoring mistake from a fit-driven render
// mismatch.
func collectDiagramCellFindings(cell shapegrid.ResolvedCell, slideIdx int) []patterns.FitFinding {
	path := slidepath.GridCellField(slideIdx, cell.RowIdx, cell.ColIdx, "diagram")
	var findings []patterns.FitFinding
	if f := generator.CheckDiagramInNarrowBoundsFinding(cell.DiagramSpec, cell.Bounds.CX, path); f != nil {
		findings = append(findings, *f)
	}
	cellBox := types.BoundingBox{Width: cell.CellBounds.CX, Height: cell.CellBounds.CY}
	renderBox := types.BoundingBox{Width: cell.Bounds.CX, Height: cell.Bounds.CY}
	if f := generator.CheckDiagramAspectMismatchFinding(cell.DiagramSpec, cellBox, renderBox, path); f != nil {
		findings = append(findings, *f)
	}
	if f := generator.CheckDiagramAspectConflictFinding(cell.DiagramSpec, cell.Bounds.CX, cell.Bounds.CY, path); f != nil {
		findings = append(findings, *f)
	}
	return findings
}

// generateGridOutput converts resolved grid cells into XML fragments and media inserts.
// slideIdx is the 0-based slide index used for constructing JSON paths in findings.
func generateGridOutput(result *shapegrid.ResolveResult, alloc *pptx.ShapeIDAllocator, diagCtx *GridDiagramContext, slideIdx int) (*ShapeGridResult, error) {
	var shapes [][]byte
	var iconInserts []generator.IconInsert
	var imageInserts []generator.ImageInsert
	var warnings []string
	var fitFindings []patterns.FitFinding

	// Connectors are emitted FIRST so they render BEHIND the cells they
	// connect; otherwise a horizontal arrow drawn between adjacent cells
	// shows through (and overlays) the cell fills and labels (e.g.
	// process-flow, timeline-horizontal).
	for _, conn := range result.Connectors {
		xml, err := generateConnectorXML(conn)
		if err != nil {
			return nil, fmt.Errorf("connector id %d: %w", conn.ID, err)
		}
		shapes = append(shapes, xml)
	}

	for _, cell := range result.Cells {
		var cellShapes [][]byte
		var cellIcons []generator.IconInsert
		var cellImages []generator.ImageInsert

		switch cell.Kind {
		case shapegrid.CellKindShape:
			s, icons, err := generateShapeCellXML(cell, alloc)
			if err != nil {
				return nil, err
			}
			cellShapes = append(cellShapes, s...)
			cellIcons = append(cellIcons, icons...)
		case shapegrid.CellKindTable:
			cfg := generator.TableRenderConfig{
				Bounds: types.BoundingBox{
					X:      cell.Bounds.X,
					Y:      cell.Bounds.Y,
					Width:  cell.Bounds.CX,
					Height: cell.Bounds.CY,
				},
				Style:            cell.TableSpec.Style,
				ColumnAlignments: cell.TableSpec.ColumnAlignments,
			}
			tblResult, err := generator.GenerateTableXML(cell.TableSpec, cfg)
			if err != nil {
				return nil, fmt.Errorf("table in grid: %w", err)
			}
			cellShapes = append(cellShapes, []byte(tblResult.XML))
		case shapegrid.CellKindIcon:
			svgData, err := resolveIconSVG(cell.IconSpec)
			if err != nil {
				return nil, fmt.Errorf("icon in grid: %w", err)
			}
			cellIcons = append(cellIcons, generator.IconInsert{
				SVGData:  svgData,
				Alt:      iconAltText(cell.IconSpec),
				OffsetX:  cell.Bounds.X,
				OffsetY:  cell.Bounds.Y,
				ExtentCX: cell.Bounds.CX,
				ExtentCY: cell.Bounds.CY,
			})
		case shapegrid.CellKindDiagram:
			icons, diagramWarnings, err := generateDiagramCellInserts(cell, diagCtx)
			if err != nil {
				return nil, err
			}
			cellIcons = append(cellIcons, icons...)
			warnings = append(warnings, diagramWarnings...)
			fitFindings = append(fitFindings, collectDiagramCellFindings(cell, slideIdx)...)
		case shapegrid.CellKindImage:
			s, imgs, err := generateImageCellXML(cell, alloc)
			if err != nil {
				return nil, err
			}
			cellShapes = append(cellShapes, s...)
			cellImages = append(cellImages, imgs...)
		case shapegrid.CellKindSubGrid:
			// Bounds-only placeholder; the parent resolver delegates nested
			// rendering to its caller, which uses cell.Bounds to recursively
			// resolve the sub-grid. Skip XML emission here.
			continue
		default:
			return nil, fmt.Errorf("unsupported cell kind: %s", cell.Kind)
		}

		// Wrap in p:grpSp if group flag is set and there are XML fragments to wrap
		if cell.Group && len(cellShapes) > 0 {
			groupID := alloc.Alloc()
			grpXML, err := pptx.GenerateGroup(pptx.GroupOptions{
				ID:       groupID,
				Bounds:   cell.Bounds,
				Children: cellShapes,
			})
			if err != nil {
				return nil, fmt.Errorf("group for cell id %d: %w", cell.ID, err)
			}
			shapes = append(shapes, grpXML)
		} else {
			shapes = append(shapes, cellShapes...)
		}
		iconInserts = append(iconInserts, cellIcons...)
		imageInserts = append(imageInserts, cellImages...)
	}

	// Generate XML for accent bars
	for _, bar := range result.AccentBars {
		xml, err := shapegrid.GenerateAccentBarXML(&bar)
		if err != nil {
			return nil, fmt.Errorf("accent bar id %d: %w", bar.ID, err)
		}
		shapes = append(shapes, xml)
	}

	return &ShapeGridResult{
		Shapes:       shapes,
		Cells:        result.Cells,
		IconInserts:  iconInserts,
		ImageInserts: imageInserts,
		RowOverflows: result.RowOverflows,
		Warnings:     warnings,
		FitFindings:  fitFindings,
	}, nil
}

// generateShapeCellXML produces XML and icon inserts for a shape cell.
func generateShapeCellXML(cell shapegrid.ResolvedCell, _ *pptx.ShapeIDAllocator) ([][]byte, []generator.IconInsert, error) {
	xml, err := shapegrid.GenerateShapeXML(cell.ShapeSpec, cell.ID, cell.Bounds, cell.TextInsets)
	if err != nil {
		return nil, nil, fmt.Errorf("shape id %d: %w", cell.ID, err)
	}
	shapes := [][]byte{xml}
	var icons []generator.IconInsert
	if cell.IconSpec != nil {
		svgData, err := resolveIconSVG(cell.IconSpec)
		if err != nil {
			return nil, nil, fmt.Errorf("icon overlay on shape id %d: %w", cell.ID, err)
		}
		ib := cell.IconBounds
		if ib.CX == 0 && ib.CY == 0 {
			ib = cell.Bounds
		}
		icons = append(icons, generator.IconInsert{
			SVGData:  svgData,
			Alt:      iconAltText(cell.IconSpec),
			OffsetX:  ib.X,
			OffsetY:  ib.Y,
			ExtentCX: ib.CX,
			ExtentCY: ib.CY,
		})
	}
	return shapes, icons, nil
}

// iconAltText returns alt text for an icon. Prefers the explicit Alt field,
// falling back to a value derived from Name or Path.
func iconAltText(spec *shapegrid.IconSpec) string {
	if spec == nil {
		return ""
	}
	if spec.Alt != "" {
		return spec.Alt
	}
	if spec.Name != "" {
		return spec.Name + " icon"
	}
	if spec.Path != "" {
		return filepath.Base(spec.Path) + " icon"
	}
	return "icon"
}

// generateImageCellXML produces XML overlays/text and image inserts for an image cell.
func generateImageCellXML(cell shapegrid.ResolvedCell, alloc *pptx.ShapeIDAllocator) ([][]byte, []generator.ImageInsert, error) {
	imgs := []generator.ImageInsert{{
		Path:     cell.ImageSpec.Path,
		Alt:      cell.ImageSpec.Alt,
		OffsetX:  cell.Bounds.X,
		OffsetY:  cell.Bounds.Y,
		ExtentCX: cell.Bounds.CX,
		ExtentCY: cell.Bounds.CY,
	}}
	var shapes [][]byte
	if cell.ImageSpec.Overlay != nil {
		overlayID := alloc.Alloc()
		overlayXML, err := shapegrid.GenerateImageOverlayXML(cell.ImageSpec.Overlay, overlayID, cell.Bounds)
		if err != nil {
			return nil, nil, fmt.Errorf("image overlay id %d: %w", overlayID, err)
		}
		shapes = append(shapes, overlayXML)
	}
	if cell.ImageSpec.Text != nil {
		textID := alloc.Alloc()
		textXML, err := shapegrid.GenerateImageTextXML(cell.ImageSpec.Text, textID, cell.Bounds)
		if err != nil {
			return nil, nil, fmt.Errorf("image text id %d: %w", textID, err)
		}
		shapes = append(shapes, textXML)
	}
	return shapes, imgs, nil
}

// cloneDiagramSpecForCell returns a shallow clone of spec with an independent
// Style so callers can mutate render-time fields (Style.ThemeColors,
// Style.DataPalette, Width, Height) without altering the caller's struct.
// Data/Warnings slices are shared by reference because they are read-only
// inside the cell rendering path.
func cloneDiagramSpecForCell(spec *types.DiagramSpec) *types.DiagramSpec {
	specCopy := *spec
	if spec.Style != nil {
		styleCopy := *spec.Style
		specCopy.Style = &styleCopy
	} else {
		specCopy.Style = &types.DiagramStyle{}
	}
	return &specCopy
}

// generateDiagramCellInserts renders a diagram cell via svggen and returns IconInserts
// for native SVG embedding. The diagram is rendered as SVG only (no rasterization
// needed — the singlepass generator uses a 1x1 transparent PNG fallback for native SVG).
//
// diagCtx provides template theme colors and data palette so grid-cell diagrams
// inherit the same color scheme as placeholder-based diagrams.
func generateDiagramCellInserts(cell shapegrid.ResolvedCell, diagCtx *GridDiagramContext) ([]generator.IconInsert, []string, error) {
	// Clone the caller's DiagramSpec before injecting theme/palette state or
	// defaulting Width/Height. An agent reusing the same DiagramSpec across
	// cells, slides, or retries must observe byte-identical input on every
	// call; persisting injected state between calls is hidden, surprising
	// behavior at the MCP boundary (go-slide-creator-zg8q.7).
	diagramSpec := cloneDiagramSpecForCell(cell.DiagramSpec)

	// Inject theme colors if diagram doesn't have explicit Colors set,
	// mirroring the placeholder-based diagram path in processDiagramContent.
	var themeColors []types.ThemeColor
	if diagCtx != nil {
		themeColors = diagCtx.ThemeColors
		if len(diagramSpec.Style.Colors) == 0 && len(diagCtx.ThemeColors) > 0 {
			diagramSpec.Style.ThemeColors = diagCtx.ThemeColors
		}
		if len(diagramSpec.Style.Colors) == 0 && len(diagCtx.DataPalette) > 0 {
			diagramSpec.Style.DataPalette = diagCtx.DataPalette
		}
		// Inject the template body font when the diagram doesn't set one
		// explicitly, so grid-cell diagrams render with the template's
		// typography. An explicit style.font_family always wins.
		if diagramSpec.Style.FontFamily == "" && diagCtx.FontFamily != "" {
			diagramSpec.Style.FontFamily = diagCtx.FontFamily
		}
	}

	// Resolve the effective render dimensions once via the shared resolver so the
	// rendered SVG matches the post-fit cell aspect and the aspect/fit findings
	// (which resolve the same way against the same Bounds) agree with what is
	// drawn. Without this, grid-cell diagrams default to 800x600 and silently
	// letterbox inside non-4:3 cells. Explicit dims are preserved verbatim; a
	// single author-set dimension keeps that axis and derives the other from the
	// cell aspect. The placeholder path uses the same resolver (see media.go).
	cellBox := types.BoundingBox{
		X:      cell.Bounds.X,
		Y:      cell.Bounds.Y,
		Width:  cell.Bounds.CX,
		Height: cell.Bounds.CY,
	}
	diagramSpec.Width, diagramSpec.Height, _ = generator.ResolveDiagramRenderDimensions(diagramSpec, cellBox)

	result, err := generator.RenderDiagramSpecWithMetadata(diagramSpec, themeColors, 0, true)
	if err != nil {
		return nil, nil, fmt.Errorf("diagram in grid cell %d: %w", cell.ID, err)
	}
	if len(result.SVG) == 0 {
		return nil, nil, fmt.Errorf("diagram in grid cell %d: renderer returned empty SVG", cell.ID)
	}

	// Check for complex diagram in narrow cell
	var warnings []string
	if diagCtx != nil {
		location := fmt.Sprintf("grid cell %d", cell.ID)
		if w := generator.CheckDiagramInNarrowBounds(diagramSpec, cell.Bounds.CX, diagCtx.SlideNum, location); w != "" {
			warnings = append(warnings, w)
		}
	}

	// Build alt-text from diagram type and title
	alt := strings.ReplaceAll(diagramSpec.Type, "_", " ") + " diagram"
	if diagramSpec.Title != "" {
		alt = diagramSpec.Title + " (" + diagramSpec.Type + ")"
	}
	return []generator.IconInsert{{
		SVGData:  result.SVG,
		Alt:      alt,
		OffsetX:  cell.Bounds.X,
		OffsetY:  cell.Bounds.Y,
		ExtentCX: cell.Bounds.CX,
		ExtentCY: cell.Bounds.CY,
		// Honor the cell.Group flag for diagram cells: the singlepass
		// emitter wraps the resulting p:pic in a p:grpSp so PowerPoint
		// treats the diagram as a single selection target, matching the
		// behavior of grouped native shape cells (go-slide-creator-zg8q.10).
		Group: cell.Group,
	}}, warnings, nil
}

// resolveColumnsDTO parses the JSON columns field and returns percentage widths.
func resolveColumnsDTO(raw json.RawMessage, rows []GridRowInput) ([]float64, error) {
	if len(raw) == 0 {
		// Infer from max cell count across rows
		maxCols := 0
		for _, row := range rows {
			if len(row.Cells) > maxCols {
				maxCols = len(row.Cells)
			}
		}
		if maxCols == 0 {
			return nil, fmt.Errorf("shape_grid: no cells defined; add cells with a \"shape\", \"table\", \"icon\", \"image\", \"diagram\", or \"composite\" key to at least one row")
		}
		return shapegrid.ResolveColumns(nil, []int{maxCols})
	}

	// Try number (equal columns)
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return shapegrid.ResolveColumns(int(n), nil)
	}

	// Try array of percentages
	var arr []float64
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("shape_grid: columns must be a number (e.g. 3) or array of percentages (e.g. [30, 40, 30]): %w", err)
	}
	return shapegrid.ResolveColumns(arr, nil)
}

// imageAssetExtensions lists allowed extensions for image_value, GridImageInput,
// and slide background.image fields. Compared case-insensitively against the
// file extension (including the leading dot).
var imageAssetExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".webp": true,
}

// resolveIconPaths resolves and validates icon.path fields across all slides.
// Relative paths are resolved against baseDir (the directory containing the JSON input file).
// Each path is cleaned, converted to absolute form, evaluated for symlinks, and validated
// against path traversal attacks.
//
// All failures are collected as structured diagnostics — one per offending icon —
// so callers can surface the full set in a single pass instead of stopping at
// the first error. On success the slice is empty (nil). Per-icon JSON Pointer
// paths (RFC 6901) and the input value are recorded so agents can locate and
// repair each broken icon.
//
// Prefer resolveLocalAssetPaths in new callers — it walks all local-asset
// surfaces (icon, image, background) in one pass.
func resolveIconPaths(slides []SlideInput, baseDir string) []diagnostics.Diagnostic {
	var findings []diagnostics.Diagnostic
	for i := range slides {
		if slides[i].ShapeGrid == nil {
			continue
		}
		for j := range slides[i].ShapeGrid.Rows {
			for k := range slides[i].ShapeGrid.Rows[j].Cells {
				cell := slides[i].ShapeGrid.Rows[j].Cells[k]
				if cell == nil {
					continue
				}
				// Resolve icon on cell
				if cell.Icon != nil {
					path := slidepath.GridCellField(i, j, k, "icon")
					findings = append(findings, resolveIconInputPath(cell.Icon, baseDir, i, path)...)
				}
				// Resolve icon nested inside shape
				if cell.Shape != nil && cell.Shape.Icon != nil {
					path := slidepath.GridCellField(i, j, k, "shape/icon")
					findings = append(findings, resolveIconInputPath(cell.Shape.Icon, baseDir, i, path)...)
				}
			}
		}
	}
	return findings
}

// resolveLocalAssetPaths walks every slide and rewrites all relative
// local-asset paths to absolute, symlink-evaluated form. This covers:
//
//   - icon.path on shape_grid cells (cell.Icon, cell.Shape.Icon) — same
//     semantics as resolveIconPaths.
//   - image_value.path on content items (slide.content[].image_value.path).
//   - GridImageInput.path on shape_grid cells (cell.Image.Path).
//   - slide.background.image on the slide-level Background.
//
// Failures are collected as structured diagnostics — one per offending field —
// so callers see every broken asset in one pass instead of bailing on the
// first error. Each diagnostic carries an "asset_kind" detail
// ("icon" / "image" / "background") plus the offending input value and a
// JSON Pointer path, so agents can locate and repair each asset
// independently.
//
// URL-only and inline-svg fields are skipped — they are resolved elsewhere
// (resolveURLs for URLs; inline svg_data needs no resolution).
//
// panelFamilyDiagramTypes are the diagram types rendered by the native panel
// path that accept a per-panel icon.
var panelFamilyDiagramTypes = map[string]bool{
	"panel_layout": true,
	"icon_columns": true,
	"icon_rows":    true,
	"stat_cards":   true,
}

// resolvePanelDiagramIcons resolves file-path icons inside panel-family diagram
// content into inline svg_data (read from disk and recolored) so the generator
// embeds them as native SVG, exactly as shape_grid icon paths are handled.
// Bundled-name, inline-svg, data-URI, and URL icons are left untouched for the
// generator's resolver. Path resolution and traversal-safety reuse
// resolveIconInputPath; the read + fill reuse resolveIconSVG.
func resolvePanelDiagramIcons(slide *SlideInput, baseDir string, slideIdx int) []diagnostics.Diagnostic {
	var findings []diagnostics.Diagnostic
	for j := range slide.Content {
		c := &slide.Content[j]
		if c.Type != "diagram" || c.DiagramValue == nil || !panelFamilyDiagramTypes[c.DiagramValue.Type] {
			continue
		}
		panels, ok := c.DiagramValue.Data["panels"].([]any)
		if !ok {
			continue
		}
		jsonPath := slidepath.ContentField(slideIdx, j, "diagram_value/data/panels")
		for k := range panels {
			m, ok := panels[k].(map[string]any)
			if !ok {
				continue
			}
			raw, ok := m["icon"]
			if !ok {
				continue
			}
			icon := panelIconToInput(raw)
			if icon == nil || icon.Path == "" {
				continue // only file-path icons need cmd-side resolution
			}
			if diags := resolveIconInputPath(icon, baseDir, slideIdx, jsonPath); len(diags) > 0 {
				findings = append(findings, diags...)
				if anyDiagnosticError(diags) {
					continue
				}
			}
			svg, err := resolveIconSVG(&shapegrid.IconSpec{Path: icon.Path, Fill: icon.Fill})
			if err != nil {
				findings = append(findings, diagnostics.Diagnostic{
					Code:     diagnostics.CodeIconPath,
					Message:  fmt.Sprintf("panel icon path %q: %v", icon.Path, err),
					Path:     jsonPath,
					Severity: diagnostics.SeverityError,
					Details:  map[string]any{"slide_index": slideIdx, "asset_kind": "icon"},
				})
				continue
			}
			// Replace the path icon with pre-resolved, pre-recolored inline SVG.
			m["icon"] = map[string]any{"svg_data": string(svg), "alt": icon.Alt}
		}
	}
	return findings
}

// panelIconToInput converts a panel "icon" field (string shorthand or object)
// into an *IconInput. A bare string is treated as a .svg file path only when it
// carries a .svg extension; bundled names, inline SVG, URLs, and data URIs leave
// Path empty so they fall through to the generator's resolver.
func panelIconToInput(raw any) *IconInput {
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		ii := &IconInput{}
		lower := strings.ToLower(s)
		switch {
		case strings.HasPrefix(lower, "<svg"), strings.HasPrefix(lower, "http://"),
			strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "data:"):
			// inline / url / data URI — handled elsewhere.
		case strings.HasSuffix(lower, ".svg"):
			ii.Path = s
		default:
			ii.Name = s
		}
		return ii
	case map[string]any:
		ii := &IconInput{}
		if s, ok := v["name"].(string); ok {
			ii.Name = s
		}
		if s, ok := v["path"].(string); ok {
			ii.Path = s
		}
		if s, ok := v["url"].(string); ok {
			ii.URL = s
		}
		if s, ok := v["svg_data"].(string); ok {
			ii.SVGData = s
		}
		if s, ok := v["fill"].(string); ok {
			ii.Fill = s
		}
		if s, ok := v["alt"].(string); ok {
			ii.Alt = s
		}
		return ii
	default:
		return nil
	}
}

// anyDiagnosticError reports whether any diagnostic has error severity.
func anyDiagnosticError(diags []diagnostics.Diagnostic) bool {
	for i := range diags {
		if diags[i].Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}

func resolveLocalAssetPaths(slides []SlideInput, baseDir string) []diagnostics.Diagnostic {
	var findings []diagnostics.Diagnostic
	// Reuse the existing icon walker so its tests stay authoritative.
	findings = append(findings, resolveIconPaths(slides, baseDir)...)
	for i := range slides {
		findings = append(findings, resolveSlideAssets(&slides[i], baseDir, i)...)
	}
	return findings
}

// resolveSlideAssets resolves background, content image_value, and shape-grid
// image cell paths for one slide. Extracted from resolveLocalAssetPaths to
// keep that function's cognitive complexity under the linter ceiling.
func resolveSlideAssets(slide *SlideInput, baseDir string, slideIdx int) []diagnostics.Diagnostic {
	var findings []diagnostics.Diagnostic
	if bg := slide.Background; bg != nil && bg.Image != "" {
		path := slidepath.SlideField(slideIdx, "background/image")
		findings = append(findings, applyLocalAssetPath(&bg.Image, baseDir,
			diagnostics.CodeBackgroundImagePath, "background", slideIdx, path)...)
	}
	for j := range slide.Content {
		c := &slide.Content[j]
		if c.Type != "image" || c.ImageValue == nil || c.ImageValue.Path == "" {
			continue
		}
		path := slidepath.ContentField(slideIdx, j, "image_value/path")
		findings = append(findings, applyLocalAssetPath(&c.ImageValue.Path, baseDir,
			diagnostics.CodeImagePath, "image", slideIdx, path)...)
	}
	// Resolve file-path icons inside panel-family diagrams into inline svg_data so
	// the generator embeds them as native SVG (matching shape_grid icon handling).
	findings = append(findings, resolvePanelDiagramIcons(slide, baseDir, slideIdx)...)
	if slide.ShapeGrid == nil {
		return findings
	}
	for j := range slide.ShapeGrid.Rows {
		for k := range slide.ShapeGrid.Rows[j].Cells {
			cell := slide.ShapeGrid.Rows[j].Cells[k]
			if cell == nil || cell.Image == nil || cell.Image.Path == "" {
				continue
			}
			path := slidepath.GridCellField(slideIdx, j, k, "image/path")
			findings = append(findings, applyLocalAssetPath(&cell.Image.Path, baseDir,
				diagnostics.CodeImagePath, "image", slideIdx, path)...)
		}
	}
	return findings
}

// applyLocalAssetPath wraps resolveLocalAssetPath plus the writeback rule:
// commit the resolved path whenever one is returned (which covers both the
// fully-clean case and the soft-cap warning case), and bubble any diagnostic
// the resolver produced. Centralizes the writeback so resolveSlideAssets
// stays linear.
func applyLocalAssetPath(dst *string, baseDir string, code diagnostics.Code, assetKind string, slideIdx int, jsonPath string) []diagnostics.Diagnostic {
	resolved, diag := resolveLocalAssetPath(*dst, baseDir, imageAssetExtensions, code, assetKind, slideIdx, jsonPath)
	if resolved != "" {
		*dst = resolved
	}
	if diag != nil {
		return []diagnostics.Diagnostic{*diag}
	}
	return nil
}

// expandAssetPath expands a leading "~/" (or bare "~") to the user's home
// directory and expands "$VAR" / "${VAR}" references via the process
// environment. The leading-tilde rule mirrors POSIX shells: only "~" or "~/"
// is rewritten; embedded tildes are passed through unchanged. Environment
// expansion accepts the same syntax as os.Expand.
//
// When any referenced environment variable is unset, expansion is aborted and
// the variable name is returned in unsetVar — callers surface this as an
// ASSET_PATH_ENV_UNSET finding rather than letting an empty expansion silently
// produce a confusing "no such file" downstream. The returned path is the
// original rawPath when unsetVar is non-empty so error messages quote the
// agent's literal input.
func expandAssetPath(rawPath string) (string, string) {
	p := rawPath
	switch {
	case p == "~":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			p = home
		}
	case strings.HasPrefix(p, "~/"):
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			p = filepath.Join(home, p[2:])
		}
	}
	var unsetVar string
	expanded := os.Expand(p, func(name string) string {
		v, ok := os.LookupEnv(name)
		if !ok && unsetVar == "" {
			unsetVar = name
		}
		return v
	})
	if unsetVar != "" {
		return rawPath, unsetVar
	}
	return expanded, ""
}

// prepareIconPath runs the per-icon path prep steps shared by all callers:
// a pre-clean traversal check (catches "../.." before filepath.Clean
// collapses it), tilde/$ENV expansion, unset-env-var detection, and a
// post-expansion traversal check. Returns the expanded path on success, or a
// single diagnostic naming the failure mode. Extracted from
// resolveIconInputPath to keep that function's cyclomatic complexity under
// the linter ceiling without inlining a Nolint pragma.
func prepareIconPath(rawPath string, slideIdx int, jsonPath string) (string, *diagnostics.Diagnostic) {
	if err := utils.ValidatePath(filepath.FromSlash(rawPath), nil); err != nil {
		return "", &diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconPathTraversal,
			Message:  fmt.Sprintf("icon path %q: %v", rawPath, err),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  "icon",
				"input_value": rawPath,
				"remediation": "use a path that does not escape the base directory",
			},
		}
	}
	expanded, unsetVar := expandAssetPath(rawPath)
	if unsetVar != "" {
		return "", &diagnostics.Diagnostic{
			Code:     diagnostics.CodeAssetPathEnvUnset,
			Message:  fmt.Sprintf("icon path %q references unset environment variable %q", rawPath, unsetVar),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index":  slideIdx,
				"asset_kind":   "icon",
				"input_value":  rawPath,
				"env_variable": unsetVar,
				"remediation":  fmt.Sprintf("export %s before invoking, or supply a literal path", unsetVar),
			},
		}
	}
	if err := utils.ValidatePath(filepath.FromSlash(expanded), nil); err != nil {
		return "", &diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconPathTraversal,
			Message:  fmt.Sprintf("icon path %q (expanded %q): %v", rawPath, expanded, err),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  "icon",
				"input_value": rawPath,
				"remediation": "use a path that does not escape the base directory after expansion",
			},
		}
	}
	return expanded, nil
}

// resolveLocalAssetPath validates a single asset path field (image or
// background) and returns either the resolved absolute path or a single
// diagnostic describing the problem. The caller is responsible for writing
// the resolved value back to the input struct.
//
// Validation pipeline (mirrors resolveIconInputPath):
//  1. Extension allow-list (assetKindExtensions).
//  2. Tilde / $ENV expansion (expandAssetPath) — unset env vars yield
//     ASSET_PATH_ENV_UNSET; tilde with no home directory passes through
//     unchanged and fails normally at EvalSymlinks.
//  3. Relative-to-absolute resolution against baseDir.
//  4. filepath.EvalSymlinks (catches missing files and symlink loops).
//  5. utils.ValidatePath (catches ".." traversal).
//
// code is the diagnostic Code emitted on failure; assetKind is recorded in
// the Details map so callers can group / filter by kind without parsing
// codes.
func resolveLocalAssetPath(rawPath, baseDir string, allowedExts map[string]bool, code diagnostics.Code, assetKind string, slideIdx int, jsonPath string) (string, *diagnostics.Diagnostic) {
	// Extension allow-list check (case-insensitive). Run on the raw input so
	// the diagnostic quotes what the agent supplied; ~/foo.png and $VAR/foo.png
	// keep their .png extension across expansion.
	ext := strings.ToLower(filepath.Ext(rawPath))
	if !allowedExts[ext] {
		return "", &diagnostics.Diagnostic{
			Code:     code,
			Message:  fmt.Sprintf("%s path %q: unsupported extension %q", assetKind, rawPath, ext),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  assetKind,
				"input_value": rawPath,
				"remediation": "use a supported file extension: " + joinSortedExts(allowedExts),
			},
		}
	}

	// Expand "~/..." and "$VAR" before joining baseDir so an agent-supplied
	// "~/assets/logo.png" or "$BRAND_ASSETS/logo.png" resolves against the home
	// directory or env-pointed root instead of being silently rooted under
	// baseDir. Unset env vars short-circuit with a structured finding.
	expanded, unsetVar := expandAssetPath(rawPath)
	if unsetVar != "" {
		return "", &diagnostics.Diagnostic{
			Code:     diagnostics.CodeAssetPathEnvUnset,
			Message:  fmt.Sprintf("%s path %q references unset environment variable %q", assetKind, rawPath, unsetVar),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index":  slideIdx,
				"asset_kind":   assetKind,
				"input_value":  rawPath,
				"env_variable": unsetVar,
				"remediation":  fmt.Sprintf("export %s before invoking, or supply a literal path", unsetVar),
			},
		}
	}

	// Resolve relative paths against baseDir.
	p := filepath.FromSlash(expanded)
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	p = filepath.Clean(p)

	// Evaluate symlinks for security (also catches missing files).
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", &diagnostics.Diagnostic{
			Code:     code,
			Message:  fmt.Sprintf("%s path %q: %v", assetKind, rawPath, err),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  assetKind,
				"input_value": rawPath,
				"remediation": "verify the file exists relative to the JSON input directory (CLI) or base_dir (MCP)",
			},
		}
	}

	// Validate against path traversal.
	if err := utils.ValidatePath(resolved, nil); err != nil {
		return "", &diagnostics.Diagnostic{
			Code:     code,
			Message:  fmt.Sprintf("%s path %q: %v", assetKind, rawPath, err),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  assetKind,
				"input_value": rawPath,
				"remediation": "use a path that does not escape the base directory",
			},
		}
	}

	// Enforce per-asset size caps. A hard-cap breach is fatal: the path stays
	// unresolved so the caller skips committing it. A soft-cap breach is
	// advisory: the resolved path is returned, but the warning diagnostic is
	// surfaced alongside so callers append it to the diagnostic stream.
	if diag := checkAssetSize(resolved, rawPath, assetKind, slideIdx, jsonPath, code); diag != nil {
		if diag.Severity == diagnostics.SeverityError {
			return "", diag
		}
		return resolved, diag
	}

	return resolved, nil
}

// joinSortedExts returns a stable, comma-separated string of allowed
// extensions for inclusion in diagnostic remediation messages. Sorting keeps
// the message deterministic across runs (Go map iteration order is random).
func joinSortedExts(exts map[string]bool) string {
	keys := make([]string, 0, len(exts))
	for k := range exts {
		keys = append(keys, k)
	}
	// Simple insertion sort — set is small (<10 entries) so this is cheap and
	// avoids pulling in sort just for the side effect.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return strings.Join(keys, ", ")
}

// checkIconSourceArity validates that exactly one of name/path/url/svg_data
// is set on an icon spec. Returns the conflict (>1 source) or missing-source
// diagnostic, or nil when exactly one source is set. Extracted from
// resolveIconInputPath to keep that function under the linter complexity
// ceiling.
func checkIconSourceArity(hasName, hasPath, hasURL, hasSVGData bool, slideIdx int, jsonPath string) *diagnostics.Diagnostic {
	var setFields []string
	if hasName {
		setFields = append(setFields, "name")
	}
	if hasPath {
		setFields = append(setFields, "path")
	}
	if hasURL {
		setFields = append(setFields, "url")
	}
	if hasSVGData {
		setFields = append(setFields, "svg_data")
	}
	if len(setFields) > 1 {
		quoted := make([]string, len(setFields))
		for i, f := range setFields {
			quoted[i] = "'" + f + "'"
		}
		conflicting := strings.Join(quoted, ", ")
		return &diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconAmbiguous,
			Message:  fmt.Sprintf("icon has conflicting sources %s; exactly one of 'name', 'path', 'url', or 'svg_data' is allowed", conflicting),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index":        slideIdx,
				"conflicting_fields": setFields,
				"remediation":        fmt.Sprintf("keep one of %s and remove the others", conflicting),
			},
		}
	}
	if len(setFields) == 0 {
		example := "examples:\n" +
			`  { "name": "chart-pie" }                      // bundled icon` + "\n" +
			`  { "path": "icons/custom.svg" }               // local SVG file` + "\n" +
			`  { "url": "https://example.com/icon.svg" }    // remote SVG` + "\n" +
			`  { "svg_data": "<svg ...>...</svg>" }         // inline SVG`
		return &diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconMissing,
			Message:  "icon must have one of 'name', 'path', 'url', or 'svg_data'\n" + example,
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"remediation": "set one of 'name' (bundled icon), 'path' (filesystem), 'url' (remote), or 'svg_data' (inline)",
				"example":     example,
			},
		}
	}
	return nil
}

// resolveIconInputPath validates and resolves a single IconInput's path field.
// Returns a slice of diagnostics describing any problems found. Returns nil on
// success (in which case icon.Path is rewritten to the resolved absolute form).
// URL-based icons are skipped here — they are resolved by resolveURLs.
// Inline svg_data is also skipped — no disk I/O needed.
//
// jsonPath is the RFC 6901 pointer to the icon node, used so callers can map
// each finding back to the exact JSON location.
func resolveIconInputPath(icon *IconInput, baseDir string, slideIdx int, jsonPath string) []diagnostics.Diagnostic {
	hasName := icon.Name != ""
	hasPath := icon.Path != ""
	hasURL := icon.URL != ""
	hasSVGData := icon.SVGData != ""

	if diag := checkIconSourceArity(hasName, hasPath, hasURL, hasSVGData, slideIdx, jsonPath); diag != nil {
		return []diagnostics.Diagnostic{*diag}
	}

	var findings []diagnostics.Diagnostic
	if hasSVGData && icon.Fill != "" {
		findings = append(findings, diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconFillIgnoredInline,
			Message:  fmt.Sprintf("icon 'fill' (%q) is ignored when 'svg_data' is set; pre-style the SVG or switch to 'name'/'path'", icon.Fill),
			Path:     jsonPath,
			Severity: diagnostics.SeverityWarning,
			Details: map[string]any{
				"slide_index": slideIdx,
				"input_value": icon.Fill,
				"remediation": "either pre-color the inline svg_data markup, or remove svg_data and use 'name'/'path' with 'fill'",
			},
		})
	}

	if hasSVGData {
		if extra, blocked := checkInlineSVGSize(icon.SVGData, slideIdx, jsonPath); extra != nil {
			findings = append(findings, *extra)
			if blocked {
				return findings
			}
		}
	}

	if hasName {
		// Validate the bundled name exists in the embedded icon registry.
		// Catches typos and missing "filled:" prefixes before generate so
		// agents don't burn a generate cycle on a fixable lookup.
		if !icons.Exists(icon.Name) {
			return append(findings, buildBundledIconNameFinding(icon.Name, slideIdx, jsonPath))
		}
		return findings
	}

	if !hasPath {
		return findings // URL or inline svg_data — no local path resolution needed
	}

	// Icons must be SVG — catches agents passing PNGs to icon.path before
	// disk I/O so the failure is deterministic and the remediation obvious.
	if ext := strings.ToLower(filepath.Ext(icon.Path)); ext != ".svg" {
		return []diagnostics.Diagnostic{{
			Code:     diagnostics.CodeIconPathExtInvalid,
			Message:  fmt.Sprintf("icon path %q: unsupported extension %q (icons must be .svg)", icon.Path, ext),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  "icon",
				"input_value": icon.Path,
				"remediation": "use a .svg file; for raster images use image_value or shape_grid image cells instead",
			},
		}}
	}

	// Pre-clean traversal check, "~/..." / "$VAR" expansion, unset-env-var
	// rejection, and post-expansion traversal check are bundled in
	// prepareIconPath so this function stays under the gocyclo ceiling.
	expanded, prepDiag := prepareIconPath(icon.Path, slideIdx, jsonPath)
	if prepDiag != nil {
		return []diagnostics.Diagnostic{*prepDiag}
	}

	// Resolve relative path against baseDir
	p := filepath.FromSlash(expanded)
	wasRelative := !filepath.IsAbs(p)
	if wasRelative {
		p = filepath.Join(baseDir, p)
	}
	p = filepath.Clean(p)

	// Evaluate symlinks for security
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		code := diagnostics.CodeIconPath
		remediation := "verify the file exists; switch to a bundled icon via 'name' or supply 'svg_data'"
		if os.IsNotExist(err) {
			code = diagnostics.CodeIconNotFound
		}
		return []diagnostics.Diagnostic{{
			Code:     code,
			Message:  fmt.Sprintf("icon path %q: %v", icon.Path, err),
			Path:     jsonPath,
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"slide_index": slideIdx,
				"asset_kind":  "icon",
				"input_value": icon.Path,
				"remediation": remediation,
			},
		}}
	}

	// Symlink escape: when the input was relative, the resolved path must stay
	// inside the absolute baseDir. A symlink that points outside baseDir lets
	// an agent read or attach arbitrary files via a relative-looking path.
	//
	// Both sides need symlink resolution before comparison. On macOS, /var is
	// itself a symlink to /private/var, so a baseDir under /var would otherwise
	// falsely flag every legitimate file beneath it.
	if wasRelative && baseDir != "" {
		absBase, absErr := filepath.Abs(filepath.Clean(baseDir))
		if absErr == nil {
			// EvalSymlinks may fail if baseDir doesn't exist; fall back to the
			// abs form so we still get a sensible comparison.
			realBase, realErr := filepath.EvalSymlinks(absBase)
			if realErr != nil {
				realBase = absBase
			}
			rel, relErr := filepath.Rel(realBase, resolved)
			if relErr != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
				return []diagnostics.Diagnostic{{
					Code:     diagnostics.CodeIconPathSymlinkEscape,
					Message:  fmt.Sprintf("icon path %q: resolves outside base directory via symlink", icon.Path),
					Path:     jsonPath,
					Severity: diagnostics.SeverityError,
					Details: map[string]any{
						"slide_index":   slideIdx,
						"asset_kind":    "icon",
						"input_value":   icon.Path,
						"resolved_path": resolved,
						"remediation":   "supply an absolute path explicitly or remove the symlink chain that escapes the base directory",
					},
				}}
			}
		}
	}

	// Update the path to the resolved absolute path
	icon.Path = resolved

	// Enforce per-asset size caps. A hard-cap breach is fatal — return the
	// finding without committing the resolved path? The path is already
	// committed above for symmetry with the existing nil-return contract,
	// but a downstream caller using iconFindingsToError will still treat the
	// error-severity finding as blocking. A soft-cap breach is advisory and
	// passes through as a warning so generation proceeds.
	if diag := checkAssetSize(resolved, icon.Path, "icon", slideIdx, jsonPath, diagnostics.CodeAssetTooLarge); diag != nil {
		return append(findings, *diag)
	}
	return findings
}

// buildBundledIconNameFinding constructs an ICON_BUNDLED_NAME_UNKNOWN
// diagnostic for an icon.name that does not resolve in the bundled registry.
// It includes Levenshtein-based suggestions (and a cross-set "did you mean
// filled:X?" hint) so agents can repair the name without a separate
// list_icons round-trip.
func buildBundledIconNameFinding(name string, slideIdx int, jsonPath string) diagnostics.Diagnostic {
	suggestions := icons.Suggest(name, 3)
	details := map[string]any{
		"slide_index": slideIdx,
		"input_value": name,
		"remediation": "use a bundled icon name (call list_icons to discover available names and their qualified set:name form), or supply 'path', 'url', or 'svg_data' instead",
	}
	if len(suggestions) > 0 {
		details["suggestions"] = suggestions
	}
	msg := fmt.Sprintf("icon name %q not found in bundled registry", name)
	if len(suggestions) > 0 {
		msg = fmt.Sprintf("%s; did you mean %q?", msg, suggestions[0])
	}
	return diagnostics.Diagnostic{
		Code:     diagnostics.CodeIconBundledNameUnknown,
		Message:  msg,
		Path:     jsonPath,
		Severity: diagnostics.SeverityError,
		Details:  details,
	}
}

// iconFindingsToError aggregates blocking local-asset-resolution diagnostics
// (severity == error) into a single error suitable for CLI callers. Returns
// nil when findings is empty or contains only non-blocking warnings/info — the
// caller is expected to surface those through the warning channel instead.
// Despite the historical name, it now handles icon, image, and background
// findings emitted by resolveLocalAssetPaths.
func iconFindingsToError(findings []diagnostics.Diagnostic) error {
	parts := make([]string, 0, len(findings))
	for _, d := range findings {
		if d.Severity != diagnostics.SeverityError {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("asset path errors (%d):\n  - %s", len(parts), strings.Join(parts, "\n  - "))
}

// resolveIconSVG loads SVG bytes for an icon spec, optionally applying a fill color override.
// For inline SVG (SVGData set), the bytes are returned as-is (Fill is ignored).
// For bundled icons (Name set), it looks up from the embedded icon library.
// For custom icons (Path set), it reads the SVG file from disk.
// Fill color override is applied to both bundled and custom SVG icons.
func resolveIconSVG(spec *shapegrid.IconSpec) ([]byte, error) {
	limits := svgSizeLimits()
	if spec.SVGData != "" {
		// Inline SVG markup — no disk I/O, no fill recolor (agent supplies pre-styled SVG).
		// Hard cap enforcement is a defense in depth: preflight should have
		// already flagged oversized inline SVG, but a renderer-entry check
		// guards against callers that bypass preflight.
		if int64(len(spec.SVGData)) > limits.HardBytes {
			return nil, fmt.Errorf("inline svg_data exceeds hard cap of %s (got %s); shrink the SVG or supply a smaller alternative",
				humanizeBytes(limits.HardBytes), humanizeBytes(int64(len(spec.SVGData))))
		}
		return []byte(spec.SVGData), nil
	}
	if spec.Path != "" {
		// Custom SVG from file path (already resolved to absolute path). Stat
		// before reading so a 1 GB file does not get pulled into memory just
		// to be rejected.
		if info, err := os.Stat(spec.Path); err == nil && info.Size() > limits.HardBytes {
			return nil, fmt.Errorf("custom icon %q: file size %s exceeds hard cap of %s; shrink the SVG or supply a smaller alternative",
				spec.Path, humanizeBytes(info.Size()), humanizeBytes(limits.HardBytes))
		}
		svgData, err := os.ReadFile(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("custom icon %q: %w", spec.Path, err)
		}
		if spec.Fill != "" {
			svgData = applyIconFill(svgData, spec.Fill)
		}
		return svgData, nil
	}

	// Bundled icon lookup
	svgData, err := icons.Lookup(spec.Name)
	if err != nil {
		return nil, err
	}
	if spec.Fill != "" {
		svgData = applyIconFill(svgData, spec.Fill)
	}
	return svgData, nil
}

// generateConnectorXML creates a p:cxnSp XML fragment for a resolved connector.
func generateConnectorXML(conn shapegrid.ResolvedConnector) ([]byte, error) {
	spec := conn.Spec

	// Resolve line color and width
	color := spec.Color
	if color == "" {
		color = "000000"
	}
	width := spec.Width
	if width == 0 {
		width = 1.0
	}

	line := pptx.ResolveColorLinePoints(width, color)
	if spec.Dash != "" {
		line.Dash = spec.Dash
	}

	opts := pptx.ConnectorOptions{
		ID:       conn.ID,
		Geometry: pptx.GeomStraightConnector1,
		Bounds:   conn.Bounds,
		Line:     line,
		StartConn: &pptx.ConnectionRef{
			ShapeID: conn.SourceID,
			SiteIdx: conn.StartSite,
		},
		EndConn: &pptx.ConnectionRef{
			ShapeID: conn.TargetID,
			SiteIdx: conn.EndSite,
		},
	}

	// Add arrowhead for "arrow" style
	if spec.Style == "arrow" {
		opts.TailEnd = &pptx.ArrowHead{
			Type: "triangle",
			W:    "med",
			Len:  "med",
		}
	}

	return pptx.GenerateConnector(opts)
}

// applyIconFill recolors an SVG icon by replacing color attributes on the root <svg> element.
//
// Outline icons (Lucide/Tabler) use fill="none" + stroke="currentColor":
//   - stroke="currentColor" is replaced with stroke="<color>"
//   - fill="none" is kept as-is (outline icons should remain unfilled)
//
// Filled icons use fill="currentColor":
//   - fill="currentColor" is replaced with fill="<color>"
//
// This avoids creating duplicate attributes (invalid XML that LibreOffice rejects).
func applyIconFill(svgData []byte, fill string) []byte {
	s := string(svgData)
	// Find the opening <svg tag
	svgStart := strings.Index(s, "<svg")
	if svgStart < 0 {
		return svgData
	}
	// Find the end of the opening tag
	closeIdx := strings.Index(s[svgStart:], ">")
	if closeIdx < 0 {
		return svgData
	}
	tagEnd := svgStart + closeIdx

	// Extract just the opening <svg ...> tag for attribute replacement
	tag := s[svgStart:tagEnd]
	modified := false

	// Escape fill for safe insertion as an XML attribute value.
	escapedFill := html.EscapeString(fill)

	// Replace stroke="currentColor" so outline icons show the requested color
	if i := strings.Index(tag, ` stroke="currentColor"`); i >= 0 {
		tag = tag[:i] + fmt.Sprintf(` stroke="%s"`, escapedFill) + tag[i+len(` stroke="currentColor"`):]
		modified = true
	}

	// Replace fill="currentColor" for filled icons
	if i := strings.Index(tag, ` fill="currentColor"`); i >= 0 {
		tag = tag[:i] + fmt.Sprintf(` fill="%s"`, escapedFill) + tag[i+len(` fill="currentColor"`):]
		modified = true
	}

	// If no currentColor attributes were found, insert fill as attribute
	// (but don't duplicate an existing fill attribute)
	if !modified {
		if i := strings.Index(tag, ` fill="`); i >= 0 {
			// Replace existing fill value
			end := strings.Index(tag[i+7:], `"`)
			if end >= 0 {
				tag = tag[:i] + fmt.Sprintf(` fill="%s"`, escapedFill) + tag[i+7+end+1:]
			}
		} else {
			tag = tag + fmt.Sprintf(` fill="%s"`, escapedFill)
		}
	}

	return []byte(s[:svgStart] + tag + s[tagEnd:])
}
