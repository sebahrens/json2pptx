package shapegrid

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Validate checks a Grid for structural errors. It returns a multi-error
// (via errors.Join) containing all problems found, or nil if the grid is valid.
func Validate(grid *Grid) error { //nolint:gocognit,gocyclo
	if grid == nil {
		return nil
	}

	var errs []error

	if len(grid.Columns) == 0 {
		errs = append(errs, fmt.Errorf("shape_grid: empty columns; set \"columns\" to a number (e.g. 3) or an array of percentages (e.g. [30, 40, 30])"))
	}

	errs = append(errs, ValidateTrackWeights(grid)...)

	numCols := len(grid.Columns)
	numRows := len(grid.Rows)

	if numRows == 0 {
		errs = append(errs, fmt.Errorf("shape_grid: no rows defined; add at least one entry to the \"rows\" array, each containing a \"cells\" array"))
	}

	// Track occupied cells for overlap detection
	occupied := make([][]bool, numRows)
	for r := range occupied {
		occupied[r] = make([]bool, numCols)
	}

	for r, row := range grid.Rows {
		col := 0
		for ci, cell := range row.Cells {
			// Skip occupied cells
			for col < numCols && occupied[r][col] {
				col++
			}

			// Enforce payload exclusivity: at most one of {shape, table, icon,
			// image, diagram, composite} may be set per cell. Legacy carve-out:
			// a cell with exactly {shape, icon} is permitted (icon-overlay
			// rendering). Composite cells must not be combined with any of the
			// legacy payload keys.
			var present []string
			if cell.Shape != nil {
				present = append(present, "shape")
			}
			if cell.TableSpec != nil {
				present = append(present, "table")
			}
			if cell.Icon != nil {
				present = append(present, "icon")
			}
			if cell.Image != nil {
				present = append(present, "image")
			}
			if cell.DiagramSpec != nil {
				present = append(present, "diagram")
			}
			if cell.Composite != nil {
				present = append(present, "composite")
			}
			isShapeIconOverlay := len(present) == 2 && cell.Shape != nil && cell.Icon != nil
			if len(present) > 1 && !isShapeIconOverlay {
				if len(present) == 2 && cell.Shape != nil && cell.TableSpec != nil {
					// Preserve the historical phrasing for the shape+table case
					// so existing error consumers and tests keep working.
					errs = append(errs, fmt.Errorf("row %d col %d: cell has both shape and table (only one allowed); remove either the \"shape\" or \"table\" key from this cell", r, ci))
				} else if cell.Composite != nil {
					// Highlight composite conflicts explicitly so agents fix the
					// composite-vs-legacy collision rather than the cosmetic key set.
					var others []string
					for _, p := range present {
						if p != "composite" {
							others = append(others, p)
						}
					}
					errs = append(errs, fmt.Errorf("row %d col %d: cell has \"composite\" alongside legacy payload keys (%s); composite bundles text + sub_diagram and must not be combined with \"shape\", \"table\", \"icon\", \"image\", or \"diagram\"", r, ci, strings.Join(others, ", ")))
				} else {
					errs = append(errs, fmt.Errorf("row %d col %d: cell has conflicting payload keys (%s); only one of \"shape\", \"table\", \"icon\", \"image\", \"diagram\", \"composite\" may be set per cell (the shape+icon overlay is the sole exception)", r, ci, strings.Join(present, ", ")))
				}
			}

			if cell.Composite != nil {
				if cell.Composite.Text == nil {
					errs = append(errs, fmt.Errorf("row %d col %d: composite cell missing \"text\" — composite requires both \"text\" (native text shape) and \"sub_diagram\"", r, ci))
				}
				if cell.Composite.SubDiagram == nil {
					errs = append(errs, fmt.Errorf("row %d col %d: composite cell missing \"sub_diagram\" — composite requires both \"text\" (native text shape) and \"sub_diagram\"", r, ci))
				}
				switch cell.Composite.Split {
				case CompositeSplitDefault, CompositeSplitTop, CompositeSplitBottom:
					// valid
				default:
					errs = append(errs, fmt.Errorf("row %d col %d: composite split %q invalid; valid values are \"top\" (default — text on top) or \"bottom\"", r, ci, cell.Composite.Split))
				}
				if cell.Composite.Ratio != 0 {
					if cell.Composite.Ratio <= 0 || cell.Composite.Ratio >= 1 {
						errs = append(errs, fmt.Errorf("row %d col %d: composite ratio %g outside (0,1); set ratio between 0 and 1 (e.g. 0.4 = text portion gets 40%% of cell height) or omit for default 0.5", r, ci, cell.Composite.Ratio))
					}
				}
			}

			errs = append(errs, validateLayers(cell, r, ci)...)

			if cell.Fit != "" && cell.Fit != FitContain && cell.Fit != FitWidth && cell.Fit != FitHeight {
				errs = append(errs, fmt.Errorf("row %d col %d: invalid fit mode %q; valid values are \"contain\", \"fit-width\", or \"fit-height\" (omit for default stretch behavior)", r, ci, cell.Fit))
			}

			if cell.Image != nil && cell.Image.Geometry != "" && cell.Image.Geometry != "rect" && cell.Image.Geometry != "ellipse" {
				errs = append(errs, fmt.Errorf("row %d col %d: invalid image geometry %q; valid values are \"rect\" or \"ellipse\" (omit for rect)", r, ci, cell.Image.Geometry))
			}
			if cell.Image != nil && cell.Image.Fit != "" && cell.Image.Fit != "cover" && cell.Image.Fit != "contain" {
				errs = append(errs, fmt.Errorf("row %d col %d: invalid image fit %q; valid values are \"cover\" or \"contain\" (omit for cover)", r, ci, cell.Image.Fit))
			}

			// Empty spacer cells are not skipped: they claim their
			// col_span × row_span footprint exactly as Resolve does, so span
			// and overlap checks see the same layout that renders
			// (go-slide-creator-s1uvj.38). Trailing empty cells past the last
			// free column (patterns emit nil cells for positions covered by an
			// earlier row_span) render nothing and are ignored, as Resolve
			// stops at the grid edge.
			isEmpty := !cell.hasContent()
			if isEmpty && col >= numCols {
				continue
			}

			colSpan := cell.ColSpan
			if colSpan < 1 {
				colSpan = 1
			}
			rowSpan := cell.RowSpan
			if rowSpan < 1 {
				rowSpan = 1
			}

			// Check col_span exceeds grid
			if col+colSpan > numCols {
				errs = append(errs, fmt.Errorf("row %d col %d: col_span %d exceeds grid width %d; reduce col_span to at most %d, or add more columns", r, ci, colSpan, numCols, numCols-col))
			}

			// Check row_span exceeds grid
			if r+rowSpan > numRows {
				errs = append(errs, fmt.Errorf("row %d col %d: row_span %d exceeds grid height %d; reduce row_span to at most %d, or add more rows", r, ci, rowSpan, numRows, numRows-r))
			}

			// Mark cells as occupied, detecting overlaps
			for dr := 0; dr < rowSpan && r+dr < numRows; dr++ {
				for dc := 0; dc < colSpan && col+dc < numCols; dc++ {
					if occupied[r+dr][col+dc] {
						errs = append(errs, fmt.Errorf("row %d col %d: cell overlap at row %d col %d; another cell's col_span or row_span already covers this position — reduce spans or rearrange cells", r, ci, r+dr, col+dc))
					}
					occupied[r+dr][col+dc] = true
				}
			}

			col += colSpan
		}
	}

	return errors.Join(errs...)
}

