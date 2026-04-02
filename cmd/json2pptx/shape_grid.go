package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/icons"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// ShapeGridResult holds the output of resolveShapeGrid: both the raw XML
// fragments ready for injection and the resolved cell metadata (bounds, IDs,
// specs) for downstream processing such as icon insertion or validation.
type ShapeGridResult struct {
	Shapes       [][]byte                 // Raw <p:sp>/<p:graphicFrame> XML fragments
	Cells        []shapegrid.ResolvedCell // Resolved cell metadata with absolute coordinates
	IconInserts  []generator.IconInsert   // Icon cells requiring media registration in the generator
	ImageInserts []generator.ImageInsert  // Image cells requiring media registration in the generator
}

// virtualLayoutResult holds the result of virtual layout resolution.
type virtualLayoutResult struct {
	LayoutID string                // Selected base layout ID
	Bounds   pptx.RectEmu         // Computed grid bounds from placeholder metadata
	Zone     *shapegrid.ContentZone // Template-derived safe content area (nil if unavailable)
}

// resolveVirtualLayout selects a base layout for shape_grid slides and computes
// grid bounds from the layout's placeholder metadata.
//
// Selection priority:
//  1. Layout tagged "blank" with a title placeholder
//  2. Layout tagged "blank-title" (synthesized)
//  3. Any layout with a body/content placeholder (bounds = body placeholder)
//
// Returns nil if no suitable layout is found.
func resolveVirtualLayout(layouts []types.LayoutMetadata, slideWidth, slideHeight int64) *virtualLayoutResult {
	// Priority 1 & 2: find blank or blank-title layout with title placeholder
	var blankLayout, blankTitleLayout *types.LayoutMetadata
	for i := range layouts {
		for _, tag := range layouts[i].Tags {
			if tag == "blank-title" {
				blankTitleLayout = &layouts[i]
			}
			if tag == "blank" {
				// Check if it has a title placeholder
				for _, ph := range layouts[i].Placeholders {
					if ph.Type == types.PlaceholderTitle {
						blankLayout = &layouts[i]
						break
					}
				}
			}
		}
	}

	// Try blank with title first, then blank-title
	if chosen := pickBlankLayout(blankLayout, blankTitleLayout, slideWidth, slideHeight); chosen != nil {
		return chosen
	}

	// Priority 3: any layout with a body/content placeholder
	for i := range layouts {
		for _, ph := range layouts[i].Placeholders {
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				bounds := pptx.RectEmu{
					X:  ph.Bounds.X,
					Y:  ph.Bounds.Y,
					CX: ph.Bounds.Width,
					CY: ph.Bounds.Height,
				}
				return &virtualLayoutResult{
					LayoutID: layouts[i].ID,
					Bounds:   shapegrid.BoundsFromPlaceholder(bounds),
				}
			}
		}
	}

	return nil
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

// needsVirtualLayout returns true if the slide should use virtual layout resolution
// (has shape_grid and either no layout_id or a blank/virtual slide type).
func needsVirtualLayout(slide SlideInput) bool {
	if slide.ShapeGrid == nil {
		return false
	}
	st := strings.ToLower(slide.SlideType)
	return slide.LayoutID == "" || st == "blank" || st == "virtual"
}