// validateLayers checks a cell's layers: each needs a shape and a frame that
// stays inside the cell, and layers stack only on a shape, icon or image cell
// or on an otherwise empty one.
func validateLayers(cell Cell, r, ci int) []error {
	if len(cell.Layers) == 0 {
		return nil
	}
	var errs []error
	for _, host := range []struct {
		set  bool
		name string
	}{{cell.TableSpec != nil, "table"}, {cell.DiagramSpec != nil, "diagram"}, {cell.Composite != nil, "composite"}, {cell.Placeholder, "grid / pattern"}} {
		if host.set {
			errs = append(errs, fmt.Errorf("row %d col %d: cell has \"layers\" alongside \"%s\"; layers stack on a \"shape\", \"icon\" or \"image\" cell, or on a cell with no other content — move the %s to its own cell", r, ci, host.name, host.name))
		}
	}
	for i, layer := range cell.Layers {
		label := fmt.Sprintf("row %d col %d: layers[%d]", r, ci, i)
		if layer.Name != "" {
			label += fmt.Sprintf(" (%q)", layer.Name)
		}
		if layer.Shape == nil {
			errs = append(errs, fmt.Errorf("%s: missing \"shape\"; a layer is a shape in a frame, e.g. {\"frame\": {\"x\": 0, \"y\": 0, \"w\": 1, \"h\": 1}, \"shape\": {\"geometry\": \"ellipse\"}}", label))
		}
		f := layer.Frame
		finite := true
		for _, v := range []float64{f.X, f.Y, f.W, f.H} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				finite = false
			}
		}
		switch {
		case !finite:
			errs = append(errs, fmt.Errorf("%s: %s: frame {x: %g, y: %g, w: %g, h: %g} is not a finite rectangle; give x, y, w and h as fractions (0..1) of the cell", label, LayerFrameOutOfCell, f.X, f.Y, f.W, f.H))
		case f.W <= 0 || f.H <= 0:
			errs = append(errs, fmt.Errorf("%s: %s: frame {x: %g, y: %g, w: %g, h: %g} has no area; w and h are fractions of the cell and must be greater than 0 (at most 1)", label, LayerFrameOutOfCell, f.X, f.Y, f.W, f.H))
		case f.X < -layerFrameSlack || f.Y < -layerFrameSlack || f.X+f.W > 1+layerFrameSlack || f.Y+f.H > 1+layerFrameSlack:
			errs = append(errs, fmt.Errorf("%s: %s: frame {x: %g, y: %g, w: %g, h: %g} leaves the cell; x, y, w and h are fractions (0..1) of the cell's fitted bounds with x+w <= 1 and y+h <= 1", label, LayerFrameOutOfCell, f.X, f.Y, f.W, f.H))
		}
	}
	return errs
}