// resolveShapeGrid converts a ShapeGridInput into a ShapeGridResult containing
// both raw XML fragments and resolved cell metadata.
// If overrideBounds is non-nil, it is used instead of DefaultBounds or input.Bounds.
// If zone is non-nil, explicit input.Bounds are clamped against it to prevent content
// from overlapping title or footer chrome.
// slideWidth and slideHeight are the template's actual slide dimensions in EMU (0 = use 16:9 defaults).
func resolveShapeGrid(input *ShapeGridInput, alloc *pptx.ShapeIDAllocator, overrideBounds *pptx.RectEmu, zone *shapegrid.ContentZone, slideWidth, slideHeight int64) (*ShapeGridResult, error) { //nolint:gocognit,gocyclo
	if input == nil || len(input.Rows) == 0 {
		return nil, nil
	}

	// Convert JSON DTO columns to []float64
	colWidths, err := resolveColumnsDTO(input.Columns, input.Rows)
	if err != nil {
		return nil, err
	}

	// Resolve bounds: explicit input.Bounds > overrideBounds > default
	var bounds pptx.RectEmu
	if input.Bounds != nil {
		bounds = shapegrid.BoundsFromPercentages(input.Bounds.X, input.Bounds.Y, input.Bounds.Width, input.Bounds.Height, slideWidth, slideHeight)
		// Clamp explicit bounds against ContentZone to prevent overlapping chrome
		if zone != nil {
			bounds = shapegrid.ClampBoundsToZone(bounds, *zone)
		}
	} else if overrideBounds != nil {
		bounds = *overrideBounds
	} else if zone != nil {
		bounds = shapegrid.DefaultBoundsFromZone(*zone, 9.0)
	} else {
		bounds = shapegrid.DefaultBounds(slideWidth, slideHeight)
	}

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
	rows := make([]shapegrid.Row, len(input.Rows))
	for i, r := range input.Rows {
		cells := make([]shapegrid.Cell, len(r.Cells))
		for j, c := range r.Cells {
			if c == nil || (c.Shape == nil && c.Table == nil && c.Icon == nil && c.Image == nil) {
				cells[j] = shapegrid.Cell{}
				continue
			}
			cell := shapegrid.Cell{
				ColSpan: c.ColSpan,
				RowSpan: c.RowSpan,
				Fit:     shapegrid.FitMode(c.Fit),
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
					Fill:     c.Icon.Fill,
					Position: c.Icon.Position,
				}
			}
			// Support icon nested inside shape (e.g. {"shape": {"fill": "accent1", "icon": {"name": "shield"}}})
			if c.Shape != nil && c.Shape.Icon != nil && cell.Icon == nil {
				cell.Icon = &shapegrid.IconSpec{
					Name:     c.Shape.Icon.Name,
					Path:     c.Shape.Icon.Path,
					Fill:     c.Shape.Icon.Fill,
					Position: c.Shape.Icon.Position,
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
			if c.AccentBar != nil {
				cell.AccentBar = &shapegrid.AccentBarSpec{
					Position: c.AccentBar.Position,
					Color:    c.AccentBar.Color,
					Width:    c.AccentBar.Width,
				}
			}
			cells[j] = cell
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
			Cells:      cells,
			Connector:  connSpec,
		}
	}

	grid := &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    rows,
		ColGap:  colGap,
		RowGap:  rowGap,
	}

	// Resolve grid into cells with absolute coordinates
	result, err := shapegrid.Resolve(grid, alloc)
	if err != nil {
		return nil, err
	}

	// Generate XML for each resolved cell
	var shapes [][]byte
	var iconInserts []generator.IconInsert
	var imageInserts []generator.ImageInsert
	for _, cell := range result.Cells {
		switch cell.Kind {
		case shapegrid.CellKindShape:
			xml, err := shapegrid.GenerateShapeXML(cell.ShapeSpec, cell.ID, cell.Bounds, cell.TextInsets)
			if err != nil {
				return nil, fmt.Errorf("shape id %d: %w", cell.ID, err)
			}
			shapes = append(shapes, xml)
			// If the shape also has an icon overlay, emit an icon insert on top
			if cell.IconSpec != nil {
				svgData, err := resolveIconSVG(cell.IconSpec)
				if err != nil {
					return nil, fmt.Errorf("icon overlay on shape id %d: %w", cell.ID, err)
				}
				ib := cell.IconBounds
				if ib.CX == 0 && ib.CY == 0 {
					ib = cell.Bounds // fallback
				}
				iconInserts = append(iconInserts, generator.IconInsert{
					SVGData:  svgData,
					OffsetX:  ib.X,
					OffsetY:  ib.Y,
					ExtentCX: ib.CX,
					ExtentCY: ib.CY,
				})
			}
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
			result, err := generator.GenerateTableXML(cell.TableSpec, cfg)
			if err != nil {
				return nil, fmt.Errorf("table in grid: %w", err)
			}
			shapes = append(shapes, []byte(result.XML))
		case shapegrid.CellKindIcon:
			svgData, err := resolveIconSVG(cell.IconSpec)
			if err != nil {
				return nil, fmt.Errorf("icon in grid: %w", err)
			}
			iconInserts = append(iconInserts, generator.IconInsert{
				SVGData:  svgData,
				OffsetX:  cell.Bounds.X,
				OffsetY:  cell.Bounds.Y,
				ExtentCX: cell.Bounds.CX,
				ExtentCY: cell.Bounds.CY,
			})
		case shapegrid.CellKindImage:
			imageInserts = append(imageInserts, generator.ImageInsert{
				Path:     cell.ImageSpec.Path,
				Alt:      cell.ImageSpec.Alt,
				OffsetX:  cell.Bounds.X,
				OffsetY:  cell.Bounds.Y,
				ExtentCX: cell.Bounds.CX,
				ExtentCY: cell.Bounds.CY,
			})
			// Generate overlay rectangle on top of image
			if cell.ImageSpec.Overlay != nil {
				overlayID := alloc.Alloc()
				overlayXML, err := shapegrid.GenerateImageOverlayXML(cell.ImageSpec.Overlay, overlayID, cell.Bounds)
				if err != nil {
					return nil, fmt.Errorf("image overlay id %d: %w", overlayID, err)
				}
				shapes = append(shapes, overlayXML)
			}
			// Generate text label on top of image (and overlay)
			if cell.ImageSpec.Text != nil {
				textID := alloc.Alloc()
				textXML, err := shapegrid.GenerateImageTextXML(cell.ImageSpec.Text, textID, cell.Bounds)
				if err != nil {
					return nil, fmt.Errorf("image text id %d: %w", textID, err)
				}
				shapes = append(shapes, textXML)
			}
		default:
			return nil, fmt.Errorf("unsupported cell kind: %s", cell.Kind)
		}
	}

	// Generate XML for accent bars
	for _, bar := range result.AccentBars {
		xml, err := shapegrid.GenerateAccentBarXML(&bar)
		if err != nil {
			return nil, fmt.Errorf("accent bar id %d: %w", bar.ID, err)
		}
		shapes = append(shapes, xml)
	}

	// Generate XML for connectors between cells
	for _, conn := range result.Connectors {
		xml, err := generateConnectorXML(conn)
		if err != nil {
			return nil, fmt.Errorf("connector id %d: %w", conn.ID, err)
		}
		shapes = append(shapes, xml)
	}

	return &ShapeGridResult{
		Shapes:       shapes,
		Cells:        result.Cells,
		IconInserts:  iconInserts,
		ImageInserts: imageInserts,
	}, nil
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
			return nil, fmt.Errorf("shape_grid: no cells defined; add cells with a \"shape\", \"table\", \"icon\", or \"image\" key to at least one row")
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

// resolveIconPaths resolves and validates icon.path fields across all slides.
// Relative paths are resolved against baseDir (the directory containing the JSON input file).
// Each path is cleaned, converted to absolute form, evaluated for symlinks, and validated
// against path traversal attacks.
func resolveIconPaths(slides []SlideInput, baseDir string) error {
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
					if err := resolveIconInputPath(cell.Icon, baseDir, i+1); err != nil {
						return err
					}
				}
				// Resolve icon nested inside shape
				if cell.Shape != nil && cell.Shape.Icon != nil {
					if err := resolveIconInputPath(cell.Shape.Icon, baseDir, i+1); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// resolveIconInputPath validates and resolves a single IconInput's path field.
// Returns an error if more than one of name/path/url is set, or if none is set,
// or if the path is unsafe (traversal/symlink escape).
// URL-based icons are skipped here — they are resolved by resolveURLs.
func resolveIconInputPath(icon *IconInput, baseDir string, slideNum int) error {
	hasName := icon.Name != ""
	hasPath := icon.Path != ""
	hasURL := icon.URL != ""

	set := 0
	if hasName {
		set++
	}
	if hasPath {
		set++
	}
	if hasURL {
		set++
	}

	if set > 1 {
		return fmt.Errorf("slide %d: icon must have exactly one of 'name', 'path', or 'url'", slideNum)
	}
	if set == 0 {
		return fmt.Errorf("slide %d: icon must have one of 'name', 'path', or 'url'", slideNum)
	}

	if !hasPath {
		return nil // bundled icon or URL — no local path resolution needed
	}

	// Resolve relative path against baseDir
	p := filepath.FromSlash(icon.Path)
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	p = filepath.Clean(p)

	// Evaluate symlinks for security
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return fmt.Errorf("slide %d: icon path %q: %w", slideNum, icon.Path, err)
	}

	// Validate against path traversal
	if err := utils.ValidatePath(resolved, nil); err != nil {
		return fmt.Errorf("slide %d: icon path %q: %w", slideNum, icon.Path, err)
	}

	// Update the path to the resolved absolute path
	icon.Path = resolved
	return nil
}

// resolveIconSVG loads SVG bytes for an icon spec, optionally applying a fill color override.
// For bundled icons (Name set), it looks up from the embedded icon library.
// For custom icons (Path set), it reads the SVG file from disk. Fill rewrite is skipped for custom SVGs.
func resolveIconSVG(spec *shapegrid.IconSpec) ([]byte, error) {
	if spec.Path != "" {
		// Custom SVG from file path (already resolved to absolute path)
		svgData, err := os.ReadFile(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("custom icon %q: %w", spec.Path, err)
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

	line := pptx.SolidLinePoints(width, color)
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

	// Replace stroke="currentColor" so outline icons show the requested color
	if i := strings.Index(tag, ` stroke="currentColor"`); i >= 0 {
		tag = tag[:i] + fmt.Sprintf(` stroke="%s"`, fill) + tag[i+len(` stroke="currentColor"`):]
		modified = true
	}

	// Replace fill="currentColor" for filled icons
	if i := strings.Index(tag, ` fill="currentColor"`); i >= 0 {
		tag = tag[:i] + fmt.Sprintf(` fill="%s"`, fill) + tag[i+len(` fill="currentColor"`):]
		modified = true
	}

	// If no currentColor attributes were found, insert fill as attribute
	// (but don't duplicate an existing fill attribute)
	if !modified {
		if i := strings.Index(tag, ` fill="`); i >= 0 {
			// Replace existing fill value
			end := strings.Index(tag[i+7:], `"`)
			if end >= 0 {
				tag = tag[:i] + fmt.Sprintf(` fill="%s"`, fill) + tag[i+7+end+1:]
			}
		} else {
			tag = tag + fmt.Sprintf(` fill="%s"`, fill)
		}
	}

	return []byte(s[:svgStart] + tag + s[tagEnd:])
}