// ValidateTrackWeights rejects column widths and row height/flex weights
// that cannot describe a layout: negative or non-finite values (a negative
// column width produced a negative cx that later stages accepted) and a
// columns array whose widths sum to zero (go-slide-creator-s1uvj.39).
func ValidateTrackWeights(grid *Grid) []error {
	var errs []error
	bad := func(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) || v < 0 }

	var colSum float64
	for i, c := range grid.Columns {
		if bad(c) {
			errs = append(errs, fmt.Errorf("shape_grid: columns[%d] is %g; column widths must be finite and >= 0 (percentages such as [30, 40, 30], or a number for equal columns)", i, c))
			continue
		}
		colSum += c
	}
	if len(grid.Columns) > 0 && len(errs) == 0 && colSum == 0 {
		errs = append(errs, fmt.Errorf("shape_grid: columns widths sum to 0; give at least one column a positive width (e.g. [30, 40, 30]) or use a number for equal columns"))
	}

	for r, row := range grid.Rows {
		if bad(row.Height) {
			errs = append(errs, fmt.Errorf("shape_grid: rows[%d].height is %g; row height must be a finite percentage >= 0 (0 = flex row)", r, row.Height))
		}
		if bad(row.Flex) {
			errs = append(errs, fmt.Errorf("shape_grid: rows[%d].flex is %g; flex must be finite and >= 0", r, row.Flex))
		}
		if bad(row.MinHeight) {
			errs = append(errs, fmt.Errorf("shape_grid: rows[%d].min_height is %g; min_height must be finite and >= 0 points", r, row.MinHeight))
		}
		if bad(row.MaxHeight) {
			errs = append(errs, fmt.Errorf("shape_grid: rows[%d].max_height is %g; max_height must be finite and >= 0 points", r, row.MaxHeight))
		}
		if row.Rule != "" && row.Rule != "above" && row.Rule != "below" && row.Rule != "both" {
			errs = append(errs, fmt.Errorf("shape_grid: rows[%d].rule is %q; valid values are \"above\", \"below\" or \"both\" (omit for no rule)", r, row.Rule))
		}
		if len(row.Band) > 0 {
			if _, err := ResolveFillInput(row.Band); err != nil {
				errs = append(errs, fmt.Errorf("shape_grid: rows[%d].band is not a fill (a colour string or a {color, alpha, lumMod, lumOff} object): %w", r, err))
			}
		}
		for c, cell := range row.Cells {
			for _, f := range []struct {
				name string
				v    float64
			}{{"bleed_left", cell.BleedLeft}, {"bleed_top", cell.BleedTop}, {"inset_top", cell.InsetTop}, {"inset_bottom", cell.InsetBottom}} {
				if bad(f.v) {
					errs = append(errs, fmt.Errorf("shape_grid: rows[%d].cells[%d].%s is %g; %s must be finite and >= 0 points", r, c, f.name, f.v, f.name))
				}
			}
		}
	}
	return errs
}
